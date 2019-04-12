// +build integration

// Run as: go test -tags=integration
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdotgordon/produce-demo/types"
)

var (
	produceAddr string
	prodClient  *http.Client
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	produceAddr, _ = getAppAddr("8080", "produce-demo")
	prodClient = &http.Client{}
	os.Exit(m.Run())
}

// First, ensure random Produce generator is working!
func TestRandGen(t *testing.T) {
	prods := createRandomProduce(5, 25)
	if len(prods) != 25 {
		t.Fatal("wrong array length", len(prods))
	}
	for i := 5; i <= 29; i++ {
		v := prods[i-5]
		if types.ValidateAndConvertProduce(&v) != "" {
			t.Fatalf("produce item not valid: %s",
				types.ValidateAndConvertProduce(&v))
		}
		if !strings.HasPrefix(v.Code, fmt.Sprintf("%04d", i)) {
			t.Fatalf("code should begin with %04d, but has %s", i, v.Code[:4])
		}
	}
}

func TestStatus(t *testing.T) {
	resp, err := http.Get("http://" + produceAddr + "/v1/status")
	if err != nil {
		t.Fatalf("status failed: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Unexpected return code: %d", resp.StatusCode)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("error reading response body", err)
	}
	var statResp map[string]string
	if err := json.Unmarshal(b, &statResp); err != nil {
		t.Fatal("error deserializing JSON", err)
	}
	if statResp["status"] != "produce service is up and running" {
		t.Fatal("unexpected status repsonse", statResp["status"])
	}
}

// While the test following this one is more concerned with the exercising
// various add/list/delete sceanrios in a structured and predictable way,
// the purpose of this test is to launch all three operation types concurrently,
// and ensure that everything works well and ends in the expected state.
func TestConcurrency(t *testing.T) {
	invokeReset(t)
	done := make(chan struct{})

	// Create pairs of items to add.
	itemCnt := 100
	items := createRandomProduce(1, itemCnt)
	partitions := partitionBlocks(items, 2)

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var gotError uint32
	var addCnt uint32

	// Fire off a goroutine that gets (lists) all the items and then sleeps,
	// exiting either when it gets 0 items, or is cancelled.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			status, _, err := invokeListAll()
			if err != nil {
				atomic.AddUint32(&gotError, 1)
				close(done)
				return
			}
			if status != http.StatusOK {
				atomic.AddUint32(&gotError, 1)
				close(done)
				return
			}
			if atomic.LoadUint32(&addCnt) == uint32(len(items)) {
				close(done)
				return
			}

			tick := time.NewTicker(50 * time.Millisecond)
			select {
			case <-tick.C:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Fire off a goroutine for each produce code that keeps trying
	// until it successfully deletes it.
	for i := len(items) - 1; i >= 0; i-- {
		wg.Add(1)
		code := items[i].Code
		go func() {
			defer wg.Done()

			for {
				tick := time.NewTicker(200 * time.Millisecond)
				select {
				case <-tick.C:
				case <-ctx.Done():
					return
				}

				status, err := invokeDelete(code)
				if err != nil {
					atomic.AddUint32(&gotError, 1)
					return
				}

				switch status {
				case http.StatusNoContent:
					return
				case http.StatusNotFound:
				default:
					atomic.AddUint32(&gotError, 1)
					return
				}
			}
		}()
	}

	// Finally, fire off the goroutines that add the pairs of items.
	for i := range partitions {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Let the delete and list get started first
			tick := time.NewTicker(500 * time.Millisecond)
			select {
			case <-tick.C:
			case <-ctx.Done():
				return
			}
			status, _, err := invokeAdd(partitions[i])
			if err != nil {
				atomic.AddUint32(&gotError, 1)
				return
			}

			switch status {
			case http.StatusCreated:
				atomic.AddUint32(&addCnt, 2)
				return
			default:
				atomic.AddUint32(&gotError, 1)
				return
			}
		}()
	}

	tkr := time.NewTicker(10 * time.Second)
	select {
	case <-done:
	case <-tkr.C:
		cancel()
	}

	wg.Wait()
	_, res, _ := invokeListAll()

	if gotError != 0 {
		t.Fatal("unexpected error(s)")
	}
	if len(res) != 0 {
		t.Fatal("list length was", len(res))
	}
	if int(addCnt) != itemCnt {
		t.Fatal("item add count was", addCnt, "expected", itemCnt)
	}
}

// Test concurrently adding items, ensure the returned list is correct.
// Then concurrently delete the items, checking the codes and finally
// that the list is empty.  Testing with both individual and batched add
// produce requests are trsted.
func TestAddListDelete(t *testing.T) {
	for c, v := range []struct {
		numGood int // number of good items to add
		numDup  int // number of duplicate adds
		numBad  int // number of bad format adds
		blkSize int // invoke adds in blocks
	}{
		{
			numGood: 25,
		},
		{
			numGood: 25,
			blkSize: 6,
		},
		{
			numGood: 25,
			blkSize: 6,
			numDup:  2,
			numBad:  3,
		},
		{
			numGood: 25,
			numDup:  2,
			numBad:  3,
		},
		{
			numGood: 200,
			blkSize: 7,
			numDup:  13,
			numBad:  11,
		},
	} {
		invokeReset(t)
		items := createRandomProduce(1, v.numGood)
		if v.numDup > 0 {
			items = append(items, items[:v.numDup]...)
		}
		if v.numBad > 0 {
			for i := 0; i < v.numBad; i++ {
				cp := items[i]
				cp.Name = "@@@@%%%%$$$$$!!!!!"
				items = append(items, cp)
			}
		}

		// Add the items and wait for them to complete.  The code is long
		// enough that it is moved into a separate function.
		succCnt, badReqCnt, conflictCnt, err := runAdds(items, v.blkSize)
		if err != nil {
			t.Fatalf("(%d) app phas failed: %v", c, err)
		}
		if succCnt != v.numGood {
			t.Fatalf("(%d) expected %d success, got %d", c, v.numGood, succCnt)
		}
		if badReqCnt != v.numBad {
			t.Fatalf("(%d) expected %d bad requests, got %d", c, v.numBad, badReqCnt)
		}
		if conflictCnt != v.numDup {
			t.Fatalf("(%d) expected %d conflicts, got %d", c, v.numDup, conflictCnt)
		}

		// Compare the two lists, sorintg, and converting the incoming list to
		// canonical.
		status, litems, err := invokeListAll()
		if err != nil {
			t.Fatal("error listing items", err)
		}
		if status != http.StatusOK {
			t.Fatalf("(%d) list returned unexpcted status: %d", c, status)
		}
		if len(litems) != int(v.numGood) {
			t.Fatalf("(%d) expected %d list items, got %d", c, v.numGood, len(litems))
		}

		// Save the keys for the delete test.
		keys := make([]string, len(items))
		for i := range items {
			keys[i] = items[i].Code
		}

		goodItems := make([]types.Produce, v.numGood)
		copy(goodItems, items)
		for i := range goodItems {
			goodItems[i] = toUpper(items[i])
		}
		sort.Sort(produceSorter{goodItems})
		sort.Sort(produceSorter{litems})
		for i, v := range goodItems {
			if v != litems[i] {
				t.Fatalf("(%d) list items don't match: %+v, %+v", c, v, litems[i])
			}
		}
		// Now delete the items, adding one of them twice, to generate a
		// error return code. No content is the HTTP code on success, not
		// found on error.
		var ncCnt uint32
		var nfCnt uint32
		var wg2 sync.WaitGroup
		var delErr error
		var dmu sync.Mutex
		for i := 0; i < len(items); i++ {
			i := i
			wg2.Add(1)
			go func() {
				defer wg2.Done()

				dstatus, err := invokeDelete(keys[i])
				if err != nil {
					fmt.Println("del err", err, i)
					dmu.Lock()
					delErr = err
					dmu.Unlock()
					return
				}
				switch dstatus {
				case http.StatusNoContent:
					atomic.AddUint32(&ncCnt, 1)
				case http.StatusNotFound:
					atomic.AddUint32(&nfCnt, 1)
				}
			}()
		}
		wg2.Wait()
		if delErr != nil {
			t.Fatalf("error occurred during delete phase: %v", delErr)
		}

		if int(ncCnt) != v.numGood {
			t.Fatalf("(%d) expected %d no content, got %d", c, v.numGood, ncCnt)
		}
		if int(nfCnt) != v.numDup+v.numBad {
			t.Fatalf("(%d) expected %d not found, got %d", c, v.numDup+v.numBad, nfCnt)
		}

		status, items, err = invokeListAll()
		if err != nil {
			t.Fatal("error listing items", err)
		}
		if status != http.StatusOK {
			t.Fatal("list returned unexpcted status", status)
		}
		if len(items) != 0 {
			t.Fatal("list was not empty", items)
		}
	}
}

// Called from TestAddListDelete to add the produce items.
func runAdds(items []types.Produce, blkSize int) (int, int, int, error) {
	// if using blocks, partition items into lists.
	var blks [][]types.Produce
	if blkSize != 0 {
		blks = partitionBlocks(items, blkSize)
	}

	// Add the items and wait for them to complete.
	var succCnt uint32
	var badReqCnt uint32
	var conflictCnt uint32
	var wg sync.WaitGroup
	var mu sync.Mutex
	var addErr error

	// Handle the block adds or the single adds, depending on the config.
	if blkSize != 0 {
		for i := range blks {
			// For a block add, the rules are:
			// - all adds succeed, then a simple 201 Created is returned.
			// - at least one fails, a 200 is returned, and the response body
			// is an array of ProduceAddRepsonses, each of which contains
			// the produce code and it's corresponding HTTP result.
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()

				status, resp, err := invokeAdd(blks[i])
				if err != nil {
					mu.Lock()
					addErr = err
					mu.Unlock()
					return
				}
				switch status {
				case http.StatusOK:
					// There is an array of results for HTTP 200.
					if resp == nil {
						mu.Lock()
						addErr = errors.New("Block add, no body for HTTP 200 response")
						mu.Unlock()
						return
					}
					for _, r := range resp {
						switch r.StatusCode {
						case http.StatusCreated:
							atomic.AddUint32(&succCnt, 1)
						case http.StatusBadRequest:
							atomic.AddUint32(&badReqCnt, 1)
						case http.StatusConflict:
							atomic.AddUint32(&conflictCnt, 1)
						}
					}
				case http.StatusCreated:
					atomic.AddUint32(&succCnt, uint32(len(blks[i])))
				case http.StatusBadRequest:
					atomic.AddUint32(&badReqCnt, 1)
				case http.StatusConflict:
					atomic.AddUint32(&conflictCnt, 1)
				}
			}()
			wg.Wait()
		}
	} else {
		for i := range items {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()

				if addErr != nil {
					return
				}
				status, err := invokeAddSingle(items[i])
				if err != nil {
					addErr = err
					return
				}
				switch status {
				case http.StatusCreated:
					atomic.AddUint32(&succCnt, 1)
				case http.StatusBadRequest:
					atomic.AddUint32(&badReqCnt, 1)
				case http.StatusConflict:
					atomic.AddUint32(&conflictCnt, 1)
				}
			}()
		}
		wg.Wait()
		if addErr != nil {
			return 0, 0, 0, addErr
		}
	}
	return int(succCnt), int(badReqCnt), int(conflictCnt), addErr
}

// Called during TestAddListDelete to add the produce items.
func partitionBlocks(items []types.Produce, blkSize int) [][]types.Produce {
	blks := make([][]types.Produce, (len(items)+blkSize-1)/blkSize)
	nxt := 0
	for i := 0; i < len(items); i += blkSize {
		var blen int
		if i+blkSize <= len(items) {
			blen = blkSize
		} else {
			blen = len(items) - i
		}
		blks[nxt] = items[i : i+blen]
		nxt++
	}
	return blks
}

func getAppAddr(port string, app ...string) (string, error) {
	var err error
	var res []byte
	for _, a := range app {
		res, err = exec.Command("docker", "port", a, port).CombinedOutput()
		if err == nil {
			break
		}
	}

	if err != nil {
		log.Fatalf("docker-compose error: failed to get exposed port: %v", err)
	}
	return string(res[:len(res)-1]), nil
}

// Form of add that takes an array of Produce
func invokeAdd(items types.ProduceAddRequest) (int, types.ProduceAddResponse, error) {
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+produceAddr+"/v1/produce",
		bytes.NewReader(b))
	if err != nil {
		return 0, nil, err
	}

	resp, err := prodClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	// There will only be a response if the status code is 200 (mixed results)
	var respItems types.ProduceAddResponse
	if resp.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return 0, nil, err
		}
		if err = json.Unmarshal(body, &respItems); err != nil {
			return 0, nil, err
		}
	}
	return resp.StatusCode, respItems, nil
}

// Form of add that takes a single Produce item
func invokeAddSingle(item types.Produce) (int, error) {
	b, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+produceAddr+"/v1/produce",
		bytes.NewReader(b))
	if err != nil {
		return 0, err
	}

	resp, err := prodClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

func invokeDelete(code string) (int, error) {
	req, err := http.NewRequest(http.MethodDelete,
		"http://"+produceAddr+"/v1/produce/"+code, nil)
	if err != nil {
		return 0, err
	}

	resp, err := prodClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func invokeListAll() (int, types.ProduceListResponse, error) {
	req, err := http.NewRequest(http.MethodGet, "http://"+produceAddr+"/v1/produce", nil)
	if err != nil {
		return 0, nil, err
	}

	resp, err := prodClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	var items types.ProduceListResponse

	if resp.StatusCode == http.StatusOK {
		if err = json.Unmarshal(body, &items); err != nil {
			return 0, nil, err
		}
	}
	return resp.StatusCode, items, nil
}

func invokeReset(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://"+produceAddr+"/v1/reset", nil)
	if err != nil {
		t.Fatal("reset error creating request", err)
	}

	resp, err := prodClient.Do(req)
	if err != nil {
		t.Fatal("reset returned unexpcted error", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatal("Bad status code resetting db", resp.StatusCode)
	}
}

// Creates random Produce items with unique codes.  It uses an integer
// sequence as the first four chars of the produce code to ensure uniqueness.
// Examples: {0003-OanB-HULV-oknJ Btdg $1.96}, {0014-QECf-bxZ9-c8wO BtG $2.24}
func createRandomProduce(from, count int) []types.Produce {
	alphaBlank := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ     ")
	alpha := alphaBlank[:len(alphaBlank)-5]
	rand.Seed(time.Now().Unix())
	res := make([]types.Produce, 0, count)
	for i := from; i < from+count; i++ {
		// Add random characters form alpha to name.
		p := types.Produce{}
		nlen := rand.Intn(12) + 5
		name := make([]byte, nlen)
		for j := 0; j < nlen; j++ {
			if j == 0 {
				name[j] = alpha[rand.Intn(len(alpha))]
			} else {
				name[j] = alphaBlank[rand.Intn(len(alphaBlank))]
			}
		}
		p.Name = string(name)

		// Price is random number between .01 and 10.00
		p.UnitPrice = types.USD(rand.Intn(1000 + 1))

		// Frist four letters of code will be the sequence number, to
		// guarantee uniqueness.
		p.Code = fmt.Sprintf("%04d-%s-%s-%s", i, randAlnumQuartet(),
			randAlnumQuartet(), randAlnumQuartet())
		res = append(res, p)
	}
	return res
}

func toUpper(p types.Produce) types.Produce {
	res := p
	res.Code = strings.ToUpper(res.Code)
	newName, ok := types.ValidateAndConvertName(res.Name)
	if ok {
		res.Name = newName
	}
	return res
}

func randAlnumQuartet() string {
	alnum := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	len := len(alnum)
	res := make([]byte, 4)
	for i := 0; i < 4; i++ {
		res[i] = alnum[rand.Intn(len)]
	}
	return string(res)
}

// ResSorter sorts slices of AddResult.  Sort by key, since it is unique.
type produceSorter struct {
	prod []types.Produce
}

// Len is part of sort.Interface.
func (ps produceSorter) Len() int {
	return len(ps.prod)
}

// Swap is part of sort.Interface.
func (ps produceSorter) Swap(i, j int) {
	ps.prod[i], ps.prod[j] = ps.prod[j], ps.prod[i]
}

// Less is part of sort.Interface.
func (ps produceSorter) Less(i, j int) bool {
	return strings.Compare(ps.prod[i].Code, ps.prod[j].Code) < 0
}

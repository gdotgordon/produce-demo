// +build integration

// Run as: go test -tags=integration
package integration

import (
	"bytes"
	"encoding/json"
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

// testTypes are the pre-conditions for the test cases
type testType byte

const (
	valid testType = iota
	dups
	invalid
)

var (
	produceAddr string
	prodClient  *http.Client
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	produceAddr, _ = getAppAddr("8080", "produce-demo_produce-demo_1",
		"producedemo_produce-demo_1")
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

func TestList(t *testing.T) {
	status, items := invokeListAll(t)
	if status != http.StatusOK {
		t.Fatal("list returned unexpcted status", status)
	}
	if len(items) != 0 {
		t.Fatal("list was not empty", items)
	}
}

// Test concurrently adding items, ensure the returned list is correct.
// Then concurrently delete the items, checking the codes and finally
// that the list is empty.
func TestAddListDelete(t *testing.T) {
	for c, v := range []struct {
		ttype          testType // one of the test types defined above
		cnt            int      // number of items to add
		numDup         int      // number of duplicate adds
		numBad         int      // number of bad format adds
		expAddSucc     uint32   // number of successful adds
		expAddBadReq   uint32   // number of bad request adds
		expAddConflict uint32   // number of conflict adds
		listBeforeCnt  int      // list count before deleting
		delNCCnt       uint32   // no content count for deletes
		delNFCnt       uint32   // not found count for deltes
	}{
		{
			ttype:         valid,
			cnt:           25,
			expAddSucc:    25,
			listBeforeCnt: 25,
			delNCCnt:      25,
			delNFCnt:      1,
		},
		{
			ttype:          dups,
			cnt:            25,
			numDup:         2,
			expAddSucc:     23,
			expAddConflict: 2,
			listBeforeCnt:  23,
			delNCCnt:       23,
			delNFCnt:       1,
		},
		{
			ttype:         invalid,
			cnt:           25,
			numBad:        2,
			expAddSucc:    23,
			expAddBadReq:  2,
			listBeforeCnt: 23,
			delNCCnt:      23,
			delNFCnt:      1,
		},
	} {
		invokeReset(t)
		items := createRandomProduce(1, v.cnt-v.numDup-v.numBad)

		// Add the items and wait for them to complete.
		var succCnt uint32
		var badReqCnt uint32
		var conflictCnt uint32
		var wg sync.WaitGroup
		for i := 0; i < len(items)+v.numDup+v.numBad; i++ {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()

				var res int
				if i >= len(items) {
					if v.expAddBadReq > 0 {
						bogusItem := items[i-len(items)]
						bogusItem.Name = "&&&&&&$$$$$$$$$"
						res = invokeAddSingle(t, bogusItem)
					} else if v.expAddConflict > 0 {
						res = invokeAddSingle(t, items[i-len(items)])
					}
				} else {
					res = invokeAddSingle(t, items[i])
				}
				switch res {
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

		if succCnt != v.expAddSucc {
			t.Fatalf("(%d) expected %d success, got %d", c, v.cnt, succCnt)
		}
		if badReqCnt != v.expAddBadReq {
			t.Fatalf("(%d) expected %d bad requests, got %d", c, v.expAddBadReq, badReqCnt)
		}
		if conflictCnt != v.expAddConflict {
			t.Fatalf("(%d) expected %d conflicts, got %d", c, v.expAddConflict, conflictCnt)
		}

		// Compare the two lists, sorintg, and converting the incoming list to
		// canonical.
		status, litems := invokeListAll(t)
		if status != http.StatusOK {
			t.Fatalf("(%d) list returned unexpcted status: %d", c, status)
		}
		if len(items) != int(v.expAddSucc) {
			t.Fatalf("(%d) expected %d list items, got %d", c, v.cnt, len(items))
		}

		// Save the keys for the delete test.
		keys := make([]string, v.cnt)
		for i := range items {
			keys[i] = items[i].Code
			items[i] = toUpper(items[i])
		}
		sort.Sort(produceSorter{items})
		sort.Sort(produceSorter{litems})
		for i, v := range items {
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
		for i := 0; i <= len(items); i++ {
			i := i
			wg2.Add(1)
			go func() {
				defer wg2.Done()

				var res int
				if i == len(items) {
					// nefarious dduplicate delete
					res = invokeDelete(t, keys[0])
				} else {
					res = invokeDelete(t, keys[i])
				}
				switch res {
				case http.StatusNoContent:
					atomic.AddUint32(&ncCnt, 1)
				case http.StatusNotFound:
					atomic.AddUint32(&nfCnt, 1)
				}
			}()
		}
		wg2.Wait()
		if ncCnt != v.delNCCnt {
			t.Fatalf("(%d) expected %d no content, got %d", c, v.cnt, ncCnt)
		}
		if nfCnt != v.delNFCnt {
			t.Fatalf("(%d) expected 1 not found, got %d", c, ncCnt)
		}

		status, items = invokeListAll(t)
		if status != http.StatusOK {
			t.Fatal("list returned unexpcted status", status)
		}
		if len(items) != 0 {
			t.Fatal("list was not empty", items)
		}
	}
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
func invokeAdd(t *testing.T, items types.ProduceAddRequest) (int, types.ProduceAddResponse) {
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		t.Fatal("add error marshaling request", err)
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+produceAddr+"/v1/produce",
		bytes.NewReader(b))
	if err != nil {
		t.Fatal("list all error creating request", err)
	}

	resp, err := prodClient.Do(req)
	if err != nil {
		t.Fatal("list all returned unexpcted error", err)
	}
	defer resp.Body.Close()

	// There will only be a response if the status code is 200 (mixed results)
	var respItems types.ProduceAddResponse
	if resp.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal("list all error reading body", err)
		}
		if err = json.Unmarshal(body, &respItems); err != nil {
			t.Fatal("list all returned unexpcted error", err)
		}
	}
	return resp.StatusCode, respItems
}

// Form of add that takes a single Produce item
func invokeAddSingle(t *testing.T, item types.Produce) int {
	b, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		t.Fatal("add error marshaling request", err)
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+produceAddr+"/v1/produce",
		bytes.NewReader(b))
	if err != nil {
		t.Fatal("list all error creating request", err)
	}

	resp, err := prodClient.Do(req)
	if err != nil {
		t.Fatal("list all returned unexpcted error", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

func invokeDelete(t *testing.T, code string) int {
	req, err := http.NewRequest(http.MethodDelete,
		"http://"+produceAddr+"/v1/produce/"+code, nil)
	if err != nil {
		t.Fatal("delete error creating request", err)
	}

	resp, err := prodClient.Do(req)
	if err != nil {
		t.Fatal("delete returned unexpcted error", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func invokeListAll(t *testing.T) (int, types.ProduceListResponse) {
	req, err := http.NewRequest(http.MethodGet, "http://"+produceAddr+"/v1/produce", nil)
	if err != nil {
		t.Fatal("list all error creating request", err)
	}

	resp, err := prodClient.Do(req)
	if err != nil {
		t.Fatal("list all returned unexpcted error", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("list all error reading body", err)
	}
	var items types.ProduceListResponse

	if resp.StatusCode == http.StatusOK {
		if err = json.Unmarshal(body, &items); err != nil {
			t.Fatal("list all returned unexpcted error", err)
		}
	}
	return resp.StatusCode, items
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

// +build integration

// Run as: go test -tags=integration
package integration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"testing"
)

const (
	app = "produce-demo"
)

var (
	produceAddr string
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	produceAddr, _ = getAppAddr("produce-demo", "8080")
	os.Exit(m.Run())
}

func TestStatus(t *testing.T) {
	fmt.Println("http://" + produceAddr + "/v1/status")
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

func getAppAddr(app, port string) (string, error) {
	res, err := exec.Command("docker", "port", app, port).CombinedOutput()
	if err != nil {
		log.Fatalf("docker-compose error: failed to get exposed port: %v", err)
	}
	return string(res[:len(res)-1]), nil
}
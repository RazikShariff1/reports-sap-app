// Package accountservice resolves masjid (m_id) and halqa (h_id) names by
// calling account-service-app's HTTP API, since that data lives in its own
// primary database and isn't reachable from this app's SQL connections.
package accountservice

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
)

var (
	mu          sync.Mutex
	halqaNames  = map[int]string{}
	masjidNames = map[int]string{}
)

type entityResponse struct {
	Data struct {
		Name string `json:"name"`
	} `json:"data"`
}

// HalqaName returns the name of the halqa with the given id, fetching it
// from account-service-app on first use and caching it for the life of the process.
func HalqaName(id int) (string, error) {
	return cachedName(halqaNames, id, "h")
}

// MasjidName returns the name of the masjid with the given id, fetching it
// from account-service-app on first use and caching it for the life of the process.
func MasjidName(id int) (string, error) {
	return cachedName(masjidNames, id, "m")
}

func cachedName(cache map[int]string, id int, path string) (string, error) {
	mu.Lock()
	name, ok := cache[id]
	mu.Unlock()

	if ok {
		return name, nil
	}

	name, err := fetchName(fmt.Sprintf("%s/%s/%d", os.Getenv("ACCOUNT_SERVICE_URL"), path, id))
	if err != nil {
		return "", err
	}

	mu.Lock()
	cache[id] = name
	mu.Unlock()

	return name, nil
}

func fetchName(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("account-service: %s: status %d", url, resp.StatusCode)
	}

	var out entityResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("account-service: %s: decode: %w", url, err)
	}

	return out.Data.Name, nil
}

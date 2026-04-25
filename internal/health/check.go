package health

import (
	"net/http"
	"time"
)

var client = &http.Client{
	Timeout: 2 * time.Second,
}

func checkProvider(p BackendProvider) {
	for _, b := range p.GetBackends() {
		checkBackend(b)
	}
}

func checkBackend(b Backend) {
	u := b.GetURL()
	if u == nil {
		b.SetAlive(false)
		return
	}

	resp, err := client.Get(u.String() + "/health")
	if err != nil {
		b.SetAlive(false)
		return
	}
	defer resp.Body.Close()

	b.SetAlive(resp.StatusCode >= 200 && resp.StatusCode < 500)
}

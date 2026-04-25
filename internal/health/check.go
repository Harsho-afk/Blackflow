package health

import (
	"net/http"
	"time"
)

var client = &http.Client{
	Timeout: 2 * time.Second,
}

func checkProvider(p BackendProvider) {
	backends := p.GetBackends()

	for _, b := range backends {
		checkBackend(b)
	}
}

func checkBackend(b Backend) {
	url := b.GetURL()

	if url == nil {
		b.SetAlive(false)
		return
	}

	resp, err := client.Get(url.String() + "/health")

	if err != nil {
		b.SetAlive(false)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		b.SetAlive(true)
	} else {
		b.SetAlive(false)
	}
}

package health

import (
	"log/slog"
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

	wasAlive := b.IsAlive()

	resp, err := client.Get(u.String() + "/health")
	if err != nil {
		b.SetAlive(false)
		if wasAlive {
			slog.Warn("backend down", "backend", u.String(), "error", err)
		}
		return
	}
	defer resp.Body.Close()

	alive := resp.StatusCode >= 200 && resp.StatusCode < 500
	b.SetAlive(alive)

	if alive != wasAlive {
		if alive {
			slog.Info("backend up", "backend", u.String())
		} else {
			slog.Warn("backend down", "backend", u.String(), "status", resp.StatusCode)
		}
	}
}

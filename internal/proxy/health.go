package proxy

import (
	"fmt"
	"net/http"
	"time"
)

type HealthChecker struct {
	pool     *Pool
	interval time.Duration
	client   *http.Client
}

func NewHealthChecker(pool *Pool, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		pool:     pool,
		interval: interval,
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

func (h *HealthChecker) Start() {
	ticker := time.NewTicker(h.interval)
	go func() {
		for range ticker.C {
			h.checkAll()
		}
	}()
}

func (h *HealthChecker) checkAll() {
	backends := h.pool.getBackends()
	for _, backend := range backends {
		go h.checkBackend(backend)
	}
}

func (h *HealthChecker) checkBackend(b *Backend) {
	resp, err := h.client.Get(b.GetURL().String() + "/health")
	if err != nil {
		b.SetAlive(false)
		// log.Printf("Server '%s' is not responsive. Error: %v", b.GetURL().String(), err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		b.SetAlive(true)
		// log.Printf("Server '%s' is online.",b.GetURL().String())
	} else {
		// log.Printf("Server '%s' is not responsive. Response code: %v", b.GetURL().String(), resp.StatusCode)
		b.SetAlive(false)
	}
}

func (h *HealthChecker) GetInterval() string {
	return h.interval.String()
}

func (h *HealthChecker) SetInterval(interval time.Duration) error {
	if interval < time.Second {
		return fmt.Errorf("interval %v is too short: minimum is 1s", interval)
	}
	h.interval = interval
	return nil
}

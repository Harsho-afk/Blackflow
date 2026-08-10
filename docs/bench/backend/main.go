// Command bench-backend is a minimal, real HTTP backend used to benchmark
// Blackflow end-to-end. Unlike `python3 -m http.server` (single-threaded,
// no concurrency), this uses Go's net/http server, which handles requests
// on real concurrent goroutines — so proxy overhead measurements aren't
// distorted by the backend itself being the bottleneck.
//
// Usage:
//
//	go build -o bin/bench-backend ./docs/bench/backend
//	./bin/bench-backend -addr :8081 -latency 5ms
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	addr := flag.String("addr", ":8081", "address to listen on")
	latency := flag.Duration("latency", 0, "artificial upstream latency to simulate per request (e.g. 5ms)")
	body := flag.String("body", "ok", "response body to return from non-health routes")
	flag.Parse()

	mux := http.NewServeMux()

	// Health check endpoint — Blackflow's health checker polls this path
	// and expects any 2xx-4xx status to count as alive.
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Catch-all route — simulates real upstream work via -latency, then
	// responds. This is what load generators (wrk2/hey) should hit
	// through the proxy.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if *latency > 0 {
			time.Sleep(*latency)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, *body)
	})

	srv := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("bench-backend listening on %s (latency=%s)", *addr, *latency)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

package balancer

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/Harsho-afk/blackflow/internal/backend"
	"github.com/Harsho-afk/blackflow/internal/pool"
)

// benchPool builds a pool of n backends, all marked alive, for
// benchmarking the hot-path selection logic in isolation from
// network I/O or health-check timing.
func benchPool(n int) *pool.Pool {
	p := pool.New()
	for i := 0; i < n; i++ {
		b, err := backend.New(fmt.Sprintf("http://backend-%d.local:80", i))
		if err != nil {
			panic(err)
		}
		b.SetAlive(true)
		p.AddBackend(b)
	}
	return p
}

func benchmarkBalancer(b *testing.B, algo string, poolSize int) {
	p := benchPool(poolSize)
	bal := NewBalancer(p, algo)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// IPHash needs a varying key to exercise the hash path
		// realistically; the others ignore it.
		_ = bal.NextBackend("192.168.1." + strconv.Itoa(i%255))
	}
}

func BenchmarkRoundRobin_2Backends(b *testing.B)  { benchmarkBalancer(b, "round_robin", 2) }
func BenchmarkRoundRobin_10Backends(b *testing.B) { benchmarkBalancer(b, "round_robin", 10) }
func BenchmarkRoundRobin_100Backends(b *testing.B) { benchmarkBalancer(b, "round_robin", 100) }

func BenchmarkLeastConnection_2Backends(b *testing.B)   { benchmarkBalancer(b, "least_connection", 2) }
func BenchmarkLeastConnection_10Backends(b *testing.B)  { benchmarkBalancer(b, "least_connection", 10) }
func BenchmarkLeastConnection_100Backends(b *testing.B) { benchmarkBalancer(b, "least_connection", 100) }

func BenchmarkIPHash_2Backends(b *testing.B)   { benchmarkBalancer(b, "ip_hash", 2) }
func BenchmarkIPHash_10Backends(b *testing.B)  { benchmarkBalancer(b, "ip_hash", 10) }
func BenchmarkIPHash_100Backends(b *testing.B) { benchmarkBalancer(b, "ip_hash", 100) }

// BenchmarkRoundRobin_Parallel exercises the atomic counter under
// concurrent access, which is the realistic case (many goroutines,
// one per in-flight HTTP request, calling NextBackend concurrently).
func BenchmarkRoundRobin_Parallel(b *testing.B) {
	p := benchPool(10)
	bal := NewBalancer(p, "round_robin")

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = bal.NextBackend("")
		}
	})
}

func BenchmarkLeastConnection_Parallel(b *testing.B) {
	p := benchPool(10)
	bal := NewBalancer(p, "least_connection")

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = bal.NextBackend("")
		}
	})
}

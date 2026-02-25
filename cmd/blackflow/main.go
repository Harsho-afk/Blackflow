package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Harsho-afk/blackflow/config"
	"github.com/Harsho-afk/blackflow/internal/proxy"
)

func main() {
	config, err := config.LoadServerConfig("config/config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	pool := proxy.NewPool()
	pool.LoadBackends(config.Server.Routes)
	balancer := proxy.NewBalancer(pool, config.Server.LoadBalancer.Algorithm)
	proxy, err := proxy.NewProxy(pool, balancer)
	if err != nil {
		log.Fatalf("Failed to create Proxy: %v", err)
	}
	fmt.Println("Endpoitns Mapping:")
	for _, x := range proxy.Pool.GetBackends() {
		prefix := x.Prefix
		target := x.URL.String()
		fmt.Printf("- %s\t\t->\t%s\n", prefix, target)
	}
	fmt.Printf("Load Balancing Algorithm: %s\n", proxy.Balancer.GetAlgorithm())
	fmt.Printf("Running on port: %s\n", config.Server.Port)
	http.ListenAndServe(":"+config.Server.Port, proxy)
}

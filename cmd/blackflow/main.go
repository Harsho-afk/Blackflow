package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/Harsho-afk/blackflow/config"
	"github.com/Harsho-afk/blackflow/internal/proxy"
)

func main() {
	config, err := config.LoadServerConfig("config/config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	pool := proxy.NewPool()
	for _, x := range config.Server.Routes {
		u, err := url.Parse(x)
		if err != nil {
			log.Fatalf("Failed to parse url: %v", err)
		}
		backend := &proxy.Backend{
			URL:   u,
			Alive: true,
		}
		pool.AddBackend(backend)
	}
	p, err := proxy.NewProxy(pool)
	if err != nil {
		log.Fatalf("Failed to create Proxy: %v", err)
	}
	fmt.Println("Endpoitns Mapping:")
	for prefix, target := range config.Server.Routes {
		fmt.Printf("- %s\t\t->\t%s\n", prefix, target)
	}
	fmt.Printf("Load Balancing Algorithm: %s\n", config.Server.LoadBalancer.Algorithm)
	fmt.Printf("Running on port: %s\n", config.Server.Port)
	http.ListenAndServe(":"+config.Server.Port, p)
}

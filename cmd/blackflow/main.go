package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Harsho-afk/blackflow/config"
	"github.com/Harsho-afk/blackflow/internal/proxy"
)

func main() {
	config_path := ""
	args := os.Args[1:]
	if len(args) > 0 {
		config_path = args[0]
	}
	config, config_path := config.LoadServerConfig(config_path)
	log.Printf("Loaded Config from %s", config_path)
	var routes []*proxy.Route
	for prefix, routeConfig := range config.Server.Routes {
		pool := proxy.NewPool()
		pool.LoadBackends(routeConfig.Backends)
		balancer := proxy.NewBalancer(pool, routeConfig.Algorithm)
		route := &proxy.Route{
			Prefix:   prefix,
			Pool:     pool,
			Balancer: balancer,
		}
		routes = append(routes, route)

	}
	proxy, err := proxy.NewProxy(routes)
	if err != nil {
		log.Fatalf("Failed to create proxy: %v", err)
	}
	fmt.Println("Endpoitns Mapping:")
	for _, route := range proxy.Routes {
		prefix := route.Prefix
		fmt.Printf("Prefix: %s\nAlgorithm: %s\n", prefix, route.Balancer.GetAlgorithm())
		for _, backend := range route.Pool.GetBackends() {
			fmt.Printf("\t- %s\n", backend.URL.String())
		}
	}
	fmt.Printf("Running on port: %s\n", config.Server.Port)
	http.ListenAndServe(":"+config.Server.Port, proxy)
}

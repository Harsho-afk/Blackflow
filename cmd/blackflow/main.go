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
	http.ListenAndServe(":"+config.Server.Port, proxy)
}

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	server := &http.Server{
		Addr:    ":" + config.Server.Port,
		Handler: proxy,
	}
	go func() {
		fmt.Printf("Running on port: %s\n", config.Server.Port)
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Listen Error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting Down...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = server.Shutdown(ctx)
	if err != nil {
		log.Printf("Forced shutdown: %v", err)
	} else {
		log.Println("Server shutdown completed")
	}
}

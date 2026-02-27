package main

import (
	"context"
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
	log.Printf("Loaded config from %s", config_path)
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
	log.Println("Endpoitns mapping:")
	for _, route := range proxy.Routes {
		prefix := route.Prefix
		log.Printf("Prefix: %s\nAlgorithm: %s", prefix, route.Balancer.GetAlgorithm())
		for _, backend := range route.Pool.GetBackends() {
			log.Printf("\t- %s", backend.URL.String())
		}
	}
	server := &http.Server{
		Addr:    ":" + config.Server.Port,
		Handler: proxy,
	}
	go func() {
		log.Printf("Running on port: %s", config.Server.Port)
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Listen error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = server.Shutdown(ctx)
	if err != nil {
		log.Printf("Forced shutdown: %v", err)
	} else {
		log.Println("Server shutdown completed")
	}
}

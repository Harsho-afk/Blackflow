package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Harsho-afk/blackflow/config"
	"github.com/Harsho-afk/blackflow/internal/proxy"
)

func main() {
	config, err := config.LoadConfig("config/config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	p, err := proxy.NewProxy(config.Server.Routes)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Endpoitns Mapping:")
	for prefix, target := range config.Server.Routes {
		fmt.Printf("- %s\t\t->\t%s\n", prefix, target)
	}
	fmt.Printf("Load Balancing Algorithm: %s\n",config.Server.LoadBalancer.Algorithm)
	fmt.Printf("Runnong on port: %s\n",config.Server.Port)
	http.ListenAndServe(":"+config.Server.Port, p)
}

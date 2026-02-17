package main

import (
	"log"
	"net/http"

	"github.com/Harsho-afk/blackflow/internal/proxy"
)

func main() {
	routes := map[string]string{
		"/test1": "http://localhost:8081",
		"/test2": "http://localhost:8082",
	}
	p, err := proxy.NewProxy(routes)
	if err != nil {
		log.Fatal(err)
	}

	http.ListenAndServe(":8080", p)
}

package main

import (
	"log"
	"net/http"

	"gateway/internal/api"
)

const addr = ":8080"

func main() {
	log.Printf("Gateway service started on %s", addr)
	log.Fatal(http.ListenAndServe(addr, api.NewRouter()))
}

package main

import (
	"context"
	"log"
	"net/http"

	"gateway/internal/api"
	"ledger"
)

const addr = ":8080"

func main() {
	if err := ledger.Init(context.Background()); err != nil {
		log.Fatalf("Ledger: %v", err)
	}

	log.Printf("Gateway service started on %s", addr)
	log.Fatal(http.ListenAndServe(addr, api.NewRouter()))
}

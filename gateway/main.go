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
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	svc, closeFn, err := ledger.NewService(context.Background())
	if err != nil {
		return err
	}
	defer closeFn()

	log.Printf("Gateway service started on %s", addr)
	return http.ListenAndServe(addr, api.NewRouter(svc))
}

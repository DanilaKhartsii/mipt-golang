package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

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
	timeout, err := requestTimeoutFromEnv()
	if err != nil {
		return err
	}
	svc, closeFn, err := ledger.NewService(context.Background())
	if err != nil {
		return err
	}
	defer closeFn()

	log.Printf("Gateway service started on %s (request timeout: %s)", addr, timeout)
	return http.ListenAndServe(addr, api.NewRouter(svc, timeout))
}

func requestTimeoutFromEnv() (time.Duration, error) {
	value := os.Getenv("REQUEST_TIMEOUT")
	if value == "" {
		return api.DefaultRequestTimeout, nil
	}
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		return 0, fmt.Errorf("REQUEST_TIMEOUT должна быть положительной длительностью, например 2s или 50ms: %q", value)
	}
	return timeout, nil
}

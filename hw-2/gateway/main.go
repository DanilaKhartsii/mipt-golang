package main

import (
	"fmt"
	"log"
	"net/http"
)

const addr = ":8080"

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", ping)

	log.Printf("Gateway service started on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func ping(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "pong")
}

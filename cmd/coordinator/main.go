package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Maverick-sst/OpenCompute/internal/api"
	"github.com/Maverick-sst/OpenCompute/internal/jobs"
)

func main() {
	addr := os.Getenv("OCF_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	store := jobs.NewMemoryStore()
	handler := api.NewServer(store).Handler()
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("OpenCompute coordinator listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

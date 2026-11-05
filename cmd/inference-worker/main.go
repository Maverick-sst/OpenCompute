package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Maverick-sst/OpenCompute/internal/inference"
)

func main() {
	binary := flag.String("llama-server", env("OCF_LLAMA_SERVER", os.ExpandEnv("$HOME/src/llama.cpp/build-opencompute/bin/llama-server")), "Path to llama-server")
	model := flag.String("model", env("OCF_MODEL_PATH", os.ExpandEnv("$HOME/models/opencompute/qwen2.5-0.5b-instruct-q4_k_m.gguf")), "Path to GGUF model")
	nativeHost := flag.String("native-host", "127.0.0.1", "Loopback address for llama-server")
	nativePort := flag.Int("native-port", 8081, "llama-server HTTP port")
	threads := flag.Int("threads", 2, "CPU threads for inference")
	ctxSize := flag.Int("ctx-size", 2048, "Inference context size")
	agentHost := flag.String("host", "127.0.0.1", "Loopback address for supervisor health API")
	agentPort := flag.Int("port", 8090, "Supervisor health API port")
	flag.Parse()

	if !loopback(*agentHost) {
		log.Fatal("supervisor health API must bind to loopback during M1")
	}

	sup := inference.NewSupervisor(inference.Config{
		BinaryPath: *binary,
		ModelPath: *model,
		Host: *nativeHost,
		Port: *nativePort,
		Threads: *threads,
		ContextSize: *ctxSize,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := sup.Start(ctx); err != nil {
		log.Fatalf("start native inference runtime: %v", err)
	}
	log.Printf("native inference runtime ready: %s", sup.Endpoint())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", sup.HealthHandler)
	mux.HandleFunc("GET /readyz", sup.HealthHandler)
	mux.HandleFunc("GET /status", sup.HealthHandler)
	server := &http.Server{
		Addr: net.JoinHostPort(*agentHost, strconv.Itoa(*agentPort)),
		Handler: mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("OpenCompute inference worker health API listening on http://%s", server.Addr)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Printf("shutdown signal received")
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("health API stopped: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	if err := sup.Shutdown(); err != nil {
		log.Printf("native runtime shutdown: %v", err)
	}
	log.Printf("OpenCompute inference worker stopped")
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" { return value }
	return fallback
}

func loopback(host string) bool {
	if host == "localhost" { return true }
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

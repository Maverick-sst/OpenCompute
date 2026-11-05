package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Maverick-sst/OpenCompute/internal/inference"
)

type registration struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type workerAgent struct {
	base, token, name, model, endpoint, localEndpoint string
	cores	int
	memoryMB	int64
	client	*http.Client
}

func main() {
	binary := flag.String("llama-server", env("OCF_LLAMA_SERVER", os.ExpandEnv("$HOME/src/llama.cpp/build-opencompute/bin/llama-server")), "Path to llama-server")
	model := flag.String("model", env("OCF_MODEL_PATH", os.ExpandEnv("$HOME/models/opencompute/qwen2.5-0.5b-instruct-q4_k_m.gguf")), "Path to GGUF model")
	nativeHost := flag.String("native-host", "127.0.0.1", "Loopback address for llama-server")
	nativePort := flag.Int("native-port", 8081, "llama-server HTTP port")
	advertiseEndpoint := flag.String("advertise-endpoint", "", "reachable worker inference endpoint advertised to the coordinator")
	threads := flag.Int("threads", 2, "CPU threads for inference")
	ctxSize := flag.Int("ctx-size", 2048, "Inference context size")
	agentHost := flag.String("host", "127.0.0.1", "Loopback address for supervisor health API")
	agentPort := flag.Int("port", 8090, "Supervisor health API port")
	coordinator := flag.String("coordinator", env("OCF_COORDINATOR_URL", "http://127.0.0.1:8080"), "Coordinator base URL")
	name := flag.String("name", "local-inference-worker", "Inference worker name")
	token := flag.String("token", os.Getenv("OCF_API_TOKEN"), "Coordinator bearer token (or OCF_API_TOKEN)")
	heartbeat := flag.Duration("heartbeat", 5*time.Second, "Coordinator heartbeat interval")
	memory := flag.Int64("memory-mb", 1024, "Memory capacity to advertise in MiB")
	flag.Parse()

	if !loopback(*agentHost) && *token == "" {
		log.Fatal("a coordinator token is required when the worker API binds beyond loopback")
	}
	if *heartbeat < time.Second {
		log.Fatal("heartbeat interval must be at least 1s")
	}

	sup := inference.NewSupervisor(inference.Config{
		BinaryPath:  *binary,
		ModelPath:   *model,
		Host:        *nativeHost,
		Port:        *nativePort,
		Threads:     *threads,
		ContextSize: *ctxSize,
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := sup.Start(ctx); err != nil {
		log.Fatalf("start native inference runtime: %v", err)
	}
	log.Printf("native inference runtime ready: %s", sup.Endpoint())

	agent := &workerAgent{
		base:	strings.TrimRight(*coordinator, "/"),
		token:	*token,
		name:	*name,
		model:	baseName(*model),
		endpoint:	sup.Endpoint(),
		localEndpoint:	sup.Endpoint(),
		cores:	*threads,
		memoryMB:	*memory,
		client:	&http.Client{Timeout: 5 * time.Second},
	}
	if *advertiseEndpoint != "" {
		agent.endpoint = strings.TrimRight(*advertiseEndpoint, "/")
	}
	if agent.base == "" {
		log.Fatal("coordinator URL is required")
	}

	reg, err := agent.register(ctx)
	if err != nil {
		_ = sup.Shutdown()
		log.Fatalf("register inference worker: %v", err)
	}
	log.Printf("registered inference worker %s (%s) with coordinator", reg.Name, reg.ID)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeStatus(w, sup, reg)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		writeStatus(w, sup, reg)
	})
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		writeStatus(w, sup, reg)
	})
	mux.Handle("POST /v1/chat/completions", agent.authorizeWorker(http.HandlerFunc(agent.proxyChatCompletion)))

	server := &http.Server{
		Addr:              net.JoinHostPort(*agentHost, strconv.Itoa(*agentPort)),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("OpenCompute inference worker health API listening on http://%s", server.Addr)
		serverErr <- server.ListenAndServe()
	}()

	heartbeatDone := make(chan struct{})
	go agent.heartbeatLoop(ctx, reg.ID, *heartbeat, heartbeatDone)

	select {
	case <-ctx.Done():
		log.Printf("shutdown signal received")
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("health API stopped: %v", err)
		}
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	<-heartbeatDone

	if err := sup.Shutdown(); err != nil {
		log.Printf("native runtime shutdown: %v", err)
	}
	log.Printf("OpenCompute inference worker stopped")
}


func (a *workerAgent) authorizeWorker(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.token != "" {
			supplied := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(supplied), []byte(a.token)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *workerAgent) proxyChatCompletion(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength > 1<<20 {
		http.Error(w, "request body exceeds 1 MiB", http.StatusRequestEntityTooLarge)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	// The advertised endpoint may be a LAN address. The native process itself
	// remains bound to loopback; use the supervisor's local endpoint here.
	target, err := url.Parse(strings.TrimRight(a.localEndpoint, "/"))
	if err != nil {
		http.Error(w, "invalid local inference endpoint", http.StatusBadGateway)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(req *http.Request) {
		director(req)
		req.Method = http.MethodPost
		req.URL.Path = strings.TrimRight(target.Path, "/") + "/v1/chat/completions"
		req.URL.RawPath = ""
		req.URL.RawQuery = ""
		req.Host = target.Host
		req.Header.Del("Authorization")
		req.Header.Set("Content-Type", "application/json")
	}
	proxy.FlushInterval = -1
	proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, proxyErr error) {
		http.Error(rw, "native inference request failed: "+proxyErr.Error(), http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, r)
}

func (a *workerAgent) register(ctx context.Context) (registration, error) {
	payload := map[string]any{
		"name":               a.name,
		"kind":               "inference",
		"cpu_cores":          a.cores,
		"memory_mb":          a.memoryMB,
		"runtime":            "llama.cpp",
		"model":              a.model,
		"inference_endpoint": a.endpoint,
		"operations":         []string{"chat-completions", "completion"},
	}

	var out registration
	err := a.request(ctx, http.MethodPost, a.base+"/api/v1/workers", payload, &out)
	return out, err
}

func (a *workerAgent) heartbeatLoop(ctx context.Context, id string, interval time.Duration, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.request(ctx, http.MethodPost, a.base+"/api/v1/workers/"+id+"/heartbeat", nil, nil); err != nil {
				log.Printf("coordinator heartbeat failed: %v", err)
			} else {
				log.Printf("coordinator heartbeat acknowledged")
			}
		}
	}
}

func (a *workerAgent) request(ctx context.Context, method, url string, payload, out any) error {
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("coordinator returned %s", resp.Status)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func writeStatus(w http.ResponseWriter, s *inference.Supervisor, r registration) {
	status := s.Status()
	w.Header().Set("Content-Type", "application/json")
	code := http.StatusServiceUnavailable
	if status.State == inference.StateReady {
		code = http.StatusOK
	}
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"runtime": status, "coordinator_worker": r})
}

func baseName(path string) string {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	return parts[len(parts)-1]
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func loopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

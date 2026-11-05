package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Maverick-sst/OpenCompute/internal/jobs"
)

func TestCreateAndGetJob(t *testing.T) {
	handler := NewServer(jobs.NewMemoryStore()).Handler()
	body := []byte(`{"total_items":1000000,"batch_size":250000}`)
	create := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewReader(body))
	createResult := httptest.NewRecorder()
	handler.ServeHTTP(createResult, create)
	if createResult.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", createResult.Code, createResult.Body.String())
	}
	var created jobs.Job
	if err := json.Unmarshal(createResult.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if len(created.Batches) != 4 {
		t.Fatalf("got %d batches, want 4", len(created.Batches))
	}
	get := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+created.ID, nil)
	getResult := httptest.NewRecorder()
	handler.ServeHTTP(getResult, get)
	if getResult.Code != http.StatusOK {
		t.Fatalf("get status = %d, body=%s", getResult.Code, getResult.Body.String())
	}
}
func TestCreateRejectsInvalidJob(t *testing.T) {
	handler := NewServer(jobs.NewMemoryStore()).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBufferString(`{"total_items":0,"batch_size":10}`))
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)
	if result.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", result.Code, http.StatusUnprocessableEntity)
	}
}
func TestRegisterComputeWorkerBackwardCompatible(t *testing.T) {
	handler := NewServer(jobs.NewMemoryStore()).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workers", bytes.NewBufferString(`{"name":"legacy-worker","cpu_cores":4,"memory_mb":2048}`))
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)
	if result.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
	}
	var got jobs.Worker
	if err := json.Unmarshal(result.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Kind != "compute" {
		t.Fatalf("kind=%q want compute", got.Kind)
	}
}
func TestRegisterInferenceWorkerAndListCapabilities(t *testing.T) {
	handler := NewServer(jobs.NewMemoryStore()).Handler()
	payload := `{"name":"qwen-node","kind":"inference","cpu_cores":2,"memory_mb":4096,"runtime":"llama.cpp","model":"qwen2.5-0.5b-instruct-q4_k_m.gguf","inference_endpoint":"http://127.0.0.1:8081","operations":["chat-completions","completion"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workers", bytes.NewBufferString(payload))
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)
	if result.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
	}
	var created jobs.Worker
	if err := json.Unmarshal(result.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Kind != "inference" || created.Runtime != "llama.cpp" || created.Model == "" || created.InferenceEndpoint == "" || len(created.Operations) != 2 {
		t.Fatalf("incomplete capabilities: %+v", created)
	}
	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/workers", nil))
	var workers []jobs.Worker
	if err := json.Unmarshal(list.Body.Bytes(), &workers); err != nil {
		t.Fatal(err)
	}
	if len(workers) != 1 || workers[0].ID != created.ID {
		t.Fatalf("unexpected worker list: %+v", workers)
	}
}
func TestRegisterInferenceWorkerRejectsMissingCapabilities(t *testing.T) {
	handler := NewServer(jobs.NewMemoryStore()).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workers", bytes.NewBufferString(`{"name":"broken","kind":"inference","cpu_cores":2,"memory_mb":1024}`))
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)
	if result.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
	}
}

func TestProxyChatCompletionToRegisteredInferenceWorker(t *testing.T) {
	var gotAuthorization string
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" { t.Errorf("upstream path=%q", r.URL.Path) }
		if r.Method != http.MethodPost { t.Errorf("upstream method=%q", r.Method) }
		gotAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","choices":[{"message":{"content":"hello"}}]}`))
	}))
	defer native.Close()
	store := jobs.NewMemoryStore()
	worker, err := store.RegisterInferenceWorker("test-inference", 2, 2048, "llama.cpp", "test.gguf", native.URL, []string{"chat-completions"})
	if err != nil { t.Fatal(err) }
	handler := NewServerWithToken(store, "coordinator-secret").Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/inference/workers/"+worker.ID+"/chat/completions", strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer coordinator-secret")
	req.Header.Set("Content-Type", "application/json")
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)
	if result.Code != http.StatusOK { t.Fatalf("status=%d body=%s", result.Code, result.Body.String()) }
	if gotAuthorization != "Bearer coordinator-secret" { t.Fatalf("worker gateway did not receive coordinator authorization: %q", gotAuthorization) }
	if !strings.Contains(result.Body.String(), "chatcmpl-test") { t.Fatalf("unexpected upstream response: %s", result.Body.String()) }
}

func TestProxyChatCompletionRejectsMissingAuth(t *testing.T) {
	handler := NewServerWithToken(jobs.NewMemoryStore(), "secret").Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/inference/workers/missing/chat/completions", strings.NewReader(`{}`))
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)
	if result.Code != http.StatusUnauthorized { t.Fatalf("status=%d want %d", result.Code, http.StatusUnauthorized) }
}

func TestProxyChatCompletionRejectsComputeWorker(t *testing.T) {
	store := jobs.NewMemoryStore()
	worker, err := store.RegisterWorker("compute", 2, 2048)
	if err != nil { t.Fatal(err) }
	handler := NewServer(store).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/inference/workers/"+worker.ID+"/chat/completions", strings.NewReader(`{}`))
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)
	if result.Code != http.StatusUnprocessableEntity { t.Fatalf("status=%d body=%s", result.Code, result.Body.String()) }
}

func TestProxyChatCompletionRejectsWorkerWithoutOperation(t *testing.T) {
	store := jobs.NewMemoryStore()
	worker, err := store.RegisterInferenceWorker("completion-only", 2, 2048, "llama.cpp", "test.gguf", "http://127.0.0.1:8081", []string{"completion"})
	if err != nil { t.Fatal(err) }
	handler := NewServer(store).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/inference/workers/"+worker.ID+"/chat/completions", strings.NewReader(`{}`))
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)
	if result.Code != http.StatusUnprocessableEntity { t.Fatalf("status=%d body=%s", result.Code, result.Body.String()) }
}

func TestRoundRobinInferenceRoutingUsesHealthyWorkers(t *testing.T) {
	var firstCount, secondCount atomic.Int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		firstCount.Add(1)
		_, _ = w.Write([]byte(`{"worker":"first"}`))
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		secondCount.Add(1)
		_, _ = w.Write([]byte(`{"worker":"second"}`))
	}))
	defer second.Close()

	store := jobs.NewMemoryStore()
	for _, item := range []struct{name, endpoint string}{{"first", first.URL}, {"second", second.URL}} {
		if _, err := store.RegisterInferenceWorker(item.name, 2, 2048, "llama.cpp", "test.gguf", item.endpoint, []string{"chat-completions"}); err != nil {
			t.Fatal(err)
		}
	}
	handler := NewServer(store).Handler()
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/inference/chat/completions", strings.NewReader(`{"messages":[]}`))
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, req)
		if result.Code != http.StatusOK {
			t.Fatalf("request %d status=%d body=%s", i, result.Code, result.Body.String())
		}
	}
	if firstCount.Load() != 2 || secondCount.Load() != 2 {
		t.Fatalf("round-robin counts first=%d second=%d, want 2 each", firstCount.Load(), secondCount.Load())
	}
}

func TestRoundRobinInferenceRoutingReturnsUnavailableWithoutWorkers(t *testing.T) {
	handler := NewServer(jobs.NewMemoryStore()).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/inference/chat/completions", strings.NewReader(`{}`))
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)
	if result.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want %d body=%s", result.Code, http.StatusServiceUnavailable, result.Body.String())
	}
}

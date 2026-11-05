package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkerGatewayRequiresCoordinatorToken(t *testing.T) {
	agent := &workerAgent{token: "shared-secret"}
	handler := agent.authorizeWorker(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)
	if result.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want %d", result.Code, http.StatusUnauthorized)
	}
}

func TestWorkerGatewayForwardsToLoopbackRuntimeWithoutAuthHeader(t *testing.T) {
	var upstreamAuthorization string
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path=%q", r.URL.Path)
		}
		upstreamAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer native.Close()

	agent := &workerAgent{token: "shared-secret", localEndpoint: native.URL}
	handler := agent.authorizeWorker(http.HandlerFunc(agent.proxyChatCompletion))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"messages":[]}`))
	req.Header.Set("Authorization", "Bearer shared-secret")
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)
	if result.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
	}
	if upstreamAuthorization != "" {
		t.Fatalf("worker credential leaked to native runtime: %q", upstreamAuthorization)
	}
}

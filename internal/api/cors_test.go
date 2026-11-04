package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Maverick-sst/OpenCompute/internal/jobs"
)

func TestCORSAllowsConfiguredOriginWithDashboardPath(t *testing.T) {
	t.Setenv("OCF_ALLOWED_ORIGINS", " https://maverick-sst.github.io/OpenCompute/ , http://localhost:5173/ ")
	handler := NewServer(jobs.NewMemoryStore()).Handler()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://maverick-sst.github.io")
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)

	if result.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", result.Code, http.StatusOK, result.Body.String())
	}
	if got := result.Header().Get("Access-Control-Allow-Origin"); got != "https://maverick-sst.github.io" {
		t.Fatalf("allow-origin = %q, want canonical dashboard origin", got)
	}
	if got := result.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("Vary = %q, want Origin", got)
	}
}

func TestCORSPreflightAllowsAuthorizationHeader(t *testing.T) {
	t.Setenv("OCF_ALLOWED_ORIGINS", "https://maverick-sst.github.io")
	handler := NewServerWithToken(jobs.NewMemoryStore(), "test-token").Handler()

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/jobs", nil)
	req.Header.Set("Origin", "https://maverick-sst.github.io")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)

	if result.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", result.Code, http.StatusNoContent)
	}
	if got := result.Header().Get("Access-Control-Allow-Headers"); got != "Authorization, Content-Type" {
		t.Fatalf("allow-headers = %q", got)
	}
	if got := result.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Fatalf("allow-methods = %q", got)
	}
}

func TestCORSRejectsUnconfiguredOrigin(t *testing.T) {
	t.Setenv("OCF_ALLOWED_ORIGINS", "https://maverick-sst.github.io")
	handler := NewServer(jobs.NewMemoryStore()).Handler()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://attacker.example")
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)

	if result.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", result.Code, http.StatusForbidden)
	}
	if got := result.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected allow-origin header %q", got)
	}
}

func TestCORSDefaultsToDeployedDashboardAndLocalDevelopment(t *testing.T) {
	t.Setenv("OCF_ALLOWED_ORIGINS", "")
	handler := NewServer(jobs.NewMemoryStore()).Handler()

	for _, origin := range []string{"https://maverick-sst.github.io", "http://localhost:5173"} {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Header.Set("Origin", origin)
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, req)
		if result.Code != http.StatusOK {
			t.Fatalf("origin %q: status = %d, want %d", origin, result.Code, http.StatusOK)
		}
		if got := result.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Fatalf("origin %q: allow-origin = %q", origin, got)
		}
	}
}

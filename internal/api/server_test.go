package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

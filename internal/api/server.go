package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Maverick-sst/OpenCompute/internal/jobs"
)

type Server struct {
	store jobs.Store
}

func NewServer(store jobs.Store) *Server {
	return &Server{store: store}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("POST /api/v1/jobs", s.createJob)
	mux.HandleFunc("GET /api/v1/jobs/{id}", s.getJob)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "ocf-coordinator"})
}

type createJobRequest struct {
	TotalItems int64 `json:"total_items"`
	BatchSize  int64 `json:"batch_size"`
}

func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req createJobRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request")
		return
	}
	if req.TotalItems <= 0 || req.BatchSize <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "total_items and batch_size must be greater than zero")
		return
	}
	job, err := s.store.Create(req.TotalItems, req.BatchSize)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	job, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, jobs.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not retrieve job")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

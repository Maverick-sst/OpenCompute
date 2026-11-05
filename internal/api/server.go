package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Maverick-sst/OpenCompute/internal/dataset"
	"github.com/Maverick-sst/OpenCompute/internal/jobs"
)

const leaseDuration = 30 * time.Second

type Server struct { store jobs.Store; token string; datasets *dataset.Manager; datasetsRoot string }

func NewServer(store jobs.Store) *Server { return NewServerWithToken(store, os.Getenv("OCF_API_TOKEN")) }
func NewServerWithToken(store jobs.Store, token string) *Server {
	root:=os.Getenv("OCF_DATASET_ROOT");if root==""{root="./data/datasets"}
	manager,err:=dataset.NewManager(root,256<<20,1<<20,4<<20);if err!=nil{panic(err)}
	return &Server{store:store,token:token,datasets:manager,datasetsRoot:filepath.Clean(root)}
}
func (s *Server) Handler() http.Handler {
	mux:=http.NewServeMux()
	mux.HandleFunc("GET /healthz",s.health)
	mux.Handle("POST /api/v1/datasets",s.authorize(http.HandlerFunc(s.uploadDataset)))
	mux.Handle("GET /api/v1/datasets/{id}",s.authorize(http.HandlerFunc(s.getDataset)))
	mux.Handle("GET /api/v1/datasets/{id}/partitions",s.authorize(http.HandlerFunc(s.listDatasetPartitions)))
	mux.Handle("GET /api/v1/partitions/{id}/data",s.authorize(http.HandlerFunc(s.downloadPartition)))
	mux.Handle("POST /api/v1/jobs",s.authorize(http.HandlerFunc(s.createJob)))
	mux.Handle("GET /api/v1/jobs",s.authorize(http.HandlerFunc(s.listJobs)))
	mux.Handle("GET /api/v1/jobs/{id}",s.authorize(http.HandlerFunc(s.getJob)))
	mux.Handle("GET /api/v1/workers",s.authorize(http.HandlerFunc(s.listWorkers)))
	mux.Handle("POST /api/v1/workers",s.authorize(http.HandlerFunc(s.registerWorker)))
	mux.Handle("POST /api/v1/workers/{id}/heartbeat",s.authorize(http.HandlerFunc(s.heartbeat)))
	mux.Handle("POST /api/v1/workers/{id}/leases",s.authorize(http.HandlerFunc(s.leaseNext)))
	mux.Handle("POST /api/v1/leases/{id}/complete",s.authorize(http.HandlerFunc(s.completeLease)))
	return s.cors(mux)
}
func (s *Server) listJobs(w http.ResponseWriter,_ *http.Request){writeJSON(w,200,s.store.ListJobs())}
func (s *Server) listWorkers(w http.ResponseWriter,_ *http.Request){writeJSON(w,200,s.store.ListWorkers())}
func (s *Server) health(w http.ResponseWriter,_ *http.Request){writeJSON(w,200,map[string]string{"status":"ok","service":"ocf-coordinator"})}
func (s *Server) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if s.token!=""{supplied:=strings.TrimPrefix(r.Header.Get("Authorization"),"Bearer ");if subtle.ConstantTimeCompare([]byte(supplied),[]byte(s.token))!=1{writeError(w,401,"missing or invalid bearer token");return}};next.ServeHTTP(w,r)})
}
type createJobRequest struct { TotalItems int64 `json:"total_items"`; BatchSize int64 `json:"batch_size"` }
func (s *Server) createJob(w http.ResponseWriter,r *http.Request){var req createJobRequest;if !decodeJSON(w,r,&req){return};if req.TotalItems<=0||req.BatchSize<=0{writeError(w,422,"total_items and batch_size must be greater than zero");return};job,err:=s.store.Create(req.TotalItems,req.BatchSize);if err!=nil{writeError(w,422,err.Error());return};writeJSON(w,201,job)}
func (s *Server) getJob(w http.ResponseWriter,r *http.Request){job,err:=s.store.Get(strings.TrimSpace(r.PathValue("id")));if err!=nil{writeStoreError(w,err);return};writeJSON(w,200,job)}
type registerWorkerRequest struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	CPUCores int `json:"cpu_cores"`
	MemoryMB int64 `json:"memory_mb"`
	Runtime string `json:"runtime"`
	Model string `json:"model"`
	InferenceEndpoint string `json:"inference_endpoint"`
	Operations []string `json:"operations"`
}
func (s *Server) registerWorker(w http.ResponseWriter,r *http.Request){
	var req registerWorkerRequest;if !decodeJSON(w,r,&req){return}
	var worker jobs.Worker;var err error
	switch req.Kind {
	case "", "compute": worker,err=s.store.RegisterWorker(strings.TrimSpace(req.Name),req.CPUCores,req.MemoryMB)
	case "inference": worker,err=s.store.RegisterInferenceWorker(strings.TrimSpace(req.Name),req.CPUCores,req.MemoryMB,strings.TrimSpace(req.Runtime),strings.TrimSpace(req.Model),strings.TrimSpace(req.InferenceEndpoint),req.Operations)
	default: writeError(w,422,"kind must be compute or inference");return
	}
	if err!=nil{writeError(w,422,err.Error());return};writeJSON(w,201,worker)
}
func (s *Server) heartbeat(w http.ResponseWriter,r *http.Request){worker,err:=s.store.Heartbeat(r.PathValue("id"));if err!=nil{writeStoreError(w,err);return};writeJSON(w,200,worker)}
func (s *Server) leaseNext(w http.ResponseWriter,r *http.Request){lease,err:=s.store.LeaseNext(r.PathValue("id"),leaseDuration);if errors.Is(err,jobs.ErrLeaseNotFound){w.WriteHeader(http.StatusNoContent);return};if err!=nil{writeStoreError(w,err);return};writeJSON(w,200,lease)}
type completeLeaseRequest struct { WorkerID string `json:"worker_id"`; FencingToken uint64 `json:"fencing_token"`; Processed int64 `json:"processed"`; Checksum uint64 `json:"checksum"` }
func (s *Server) completeLease(w http.ResponseWriter,r *http.Request){var req completeLeaseRequest;if !decodeJSON(w,r,&req){return};job,err:=s.store.CompleteLease(req.WorkerID,r.PathValue("id"),req.FencingToken,jobs.BatchResult{Processed:req.Processed,Checksum:req.Checksum});if err!=nil{writeStoreError(w,err);return};writeJSON(w,200,job)}
func decodeJSON(w http.ResponseWriter,r *http.Request,dst any)bool{r.Body=http.MaxBytesReader(w,r.Body,1<<20);d:=json.NewDecoder(r.Body);d.DisallowUnknownFields();if err:=d.Decode(dst);err!=nil{writeError(w,400,"invalid JSON request");return false};return true}
func writeStoreError(w http.ResponseWriter,err error){switch{case errors.Is(err,jobs.ErrNotFound),errors.Is(err,jobs.ErrLeaseNotFound):writeError(w,404,err.Error());case errors.Is(err,jobs.ErrLeaseOwnership):writeError(w,403,err.Error());case errors.Is(err,jobs.ErrLeaseExpired):writeError(w,409,err.Error());default:writeError(w,422,err.Error())}}
func writeJSON(w http.ResponseWriter,status int,value any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(value)}
func writeError(w http.ResponseWriter,status int,message string){writeJSON(w,status,map[string]string{"error":message})}

package api

import (
 "encoding/json"
 "errors"
 "net/http"
 "strings"
 "time"
 "github.com/Maverick-sst/OpenCompute/internal/jobs"
)

const leaseDuration=30*time.Second
type Server struct{store jobs.Store}
func NewServer(store jobs.Store)*Server{return &Server{store:store}}
func(s *Server)Handler()http.Handler{
 mux:=http.NewServeMux()
 mux.HandleFunc("GET /healthz",s.health)
 mux.HandleFunc("POST /api/v1/jobs",s.createJob)
 mux.HandleFunc("GET /api/v1/jobs/{id}",s.getJob)
 mux.HandleFunc("POST /api/v1/workers",s.registerWorker)
 mux.HandleFunc("POST /api/v1/workers/{id}/heartbeat",s.heartbeat)
 mux.HandleFunc("POST /api/v1/workers/{id}/leases",s.leaseNext)
 mux.HandleFunc("POST /api/v1/leases/{id}/complete",s.completeLease)
 return mux
}
func(s *Server)health(w http.ResponseWriter,_ *http.Request){writeJSON(w,200,map[string]string{"status":"ok","service":"ocf-coordinator"})}
type createJobRequest struct{TotalItems int64 `json:"total_items"`;BatchSize int64 `json:"batch_size"`}
func(s *Server)createJob(w http.ResponseWriter,r *http.Request){
 var req createJobRequest;if !decodeJSON(w,r,&req){return}
 if req.TotalItems<=0||req.BatchSize<=0{writeError(w,422,"total_items and batch_size must be greater than zero");return}
 job,err:=s.store.Create(req.TotalItems,req.BatchSize);if err!=nil{writeError(w,422,err.Error());return};writeJSON(w,201,job)
}
func(s *Server)getJob(w http.ResponseWriter,r *http.Request){job,err:=s.store.Get(strings.TrimSpace(r.PathValue("id")));if err!=nil{writeStoreError(w,err);return};writeJSON(w,200,job)}
type registerWorkerRequest struct{Name string `json:"name"`;CPUCores int `json:"cpu_cores"`;MemoryMB int64 `json:"memory_mb"`}
func(s *Server)registerWorker(w http.ResponseWriter,r *http.Request){
 var req registerWorkerRequest;if !decodeJSON(w,r,&req){return}
 worker,err:=s.store.RegisterWorker(strings.TrimSpace(req.Name),req.CPUCores,req.MemoryMB);if err!=nil{writeError(w,422,err.Error());return};writeJSON(w,201,worker)
}
func(s *Server)heartbeat(w http.ResponseWriter,r *http.Request){worker,err:=s.store.Heartbeat(r.PathValue("id"));if err!=nil{writeStoreError(w,err);return};writeJSON(w,200,worker)}
func(s *Server)leaseNext(w http.ResponseWriter,r *http.Request){
 lease,err:=s.store.LeaseNext(r.PathValue("id"),leaseDuration)
 if errors.Is(err,jobs.ErrLeaseNotFound){w.WriteHeader(http.StatusNoContent);return}
 if err!=nil{writeStoreError(w,err);return};writeJSON(w,200,lease)
}
type completeLeaseRequest struct{WorkerID string `json:"worker_id"`;FencingToken uint64 `json:"fencing_token"`;Processed int64 `json:"processed"`;Checksum uint64 `json:"checksum"`}
func(s *Server)completeLease(w http.ResponseWriter,r *http.Request){
 var req completeLeaseRequest;if !decodeJSON(w,r,&req){return}
 job,err:=s.store.CompleteLease(req.WorkerID,r.PathValue("id"),req.FencingToken,jobs.BatchResult{Processed:req.Processed,Checksum:req.Checksum})
 if err!=nil{writeStoreError(w,err);return};writeJSON(w,200,job)
}
func decodeJSON(w http.ResponseWriter,r *http.Request,dst any)bool{
 r.Body=http.MaxBytesReader(w,r.Body,1<<20);d:=json.NewDecoder(r.Body);d.DisallowUnknownFields()
 if err:=d.Decode(dst);err!=nil{writeError(w,400,"invalid JSON request");return false};return true
}
func writeStoreError(w http.ResponseWriter,err error){
 switch{
 case errors.Is(err,jobs.ErrNotFound),errors.Is(err,jobs.ErrLeaseNotFound):writeError(w,404,err.Error())
 case errors.Is(err,jobs.ErrLeaseOwnership):writeError(w,403,err.Error())
 case errors.Is(err,jobs.ErrLeaseExpired):writeError(w,409,err.Error())
 default:writeError(w,422,err.Error())
 }
}
func writeJSON(w http.ResponseWriter,status int,value any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(value)}
func writeError(w http.ResponseWriter,status int,message string){writeJSON(w,status,map[string]string{"error":message})}

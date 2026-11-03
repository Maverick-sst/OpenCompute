package api

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "testing"
 "github.com/Maverick-sst/OpenCompute/internal/jobs"
)

func TestListEndpoints(t *testing.T){
 store:=jobs.NewMemoryStore()
 _,err:=store.Create(12,4);if err!=nil{t.Fatal(err)}
 _,err=store.RegisterWorker("worker",2,1024);if err!=nil{t.Fatal(err)}
 h:=NewServer(store).Handler()
 jobsResult:=httptest.NewRecorder()
 h.ServeHTTP(jobsResult,httptest.NewRequest(http.MethodGet,"/api/v1/jobs",nil))
 if jobsResult.Code!=http.StatusOK{t.Fatalf("jobs status=%d",jobsResult.Code)}
 var jobList []jobs.Job
 if err:=json.Unmarshal(jobsResult.Body.Bytes(),&jobList);err!=nil{t.Fatal(err)}
 if len(jobList)!=1{t.Fatalf("jobs count=%d want 1",len(jobList))}
 workersResult:=httptest.NewRecorder()
 h.ServeHTTP(workersResult,httptest.NewRequest(http.MethodGet,"/api/v1/workers",nil))
 if workersResult.Code!=http.StatusOK{t.Fatalf("workers status=%d",workersResult.Code)}
 var workerList []jobs.Worker
 if err:=json.Unmarshal(workersResult.Body.Bytes(),&workerList);err!=nil{t.Fatal(err)}
 if len(workerList)!=1{t.Fatalf("workers count=%d want 1",len(workerList))}
}

func TestCORSAllowsConfiguredDevelopmentOrigin(t *testing.T){
 h:=NewServer(jobs.NewMemoryStore()).Handler()
 req:=httptest.NewRequest(http.MethodOptions,"/api/v1/jobs",nil)
 req.Header.Set("Origin","http://localhost:5173")
 req.Header.Set("Access-Control-Request-Method","POST")
 result:=httptest.NewRecorder()
 h.ServeHTTP(result,req)
 if result.Code!=http.StatusNoContent{t.Fatalf("preflight status=%d",result.Code)}
 if result.Header().Get("Access-Control-Allow-Origin")!="http://localhost:5173"{t.Fatal("expected CORS allow-origin header")}
}

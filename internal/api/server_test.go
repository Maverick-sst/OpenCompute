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
	handler:=NewServer(jobs.NewMemoryStore()).Handler()
	body:=[]byte(`{"total_items":1000000,"batch_size":250000}`)
	create:=httptest.NewRequest(http.MethodPost,"/api/v1/jobs",bytes.NewReader(body));createResult:=httptest.NewRecorder();handler.ServeHTTP(createResult,create)
	if createResult.Code!=http.StatusCreated{t.Fatalf("create status = %d, body=%s",createResult.Code,createResult.Body.String())}
	var created jobs.Job;if err:=json.Unmarshal(createResult.Body.Bytes(),&created);err!=nil{t.Fatal(err)}
	if len(created.Batches)!=4{t.Fatalf("got %d batches, want 4",len(created.Batches))}
	get:=httptest.NewRequest(http.MethodGet,"/api/v1/jobs/"+created.ID,nil);getResult:=httptest.NewRecorder();handler.ServeHTTP(getResult,get)
	if getResult.Code!=http.StatusOK{t.Fatalf("get status = %d, body=%s",getResult.Code,getResult.Body.String())}
}
func TestCreateRejectsInvalidJob(t *testing.T){
	handler:=NewServer(jobs.NewMemoryStore()).Handler();req:=httptest.NewRequest(http.MethodPost,"/api/v1/jobs",bytes.NewBufferString(`{"total_items":0,"batch_size":10}`));result:=httptest.NewRecorder();handler.ServeHTTP(result,req)
	if result.Code!=http.StatusUnprocessableEntity{t.Fatalf("status = %d, want %d",result.Code,http.StatusUnprocessableEntity)}
}
func TestRegisterComputeWorkerBackwardCompatible(t *testing.T){
	handler:=NewServer(jobs.NewMemoryStore()).Handler()
	req:=httptest.NewRequest(http.MethodPost,"/api/v1/workers",bytes.NewBufferString(`{"name":"legacy-worker","cpu_cores":4,"memory_mb":2048}`));result:=httptest.NewRecorder();handler.ServeHTTP(result,req)
	if result.Code!=http.StatusCreated{t.Fatalf("status=%d body=%s",result.Code,result.Body.String())}
	var got jobs.Worker;if err:=json.Unmarshal(result.Body.Bytes(),&got);err!=nil{t.Fatal(err)}
	if got.Kind!="compute"{t.Fatalf("kind=%q want compute",got.Kind)}
}
func TestRegisterInferenceWorkerAndListCapabilities(t *testing.T){
	handler:=NewServer(jobs.NewMemoryStore()).Handler()
	payload:=`{"name":"qwen-node","kind":"inference","cpu_cores":2,"memory_mb":4096,"runtime":"llama.cpp","model":"qwen2.5-0.5b-instruct-q4_k_m.gguf","inference_endpoint":"http://127.0.0.1:8081","operations":["chat-completions","completion"]}`
	req:=httptest.NewRequest(http.MethodPost,"/api/v1/workers",bytes.NewBufferString(payload));result:=httptest.NewRecorder();handler.ServeHTTP(result,req)
	if result.Code!=http.StatusCreated{t.Fatalf("status=%d body=%s",result.Code,result.Body.String())}
	var created jobs.Worker;if err:=json.Unmarshal(result.Body.Bytes(),&created);err!=nil{t.Fatal(err)}
	if created.Kind!="inference"||created.Runtime!="llama.cpp"||created.Model==""||created.InferenceEndpoint==""||len(created.Operations)!=2{t.Fatalf("incomplete capabilities: %+v",created)}
	list:=httptest.NewRecorder();handler.ServeHTTP(list,httptest.NewRequest(http.MethodGet,"/api/v1/workers",nil))
	var workers []jobs.Worker;if err:=json.Unmarshal(list.Body.Bytes(),&workers);err!=nil{t.Fatal(err)}
	if len(workers)!=1||workers[0].ID!=created.ID{t.Fatalf("unexpected worker list: %+v",workers)}
}
func TestRegisterInferenceWorkerRejectsMissingCapabilities(t *testing.T){
	handler:=NewServer(jobs.NewMemoryStore()).Handler()
	req:=httptest.NewRequest(http.MethodPost,"/api/v1/workers",bytes.NewBufferString(`{"name":"broken","kind":"inference","cpu_cores":2,"memory_mb":1024}`));result:=httptest.NewRecorder();handler.ServeHTTP(result,req)
	if result.Code!=http.StatusUnprocessableEntity{t.Fatalf("status=%d body=%s",result.Code,result.Body.String())}
}

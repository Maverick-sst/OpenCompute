package main

import (
 "bytes"
 "context"
 "encoding/json"
 "flag"
 "fmt"
 "log"
 "net/http"
 "runtime"
 "time"
)

type worker struct{base,id string;client *http.Client}
type lease struct{ID string `json:"id"`;JobID string `json:"job_id"`;Batch struct{Index int `json:"index"`;Start int64 `json:"start"`;End int64 `json:"end"`} `json:"batch"`;FencingToken uint64 `json:"fencing_token"`}
func main(){
 base:=flag.String("coordinator","http://127.0.0.1:8080","Coordinator URL")
 name:=flag.String("name","local-worker","Worker name")
 flag.Parse()
 var registered struct{ID string `json:"id"`;Name string `json:"name"`}
 err:=request(http.MethodPost,*base+"/api/v1/workers",map[string]any{"name":*name,"cpu_cores":runtime.NumCPU(),"memory_mb":1024},&registered)
 if err!=nil{log.Fatal(err)}
 w:=worker{base:*base,id:registered.ID,client:&http.Client{Timeout:10*time.Second}}
 log.Printf("worker registered: %s (%s)",registered.Name,registered.ID)
 for{
  if err:=w.heartbeat();err!=nil{log.Printf("heartbeat: %v",err);time.Sleep(time.Second);continue}
  l,err:=w.next()
  if err==errNoWork{time.Sleep(time.Second);continue}
  if err!=nil{log.Printf("lease: %v",err);time.Sleep(time.Second);continue}
  processed,checksum:=run(l.Batch.Start,l.Batch.End)
  payload:=map[string]any{"worker_id":w.id,"fencing_token":l.FencingToken,"processed":processed,"checksum":checksum}
  if err:=w.do(http.MethodPost,"/api/v1/leases/"+l.ID+"/complete",payload,nil);err!=nil{log.Printf("completion: %v",err)}else{log.Printf("completed job=%s batch=%d processed=%d",l.JobID,l.Batch.Index,processed)}
 }
}
var errNoWork=fmt.Errorf("no work available")
func(w *worker)heartbeat()error{return w.do(http.MethodPost,"/api/v1/workers/"+w.id+"/heartbeat",nil,nil)}
func(w *worker)next()(lease,error){
 req,_:=http.NewRequestWithContext(context.Background(),http.MethodPost,w.base+"/api/v1/workers/"+w.id+"/leases",nil)
 resp,err:=w.client.Do(req);if err!=nil{return lease{},err};defer resp.Body.Close()
 if resp.StatusCode==http.StatusNoContent{return lease{},errNoWork}
 if resp.StatusCode!=http.StatusOK{return lease{},fmt.Errorf("coordinator returned %s",resp.Status)}
 var l lease;err=json.NewDecoder(resp.Body).Decode(&l);return l,err
}
func(w *worker)do(method,path string,payload,out any)error{
 var body *bytes.Reader
 if payload==nil{body=bytes.NewReader(nil)}else{b,err:=json.Marshal(payload);if err!=nil{return err};body=bytes.NewReader(b)}
 req,err:=http.NewRequestWithContext(context.Background(),method,w.base+path,body);if err!=nil{return err}
 if payload!=nil{req.Header.Set("Content-Type","application/json")}
 resp,err:=w.client.Do(req);if err!=nil{return err};defer resp.Body.Close()
 if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("coordinator returned %s",resp.Status)}
 if out!=nil{return json.NewDecoder(resp.Body).Decode(out)};return nil
}
func request(method,url string,payload,out any)error{
 b,err:=json.Marshal(payload);if err!=nil{return err}
 req,err:=http.NewRequest(method,url,bytes.NewReader(b));if err!=nil{return err};req.Header.Set("Content-Type","application/json")
 resp,err:=http.DefaultClient.Do(req);if err!=nil{return err};defer resp.Body.Close()
 if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("coordinator returned %s",resp.Status)}
 return json.NewDecoder(resp.Body).Decode(out)
}
// run is a reproducible CPU workload over synthetic item indices.
func run(start,end int64)(int64,uint64){
 var checksum uint64
 for i:=start;i<end;i++{
  x:=uint64(i)+0x9e3779b97f4a7c15
  x=(x^(x>>30))*0xbf58476d1ce4e5b9
  x=(x^(x>>27))*0x94d049bb133111eb
  checksum^=x^(x>>31)
 }
 return end-start,checksum
}

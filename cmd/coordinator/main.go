package main

import (
 "log"
 "net"
 "net/http"
 "os"
 "time"

 "github.com/Maverick-sst/OpenCompute/internal/api"
 "github.com/Maverick-sst/OpenCompute/internal/jobs"
)

func main(){
 addr:=os.Getenv("OCF_ADDR");if addr==""{addr="127.0.0.1:8080"}
 token:=os.Getenv("OCF_API_TOKEN")
 host,_,err:=net.SplitHostPort(addr)
 if err!=nil{log.Fatalf("invalid OCF_ADDR: %v",err)}
 ip:=net.ParseIP(host)
 if token==""&&(host==""||host=="0.0.0.0"||host=="::"||(ip!=nil&&!ip.IsLoopback())||(ip==nil&&host!="localhost")){
  log.Fatal("OCF_API_TOKEN is required when binding beyond localhost")
 }
 statePath:=os.Getenv("OCF_STATE_PATH");if statePath==""{statePath="./data/ocf-state.json"}
 store,err:=jobs.OpenFileStore(statePath);if err!=nil{log.Fatalf("open coordinator state: %v",err)}
 handler:=api.NewServerWithToken(store,token).Handler()
 server:=&http.Server{Addr:addr,Handler:handler,ReadHeaderTimeout:5*time.Second}
 log.Printf("OpenCompute coordinator listening on %s (state: %s)",addr,statePath)
 if err:=server.ListenAndServe();err!=nil&&err!=http.ErrServerClosed{log.Fatal(err)}
}

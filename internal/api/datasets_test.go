package api

import (
 "bytes"
 "mime/multipart"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "testing"

 "github.com/Maverick-sst/OpenCompute/internal/jobs"
)

func TestUploadDatasetAndListPartitions(t *testing.T){
 root:=filepath.Join(t.TempDir(),"datasets")
 t.Setenv("OCF_DATASET_ROOT",root)
 h:=NewServerWithToken(jobs.NewMemoryStore(),"secret").Handler()
 var body bytes.Buffer
 mw:=multipart.NewWriter(&body)
 part,err:=mw.CreateFormFile("file","access.jsonl");if err!=nil{t.Fatal(err)}
 _,_=part.Write([]byte("{\"level\":\"INFO\"}\n{\"level\":\"ERROR\"}"))
 _=mw.Close()
 req:=httptest.NewRequest(http.MethodPost,"/api/v1/datasets",&body);req.Header.Set("Content-Type",mw.FormDataContentType());req.Header.Set("Authorization","Bearer secret")
 res:=httptest.NewRecorder();h.ServeHTTP(res,req)
 if res.Code!=http.StatusCreated{t.Fatalf("upload status=%d body=%s",res.Code,res.Body.String())}
 if _,err:=os.Stat(root);err!=nil{t.Fatal(err)}
}
func TestDatasetUploadRequiresToken(t *testing.T){
 t.Setenv("OCF_DATASET_ROOT",filepath.Join(t.TempDir(),"datasets"))
 h:=NewServerWithToken(jobs.NewMemoryStore(),"secret").Handler()
 req:=httptest.NewRequest(http.MethodPost,"/api/v1/datasets",nil)
 res:=httptest.NewRecorder();h.ServeHTTP(res,req)
 if res.Code!=http.StatusUnauthorized{t.Fatalf("status=%d",res.Code)}
}

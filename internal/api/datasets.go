package api

import (
 "io"
 "net/http"
 "os"
 "strconv"
 "strings"

 "github.com/Maverick-sst/OpenCompute/internal/dataset"
)

func (s *Server) uploadDataset(w http.ResponseWriter,r *http.Request){
 max:=int64(256<<20)
 r.Body=http.MaxBytesReader(w,r.Body,max+(1<<20))
 if err:=r.ParseMultipartForm(8<<20);err!=nil{writeError(w,http.StatusBadRequest,"invalid multipart upload or dataset exceeds size limit");return}
 file,header,err:=r.FormFile("file");if err!=nil{writeError(w,400,"multipart field 'file' is required");return};defer file.Close()
 d,parts,err:=s.datasets.Save(header.Filename,file)
 if err!=nil{status:=http.StatusUnprocessableEntity;if strings.Contains(err.Error(),"maximum size"){status=http.StatusRequestEntityTooLarge};writeError(w,status,err.Error());return}
 writeJSON(w,http.StatusCreated,map[string]any{"dataset":d,"partitions":parts})
}
func(s *Server)getDataset(w http.ResponseWriter,r *http.Request){
 d,err:=s.datasets.Get(strings.TrimSpace(r.PathValue("id")));if err!=nil{if err==dataset.ErrNotFound{writeError(w,404,"dataset not found")}else{writeError(w,500,"unable to read dataset metadata")};return}
 writeJSON(w,200,d)
}
func(s *Server)listDatasetPartitions(w http.ResponseWriter,r *http.Request){
 p,err:=s.datasets.Partitions(strings.TrimSpace(r.PathValue("id")));if err!=nil{writeError(w,404,"dataset or manifest not found");return}
 writeJSON(w,200,map[string]any{"partitions":p})
}
func(s *Server)downloadPartition(w http.ResponseWriter,r *http.Request){
 id:=strings.TrimSpace(r.PathValue("id"))
 entries,err:=os.ReadDir(s.datasetsRoot);if err!=nil{writeError(w,404,"partition not found");return}
 for _,entry:=range entries{
  if !entry.IsDir(){continue}
  parts,e:=s.datasets.Partitions(entry.Name());if e!=nil{continue}
  for _,p:=range parts{if p.ID==id{
   f,e:=s.datasets.OpenPartition(entry.Name(),p.StartOffset,p.EndOffset);if e!=nil{writeError(w,500,"unable to open partition");return};defer f.Close()
   w.Header().Set("Content-Type","application/x-ndjson")
   w.Header().Set("Content-Length",strconv.FormatInt(p.EndOffset-p.StartOffset,10))
   _,_=io.Copy(w,f);return
  }}
 }
 writeError(w,404,"partition not found")
}

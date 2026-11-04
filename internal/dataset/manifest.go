package dataset

import (
 "bufio"
 "encoding/json"
 "fmt"
 "io"
 "os"
 "path/filepath"
)

type randomReader struct{}
func(randomReader)Read(p []byte)(int,error){return os.ReadFile("/dev/urandom")Read(p)}
func buildManifest(path,id string,target,maxRecord int64)([]Partition,int64,error){
 f,err:=os.Open(path);if err!=nil{return nil,0,err};defer f.Close()
 st,err:=f.Stat();if err!=nil{return nil,0,err}
 r:=bufio.NewReaderSize(f,64*1024)
 var parts []Partition
 var offset,start,records,partRecords int64
 index:=0
 for {
  line,e:=r.ReadBytes('\n')
  if len(line)>0 {
   if int64(len(line))>maxRecord+1{return nil,0,fmt.Errorf("JSONL record exceeds %d bytes",maxRecord)}
   raw:=line
   if raw[len(raw)-1]=='\n'{raw=raw[:len(raw)-1]}
   if len(raw)>0 && raw[len(raw)-1]=='\r'{raw=raw[:len(raw)-1]}
   if len(raw)>0 {
    // Upload validates JSON syntax and requires each nonblank record to be an object.
    var value map[string]json.RawMessage
    if err:=json.Unmarshal(raw,&value);err!=nil || value==nil {return nil,0,fmt.Errorf("invalid JSON object near byte offset %d",offset)}
    records++;partRecords++
   }
   offset+=int64(len(line))
   if offset-start>=target && partRecords>0 {
    parts=append(parts,Partition{ID:fmt.Sprintf("%s-%06d",id,index),DatasetID:id,Index:index,StartOffset:start,EndOffset:offset,RecordCount:partRecords})
    index++;start=offset;partRecords=0
   }
  }
  if e==io.EOF{break};if e!=nil{return nil,0,e}
 }
 if start<st.Size(){parts=append(parts,Partition{ID:fmt.Sprintf("%s-%06d",id,index),DatasetID:id,Index:index,StartOffset:start,EndOffset:st.Size(),RecordCount:partRecords})}
 return parts,records,nil
}
func writeJSON(path string,v any)error{b,e:=json.MarshalIndent(v,"","  ");if e!=nil{return e};return os.WriteFile(path,b,0600)}
func readJSON(path string,v any)error{b,e:=os.ReadFile(filepath.Clean(path));if e!=nil{return e};return json.Unmarshal(b,v)}

package dataset

import (
 "crypto/sha256"
 "encoding/hex"
 "errors"
 "fmt"
 "io"
 "os"
 "path/filepath"
 "strings"
 "time"
)

var ErrNotFound = errors.New("dataset not found")

type Manager struct {
 root string
 maxBytes int64
 maxRecord int64
 partitionBytes int64
}

func NewManager(root string, maxBytes, maxRecord, partitionBytes int64) (*Manager,error) {
 if strings.TrimSpace(root)=="" { return nil,fmt.Errorf("dataset root is required") }
 if maxBytes<=0 { maxBytes=256<<20 }; if maxRecord<=0 { maxRecord=1<<20 }; if partitionBytes<=0 { partitionBytes=4<<20 }
 if err:=os.MkdirAll(root,0700); err!=nil { return nil,err }
 return &Manager{root:root,maxBytes:maxBytes,maxRecord:maxRecord,partitionBytes:partitionBytes},nil
}

func (m *Manager) DatasetDir(id string) string { return filepath.Join(m.root,id) }

func (m *Manager) Save(filename string, src io.Reader) (Dataset, []Partition, error) {
 id,err:=newID(); if err!=nil{return Dataset{},nil,err}
 tmp,err:=os.CreateTemp(m.root,".upload-*.tmp"); if err!=nil{return Dataset{},nil,err}
 tmpName:=tmp.Name(); defer os.Remove(tmpName)
 h:=sha256.New()
 limited:=io.LimitReader(src,m.maxBytes+1)
 n,err:=io.Copy(io.MultiWriter(tmp,h),limited)
 if err!=nil {tmp.Close();return Dataset{},nil,err}
 if n>m.maxBytes {tmp.Close();return Dataset{},nil,fmt.Errorf("dataset exceeds maximum size of %d bytes",m.maxBytes)}
 if err=tmp.Sync(); err!=nil {tmp.Close();return Dataset{},nil,err}
 if err=tmp.Close(); err!=nil{return Dataset{},nil,err}
 parts,count,err:=buildManifest(tmpName,id,m.partitionBytes,m.maxRecord)
 if err!=nil{return Dataset{},nil,err}
 now:=time.Now().UTC()
 d:=Dataset{ID:id,Filename:safeFilename(filename),Format:"jsonl",SizeBytes:n,RecordCount:count,Checksum:hex.EncodeToString(h.Sum(nil)),Status:"ready",CreatedAt:now}
 dir:=m.DatasetDir(id)
 if err=os.Mkdir(dir,0700); err!=nil{return Dataset{},nil,err}
 if err=os.Rename(tmpName,filepath.Join(dir,"source.jsonl")); err!=nil{_ = os.RemoveAll(dir);return Dataset{},nil,err}
 if err=writeJSON(filepath.Join(dir,"metadata.json"),d); err!=nil{_ = os.RemoveAll(dir);return Dataset{},nil,err}
 if err=writeJSON(filepath.Join(dir,"manifest.json"),parts); err!=nil{_ = os.RemoveAll(dir);return Dataset{},nil,err}
 return d,parts,nil
}

func (m *Manager) Get(id string) (Dataset,error) {
 if !validID(id){return Dataset{},ErrNotFound}
 var d Dataset
 if err:=readJSON(filepath.Join(m.DatasetDir(id),"metadata.json"),&d); err!=nil {if os.IsNotExist(err){return Dataset{},ErrNotFound};return Dataset{},err}
 return d,nil
}
func (m *Manager) Partitions(id string) ([]Partition,error) {
 if _,err:=m.Get(id); err!=nil{return nil,err}
 var p []Partition
 if err:=readJSON(filepath.Join(m.DatasetDir(id),"manifest.json"),&p); err!=nil{return nil,err}
 return p,nil
}
func (m *Manager) OpenSource(id string) (*os.File,error) {
 if _,err:=m.Get(id); err!=nil{return nil,err}
 return os.Open(filepath.Join(m.DatasetDir(id),"source.jsonl"))
}
func (m *Manager) OpenPartition(id string,start,end int64) (io.ReadCloser,error) {
 d,err:=m.Get(id); if err!=nil{return nil,err}
 if start<0 || end<start || end>d.SizeBytes{return nil,fmt.Errorf("invalid partition range")}
 f,err:=os.Open(filepath.Join(m.DatasetDir(id),"source.jsonl")); if err!=nil{return nil,err}
 if _,err=f.Seek(start,io.SeekStart);err!=nil{f.Close();return nil,err}
 return &sectionFile{File:f,remaining:end-start},nil
}
type sectionFile struct{*os.File;remaining int64}
func(s *sectionFile)Read(p []byte)(int,error){if s.remaining<=0{return 0,io.EOF};if int64(len(p))>s.remaining{p=p[:s.remaining]};n,e:=s.File.Read(p);s.remaining-=int64(n);return n,e}

func safeFilename(v string) string { v=filepath.Base(strings.TrimSpace(v)); if v=="."||v==string(filepath.Separator)||v=="" {return "dataset.jsonl"}; return v }
func validID(v string) bool { if len(v)!=32{return false}; for _,c:=range v {if !(c>='0'&&c<='9'||c>='a'&&c<='f'){return false}};return true }

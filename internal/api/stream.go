package api

import (
 "io"
 "net/http"
)
func copyHTTP(w http.ResponseWriter,r io.Reader)(int64,error){return io.Copy(w,r)}

package api

import (
 "net/http"
 "net/http/httptest"
 "testing"
 "github.com/Maverick-sst/OpenCompute/internal/jobs"
)

func TestBearerTokenProtectsAPIButNotHealth(t *testing.T){
 h:=NewServerWithToken(jobs.NewMemoryStore(),"secret-token").Handler()
 health:=httptest.NewRecorder()
 h.ServeHTTP(health,httptest.NewRequest(http.MethodGet,"/healthz",nil))
 if health.Code!=http.StatusOK{t.Fatalf("health status=%d",health.Code)}
 unauth:=httptest.NewRecorder()
 h.ServeHTTP(unauth,httptest.NewRequest(http.MethodPost,"/api/v1/jobs",nil))
 if unauth.Code!=http.StatusUnauthorized{t.Fatalf("unauthenticated status=%d",unauth.Code)}
 req:=httptest.NewRequest(http.MethodPost,"/api/v1/jobs",nil)
 req.Header.Set("Authorization","Bearer secret-token")
 authed:=httptest.NewRecorder()
 h.ServeHTTP(authed,req)
 if authed.Code!=http.StatusBadRequest{t.Fatalf("authenticated malformed request status=%d",authed.Code)}
}

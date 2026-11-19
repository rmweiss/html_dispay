package main

import (
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
)

func TestWebUI(t *testing.T) {
 mux:=http.NewServeMux()
 registerWebUI(mux)
 mux.HandleFunc("/api/player",func(w http.ResponseWriter,r *http.Request){w.WriteHeader(http.StatusNoContent)})
 for _,tc:=range []struct{method,path string;code int}{
  {"GET","/",200},{"HEAD","/",200},{"POST","/",405},{"GET","/missing",404},{"GET","/api/player",204},
 }{
  w:=httptest.NewRecorder()
  mux.ServeHTTP(w,httptest.NewRequest(tc.method,tc.path,nil))
  if w.Code!=tc.code{t.Errorf("%s %s: got %d want %d",tc.method,tc.path,w.Code,tc.code)}
  if tc.method=="GET"&&tc.path=="/"{
   if !strings.Contains(w.Body.String(),"/api/player/")||!strings.Contains(w.Body.String(),"Video URL"){t.Error("missing playback form")}
   if w.Header().Get("Cache-Control")!="no-store"{t.Error("missing cache policy")}
  }
  if tc.method=="HEAD"&&w.Body.Len()!=0{t.Error("unexpected HEAD body")}
 }
}

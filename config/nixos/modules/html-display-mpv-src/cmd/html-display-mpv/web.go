package main

import (
 _ "embed"
 "net/http"
)

//go:embed index.html
var indexHTML []byte

func registerWebUI(mux *http.ServeMux) {
 mux.HandleFunc("/",func(w http.ResponseWriter,r *http.Request){
  if r.URL.Path!="/" {http.NotFound(w,r);return}
  if r.Method!=http.MethodGet && r.Method!=http.MethodHead {
   w.Header().Set("Allow","GET, HEAD")
   http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
   return
  }
  w.Header().Set("Content-Type","text/html; charset=utf-8")
  w.Header().Set("Cache-Control","no-store")
  w.Header().Set("X-Content-Type-Options","nosniff")
  w.Header().Set("Referrer-Policy","no-referrer")
  w.Header().Set("Content-Security-Policy","default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
  if r.Method==http.MethodGet {_,_=w.Write(indexHTML)}
 })
}

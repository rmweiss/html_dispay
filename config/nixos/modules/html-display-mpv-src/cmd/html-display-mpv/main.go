package main

import (
 "context"
 "encoding/json"
 "errors"
 "flag"
 "fmt"
 "log"
 "net/http"
 "os"
 "os/signal"
 "strconv"
 "strings"
 "syscall"
 "time"

 "example.com/html-display-mpv/internal/mqtt"
 "example.com/html-display-mpv/internal/player"
)
func main(){
 listen:=flag.String("http-listen","127.0.0.1:8081","HTTP listening address")
 broker:=flag.String("mqtt-broker","tcp://192.168.10.12:1883","MQTT broker URL, empty disables")
 prefix:=flag.String("mqtt-prefix","html-display/mpv","MQTT topic prefix")
 mpvBin:=flag.String("mpv-bin","html-display-mpv","MPV audio Takeover wrapper executable")
 options:=flag.String("mpv-options",`["--fullscreen","--hwdec=vaapi","--vo=gpu","--gpu-api=opengl","--ytdl=yes","--ytdl-format=bestvideo[height<=1080]+bestaudio/best[height<=1080]/best","--cache=yes","--cache-secs=60","--demuxer-max-bytes=128MiB","--cache-pause=yes","--cache-pause-initial=no","--cache-pause-wait=10","--cache-on-disk=no"]`,"JSON array of MPV options")
 roots:=flag.String("local-roots",`["/home/display/Videos"]`,"JSON array of allowed local media directories")
 flag.Parse()
 var args,allowed []string
 if err:=json.Unmarshal([]byte(*options),&args);err!=nil{log.Fatal("mpv-options: ",err)}
 if err:=json.Unmarshal([]byte(*roots),&allowed);err!=nil{log.Fatal("local-roots: ",err)}
 for _,arg:=range args{if !strings.HasPrefix(arg,"-"){log.Fatal("mpv option must begin with -: ",arg)}}
 if *broker!=""&&!strings.HasPrefix(*broker,"tcp://"){log.Fatal("mqtt-broker must be tcp://... or empty")}
 updates:=make(chan struct{},1)
 p:=player.New(*mpvBin,args,allowed,func(player.State){select{case updates<-struct{}{}:default:}})
 var mc *mqtt.Client
 if *broker!=""{
  mc=&mqtt.Client{Address:strings.TrimPrefix(*broker,"tcp://"),Prefix:*prefix}
  publish:=func(){
   state:=p.Snapshot()
   mc.State("playback",state.State)
   mc.State("source",state.Source)
   mc.State("error",state.Error)
   mc.State("position",strconv.FormatFloat(state.Position,'f',1,64))
   mc.State("duration",strconv.FormatFloat(state.Duration,'f',1,64))
   // Empty retained values indicate that MPV has no cache metric available.
   percent:=""
   if state.BufferPercent!=nil{percent=strconv.FormatFloat(*state.BufferPercent,'f',0,64)}
   seconds:=""
   if state.BufferSeconds!=nil{seconds=strconv.FormatFloat(*state.BufferSeconds,'f',1,64)}
   mc.State("buffer",percent)
   mc.State("buffer_seconds",seconds)
  }
  mc.OnConnect=publish
  mc.OnCommand=func(name,payload string){
   var err error
   switch name{
   case "source":_,err=p.Play(payload)
   case "playback":
    switch strings.ToLower(strings.TrimSpace(payload)){
    case "stop":err=p.Stop()
    case "pause":err=p.Pause()
    case "resume","play":err=p.Resume()
    default:err=fmt.Errorf("invalid playback command")
    }
   case "seek":
    var t float64;t,err=strconv.ParseFloat(strings.TrimSpace(payload),64);if err==nil{err=p.Seek(t)}
   default:return
   }
   if err!=nil{log.Printf("MQTT command %s: %v",name,err)}
  }
  go mc.Run()
  go func(){for range updates{publish()}}()
 }
 mux:=http.NewServeMux()
 respond:=func(w http.ResponseWriter,status int,v any){
  w.Header().Set("Content-Type","application/json");w.Header().Set("Cache-Control","no-store")
  w.WriteHeader(status);_ =json.NewEncoder(w).Encode(v)
 }
 mux.HandleFunc("/health",func(w http.ResponseWriter,r *http.Request){
  if r.Method!="GET"{http.Error(w,"method not allowed",405);return}
  respond(w,200,map[string]string{"status":"ok"})
 })
 mux.HandleFunc("/api/player",func(w http.ResponseWriter,r *http.Request){
  if r.Method!="GET"{http.Error(w,"method not allowed",405);return}
  respond(w,200,p.Snapshot())
 })
 decode:=func(w http.ResponseWriter,r *http.Request,dest any)error{
  r.Body=http.MaxBytesReader(w,r.Body,8192)
  decoder:=json.NewDecoder(r.Body);decoder.DisallowUnknownFields()
  return decoder.Decode(dest)
 }
 action:=func(w http.ResponseWriter,r *http.Request,fn func()error){
  if r.Method!="POST"{http.Error(w,"method not allowed",405);return}
  err:=fn();if err!=nil{
   code:=400;if errors.Is(err,player.ErrBusy){code=409}
   respond(w,code,map[string]string{"error":err.Error()});return
  }
  respond(w,200,p.Snapshot())
 }
 mux.HandleFunc("/api/player/play",func(w http.ResponseWriter,r *http.Request){
  action(w,r,func()error{
   var body struct{Source string `json:"source"`}
   if err:=decode(w,r,&body);err!=nil{return err}
   _,err:=p.Play(body.Source);return err
  })
 })
 mux.HandleFunc("/api/player/stop",func(w http.ResponseWriter,r *http.Request){action(w,r,p.Stop)})
 mux.HandleFunc("/api/player/pause",func(w http.ResponseWriter,r *http.Request){action(w,r,p.Pause)})
 mux.HandleFunc("/api/player/resume",func(w http.ResponseWriter,r *http.Request){action(w,r,p.Resume)})
 mux.HandleFunc("/api/player/seek",func(w http.ResponseWriter,r *http.Request){
  action(w,r,func()error{
   var body struct{Position float64 `json:"position"`}
   if err:=decode(w,r,&body);err!=nil{return err}
   return p.Seek(body.Position)
  })
 })
 server:=&http.Server{Addr:*listen,Handler:mux,ReadHeaderTimeout:5*time.Second}
 go func(){log.Printf("MPV HTTP listening on %s",*listen);if err:=server.ListenAndServe();err!=nil&&!errors.Is(err,http.ErrServerClosed){log.Fatal(err)}}()
 sig:=make(chan os.Signal,1);signal.Notify(sig,syscall.SIGINT,syscall.SIGTERM);<-sig
 log.Print("shutting down MPV controller")
 ctx,cancel:=context.WithTimeout(context.Background(),4*time.Second);defer cancel()
 _=server.Shutdown(ctx)
 p.Close()
 if mc!=nil{mc.Close()}
}

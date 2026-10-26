package main

import (
 "bufio"
 "encoding/json"
 "example.com/html-display-tv/internal/control"
 "flag"
 "fmt"
 "net"
 "os"
 "strings"
 "time"
)

func main() {
 sock := flag.String("socket", "/run/html-display-tv/control.sock", "Unix socket")
 flag.Parse()
 args := flag.Args()
 if len(args) < 1 {
  fmt.Fprintln(os.Stderr, "usage: tvctl [--socket PATH] status | power on|off|get | volume get|set N | backlight get|set N | mute get | input get|set HDMI1..HDMI4")
  os.Exit(2)
 }
 req := control.Request{Command:args[0], Action:"get"}
 if len(args)>1 {
  switch strings.ToLower(args[1]) {
  case "get":
  case "on", "off":
   req.Action="set"
   req.Value,_=json.Marshal(strings.ToUpper(args[1]))
  case "set":
   if len(args)<3 { fmt.Fprintln(os.Stderr, "missing value"); os.Exit(2) }
   req.Action="set"
   if req.Command=="input" { req.Value,_=json.Marshal(strings.ToUpper(args[2])) } else { req.Value=[]byte(args[2]) }
  default:
   fmt.Fprintln(os.Stderr, "invalid action")
   os.Exit(2)
  }
 }
 conn,e:=net.DialTimeout("unix",*sock,3*time.Second)
 if e!=nil { fmt.Fprintln(os.Stderr,e); os.Exit(1) }
 defer conn.Close()
 conn.SetDeadline(time.Now().Add(35*time.Second))
 if e=json.NewEncoder(conn).Encode(req); e!=nil { fmt.Fprintln(os.Stderr,e); os.Exit(1) }
 var reply control.Reply
 if e=json.NewDecoder(bufio.NewReader(conn)).Decode(&reply);e!=nil {fmt.Fprintln(os.Stderr,e);os.Exit(1)}
 if !reply.OK {fmt.Fprintln(os.Stderr,reply.Error);os.Exit(1)}
 out,_:=json.MarshalIndent(reply,"","  ")
 fmt.Println(string(out))
}

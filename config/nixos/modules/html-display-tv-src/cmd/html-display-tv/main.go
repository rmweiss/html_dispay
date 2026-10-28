package main

import (
 "bufio"
 "encoding/json"
 "example.com/html-display-tv/internal/control"
 "example.com/html-display-tv/internal/exlink"
 "example.com/html-display-tv/internal/mqtt"
 "flag"
 "fmt"
 "io"
 "log"
 "net"
 "os"
 "os/signal"
 "path/filepath"
 "strings"
 "syscall"
 "time"
)

func main() {
 device := flag.String("device", "/dev/serial/by-id/usb-Prolific_Technology_Inc._USB-Serial_Controller_D-if00-port0", "serial device")
 sock := flag.String("socket", "/run/html-display-tv/control.sock", "Unix socket")
 broker := flag.String("mqtt-broker", "tcp://192.168.10.12:1883", "MQTT URI; empty disables MQTT")
 prefix := flag.String("mqtt-prefix", "html-display/tv", "MQTT topic root")
 osdStartup := flag.String("osd-startup", "off", "Command result OSD on daemon startup: off, on, keep")
 poll := flag.Duration("poll", 15*time.Second, "polling interval")
 flag.Parse()
 port, e := exlink.Open(*device)
 if e != nil { log.Fatal(e) }
 defer port.Close()
 c := &control.Controller{Port: port}
 policy := strings.ToLower(strings.TrimSpace(*osdStartup))
 if policy != "keep" && policy != "on" && policy != "off" { log.Fatalf("invalid -osd-startup %q (expected keep/on/off)", policy) }
 if policy != "keep" {
   v,_ := json.Marshal(strings.ToUpper(policy))
   r := c.Handle(control.Request{Command:"osd", Action:"set", Value:v})
   if !r.OK { log.Printf("warning: startup OSD %s failed: %s", policy, r.Error) } else { log.Printf("startup command result OSD: %s (ACK)", policy) }
 }
 var mc *mqtt.Client
 if *broker != "" {
  if !strings.HasPrefix(*broker, "tcp://") { log.Fatal("broker must start tcp://") }
  mc = &mqtt.Client{Address: strings.TrimPrefix(*broker, "tcp://"), Prefix: *prefix}
  c.Publish = func(k string, v any) { mc.Publish(k, fmt.Sprint(v)) }
  mc.OnCommand = func(k, payload string) {
   value := json.RawMessage([]byte(payload))
   if k == "power" || k == "input" || k == "osd" { value, _ = json.Marshal(strings.ToUpper(strings.TrimSpace(payload))) }
   if k != "power" && k != "volume" && k != "backlight" && k != "input" && k != "osd" { return }
   r := c.Handle(control.Request{Command:k, Action:"set", Value:value})
   if !r.OK { log.Printf("MQTT %s: %s", k, r.Error) }
  }
  mc.OnConnect = func() {
  c.Poll()
  r := c.Handle(control.Request{Command:"osd",Action:"get"})
  if r.OK && r.Value != "UNKNOWN" { mc.Publish("osd",fmt.Sprint(r.Value)) }
 }
  go mc.Run()
  defer mc.Close()
 }
 if e = os.MkdirAll(filepath.Dir(*sock), 0755); e != nil { log.Fatal(e) }
 os.Remove(*sock)
 ln, e := net.Listen("unix", *sock)
 if e != nil { log.Fatal(e) }
 defer os.Remove(*sock)
 defer ln.Close()
 if e = os.Chmod(*sock, 0660); e != nil { log.Fatal(e) }
 if *poll > 0 {
  go func() {
   ticker := time.NewTicker(*poll)
   defer ticker.Stop()
   for range ticker.C { c.Poll() }
  }()
 }
 sig := make(chan os.Signal, 1)
 signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
 go func() { <-sig; ln.Close() }()
 log.Printf("listening on %s using %s", *sock, *device)
 for {
  conn, e := ln.Accept()
  if e != nil { return }
  go func() {
   defer conn.Close()
   conn.SetDeadline(time.Now().Add(35*time.Second))
   line, e := bufio.NewReader(io.LimitReader(conn, 65536)).ReadBytes('\n')
   if e != nil { return }
   var req control.Request
   if e = json.Unmarshal(line, &req); e != nil {
    json.NewEncoder(conn).Encode(control.Reply{Error:e.Error()})
    return
   }
   json.NewEncoder(conn).Encode(c.Handle(req))
  }()
 }
}

package player

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"
)

func waitState(t *testing.T,c *Controller,want string)State {
 t.Helper()
 deadline:=time.Now().Add(4*time.Second)
 for time.Now().Before(deadline){
  state:=c.Snapshot()
  if state.State==want{return state}
  time.Sleep(20*time.Millisecond)
 }
 t.Fatalf("state did not reach %s, got %+v",want,c.Snapshot())
 return State{}
}
func TestFailedProcessKeepsSourceAndError(t *testing.T){
 dir:=t.TempDir()
 fake:=filepath.Join(dir,"mpv")
 if err:=os.WriteFile(fake,[]byte("#!/bin/sh\nexit 7\n"),0700);err!=nil{t.Fatal(err)}
 c:=New(fake,nil,nil,nil)
 source:="https://example.com/broken.mp4"
 if _,err:=c.Play(source);err!=nil{t.Fatal(err)}
 state:=waitState(t,c,"error")
 if state.Source!=source{t.Fatalf("lost failed source: %q",state.Source)}
 if !strings.Contains(state.Error,"exit status 7"){t.Fatalf("lost exit reason: %q",state.Error)}
}
func TestMissingIPCProducesTimeout(t *testing.T){
 dir:=t.TempDir()
 fake:=filepath.Join(dir,"mpv")
 if err:=os.WriteFile(fake,[]byte("#!/bin/sh\nsleep 20\n"),0700);err!=nil{t.Fatal(err)}
 c:=New(fake,nil,nil,nil)
 c.startupTimeout=80*time.Millisecond
 if _,err:=c.Play("https://example.com/video.mp4");err!=nil{t.Fatal(err)}
 defer c.Close()
 state:=waitState(t,c,"error")
 if !strings.Contains(state.Error,"did not become ready"){t.Fatalf("lost IPC timeout: %q",state.Error)}
 if state.Source!="https://example.com/video.mp4"{t.Fatalf("lost source: %+v",state)}
}
func TestNormalStoppedProcessClearsSource(t *testing.T){
 dir:=t.TempDir()
 fake:=filepath.Join(dir,"mpv")
 if err:=os.WriteFile(fake,[]byte("#!/bin/sh\nexec sleep 20\n"),0700);err!=nil{t.Fatal(err)}
 c:=New(fake,nil,nil,nil)
 if _,err:=c.Play("https://example.com/video.mp4");err!=nil{t.Fatal(err)}
 if err:=c.Stop();err!=nil{t.Fatal(err)}
 state:=waitState(t,c,"idle")
 if state.Source!=""||state.Error!=""{t.Fatalf("stopped player not cleared: %+v",state)}
}

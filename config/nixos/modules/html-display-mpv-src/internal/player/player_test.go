package player

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
)
func TestAllowedLocalMedia(t *testing.T){
 root:=t.TempDir()
 dir:=filepath.Join(root,"videos")
 if err:=os.Mkdir(dir,0700);err!=nil{t.Fatal(err)}
 file:=filepath.Join(dir,"clip.mp4")
 if err:=os.WriteFile(file,[]byte("test"),0600);err!=nil{t.Fatal(err)}
 c:=New("mpv",nil,[]string{dir},nil)
 got,err:=c.validateSource(file)
 if err!=nil||got!=file{t.Fatalf("got %q, %v",got,err)}
 if _,err:=c.validateSource(filepath.Join(root,"outside.mp4"));err==nil{t.Fatal("path outside root accepted")}
 link:=filepath.Join(dir,"link")
 if err:=os.Symlink(filepath.Join(root,"outside.mp4"),link);err!=nil{t.Fatal(err)}
 if _,err:=c.validateSource(link);err==nil{t.Fatal("escaped symlink accepted")}
}
func TestSourceSchemes(t *testing.T){
 c:=New("mpv",nil,nil,nil)
 for _,source:=range []string{"https://www.youtube.com/watch?v=abc","http://example.com/video.mp4"}{
  if _,err:=c.validateSource(source);err!=nil{t.Errorf("valid %s: %v",source,err)}
 }
 for _,source:=range []string{"","relative.mp4","file:///etc/passwd","ftp://example.org/video.mp4","https://user:pass@example.org/video.mp4","https:///missing-host","--network-option"}{
  if _,err:=c.validateSource(source);err==nil{t.Errorf("accepted %q",source)}
 }
}
func TestBusyError(t *testing.T){
 if !strings.Contains(ErrBusy.Error(),"already"){t.Fatal("unexpected busy message")}
}

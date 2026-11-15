package player

import (
 "bufio"
 "encoding/json"
 "errors"
 "fmt"
 "math"
 "net"
 "net/url"
 "os"
 "os/exec"
 "path/filepath"
 "strconv"
 "strings"
 "sync"
 "syscall"
 "time"
)

type State struct {
 State string `json:"state"`
 Source string `json:"source"`
 Position float64 `json:"position"`
 Duration float64 `json:"duration"`
 BufferPercent *float64 `json:"buffer_percent"`
 BufferSeconds *float64 `json:"buffer_seconds"`
 Error string `json:"error,omitempty"`
}
type Controller struct {
 mu sync.Mutex
 state State
 cmd *exec.Cmd
 socket string
 generation uint64
 binary string
 args, roots []string
 onChange func(State)
}
func New(binary string,args,roots []string,notify func(State)) *Controller {
 return &Controller{binary:binary,args:append([]string(nil),args...),roots:roots,onChange:notify,state:State{State:"idle"}}
}
func (c *Controller) Snapshot() State {c.mu.Lock();defer c.mu.Unlock();return c.state}
func (c *Controller) publishLocked(){if c.onChange!=nil {state:=c.state;go c.onChange(state)}}
func (c *Controller) validateSource(source string)(string,error){
 source=strings.TrimSpace(source)
 if source==""||len(source)>4096{return "",fmt.Errorf("invalid or empty source")}
 if strings.HasPrefix(source,"https://")||strings.HasPrefix(source,"http://"){
  u,err:=url.Parse(source)
  if err!=nil{return "",err}
  if u.User!=nil||u.Hostname()==""{return "",fmt.Errorf("URL must have a host and no embedded credentials")}
  return source,nil
 }
 if !filepath.IsAbs(source){return "",fmt.Errorf("local source must be an absolute path")}
 real,err:=filepath.EvalSymlinks(source);if err!=nil{return "",err}
 st,err:=os.Stat(real);if err!=nil{return "",err};if !st.Mode().IsRegular(){return "",fmt.Errorf("source is not a regular file")}
 for _,root:=range c.roots{
  resolved,err:=filepath.EvalSymlinks(root);if err!=nil{continue}
  rel,err:=filepath.Rel(resolved,real)
  if err==nil&&rel!=".."&&!strings.HasPrefix(rel,".."+string(filepath.Separator)){return real,nil}
 }
 return "",fmt.Errorf("local file is outside allowed roots")
}
var ErrBusy=errors.New("player is already active")
func (c *Controller) Play(source string) (State,error) {
 source,err:=c.validateSource(source);if err!=nil{return c.Snapshot(),err}
 c.mu.Lock()
 if c.cmd!=nil{state:=c.state;c.mu.Unlock();return state,ErrBusy}
 dir,err:=os.MkdirTemp("","html-display-mpv-")
 if err!=nil{c.mu.Unlock();return State{},err}
 socket:=filepath.Join(dir,"ipc.sock")
 args:=append([]string{},c.args...)
 args=append(args,"--no-config","--keep-open=no","--idle=no","--input-ipc-server="+socket,"--",source)
 cmd:=exec.Command(c.binary,args...)
 cmd.SysProcAttr=&syscall.SysProcAttr{Setpgid:true}
 cmd.Stdout=os.Stdout;cmd.Stderr=os.Stderr
 cmd.Env=displayEnv()
 if err=cmd.Start();err!=nil{os.RemoveAll(dir);c.mu.Unlock();return State{},err}
 c.generation++;gen:=c.generation
 c.cmd=cmd;c.socket=socket
 c.state=State{State:"starting",Source:source}
 c.publishLocked();s:=c.state;c.mu.Unlock()
 go c.watch(gen,cmd,dir)
 go c.monitor(gen,socket)
 return s,nil
}
func displayEnv()[]string{
 env:=os.Environ()
 runtime:=os.Getenv("XDG_RUNTIME_DIR")
 if runtime=="" {runtime=filepath.Join("/run/user",strconv.Itoa(os.Getuid()));env=append(env,"XDG_RUNTIME_DIR="+runtime)}
 if os.Getenv("WAYLAND_DISPLAY")=="" {
  entries,_:=os.ReadDir(runtime)
  for _,e:=range entries{if strings.HasPrefix(e.Name(),"wayland-")&&!strings.HasSuffix(e.Name(),".lock"){
   fi,err:=os.Stat(filepath.Join(runtime,e.Name()))
   if err==nil&&fi.Mode()&os.ModeSocket!=0{env=append(env,"WAYLAND_DISPLAY="+e.Name());break}
  }}
 }
 if os.Getenv("PULSE_SERVER")==""{env=append(env,"PULSE_SERVER=unix:"+filepath.Join(runtime,"pulse","native"))}
 return env
}
func (c *Controller) watch(gen uint64,cmd *exec.Cmd,dir string){
 err:=cmd.Wait()
 os.RemoveAll(dir)
 c.mu.Lock();defer c.mu.Unlock()
 if gen!=c.generation{return}
 stopped:=c.state.State=="stopping"
 c.cmd=nil;c.socket=""
 if err!=nil&&!stopped{c.state.State="error";c.state.Error=err.Error()}else{c.state.State="idle";c.state.Error=""}
 c.state.Source="";c.state.Position=0;c.state.Duration=0
 c.state.BufferPercent=nil;c.state.BufferSeconds=nil
 c.publishLocked()
}
func (c *Controller) monitor(gen uint64,socket string){
 ticker:=time.NewTicker(time.Second);defer ticker.Stop()
 for range ticker.C {
  c.mu.Lock();active:=gen==c.generation&&c.cmd!=nil;c.mu.Unlock()
  if !active{return}
  paused,err1:=request(socket,"get_property","pause")
  pos,err2:=request(socket,"get_property","time-pos")
  duration,err3:=request(socket,"get_property","duration")
  cachePause,err4:=request(socket,"get_property","paused-for-cache")
  cachePercent,err5:=request(socket,"get_property","cache-buffering-state")
  cacheSeconds,err6:=request(socket,"get_property","demuxer-cache-duration")
  if err1!=nil{continue}
  c.mu.Lock()
  if gen!=c.generation||c.cmd==nil{c.mu.Unlock();return}
  if c.state.State!="stopping" {
   c.state.State=playbackState(paused==true,err4==nil&&cachePause==true)
   c.state.Error=""
   if err2==nil{if f,ok:=pos.(float64);ok{c.state.Position=f}}
   if err3==nil{if f,ok:=duration.(float64);ok{c.state.Duration=f}}
   c.state.BufferPercent=validBufferValue(cachePercent,err5,100)
   c.state.BufferSeconds=validBufferValue(cacheSeconds,err6,math.MaxFloat64)
   c.publishLocked()
  }
  c.mu.Unlock()
 }
}
// Cache pause takes precedence over the user's pause state: when MPV has
// suspended playback to refill the cache, report why it cannot advance.
func playbackState(paused,cachePaused bool)string{
 if cachePaused{return "buffering"}
 if paused{return "paused"}
 return "playing"
}

// Missing/unavailable MPV properties must be unknown, not misleading zeros.
func validBufferValue(value any, err error, max float64)*float64{
 if err!=nil{return nil}
 v,ok:=value.(float64)
 if !ok||math.IsNaN(v)||math.IsInf(v,0)||v<0||v>max{return nil}
 return &v
}
func request(socket string,args ...any)(any,error){
 conn,err:=net.DialTimeout("unix",socket,400*time.Millisecond);if err!=nil{return nil,err}
 defer conn.Close();conn.SetDeadline(time.Now().Add(800*time.Millisecond))
 if err:=json.NewEncoder(conn).Encode(map[string]any{"command":args,"request_id":1});err!=nil{return nil,err}
 decoder:=json.NewDecoder(bufio.NewReader(conn))
 for {
  var response struct{
   RequestID int `json:"request_id"`
   Error string `json:"error"`
   Data any `json:"data"`
  }
  if err:=decoder.Decode(&response);err!=nil{return nil,err}
  if response.RequestID!=1{continue} // asynchronous MPV events on the same socket
  if response.Error!="success"{return nil,fmt.Errorf("mpv IPC: %s",response.Error)}
  return response.Data,nil
 }
}
func (c *Controller) command(args ...any)error{
 c.mu.Lock();socket:=c.socket;active:=c.cmd!=nil;c.mu.Unlock()
 if !active{return errors.New("nothing playing")}
 _,err:=request(socket,args...);return err
}
func(c *Controller) Pause()error{return c.command("set_property","pause",true)}
func(c *Controller) Resume()error{return c.command("set_property","pause",false)}
func(c *Controller) Seek(position float64)error{
 if position<0||position>86400{return fmt.Errorf("invalid seek position")}
 return c.command("seek",position,"absolute")
}
func(c *Controller) Stop()error{
 c.mu.Lock()
 if c.cmd==nil{c.mu.Unlock();return nil}
 cmd:=c.cmd
 c.state.State="stopping";c.publishLocked()
 c.mu.Unlock()
 if cmd.Process==nil{return nil}
 err:=syscall.Kill(-cmd.Process.Pid,syscall.SIGTERM)
 if err!=nil&&err!=syscall.ESRCH{return err}
 go func(pid int){time.Sleep(3*time.Second);c.mu.Lock();still:=c.cmd!=nil&&c.cmd.Process!=nil&&c.cmd.Process.Pid==pid;c.mu.Unlock();if still{_ =syscall.Kill(-pid,syscall.SIGKILL)}}(cmd.Process.Pid)
 return nil
}
func(c *Controller) Close(){
 _=c.Stop()
 // Do not let the daemon exit while an orphan player still owns fullscreen.
 deadline:=time.Now().Add(4*time.Second)
 for time.Now().Before(deadline){
  c.mu.Lock();cmd:=c.cmd;c.mu.Unlock()
  if cmd==nil{return}
  time.Sleep(50*time.Millisecond)
 }
 c.mu.Lock();cmd:=c.cmd;c.mu.Unlock()
 if cmd!=nil&&cmd.Process!=nil{_ =syscall.Kill(-cmd.Process.Pid,syscall.SIGKILL)}
}

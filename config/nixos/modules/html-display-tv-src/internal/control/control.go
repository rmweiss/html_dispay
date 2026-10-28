package control

import (
 "encoding/hex"
 "encoding/json"
 "errors"
 "example.com/html-display-tv/internal/exlink"
 "fmt"
 "strings"
 "sync"
 "time"
)

type Request struct {
 Command string `json:"command"`
 Action string `json:"action,omitempty"`
 Value json.RawMessage `json:"value,omitempty"`
}
type Reply struct {
 OK bool `json:"ok"`
 Value any `json:"value,omitempty"`
 State map[string]any `json:"state,omitempty"`
 Confirmation string `json:"confirmation,omitempty"`
 Error string `json:"error,omitempty"`
 Sent string `json:"sent,omitempty"`
 Response string `json:"response,omitempty"`
}
type Controller struct {
 Port *exlink.Port
 mu sync.Mutex
 Publish func(string,any)
 OSDLast string // Last ACKed setting this daemon session, not queried from TV.
}
var inputs=map[byte]string{0x39:"HDMI1",0x47:"HDMI1",0x3a:"HDMI2",0x48:"HDMI2",0x3b:"HDMI3",0x49:"HDMI3",0x3c:"HDMI4",0x4a:"HDMI4"}

func (c *Controller) get(k string)(any,error){
 idx,ok:=map[string]byte{"power":0,"volume":1,"mute":2,"input":4,"backlight":10}[k]
 if !ok{return nil,fmt.Errorf("unsupported property %s",k)}
 n,e:=c.Port.Query(idx)
 if e!=nil{return nil,e}
 switch k{
 case "power":
  switch n{case 5:return "ON",nil;case 4,8:return "OFF",nil;default:return "UNKNOWN",nil}
 case "mute":return n==1,nil
 case "input":
  if s,ok:=inputs[n];ok{return s,nil}
  return fmt.Sprintf("0x%02X",n),nil
 default:return int(n),nil
 }
}
func decodeValue(b json.RawMessage)(int,error){
 var n int
 if err:=json.Unmarshal(b,&n);err==nil{return n,nil}
 var s string
 if err:=json.Unmarshal(b,&s);err==nil{
  if s=="ON"{return 1,nil}
  if s=="OFF"{return 0,nil}
 }
 return 0,fmt.Errorf("invalid value: %s",string(b))
}
func (c *Controller) Handle(r Request)Reply{
 c.mu.Lock()
 defer c.mu.Unlock()
 res,e:=c.handle(r)
 if e!=nil{return Reply{Error:e.Error()}}
 return res
}
func (c *Controller) power(on bool)(string,error){
 target:="OFF"
 if on{target="ON"}
 var last error
 for attempt:=0;attempt<3;attempt++{
  if attempt>0{time.Sleep(400*time.Millisecond)}
  err:=c.Port.Power(on)
  if err==nil{return "ack",nil}
  last=err
  if !errors.Is(err,exlink.ErrTimeout){return "",err}
  if on{
   if got,err:=c.get("power");err==nil&&got==target{return "readback",nil}
  }
 }
 if on{
  if got,err:=c.get("power");err==nil&&got==target{return "readback",nil}
 }
 return "",fmt.Errorf("power %s not acknowledged after 3 attempts: %w",target,last)
}
func (c *Controller) handle(r Request)(Reply,error){
 if r.Command=="raw"{
  if r.Action!="set"{return Reply{},fmt.Errorf("raw requires hex command bytes")}
  var input string
  if err:=json.Unmarshal(r.Value,&input);err!=nil{return Reply{},fmt.Errorf("raw value must be a hex string")}
  frame,err:=DecodeRaw(input)
  if err!=nil{return Reply{},err}
  reply,err:=c.Port.Raw(frame)
  if err!=nil{return Reply{},fmt.Errorf("raw %x: %w (response %x)",frame,err,reply)}
  return Reply{OK:true,Sent:hex.EncodeToString(frame),Response:hex.EncodeToString(reply),Confirmation:"ack"},nil
 }
 if r.Command=="osd"{
  if r.Action==""||r.Action=="get"{
   if c.OSDLast==""{return Reply{OK:true,Value:"UNKNOWN",Confirmation:"not_queried"},nil}
   return Reply{OK:true,Value:c.OSDLast,Confirmation:"last_commanded"},nil
  }
  if r.Action!="set"{return Reply{},fmt.Errorf("osd requires on or off")}
  var value string
  if err:=json.Unmarshal(r.Value,&value);err!=nil{return Reply{},fmt.Errorf("osd requires ON or OFF")}
  value=strings.ToUpper(strings.TrimSpace(value))
  if value!="ON"&&value!="OFF"{return Reply{},fmt.Errorf("osd requires ON or OFF")}
  v:=byte(0)
  if value=="ON"{v=1}
  frame:=exlink.Frame(0x08,0x22,0x0e,0x01,0x00,v)
  reply,err:=c.Port.Raw(frame)
  if err!=nil{return Reply{},fmt.Errorf("OSD %s: %w (response %x)",value,err,reply)}
  c.OSDLast=value
  if c.Publish!=nil{c.Publish("osd",value)}
  return Reply{OK:true,Value:value,Confirmation:"ack"},nil
 }
 if r.Command=="status"{
  state:=map[string]any{}
  for _,k:=range []string{"power","volume","mute","input","backlight"}{
   v,e:=c.get(k)
   if e==nil{state[k]=v}else{state[k]=nil}
  }
  if c.OSDLast!="" {state["osd_last_commanded"]=c.OSDLast}
  return Reply{OK:true,State:state},nil
 }
 if r.Action==""{r.Action="get"}
 if r.Action=="get"{
  v,e:=c.get(r.Command)
  if e!=nil{return Reply{},e}
  return Reply{OK:true,Value:v},nil
 }
 if r.Action!="set"{return Reply{},fmt.Errorf("invalid action")}
 if r.Command=="input"{
  var requested string
  if err:=json.Unmarshal(r.Value,&requested);err!=nil{return Reply{},fmt.Errorf("input requires HDMI1, HDMI2, HDMI3 or HDMI4")}
  requested=strings.ToUpper(strings.TrimSpace(requested))
  hdmi:=map[string]int{"HDMI1":1,"HDMI2":2,"HDMI3":3,"HDMI4":4}[requested]
  if hdmi==0{return Reply{},fmt.Errorf("unsupported input %q (use HDMI1..HDMI4)",requested)}
  if err:=c.Port.Input(hdmi);err!=nil{return Reply{},err}
  var actual any
  var err error
  for i:=0;i<5;i++{
   if i>0{time.Sleep(350*time.Millisecond)}
   actual,err=c.get("input")
   if err==nil&&actual==requested{
    if c.Publish!=nil{c.Publish("input",actual)}
    return Reply{OK:true,Value:actual,Confirmation:"readback"},nil
   }
  }
  return Reply{},fmt.Errorf("input command acknowledged but readback did not confirm %s (last=%v, error=%v)",requested,actual,err)
 }
 v,e:=decodeValue(r.Value)
 if e!=nil{return Reply{},e}
 switch r.Command{
 case "power":
  if v!=0&&v!=1{return Reply{},fmt.Errorf("power requires ON/OFF")}
  confirmation,err:=c.power(v==1)
  if err!=nil{return Reply{},err}
  s:="OFF"
  if v==1{s="ON"}
  if c.Publish!=nil{c.Publish("power",s)}
  return Reply{OK:true,Value:s,Confirmation:confirmation},nil
 case "volume":
  if v<0||v>100{return Reply{},fmt.Errorf("volume out of range 0..100")}
  e=c.Port.Volume(byte(v))
 case "backlight":
  if v<0||v>50{return Reply{},fmt.Errorf("backlight out of range 0..50")}
  e=c.Port.Backlight(byte(v))
 default:return Reply{},fmt.Errorf("unsupported write: %s",r.Command)
 }
 if e!=nil{return Reply{},e}
 actual,e:=c.get(r.Command)
 if e!=nil{return Reply{},fmt.Errorf("command ACK received but readback failed: %w",e)}
 if c.Publish!=nil{c.Publish(r.Command,actual)}
 if actual!=v{return Reply{},fmt.Errorf("readback mismatch: wanted %d got %v",v,actual)}
 return Reply{OK:true,Value:actual,Confirmation:"readback"},nil
}
func (c *Controller) Poll(){
 c.mu.Lock()
 defer c.mu.Unlock()
 r,err:=c.handle(Request{Command:"status"})
 if err!=nil||!r.OK||c.Publish==nil{return}
 for k,v:=range r.State{
  if v!=nil{c.Publish(k,v)}
 }
}

// DecodeRaw accepts four payload bytes, six frame bytes without checksum,
// or a complete checksummed seven-byte frame.
func DecodeRaw(input string)([]byte,error){
 cleaned:=strings.NewReplacer(" ","",":","","-","","\t","").Replace(strings.TrimSpace(input))
 raw,err:=hex.DecodeString(cleaned)
 if err!=nil{return nil,fmt.Errorf("invalid hex: %w",err)}
 switch len(raw){
 case 4:
  return exlink.Frame(append([]byte{0x08,0x22},raw...)...),nil
 case 6:
  if raw[0]!=0x08||raw[1]!=0x22{return nil,fmt.Errorf("expected ExLink header 08 22")}
  return exlink.Frame(raw...),nil
 case 7:
  if err:=exlink.ValidateRawFrame(raw);err!=nil{return nil,err}
  return raw,nil
 default:
  return nil,fmt.Errorf("expected 4 command bytes, 6 frame bytes without checksum, or 7 complete bytes; got %d",len(raw))
 }
}

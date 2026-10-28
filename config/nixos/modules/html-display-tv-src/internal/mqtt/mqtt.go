// Package mqtt implements only the MQTT 3.1.1 QoS0 features needed by this bridge.
// It intentionally does not implement TLS, authentication, QoS1 or durable sessions.
package mqtt

import (
 "bufio"
 "bytes"
 "encoding/binary"
 "encoding/json"
 "fmt"
 "io"
 "log"
 "net"
 "strings"
 "sync"
 "time"
)
type Client struct {
 Address,Prefix string
 OnCommand func(string,string)
 OnConnect func()
 mu sync.Mutex
 conn net.Conn
 stop chan struct{}
}
func enc(s string)[]byte{
 var b bytes.Buffer
 binary.Write(&b,binary.BigEndian,uint16(len(s)))
 b.WriteString(s)
 return b.Bytes()
}
func packet(typ byte,payload []byte)[]byte{
 r:=[]byte{typ}
 n:=len(payload)
 for {
  b:=byte(n%128)
  n/=128
  if n>0{b|=128}
  r=append(r,b)
  if n==0{break}
 }
 return append(r,payload...)
}
func readPacket(r *bufio.Reader)(byte,[]byte,error){
 t,e:=r.ReadByte()
 if e!=nil{return 0,nil,e}
 size,mult:=0,1
 for i:=0;i<4;i++{
  b,e:=r.ReadByte()
  if e!=nil{return 0,nil,e}
  size+=int(b&127)*mult
  if b&128==0{
   if size>65536{return 0,nil,fmt.Errorf("packet too large")}
   data:=make([]byte,size)
   _,e=io.ReadFull(r,data)
   return t,data,e
  }
  mult*=128
 }
 return 0,nil,fmt.Errorf("malformed length")
}
func (c *Client) send(payload []byte)error{
 c.mu.Lock()
 defer c.mu.Unlock()
 if c.conn==nil{return fmt.Errorf("MQTT disconnected")}
 c.conn.SetWriteDeadline(time.Now().Add(3*time.Second))
 _,e:=c.conn.Write(payload)
 return e
}
func (c *Client) Publish(prop,value string){
 c.publish(c.Prefix+"/"+prop+"/state",value,true)
}
func (c *Client) publish(topic,value string,retained bool){
 header:=byte(0x30)
 if retained{header|=1}
 body:=append(enc(topic),[]byte(value)...)
 _=c.send(packet(header,body))
}
// Discovery publishes power, volume, backlight and input controls plus mute status.
func (c *Client) PublishDiscovery(){
 type config map[string]any
 device:=config{"identifiers":[]string{"html-display-samsung-frame"},"name":"Samsung Frame","manufacturer":"Samsung","model":"LS03B"}
 availability:=c.Prefix+"/availability"
 entity:=func(component,id string,fields config){
  fields["name"]=id
  fields["unique_id"]="html_display_tv_"+id
  fields["device"]=device
  fields["availability_topic"]=availability
  fields["payload_available"]="online"
  fields["payload_not_available"]="offline"
  raw,err:=json.Marshal(fields)
  if err==nil{c.publish("homeassistant/"+component+"/html_display_tv_"+id+"/config",string(raw),true)}
 }
 entity("switch","power",config{
  "command_topic":c.Prefix+"/power/set","state_topic":c.Prefix+"/power/state",
  "payload_on":"ON","payload_off":"OFF","state_on":"ON","state_off":"OFF","icon":"mdi:television"})
 for _,spec:=range []struct{name string;max int}{{"volume",100},{"backlight",50}}{
  entity("number",spec.name,config{
   "command_topic":c.Prefix+"/"+spec.name+"/set",
   "state_topic":c.Prefix+"/"+spec.name+"/state",
   "min":0,"max":spec.max,"step":1,"mode":"box"})
 }
 entity("switch","osd",config{
  "command_topic":c.Prefix+"/osd/set",
  "state_topic":c.Prefix+"/osd/state",
  "payload_on":"ON","payload_off":"OFF","state_on":"ON","state_off":"OFF",
  "icon":"mdi:monitor-eye"})
 entity("binary_sensor","mute",config{
  "state_topic":c.Prefix+"/mute/state","payload_on":"true","payload_off":"false","icon":"mdi:volume-off"})
 entity("select","input",config{
  "command_topic":c.Prefix+"/input/set",
  "state_topic":c.Prefix+"/input/state",
  "options":[]string{"HDMI1","HDMI2","HDMI3","HDMI4"},
  "icon":"mdi:video-input-hdmi"})
 // Remove the obsolete read-only input sensor.
 c.publish("homeassistant/sensor/html_display_tv_input/config","",true)
}
func (c *Client) Run(){
 if c.stop==nil{c.stop=make(chan struct{})}
 for {
  select{case <-c.stop:return;default:}
  if err:=c.session();err!=nil{log.Printf("MQTT: %v",err)}
  select{case <-c.stop:return;case <-time.After(3*time.Second):}
 }
}
func (c *Client) session()error{
 nc,e:=net.DialTimeout("tcp",c.Address,3*time.Second)
 if e!=nil{return e}
 defer nc.Close()
 c.mu.Lock()
 c.conn=nc
 c.mu.Unlock()
 defer func(){c.mu.Lock();c.conn=nil;c.mu.Unlock()}()
 willTopic:=c.Prefix+"/availability"
 body:=append(enc("MQTT"),4,0x06,0,30)
 body=append(body,enc("html-display-tv")...)
 body=append(body,enc(willTopic)...)
 body=append(body,enc("offline")...)
 if e=c.send(packet(0x10,body));e!=nil{return e}
 nc.SetReadDeadline(time.Now().Add(5*time.Second))
 r:=bufio.NewReader(nc)
 t,p,e:=readPacket(r)
 if e!=nil{return e}
 if t!=0x20||len(p)!=2||p[1]!=0{return fmt.Errorf("MQTT connack rejected: %x %x",t,p)}
 c.publish(willTopic,"online",true)
 c.PublishDiscovery()
 if c.OnConnect!=nil{go c.OnConnect()}
 sub:=append([]byte{0,1},enc(c.Prefix+"/+/set")...)
 sub=append(sub,0)
 if e=c.send(packet(0x82,sub));e!=nil{return e}
 ticker:=time.NewTicker(10*time.Second)
 defer ticker.Stop()
 incoming:=make(chan struct{t byte;p []byte;e error},1)
 go func(){
  for {
   nc.SetReadDeadline(time.Now().Add(35*time.Second))
   t,p,e:=readPacket(r)
   select{
   case incoming<-struct{t byte;p []byte;e error}{t,p,e}:
   case <-c.stop:return
   }
   if e!=nil{return}
  }
 }()
 for {
  select{
  case <-c.stop:return nil
  case <-ticker.C:
   if e=c.send([]byte{0xc0,0});e!=nil{return e}
  case msg:=<-incoming:
   if msg.e!=nil{return msg.e}
   if msg.t>>4!=3||len(msg.p)<2{continue}
   l:=int(binary.BigEndian.Uint16(msg.p[:2]))
   if len(msg.p)<2+l{continue}
   topic:=string(msg.p[2:2+l])
   name:=strings.TrimSuffix(strings.TrimPrefix(topic,c.Prefix+"/"),"/set")
   if !strings.HasSuffix(topic,"/set"){continue}
   if c.OnCommand!=nil{go c.OnCommand(name,string(msg.p[2+l:]))}
  }
 }
}
func (c *Client) Close(){
 if c.stop!=nil{close(c.stop)}
 c.mu.Lock()
 defer c.mu.Unlock()
 if c.conn!=nil{c.conn.Close()}
}

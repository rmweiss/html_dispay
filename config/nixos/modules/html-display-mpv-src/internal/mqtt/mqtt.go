// Package mqtt provides a deliberately small MQTT 3.1.1 TCP/QoS0 adapter.
// Like the existing TV bridge it assumes a trusted broker; no TLS or credentials.
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
 OnCommand func(name,payload string)
 OnConnect func()
 mu sync.Mutex
 conn net.Conn
 done chan struct{}
 once sync.Once
}
func enc(s string)[]byte{
 var b bytes.Buffer
 _=binary.Write(&b,binary.BigEndian,uint16(len(s)))
 b.WriteString(s);return b.Bytes()
}
func packet(t byte,payload []byte)[]byte{
 r:=[]byte{t};n:=len(payload)
 for {b:=byte(n%128);n/=128;if n>0{b|=128};r=append(r,b);if n==0{break}}
 return append(r,payload...)
}
func readPacket(r *bufio.Reader)(byte,[]byte,error){
 t,e:=r.ReadByte();if e!=nil{return 0,nil,e}
 n,m:=0,1
 for i:=0;i<4;i++{b,e:=r.ReadByte();if e!=nil{return 0,nil,e};n+=int(b&127)*m
  if b&128==0{if n>65536{return 0,nil,fmt.Errorf("oversized MQTT packet")};v:=make([]byte,n);_,e=io.ReadFull(r,v);return t,v,e};m*=128}
 return 0,nil,fmt.Errorf("bad MQTT remaining length")
}
func(c *Client) send(b []byte)error{
 c.mu.Lock();defer c.mu.Unlock()
 if c.conn==nil{return fmt.Errorf("disconnected")}
 _=c.conn.SetWriteDeadline(time.Now().Add(3*time.Second))
 _,e:=c.conn.Write(b);return e
}
func(c *Client) publish(topic,payload string,retain bool){
 h:=byte(0x30);if retain{h|=1}
 _=c.send(packet(h,append(enc(topic),[]byte(payload)...)))
}
func(c *Client) State(prop,value string){c.publish(c.Prefix+"/"+prop+"/state",value,true)}
func(c *Client) Discovery(){
 device:=map[string]any{"identifiers":[]string{"html-display-mpv"},"name":"HTML Display MPV","manufacturer":"HTML Display"}
 availability:=c.Prefix+"/availability"
 entity:=func(kind,id string,fields map[string]any){
  fields["name"]=id;fields["unique_id"]="html_display_mpv_"+id
  fields["device"]=device;fields["availability_topic"]=availability
  fields["payload_available"]="online";fields["payload_not_available"]="offline"
  payload,_:=json.Marshal(fields)
  c.publish("homeassistant/"+kind+"/html_display_mpv_"+id+"/config",string(payload),true)
 }
 for _,name:=range []string{"playback","source","position","duration"}{
  f:=map[string]any{"state_topic":c.Prefix+"/"+name+"/state"}
  if name=="position"||name=="duration"{f["unit_of_measurement"]="s";f["state_class"]="measurement"}
  entity("sensor",name,f)
 }
 entity("binary_sensor","video_active",map[string]any{
  "state_topic":c.Prefix+"/playback/state",
  "value_template":`{{ 'ON' if value in ['starting','playing','paused','stopping'] else 'OFF' }}`,
  "payload_on":"ON","payload_off":"OFF",
  "icon":"mdi:movie-play",
 })
}
func(c *Client) Run(){
 if c.done==nil{c.done=make(chan struct{})}
 for {
  select{case <-c.done:return;default:}
  if err:=c.session();err!=nil{log.Printf("MPV MQTT: %v",err)}
  select{case <-c.done:return;case <-time.After(3*time.Second):}
 }
}
func(c *Client) session()error{
 nc,e:=net.DialTimeout("tcp",c.Address,3*time.Second);if e!=nil{return e}
 defer nc.Close()
 c.mu.Lock();c.conn=nc;c.mu.Unlock()
 defer func(){c.mu.Lock();c.conn=nil;c.mu.Unlock()}()
 willTopic:=c.Prefix+"/availability"
 body:=append(enc("MQTT"),4,0x06,0,30)
 body=append(body,enc("html-display-mpv")...)
 body=append(body,enc(willTopic)...)
 body=append(body,enc("offline")...)
 if err:=c.send(packet(0x10,body));err!=nil{return err}
 r:=bufio.NewReader(nc);_=nc.SetReadDeadline(time.Now().Add(5*time.Second))
 t,p,err:=readPacket(r)
 if err!=nil{return err}
 if t!=0x20||len(p)!=2||p[1]!=0{return fmt.Errorf("MQTT connack failed: %x %x",t,p)}
 sub:=append([]byte{0,1},enc(c.Prefix+"/+/set")...);sub=append(sub,0)
 if err:=c.send(packet(0x82,sub));err!=nil{return err}
 c.publish(willTopic,"online",true)
 c.Discovery()
 if c.OnConnect!=nil{c.OnConnect()}
 tick:=time.NewTicker(10*time.Second);defer tick.Stop()
 incoming:=make(chan struct{t byte;p []byte;err error},1)
 go func(){
  for{
   _=nc.SetReadDeadline(time.Now().Add(35*time.Second))
   t,p,err:=readPacket(r)
   select{case incoming<-struct{t byte;p []byte;err error}{t,p,err}:
    case <-c.done:return}
   if err!=nil{return}
  }
 }()
 for{
  select{
  case <-c.done:
   // Graceful disconnect updates availability before leaving.
   c.publish(willTopic,"offline",true);_ =c.send([]byte{0xe0,0});return nil
  case <-tick.C:
   if err:=c.send([]byte{0xc0,0});err!=nil{return err}
  case msg:=<-incoming:
   if msg.err!=nil{return msg.err}
   if msg.t>>4!=3||len(msg.p)<2||msg.t&1!=0{continue} // ignore retained commands
   l:=int(binary.BigEndian.Uint16(msg.p[:2]));if len(msg.p)<2+l{continue}
   topic:=string(msg.p[2:2+l])
   if !strings.HasPrefix(topic,c.Prefix+"/")||!strings.HasSuffix(topic,"/set"){continue}
   name:=strings.TrimSuffix(strings.TrimPrefix(topic,c.Prefix+"/"),"/set")
   if c.OnCommand!=nil{go c.OnCommand(name,string(msg.p[2+l:]))}
  }
 }
}
func(c *Client) Close(){
 c.once.Do(func(){if c.done!=nil{close(c.done)}})
 c.mu.Lock();defer c.mu.Unlock();if c.conn!=nil{_ =c.conn.Close()}
}

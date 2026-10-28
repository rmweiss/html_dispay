package exlink

import (
 "encoding/hex"
 "errors"
 "fmt"
 "io"
 "os"
 "sync"
 "syscall"
 "time"
 "unsafe"
)

var ErrTimeout = errors.New("ExLink response timeout")
var ErrNack = errors.New("ExLink command rejected")

type Port struct {
 f *os.File
 mu sync.Mutex
}

// Open configures a Linux USB serial tty at 9600 8N1, raw, no flow control.
func Open(path string) (*Port, error) {
 fd,e:=syscall.Open(path,syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK,0)
 if e!=nil {return nil,e}
 cleanup:=func(e error)(*Port,error){syscall.Close(fd);return nil,e}
 var t syscall.Termios
 if _,_,errno:=syscall.Syscall(syscall.SYS_IOCTL,uintptr(fd),uintptr(syscall.TCGETS),uintptr(unsafe.Pointer(&t)));errno!=0{return cleanup(errno)}
 t.Iflag=0
 t.Oflag=0
 t.Lflag=0
 t.Cflag=syscall.CS8|syscall.CREAD|syscall.CLOCAL|syscall.B9600
 t.Cc[syscall.VMIN]=0
 t.Cc[syscall.VTIME]=0
 if _,_,errno:=syscall.Syscall(syscall.SYS_IOCTL,uintptr(fd),uintptr(syscall.TCSETS),uintptr(unsafe.Pointer(&t)));errno!=0{return cleanup(errno)}
 return &Port{f:os.NewFile(uintptr(fd),path)},nil
}
func (p *Port) Close() error {return p.f.Close()}
func Frame(body ...byte) []byte {
 out:=append([]byte(nil),body...)
 var n byte
 for _,b:=range out {n+=b}
 return append(out,-n)
}
func (p *Port) exchange(command []byte, query *byte)(byte,error){
 p.mu.Lock()
 defer p.mu.Unlock()
 fd:=int(p.f.Fd())
 tmp:=make([]byte,256)
 for {
  n,e:=syscall.Read(fd,tmp)
  if n<=0||e!=nil {break}
 }
 for offset:=0;offset<len(command);{
  n,e:=syscall.Write(fd,command[offset:])
  if e==syscall.EAGAIN {time.Sleep(5*time.Millisecond);continue}
  if e!=nil {return 0,e}
  offset+=n
 }
 deadline:=time.Now().Add(1800*time.Millisecond)
 data:=make([]byte,0,64)
 ack:=false
 for time.Now().Before(deadline) {
  n,e:=syscall.Read(fd,tmp)
  if n>0 {data=append(data,tmp[:n]...)} else if e!=nil&&e!=syscall.EAGAIN&&e!=syscall.EWOULDBLOCK {return 0,e}
  for i:=0;i+3<=len(data);i++ {
   if data[i]==3&&data[i+1]==12 {
    if data[i+2]==0xff{return 0,ErrNack}
    if data[i+2]==0xf1{ack=true}
   }
  }
  if query==nil&&ack{return 0,nil}
  if query!=nil {
   for i:=0;i+13<=len(data);i++{
    r:=data[i:i+13]
    if r[0]!=3||r[1]!=12||r[2]!=0xf5||r[3]!=8||r[4]!=0xf0||r[5]!=*query{continue}
    var sum byte
    for _,b:=range r{sum+=b}
    if sum!=0{return 0,errors.New("bad status checksum")}
    if r[8]!=0xf1{return 0,fmt.Errorf("status code 0x%02x",r[8])}
    return r[9],nil
   }
  }
  if len(data)>128{data=append([]byte(nil),data[len(data)-32:]...)}
  time.Sleep(10*time.Millisecond)
 }
 return 0,ErrTimeout
}
func (p *Port) Query(index byte)(byte,error){return p.exchange(Frame(8,0x22,0xf0,index,0,0),&index)}
func (p *Port) Power(on bool)error{
 v:=byte(1)
 if on {v=2}
 _,e:=p.exchange(Frame(8,0x22,0,0,0,v),nil)
 return e
}
func (p *Port) Volume(v byte)error{_,e:=p.exchange(Frame(8,0x22,1,0,0,v),nil);return e}
func (p *Port) Backlight(v byte)error{
 _,e:=p.exchange(Frame(8,0x22,0x0b,1,0,v),nil)
 return e
}
func (p *Port) Check()error{
 _,e:=p.f.Stat()
 if e!=nil{return e}
 _,e=p.f.Seek(0,io.SeekCurrent)
 if e==nil{return errors.New("expected tty, got seekable file")}
 return nil
}
// Input selects HDMI1..HDMI4. The source selection group is 0x05.
func (p *Port) Input(hdmi int)error{
 if hdmi<1||hdmi>4{return fmt.Errorf("unsupported HDMI input %d",hdmi)}
 _,e:=p.exchange(Frame(0x08,0x22,0x0a,0x00,0x05,byte(hdmi-1)),nil)
 return e
}

// ValidateRawFrame accepts a complete seven-byte Samsung ExLink command.
func ValidateRawFrame(frame []byte) error {
 if len(frame)!=7 { return fmt.Errorf("ExLink frame must be 7 bytes, got %d",len(frame)) }
 if frame[0]!=0x08 || frame[1]!=0x22 {return fmt.Errorf("expected ExLink header 08 22")}
 var sum byte
 for _,v:=range frame {sum+=v}
 if sum!=0 {return fmt.Errorf("invalid ExLink checksum; expected %02X",Frame(frame[:6]...)[6])}
 return nil
}
// Raw sends a validated frame, captures ACK and response, and shares the serial mutex.
func (p *Port) Raw(frame []byte)([]byte,error){
 if err:=ValidateRawFrame(frame);err!=nil{return nil,err}
 p.mu.Lock()
 defer p.mu.Unlock()
 fd:=int(p.f.Fd())
 tmp:=make([]byte,256)
 for{
  n,e:=syscall.Read(fd,tmp)
  if n<=0||e!=nil{break}
 }
 for offset:=0;offset<len(frame);{
  n,e:=syscall.Write(fd,frame[offset:])
  if e==syscall.EAGAIN{time.Sleep(5*time.Millisecond);continue}
  if e!=nil{return nil,e}
  if n==0{return nil,io.ErrNoProgress}
  offset+=n
 }
 deadline:=time.Now().Add(1800*time.Millisecond)
 var received []byte
 var lastByte time.Time
 ack,nack:=false,false
 for time.Now().Before(deadline){
  n,e:=syscall.Read(fd,tmp)
  if n>0 {received=append(received,tmp[:n]...);lastByte=time.Now()} else if e!=nil&&e!=syscall.EAGAIN&&e!=syscall.EWOULDBLOCK {return received,e}
  for i:=0;i+3<=len(received);i++{
   if received[i]==3&&received[i+1]==12{
    if received[i+2]==0xf1{ack=true}
    if received[i+2]==0xff{nack=true}
   }
  }
  if nack{return received,fmt.Errorf("%w (response %s)",ErrNack,hex.EncodeToString(received))}
  if ack&&time.Since(lastByte)>=120*time.Millisecond{return received,nil}
  if len(received)>=512{return received,fmt.Errorf("raw response exceeds 512 bytes")}
  time.Sleep(10*time.Millisecond)
 }
 return received,fmt.Errorf("%w (response %s)",ErrTimeout,hex.EncodeToString(received))
}

package control

import (
 "bytes"
 "testing"
)

func TestDecodeRaw(t *testing.T){
 cases:=[]struct{in string;want []byte}{
  {"0E 01 00 00",[]byte{0x08,0x22,0x0e,0x01,0,0,0xc7}},
  {"08 22 0e 01 00 00",[]byte{0x08,0x22,0x0e,0x01,0,0,0xc7}},
  {"08 22 0e 01 00 00 c7",[]byte{0x08,0x22,0x0e,0x01,0,0,0xc7}},
 }
 for _,tc:=range cases{
  got,err:=DecodeRaw(tc.in)
  if err!=nil{t.Fatalf("%q: %v",tc.in,err)}
  if !bytes.Equal(got,tc.want){t.Errorf("%q got %x, want %x",tc.in,got,tc.want)}
 }
 for _,v:=range []string{"01 02","08 22 0E 01 00 00 FF","10 20 0e 01 00 00 c7"}{
  if _,err:=DecodeRaw(v);err==nil{t.Errorf("%q unexpectedly accepted",v)}
 }
}

package main

import "testing"

func TestParseVolume(t *testing.T) {
 for _, tc := range []struct{ input string; want int }{
  {"Volume: 0.40\n",40},
  {"Volume: 1.00",100},
  {"Volume: 0.00 [MUTED]",0},
  {"Volume: 0.255",26},
 } {
  got,err:=parseVolume(tc.input)
  if err!=nil || got!=tc.want { t.Errorf("parseVolume(%q) = %d, %v; want %d",tc.input,got,err,tc.want) }
 }
 for _, input:=range []string{"", "oops", "Volume: -0.1", "Volume: 1.50"} {
  if _,err:=parseVolume(input);err==nil { t.Errorf("accepted %q",input) }
 }
}

package player

import (
 "errors"
 "math"
 "testing"
)

func TestPlaybackState(t *testing.T) {
 for _, tc := range []struct {
  paused,cachePaused bool
  want string
 }{
  {false,false,"playing"},
  {true,false,"paused"},
  {false,true,"buffering"},
  {true,true,"buffering"},
 }{
  got:=playbackState(tc.paused,tc.cachePaused)
  if got!=tc.want{t.Errorf("playbackState(%t,%t)=%s, want %s",tc.paused,tc.cachePaused,got,tc.want)}
 }
}

func TestValidBufferValue(t *testing.T) {
 for _,tc:=range []struct{
  value any
  err error
  max float64
  wantValid bool
 }{
  {float64(0),nil,100,true},
  {float64(100),nil,100,true},
  {float64(42.5),nil,100,true},
  {float64(-1),nil,100,false},
  {float64(101),nil,100,false},
  {math.NaN(),nil,100,false},
  {math.Inf(1),nil,100,false},
  {nil,nil,100,false},
  {true,nil,100,false},
  {float64(50),errors.New("property unavailable"),100,false},
  {float64(120.7),nil,math.MaxFloat64,true},
 } {
  got:=validBufferValue(tc.value,tc.err,tc.max)
  if (got!=nil)!=tc.wantValid{t.Errorf("validBufferValue(%v,%v,%v) valid=%t want %t",tc.value,tc.err,tc.max,got!=nil,tc.wantValid)}
 }
}

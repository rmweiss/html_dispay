package main

import (
 "encoding/json"
 "errors"
 "fmt"
 "log"
 "net/http"
 "os"
 "os/exec"
 "path/filepath"
 "strconv"
 "syscall"
 "strings"
 "sync"
 "time"
)

const (
 brightnessFile = "/var/lib/html-display/software-brightness.json"
 brightnessSetTopic = "html-display/display/brightness/set"
 brightnessStateTopic = "html-display/display/brightness/state"
)

type brightnessController struct {
 mu sync.Mutex
 executable string
 stateFile string
 value int
 process *exec.Cmd
 processDone chan error
}

func newBrightnessController(executable, stateFile string) (*brightnessController, error) {
 b := &brightnessController{executable: executable, stateFile: stateFile, value: 100}
 data, err := os.ReadFile(stateFile)
 if err == nil {
  var saved struct { Brightness int `json:"brightness"` }
  if err := json.Unmarshal(data, &saved); err != nil { return nil, fmt.Errorf("read software brightness: %w", err) }
  if saved.Brightness < 10 || saved.Brightness > 100 { return nil, fmt.Errorf("stored brightness out of range: %d", saved.Brightness) }
  b.value = saved.Brightness
 } else if !errors.Is(err, os.ErrNotExist) { return nil, err }
 return b, nil
}

func (b *brightnessController) get() int {
 b.mu.Lock(); defer b.mu.Unlock()
 return b.value
}

// stopLocked releases the previous gamma-control object before starting another.
// Sway only allows one active controller per output.
func (b *brightnessController) stopLocked() {
 if b.process == nil { return }
 cmd, done := b.process, b.processDone
 b.process = nil
 b.processDone = nil
 _ = cmd.Process.Signal(syscall.SIGTERM)
 select {
 case <-done:
 case <-time.After(2 * time.Second):
  _ = cmd.Process.Kill()
  <-done
 }
}

// apply must be called with b.mu held. Unlike -O, a running Gammastep
// process owns the Sway gamma-control object and keeps the dimming active.
func (b *brightnessController) apply(value int) error {
 if b.executable == "" { return fmt.Errorf("software brightness backend is not configured") }
 runtime := os.Getenv("XDG_RUNTIME_DIR")
 if runtime == "" { runtime = fmt.Sprintf("/run/user/%d", os.Getuid()) }
 display := os.Getenv("WAYLAND_DISPLAY")
 if display == "" {
  sockets, _ := filepath.Glob(filepath.Join(runtime, "wayland-*"))
  for _, socket := range sockets {
   if strings.HasSuffix(socket, ".lock") { continue }
   info, err := os.Stat(socket)
   if err == nil && info.Mode()&os.ModeSocket != 0 {
    display = filepath.Base(socket)
    break
   }
  }
 }
 if display == "" { return fmt.Errorf("no Wayland display socket found in %s", runtime) }
 // The constant day/night settings avoid any time-based adjustment.
 brightness := strconv.FormatFloat(float64(value)/100, 'f', 2, 64)
 b.stopLocked()
 cmd := exec.Command(b.executable, "-m", "wayland", "-l", "0:0", "-t", "6500:6500", "-b", brightness+":"+brightness, "-r")
 cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+runtime, "WAYLAND_DISPLAY="+display)
 cmd.Stdout = os.Stdout
 cmd.Stderr = os.Stderr
 if err := cmd.Start(); err != nil { return fmt.Errorf("start gammastep: %w", err) }
 done := make(chan error, 1)
 go func() { done <- cmd.Wait() }()
 // Reject immediately failing processes instead of reporting success to MQTT.
 select {
 case err := <-done:
  return fmt.Errorf("gammastep exited during startup: %v", err)
 case <-time.After(500 * time.Millisecond):
  b.process = cmd
  b.processDone = done
  return nil
 }
}

// close stops the managed gamma controller on orderly shutdown.
func (b *brightnessController) close() {
 b.mu.Lock()
 defer b.mu.Unlock()
 b.stopLocked()
}

func (b *brightnessController) set(value int) error {
 if value < 10 || value > 100 { return fmt.Errorf("brightness must be between 10 and 100") }
 b.mu.Lock()
 defer b.mu.Unlock()
 if err := b.apply(value); err != nil { return err }
 data, _ := json.Marshal(struct { Brightness int `json:"brightness"` }{value})
 dir := filepath.Dir(b.stateFile)
 tmp, err := os.CreateTemp(dir, "brightness-*.tmp")
 if err != nil { return err }
 defer os.Remove(tmp.Name())
 if _, err = tmp.Write(data); err != nil { tmp.Close(); return err }
 if err = tmp.Close(); err != nil { return err }
 if err = os.Rename(tmp.Name(), b.stateFile); err != nil { return err }
 b.value = value
 return nil
}

func (s *Server) softwareBrightness(w http.ResponseWriter, r *http.Request) {
 if s.brightness == nil {
  s.writeAPIError(w, r, http.StatusServiceUnavailable, "software brightness is disabled")
  return
 }
 if r.Method == http.MethodGet {
  w.Header().Set("Content-Type", "application/json")
  _ = json.NewEncoder(w).Encode(map[string]int{"brightness": s.brightness.get()})
  return
 }
 var request struct { Brightness *int `json:"brightness"` }
 decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
 decoder.DisallowUnknownFields()
 if err := decoder.Decode(&request); err != nil || request.Brightness == nil {
  s.writeAPIError(w, r, http.StatusBadRequest, "expected JSON with integer brightness (10-100)")
  return
 }
 if *request.Brightness < 10 || *request.Brightness > 100 {
  s.writeAPIError(w, r, http.StatusBadRequest, "brightness must be 10-100")
  return
 }
 if err := s.brightness.set(*request.Brightness); err != nil {
  s.writeAPIError(w, r, http.StatusInternalServerError, err.Error())
  return
 }
 if s.mqtt != nil { if err := s.mqtt.publishBrightness(*request.Brightness); err != nil { log.Printf("brightness applied but MQTT state publish failed: %v", err) } }
 w.WriteHeader(http.StatusNoContent)
}

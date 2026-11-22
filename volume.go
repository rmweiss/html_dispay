package main

import (
 "context"
 "fmt"
 "math"
 "os"
 "os/exec"
 "regexp"
 "strconv"
 "strings"
 "time"
)

const (
 volumeSetTopic = "html-display/audio/volume/set"
 volumeStateTopic = "html-display/audio/volume/state"
 hdmiSinkName = "alsa_output.pci-0000_00_1f.3.hdmi-stereo"
)

var volumePattern = regexp.MustCompile(`^Volume:\s+([0-9]+(?:\.[0-9]+)?)`)

// volumeController always queries the actual PipeWire default sink. The
// appliance uses a fixed HDMI output; refuse to change a different sink.
type volumeController struct { executable string }

func (v *volumeController) run(args ...string) (string, error) {
 ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
 defer cancel()
 cmd := exec.CommandContext(ctx, v.executable, args...)
 runtime := os.Getenv("XDG_RUNTIME_DIR")
 if runtime == "" { runtime = fmt.Sprintf("/run/user/%d", os.Getuid()) }
 cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+runtime)
 out, err := cmd.CombinedOutput()
 if err != nil { return "", fmt.Errorf("wpctl %v: %w: %s", args, err, strings.TrimSpace(string(out))) }
 return string(out), nil
}

func (v *volumeController) checkSink() error {
 out, err := v.run("inspect", "@DEFAULT_AUDIO_SINK@")
 if err != nil { return err }
 for _, line := range strings.Split(out, "\n") {
  if strings.TrimLeft(strings.TrimSpace(line), "* ") == "node.name = \""+hdmiSinkName+"\"" { return nil }
 }
 return fmt.Errorf("default PipeWire sink is not the configured HDMI output %q", hdmiSinkName)
}

func parseVolume(out string) (int, error) {
 match := volumePattern.FindStringSubmatch(strings.TrimSpace(out))
 if match == nil { return 0, fmt.Errorf("unexpected wpctl volume response %q", strings.TrimSpace(out)) }
 n, err := strconv.ParseFloat(match[1], 64)
 if err != nil || math.IsNaN(n) || n < 0 || n > 1.001 { return 0, fmt.Errorf("invalid PipeWire volume %q", match[1]) }
 return int(math.Round(n * 100)), nil
}

func (v *volumeController) get() (int, error) {
 if err := v.checkSink(); err != nil { return 0, err }
 out, err := v.run("get-volume", "@DEFAULT_AUDIO_SINK@")
 if err != nil { return 0, err }
 return parseVolume(out)
}

func (v *volumeController) set(percent int) (int, error) {
 if percent < 0 || percent > 100 { return 0, fmt.Errorf("volume must be 0-100") }
 if err := v.checkSink(); err != nil { return 0, err }
 if _, err := v.run("set-volume", "@DEFAULT_AUDIO_SINK@", fmt.Sprintf("%.2f", float64(percent)/100)); err != nil { return 0, err }
 return v.get()
}

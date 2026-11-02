package main

import (
 "os"
 "path/filepath"
 "testing"
)

func TestBrightnessDefaultAndStoredValue(t *testing.T) {
 path := filepath.Join(t.TempDir(), "brightness.json")
 b, err := newBrightnessController("gammastep", path)
 if err != nil || b.get() != 100 { t.Fatalf("default: brightness=%v err=%v", b.get(), err) }
 if err := os.WriteFile(path, []byte(`{"brightness":25}`), 0600); err != nil { t.Fatal(err) }
 b, err = newBrightnessController("gammastep", path)
 if err != nil || b.get() != 25 { t.Fatalf("saved: brightness=%v err=%v", b.get(), err) }
}

func TestBrightnessInvalidSavedValue(t *testing.T) {
 path := filepath.Join(t.TempDir(), "brightness.json")
 for _, content := range []string{`{"brightness":0}`, `{"brightness":101}`, "not-json"} {
  if err := os.WriteFile(path, []byte(content), 0600); err != nil { t.Fatal(err) }
  if _, err := newBrightnessController("gammastep", path); err == nil { t.Fatalf("accepted %q", content) }
 }
}

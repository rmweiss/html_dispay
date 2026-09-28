# Kiosk deployment

## Requirements in this repository

Build the Go 1.24 module:

```sh
CGO_ENABLED=0 go build -o html-display-server .
```

The server expects an existing Chromium kiosk page at `http://127.0.0.1:8080/`, CDP on `127.0.0.1:9222`, a writable `/var/lib/html-display` directory, and port 8080 reachable by intended controllers. Its addresses and paths are constants in [main.go](../main.go), not configuration options. The binary can be installed at `/opt/html-display/html-display-server`. A NixOS systemd unit can use `StateDirectory = "html-display";` to manage the persistent state path. Start it after Chromium is ready; the server currently does not wait and retry its initial CDP connection.

## Reported working machine

The earlier project notes describe an N100 mini PC running NixOS as an appliance, with Sway launching Chromium in kiosk mode on an external monitor. Chromium uses XWayland (`--ozone-platform=x11`), a volatile profile under `/run/html-display/chromium`, and CDP bound to loopback only. Sway hides the pointer after inactivity. The Go service was reported running on port 8080, coupled to the Sway service so a kiosk restart also restarts the controller.

Those notes also report PipeWire/WirePlumber audio with a preferred available HDMI/DisplayPort sink and Chromium's `--autoplay-policy=no-user-gesture-required`. Browser audio playback and N100 hardware video decoding for H.264 and AV1 were reported as tested on that machine. The machine's `system.stateVersion = "26.05"` is a NixOS installation compatibility value, not proof of the currently running release.

**Scope of verification:** This repository contains Go code and a README, but no `configuration.nix`, systemd unit, or hardware configuration. Treat the machine details above as a historical deployment report and verify them against the actual NixOS configuration before reproducing or changing the appliance. The current repository code can confirm the CDP address, startup URL, port, state path, and build command.

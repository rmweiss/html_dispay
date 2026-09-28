# Kiosk deployment

## Checked-in NixOS configuration

The [current machine configuration](../config/nixos/configuration.nix) is a copy of the supplied `/etc/nixos/configuration.nix`. It configures an N100 appliance with Sway launching Chromium, the Go server as a systemd service, PipeWire/WirePlumber audio, Intel media acceleration, and an `html-display` state directory. It is a record of the working setup, not a complete NixOS installation.

It imports `./hardware-configuration.nix`, which is generated for the machine and is **not** in this repository. The Go binary must also be built and installed at `/opt/html-display/html-display-server`; the NixOS file does not build or package it. Review paths and machine-specific settings before using the file on another host.

The configuration enables OpenSSH with password authentication and `PermitRootLogin = "yes"`, as used on this LAN appliance. It opens the SSH firewall port and TCP 8080. The Go server listens on all interfaces on port 8080; keep network exposure within the intended trusted LAN. Chromium's CDP port 9222 is used locally and is not opened in the NixOS firewall. The config starts Chromium with `--remote-debugging-port=9222`; the Go server connects to `127.0.0.1:9222`.

## Build and startup

Build the Go 1.24 module:

```sh
CGO_ENABLED=0 go build -o html-display-server .
```

Install that binary at `/opt/html-display/html-display-server` as expected by the checked-in unit. The unit uses `StateDirectory = "html-display";` to manage `/var/lib/html-display`, which holds the latest uploaded HTML document. It requires and starts after `html-display-sway.service`, and a Sway restart also restarts the controller. The Go process still exits if CDP or the exact kiosk page target is not ready when it first connects; systemd's restart policy retries the process.

Sway starts Chromium at `http://127.0.0.1:8080/` in kiosk mode with XWayland (`--ozone-platform=x11`) and a volatile profile under `/run/html-display/chromium`. It hides the pointer after two seconds of inactivity. Chromium uses `--autoplay-policy=no-user-gesture-required`; PipeWire/WirePlumber is configured to prefer an available HDMI/DisplayPort sink. The notes for this machine report working browser audio and N100 hardware decoding for H.264 and AV1. Those observations depend on the connected hardware and were not re-tested as part of this documentation change.

`system.stateVersion = "26.05"` is the installation compatibility setting in the supplied file, not a claim about the currently running NixOS release.

# Kiosk deployment

## Checked-in NixOS configuration

The [current machine configuration](../config/nixos/configuration.nix) records the supplied `/etc/nixos/configuration.nix` plus the audio changes tested during appliance setup. It configures an N100 appliance with Sway launching Chromium, the Go server as a systemd service, PipeWire/WirePlumber audio, Intel media acceleration, and an `html-display` state directory. It is a record of the working setup, not a complete NixOS installation.

It imports `./hardware-configuration.nix`, which is generated for the machine and is **not** in this repository. The Go binary must also be built and installed at `/opt/html-display/html-display-server`; the NixOS file does not build or package it. Review paths and machine-specific settings before using the file on another host.

The configuration enables OpenSSH with password authentication and `PermitRootLogin = "yes"`, as used on this LAN appliance. It opens the SSH firewall port and TCP 8080. The Go server listens on all interfaces on port 8080; keep network exposure within the intended trusted LAN. Chromium's CDP port 9222 is used locally and is not opened in the NixOS firewall. The config starts Chromium with `--remote-debugging-port=9222`; the Go server connects to `127.0.0.1:9222`.

## Kiosk VT ownership and rebuild safety (do not remove)

**Why this matters:** This appliance launches Sway directly through `html-display-sway.service` on `/dev/tty1`, rather than through a conventional display manager. The Sway unit explicitly `Conflicts=` with `getty@tty1.service`. The HTML Display Go server has `Requires=` and `PartOf=` relationships with Sway. If systemd starts the tty1 login prompt, it can stop Sway and consequently stop the Go server. During troubleshooting on October 21, 2026, `nixos-rebuild switch` reported starting `getty@tty1.service`, after which **both kiosk services were inactive** and the local display showed a text console.

**Required settings** in [configuration.nix](../config/nixos/configuration.nix):

```nix
# tty1 is exclusively owned by Sway.
systemd.services."getty@tty1".enable = false;
systemd.services."autovt@tty1".enable = false;

# Do not interrupt the kiosk for routine NixOS unit changes.
systemd.services.html-display-sway.restartIfChanged = false;
systemd.services.html-display-server.restartIfChanged = false;
```

Keep the Sway unit's `TTYPath = "/dev/tty1"` and `conflicts = [ "getty@tty1.service" ];` consistent with this policy. The active `graphical.target` and `multi-user.target` were not the cause; moving Sway between targets was **not** the verified solution. Disabling tty1 getty/autovt together with preserving both kiosk services was tested successfully with `nixos-rebuild switch`: Sway and server were both `active`, getty was `inactive`, and NixOS no longer started `getty@tty1.service`.

**Tradeoff:** `restartIfChanged = false` prevents routine NixOS activation from automatically restarting these units when their definitions change. Changes to the Sway unit, Chromium startup options, or server unit may require an explicit controlled restart (or reboot). It does not turn off their `serviceConfig.Restart = "always"` crash recovery. It also does not prevent stops requested by other systemd dependencies. Keep other virtual terminals or SSH available for administration.

**Regression check after changing NixOS services:**

```sh
sudo nixos-rebuild dry-activate
sudo nixos-rebuild switch
systemctl is-active html-display-sway html-display-server
systemctl is-active getty@tty1.service
```

Expected: `active`, `active`, `inactive`. Neither kiosk unit should be stopped during a routine switch, and getty should not start on tty1. If the console reappears, inspect `journalctl -b -u html-display-sway -u html-display-server -u getty@tty1` and the rebuild's activation messages before changing the target or dependency layout. A full reboot with these particular settings had not yet been reported as tested when this note was written.

## TV power and HDMI-CEC

The optional [CEC module](../config/nixos/modules/cec.nix) uses the Pulse-Eight USB-CEC Adapter v2, detected on this machine as `/dev/serial/by-id/usb-Pulse-Eight_USB-CEC_Adapter_v2_r8-if00`. It uses libCEC and does **not** require a kernel `/dev/cecX` device; the separate Intel DisplayPort CEC interface was detected but did not successfully communicate with the TV.

The module installs `display-tv` with commands `on`, `off`, `status`, `active`, `scan`, `volume-up`, `volume-down`, `mute`, and `volume-status`. For example:

```sh
display-tv off
display-tv on
display-tv status
```

The administrator needs membership in `dialout` for the USB serial device; the module adds that group to `admin`. Log out and back in after enabling it. The command is independent of the browser, REST server and Squeezelite; no TV endpoints have been added to the Go API.

A separate `display-tv-boot.service` sends `on 0` once during boot. It waits up to 20 seconds for the Pulse-Eight USB serial device to appear; if unavailable, the service fails and logs the reason without making the kiosk or audio services depend on it. Inspect with `systemctl status display-tv-boot.service` and `journalctl -b -u display-tv-boot.service`.

A systemd `system-sleep` hook sends `standby 0` before system suspend and `on 0` after resume, with a two-second post-resume delay and ten-second command timeout. This is a **best-effort policy**, not a way to wake the N100 from the TV. Power control via the Pulse-Eight adapter was manually reported working; the suspend/resume hook has **not yet been tested** on the appliance. Hooks run as root, and the timeout prevents an unresponsive adapter from indefinitely blocking the sleep transition. The hook does not act on reboot, shutdown, or Chromium restarts; boot-time power-on is handled by the separate service. Neither mechanism automatically powers down the TV at shutdown.

CEC volume status may return `7f` (unknown) when the TV uses built-in speakers, as observed here. The intended audio design is to keep the TV's speaker volume at a fixed level and adjust Linux/PipeWire audio output instead. CEC volume commands remain available as optional controls.

To apply, install the [checked-in machine config](../config/nixos/configuration.nix) at `/etc/nixos/configuration.nix`, the CEC module at `/etc/nixos/modules/cec.nix`, and run `sudo nixos-rebuild switch`. Check the USB device symlink on other machines and change `cecDevice` accordingly. Test `display-tv off` / `display-tv on`, then `sudo systemctl start display-tv-boot.service` to test boot-time wake without rebooting. Test `systemctl suspend` separately. The boot service has not yet been tested on the appliance.

## Build and startup

Build the Go 1.24 module:

```sh
CGO_ENABLED=0 go build -o html-display-server .
```

Install that binary at `/opt/html-display/html-display-server` as expected by the checked-in unit. The unit uses `StateDirectory = "html-display";` to manage `/var/lib/html-display`, which holds the latest uploaded HTML document. It requires and starts after `html-display-sway.service`, and a Sway restart also restarts the controller. The Go process still exits if CDP or the kiosk page target is not ready when it first connects; systemd's restart policy retries the process. After the HTTP listener starts, the controller waits three seconds to allow a retained MQTT URL to arrive, then displays the default landing page only if Chromium is still on the black startup page. Existing remote content is not overwritten on a server restart. The systemd dependency relationships and tty1 protection remain unchanged.

Sway starts Chromium at `file:///etc/html-display/startup.html`, a locally available black page, in kiosk mode with XWayland (`--ozone-platform=x11`) and a volatile profile under `/run/html-display/chromium`. It hides the pointer after two seconds of inactivity. Chromium uses `--autoplay-policy=no-user-gesture-required`; PipeWire/WirePlumber selects the tested card's HDMI stereo profile and prefers HDMI/DisplayPort sinks. The notes for this machine report working browser audio and N100 hardware decoding for H.264 and AV1. Those observations depend on the connected hardware and were not re-tested as part of this documentation change.

`system.stateVersion = "26.05"` is the installation compatibility setting. The machine reported NixOS 26.05.10529.c508844df6c2 during the audio setup; do not change stateVersion during normal upgrades.

## Music Assistant and audio

Squeezelite runs as a separate `display` systemd user service using `pkgs.squeezelite-pulse`, sharing Chromium's per-user PipeWire instance. It has no lifecycle dependency on Sway or the Go controller. `users.users.display.linger = true` starts user services at boot and keeps them alive without a login; membership in `audio` supplies sound-device access without an active kiosk seat session. The stock NixOS Squeezelite PulseAudio service requires system-wide audio, so this setup uses a custom user unit.

In Music Assistant, add the Squeezelite player provider. The player connects explicitly to `192.168.10.12:3483` and is named `html-display`; change this machine-specific server address and port for your network. Port 3483 is the provider's Slimproto port, not the Music Assistant web interface port. No separate LMS/Lyrion installation is needed. Automatic discovery did not initially find the player; explicit `-s` resolved the connection. Following reboot, playback is available once Music Assistant detects the player again.

The tested card is `alsa_card.pci-0000_00_1f.3`, with profile `output:hdmi-stereo` and sink `alsa_output.pci-0000_00_1f.3.hdmi-stereo`. These identifiers appear in the profile preference and role output configuration; review both when changing hardware or HDMI connectors. Sink `priority.session` alone did not select the HDMI card profile. Profile restoration alone also did not retain HDMI across reboot. Explicit profile preference with restoration disabled did.

The [audio policy](audio.md) makes Chromium higher priority than Squeezelite using two virtual role sinks. Music Assistant keeps playing while external sound is suppressed. Music may take multiple seconds to return after browser playback pauses; navigation to a different page released it faster. mpv takeover is still planned.

### Applying the configuration

Install the configuration at the path actually used by the rebuild, normally `/etc/nixos/configuration.nix`, together with the existing machine-generated hardware configuration. Uploading it to another directory does not change the running configuration.

```sh
sudo nixos-rebuild switch
sudo reboot
```

Reboot after first adding the audio group so the user manager picks up membership. Later role-policy edits can be applied by restarting WirePlumber, pipewire-pulse, and Squeezelite; recreate Chromium playback streams too. Audio service restarts interrupt browser audio.

### Diagnostics

Run audio commands against the **display** session, even when logged in as admin or root. A bare `wpctl status` from admin inspects admin's separate instance and can start that instance through socket activation.

```sh
display_uid=$(id -u display)
sudo -u display env \
  XDG_RUNTIME_DIR="/run/user/$display_uid" \
  wpctl status

sudo -u display env \
  XDG_RUNTIME_DIR="/run/user/$display_uid" \
  DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/$display_uid/bus" \
  systemctl --user status squeezelite pipewire-pulse wireplumber --no-pager -l

sudo journalctl _UID="$display_uid" -b \
  _SYSTEMD_USER_UNIT=squeezelite.service \
  _SYSTEMD_USER_UNIT=wireplumber.service \
  _SYSTEMD_USER_UNIT=pipewire-pulse.service \
  -n 100 --no-pager
```

Expected routing is Squeezelite → External role sink → HDMI and Chromium → Browser role sink → HDMI. Use current IDs from `wpctl status` for `wpctl inspect ID`; IDs change across restarts. To inspect card profiles, run `pw-cli enum-params CARD_ID EnumProfile` with the same display runtime environment.

- **Dummy Output / no card:** check `id display`, device permissions under `/dev/snd`, session state, and WirePlumber logs. Lingering alone does not grant hardware access.
- **Analog output:** inspect card profiles and confirm the deployed profile preference matches the card. The manual test used `wpctl set-profile CARD_ID 4`; index 4 was HDMI on this device, not a portable constant.
- **Player absent in Music Assistant:** check Squeezelite service/logs, the explicit server address, and provider port. Audio-output initialization failure can prevent the player from connecting.
- **WirePlumber dependency error mentioning quoted support.export-core:** retain the raw SPA-JSON component fragment in `configPackages`. After correcting it, use `systemctl --user reset-failed wireplumber.service squeezelite.service` in the display session before restarting.
- **Delayed return of music:** compare browser pause with navigation to a silent page. The observed difference suggests retained browser stream lifetime; changing Music Assistant transport does not solve activity detection.

Validate independent playback after reboot, Browser → External recovery, short effects, multiple browser streams, and deliberate kiosk restart separately. Only the observations listed in [audio](audio.md) have been reported as tested.

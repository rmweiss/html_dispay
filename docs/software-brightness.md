# Software brightness (wl-gammarelay-rs)

Software brightness is independent of the Samsung TV backlight and `html-display-tv`. It adjusts Sway's output gamma ramps, not physical backlight power.

## Runtime / NixOS

The Sway config launches `wl-gammarelay-rs` independently of Chromium and the Go server. The relay owns the Wayland gamma-control connection and exposes the writable D-Bus property `rs.wl-gammarelay` at `/`, interface `rs.wl.gammarelay`, property `Brightness` (double, 0.0–1.0).

The NixOS `html-display-server` service enables the backend using `-brightness-backend ${pkgs.systemd}/bin/busctl`. The flag is a path to **busctl**, not to the relay or Gammastep. With no flag (for example in local development) software brightness is disabled and REST returns 503.

The server runs as the `display` user and talks to that user's D-Bus session bus at `/run/user/<uid>/bus` via `DBUS_SESSION_BUS_ADDRESS` if no address is supplied. No Wayland socket discovery is needed in the server itself.

## REST

- `GET /api/display/brightness` → `{"brightness":25}` (saved desired value)
- `PUT /api/display/brightness` with `{"brightness":25}` → 204 on successfully applied setting

Values are integers **10–100**, corresponding to relay brightness 0.10–1.00. 100 is no software dimming. The floor is intended to avoid making the display fully invisible.

```sh
curl -X PUT http://html-display:8080/api/display/brightness \
  -H 'Content-Type: application/json' -d '{"brightness":25}'
```

## MQTT / Home Assistant

The existing `homeassistant/device/html-display/config` discovery includes a `number` component called **Software Brightness**:

- Command: `html-display/display/brightness/set` (plain integer)
- State: `html-display/display/brightness/state` (retained plain integer)

Invalid commands are ignored. An MQTT state message is published after a successful D-Bus write and on MQTT connection. State published at connection reflects the saved desired brightness, which may temporarily differ from actual gamma ramps if the relay has not started yet.

The last successfully applied brightness is persisted as `/var/lib/html-display/software-brightness.json`. The Go server checks the relay brightness every 10 seconds and reapplies its saved setting when the relay has started or restarted. D-Bus failures are logged; the server stays operational, REST writes report an error and MQTT writes do not acknowledge success.

## Manual diagnostics

Run the following as the `display` user with its user bus available:

```sh
busctl --user get-property rs.wl-gammarelay / rs.wl.gammarelay Brightness
busctl --user set-property rs.wl-gammarelay / rs.wl.gammarelay Brightness d 0.25
```

Inspect the relay process with `pgrep -af wl-gammarelay-rs` and the server logs with `journalctl -u html-display-server -b`. Do not run a separate Gammastep process concurrently; only one gamma-ramp controller should own a Sway output.

Because the relay stays running, changes do not cause a gamma-controller teardown and should avoid the normal-brightness flashes from the former Gammastep implementation. Changes have not yet been validated on the target appliance.

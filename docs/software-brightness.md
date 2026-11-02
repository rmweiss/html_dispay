# Software brightness (Gammastep)

Software brightness is independent of the Samsung TV backlight and `html-display-tv`. It adjusts the Sway output's gamma ramps, not physical backlight power.

The NixOS `html-display-server` service enables the backend using `-brightness-backend ${pkgs.gammastep}/bin/gammastep`. With no flag (e.g. local development), brightness is disabled and the REST endpoint returns 503.

## REST

- `GET /api/display/brightness` → `{"brightness":25}`
- `PUT /api/display/brightness` with `{"brightness":25}` → 204

Values are integers **10–100**, corresponding to Gammastep brightness 0.10–1.00. 100 means no software dimming. The 10% floor prevents setting an unexpectedly invisible output.

```sh
curl -X PUT http://html-display:8080/api/display/brightness \
  -H 'Content-Type: application/json' -d '{"brightness":25}'
```

## MQTT / Home Assistant

The existing `homeassistant/device/html-display/config` device discovery gains a `number` component called **Software Brightness**:

- Command: `html-display/display/brightness/set` (plain integer)
- State: `html-display/display/brightness/state` (retained plain integer)

Command input is range-checked; only successfully applied values are published as state. State is also published when MQTT connects. Commands are not retained by the server.

The last applied setting is saved atomically as `/var/lib/html-display/software-brightness.json` and applied again at startup. On failure to find the Wayland display or apply Gammastep, the server logs the error and remains operational; REST errors are reported and MQTT does not publish a falsely successful state.

The backend detects a Wayland socket in `/run/user/<display UID>/wayland-*` when `WAYLAND_DISPLAY` is not set, and operates as the `display` user.

## Notes

Software dimming does not save backlight energy, and is intentionally separate from the TV backlight entity. This is a gamma-ramp operation affecting the whole Sway output, not Chromium alone. Only one gamma-ramp manager should own the display; do not run other Gammastep instances concurrently. If the compositor restarts, the service restarts with it and re-applies the stored value.

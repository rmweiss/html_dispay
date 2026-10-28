# Samsung Frame TV controller

The N100 drives a Samsung Frame LS03B through a Prolific USB-to-serial ExLink cable. The Go daemon owns the serial port and exposes a Unix domain socket at `/run/html-display-tv/control.sock`, while connecting to the shared MQTT broker (default `tcp://192.168.10.12:1883`).

## Local commands

```sh
tvctl status
tvctl power on
tvctl power off
tvctl volume get
tvctl volume set 25
tvctl backlight get
tvctl backlight set 10
tvctl backlight set 50
tvctl osd get
tvctl osd off
tvctl osd on
tvctl raw 0E 01 00 00
tvctl input get
tvctl input set HDMI4
```

ExLink command acknowledgements and, where available, state readbacks are returned synchronously. The daemon serializes ExLink access. Input selection is verified with bounded readback attempts.

## Home Assistant discovery

The daemon advertises:
- Power switch
- Volume number **box** (0–100)
- Backlight number **box** (0–50)
- Mute binary sensor (read-only)
- HDMI input **select** (HDMI1–HDMI4)
- Command-result OSD **switch** (ON/OFF, last commanded state only)

Topics use `html-display/tv/<property>/set` and `html-display/tv/<property>/state`; availability uses `html-display/tv/availability`.

The old read-only input sensor discovery configuration is cleared on startup.

## Deployment

Import `config/nixos/modules/tv.nix` in `configuration.nix`. Enable `services.htmlDisplayTV.enable = true;` and configure `mqttBroker`. Keep `lifecycle.enable = false` while HDMI-CEC hooks are active. Once CEC hooks are removed, enable lifecycle handling separately.

The USB adapter path is `/dev/serial/by-id/usb-Prolific_Technology_Inc._USB-Serial_Controller_D-if00-port0`.

For production: the lightweight MQTT client supports QoS 0 only and has no broker authentication/TLS. Ensure MQTT is on a trusted network. Backlight read/write, input switching, and controls were tested on the LS03B.

## Command-result OSD

The NixOS module defaults to `services.htmlDisplayTV.osdOnStartup = "off";`.
Available values are `"on"`, `"off"` and `"keep"`. The last setting sends
no command during controller startup. A manually issued `tvctl osd on` or
`tvctl osd off` overrides the startup policy for that running session.

OSD commands are **ACK-confirmed, not independently queryable**. The command
`tvctl osd get` returns the last acknowledged value seen by the daemon, or
`UNKNOWN`. MQTT publishes successful writes as retained
`html-display/tv/osd/state` with payload `ON` or `OFF`; retained state can
become stale after power cycles or external changes.

The verified command-result OSD frames are:

- Off: `08 22 0E 01 00 00 C7`
- On: `08 22 0E 01 00 01 C6`

The N100's Samsung Frame LS03B was tested with both OSD states. Turning command
OSD off suppressed the overlays on ExLink volume and backlight adjustments.

## Experimental raw ExLink commands

The local Unix-socket CLI can transmit raw ExLink commands without stopping
the service. It supports a four-byte command payload (`tvctl raw 0E 01 00 00`),
six bytes with header but no checksum, or a complete checksummed seven-byte
frame. Full frames are validated before transmission; the response includes
the TV's raw ACK/response. Raw commands are deliberately **not** exposed
through MQTT, because they may modify persistent TV settings.

Backlight writes and Home Assistant discovery now support 0–50. An automatic
brightness policy may use a narrower range independently.

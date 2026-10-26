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
tvctl input get
tvctl input set HDMI4
```

ExLink command acknowledgements and, where available, state readbacks are returned synchronously. The daemon serializes ExLink access. Input selection is verified with bounded readback attempts.

## Home Assistant discovery

The daemon advertises:
- Power switch
- Volume number **box** (0–100)
- Backlight number **box** (0–20)
- Mute binary sensor (read-only)
- HDMI input **select** (HDMI1–HDMI4)

Topics use `html-display/tv/<property>/set` and `html-display/tv/<property>/state`; availability uses `html-display/tv/availability`.

The old read-only input sensor discovery configuration is cleared on startup.

## Deployment

Import `config/nixos/modules/tv.nix` in `configuration.nix`. Enable `services.htmlDisplayTV.enable = true;` and configure `mqttBroker`. Keep `lifecycle.enable = false` while HDMI-CEC hooks are active. Once CEC hooks are removed, enable lifecycle handling separately.

The USB adapter path is `/dev/serial/by-id/usb-Prolific_Technology_Inc._USB-Serial_Controller_D-if00-port0`.

For production: the lightweight MQTT client supports QoS 0 only and has no broker authentication/TLS. Ensure MQTT is on a trusted network. Backlight read/write, input switching, and controls were tested on the LS03B.

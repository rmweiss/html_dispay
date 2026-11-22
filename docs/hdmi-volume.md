# HDMI master volume (PipeWire)

The HTML Display server can expose the **mini PC's HDMI output volume** as an
MQTT-discovered Home Assistant number entity. This is distinct from the Samsung
Frame's ExLink hardware volume and from individual Bluetooth, Chromium, or
Squeezelite stream volumes.

NixOS enables it by passing `-volume-backend ${pkgs.wireplumber}/bin/wpctl`
to `html-display-server`. It runs in the existing `display` user session.
The controller verifies that `@DEFAULT_AUDIO_SINK@` is the expected
`alsa_output.pci-0000_00_1f.3.hdmi-stereo` before reading or changing it,
rather than depending on a transient PipeWire object ID.

Home Assistant discovery adds **HDMI Volume** under the HTML Display device,
using:
- Command topic: `html-display/audio/volume/set` (integer 0–100, not retained)
- State topic: `html-display/audio/volume/state` (retained percentage)
- Discovery topic: `homeassistant/device/html-display/config` (existing)

The real PipeWire value is read on MQTT connection, after each volume command,
and every three seconds to detect changes from other tools. No separate stored
volume is used, and changing audio devices does not silently change another sink.
If the HDMI output is unavailable, commands fail safely and errors are logged.

## Validation

On the appliance after deployment:

```sh
uid=$(id -u display)
sudo -u display env XDG_RUNTIME_DIR="/run/user/$uid" wpctl status
sudo -u display env XDG_RUNTIME_DIR="/run/user/$uid" wpctl get-volume @DEFAULT_AUDIO_SINK@
```

Change HDMI Volume in Home Assistant, confirm the PipeWire value changes, and
then change the volume with `wpctl` and confirm Home Assistant updates within
a few seconds. Test while playing Bluetooth, Squeezelite and Chromium audio.
If the HDMI device name changes, update `hdmiSinkName` in `volume.go`.

The feature has not yet been tested on the physical device.

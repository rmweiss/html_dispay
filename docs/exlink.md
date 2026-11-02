# Samsung Frame LS03B — ExLink command reference

Practical reference for **Samsung The Frame LS03B (2022)** with the HTML Display N100 appliance. Collected from working ExLink tests, the controller source, a publicly examined third-party SRS-2 firmware, and query scans. **Not an official Samsung protocol specification.** Commands marked *observed* were tested on the appliance; other commands are marked *firmware-derived* or *unconfirmed*. See [backlight and deep-standby behavior](brightness.md) for a separate fault study.

## Connection and frame format

- Transport: Prolific USB-serial adapter, **9600 baud, 8 data bits, no parity, 1 stop bit**, no flow control.
- Typical port: `/dev/ttyUSB0`; stable device path `/dev/serial/by-id/usb-Prolific_Technology_Inc._USB-Serial_Controller_D-if00-port0`.
- Send **7-byte** frames: `08 22 AA BB CC DD KK`, with `KK = (-sum(08,22,AA,BB,CC,DD)) & 0xFF`, so all seven bytes sum to zero modulo 256.
- Example: power status `08 22 F0 00 00 00 E6`.
- Typical three-byte acknowledgment `03 0C F1`; a `03 0C FF` marker can indicate rejection. An ACK is **not** proof that visible hardware state changed.
- A valid read-data reply was observed in a 13-byte form like `03 0C F5 08 F0 ID 00 00 F1 VALUE 00 00 CHECKSUM`. The full reply checksum sums to zero modulo 256. Some queries return a status code `FF` in place of `F1` or only an ACK.

All serial access should pass through the **single owner** `html-display-tv` daemon while it is running. Do not independently open the port from a test script without first stopping the daemon.

## Tested commands (hex; KK means computed checksum)

| Operation | Four command bytes (after 08 22) | Full example / notes | Evidence |
| --- | --- | --- | --- |
| Power ON | `00 00 00 02` | `08 22 00 00 00 02 D4` | Observed |
| Power OFF | `00 00 00 01` | `08 22 00 00 00 01 D5` | Observed |
| Read power | `F0 00 00 00` | `08 22 F0 00 00 00 E6` | Observed |
| Set volume | `01 00 00 VV` | 0–100 as enforced by `tvctl` | Observed |
| Read volume | `F0 01 00 00` | Returns value | Observed |
| Set backlight | `0B 01 00 VV` | **0–50**, not the picture black-level setting | Observed |
| Read backlight | `F0 0A 00 00` | `08 22 F0 0A 00 00 DC` | Observed |
| Read picture brightness | `F0 0C 00 00` | `08 22 F0 0C 00 00 DA` | Observed |
| Set picture brightness | `0B 03 00 VV` | Different from backlight | Identified; less thoroughly exercised |
| Read input | `F0 04 00 00` | HDMI4 observed as `4A` | Observed |
| Select HDMI 1–4 | `0A 00 05 VV` | VV = `00`…`03` for HDMI1…4 | Observed |
| Read mute | `F0 02 00 00` | Typically 0/1 | Observed through controller |
| Command-result OSD OFF | `0E 01 00 00` | `08 22 0E 01 00 00 C7` | **Observed suppression** |
| Command-result OSD ON | `0E 01 00 01` | `08 22 0E 01 00 01 C6` | Observed |

The command-result OSD policy suppresses the popup normally shown for ExLink setting commands. The OSD command itself can still display a notification. The TV may lose the setting after a full wake. No reliable readback command was found; the daemon's `osd get` exposes **last acknowledged command in this daemon session**, *not actual TV OSD state*.

## Additional known or firmware-derived commands

These were identified in the protocol exploration and/or SRS-2 firmware; **do not assume all were independently verified on the LS03B**:

| Operation | Four command bytes | Note |
| --- | --- | --- |
| Relative volume UP | `01 00 01 00` | Firmware / protocol reference |
| Relative volume DOWN | `01 00 02 00` | Firmware / protocol reference |
| Toggle mute | `02 00 00 00` | Toggle, not absolute state |
| General OSD hide | `0E 00 00 01` | Different from command-result OSD |
| General OSD show | `0E 00 00 00` | Different from command-result OSD |
| Eco sensor OFF | `0B 0B 0D 00` | `08 22 0B 0B 0D 00 B3` |
| Eco sensor ON | `0B 0B 0D 01` | `08 22 0B 0B 0D 01 B2` |
| IR remote key forwarding | `0D 00 00 KK` | KK is IR-key code, **not** frame checksum |
| Query Eco sensor | `F0 25 00 00` | Returned `00` in a test |
| Query minimum backlight | `F0 24 00 00` | Can return unsupported/error |

The earlier firmware-derived OSD suppression and brightness range were validated by running a two-state OSD/backlight test on the actual TV. No mechanism to query OSD suppression state was verified.

## Known status-query IDs

Read using four payload bytes `F0 ID 00 00`; `tvctl raw F0 ID 00 00` automatically calculates frame checksum.

| ID | Meaning | ID | Meaning |
| --- | --- | --- | --- |
| `00` | Power | `01` | Volume |
| `02` | Mute | `03` | Channel |
| `04` | Input | `05` | Picture size |
| `06` | 3D mode | `07` | Picture mode |
| `08` | Sound mode | `0A` | Backlight |
| `0B` | Contrast | `0C` | Picture brightness |
| `24` | Minimum backlight | `25` | Eco sensor |

These names come from an older published ExLink query table and field inspection. Other query IDs up to `3A` exist in that historical table but are not fully enumerated here; the scan also obtained responses for `3B` and `3C`, whose meanings remain unknown.

**OSD status scan:** The 59 IDs `00`–`3A` produced no meaningful OSD-dependent value (two initial missing responses were attributed to startup/timing). A later full **256-ID** `00`–`FF` scan produced **identical ON/OFF replies for every ID**: 49 value frames, 8 unusual-marker frames, and 199 ACK-only. Thus no OSD state query was found in the simple `F0 ID 00 00` command family. This does not exclude other command families.

**Brightness-fault comparison:** A separate **64-ID** scan `00`–`3F` produced identical responses before and after a successful power-cycle fix for frozen physical brightness. No queried register revealed that fault.

## Using the existing controller

The Go daemon `html-display-tv` owns the ExLink serial device and exposes a Unix socket at `/run/html-display-tv/control.sock`. The CLI is `tvctl`, independent of Chromium, kiosk server and Sway.

```sh
tvctl status
tvctl power get
tvctl power on
tvctl power off
tvctl backlight get
tvctl backlight set 20
tvctl volume get
tvctl volume set 50
tvctl mute get
tvctl input get
tvctl input set HDMI4
tvctl osd get
tvctl osd off
tvctl raw F0 0A 00 00   # read backlight (raw reply hex)
tvctl raw 08 22 F0 00 00 00 E6
```

`tvctl raw` accepts four command bytes, six frame bytes without checksum, or a complete valid seven-byte frame; its decoder verifies checksum and serializes access through the daemon. Use raw commands carefully: some commands change TV settings. Status queries do not.

Home Assistant entities are published via MQTT discovery (power switch, volume and backlight number entities, HDMI selector, OSD switch, mute state). Topic pattern: `html-display/tv/<property>/set` and `html-display/tv/<property>/state`; MQTT broker and other defaults are configurable in [the NixOS module](../config/nixos/modules/tv.nix). Raw ExLink access is **deliberately not exposed over MQTT**.

The NixOS module is `config/nixos/modules/tv.nix`, with `services.htmlDisplayTV.enable`, `lifecycle.enable`, `serialDevice`, `mqttBroker`, and `osdOnStartup = "off" | "on" | "keep"`. Startup OSD OFF is policy-based; it is not TV-state readback. If lifecycle is enabled, systemd uses ExLink for TV power at boot/shutdown and around N100 suspend/resume. Do not simultaneously enable conflicting CEC lifecycle control without deliberate coordination.

## Safety and limitations

- A valid `F0` query readback indicates a stored TV setting, not actual panel luminance or backlight-driver state.
- ExLink power responses can be absent or delayed during standby/initialization. For the LS03B, power readback `05` has been treated as ON and `04` / `08` as OFF by the controller; these mappings may differ on other Samsung models.
- HDMI4 readback `4A` was seen in testing; the controller also maps `3C` to HDMI4.
- A quick ExLink OFF, wait about 3 s, ON **did fix** a frozen backlight after a Knox-logo deep wake, but should not be repeated blindly.
- The TV's automatic Brightness Optimisation / Eco controls are separate potential influences. Do not infer all adaptive functions from `F0 25` alone.
- Direct Python serial experiments require stopping `html-display-tv` first; `tvctl raw` is the preferred safe path while the daemon runs.
- DDC/CI at I²C address 0x37 did not respond in the earlier test; ExLink is the confirmed working transport.

## Source/maintenance notes

Code and behavior: [TV module](../config/nixos/modules/tv.nix), [tvctl command implementation](../config/nixos/modules/html-display-tv-src/cmd/tvctl/main.go), [ExLink transport](../config/nixos/modules/html-display-tv-src/internal/exlink/exlink.go), and [controller](../config/nixos/modules/html-display-tv-src/internal/control/control.go). Observations include local OSD toggling tests, full OSD query comparisons, and frozen-brightness before/after snapshots performed in October–November 2026.

When adding a command, record its four payload bytes, resulting checksummed seven-byte frame, whether the user **tested** it on this specific TV, and whether its readback reflects a setting or an independently observed physical effect.

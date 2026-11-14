# Audio architecture

> **Status:** Browser > External suppression was validated in October 2026. On 2026-11-14, MPV > External takeover and automatic restoration were also verified on the N100 using explicit MPV PulseAudio role/sink routing. A standalone MPV controller, MQTT state reporting and media-player API are not yet implemented.

HTML Display hosts the Chromium kiosk; it does not interpret Scene Composer scenes or mix their sounds. Audio has two nested scopes: Scene Composer mixes sounds *within its browser player*, while PipeWire/WirePlumber is intended to arbitrate *between system audio producers*.

## Scene audio stays inside Chromium

The [Scene Composer audio design](https://github.com/rmweiss/scene_composer/blob/main/docs/audio.md) has an implemented browser mixer and effect-trigger API. Ducking and video soundtrack routing remain future work. It uses one shared browser Web Audio `AudioContext`: source gains feed background and effects buses, then a master gain. Loops, overlapping effects, future ducking/fades, and any future routed video audio belong to that graph. Scene Composer's server owns desired scene state and effect commands; each display's browser produces its own sound, without synchronized playback clocks.

The effect trigger belongs to **Scene Composer**, not HTML Display:

```http
POST /api/scenes/{scene}/instances/{instance}/audio/effects/{effect}/play
Content-Type: application/json

{"volume": 0.8}
```

Acceptance does not mean playback finished or was heard. On reconnect, loops start from the beginning using desired state; a scene reset restores authored defaults. Missing media should produce diagnostics without breaking visuals. Scene Composer calls for an Enable audio control if autoplay is blocked; the unattended kiosk currently uses `--autoplay-policy=no-user-gesture-required`.

Web Audio bus names and gains do **not** become PipeWire roles. At the system boundary, all Chromium playback streams should receive the **Browser** class, even if Chromium exposes several streams. Scene Composer must duck its own background bus when an effect calls for it; WirePlumber sees only Chromium's output stream(s), not individual scene sounds. Arbitrary uploaded HTML can also produce browser audio if its code and assets permit it. See [usage](usage.md) and [deployment](deployment.md).

## System audio priority

The intended appliance policy is:

| Priority | Producer | Behavior |
| --- | --- | --- |
| Highest: Takeover | Supervised mpv playback | Sole audible class while actively producing audio; video also occupies the foreground screen |
| Middle: Browser | Chromium, including Scene Composer and other pages | Audible when no takeover audio is active; suppresses external audio while producing audio |
| Lowest: External | Independent players such as Squeezelite | Audible beside a silent dashboard, slideshow, or other silent page |

Only the highest class **currently producing audio** should be audible. A running Chromium process or a silent page alone must not suppress external audio. Lower-class producers may continue playing while inaudible and become audible again automatically when higher-class audio stops. This is the desired behavior. The current two-class implementation suppresses Squeezelite without pausing Music Assistant; playback position continues advancing. Its activity decision follows stream/link lifetime rather than measuring actual sound level. An external player can survive browser navigation because it is independent of the page. Its own service or controller owns its playlist, transport, and volume; HTML Display does not need an `/api/audio` endpoint for Squeezelite.

This is an application-level policy, separate from Scene Composer's internal background/effects mixer. An mpv takeover must suppress both browser and external audio. An audio-only mpv item, if supported, keeps Chromium visible while retaining the highest audio priority. See the [mpv concept](mpv-takeover-concept.md) for its separate process/API lifecycle.

## Implemented WirePlumber role-based priority

The [NixOS configuration](../config/nixos/configuration.nix) creates three WirePlumber loopback sinks: `html-display-external` (priority 10), `html-display-browser` (priority 20), and `html-display-takeover` (priority 30). Both output to the tested HDMI sink. PipeWire PulseAudio `stream.rules` classify playback streams by the observed `application.process.binary`: `squeezelite-pulse` gets role External, and `chromium` gets role Browser. Matching `Stream/Output/Audio` avoids treating Chromium's input client as browser playback.

Each stream explicitly targets its role sink. This matters because the observed Squeezelite stream explicitly targeted the physical HDMI sink; direct routing would bypass the role policy. Loopback playback streams use the unused Loopback role and passive output links. Each higher role applies `duck` with `linking.role-based.duck-level = 0.0`, making external playback inaudible without changing Music Assistant's transport or the shared hardware sink volume.

The role component fragment is supplied through `wireplumber.configPackages` as raw SPA-JSON. On WirePlumber 0.5.14, supplying component `requires` through Nix-generated JSON caused a fatal dependency error containing literal quotation marks around `support.export-core`. Preserve the raw fragment's unquoted dependency tokens; do not move these components into JSON-backed `extraConfig`.

### Reported validation and limits

The appliance reported NixOS 26.05.10529.c508844df6c2, PipeWire 1.6.6, WirePlumber 0.5.14, and Squeezelite 2.0.0-1563. Independent Music Assistant playback and HDMI profile selection survived reboot. After the corrected role configuration was applied, browser playback suppressed external audio, Music Assistant's playback position continued advancing, and external audio returned when browser playback stopped.

Return after pausing browser audio took multiple seconds. Navigating to another page restored music much faster. This is consistent with Chromium retaining its stream after pause; it is not proof of the exact cause. The current policy does not detect sample-level silence, so an open silent stream can retain priority. No idle-stream tuning or sample-level detector has been implemented.

Post-reboot priority behavior, short Web Audio effects, multiple browser streams, and kiosk failure/restart behavior still need explicit validation. Service independence is configured; surviving a deliberate Sway restart was not reported as tested. MPV is installed, and the Takeover sink exists. MPV is not automatically classified through the existing `pipewire-pulse` stream rules: manual testing showed its stream directed to the physical HDMI sink without a Takeover role when launched with just `--ao=pulse`. Explicit role and sink selection is required.

### Validated MPV Takeover routing (2026-11-14)

On the N100, starting MPV with `PULSE_PROP='media.role=Takeover'`, `--ao=pulse`, and `--audio-device=pulse/html-display-takeover` produced the expected PipeWire graph: MPV in Takeover (priority 30) and Squeezelite in External (priority 10). Only MPV was audible during video playback; Squeezelite continued playing in Music Assistant and became audible again as soon as MPV exited. This verifies the **MPV > External** path; MPV suppression of simultaneously active Chromium audio and pause/crash behavior still need explicit testing.

The NixOS configuration installs a minimal `html-display-mpv` wrapper to preserve this **working audio baseline**, with no fixed renderer, hardware decoder, YouTube format, caching, fullscreen, or playback-length options:

```sh
html-display-mpv --no-config --fullscreen --keep-open=no \
  --hwdec=vaapi --vo=gpu --gpu-api=opengl \
  'https://www.youtube.com/watch?v=nfsEFdA76mQ'
```

The wrapper sets the `Takeover` role, selects MPV's PulseAudio backend and the `html-display-takeover` sink, then passes all user-supplied MPV arguments through unchanged. It runs as a normal command, **not** a background player service; it must be invoked inside the `display` user's audio and graphical session. The future standalone controller can invoke this wrapper or reproduce its audio settings, while making all other MPV options configurable. Do not assume invoking plain `mpv` will apply the desired priority. Changing audio backend/sink flags may intentionally bypass the tested baseline.

During the successful takeover test, MPV logged one startup audio underrun and 905 dropped frames over 90 seconds; no obvious stuttering was noticed, but video was not closely watched. Earlier native PipeWire/OpenGL testing showed better reported frame delivery. Performance under PulseAudio therefore remains a tuning task, not a reason to alter the tested priority topology prematurely.

### Why suppression rather than Music Assistant pause

Music Assistant remains responsible for playlist, transport, and player volume. Local suppression is the chosen default for background music: it removes an audio/API dependency on the remote server and lets playback continue during interruptions. Pause/resume could later be useful for podcasts or audiobooks, but would require reliable browser activity detection, manual-control ownership tracking, and a decision about group playback. It is not currently implemented.

See [deployment](deployment.md#music-assistant-and-audio) for setup and diagnostics. Future MPV integration must still test Browser → MPV → Browser and all three together, plus explicit pause/stop/crash recovery.

Upstream references: [role policy](https://pipewire.pages.freedesktop.org/wireplumber/daemon/configuration/features.html), [WirePlumber 0.5.14 role-sink example](https://github.com/PipeWire/wireplumber/blob/0.5.14/src/config/wireplumber.conf.d.examples/media-role-nodes.conf), and [PipeWire PulseAudio stream rules](https://docs.pipewire.org/page_module_protocol_pulse.html).

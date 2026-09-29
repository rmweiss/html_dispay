# Audio architecture (planned)

> **Status:** The audio priority policy, external player integration, and mpv takeover are planned, not implemented. The reported N100 setup plays Chromium audio through PipeWire to HDMI/DisplayPort. No HTML Display audio upload or playback API exists today.

HTML Display hosts the Chromium kiosk; it does not interpret Scene Composer scenes or mix their sounds. Audio has two nested scopes: Scene Composer mixes sounds *within its browser player*, while PipeWire/WirePlumber is intended to arbitrate *between system audio producers*.

## Scene audio stays inside Chromium

The [Scene Composer audio design](https://github.com/rmweiss/scene_composer/blob/main/docs/audio.md) is unimplemented. It specifies one shared browser Web Audio `AudioContext`: source gains feed background and effects buses, then a master gain. Loops, overlapping effects, future ducking/fades, and any future routed video audio belong to that graph. Scene Composer's server owns desired scene state and effect commands; each display's browser produces its own sound, without synchronized playback clocks.

The proposed effect trigger belongs to **Scene Composer**, not HTML Display:

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

Only the highest class **currently producing audio** should be audible. A running Chromium process or a silent page alone must not suppress external audio. Lower-class producers may continue playing while inaudible and become audible again automatically when higher-class audio stops. This is the desired behavior; stream-activity detection and whether suppression advances or pauses an external player's transport require device testing. An external player can survive browser navigation because it is independent of the page. Its own service or controller owns its playlist, transport, and volume; HTML Display does not need an `/api/audio` endpoint for Squeezelite.

This is an application-level policy, separate from Scene Composer's internal background/effects mixer. An mpv takeover must suppress both browser and external audio. An audio-only mpv item, if supported, keeps Chromium visible while retaining the highest audio priority. See the [mpv concept](mpv-takeover-concept.md) for its separate process/API lifecycle.

## PipeWire/WirePlumber implementation candidate

Evaluate WirePlumber 0.5's optional [`policy.linking.role-based`](https://pipewire.pages.freedesktop.org/wireplumber/daemon/configuration/features.html) first. It groups streams by `media.role` behind virtual role sinks and can duck or cork lower-priority roles when a higher role stream is active. Its [example role-sink configuration](https://pipewire.pages.freedesktop.org/wireplumber/daemon/configuration/example_fragments.html) is a starting point, not a tested N100 configuration. `priority.session` selects a preferred sink; it does not arbitrate audibility between applications. The role policy is designed for automotive/mobile use, and its desktop behavior must be validated here.

Assign mpv, **every** Chromium playback stream, and Squeezelite to distinct roles using their observed stream properties and an appropriate rule at the client or session-manager boundary. All three must output through PipeWire, including a Squeezelite ALSA/PulseAudio path if used; direct access to the HDMI ALSA hardware would bypass the policy. Do not assume a process name or a single Chromium stream before inspecting the live graph.

Test `cork` against zero-level `duck` (`linking.role-based.duck-level = 0.0`). The aim is complete inaudibility of lower roles while their playback time continues; whether either mechanism achieves that with the chosen player and Chromium needs measurement. Also check that a present but silent/idle Chromium stream releases the external role promptly; stream existence alone is not a sufficient activity test. If stock policy fails these cases, use a small WirePlumber customization or controller based on actual stream state. Avoid globally muting the sink or blindly restoring another application's mute state.

On the N100, inspect `wpctl status` and the actual stream properties, then test External → Browser → External, External → mpv → External, Browser → mpv → Browser, and all three together. Include silent pages, short Web Audio effects, navigation, new Chromium streams, failure/restart cleanup, and whether suppressed playback advances. No current deployment instruction should imply that this policy is already configured.

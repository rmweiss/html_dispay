# Audio and Scene Composer

HTML Display is the kiosk host. It navigates Chromium to a URL or serves a complete uploaded HTML page; it does not interpret scene definitions or mix audio. The [Scene Composer audio design](https://github.com/rmweiss/scene_composer/blob/main/docs/audio.md) is the current design direction for scene audio. Scene Composer's audio API is itself planned, not implemented on its current main branch.

## Proposed integration

When Scene Composer is displayed on the kiosk, its browser player will own playback through one Web Audio `AudioContext`, with source gains, background/effects buses, and a master gain. Scene Composer's server owns the desired logical audio state and accepts effect triggers on a scene instance. The browser on each display produces its own sound. Background tracks start with an active instance and can loop; effects play only on command, and repeated triggers may overlap. Shared scene state does not imply synchronized playback clocks across displays.

The proposed trigger is on **Scene Composer**, not HTML Display:

```http
POST /api/scenes/{scene}/instances/{instance}/audio/effects/{effect}/play
Content-Type: application/json

{"volume": 0.8}
```

The response confirms that the command was accepted, not that the sound finished or was heard. A reconnecting Scene Composer player starts its loops from the beginning using the desired state. A scene reset restores authored defaults. Missing media should produce diagnostics without breaking visuals. Scene Composer's design also calls for an Enable audio control when a browser blocks playback; the unattended Chromium appliance uses `--autoplay-policy=no-user-gesture-required`.

HTML Display's reported NixOS setup already supports browser audio through PipeWire and an HDMI/DisplayPort sink; see [deployment](deployment.md). Displaying Scene Composer should therefore require no special audio endpoint in HTML Display. A locally generated HTML document can likewise play audio through the browser if its own code and asset URLs handle it.

## Separate open decision

The original HTML Display notes also express a wish to **send an audio file directly to the client**. That is a different operation from triggering a named Scene Composer effect. Neither repository's current implementation provides that direct upload/play command. The Scene Composer design does not settle whether HTML Display needs one. Add it only if a use case requires sound independent of the displayed page or Scene Composer instance; then define where the file is stored, playback concurrency, volume, and what navigation or restart does to it.

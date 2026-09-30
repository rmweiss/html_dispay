# Audio and Scene Composer

HTML Display is the kiosk host. It navigates Chromium to a URL or serves a complete uploaded HTML page; it does not interpret scene definitions or mix audio. The [Scene Composer audio design](https://github.com/rmweiss/scene_composer/blob/main/docs/audio.md) is the current design direction for scene audio. Scene Composer's browser mixer and effect-trigger API are implemented on its current main branch; ducking and video soundtrack routing remain future work.

## Normal path: Scene Composer

Audio should nearly always go through Scene Composer at the current stage of both projects. When Scene Composer is displayed on the kiosk, its browser player owns playback through one Web Audio `AudioContext`, with source gains, background/effects buses, and a master gain. Scene Composer's server owns the desired logical audio state and accepts effect triggers on a scene instance. The browser on each display produces its own sound. Background tracks start with an active instance and can loop; effects play only on command, and repeated triggers may overlap. Shared scene state does not imply synchronized playback clocks across displays.

The effect trigger is on **Scene Composer**, not HTML Display:

```http
POST /api/scenes/{scene}/instances/{instance}/audio/effects/{effect}/play
Content-Type: application/json

{"volume": 0.8}
```

The response confirms that the command was accepted, not that the sound finished or was heard. A reconnecting Scene Composer player starts its loops from the beginning using the desired state. A scene reset restores authored defaults. Missing media should produce diagnostics without breaking visuals. Scene Composer's design also calls for an Enable audio control when a browser blocks playback; the unattended Chromium appliance uses `--autoplay-policy=no-user-gesture-required`.

HTML Display's reported NixOS setup already supports browser audio through PipeWire and an HDMI/DisplayPort sink; see [deployment](deployment.md). Displaying Scene Composer should therefore require no special audio endpoint in HTML Display. A locally generated HTML document can likewise play audio through the browser if its own code and asset URLs handle it.

## Possible independent audio

There may eventually be a reason to play a spontaneous sound, or background music while the kiosk displays a page that does not come from Scene Composer. That would be independent of a Scene Composer instance and its named effects. It is a **possible future capability**, not a current requirement or implemented API. In particular, do not assume that sending an arbitrary audio file to HTML Display works today.

If this need becomes concrete, decide how independent audio coexists with the currently displayed page: whether playback should survive navigation, how it stops, where files live, and how volume and simultaneous sounds are controlled. A page's own audio may cover some cases, but that depends on the page and does not supply a general display-level command.

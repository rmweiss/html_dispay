# First concept: temporary mpv playback

> **Status: first concept for discussion (2026-09-29).** This is a proposal, not implemented behavior or an agreed API. Validate the session and audio details on the N100 before treating them as requirements.

## Goal

Allow the N100 to play a video or other media in fullscreen mpv on the same output as the Chromium kiosk. mpv takes foreground display and audio while it runs. At end of file, explicit stop, or playback failure, the system returns to the still-running Chromium page. Browser navigation and uploaded HTML remain the persistent display state.

This adds a temporary media mode alongside the existing browser display. It does not replace Scene Composer's browser-based audio mixer for sounds belonging to a scene; see [audio and Scene Composer](audio.md). Audio-only mpv playback is a candidate for the same supervised lifecycle, without covering Chromium; it still has highest audio priority. External background playback, such as Squeezelite, is a separate low-priority producer governed by the [audio policy](audio.md).

## Existing starting point

The checked-in [NixOS configuration](../config/nixos/configuration.nix) starts Sway as the `display` user and launches Chromium fullscreen under XWayland (`--ozone-platform=x11`). The Go server also runs as `display` and controls Chromium over local CDP. PipeWire and WirePlumber are enabled, with an HDMI/DP sink preferred. mpv is not installed or launched by the current config, and the server has no media API.

## Proposed ownership and sequence

**Architecture update (2026-11-14):** The Go-server-owned process model below is historical, not the current preference. The preferred design is an optional, independent mpv launcher/controller that starts a fresh mpv process per video, leaves Chromium and `html-display-server` running, and reports playback directly over MQTT. Supervision, transport control and process ownership remain to be implemented. The original sequence is retained below for context.

The original concept had the Go server own one mpv child process at a time. It retained its existing Chromium connection and the browser page kept running underneath. A play request transitioned to media mode:

1. Validate the requested source and reserve the media slot. Only a single request owns playback at a time.
2. Start mpv in the Sway session as the `display` user. For video, make it fullscreen and focused; audio-only playback leaves Chromium visible. Confirm window and audio behavior on the device rather than assuming launch order guarantees focus.
3. Wait for mpv to exit. On EOF, stop, startup failure, or process crash, release the slot and restore browser visibility/focus after video. Audio policy should automatically restore the highest remaining active class. Cleanup must run for every exit path.

A stop request terminates the owned process gracefully, then uses the same cleanup path. An interrupted or failed attempt must not leave lower-priority audio suppressed indefinitely. If the server itself crashes, process lifetime and audio recovery need an explicit policy, such as a supervised process group and startup reconciliation.

While media mode is active, the server may still accept browser display updates: Chromium navigates underneath, and the newest page is shown when mpv ends. This is a proposed default, to be confirmed. Replacing a currently playing mpv item, queuing, playlists, pause/seek, and volume control can be later extensions rather than first-version requirements.

## Foreground display and audio

Sway can match mpv's actual Wayland `app_id` and apply fullscreen/focus rules. The controller needs access to the correct Wayland socket and session environment; running as the same Unix user alone does not establish that. A small session helper or explicit environment handoff may be needed. Chromium's XWayland fullscreen rule should remain intact.

“Highest audio priority” means mpv alone is audible while its stream is active; it suppresses both Chromium and external players. The existing WirePlumber HDMI/DP `priority.session` only helps select a sink. The preferred approach is the role-based policy described in [audio](audio.md), subject to appliance tests for activity, cork/duck semantics, and restoration. If that fails, a state-aware controller may be needed. Do not globally mute the sink or blindly unmute other applications.

The exact Sway and PipeWire control method is deliberately undecided until tested on the appliance. mpv may use native Wayland, with `--fullscreen`, `--keep-open=no`, and a hardware decode setting selected after verifying Intel/VA-API playback. Flags and package references belong in the implementation once validated.

## Appliance validation and preferred playback profile (2026-11-14)

Manual tests with `nix-shell -p mpv yt-dlp deno` on the N100 demonstrated native Wayland fullscreen takeover of the existing Chromium window and automatic return to the same page after mpv exited, including on an EOF-limited test. Intel VA-API hardware decoding worked for local H.264 and 1080p59.94 YouTube AV1 video. Sway's Samsung HDMI-A-3 output supports 3840×2160 at 60 Hz; 4K60 is preferred over constraining YouTube to 30 fps (the test video lacked a 1080p30 variant).

The most promising manually tested YouTube profile used `--fullscreen --keep-open=no --hwdec=vaapi --vo=gpu --gpu-api=opengl --ytdl=yes`, with `--ytdl-format='bestvideo[height<=1080]+bestaudio/best[height<=1080]/best'`, `--cache=yes --cache-secs=60 --demuxer-max-bytes=128MiB --cache-pause=yes --cache-pause-initial=no --cache-pause-wait=10 --pipewire-buffer=200 --cache-on-disk=no`. A 90-second test reported no dropped frames or audio underruns, with a 58-second cache remaining; this is a promising observation, not a guarantee across different videos or network conditions. The `--length=90` test flag is not intended for production playback.

## Planned independent MQTT reporting and Home Assistant brightness policy

The standalone mpv controller should publish **read-only playback status and active media source directly to the existing MQTT broker**, not via `html-display-server`. This is a planned contract, not implemented behavior.

Proposed topic family, matching the existing `html-display/` prefix:

| Topic | Payload / behavior |
| --- | --- |
| `html-display/mpv/playback/state` | `idle`, `starting`, `playing`, `paused`, `stopping`, or `error` |
| `html-display/mpv/source/state` | Current requested media path or URL as a UTF-8 string; empty on idle/stop/failure |
| `html-display/mpv/availability` | `online` or `offline` for the reporting controller (with MQTT Last Will) |

These are **state/reporting topics**, not commands. A source is the original requested path/URL (e.g. a YouTube page URL), **not** yt-dlp's temporary signed media URL. Do not publish credentials, sensitive query parameters, or private paths without an explicit source-disclosure policy. The status is more useful than a boolean for future pause support. A Home Assistant binary sensor for active video takeover can derive its value from `starting`, `playing` and `paused` (and optionally `stopping` until the window is gone); `paused` should still mean the video occupies the screen. An optional separate diagnostic sensor can expose the full state, and a text sensor the source, via MQTT Discovery.

Publish current state/source on startup and on change, with retained state only if it is explicitly reconciled against the real player process at every controller restart. A stale retained `playing` must never be treated as a command to resume playback. On normal EOF, stop, or launch failure, publish the transition and clear the source. On unexpected controller death, the availability Last Will marks the reporting data unreliable; Home Assistant should use an availability-aware sensor rather than trusting stale state. The exact discovery payload and state transition timing remain to be implemented and tested.

**Brightness ownership:** Home Assistant already calculates Samsung Frame backlight from ambient light. Prefer reporting the video state and letting Home Assistant apply a configurable boost (initial experiment: +10 or +20 on the existing **0–50 ExLink backlight scale**, clamped to 50) while video takeover is active. When video ends, Home Assistant should recalculate from current ambient light, not restore an obsolete sampled backlight value. The mpv launcher should not write backlight itself in this design. Ensure a controller/MQTT failure cannot leave the boost enabled indefinitely. See [brightness](brightness.md) and [ExLink](exlink.md).

## API sketch, not a contract

- `POST /api/media/play`: request playback of one allowed local media item or URL; return an operation/status response without blocking until EOF.
- `POST /api/media/stop`: stop the active item; repeated stop should be harmless.
- `GET /api/media`: report idle, starting, playing, stopping, or failed, including a useful error when relevant.

The source schema, URL schemes and host policy, local file roots or upload mechanism, request limits, and authentication/trust boundary must be defined before accepting arbitrary network paths or files. The current HTTP service is reachable on the trusted LAN, so media requests should not become an unrestricted file reader or network fetcher. Errors should distinguish invalid source, busy player, failed startup, and normal completion.

## First implementation slice and checks

1. Add mpv to the NixOS package set and arrange the server/session environment and Sway rules needed to launch and focus it.
2. Add one supervised media process and a single play/stop path in Go, with serialized state transitions and cleanup.
3. Implement and test the three-class PipeWire/WirePlumber policy, including external audio suppression and automatic restoration.
4. On the actual N100, verify fullscreen takeover and return for EOF, stop, bad media, and server restart; check both silent and audio-playing browser pages, HDMI output, and H.264/AV1 hardware decoding.

These checks determine whether the simple overlay model is reliable in the existing Sway/XWayland setup. No current endpoint or deployment instruction should imply that this concept already works.

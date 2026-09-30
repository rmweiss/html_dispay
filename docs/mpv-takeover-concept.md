# First concept: temporary mpv playback

> **Status: first concept for discussion (2026-09-29).** This is a proposal, not implemented behavior or an agreed API. Validate the session and audio details on the N100 before treating them as requirements.

## Goal

Allow the N100 to play a video or other media in fullscreen mpv on the same output as the Chromium kiosk. mpv takes foreground display and audio while it runs. At end of file, explicit stop, or playback failure, the system returns to the still-running Chromium page. Browser navigation and uploaded HTML remain the persistent display state.

This adds a temporary media mode alongside the existing browser display. It does not replace Scene Composer's browser-based audio mixer for sounds belonging to a scene; see [audio and Scene Composer](audio.md). Audio-only mpv playback is a candidate for the same supervised lifecycle, without covering Chromium; it still has highest audio priority. External background playback, such as Squeezelite, is a separate low-priority producer governed by the [audio policy](audio.md).

## Existing starting point

The checked-in [NixOS configuration](../config/nixos/configuration.nix) starts Sway as the `display` user and launches Chromium fullscreen under XWayland (`--ozone-platform=x11`). The Go server also runs as `display` and controls Chromium over local CDP. PipeWire and WirePlumber are enabled, with an HDMI/DP sink preferred. mpv is not installed or launched by the current config, and the server has no media API.

## Proposed ownership and sequence

The Go server owns one mpv child process at a time. It retains its existing Chromium connection and the browser page keeps running underneath. A play request transitions to media mode:

1. Validate the requested source and reserve the media slot. Only a single request owns playback at a time.
2. Start mpv in the Sway session as the `display` user. For video, make it fullscreen and focused; audio-only playback leaves Chromium visible. Confirm window and audio behavior on the device rather than assuming launch order guarantees focus.
3. Wait for mpv to exit. On EOF, stop, startup failure, or process crash, release the slot and restore browser visibility/focus after video. Audio policy should automatically restore the highest remaining active class. Cleanup must run for every exit path.

A stop request terminates the owned process gracefully, then uses the same cleanup path. An interrupted or failed attempt must not leave lower-priority audio suppressed indefinitely. If the server itself crashes, process lifetime and audio recovery need an explicit policy, such as a supervised process group and startup reconciliation.

While media mode is active, the server may still accept browser display updates: Chromium navigates underneath, and the newest page is shown when mpv ends. This is a proposed default, to be confirmed. Replacing a currently playing mpv item, queuing, playlists, pause/seek, and volume control can be later extensions rather than first-version requirements.

## Foreground display and audio

Sway can match mpv's actual Wayland `app_id` and apply fullscreen/focus rules. The controller needs access to the correct Wayland socket and session environment; running as the same Unix user alone does not establish that. A small session helper or explicit environment handoff may be needed. Chromium's XWayland fullscreen rule should remain intact.

“Highest audio priority” means mpv alone is audible while its stream is active; it suppresses both Chromium and external players. The existing WirePlumber HDMI/DP `priority.session` only helps select a sink. The preferred approach is the role-based policy described in [audio](audio.md), subject to appliance tests for activity, cork/duck semantics, and restoration. If that fails, a state-aware controller may be needed. Do not globally mute the sink or blindly unmute other applications.

The exact Sway and PipeWire control method is deliberately undecided until tested on the appliance. mpv may use native Wayland, with `--fullscreen`, `--keep-open=no`, and a hardware decode setting selected after verifying Intel/VA-API playback. Flags and package references belong in the implementation once validated.

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

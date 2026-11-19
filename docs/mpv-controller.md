# Independent MPV controller (draft implementation)

The independent Go controller lives in `config/nixos/modules/html-display-mpv-src/`. It does **not** control Chromium, interact with the TV/ExLink daemon, or change Home Assistant brightness. It starts a new MPV process for each video and uses the existing `html-display-mpv` wrapper to select the tested WirePlumber Takeover role. The Sway browser stays running underneath.

**Status:** Proposed in a draft PR; built and appliance-tested behavior is **not yet confirmed**. The NixOS module is imported into `configuration.nix` but **disabled by default**.

## Opt in (on a test deployment)

Add alongside the existing TV service configuration:

```nix
services.htmlDisplayMPV = {
  enable = true;
  # The default only listens on 127.0.0.1 to avoid exposing unauthenticated commands.
  httpListen = "127.0.0.1:8081";
  mqttBroker = "tcp://192.168.10.12:1883";
  localRoots = [ "/home/display/Videos" ];
  # mpvOptions = [ "--fullscreen" "--hwdec=vaapi" ... ];
};
```

Run `sudo nixos-rebuild build` before `sudo nixos-rebuild switch`. The build requires the system's Nixpkgs channel to provide `buildGoModule`; the Go module has no third-party dependencies (Nix `vendorHash = null`). The service unit is `html-display-mpv-controller.service` running as `display`; it is not tied to Sway/Chromium's lifecycle. The daemon discovers the `display` user's Wayland socket when launching MPV and uses the existing PulseAudio takeover wrapper.

The HTTP listener is **loopback-only by default**. For remote LAN clients, explicitly set a LAN bind address and configure the firewall appropriately, or proxy with authentication. HTTP playback commands and the default MQTT TCP integration have no authentication. Do not expose either to untrusted networks. YouTube extraction can cause outbound requests and the URL allowlist is currently limited to **scheme** (HTTP/HTTPS), not destinations. The local media path is restricted to regular files inside symlink-resolved configured `localRoots`. Source URLs may be exposed through MQTT, so avoid credential-bearing URLs (URL userinfo is rejected).

## Minimal web interface

Open `http://127.0.0.1:8081/` (or use the SSH tunnel described below). The controller serves a small responsive page containing a video URL field, Play and Stop buttons, and current playback status. It uses the same REST endpoints as other clients. No additional package, web server, or front-end build is needed.

For access from a trusted LAN, explicitly configure `services.htmlDisplayMPV.httpListen = "0.0.0.0:8081";` and add port 8081 to the existing NixOS firewall allowed TCP ports (retain port 8080). Then open `http://html-display:8081/` using the device's resolvable hostname or IP. This exposes the **entire unauthenticated MPV REST API**, not just the form; only do this on an appropriately restricted network. The default remains loopback-only. The form does not replace an active video automatically; stop it first.

## HTTP API

- `GET /health`: { "status": "ok" } (process health, not video readiness)
- `GET /api/player`: current JSON snapshot: `state`, `source`, `position`, `duration`, optional `buffer_percent` and `buffer_seconds` (null when unknown), and optional `error`
- `POST /api/player/play`: JSON `{"source":"https://www.youtube.com/watch?v=..."}` or an allowed absolute local file path; starts a new MPV process and returns immediately
- `POST /api/player/stop`: gracefully terminate the current process
- `POST /api/player/pause`, `POST /api/player/resume`: MPV JSON IPC pause control
- `POST /api/player/seek`: JSON `{"position":42.0}`, absolute seconds

One active process only; a second play request returns HTTP 409. No replacing, queuing, playlists or playlists-on-reboot. Invalid source/IPC commands return JSON errors. HTTP 200 means the action was accepted, **not** that MPV decoded a frame.

To test over an SSH local port forward, run `ssh -L 8081:127.0.0.1:8081 admin@html-display` on the client and:

```sh
curl -s http://127.0.0.1:8081/health
curl -s -X POST http://127.0.0.1:8081/api/player/play \
  -H 'Content-Type: application/json' \
  -d '{"source":"https://www.youtube.com/watch?v=nfsEFdA76mQ"}'
curl -s http://127.0.0.1:8081/api/player
curl -s -X POST http://127.0.0.1:8081/api/player/pause
curl -s -X POST http://127.0.0.1:8081/api/player/resume
curl -s -X POST http://127.0.0.1:8081/api/player/stop
```

## MQTT

The daemon connects to the existing broker (same assumption as `html-display-tv`) and publishes retained state:

| Topic | Meaning |
| --- | --- |
| `html-display/mpv/playback/state` | `idle`, `starting`, `playing`, `paused`, `buffering`, `stopping`, `error` |
| `html-display/mpv/source/state` | Original HTTP(S) URL or resolved permitted path, empty on exit |
| `html-display/mpv/position/state` | Seconds |
| `html-display/mpv/duration/state` | Seconds |
| `html-display/mpv/buffer/state` | Cache refill progress 0–100%, empty when unavailable |
| `html-display/mpv/buffer_seconds/state` | Estimated seconds in MPV demuxer cache, empty when unavailable |
| `html-display/mpv/error/state` | Last MPV startup/exit error, empty for healthy playback or after stop |
| `html-display/mpv/availability` | `online` or `offline` (Last Will) |

Commands are non-retained and fire-and-forget:

| Topic | Payload |
| --- | --- |
| `html-display/mpv/source/set` | URL or permitted absolute file path, starts playback |
| `html-display/mpv/playback/set` | `pause`, `resume` (or `play` to resume), `stop` |
| `html-display/mpv/seek/set` | Absolute seconds as a number |

Home Assistant MQTT Discovery publishes playback/source/position/duration/cache sensors and a video-active binary sensor. Home Assistant's built-in MQTT integration cannot discover a native `media_player` entity; see [the built-in template media-player example](home-assistant-mpv.md) to create one using the existing MQTT topics and no custom integration. The binary sensor remains ON while `starting`, `playing`, `paused`, `buffering`, or `stopping`, so ambient-light automation can override the backlight while video occupies the display. Home Assistant owns all brightness changes.

The controller publishes its initial idle state after connecting/reconnecting and when the player changes state. Retained MQTT statuses do not start or restore a player after reboot; availability marks old data unavailable after an unclean exit.

## Playback failure handling

If MPV exits with a nonzero status, the controller reports `error` and retains the last attempted source and observed position/duration so the failure can be investigated via `GET /api/player` and the retained MQTT `error/state` diagnostic sensor. A successful new play request replaces the previous failure; stopping playback clears the status. If MPV never opens its JSON IPC socket, the controller terminates it after 15 seconds and records the timeout. This timeout is for establishing **IPC**, not a fixed deadline for downloading or buffering media.

The controller is still a single-player process supervisor. During systemd shutdown it terminates MPV and waits for its process to exit; systemd also owns the service cgroup to catch leftover descendants. Separately validate restart behavior on the N100. A zero-exit MPV attempt without valid media cannot always be distinguished from successful completion without additional playback event tracking.

## Buffering visibility

MPV's `paused-for-cache` property distinguishes automatic cache stalls from a user pause. The controller reports `buffering` before `paused` when both flags are true. Polling is once per second, so brief cache stalls may not be observed.

MPV's `cache-buffering-state` measures progress towards resuming from a cache stall; it is **not** the percentage of the whole video downloaded. `demuxer-cache-duration` is a rough estimate in seconds that MPV warns can be unreliable or unavailable. When a metric is missing or invalid, REST returns `null` and its retained MQTT state is published as an empty string so the HA sensor becomes unknown rather than reporting zero. Both metrics clear after MPV exits.

These additions do not change the MPV command-line options, cache policy, or the three-level audio priorities.

## MPV options and audio

The wrapper fixes only audio routing: `PULSE_PROP=media.role=Takeover`, `--ao=pulse`, and `--audio-device=pulse/html-display-takeover`. The service's `mpvOptions` NixOS list controls rendering, hardware decoding, quality, caching and fullscreen. Defaults reflect the successful N100 tests, **not** a permanent performance contract. The controller adds `--no-config --keep-open=no --idle=no`, one per-process JSON IPC socket and a single media source. It does not currently accept arbitrary playback flags through HTTP/MQTT.

## Validation checklist

1. Run `go test ./...` from `config/nixos/modules/html-display-mpv-src` and `sudo nixos-rebuild build`; check Nix evaluation before activation.
2. Enable the module; check `systemctl status html-display-mpv-controller` and `journalctl -u html-display-mpv-controller -f`.
3. Verify REST commands and MQTT state transitions on a permitted test video. Start a second item and check HTTP 409.
4. Confirm MPV appears fullscreen over Chromium, with video/audio rendering and return to the same browser page at EOF and stop.
5. Play Music Assistant/Squeezelite and Chromium audio. Confirm MPV overrides both, then each recovers after playback.
6. Verify pause/resume, seek and broker restart; test invalid URLs/files.
7. Check behaviour if Sway is restarted, the MPV process crashes, or the daemon is stopped/restarted while playing. In particular, make sure no orphan MPV window remains.
8. Assess reported dropped frames and the startup PulseAudio underrun before choosing final performance defaults.

See [audio architecture](audio.md) and [MPV design](mpv-takeover-concept.md).

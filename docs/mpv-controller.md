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

## HTTP API

- `GET /health`: { "status": "ok" } (process health, not video readiness)
- `GET /api/player`: current JSON snapshot: `state`, `source`, `position`, `duration` and optional `error`
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
| `html-display/mpv/playback/state` | `idle`, `starting`, `playing`, `paused`, `stopping`, `error` |
| `html-display/mpv/source/state` | Original HTTP(S) URL or resolved permitted path, empty on exit |
| `html-display/mpv/position/state` | Seconds |
| `html-display/mpv/duration/state` | Seconds |
| `html-display/mpv/availability` | `online` or `offline` (Last Will) |

Commands are non-retained and fire-and-forget:

| Topic | Payload |
| --- | --- |
| `html-display/mpv/source/set` | URL or permitted absolute file path, starts playback |
| `html-display/mpv/playback/set` | `pause`, `resume` (or `play` to resume), `stop` |
| `html-display/mpv/seek/set` | Absolute seconds as a number |

Home Assistant MQTT Discovery publishes a playback state sensor, source sensor, position/duration sensors, and a video-active binary sensor. There is **not yet a native media_player entity**. The binary sensor remains ON while `starting`, `playing`, `paused`, or `stopping`, so ambient-light automation can override the backlight while video occupies the display. Home Assistant owns all brightness changes.

The controller publishes its initial idle state after connecting/reconnecting and when the player changes state. Retained MQTT statuses do not start or restore a player after reboot; availability marks old data unavailable after an unclean exit.

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

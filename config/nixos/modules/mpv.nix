# Independent MPV controller, separate from Chromium and html-display-server.
{ config, lib, pkgs, ... }:
let
  cfg = config.services.htmlDisplayMPV;
  package = pkgs.buildGoModule {
    pname = "html-display-mpv-controller";
    version = "0.1.0";
    src = ./html-display-mpv-src;
    vendorHash = null;
    env.CGO_ENABLED = "0";
    subPackages = [ "cmd/html-display-mpv" ];
  };
in {
  options.services.htmlDisplayMPV = {
    enable = lib.mkEnableOption "independent MPV controller with HTTP and MQTT";
    httpListen = lib.mkOption {
      type = lib.types.str;
      default = "127.0.0.1:8081";
      description = "REST listen address. Loopback by default; LAN exposure is explicit.";
    };
    mqttBroker = lib.mkOption {
      type = lib.types.str;
      default = "tcp://192.168.10.12:1883";
      description = "MQTT broker (TCP without authentication); empty disables MQTT.";
    };
    mqttPrefix = lib.mkOption {
      type = lib.types.str;
      default = "html-display/mpv";
    };
    mpvBinary = lib.mkOption {
      type = lib.types.str;
      default = "/run/current-system/sw/bin/html-display-mpv";
      description = "Existing validated Takeover audio wrapper, not the Go daemon.";
    };
    mpvOptions = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [
        "--fullscreen" "--hwdec=vaapi" "--vo=gpu" "--gpu-api=opengl"
        "--ytdl=yes"
        "--ytdl-format=bestvideo[height<=1080]+bestaudio/best[height<=1080]/best"
        "--cache=yes" "--cache-secs=60" "--demuxer-max-bytes=128MiB"
        "--cache-pause=yes" "--cache-pause-initial=no"
        "--cache-pause-wait=10" "--cache-on-disk=no"
      ];
      description = "Options passed to MPV; audio routing belongs to the wrapper. The controller appends its IPC arguments and source.";
    };
    localRoots = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ "/home/display/Videos" ];
      description = "Permitted local file directory roots; symlink-resolved paths must remain inside.";
    };
  };

  config = lib.mkIf cfg.enable {
    systemd.services.html-display-mpv-controller = {
      description = "Independent HTML Display MPV controller";
      wantedBy = [ "multi-user.target" ];
      after = [ "network.target" "html-display-sway.service" ];
      # Unlike html-display-server, the controller stays alive if Sway restarts.
      serviceConfig = {
        Type = "simple";
        User = "display";
        Group = "users";
        ExecStart = lib.concatStringsSep " " [
          "${package}/bin/html-display-mpv"
          "-http-listen ${lib.escapeShellArg cfg.httpListen}"
          "-mqtt-broker ${lib.escapeShellArg cfg.mqttBroker}"
          "-mqtt-prefix ${lib.escapeShellArg cfg.mqttPrefix}"
          "-mpv-bin ${lib.escapeShellArg cfg.mpvBinary}"
          "-mpv-options ${lib.escapeShellArg (builtins.toJSON cfg.mpvOptions)}"
          "-local-roots ${lib.escapeShellArg (builtins.toJSON cfg.localRoots)}"
        ];
        Restart = "always";
        RestartSec = "2s";
      };
    };
  };
}

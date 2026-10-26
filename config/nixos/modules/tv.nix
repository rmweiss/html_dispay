# Samsung Frame ExLink controller. Enable lifecycle only after removing cec.nix.
{ config, lib, pkgs, ... }:
let
  cfg = config.services.htmlDisplayTV;
  package = pkgs.buildGoModule {
    pname = "html-display-tv";
    version = "0.2.0";
    src = ./html-display-tv-src;
    vendorHash = null;
    env.CGO_ENABLED = "0";
    subPackages = [ "cmd/html-display-tv" "cmd/tvctl" ];
  };
  tvctl = "${package}/bin/tvctl";
  on = "${pkgs.coreutils}/bin/timeout 32s ${tvctl} power on";
  off = "${pkgs.coreutils}/bin/timeout 32s ${tvctl} power off";
in {
  options.services.htmlDisplayTV = {
    enable = lib.mkEnableOption "Samsung Frame ExLink controller";
    lifecycle.enable = lib.mkEnableOption "TV power control during boot, shutdown and suspend/resume";
    serialDevice = lib.mkOption {
      type = lib.types.str;
      default = "/dev/serial/by-id/usb-Prolific_Technology_Inc._USB-Serial_Controller_D-if00-port0";
    };
    mqttBroker = lib.mkOption {
      type = lib.types.str;
      default = "tcp://192.168.10.12:1883";
    };
  };

  config = lib.mkIf cfg.enable {
    environment.systemPackages = [ package ];

    systemd.services.html-display-tv = {
      description = "Samsung Frame ExLink controller";
      wantedBy = [ "multi-user.target" ];
      after = [ "systemd-udev-trigger.service" ];
      serviceConfig = {
        Type = "simple";
        ExecStart = "${package}/bin/html-display-tv -device ${lib.escapeShellArg cfg.serialDevice} -socket /run/html-display-tv/control.sock -mqtt-broker ${lib.escapeShellArg cfg.mqttBroker}";
        RuntimeDirectory = "html-display-tv";
        RuntimeDirectoryMode = "0755";
        User = "root";
        Restart = "on-failure";
        RestartSec = "2s";
        TimeoutStopSec = "40s";
      };
    };

    systemd.services.html-display-tv-lifecycle = lib.mkIf cfg.lifecycle.enable {
      description = "Samsung Frame power on boot and off at shutdown";
      wantedBy = [ "multi-user.target" ];
      requires = [ "html-display-tv.service" ];
      after = [ "html-display-tv.service" ];
      serviceConfig = {
        Type = "oneshot";
        RemainAfterExit = true;
        ExecStart = on;
        ExecStop = off;
        TimeoutStartSec = "40s";
        TimeoutStopSec = "40s";
      };
    };

    systemd.services.html-display-tv-sleep = lib.mkIf cfg.lifecycle.enable {
      description = "Samsung Frame off before sleep and on after resume";
      wantedBy = [ "sleep.target" ];
      partOf = [ "sleep.target" ];
      before = [ "sleep.target" ];
      after = [ "html-display-tv.service" ];
      requires = [ "html-display-tv.service" ];
      serviceConfig = {
        Type = "oneshot";
        RemainAfterExit = true;
        ExecStart = off;
        ExecStop = pkgs.writeShellScript "html-display-tv-resume" ''
          ${pkgs.coreutils}/bin/sleep 2
          ${on}
        '';
        TimeoutStartSec = "40s";
        TimeoutStopSec = "40s";
      };
    };
  };
}

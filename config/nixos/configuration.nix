# /etc/nixos/configuration.nix
{ config, pkgs, lib, ... }:

{
  imports = [
    ./hardware-configuration.nix
  ];

  # Boot loader
  boot.loader.systemd-boot.enable = true;
  boot.loader.efi.canTouchEfiVariables = true;

  boot.tmp.useTmpfs = true;

  # ---------------------------------------------------------------------------
  # Basic system
  # ---------------------------------------------------------------------------

  networking.hostName = "html-display";
  networking.networkmanager.enable = true;
  networking.firewall.allowedTCPPorts = [ 8080 ];

  time.timeZone = "Europe/Zurich";

  # Useful for remote administration.
  services.openssh = {
    enable = true;
    openFirewall = true;
    settings = {
      PasswordAuthentication = true;
      PermitRootLogin = "yes";
    };
  };


  hardware.graphics = {
    enable = true;
    extraPackages = with pkgs; [
      intel-media-driver
    ];
  };

  environment.sessionVariables = {
    LIBVA_DRIVER_NAME = "iHD";
  };

  # ---------------------------------------------------------------------------
  # Display user
  # ---------------------------------------------------------------------------

  users.users.display = {
    isNormalUser = true;
    description = "HTML Display";
  };

  # ---------------------------------------------------------------------------
  # Admin user
  # ---------------------------------------------------------------------------


  users.users.admin = {
    isNormalUser = true;
    extraGroups = [ "wheel" ];
  };


  # ---------------------------------------------------------------------------
  # Chromium kiosk
  # ---------------------------------------------------------------------------

  systemd.tmpfiles.rules = [
    "d /run/html-display 0755 display users -"
    "d /run/html-display/chromium 0700 display users -"
  ];

  programs.sway = {
    enable = true;
    wrapperFeatures.gtk = false;
  };

  environment.etc."html-display/sway.conf".text = ''
    # No normal desktop chrome.
    default_border none
    default_floating_border none

    # Hide the mouse pointer after 2 seconds of inactivity.
    seat * hide_cursor 2000

    # Black background.
    output * bg #000000 solid_color

    # Start Chromium.
    exec ${pkgs.chromium}/bin/chromium \
      --user-data-dir=/run/html-display/chromium \
      --ozone-platform=x11 \
      --kiosk \
      --no-first-run \
      --disable-session-crashed-bubble \
      --autoplay-policy=no-user-gesture-required \
      --remote-debugging-port=9222 \
      http://127.0.0.1:8080/

    # Chromium should occupy the entire output even if Chromium itself
    # doesn't request fullscreen correctly.
    for_window [app_id="chromium"] fullscreen enable
    for_window [class="Chromium-browser"] fullscreen enable
    for_window [class="chromium"] fullscreen enable
  '';

  systemd.services.html-display-sway = {
    description = "HTML Display Sway session";

    wantedBy = [ "graphical.target" ];
    after = [
      "systemd-user-sessions.service"
      "systemd-logind.service"
    ];

    conflicts = [ "getty@tty1.service" ];

    serviceConfig = {
      User = "display";

      PAMName = "login";

      TTYPath = "/dev/tty1";
      StandardInput = "tty";
      StandardOutput = "journal";
      StandardError = "journal";

      Environment = [
        "XDG_SESSION_TYPE=wayland"
        "XDG_CURRENT_DESKTOP=sway"
        "XDG_SESSION_DESKTOP=sway"
        "LIBVA_DRIVER_NAME=iHD"
      ];

      ExecStart = "${pkgs.sway}/bin/sway --config /etc/html-display/sway.conf";

      Restart = "always";
      RestartSec = "2s";
    };

    unitConfig = {
      StartLimitIntervalSec = 30;
      StartLimitBurst = 10;
    };
  };

  # ---------------------------------------------------------------------------
  # HTML display control server
  # ---------------------------------------------------------------------------



  systemd.services.html-display-server = {
    description = "HTML Display Control Server";

    wantedBy = [ "multi-user.target" ];

    # Chromium is provided by the sway service.
    after = [
      "network.target"
      "html-display-sway.service"
    ];

    requires = [
      "html-display-sway.service"
    ];

    partOf = [
      "html-display-sway.service"
    ];
  
    serviceConfig = {
      Type = "simple";
      User = "display";
      Group = "users";

      ExecStart = "/opt/html-display/html-display-server";

      StateDirectory = "html-display";

      Restart = "always";
      RestartSec = "2s";
    };
  };


  # ---------------------------------------------------------------------------
  # Audio
  # ---------------------------------------------------------------------------

  security.rtkit.enable = true;


  services.pipewire = {
    enable = true;
    alsa.enable = true;
    pulse.enable = true;

    wireplumber = {
      enable = true;

      extraConfig."10-html-display" = {
        "wireplumber.settings" = {
          # Don't remember a manually selected profile/route from
          # whatever monitor happened to be connected previously.
          "device.restore-profile" = false;
          "device.restore-routes" = false;
        };

        "monitor.alsa.rules" = [
          {
            matches = [
              {
                "node.name" = "~alsa_output.*hdmi.*";
              }
            ];

            actions = {
              update-props = {
                # Prefer an available HDMI/DP sink over analog audio.
                "priority.session" = 2000;
              };
            };
          }
        ];
      };
    };
  };

  # ---------------------------------------------------------------------------
  # Useful tools for administration/debugging
  # ---------------------------------------------------------------------------

  environment.systemPackages = with pkgs; [
    chromium
    curl
    git
    htop
    vim
    intel_gpu_top
  ];


  # ---------------------------------------------------------------------------
  # Nix
  # ---------------------------------------------------------------------------

  nix.settings.experimental-features = [
    "nix-command"
    "flakes"
  ];


  # Set this to the NixOS release originally used to install the machine.
  # Don't change it during normal upgrades.
  system.stateVersion = "26.05";
}
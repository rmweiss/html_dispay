# /etc/nixos/configuration.nix
{ config, pkgs, lib, ... }:

{
  imports = [
    ./hardware-configuration.nix
    ./modules/cec.nix
  ];

  # Boot loader
  boot.loader.systemd-boot.enable = true;
  boot.loader.efi.canTouchEfiVariables = true;
  boot.loader.timeout = 0;

  boot.tmp.useTmpfs = true;

  # ---------------------------------------------------------------------------
  # Basic system
  # ---------------------------------------------------------------------------

  # IMPORTANT: tty1 is reserved for the Sway kiosk, not a login prompt.
  # During nixos-rebuild switch, NixOS was starting getty@tty1. Because
  # html-display-sway conflicts with getty@tty1, this stopped Sway and also
  # stopped html-display-server (which has PartOf/Requires dependencies).
  # The observed result was a text console and both kiosk units inactive.
  # Do not re-enable either tty1 getty unit without redesigning tty ownership.
  systemd.services."getty@tty1".enable = false;
  systemd.services."autovt@tty1".enable = false;

  # Preserve both kiosk services during routine NixOS configuration switches.
  # Auto-restarting the services on unit changes interrupted the display;
  # with restartIfChanged = false, manually restart/reboot when intentionally
  # changing the kiosk units, Chromium command line, or server binary.
  # This does NOT disable systemd's serviceConfig.Restart = "always" for crashes.
  systemd.services.html-display-sway.restartIfChanged = false;
  systemd.services.html-display-server.restartIfChanged = false;

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

  services.logind.settings.Login = {
    HandlePowerKey = "suspend";
  };

  # ---------------------------------------------------------------------------
  # Display user
  # ---------------------------------------------------------------------------

  users.users.display = {
    isNormalUser = true;
    description = "HTML Display";
    # Run audio services at boot and retain hardware access without a kiosk session.
    linger = true;
    extraGroups = [ "audio" ];
  };

  # ---------------------------------------------------------------------------
  # Admin user
  # ---------------------------------------------------------------------------


  users.users.admin = {
    isNormalUser = true;
    extraGroups = [ "wheel" "video" "dialout"];
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

  # Local page is available even before the Go HTTP server is listening.
  environment.etc."html-display/startup.html".text = ''
    <!doctype html>
    <html><head><meta charset="utf-8"><style>
      html, body { margin: 0; width: 100%; height: 100%; background: #000; }
    </style></head><body></body></html>
  '';

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
      file:///etc/html-display/startup.html

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

      ExecStart = "/opt/html-display/html-display-server -mqtt-broker tcp://192.168.10.12:1883";

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


        "device.profile.priority.rules" = [
          {
            matches = [
              {
                "device.name" = "alsa_card.pci-0000_00_1f.3";
              }
            ];
            actions = {
              update-props = {
                priorities = [ "output:hdmi-stereo" ];
              };
            };
          }
        ];

      };
    };
  };

  # Raw SPA-JSON preserves unquoted component dependency names. WirePlumber
  # 0.5.14 rejected the quoted requires array generated by extraConfig.
  services.pipewire.wireplumber.configPackages = [
    (pkgs.writeTextDir
      "share/wireplumber/wireplumber.conf.d/90-audio-priority.conf"
      ''
        wireplumber.profiles = {
          main = {
            policy.linking.role-based = required
            html-display.role.external = required
            html-display.role.browser = required
          }
        }
        wireplumber.settings = {
          linking.role-based.duck-level = 0.0
        }
        wireplumber.components = [
          {
            name = libpipewire-module-loopback
            type = pw-module
            provides = html-display.role.external
            requires = [ support.export-core, pw.node-factory.adapter ]
            arguments = {
              node.description = "HTML Display External"
              capture.props = {
                node.name = "html-display-external"
                media.class = Audio/Sink
                audio.position = [ FL, FR ]
                device.intended-roles = [ "External" ]
                policy.role-based.target = true
                policy.role-based.priority = 10
                policy.role-based.action.same-priority = "mix"
                policy.role-based.action.lower-priority = "duck"
              }
              playback.props = {
                node.name = "html-display-external-output"
                audio.position = [ FL, FR ]
                node.passive = true
                media.role = "Loopback"
                target.object = "alsa_output.pci-0000_00_1f.3.hdmi-stereo"
              }
            }
          }
          {
            name = libpipewire-module-loopback
            type = pw-module
            provides = html-display.role.browser
            requires = [ support.export-core, pw.node-factory.adapter ]
            arguments = {
              node.description = "HTML Display Browser"
              capture.props = {
                node.name = "html-display-browser"
                media.class = Audio/Sink
                audio.position = [ FL, FR ]
                device.intended-roles = [ "Browser" ]
                policy.role-based.target = true
                policy.role-based.priority = 20
                policy.role-based.action.same-priority = "mix"
                policy.role-based.action.lower-priority = "duck"
              }
              playback.props = {
                node.name = "html-display-browser-output"
                audio.position = [ FL, FR ]
                node.passive = true
                media.role = "Loopback"
                target.object = "alsa_output.pci-0000_00_1f.3.hdmi-stereo"
              }
            }
          }
        ]
      '')
  ];

  services.pipewire.extraConfig.pipewire-pulse."90-audio-priority" = {
    "stream.rules" = [
      {
        matches = [
          {
            "application.process.binary" = "squeezelite-pulse";
            "media.class" = "Stream/Output/Audio";
          }
        ];
        actions.update-props = {
          "media.role" = "External";
          "target.object" = "html-display-external";
        };
      }
      {
        matches = [
          {
            "application.process.binary" = "chromium";
            "media.class" = "Stream/Output/Audio";
          }
        ];
        actions.update-props = {
          "media.role" = "Browser";
          "target.object" = "html-display-browser";
        };
      }
    ];
  };

  # ---------------------------------------------------------------------------
  # Squeezelite
  # ---------------------------------------------------------------------------

  systemd.user.services.squeezelite = {
    description = "Independent Squeezelite player";
    wantedBy = [ "default.target" ];

    # NixOS installs user units for all users; run this one only as display.
    unitConfig.ConditionUser = "display";
    wants = [ "pipewire-pulse.socket" "wireplumber.service" ];
    after = [ "pipewire-pulse.socket" "wireplumber.service" ];
    serviceConfig = {
      # Machine-specific Music Assistant address and Slimproto port.
      ExecStart = "${pkgs.squeezelite-pulse}/bin/squeezelite-pulse -n html-display -s 192.168.10.12:3483";
      Restart = "always";
      RestartSec = "5s";
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
    libcec
    v4l-utils
    usbutils
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

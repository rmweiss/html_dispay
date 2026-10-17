# /etc/nixos/modules/cec.nix
# Pulse-Eight USB-CEC control, independent of Chromium and audio.
{ pkgs, ... }:

let
  cecDevice = "/dev/serial/by-id/usb-Pulse-Eight_USB-CEC_Adapter_v2_r8-if00";

  displayTv = pkgs.writeShellScriptBin "display-tv" ''
    set -euo pipefail

    if [[ ! -e "${cecDevice}" ]]; then
      echo "Pulse-Eight CEC adapter not found: ${cecDevice}" >&2
      exit 1
    fi

    case "''${1:-}" in
      on)            command="on 0" ;;
      off)           command="standby 0" ;;
      status)        command="pow 0" ;;
      active)        command="as" ;;
      scan)          command="scan" ;;
      volume-up)     command="volup" ;;
      volume-down)   command="voldown" ;;
      mute)          command="mute" ;;
      volume-status) command="gas" ;;
      *)
        echo "Usage: display-tv {on|off|status|active|scan|volume-up|volume-down|mute|volume-status}" >&2
        exit 2
        ;;
    esac

    printf '%s\n' "$command" |
      ${pkgs.coreutils}/bin/timeout 10s \
        ${pkgs.libcec}/bin/cec-client -s -d 1 "${cecDevice}"
  '';

  tvSleepHook = pkgs.writeShellScript "display-tv-sleep" ''
    # systemd passes pre before suspend and post after resume.
    # Best-effort: CEC failures must not block suspend/resume.
    send_cec() {
      if [ ! -e "${cecDevice}" ]; then
        echo "CEC adapter unavailable: ${cecDevice}" >&2
        return 1
      fi
      printf '%s\n' "$1" |
        ${pkgs.coreutils}/bin/timeout 10s \
          ${pkgs.libcec}/bin/cec-client -s -d 1 "${cecDevice}"
    }

    case "''${1:-}" in
      pre)
        send_cec "standby 0" || true
        ;;
      post)
        # USB serial adapters may need time to reappear after resume.
        ${pkgs.coreutils}/bin/sleep 2
        send_cec "on 0" || true
        ;;
    esac
  '';
in
{
  environment.systemPackages = [ pkgs.libcec displayTv ];

  # /dev/ttyACM* is generally root:dialout on this machine.
  users.users.admin.extraGroups = [ "dialout" ];

  # Executed by systemd before and after suspend.
  environment.etc."systemd/system-sleep/display-tv".source = tvSleepHook;
}

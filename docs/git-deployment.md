# Git-based NixOS deployment (migration)

This migration replaces the old hand-installed Go binary and Pulse-Eight CEC
controller with a Nix-built HTML Display server and the ExLink TV controller.

## One-time setup on the existing appliance

Keep the machine-generated `/etc/nixos/hardware-configuration.nix` in place.
The checked-in `config/nixos/configuration.nix` now imports that absolute path,
and its Nix-built server reads source from the same repository checkout.

```sh
sudo git clone https://github.com/rmweiss/html_dispay.git /etc/nixos/html_dispay
sudo cp -a /etc/nixos/configuration.nix /etc/nixos/configuration.nix.pre-git
sudo tee /etc/nixos/configuration.nix >/dev/null <<'EOF'
{ ... }: {
  imports = [ ./html_dispay/config/nixos/configuration.nix ];
}
EOF
```

If the clone directory already exists, use `git -C /etc/nixos/html_dispay pull --ff-only`
instead of cloning. Never overwrite the machine-generated hardware file.

**First-build prerequisite:** `config/nixos/modules/html-display-server.nix`
currently has a **placeholder** `vendorHash = lib.fakeHash;`. Before using this
branch on the appliance, build once in a disposable/test environment with Nix.
Nix's fixed-output hash mismatch supplies the actual `got: sha256-...` hash;
replace `lib.fakeHash` with that value and commit it. A build using the
placeholder is expected to fail. This is **not yet deployment-ready** until the
real hash has been committed and the build passes.

## Routine deployment

Once the real dependency hash is in Git and this branch is merged:

```sh
cd /etc/nixos/html_dispay
sudo git pull --ff-only
sudo nixos-rebuild switch
```

The NixOS configuration is selected via the small local importer in
`/etc/nixos/configuration.nix`. No flake or manual Go installation is required.
Updating Go sources rebuilds the store package; the server unit references the
resulting store path, and its `restartIfChanged = true` permits activation to
restart the server with the new executable. The browser/Sway service retains
`restartIfChanged = false` so routine server changes need not replace the
kiosk. Some unit dependency changes can still interrupt it.

The main server still uses `/var/lib/html-display/current.html` for the
last uploaded HTML. The old `/opt/html-display/html-display-server` is no longer
used after the new unit has successfully activated.

## TV controller migration

The configuration imports `./modules/tv.nix` instead of
`./modules/cec.nix` and enables:

```nix
services.htmlDisplayTV = {
  enable = true;
  lifecycle.enable = true;
  osdOnStartup = "off";
};
```

This uses the existing ExLink daemon, `tvctl`, MQTT integration and lifecycle
power handling. It expects the Prolific serial adapter specified by the module
(default: `/dev/serial/by-id/usb-Prolific_Technology_Inc._USB-Serial_Controller_D-if00-port0`).
Check that device on the appliance before rebuilding. CEC's `display-tv`
command, boot service, and suspend hook will no longer be installed from this
machine configuration; do not run both TV power controllers concurrently.

## Checks and recovery

```sh
sudo nixos-rebuild dry-activate
sudo nixos-rebuild switch
systemctl is-active html-display-sway html-display-server html-display-tv
systemctl is-active getty@tty1.service
tvctl status
sudo journalctl -b -u html-display-server -u html-display-tv -n 80 --no-pager
```

Expected: the three display services active and tty1 getty inactive.
Preserve the tty1 ownership protections in the machine configuration.
Keep SSH access and an alternative virtual terminal available during migration.

If activation fails, the backed-up configuration and previous NixOS generations
remain available for recovery. Do not change `system.stateVersion` in a routine update.

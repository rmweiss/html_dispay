# Build the display server with Nix, from the repository checkout.
# This file lives at config/nixos/modules/, so ../../.. is the Git root.
{ buildGoModule, lib }:

buildGoModule {
  pname = "html-display-server";
  version = "0.1.0";
  src = ../../..;
  subPackages = [ "." ];
  env.CGO_ENABLED = "0";

  # TODO: Replace with the actual dependency vendor hash from a Nix build.
  # The initial build reports the expected sha256 on a fixed-output hash mismatch.
  vendorHash = lib.fakeHash;

  ldflags = [ "-s" "-w" ];
  postInstall = ''
    mv "$out/bin/html_dispay" "$out/bin/html-display-server"
  '';
}

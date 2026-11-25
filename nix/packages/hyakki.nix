{ pkgs, src, version ? "unstable", optional ? [ ] }:
pkgs.callPackage (src + "/ryoku/hyakki/release/packages/nix/hyakki.nix") {
  inherit pkgs src version optional;
}

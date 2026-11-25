# SPDX-License-Identifier: GPL-3.0-only
# Standalone Hyakki from its corresponding-source archive (release/source-dist.sh).
#   pkgs.callPackage ./hyakki.nix { src = ./hyakki-0.1.0.tar.gz; optional = [ "tesseract" "espeak-ng" ]; }
# Optional programs are propagated into the user's environment. All four
# executables remain native sibling files so peer identity stays verifiable.
{ lib, stdenv, pkgs, src, version ? "0.1.0", optional ? [ ] }:

let
  table = builtins.fromJSON (builtins.readFile ../../optional.json);
  byId = builtins.listToAttrs (map (c: { name = c.id; value = c; }) table.components);
  pick = id:
    let c = byId.${id} or (throw "hyakki: unknown optional component ${id}");
    in if c.packages.nix == [ ]
       then throw "hyakki: ${id} has no nixpkgs package"
       else map (name: lib.getAttrFromPath (lib.splitString "." name) pkgs) c.packages.nix;
  extra = lib.concatMap pick optional;
in
assert lib.versionAtLeast pkgs.go.version "1.26";
stdenv.mkDerivation {
  pname = "hyakki";
  inherit version src;
  postUnpack = ''sourceRoot="$sourceRoot/ryoku/hyakki"'';
  nativeBuildInputs = [ pkgs.go pkgs.pkg-config pkgs.python3 ];
  propagatedUserEnvPkgs = [ pkgs.bubblewrap pkgs.git pkgs.xdg-utils ] ++ extra;
  buildInputs = [ pkgs.wayland pkgs.libxkbcommon pkgs.libGL pkgs.vulkan-headers
    pkgs.vulkan-loader pkgs.xorg.libX11 pkgs.libxcb pkgs.xorg.libXcursor pkgs.xorg.libXfixes ];

  buildPhase = ''
    runHook preBuild
    export HOME="$TMPDIR" GOCACHE="$TMPDIR/go-cache"
    HYAKKI_VERSION="$version" HYAKKI_BUILD_DIR="$TMPDIR/hyakki-build" sh release/build.sh
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall
    stage="$TMPDIR/hyakki-stage"
    HYAKKI_BUILD_DIR="$TMPDIR/hyakki-build" DESTDIR="$stage" sh release/install.sh
    sh release/check.sh "$stage"
    mkdir -p "$out"
    cp -a "$stage/usr/." "$out/"
    for file in "$out"/lib/systemd/user/* "$out"/share/hyakki/runit/*/run; do
      substituteInPlace "$file" --replace-quiet '/usr/bin/' "$out/bin/"
    done
    substituteInPlace "$out/share/applications/dev.ryoku.Hyakki.desktop" \
      --replace-fail 'Exec=hyakki-app' "Exec=$out/bin/hyakki-app"
    patchShebangs "$out/bin" "$out/share/hyakki/runit"
    runHook postInstall
  '';

  # Gio dlopens Vulkan, so the ordinary dependency-based RUNPATH shrink misses
  # its loader. Keep the native app beside oni; a wrapper would break peer trust.
  postFixup = ''
    for binary in hyakki-app oni-surface; do
      patchelf --add-rpath ${lib.makeLibraryPath [ pkgs.vulkan-loader ]} "$out/bin/$binary"
    done
  '';

  passthru.runtimePackages = [ pkgs.bubblewrap pkgs.git pkgs.xdg-utils ] ++ extra;
  passthru.optionalComponents = map (c: c.id) table.components;

  meta = {
    description = "Hyakki local-first AI desktop and command line";
    license = with lib.licenses; [ gpl3Only asl20 ofl mit bsd3 zlib {
      shortName = "SCOWL";
      fullName = "SCOWL dictionary and included source notices";
      spdxId = "LicenseRef-SCOWL";
      free = true;
      redistributable = true;
    } ];
    mainProgram = "hyakki-app";
    platforms = [ "x86_64-linux" ];
  };
}

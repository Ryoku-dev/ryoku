{
  pkgs,
  src,
}:

let
  # The eleven Mechvibes packs the shipped profiles use, vendored as a git
  # bundle beside the Arch recipe (upstream rewrote its history for Mechvibes 3
  # and the pinned commit is gone), unpacked here without touching the network.
  mechvibesSrc = pkgs.runCommand "mechvibes-src" {
    nativeBuildInputs = [ pkgs.git ];
  } ''
    git clone -q ${src}/release/packages/ryoku-keysounds-profiles/mechvibes.bundle $out
    git -C $out checkout -q 8d18b40792c7ce859a898ce0e840a0c01112888c
    rm -rf $out/.git
  '';
in

pkgs.hyprlandPlugins.mkHyprlandPlugin {
  pluginName = "ryoku-keysounds";
  version = "1.0";

  src =
    src + "/ryoku/hyprland/plugins/keysounds";

  dontUseCmakeConfigure = true;

  nativeBuildInputs = [
    pkgs.ffmpeg
    pkgs.gnumake
    pkgs.makeWrapper
    pkgs.python3
  ];

  buildInputs = [
    pkgs.libcanberra
  ];

  postPatch = ''
    # The upstream Makefile can clone Mechvibes and discovers the importer
    # through PATH. Nix builds are network-isolated, so provide both inputs
    # explicitly from immutable sources. The importer and profiles.txt live
    # in the shared ryoku/keysounds tree, outside this plugin source.
    cp \
      ${src}/ryoku/keysounds/ryoku-keysounds-import \
      ./ryoku-keysounds-import

    substituteInPlace \
      ./ryoku-keysounds-import \
      --replace-fail \
        '#!/usr/bin/env python3' \
        '#!${pkgs.python3}/bin/python3'

    chmod 755 \
      ./ryoku-keysounds-import

    # The upstream Arch build can be repaired from Settings. On NixOS
    # plugin ABI ownership belongs to the generation instead.
    substituteInPlace \
      main.cpp \
      --replace-fail \
        'rebuild it from Settings > Plugins' \
        'restart Hyprland after updating the NixOS Ryoku generation'
  '';

  buildPhase = ''
    runHook preBuild

    make \
      CXX="$CXX" \
      MECHVIBES="${mechvibesSrc}" \
      IMPORTER="$PWD/ryoku-keysounds-import" \
      PROFILES="${src}/ryoku/keysounds/profiles.txt" \
      keysounds.so \
      sounds

    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall

    mkdir -p \
      "$out/bin" \
      "$out/lib" \
      "$out/share/doc/ryoku-keysounds"

    # Keep the sample directory beside the plugin. main.cpp deliberately
    # searches <directory-containing-.so>/keysounds before its FHS fallback.
    install -Dm755 \
      keysounds.so \
      "$out/lib/keysounds.so"

    cp -a \
      sounds \
      "$out/lib/keysounds"

    find "$out/lib/keysounds" \
      -type d \
      -exec chmod 755 {} +

    find "$out/lib/keysounds" \
      -type f \
      -exec chmod 644 {} +

    install -Dm755 \
      ./ryoku-keysounds-import \
      "$out/bin/ryoku-keysounds-import"

    wrapProgram \
      "$out/bin/ryoku-keysounds-import" \
      --prefix PATH : ${pkgs.lib.makeBinPath [
        pkgs.ffmpeg
      ]}

    install -Dm644 \
      README.md \
      "$out/share/doc/ryoku-keysounds/README.md"

    install -Dm644 \
      ${src}/ryoku/keysounds/profiles.txt \
      "$out/share/doc/ryoku-keysounds/profiles.txt"

    runHook postInstall
  '';

  meta = {
    description =
      "Ryoku Hyprland keyboard sound plugin and switch profiles";

    homepage =
      "https://github.com/Ryoku-dev/ryoku-arch";

    license = [
      pkgs.lib.licenses.gpl3Plus
      pkgs.lib.licenses.mit
    ];

    platforms = [
      "x86_64-linux"
    ];
  };
}

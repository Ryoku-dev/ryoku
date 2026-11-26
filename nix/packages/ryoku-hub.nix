{
  pkgs,
  src,
  paletteBridge,
  desktopData,
}:

pkgs.stdenv.mkDerivation {
  pname = "ryoku-hub";
  version = "unstable";

  src = src + "/ryoku/hub/backend";

  nativeBuildInputs = [
    pkgs.go
    pkgs.makeWrapper
  ];

  postPatch = ''
    substituteInPlace shellpref.go \
      --replace-fail '"/usr/bin/fish"' '"/run/current-system/sw/bin/fish"' \
      --replace-fail '"/usr/bin/bash"' '"/run/current-system/sw/bin/bash"' \
      --replace-fail '"/usr/bin/zsh"' '"/run/current-system/sw/bin/zsh"'

    substituteInPlace desktop_enum.go \
      --replace-fail '"/usr/local/share:/usr/share"' '"/run/current-system/sw/share:/usr/local/share:/usr/share"'

    substituteInPlace palettebridge.go \
      --replace-fail '"/usr/share/ryoku/palette-bridge"' '"${paletteBridge}/share/ryoku/palette-bridge"'
  '';

  buildPhase = ''
    runHook preBuild

    export HOME="$TMPDIR"
    export GOCACHE="$TMPDIR/go-cache"
    export GOTOOLCHAIN=local

    go build \
      -mod=vendor \
      -trimpath \
      -o ryoku-hub \
      .

    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall

    mkdir -p "$out/bin" "$out/libexec"

    install -Dm755 \
      ryoku-hub \
      "$out/libexec/ryoku-hub"

    makeWrapper \
      "$out/libexec/ryoku-hub" \
      "$out/bin/ryoku-hub" \
      --prefix PATH : ${pkgs.lib.makeBinPath [
        pkgs.bubblewrap
        pkgs.libwebp
        pkgs.ffmpeg
        pkgs.libsecret
      ]} \
      --set RYOKU_XKB_RULES_DIR "${pkgs.xkeyboard_config}/share/X11/xkb/rules" \
      --set XKB_CONFIG_ROOT "${pkgs.xkeyboard_config}/share/X11/xkb" \
      --set-default RYOKU_SYSTEM_RICES_DIR "${desktopData}/share/ryoku/rices"

    runHook postInstall
  '';

  meta = {
    description = "Ryoku Settings and Hub backend";
    homepage = "https://github.com/Ryoku-dev/ryoku-arch";
    license = pkgs.lib.licenses.gpl3Only;
    platforms = [ "x86_64-linux" ];
  };
}

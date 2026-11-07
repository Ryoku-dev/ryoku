{
  pkgs,
  src,
  version,
  desktopData,
  paletteBridge,
}:

pkgs.stdenv.mkDerivation {
  pname = "ryoku-cli";
  inherit version;

  src = src + "/ryoku/cli";

  nativeBuildInputs = [
    pkgs.go
    pkgs.makeWrapper
  ];

  postPatch = ''
    # The Vesktop migration runs the packaged recipe from the palette-bridge package.
    substituteInPlace internal/doctor/reconcile_vesktop_palette.go \
      --replace-fail \
        '"/usr/share/ryoku/palette-bridge/install-integrations.sh"' \
        '"${paletteBridge}/share/ryoku/palette-bridge/install-integrations.sh"'
  '';

  buildPhase = ''
    runHook preBuild

    export HOME="$TMPDIR"
    export GOCACHE="$TMPDIR/go-cache"
    export GOTOOLCHAIN=local

    go build \
      -mod=vendor \
      -trimpath \
      -o ryoku \
      .

    CGO_ENABLED=0 go build \
      -mod=vendor \
      -trimpath \
      -o ryoku-host \
      ./cmd/ryoku-host

    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall

    mkdir -p "$out/bin"

    install -Dm755 \
      ryoku \
      "$out/bin/.ryoku-wrapped"

    install -Dm755 \
      ryoku-host \
      "$out/bin/.ryoku-host-wrapped"

    makeWrapper \
      "$out/bin/.ryoku-host-wrapped" \
      "$out/bin/ryoku-host" \
      --set RYOKU_HOST_PLATFORM nixos

    makeWrapper \
      "$out/bin/.ryoku-wrapped" \
      "$out/bin/ryoku" \
      --set RYOKU_CONFIG_BASE "${desktopData}/share/ryoku/config" \
      --set RYOKU_I18N_DIR "${desktopData}/share/ryoku/i18n" \
      --set RYOKU_UPDATE_BACKEND nix \
      --set RYOKU_NIX_VERSION "${version}" \
      --set RYOKU_NIX_CHANNEL nix

    runHook postInstall
  '';

  meta = {
    description = "Ryoku desktop command-line control utility";
    homepage = "https://github.com/Ryoku-dev/ryoku-arch";
    license = pkgs.lib.licenses.gpl3Only;
    platforms = [ "x86_64-linux" ];
  };
}

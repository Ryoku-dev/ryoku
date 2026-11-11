{
  pkgs,
  src,
}:

let
  repoSrc = src;
in
pkgs.buildGoModule {
  pname = "ryoku-rashin";
  version = "unstable";

  src = repoSrc + "/ryoku/rashin";
  vendorHash = null;

  nativeBuildInputs = [
    pkgs.pkg-config
    pkgs.python3
    pkgs.makeWrapper
  ];

  buildInputs = [
    pkgs.wayland
    pkgs.libxkbcommon
    pkgs.libGL
    pkgs.vulkan-headers
  ];

  tags = [
    "nox11"
  ];

  buildPhase = ''
    runHook preBuild

    export HOME="$TMPDIR"
    export GOCACHE="$TMPDIR/go-cache"
    export GOTOOLCHAIN=local

    CGO_ENABLED=0 go build -mod=vendor -trimpath \
      -ldflags "-s -w -X main.version=$version" \
      -o rashind ./cmd/rashind
    CGO_ENABLED=0 go build -mod=vendor -trimpath \
      -ldflags "-s -w -X main.version=$version" \
      -o rashin ./cmd/rashin
    CGO_ENABLED=1 go build -mod=vendor -trimpath -tags nox11 \
      -ldflags "-s -w -X main.version=$version" \
      -o rashin-app ./cmd/rashin-app

    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall

    for binary in rashind rashin rashin-app; do
      install -Dm755 "$binary" "$out/bin/$binary"
    done
    # Teams run their agents, and the programs those open, inside bwrap.
    wrapProgram "$out/bin/rashind" \
      --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.bubblewrap ]}
    for collector in ryoku-ai-usage claude-usage codex-usage opencode-usage; do
      install -Dm755 "usage/$collector" "$out/bin/$collector"
    done
    for unit in rashind.socket rashind.service ryoku-ai-usage.service ryoku-ai-usage.timer \
      rashind-routines.service rashind-routines.timer; do
      install -Dm644 "systemd/$unit" "$out/lib/systemd/user/$unit"
    done
    substituteInPlace "$out/lib/systemd/user/rashind.service" \
      --replace-fail "/usr/bin/rashind" "$out/bin/rashind"
    substituteInPlace "$out/lib/systemd/user/ryoku-ai-usage.service" \
      --replace-fail "/usr/bin/ryoku-ai-usage" "$out/bin/ryoku-ai-usage"
    substituteInPlace "$out/lib/systemd/user/rashind-routines.service" \
      --replace-fail "/usr/bin/rashin " "$out/bin/rashin "
    for collector in claude-usage codex-usage opencode-usage; do
      patchShebangs "$out/bin/$collector"
    done
    install -Dm644 \
      "${repoSrc}/release/packages/ryoku-rashin/dev.ryoku.rashin.desktop" \
      "$out/share/applications/dev.ryoku.rashin.desktop"
    install -Dm644 \
      "${repoSrc}/ryoku/assets/brand/rashin-mark.svg" \
      "$out/share/icons/hicolor/scalable/apps/dev.ryoku.rashin.svg"
    install -Dm644 "${repoSrc}/LICENSE" \
      "$out/share/licenses/ryoku-rashin/LICENSE"
    install -Dm644 NOTICE \
      "$out/share/licenses/ryoku-rashin/NOTICE"

    runHook postInstall
  '';

  meta = {
    description = "Ryoku Rashin AI system";
    homepage = "https://github.com/Ryoku-dev/ryoku";
    license = pkgs.lib.licenses.gpl3Plus;
    mainProgram = "rashin-app";
    platforms = [ "x86_64-linux" ];
  };
}

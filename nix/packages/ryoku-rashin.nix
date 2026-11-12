{
  pkgs,
  src,
  # uv sets up Hindsight, the optional memory engine. Its Python and wheels
  # are prebuilt for FHS systems, so on NixOS it also needs programs.nix-ld.
  withUv ? false,
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
      --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.bubblewrap ]} \
      ${pkgs.lib.optionalString withUv "--suffix PATH : ${pkgs.lib.makeBinPath [ pkgs.uv ]}"}
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
    install -Dm644 vendor/github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols/LICENSE \
      "$out/share/licenses/ryoku-rashin/LICENSE.material-symbols"
    install -Dm644 vendor/github.com/ryoku-dev/ryoku/docs/lithic/library/go/fonts/Geologica-OFL.txt \
      "$out/share/licenses/ryoku-rashin/LICENSE.geologica"
    install -Dm644 vendor/github.com/ryoku-dev/ryoku/docs/lithic/library/go/fonts/MartianMono-OFL.txt \
      "$out/share/licenses/ryoku-rashin/LICENSE.martian-mono"
    install -Dm644 vendor/gopkg.in/validator.v2/LICENSE \
      "$out/share/licenses/ryoku-rashin/LICENSE.validator"
    install -Dm644 LICENSE.apache-2.0 \
      "$out/share/licenses/ryoku-rashin/LICENSE.apache-2.0"
    for set in superpowers openspec speckit egm; do
      install -Dm644 "internal/skills/sets/$set/LICENSE" \
        "$out/share/licenses/ryoku-rashin/LICENSE.$set"
    done

    runHook postInstall
  '';

  meta = {
    description = "Ryoku Rashin AI system";
    homepage = "https://github.com/Ryoku-dev/ryoku";
    license = [
      pkgs.lib.licenses.gpl3Plus
      pkgs.lib.licenses.asl20
      pkgs.lib.licenses.ofl
      pkgs.lib.licenses.mit
    ];
    mainProgram = "rashin-app";
    platforms = [ "x86_64-linux" ];
  };
}

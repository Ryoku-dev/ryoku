{ pkgs, src }:

let
  version = "1.0.0";

  # The engine is a pinned download: the sandbox has no network, so the payload
  # builds against this tree instead of fetching it.
  engineTar = pkgs.fetchurl {
    url = "https://github.com/handy-computer/transcribe.cpp/archive/refs/tags/v0.4.2.tar.gz";
    hash = "sha256-VFU3Thitqg15/0gQSxMQnRXdYALt2pme2hF5qG6gnFs=";
  };

  # The English model the package ships, so dictation works on first boot with no
  # download and nothing is loaded until the user turns dictation on.
  model = pkgs.fetchurl {
    url = "https://huggingface.co/handy-computer/parakeet-tdt_ctc-110m-gguf/resolve/main/parakeet-tdt_ctc-110m-Q8_0.gguf";
    hash = "sha256-fdRMdKMx14ik5fixaROz/rKc7SLPVhOq0OD2zTBRYpY=";
  };
in
pkgs.stdenv.mkDerivation {
  pname = "ryoku-dictation";
  inherit version;

  inherit src;
  dontUnpack = true;

  nativeBuildInputs = [ pkgs.cmake pkgs.ninja pkgs.pkg-config ];
  buildInputs = [ pkgs.pipewire pkgs.wayland pkgs.libxkbcommon ];

  buildPhase = ''
    runHook preBuild
    mkdir -p build-root engine
    tar xzf ${engineTar} -C engine --strip-components=1
    cp ${model} build-root/parakeet-tdt_ctc-110m-Q8_0.gguf
    RYOKU_SRC=$src \
    RYOKU_BUILD_DIR="$PWD/build-root" \
    TRANSCRIBE_SRC="$PWD/engine" \
      bash $src/release/packages/ryoku-dictation/payload.sh build
    runHook postBuild
  '';

  # The payload lays out a /usr tree (one recipe for every edition); Nix wants it
  # under $out without the prefix, so stage and move.
  installPhase = ''
    runHook preInstall
    mkdir -p stage "$out"
    RYOKU_SRC=$src \
    RYOKU_BUILD_DIR="$PWD/build-root" \
      bash $src/release/packages/ryoku-dictation/payload.sh install "$PWD/stage" systemd
    cp -r stage/usr/. "$out/"
    runHook postInstall
  '';

  meta = {
    description = "Ryoku voice typing: local speech-to-text with Parakeet and Whisper";
    homepage = "https://ryoku.dev";
    license = [ pkgs.lib.licenses.gpl3Plus pkgs.lib.licenses.mit ];
    platforms = [ "x86_64-linux" ];
  };
}

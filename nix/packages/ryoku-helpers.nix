{ pkgs, src }:

let
  # Ryostage replaces the old separate Depth and Parallax Python runtimes.
  #
  # Upstream normally provisions a mutable Python venv on first use. NixOS
  # supplies the complete runtime declaratively instead; only downloaded model
  # weights belong in writable user state.
  stagePython = pkgs.python3.withPackages (ps: [
    ps.rembg
    ps.numpy
    ps.pillow
    ps.scipy
  ]);
in
pkgs.stdenvNoCC.mkDerivation {
  pname = "ryoku-helpers";
  version = "unstable";

  inherit src;

  nativeBuildInputs = [ pkgs.makeWrapper ];

  dontBuild = true;

  installPhase = ''
    runHook preInstall

    mkdir -p "$out/bin" "$out/libexec"

    install_helpers() {
      for helper in "$@"; do
        [ -f "$helper" ] || continue

        case "$helper" in
          *.service|*.rules|*.conf)
            continue
            ;;
        esac

        install -Dm755 \
          "$helper" \
          "$out/bin/$(basename "$helper")"
      done
    }

    install_helpers \
      system/hardware/*/ryoku-*

    # Compositor-neutral shell helpers moved out of the Hyprland payload.
    # They must exist under both Hyprland and Niri.
    install_helpers \
      ryoku/shell/scripts/ryoku-* \
      ryoku/shell/scripts/*.sh

    # Neutral helpers come directly from the source tree. Rewrite FHS/env
    # interpreters now so every installed command has an immutable Nix runtime.
    patchShebangs "$out/bin"

    if [ -x "$out/bin/ryoku-power-cutover" ]; then
      substituteInPlace "$out/bin/ryoku-power-cutover" \
        --replace-fail \
          "/usr/bin/systemd-inhibit" \
          "${pkgs.systemd}/bin/systemd-inhibit" \
        --replace-fail \
          "/usr/bin/sleep" \
          "${pkgs.coreutils}/bin/sleep" \
        --replace-fail \
          "/usr/bin/env" \
          "${pkgs.coreutils}/bin/env"
    fi

    install -Dm755 \
      ryoku/lockscreen/ryoku-qylock-activate \
      "$out/bin/ryoku-qylock-activate"

    patchShebangs "$out/bin/ryoku-qylock-activate"

    substituteInPlace "$out/bin/ryoku-qylock-activate" \
      --replace-fail \
        'power_helper=""' \
        "power_helper=\"$out/bin/ryoku-power-cutover\""

    # Upstream's ryoku-idle owns policy/rendering while NixOS owns the daemon
    # lifecycle. Keep the upstream implementation intact in libexec and expose
    # a tiny systemd-aware front door for session/autostart and Hub calls.
    install -Dm755 \
      system/hardware/power/ryoku-idle \
      "$out/libexec/ryoku-idle"

    patchShebangs "$out/libexec/ryoku-idle"

    cat > "$out/bin/ryoku-idle" <<'SH'
#!@BASH@
set -euo pipefail

real="@REAL@"
systemctl_bin="@SYSTEMCTL@"

case "''${1:-start}" in
  start)
    exec "$systemctl_bin" --user start ryoku-idle.service
    ;;
  apply)
    "$real" render
    exec "$systemctl_bin" --user restart ryoku-idle.service
    ;;
  *)
    exec "$real" "$@"
    ;;
esac
SH

    substituteInPlace "$out/bin/ryoku-idle" \
      --replace-fail '@BASH@' "${pkgs.bash}/bin/bash" \
      --replace-fail '@REAL@' "$out/libexec/ryoku-idle" \
      --replace-fail '@SYSTEMCTL@' "${pkgs.systemd}/bin/systemctl"

    chmod 755 "$out/bin/ryoku-idle"

    # These are useful desktop helpers. Package-management mutations
    # remain a separate NixOS-porting concern.
    install_helpers \
      system/extras/ryoku-cmd-present

    # ------------------------------------------------------------------
    # Ryostage
    # ------------------------------------------------------------------
    #
    # Keep upstream's public engine and command surface intact, but intercept
    # `install`: Arch provisions rembg in a user venv, while NixOS already owns
    # rembg/numpy/Pillow/scipy in the immutable store. On NixOS, install means
    # downloading the requested model weights only.
    install -Dm755 \
      ryoku/shell/scripts/ryostage \
      "$out/libexec/ryostage"

    cat > "$out/bin/ryostage" <<'SH'
#!@BASH@
set -euo pipefail

real="@REAL@"
python="@PYTHON@"
bash_bin="@BASH@"

export PATH="@STAGE_BIN@:@FINDUTILS_BIN@:@COREUTILS_BIN@:$PATH"
export PYTHONNOUSERSITE=1

xdg_state="''${XDG_STATE_HOME:-$HOME/.local/state}"
state="$xdg_state/ryoku/ryostage"
models="$state/models"
migration_marker="$state/.nix-legacy-models-adopted"

mkdir -p "$models"

export REMBG_HOME="$models"
export U2NET_HOME="$models"

# The pre-Stage Nix port stored model weights in depth/models and
# parallax/models, but did not create upstream's mutable venv. Upstream's
# migration therefore cannot detect those caches. Adopt their model data once,
# while keeping the old copies untouched for rollback.
if [[ ! -e "$migration_marker" ]]; then
  for legacy in \
    "$xdg_state/ryoku/depth/models" \
    "$xdg_state/ryoku/parallax/models"
  do
    [[ -d "$legacy" ]] || continue

    cp -a --reflink=auto --no-clobber \
      "$legacy/." \
      "$models/" \
      2>/dev/null || true
  done

  touch "$migration_marker"
fi

# All commands except install use upstream unchanged. The Nix Python runtime is
# first in PATH, so ryostage's runtime probes find it instead of creating a venv.
if [[ ''${1:-} != install ]]; then
  exec "$bash_bin" "$real" "$@"
fi

shift

if (( $# == 0 )); then
  set -- u2netp
fi

for model in "$@"; do
  case "$model" in
    u2netp|birefnet-general-lite)
      ;;
    *)
      printf 'ryostage: unsupported model: %s\n' "$model" >&2
      exit 2
      ;;
  esac

  if [[ -n "$(find "$models" -name "$model.onnx" -print -quit 2>/dev/null)" ]]; then
    printf '%s already installed\n' "$model"
    continue
  fi

  printf 'Downloading model %s...\n' "$model"

  "$python" - "$model" <<'PY'
import sys

import onnxruntime as ort
from rembg import new_session

model = sys.argv[1]

available = set(ort.get_available_providers())
providers = [
    provider
    for provider in ("CUDAExecutionProvider", "CPUExecutionProvider")
    if provider in available
]

try:
    if providers:
        new_session(model, providers=providers)
    else:
        new_session(model)
except (TypeError, ValueError):
    new_session(model)
PY
done

exec "$bash_bin" "$real" check
SH

    substituteInPlace "$out/bin/ryostage" \
      --replace-fail '@REAL@' "$out/libexec/ryostage" \
      --replace-fail '@PYTHON@' "${stagePython}/bin/python3" \
      --replace-fail '@BASH@' "${pkgs.bash}/bin/bash" \
      --replace-fail '@STAGE_BIN@' "${stagePython}/bin" \
      --replace-fail '@FINDUTILS_BIN@' "${pkgs.findutils}/bin" \
      --replace-fail '@COREUTILS_BIN@' "${pkgs.coreutils}/bin"

    chmod 755 "$out/bin/ryostage"

    # ------------------------------------------------------------------
    # Ryoku equalizer
    # ------------------------------------------------------------------
    #
    # Upstream expects normal FHS commands such as pactl, pw-cli and
    # systemctl. Keep the upstream implementation intact and give it a
    # deterministic Nix runtime PATH.
    install -Dm755 \
      ryoku/shell/scripts/ryoku-eq \
      "$out/libexec/ryoku-eq"

    patchShebangs "$out/libexec/ryoku-eq"

    cat > "$out/bin/ryoku-eq" <<'SH'
#!@BASH@
set -euo pipefail

export PATH="@EQ_PATH@:$PATH"
exec "@REAL@" "$@"
SH

    substituteInPlace "$out/bin/ryoku-eq" \
      --replace-fail '@BASH@' "${pkgs.bash}/bin/bash" \
      --replace-fail '@REAL@' "$out/libexec/ryoku-eq" \
      --replace-fail '@EQ_PATH@' "${pkgs.lib.makeBinPath [
        pkgs.jq
        pkgs.pulseaudio
        pkgs.pipewire
        pkgs.systemd
        pkgs.coreutils
      ]}"

    chmod 755 "$out/bin/ryoku-eq"

    # ------------------------------------------------------------------
    # Shared shell helpers moved out of the compositor config tree
    # ------------------------------------------------------------------

    install -Dm755 \
      ryoku/shell/scripts/ryoku-reload-cover \
      "$out/bin/ryoku-reload-cover"

    patchShebangs "$out/bin/ryoku-reload-cover"

    mkdir -p "$out/share/ryoku/reload-cover"
    cp -a \
      ryoku/shell/quickshell/reload-cover/. \
      "$out/share/ryoku/reload-cover/"

    substituteInPlace "$out/bin/ryoku-reload-cover" \
      --replace-fail \
        '/usr/share/ryoku/reload-cover' \
        "$out/share/ryoku/reload-cover"

    wrapProgram "$out/bin/ryoku-reload-cover" \
      --prefix PATH : "${pkgs.lib.makeBinPath [
        pkgs.coreutils
        pkgs.gawk
        pkgs.jq
        pkgs.quickshell
        pkgs.util-linux
      ]}"

    for helper in ryoku/shell/scripts/stash-compress.sh ryoku/shell/scripts/stash-install.sh; do
      install -Dm755 \
        "$helper" \
        "$out/bin/$(basename "$helper")"
    done

    patchShebangs "$out/bin/stash-compress.sh" "$out/bin/stash-install.sh"

    # Settings -> language integration.
    #
    # Keep the upstream tools/ layout intact because sync.py resolves
    # langs.json and catalog/ relative to its own location.
    mkdir -p \
      "$out/libexec/ryoku-i18n/tools" \
      "$out/libexec/ryoku-i18n/catalog"

    install -Dm644 \
      ryoku/i18n/langs.json \
      "$out/libexec/ryoku-i18n/langs.json"

    install -Dm644 \
      ryoku/i18n/catalog/*.json \
      "$out/libexec/ryoku-i18n/catalog/"

    install -Dm755 \
      ryoku/i18n/tools/sync.py \
      "$out/libexec/ryoku-i18n/tools/sync.py"

    printf '#!%s\nexec "%s" "%s" "$@"\n' \
      "${pkgs.bash}/bin/bash" \
      "${pkgs.python3}/bin/python3" \
      "$out/libexec/ryoku-i18n/tools/sync.py" \
      > "$out/bin/ryoku-i18n"

    chmod 755 "$out/bin/ryoku-i18n"

    runHook postInstall
  '';

  meta = {
    description = "Runtime helper commands used by the Ryoku desktop";
    homepage = "https://github.com/Ryoku-dev/ryoku-arch";
    license = pkgs.lib.licenses.gpl3Only;
    platforms = [ "x86_64-linux" ];
  };
}

#!/usr/bin/env bash
# Regression test for ryoku-cmd-brightness: the laptop panel must be driven
# even when bash has no compgen. Nix patches the script onto a bash built
# without completion builtins, and a compgen check there failed silently, so
# the brightness keys never reached brightnessctl. RYOKU_BACKLIGHT_PATH lets
# the backlight class be faked.
set -euo pipefail

here="$(cd "$(dirname "$0")/.." && pwd)"
script="$here/ryoku/shell/scripts/ryoku-cmd-brightness"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$tmp/bin" "$tmp/drm"
cat >"$tmp/bin/brightnessctl" <<'EOS'
#!/usr/bin/env bash
echo "$*" >>"$BL_RAN"
EOS
printf '#!/usr/bin/env bash\necho intel_backlight\n' >"$tmp/bin/ryoku-hw-backlight"
printf '#!/usr/bin/env bash\nexit 0\n' >"$tmp/bin/ddcutil"
chmod +x "$tmp/bin/"*

# press <backlight-dir> <delta> -- one keypress with compgen disabled, echoing
# the brightnessctl calls it made.
press() {
  : >"$tmp/bl_ran"
  PATH="$tmp/bin:$PATH" BL_RAN="$tmp/bl_ran" RYOKU_BACKLIGHT_PATH="$1" \
    RYOKU_DRM_PATH="$tmp/drm" XDG_RUNTIME_DIR="$tmp/rt" \
    bash -c 'enable -n compgen; . "$0" "$@"' "$script" "$2" >/dev/null 2>&1 || true
  cat "$tmp/bl_ran"
}

mkdir -p "$tmp/backlight/intel_backlight" "$tmp/no-backlight"

# 1. A backlight is present: the panel moves, in the asked direction.
out="$(press "$tmp/backlight" +5)"
[[ $out == "-d intel_backlight set 5%+" ]] || { echo "FAIL: +5 ran '$out'" >&2; exit 1; }
out="$(press "$tmp/backlight" -5)"
[[ $out == "-d intel_backlight set 5%-" ]] || { echo "FAIL: -5 ran '$out'" >&2; exit 1; }

# 2. No backlight (a desktop): brightnessctl is left alone.
out="$(press "$tmp/no-backlight" +5)"
[[ -z $out ]] || { echo "FAIL: no backlight still ran '$out'" >&2; exit 1; }

echo "PASS: ryoku-cmd-brightness drives the panel without compgen"

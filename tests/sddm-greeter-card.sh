#!/usr/bin/env bash
# Exercise the greeter's DRM-card choice without touching the host's devices.
set -euo pipefail

repo=${RYOKU_PATH:-$(cd "$(dirname "$0")/.." && pwd)}
greeter=$repo/ryoku/lockscreen/sddm/ryoku-greeter
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
bin=$tmp/bin
mkdir -p "$bin"

fail() { printf 'sddm-greeter-card: %s\n' "$*" >&2; exit 1; }

cat >"$bin/modetest" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$@" >"$RYOKU_TEST_MODETEST_LOG"
[ "${RYOKU_TEST_MODETEST_FAIL:-0}" = 1 ] && exit 255
printf 'Connectors:\nid\tencoder\tstatus\t\tname\t\tsize (mm)\tmodes\tencoders\n'
printf '31\t30\tconnected\t%s\t300x190\t\t1\t30\n' "$RYOKU_TEST_CONNECTOR"
printf '  modes:\n\tindex name refresh (Hz) hdisp hss hse htot vdisp vss vse vtot\n'
printf '\t  #0 1920x1080 60.00 1920 2000 2020 2080 1080 1100 1120 1160 0 flags: ; type: preferred\n'
STUB

cat >"$bin/weston" <<'STUB'
#!/usr/bin/env bash
: >"$RYOKU_TEST_WESTON_LOG"
for arg in "$@"; do
  case "$arg" in
    --config=*)
      [ -s "${arg#--config=}" ] || exit 20
      arg='--config=<generated>'
      ;;
  esac
  printf '%s\n' "$arg" >>"$RYOKU_TEST_WESTON_LOG"
done
STUB
# the stub weston exits at once; the wrapper reads that as a death and would
# start it again, so one attempt is allowed and its report goes to a stub logger
# instead of the host's system log.
cat >"$bin/logger" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$RYOKU_TEST_LOGGER_LOG"
STUB
chmod +x "$bin/modetest" "$bin/weston" "$bin/logger"

reset_fixture() {
  rm -rf "$tmp/drm" "$tmp/run"
  mkdir -p "$tmp/drm" "$tmp/run"
}

add_card() {
  local number=$1 render=$2 boot=${3:-0} vendor=${4:-0x0000} driver
  local card=$tmp/drm/card$number
  mkdir -p "$card/device/drm"
  printf '%s\n' "$boot" >"$card/device/boot_vga"
  printf '%s\n' "$vendor" >"$card/device/vendor"
  case "$vendor" in
    0x10de) driver=nvidia ;;
    0x1002) driver=amdgpu ;;
    0x8086) driver=i915 ;;
    *) driver=virtio_gpu ;;
  esac
  printf 'DRIVER=%s\n' "$driver" >"$card/device/uevent"
  [ "$render" = none ] || : >"$card/device/drm/$render"
}

add_connector() {
  local number=$1 name=$2 status=$3
  mkdir -p "$tmp/drm/card$number-$name"
  printf '%s\n' "$status" >"$tmp/drm/card$number-$name/status"
}

assert_log() {
  local path=$1 expected=$2 label=$3 actual
  actual=$(cat "$path")
  [ "$actual" = "$expected" ] ||
    fail "$label: got [$actual], want [$expected]"
}

run_case() {
  local name=$1 card=$2 connector=$3 pixman=${4:-0}
  local weston_expected driver
  RYOKU_GREETER_DRM_ROOT="$tmp/drm" \
    RYOKU_GREETER_CONF="$tmp/missing-greeter.conf" \
    RYOKU_GREETER_PRIMARY_FILE="$tmp/missing-primary" \
    RYOKU_GREETER_NUMLOCK_FILE="$tmp/missing-numlock" \
    RYOKU_GREETER_SETTLE=0 \
    RYOKU_GREETER_DEADLINE=0 \
    RYOKU_TEST_MODETEST_LOG="$tmp/run/modetest" \
    RYOKU_TEST_WESTON_LOG="$tmp/run/weston" \
    RYOKU_TEST_LOGGER_LOG="$tmp/run/logger" \
    RYOKU_GREETER_RETRY_LIMIT=1 \
    RYOKU_TEST_CONNECTOR="$connector" \
    TMPDIR="$tmp/run" \
    PATH="$bin:$PATH" \
    "$greeter" >/dev/null 2>"$tmp/run/stderr" || fail "$name did not start weston"

  driver=$(sed -n 's/^DRIVER=//p' "$tmp/drm/$card/device/uevent")
  assert_log "$tmp/run/modetest" $'-M\n'"$driver"$'\n-c' "$name modetest argv"
  weston_expected=$'--shell=kiosk\n--drm-device='"$card"$'\n'
  [ "$pixman" = 0 ] || weston_expected+=$'--renderer=pixman\n'
  weston_expected+='--config=<generated>'
  assert_log "$tmp/run/weston" "$weston_expected" "$name weston argv"
  printf 'sddm-greeter-card: %s -> %s%s\n' \
    "$name" "$card" "$([ "$pixman" = 0 ] || printf ' + pixman')"
}

# Issue #479: firmware marks the headless NVIDIA card as boot VGA, while the
# AMD iGPU owns the connected laptop panel. The panel owner must win.
reset_fixture
add_card 1 renderD128 1 0x10de
add_connector 1 HDMI-A-1 disconnected
add_card 2 renderD129 0 0x1002
add_connector 2 eDP-1 connected
run_case 'hybrid AMD panel' card2 eDP-1

# T2 Mac: the USB Touch Bar has a connected KMS output but no render node. The
# disconnected Intel panel must not beat the AMD card that currently owns eDP.
reset_fixture
add_card 0 none 1
add_connector 0 USB-1 connected
add_card 1 renderD128 1 0x8086
add_connector 1 eDP-1 disconnected
add_card 2 renderD129 0 0x1002
add_connector 2 eDP-1 connected
run_case 'T2 AMD panel' card2 eDP-1

# The same hardware after apple-gmux switches the internal panel to Intel.
reset_fixture
add_card 0 none 1
add_connector 0 USB-1 connected
add_card 1 renderD128 0 0x8086
add_connector 1 eDP-1 connected
add_card 2 renderD129 1 0x1002
add_connector 2 eDP-1 disconnected
run_case 'T2 Intel panel' card1 eDP-1

# A single NVIDIA GPU keeps its existing software-renderer path while receiving
# the same explicit card pin.
reset_fixture
add_card 0 renderD128 1 0x10de
add_connector 0 eDP-1 connected
run_case 'single NVIDIA panel' card0 eDP-1 1

# A render-capable virtual GPU has no internal connector but remains preferred.
reset_fixture
add_card 0 renderD128 1
add_connector 0 Virtual-1 connected
run_case 'virtio VM' card0 Virtual-1

# A plain bochs VM has scanout but no render node. It is still usable with
# software rendering and must receive both the card pin and pixman renderer.
reset_fixture
add_card 0 none 1
add_connector 0 Virtual-1 connected
run_case 'bochs VM' card0 Virtual-1 1

# A desktop must follow the render GPU that actually has the connected output.
reset_fixture
add_card 0 renderD128 1 0x8086
add_connector 0 HDMI-A-1 disconnected
add_card 1 renderD129 0 0x1002
add_connector 1 DP-1 connected
run_case 'desktop external output' card1 DP-1

# A card modetest cannot open (its DRM driver name differs from the kernel
# driver, as with nvidia-drm) must start weston at once instead of polling
# until the deadline in front of a black screen.
reset_fixture
add_card 0 renderD128 1 0x8086
add_connector 0 eDP-1 connected
started=$(date +%s%N)
RYOKU_GREETER_DRM_ROOT="$tmp/drm" \
  RYOKU_GREETER_CONF="$tmp/missing-greeter.conf" \
  RYOKU_GREETER_PRIMARY_FILE="$tmp/missing-primary" \
  RYOKU_GREETER_NUMLOCK_FILE="$tmp/missing-numlock" \
  RYOKU_GREETER_SETTLE=1 \
  RYOKU_GREETER_DEADLINE=6 \
  RYOKU_TEST_MODETEST_FAIL=1 \
  RYOKU_TEST_MODETEST_LOG="$tmp/run/modetest" \
  RYOKU_TEST_WESTON_LOG="$tmp/run/weston" \
  RYOKU_TEST_LOGGER_LOG="$tmp/run/logger" \
  RYOKU_GREETER_RETRY_LIMIT=1 \
  RYOKU_TEST_CONNECTOR=eDP-1 \
  TMPDIR="$tmp/run" \
  PATH="$bin:$PATH" \
  "$greeter" >/dev/null 2>"$tmp/run/stderr" || fail "unprobeable card did not start weston"
elapsed_ms=$(( ($(date +%s%N) - started) / 1000000 ))
[ "$elapsed_ms" -lt 2000 ] ||
  fail "unprobeable card waited ${elapsed_ms}ms for the settle deadline before starting weston"
grep -Fqx -- '--drm-device=card0' "$tmp/run/weston" || fail "unprobeable card lost its card pin"
printf 'sddm-greeter-card: unprobeable card -> plain weston in %sms\n' "$elapsed_ms"

printf 'sddm-greeter-card: PASS\n'

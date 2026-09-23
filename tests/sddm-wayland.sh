#!/usr/bin/env bash
# Verify the installer writes a native Qt Wayland greeter configuration.
set -euo pipefail

repo=${RYOKU_PATH:-$(cd "$(dirname "$0")/.." && pwd)}
setup=$repo/ryoku/lockscreen/sddm/setup

fail() { printf 'sddm-wayland: %s\n' "$*" >&2; exit 1; }

out=$(RYOKU_DRYRUN=1 "$setup" --dry-run)
grep -Fxq 'qt5-wayland' "$repo/system/packages/base.packages" ||
  fail "base package set omits the Qt5 Wayland plugin needed by Qt5 SDDM"
grep -Fq "  'qt5-wayland'" "$repo/release/packages/ryoku-desktop/PKGBUILD" ||
  fail "ryoku-desktop omits the Qt5 Wayland plugin needed by existing systems"
grep -Fq "  'qt6-5compat'" "$repo/release/packages/ryoku-desktop/PKGBUILD" ||
  fail "ryoku-desktop must retain Qt6 compatibility imports"
for line in \
  '[General]' \
  'DisplayServer=wayland' \
  'GreeterEnvironment=QT_QPA_PLATFORM=wayland' \
  '[Wayland]' \
  'CompositorCommand='; do

  grep -Fq "$line" <<<"$out" || fail "SDDM setup omits $line"
done

# The login screen draws its pointer from the freedesktop "default" cursor theme
# when SDDM's Wayland greeter ignores XCURSOR_THEME; the setup must establish
# that fallback (Inherits the shipped Bibata set) so the pointer is never blank.
# Grep the source, not the dry-run: on a CI box that already has a default theme
# the setup correctly skips the write.
grep -Fq '/usr/share/icons/default' "$setup" ||
  fail "SDDM setup does not establish the default cursor theme (login screen has no pointer)"
grep -Fq 'Inherits=Bibata-Modern-Ice' "$setup" ||
  fail "SDDM setup default cursor theme does not inherit the shipped Bibata set"

# --- the greeter compositor wrapper: the generated weston.ini contract. Runs
# with stubbed modetest/weston so CI needs neither the tools nor a DRM device.
greeter=$repo/ryoku/lockscreen/sddm/ryoku-greeter
gtmp=$(mktemp -d)
trap 'rm -rf "$gtmp"' EXIT
mkdir -p "$gtmp/bin"
printf '#!/bin/sh\nexit 0\n' >"$gtmp/bin/weston"
# two connected panels: an external HDMI that trained first, and an internal
# eDP. Mode lines follow modetest's real columns (index name refresh ...).
cat >"$gtmp/bin/modetest" <<'STUB'
#!/bin/sh
printf 'Connectors:\nid\tencoder\tstatus\t\tname\t\tsize (mm)\tmodes\tencoders\n'
printf '393\t392\tconnected\tHDMI-A-1     \t610x350\t\t2\t392\n'
printf '  modes:\n\tindex name refresh (Hz) hdisp hss hse htot vdisp vss vse vtot\n'
printf '\t  #0 1920x1080 144.00 1920 2000 2020 2080 1080 1100 1120 1160 0 flags: ; type: preferred\n'
printf '\t  #1 1280x720 60.00 1280 1390 1430 1650 720 725 730 750 0 flags: ; type: driver\n'
printf '394\t392\tconnected\teDP-1        \t300x190\t\t1\t392\n'
printf '  modes:\n\tindex name refresh (Hz) hdisp hss hse htot vdisp vss vse vtot\n'
printf '\t  #0 2560x1600 165.00 2560 2608 2640 2720 1600 1603 1609 1732 0 flags: ; type: preferred\n'
STUB
chmod +x "$gtmp/bin/weston" "$gtmp/bin/modetest"
gdry() { PATH="$gtmp/bin:$PATH" RYOKU_GREETER_DRYRUN=1 RYOKU_GREETER_CONF="$gtmp/none.conf" env "$@" "$greeter" 2>/dev/null; }

# The keypad must type digits at the login field: SDDM's own Numlock knob is
# inert under the Wayland greeter, so weston's [keyboard] is the only channel.
gini=$(gdry)
grep -Fq '[keyboard]' <<<"$gini" || fail "greeter config has no [keyboard] section"
grep -Fq 'numlock-on=true' <<<"$gini" || fail "greeter config does not enable NumLock (keypad types navigation keys at login)"
# With no hand-off, the internal panel keeps the pin.
grep -Fq 'name=HDMI-A-1' <<<"$gini" || fail "greeter config dropped the external panel"
# pinned <ini> <name>: the [output] block for <name> carries an app-ids line.
pinned() { awk -v want="$2" '/^\[output\]/{f=0} /^name=/{f=($0=="name="want)} f&&/^app-ids=/{found=1} END{exit !found}' <<<"$1"; }
unpinned() { awk -v want="$2" '/^\[output\]/{f=0} /^name=/{f=($0=="name="want)} f&&/^app-ids=/{found=1} END{exit found}' <<<"$1"; }
pinned "$gini" eDP-1 || fail "greeter did not pin the login form to the internal panel"
unpinned "$gini" HDMI-A-1 || fail "greeter pinned the form to the external panel without a hand-off"

# The session's Displays apply publishes the chosen main; it must outrank the
# internal-panel heuristic, so a docked laptop whose main is the external gets
# its login there too.
printf 'HDMI-A-1\n' >"$gtmp/primary"
gini=$(gdry RYOKU_GREETER_PRIMARY_FILE="$gtmp/primary")
pinned "$gini" HDMI-A-1 || fail "greeter ignored the session's chosen main display"
unpinned "$gini" eDP-1 || fail "greeter pinned the internal panel over the session's choice"

# A stale hand-off (that external is gone now) must degrade to the heuristic.
printf 'DP-9\n' >"$gtmp/primary"
gini=$(gdry RYOKU_GREETER_PRIMARY_FILE="$gtmp/primary")
pinned "$gini" eDP-1 || fail "a stale hand-off must fall back to the internal panel"

# The operator override still outranks everything.
gini=$(gdry RYOKU_GREETER_PRIMARY_FILE="$gtmp/primary" RYOKU_GREETER_PRIMARY=eDP-1)
pinned "$gini" eDP-1 || fail "RYOKU_GREETER_PRIMARY must outrank the session hand-off"

printf 'sddm-wayland: PASS\n'

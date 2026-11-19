#!/usr/bin/env bash
# hermetic test for ryoku-wayland-session, the SDDM [Wayland] SessionCommand
# wrapper that prepares graphics before the session compositor starts.
#
# Nothing here starts weston or a compositor. A fake pgrep on PATH reports weston
# "present" for a controlled number of polls, a fake user manager supplies the
# session environment, a fake session records its argv and environment, and
# RYOKU_DRI_DIR / RYOKU_DRM_CLASS_DIR provide graphics fixtures.
#
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
wrapper="$here/../ryoku/lockscreen/sddm/ryoku-wayland-session"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fail() { echo "FAIL: $1" >&2; exit 1; }

bin="$tmp/bin"; mkdir -p "$bin"

# fake pgrep: reports weston present until it has been polled "$tmp/pglimit"
# times, then gone. Every call bumps "$tmp/pgcount" so the test can see whether
# the wrapper actually waited.
cat >"$bin/pgrep" <<EOF
#!/bin/sh
c=\$(cat "$tmp/pgcount" 2>/dev/null || echo 0); c=\$((c + 1)); echo "\$c" >"$tmp/pgcount"
limit=\$(cat "$tmp/pglimit" 2>/dev/null || echo 0)
[ "\$c" -le "\$limit" ] && exit 0   # weston present
exit 1                              # weston gone
EOF
chmod +x "$bin/pgrep"

cat >"$bin/logger" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >>"$tmp/logger-log"
EOF
chmod +x "$bin/logger"

cat >"$bin/systemctl" <<EOF
#!/bin/sh
[ "\$*" = "--user show-environment" ] || exit 1
cat "$tmp/manager-env"
EOF
chmod +x "$bin/systemctl"
cat >"$bin/loginctl" <<EOF
#!/bin/sh
[ "\$*" = "show-session test-session -p Active --value" ] || exit 1
cat "$tmp/session-active"
EOF
chmod +x "$bin/loginctl"
export PATH="$bin:$PATH"
export XDG_SESSION_ID=test-session

# fake session: records its arguments and environment, and exposes controlled
# stdout, exit, and signal behavior.
cat >"$bin/fake-session" <<EOF
#!/bin/sh
printf '%s' "\$*" >"$tmp/session-args"
{
  printf 'LIBGL_ALWAYS_SOFTWARE=%s\n' "\${LIBGL_ALWAYS_SOFTWARE:-}"
  printf 'MESA_LOADER_DRIVER_OVERRIDE=%s\n' "\${MESA_LOADER_DRIVER_OVERRIDE:-}"
  printf 'GALLIUM_DRIVER=%s\n' "\${GALLIUM_DRIVER:-}"
  printf 'WLR_RENDERER_ALLOW_SOFTWARE=%s\n' "\${WLR_RENDERER_ALLOW_SOFTWARE:-}"
  printf 'XCURSOR_THEME=%s\n' "\${XCURSOR_THEME:-}"
  printf 'QML2_IMPORT_PATH=%s\n' "\${QML2_IMPORT_PATH:-}"
  printf 'QT_QPA_PLATFORM=%s\n' "\${QT_QPA_PLATFORM:-}"
  printf 'GDK_BACKEND=%s\n' "\${GDK_BACKEND:-}"
  printf 'GSK_RENDERER=%s\n' "\${GSK_RENDERER:-}"
  printf 'LIBVA_DRIVER_NAME=%s\n' "\${LIBVA_DRIVER_NAME:-}"
  printf 'NVD_BACKEND=%s\n' "\${NVD_BACKEND:-}"
  printf '__GLX_VENDOR_LIBRARY_NAME=%s\n' "\${__GLX_VENDOR_LIBRARY_NAME:-}"
  printf '__GL_GSYNC_ALLOWED=%s\n' "\${__GL_GSYNC_ALLOWED:-}"
  printf '__GL_VRR_ALLOWED=%s\n' "\${__GL_VRR_ALLOWED:-}"
  printf 'GBM_BACKEND=%s\n' "\${GBM_BACKEND:-}"
} >"$tmp/session-env"
: >"$tmp/session-ran"
[ -z "\${FAKE_SESSION_STDOUT:-}" ] || printf '%s\n' "\$FAKE_SESSION_STDOUT"
if [ "\${FAKE_SESSION_WAIT_FOR_TERM:-0}" = 1 ]; then
  trap 'printf TERM >"$tmp/session-signal"; exit 0' TERM
  : >"$tmp/session-ready"
  while :; do sleep 0.1; done
fi
exit "\${FAKE_SESSION_EXIT:-0}"
EOF
chmod +x "$bin/fake-session"
export RYOKU_WAYLAND_SESSION_DEFAULT="$bin/fake-session"
export RYOKU_NVIDIA_VERSION_PATH="$tmp/no-nvidia-version"
cat >"$tmp/manager-env" <<EOF
PATH=$bin:$PATH
XCURSOR_THEME=Ryoku-Test-Cursor
XCURSOR_SIZE=24
QML_IMPORT_PATH=/tmp/ryoku-qml
QML2_IMPORT_PATH=/tmp/ryoku-qml
QT_QPA_PLATFORMTHEME=qt6ct
GDK_BACKEND=\$'wayland,x11,*'
QT_QPA_PLATFORM=\$'wayland;xcb'
QT_AUTO_SCREEN_SCALE_FACTOR=1
QT_WAYLAND_DISABLE_WINDOWDECORATION=1
ELECTRON_OZONE_PLATFORM_HINT=auto
GSK_RENDERER=gl
RYOKU_POLKIT_AGENT=1
EDITOR=nvim
VISUAL=nvim
EOF

reset() {
  rm -f "$tmp/pgcount" "$tmp/session-ran" "$tmp/session-args" \
    "$tmp/session-env" "$tmp/session-ready" "$tmp/session-signal" \
    "$tmp/logger-log" "$tmp/session-stdout" "$tmp/session-stderr"
  printf 'no\n' >"$tmp/session-active"
}
polls() { cat "$tmp/pgcount" 2>/dev/null || echo 0; }
env_value() { sed -n "s/^$1=//p" "$tmp/session-env"; }

drm_single="$tmp/drm-single"
mkdir -p "$drm_single/card0/device"

reset
echo 3 >"$tmp/pglimit"   # weston present for the first three polls
RYOKU_DRM_CLASS_DIR="$drm_single" RYOKU_SESSION_SETTLE_DEADLINE=10 \
  "$wrapper" Hyprland --foo
[[ -e "$tmp/session-ran" ]] || fail "wrapper never handed off to the session"
[[ "$(cat "$tmp/session-args")" == "Hyprland --foo" ]] \
  || fail "session arguments were not passed through: '$(cat "$tmp/session-args")'"
(( "$(polls)" >= 2 )) || fail "wrapper handed off without waiting for weston (polled $(polls)x)"
[[ "$(env_value XCURSOR_THEME)" == Ryoku-Test-Cursor ]] \
  || fail "shared cursor environment did not reach the session"
[[ "$(env_value QML2_IMPORT_PATH)" == /tmp/ryoku-qml ]] \
  || fail "shared QML import path did not reach the session"
[[ "$(env_value QT_QPA_PLATFORM)" == "wayland;xcb" ]] \
  || fail "Qt backend list was not decoded from the user-manager environment"
[[ "$(env_value GDK_BACKEND)" == "wayland,x11,*" ]] \
  || fail "GTK backend list was not decoded from the user-manager environment"
[[ "$(env_value GSK_RENDERER)" == gl ]] \
  || fail "GTK renderer did not reach the session"

# ---- no greeter: hand off at once, no wasted polling -------------------------
reset
echo 0 >"$tmp/pglimit"   # weston already gone
"$wrapper" Hyprland
[[ -e "$tmp/session-ran" ]] || fail "wrapper did not hand off when weston was absent"
(( "$(polls)" == 1 )) || fail "wrapper polled $(polls)x when weston was already gone; expected 1"

reset
printf 'yes\n' >"$tmp/session-active"
echo 100000 >"$tmp/pglimit"
RYOKU_DRM_CLASS_DIR="$drm_single" "$wrapper" Hyprland
[[ -e "$tmp/session-ran" ]] || fail "active session did not start"
(( "$(polls)" == 1 )) ||
  fail "active session kept polling after logind completed the DRM handoff"

# ---- KMS-only card: force Mesa's software GBM path before compositor start ---
dri_2d="$tmp/dri-2d"
mkdir -p "$dri_2d"
: >"$dri_2d/card0"
reset
echo 0 >"$tmp/pglimit"
env -u LIBGL_ALWAYS_SOFTWARE -u MESA_LOADER_DRIVER_OVERRIDE \
  -u GALLIUM_DRIVER -u WLR_RENDERER_ALLOW_SOFTWARE \
  RYOKU_DRI_DIR="$dri_2d" "$wrapper" niri-session
[[ "$(env_value LIBGL_ALWAYS_SOFTWARE)" == 1 ]] \
  || fail "2D-only DRM card did not select software GL"
[[ "$(env_value MESA_LOADER_DRIVER_OVERRIDE)" == llvmpipe ]] \
  || fail "2D-only DRM card did not select Mesa llvmpipe"
[[ "$(env_value GALLIUM_DRIVER)" == llvmpipe ]] \
  || fail "2D-only DRM card did not select the Gallium llvmpipe driver"
[[ "$(env_value WLR_RENDERER_ALLOW_SOFTWARE)" == 1 ]] \
  || fail "2D-only DRM card did not allow a software compositor renderer"

# ---- render node: keep VM 3D acceleration instead of forcing llvmpipe --------
dri_3d="$tmp/dri-3d"
mkdir -p "$dri_3d"
: >"$dri_3d/card0"
: >"$dri_3d/renderD128"
reset
echo 0 >"$tmp/pglimit"
env -u LIBGL_ALWAYS_SOFTWARE -u MESA_LOADER_DRIVER_OVERRIDE \
  -u GALLIUM_DRIVER -u WLR_RENDERER_ALLOW_SOFTWARE \
  RYOKU_DRI_DIR="$dri_3d" "$wrapper" niri-session
[[ -z "$(env_value LIBGL_ALWAYS_SOFTWARE)" ]] \
  || fail "render-capable VM was forced onto software GL"
[[ -z "$(env_value MESA_LOADER_DRIVER_OVERRIDE)" ]] \
  || fail "render-capable VM was forced onto llvmpipe"

# ---- no DRM card: leave the environment alone; software cannot create KMS ----
dri_empty="$tmp/dri-empty"
mkdir -p "$dri_empty"
reset
echo 0 >"$tmp/pglimit"
env -u LIBGL_ALWAYS_SOFTWARE -u MESA_LOADER_DRIVER_OVERRIDE \
  -u GALLIUM_DRIVER -u WLR_RENDERER_ALLOW_SOFTWARE \
  RYOKU_DRI_DIR="$dri_empty" "$wrapper" Hyprland
[[ -z "$(env_value LIBGL_ALWAYS_SOFTWARE)" ]] \
  || fail "a machine with no KMS card was forced onto software GL"

# ---- NVIDIA panel: carry decode/GLX hints, but never force a GBM backend ------
drm_nvidia="$tmp/drm-nvidia"
mkdir -p "$drm_nvidia/card1-eDP-1" "$drm_nvidia/card1/device"
printf 'connected\n' >"$drm_nvidia/card1-eDP-1/status"
printf '0x10de\n' >"$drm_nvidia/card1/device/vendor"
printf 'test\n' >"$tmp/nvidia-version"
reset
echo 0 >"$tmp/pglimit"
env -u LIBVA_DRIVER_NAME -u NVD_BACKEND -u __GLX_VENDOR_LIBRARY_NAME \
  -u __GL_GSYNC_ALLOWED -u __GL_VRR_ALLOWED -u GBM_BACKEND \
  RYOKU_NVIDIA_VERSION_PATH="$tmp/nvidia-version" \
  RYOKU_DRM_CLASS_DIR="$drm_nvidia" "$wrapper" Hyprland
[[ "$(env_value LIBVA_DRIVER_NAME)" == nvidia ]] \
  || fail "NVIDIA-driven panel did not select NVIDIA VA-API"
[[ "$(env_value NVD_BACKEND)" == direct ]] \
  || fail "NVIDIA-driven panel did not select the direct NVDEC backend"
[[ "$(env_value __GLX_VENDOR_LIBRARY_NAME)" == nvidia ]] \
  || fail "NVIDIA-driven panel did not select NVIDIA GLX"
[[ "$(env_value __GL_GSYNC_ALLOWED)" == 0 && "$(env_value __GL_VRR_ALLOWED)" == 0 ]] \
  || fail "NVIDIA session did not disable driver-side G-SYNC/VRR"
[[ -z "$(env_value GBM_BACKEND)" ]] \
  || fail "session wrapper forced GBM_BACKEND on Hyprland"

# ---- iGPU panel: do not poison hybrid decoding with NVIDIA-only hints --------
drm_hybrid="$tmp/drm-hybrid"
mkdir -p "$drm_hybrid/card0-eDP-1" "$drm_hybrid/card0/device"
printf 'connected\n' >"$drm_hybrid/card0-eDP-1/status"
printf '0x1002\n' >"$drm_hybrid/card0/device/vendor"
reset
echo 0 >"$tmp/pglimit"
env -u LIBVA_DRIVER_NAME -u NVD_BACKEND -u __GLX_VENDOR_LIBRARY_NAME \
  -u __GL_GSYNC_ALLOWED -u __GL_VRR_ALLOWED \
  RYOKU_NVIDIA_VERSION_PATH="$tmp/nvidia-version" \
  RYOKU_DRM_CLASS_DIR="$drm_hybrid" "$wrapper" niri-session
[[ -z "$(env_value LIBVA_DRIVER_NAME)" && -z "$(env_value NVD_BACKEND)" ]] \
  || fail "AMD-driven hybrid panel inherited NVIDIA decode hints"

# ---- greeter never exits: the deadline caps the wait so login never hangs ----
reset
echo 100000 >"$tmp/pglimit"   # weston "present" forever
start=$SECONDS
RYOKU_SESSION_SETTLE_DEADLINE=1 "$wrapper" Hyprland
[[ -e "$tmp/session-ran" ]] || fail "wrapper hung on a greeter that never exits (must cap and hand off)"
(( SECONDS - start <= 4 )) || fail "wrapper waited past its deadline"

# ---- clean and failed exits map to SDDM's distinct helper statuses -----------
reset
echo 0 >"$tmp/pglimit"
FAKE_SESSION_EXIT=0 "$wrapper" test-desktop ||
  fail "a clean session exit did not pass through as status 0"

reset
echo 0 >"$tmp/pglimit"
set +e
FAKE_SESSION_EXIT=1 "$wrapper" test-desktop
status=$?
set -e
(( status == 2 )) ||
  fail "session status 1 became wrapper status $status instead of SDDM session error 2"
failure_log="$(cat "$tmp/logger-log" 2>/dev/null || true)"
[[ $failure_log == *"-t ryoku-session -p user.err"*test-desktop*"status 1"* ]] ||
  fail "failed session did not log its command and status: '$failure_log'"

# ---- SDDM termination reaches the compositor child ---------------------------
reset
echo 0 >"$tmp/pglimit"
FAKE_SESSION_WAIT_FOR_TERM=1 "$wrapper" test-desktop &
wrapper_pid=$!
for _ in {1..100}; do
  [[ -e $tmp/session-ready ]] && break
  sleep 0.01
done
if [[ ! -e $tmp/session-ready ]]; then
  kill -TERM "$wrapper_pid" 2>/dev/null || true
  wait "$wrapper_pid" 2>/dev/null || true
  fail "session child did not become ready for the signal test"
fi
kill -TERM "$wrapper_pid"
wait "$wrapper_pid" ||
  fail "wrapper did not finish cleanly after its child handled TERM"
[[ $(cat "$tmp/session-signal" 2>/dev/null || true) == TERM ]] ||
  fail "TERM sent to the wrapper did not reach the session child"

# ---- compositor stdout follows stderr into SDDM's Wayland session log --------
reset
echo 0 >"$tmp/pglimit"
FAKE_SESSION_STDOUT="compositor stdout" "$wrapper" test-desktop \
  >"$tmp/session-stdout" 2>"$tmp/session-stderr"
[[ ! -s $tmp/session-stdout ]] ||
  fail "session stdout remained on the wrapper's discarded stdout"
[[ $(cat "$tmp/session-stderr") == *"compositor stdout"* ]] ||
  fail "session stdout did not land on the wrapper's stderr"

# ---- stale NVIDIA module warns without delaying or blocking the session -------
cat >"$bin/modinfo-fixture" <<'EOF'
#!/bin/sh
[ "$*" = "-F version nvidia" ] || exit 64
printf '%s\n' 615.78.08
EOF
chmod +x "$bin/modinfo-fixture"
nvidia_version="$tmp/nvidia-version"

reset
echo 0 >"$tmp/pglimit"
printf '%s\n' 615.71.09 >"$nvidia_version"
RYOKU_NVIDIA_VERSION_PATH="$nvidia_version" \
  RYOKU_MODINFO_COMMAND="$bin/modinfo-fixture" \
  "$wrapper" test-desktop
[[ -e $tmp/session-ran ]] ||
  fail "NVIDIA version mismatch blocked the session"
mismatch_log="$(cat "$tmp/logger-log" 2>/dev/null || true)"
[[ $mismatch_log == *"-t ryoku-session -p user.warning"*\
"updated from 615.71.09 to 615.78.08"*\
"computer must restart"* ]] ||
  fail "NVIDIA version mismatch warning was missing or unclear: '$mismatch_log'"

reset
echo 0 >"$tmp/pglimit"
printf '%s\n' 615.78.08 >"$nvidia_version"
RYOKU_NVIDIA_VERSION_PATH="$nvidia_version" \
  RYOKU_MODINFO_COMMAND="$bin/modinfo-fixture" \
  "$wrapper" test-desktop
[[ -e $tmp/session-ran ]] ||
  fail "matching NVIDIA versions blocked the session"
[[ ! -s $tmp/logger-log ]] ||
  fail "matching NVIDIA versions emitted a warning"

# ---- a home without the Ryoku tree is laid before the session starts --------
# fake ryoku: records the call and the state of the tree at that moment, and
# lays the tree the way materialize would, so the order is observable.
cat >"$bin/ryoku" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >>"$tmp/ryoku-log"
[ -e "$tmp/session-ran" ] && printf 'late\n' >>"$tmp/ryoku-log"
mkdir -p "\$XDG_CONFIG_HOME/ryoku" "\$XDG_CONFIG_HOME/quickshell/shell"
: >"\$XDG_CONFIG_HOME/quickshell/shell/shell.qml"
EOF
chmod +x "$bin/ryoku"
reset
rm -rf "${tmp:?}/home"; mkdir -p "$tmp/home"
echo 0 >"$tmp/pglimit"
HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/home/.config" "$wrapper" test-desktop
[[ -e $tmp/session-ran ]] || fail "a home without the tree blocked the session"
[[ $(cat "$tmp/ryoku-log" 2>/dev/null) == materialize ]] ||
  fail "a home without the tree was not materialized before the session: '$(cat "$tmp/ryoku-log" 2>/dev/null)'"

reset
rm -f "$tmp/ryoku-log"
echo 0 >"$tmp/pglimit"
HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/home/.config" "$wrapper" test-desktop
[[ -e $tmp/session-ran ]] || fail "a home with the tree blocked the session"
[[ ! -e $tmp/ryoku-log ]] || fail "a home that has its tree was materialized again"

echo "wayland-session-settle: all checks passed"

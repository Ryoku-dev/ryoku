#!/usr/bin/env bash
# The greeter compositor wrapper supervises weston. SDDM 0.21 reads any death of
# the Wayland greeter as a finished login and starts nothing afterwards, so the
# wrapper must start weston again when it dies on its own, re-choosing the card,
# report each death to the system log, and still stop when SDDM asks it to.
# Nothing here touches the host: a stub weston scripted per attempt, a stub
# logger, and a DRM fixture directory.
set -euo pipefail
repo=${RYOKU_PATH:-$(cd "$(dirname "$0")/.." && pwd)}
greeter=$repo/ryoku/lockscreen/sddm/ryoku-greeter
tmp=$(mktemp -d)
wrapper=""
trap 'rm -rf "$tmp"; if [ -n "$wrapper" ]; then kill "$wrapper" 2>/dev/null || true; fi' EXIT
bin=$tmp/bin
mkdir -p "$bin" "$tmp/drm" "$tmp/run"
fail() { printf 'sddm-greeter-supervise: %s\n' "$*" >&2; exit 1; }

# weston stub: each attempt appends its argv and runs the next line of the
# script file: "fail <status> <message>" exits with status after printing the
# message on stderr; "stay" keeps running until TERM, then exits 0; "quit"
# exits 0 at once (a death too, nobody asked for it).
cat >"$bin/weston" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" | sed 's/--config=[^ ]*/--config=<generated>/' >>"$RYOKU_TEST_WESTON_ARGS"
step=$(head -n 1 "$RYOKU_TEST_WESTON_SCRIPT")
sed -i '1d' "$RYOKU_TEST_WESTON_SCRIPT"
case "$step" in
  fail*) read -r _ status message <<<"$step"; echo "$message" >&2; exit "$status" ;;
  stay)
    trap 'exit 0' TERM
    while :; do sleep 0.2; done ;;
  *) exit 0 ;;
esac
STUB
cat >"$bin/logger" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$RYOKU_TEST_LOGGER_LOG"
STUB
# modetest stub: one connected panel with one mode, so the card path composes a
# config without ever opening a host device.
cat >"$bin/modetest" <<'STUB'
#!/usr/bin/env bash
printf 'Connectors:\nid\tencoder\tstatus\t\tname\t\tsize (mm)\tmodes\tencoders\n'
printf '31\t30\tconnected\teDP-1\t300x190\t\t1\t30\n'
printf '  modes:\n\tindex name refresh (Hz) hdisp hss hse htot vdisp vss vse vtot\n'
printf '\t  #0 1920x1080 60.00 1920 2000 2020 2080 1080 1100 1120 1160 0 flags: ; type: preferred\n'
STUB
chmod +x "$bin/weston" "$bin/logger" "$bin/modetest"

add_card() {
  local card=$tmp/drm/card$1
  mkdir -p "$card/device/drm" "$tmp/drm/card$1-eDP-1"
  printf '1\n' >"$card/device/boot_vga"
  printf '0x1002\n' >"$card/device/vendor"
  printf 'DRIVER=amdgpu\n' >"$card/device/uevent"
  : >"$card/device/drm/renderD128"
  printf 'connected\n' >"$tmp/drm/card$1-eDP-1/status"
}

# becomes the wrapper (exec), so run it in the background or in a subshell.
run_wrapper() {
  : >"$tmp/run/args"
  : >"$tmp/run/logger"
  RYOKU_GREETER_DRM_ROOT="$tmp/drm" \
    RYOKU_GREETER_CONF="$tmp/none.conf" \
    RYOKU_GREETER_PRIMARY_FILE="$tmp/none-primary" \
    RYOKU_GREETER_NUMLOCK_FILE="$tmp/none-numlock" \
    RYOKU_TEST_WESTON_ARGS="$tmp/run/args" \
    RYOKU_TEST_WESTON_SCRIPT="$tmp/run/script" \
    RYOKU_TEST_LOGGER_LOG="$tmp/run/logger" \
    RYOKU_GREETER_SETTLE=0 \
    RYOKU_GREETER_DEADLINE=0 \
    TMPDIR="$tmp/run" \
    PATH="$bin:$PATH" \
    exec "$@" "$greeter" 2>"$tmp/run/stderr"
}

# --- a weston that dies twice then stays: three starts, two reports, the card
# chosen again on each retry (it appears during the first pause), and a stop
# request ends everything.
rm -rf "$tmp/drm"/*
printf 'fail 1 fatal: no DRM device found\nfail 1 fatal: failed to create compositor backend\nstay\n' >"$tmp/run/script"
run_wrapper env &
wrapper=$!
sleep 0.5
add_card 0   # the driver arrives while the wrapper is between attempts
for _ in $(seq 1 100); do
  [ "$(wc -l <"$tmp/run/args")" -ge 3 ] && break
  sleep 0.1
done
[ "$(wc -l <"$tmp/run/args")" -eq 3 ] || fail "expected three weston starts, got: $(cat "$tmp/run/args")"
grep -q -- '--drm-device=card0' <(sed -n 3p "$tmp/run/args") ||
  fail "the retry did not choose the card that appeared: $(sed -n 3p "$tmp/run/args")"
grep -q -- '--drm-device' <(sed -n 1p "$tmp/run/args") &&
  fail "the first attempt pinned a card that did not exist"
[ "$(wc -l <"$tmp/run/logger")" -eq 2 ] || fail "expected two death reports, got: $(cat "$tmp/run/logger")"
grep -q 'weston exited with status 1 (attempt 1, card none): fatal: no DRM device found' "$tmp/run/logger" ||
  fail "the first report lacks status, attempt, card or weston's last line: $(sed -n 1p "$tmp/run/logger")"
grep -q 'weston exited with status 1 (attempt 2, card card0): fatal: failed to create compositor backend' "$tmp/run/logger" ||
  fail "the second report does not name the re-chosen card: $(sed -n 2p "$tmp/run/logger")"
grep -q -- '--drm-device=card0' <(sed -n 2p "$tmp/run/args") || fail "the first retry did not re-choose the card"
grep -q -- '-t ryoku-greeter -p daemon.err' "$tmp/run/logger" || fail "reports do not go to the system log under the ryoku-greeter tag"
kill -0 "$wrapper" 2>/dev/null || fail "the wrapper exited while weston was still running"
kill -TERM "$wrapper"
wait "$wrapper" && status=0 || status=$?
[ "$status" -eq 0 ] || fail "a stop request did not end the wrapper cleanly (status $status)"
pgrep -f "$bin/weston" >/dev/null && fail "weston outlived the wrapper after the stop request"
[ "$(wc -l <"$tmp/run/logger")" -eq 2 ] || fail "the requested stop was reported as a death"

# --- a weston that exits 0 on its own is a death too: it is started again.
printf 'quit\nstay\n' >"$tmp/run/script"
run_wrapper env &
wrapper=$!
for _ in $(seq 1 100); do
  [ "$(wc -l <"$tmp/run/args")" -ge 2 ] && break
  sleep 0.1
done
[ "$(wc -l <"$tmp/run/args")" -eq 2 ] || fail "a clean exit nobody asked for was not retried"
grep -q 'weston exited with status 0 (attempt 1' "$tmp/run/logger" || fail "the clean exit was not reported: $(cat "$tmp/run/logger")"
kill -TERM "$wrapper"
wait "$wrapper" || true

# --- the retry limit bounds the loop; the last status is the wrapper's own.
printf 'fail 3 a\nfail 3 b\nfail 3 c\nfail 3 d\n' >"$tmp/run/script"
(run_wrapper env RYOKU_GREETER_RETRY_LIMIT=2) && status=0 || status=$?
[ "$status" -eq 3 ] || fail "the wrapper did not exit with weston's status after the retry limit (got $status)"
[ "$(wc -l <"$tmp/run/args")" -eq 2 ] || fail "the retry limit was not honoured: $(wc -l <"$tmp/run/args") starts"

printf 'sddm-greeter-supervise: PASS\n'

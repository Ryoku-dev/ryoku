#!/usr/bin/env bash
# Live behavioral checks for the Void init translation. Runs the actual
# translated service scripts under a real `runsvdir` inside a Void Linux
# container (ghcr.io/void-linux/void-glibc), against stub binaries, and asserts
# the behaviors the static half (tests/void-init.sh) cannot: oneshot parking
# and re-run, restart-on-crash vs park-on-clean, the ExecStop down sentinel,
# BindsTo down-propagation, the timer loop, failed-wait parking, the wait-for
# predicates, and the envdir renderer.
#
# This is the empirical basis for the runit semantics the translations rely on
# (a finish exit code does not gate restarts; only an sv down request parks,
# and finish sees $1=-1 $2=15 for one). If Void's runit ever changes that,
# this test goes red first.
#
#   tests/void-init-runit.sh            # needs docker + network (pulls the image)
#   RYOKU_VOID_IMAGE=... tests/void-init-runit.sh
set -euo pipefail

ROOT=${RYOKU_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
IMAGE=${RYOKU_VOID_IMAGE:-ghcr.io/void-linux/void-glibc:latest}

command -v docker >/dev/null 2>&1 || { echo "void-init-runit: docker not available; skipping live runit checks" >&2; exit 0; }

# The in-container harness: one POSIX sh script (Void's /bin/sh is dash), the
# repo bind-mounted read-only at /repo. One runsvdir for the whole suite;
# tests steer it with sv up/sv down and the stub control files. Services a
# test wants to start itself are installed with a `down` file so runsvdir
# does not auto-start them.
HARNESS=$(cat <<'HARNESS_EOF'
set -e
xbps-install -Sy runit >/dev/null 2>&1

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "ok: $*"; }

export WORK=/tmp/rt
rm -rf "$WORK"
mkdir -p "$WORK/bin" "$WORK/ctl" "$WORK/svc" "$WORK/home/.config/quickshell/shell" "$WORK/run"
export RYOKU_BIN_DIR="$WORK/bin"
export RYOKU_INIT_LIB=/repo/void/init/lib
export RYOKU_WAIT_BUDGET_SCALE=0   # waits poll once, never sleep-loop
export HOME="$WORK/home"
export XDG_RUNTIME_DIR="$WORK/run"
export RYOKU_BT_SYS="$WORK/fakesys"
export LOG="$WORK/events"
: > "$LOG"
# stub binaries: log "name args", then behave per $WORK/ctl/<name>:
# run (default) = sleep forever, exit0 = clean stop, crash = exit 9.
mkstub() {
	cat > "$WORK/bin/$1" <<EOF
#!/bin/sh
echo "$1 \$*" >> "$LOG"
case \$(cat "$WORK/ctl/$1" 2>/dev/null || echo run) in
	exit0) exit 0 ;;
	crash) exit 9 ;;
	*)     exec sleep 300 ;;
esac
EOF
	chmod +x "$WORK/bin/$1"
}
for b in ryoku-shell ryoku-qylock-activate ryoku-power-cutover ryoku-hub ryogami \
	ryoku-idle ryoku-clamshell pipewire bluetoothctl ryoku-rashin prowl \
	ryoku-palette-bridge ryoku claude-usage codex-usage opencode-usage nmcli; do
	mkstub "$b"
done

# argument-aware stubs: a binary that is both a pre-start hook and the daemon
# must exit for the hook and stay up for the daemon.
cat > "$WORK/bin/ryoku-shell" <<'EOF'
#!/bin/sh
echo "ryoku-shell $*" >> "$LOG"
[ "$1" = quit ] && exit 0
case $(cat "$WORK/ctl/ryoku-shell" 2>/dev/null || echo run) in
	exit0) exit 0 ;;
	crash) exit 9 ;;
	*)     exec sleep 300 ;;
esac
EOF
cat > "$WORK/bin/prowl" <<'EOF'
#!/bin/sh
echo "prowl $*" >> "$LOG"
[ "$2" = down ] && exit 0
case $(cat "$WORK/ctl/prowl" 2>/dev/null || echo run) in
	exit0) exit 0 ;;
	crash) exit 9 ;;
	*)     exec sleep 300 ;;
esac
EOF
chmod +x "$WORK/bin/ryoku-shell" "$WORK/bin/prowl"

# hook-only and loop-collector stubs must return, never sleep.
for b in ryoku-qylock-activate ryoku-power-cutover ryoku-hub ryoku \
	claude-usage codex-usage opencode-usage nmcli; do
	echo exit0 > "$WORK/ctl/$b"
done

# the ai-usage loop must not actually sleep 45s/300s in the test
export RYOKU_AI_USAGE_INITIAL=0 RYOKU_AI_USAGE_PERIOD=1

install_svc() { # repo-dir name [park]
	mkdir -p "$WORK/svc/$2"
	cp "$1/run" "$WORK/svc/$2/run"; chmod +x "$WORK/svc/$2/run"
	if [ -f "$1/finish" ]; then cp "$1/finish" "$WORK/svc/$2/finish"; chmod +x "$WORK/svc/$2/finish"; fi
	[ "${3:-}" = park ] && touch "$WORK/svc/$2/down"
	return 0
}
install_svc /repo/void/init/user/ryoku-shell ryoku-shell
install_svc /repo/void/init/user/ryoku-bootstrap ryoku-bootstrap
install_svc /repo/void/init/user/ryoku-idle ryoku-idle park
install_svc /repo/void/init/user/ryoku-eq ryoku-eq
install_svc /repo/void/init/user/ryoku-rashin ryoku-rashin park
install_svc /repo/void/init/user/ryoku-prowl ryoku-prowl park
install_svc /repo/void/init/user/ryoku-ai-usage ryoku-ai-usage
install_svc /repo/void/init/user/ryoku-bluetooth-reset ryoku-bluetooth-reset
install_svc /repo/void/init/system/ryoku-var-state ryoku-var-state
install_svc /repo/void/init/system/ryoku-network-kill-guard ryoku-network-kill-guard

# preconditions:
# - bootstrap parks (config already materialized)
mkdir -p "$HOME/.config/ryoku"; touch "$HOME/.config/quickshell/shell/shell.qml"
# - ryoku-shell needs WAYLAND_DISPLAY (ConditionEnvironment)
export WAYLAND_DISPLAY=wayland-0
# - eq's filter-chain.conf stays absent: it must park
# - bluetooth-reset: fake radio present, but no session D-Bus in this
#   container, so it must park
mkdir -p "$RYOKU_BT_SYS/class/bluetooth"; touch "$RYOKU_BT_SYS/class/bluetooth/hci0"
# - network-kill marker absent: the guard must park

runsvdir "$WORK/svc" >/dev/null 2>&1 &
RSD=$!
trap 'kill $RSD 2>/dev/null || true' EXIT

# grep -c prints 0 and exits 1 on no match; `|| true` keeps the single 0 line.
count() { grep -c "^$1" "$LOG" 2>/dev/null || true; }
stat_of() { cat "$WORK/svc/$1/supervise/stat" 2>/dev/null || echo none; }
wait_stat() { # svc expected [timeout-quarters]
	i=0; while [ "$i" -lt "${3:-80}" ]; do
		[ "$(stat_of "$1")" = "$2" ] && return 0
		sleep 0.25; i=$((i+1))
	done
	return 1
}
wait_count_ge() { # pattern n [timeout-quarters]
	i=0; while [ "$i" -lt "${3:-80}" ]; do
		[ "$(count "$1")" -ge "$2" ] && return 0
		sleep 0.25; i=$((i+1))
	done
	return 1
}

#### 1. failed-wait parking: eq (missing config file) and bluetooth-reset
####    (no session D-Bus) park instead of crash-looping.
wait_stat ryoku-eq down || fail "ryoku-eq did not park when its ConditionPathExists file is absent"
[ "$(count 'pipewire -c')" -eq 0 ] || fail "ryoku-eq started pipewire despite a missing config"
wait_stat ryoku-bluetooth-reset down || fail "bluetooth-reset did not park without a session bus"
pass "failed condition/ordering waits park, never crash-loop"

#### 2. system oneshots: var-state creates the tree and parks; the network
####    kill guard parks without its armed marker.
wait_stat ryoku-var-state down || fail "var-state did not park after running"
[ -d /var/lib/ryoku/boot ] || fail "var-state did not create /var/lib/ryoku/boot"
[ "$(stat -c %a /var/lib/ryoku/boot)" = 1777 ] || fail "var-state boot dir mode is $(stat -c %a /var/lib/ryoku/boot), want 1777"
[ "$(stat -c %a /var/lib/ryoku/greeter-primary)" = 666 ] || fail "var-state greeter-primary mode wrong"
wait_stat ryoku-network-kill-guard down || fail "kill-guard did not park without its marker"
[ "$(count 'ryoku-network-kill boot')" -eq 0 ] || fail "kill-guard ran the helper without its armed marker"
pass "system oneshots run their work and park; conditions honored"

#### 3. oneshot condition flip + re-run: bootstrap parks with config present,
####    does its work after the config vanishes, then parks again.
wait_stat ryoku-bootstrap down || fail "bootstrap did not park with config present"
[ "$(count 'ryoku materialize')" -eq 0 ] || fail "bootstrap materialized despite existing config"
rm -rf "$HOME/.config/ryoku"
sv up "$WORK/svc/ryoku-bootstrap"
wait_count_ge 'ryoku materialize' 1 || fail "bootstrap did not materialize after the config vanished"
wait_stat ryoku-bootstrap down || fail "bootstrap did not park after materialize"
pass "oneshot re-runs on sv up and parks after its work"

#### 4. Restart=always: ryoku-shell crash-loops under runsv, never parks.
####    sv term (not restart): a crash-looping service is down more often
####    than up, so sv's up-wait would time out; TERM just kills the run
####    child and runsv re-runs it into the crash mode.
echo crash > "$WORK/ctl/ryoku-shell"
before4=$(count 'ryoku-shell daemon')
sv term "$WORK/svc/ryoku-shell" || true
wait_count_ge 'ryoku-shell daemon' $((before4+3)) 120 || fail "ryoku-shell did not keep re-running after crashes (Restart=always), count=$(count 'ryoku-shell daemon')"
pass "Restart=always re-runs the daemon after a crash"

#### 5. ExecStop gating: qylock --prepare-stop fires only on a requested down.
echo run > "$WORK/ctl/ryoku-shell"
wait_stat ryoku-shell run || fail "ryoku-shell did not come back up in run mode"
before=$(count 'ryoku-qylock-activate --prepare-stop')
sv down "$WORK/svc/ryoku-shell"
wait_stat ryoku-shell down || fail "ryoku-shell did not stop on sv down"
wait_count_ge 'ryoku-qylock-activate --prepare-stop' $((before+1)) || fail "ExecStop hook did not fire on the requested down"
sleep 1
[ "$(count 'ryoku-qylock-activate --prepare-stop')" -eq $((before+1)) ] || fail "ExecStop hook fired more than once for one down"
pass "ExecStop hook fires on requested down only (finish -1 sentinel)"

#### 6. Restart=on-failure: clean self-exit parks and stays parked.
echo exit0 > "$WORK/ctl/ryoku-idle"
sv up "$WORK/svc/ryoku-idle"
wait_count_ge 'ryoku-idle start' 1 || fail "ryoku-idle never ran"
wait_stat ryoku-idle down || fail "ryoku-idle did not park on a clean exit (Restart=on-failure)"
sleep 3
[ "$(stat_of ryoku-idle)" = down ] || fail "ryoku-idle restarted after a clean exit"
pass "Restart=on-failure parks on clean exit and stays parked"

#### 7. BindsTo down-propagation: rashin brings prowl up (Wants) and down.
echo run > "$WORK/ctl/ryoku-rashin"; echo run > "$WORK/ctl/prowl"
sv up "$WORK/svc/ryoku-rashin"
wait_stat ryoku-prowl run || fail "prowl did not come up with rashin (Wants)"
wait_count_ge 'prowl gateway serve --port 8788' 1 || fail "prowl never served"
[ "$(count 'prowl gateway down --port 8788')" -ge 1 ] || fail "prowl ExecStartPre (gateway down) did not run"
sv down "$WORK/svc/ryoku-rashin"
wait_stat ryoku-prowl down || fail "prowl did not go down when rashin stopped (BindsTo)"
pass "Wants brings the child up; BindsTo takes it down with the parent"

#### 8. timer translation: the ai-usage loop fires its collectors on schedule.
wait_count_ge 'claude-usage' 1 || fail "ai-usage loop never ran claude-usage"
sleep 3
[ "$(count 'codex-usage')" -ge 2 ] || fail "ai-usage loop did not repeat on its period"
pass "timer translation loops on its schedule"

#### 9. wait-for predicates (env, file, !-inverse).
. /repo/void/init/lib/wait-for
export RYOKU_WAIT_BUDGET_SCALE=1
FOO=bar; export FOO
wait_env FOO 2 || fail "wait_env did not see an exported var"
unset FOO
if wait_env FOO 1; then fail "wait_env matched an unset var"; fi
touch "$WORK/afile"
wait_file "$WORK/afile" 2 || fail "wait_file did not see an existing file"
if wait_file "!$WORK/afile" 1; then fail "wait_file ! matched an existing file"; fi
rm -f "$WORK/afile"
wait_file "!$WORK/afile" 2 || fail "wait_file ! did not match an absent file"
pass "wait-for predicates behave"

#### 10. xdg-dirs renders Turnstile envdir files with no trailing newline.
mkdir -p "$WORK/xdgbin"
cat > "$WORK/xdgbin/xdg-user-dir" <<'EOF'
#!/bin/sh
printf '%s' "$HOME/Localized$1"
EOF
chmod +x "$WORK/xdgbin/xdg-user-dir"
printf 'XDG_PICTURES_DIR="$HOME/Imagens"\n' > "$HOME/.config/user-dirs.dirs"
out="$WORK/envdir"
PATH="$WORK/xdgbin:$PATH" /repo/void/init/env/xdg-dirs "$out"
[ -f "$out/XDG_PICTURES_DIR" ] || fail "xdg-dirs did not render XDG_PICTURES_DIR"
[ "$(cat "$out/XDG_PICTURES_DIR")" = "$HOME/LocalizedPICTURES" ] || fail "xdg-dirs value wrong: $(cat "$out/XDG_PICTURES_DIR")"
lastbyte=$(od -An -tx1 "$out/XDG_PICTURES_DIR" | tr -d ' \n' | tail -c2)
[ "$lastbyte" = 0a ] && fail "xdg-dirs wrote a trailing newline (envdir values must not have one)"
pass "xdg-dirs renders Turnstile envdir files"

kill $RSD 2>/dev/null || true
echo "void-init live runit checks passed"
HARNESS_EOF
)

printf '%s' "$HARNESS" > /tmp/void-init-runit-harness.sh
docker run --rm -v "$ROOT:/repo:ro" -v /tmp/void-init-runit-harness.sh:/harness.sh:ro "$IMAGE" sh /harness.sh

#!/usr/bin/env bash
# Static contract checks for the Void init translation (void/init/).
# Host-side, no container: manifest completeness, script hygiene, and parity
# between each systemd unit and its runit translation. The behavioral half
# (real runsvdir) is tests/void-init-runit.sh.
set -euo pipefail

ROOT=${RYOKU_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
INIT="$ROOT/void/init"
MANIFEST="$INIT/translations.tsv"

fail() { echo "void-init: $*" >&2; exit 1; }

[[ -f $MANIFEST ]] || fail "missing manifest: $MANIFEST"

# --- 1. completeness: every systemd artifact in the repo has a manifest row.
# The archiso live-image units are covered by one glob row (the Void ISO is a
# future milestone); everything else must be named exactly.
missing=()
while IFS= read -r unit; do
	case $unit in
		installation/iso/airootfs/*) continue ;;
	esac
	grep -qxF "$unit" <(cut -f1 "$MANIFEST" | grep -v '^#' | grep -v '^source$') \
		|| missing+=("$unit")
done < <(cd "$ROOT" && find ryoku system -type f \( -name '*.service' -o -name '*.timer' -o -name '*.target' \) | sort)
((${#missing[@]} == 0)) || fail "systemd units with no manifest row: ${missing[*]}"

# the environment generator and the polkit rule are not units but are part of
# the init surface; they must be in the manifest too.
for extra in \
	ryoku/shell/systemd/user-environment-generators/60-ryoku-xdg-dirs \
	system/hardware/bluetooth/54-ryoku-bluetooth-a2dp.rules \
	ryoku/cli/systemd/ryoku.tmpfiles.conf; do
	grep -qxF "$extra" <(cut -f1 "$MANIFEST") || fail "manifest row missing for $extra"
done

# --- 2. manifest integrity: valid verdicts, translation paths exist, no dupes.
dupes=$(cut -f1 "$MANIFEST" | grep -v '^#' | grep -v '^source$' | sort | uniq -d)
[[ -z $dupes ]] || fail "duplicate manifest rows: $dupes"
while IFS=$'\t' read -r src verdict trans _notes; do
	[[ $src == '#'* || $src == source ]] && continue
	[[ -n ${verdict:-} ]] || fail "row without verdict: $src"
	case $verdict in
		translated)
			[[ -e $ROOT/$trans ]] || fail "manifest points at missing translation: $trans (from $src)"
			;;
		accepted-loss)
			[[ $trans == - ]] || fail "accepted-loss row must not name a translation: $src"
			;;
		*) fail "unknown verdict '$verdict' for $src" ;;
	esac
done < "$MANIFEST"

# --- 3. every translated service directory is a valid runit service:
# executable POSIX scripts, correct user-service defaults, and shellcheck-clean.
for helper in "$INIT/lib/wait-for" "$INIT/env/xdg-dirs" "$INIT/session/session-start"; do
	[[ -x $helper ]] || fail "helper is not executable: $helper"
	head -1 "$helper" | grep -q '^#!/bin/sh$' || fail "helper must be #!/bin/sh (Void dash): $helper"
done

session_start=$INIT/session/session-start
ensure_line=$(grep -nF 'ryoku-host session ensure --user' "$session_start" | cut -d: -f1)
roster_line=$(grep -nF 'while IFS= read -r name' "$session_start" | cut -d: -f1)
[[ -n $ensure_line && -n $roster_line && $ensure_line -lt $roster_line ]] \
	|| fail "session-start does not provision user services before starting the roster"
grep -qF '$service/.ryoku-disabled' "$session_start" \
	|| fail "session-start does not preserve user-disabled services"

deploy=$ROOT/ryoku/shell/deploy.sh
grep -qF 'sudo cp -R "$init_root/user/." /usr/lib/ryoku/runit/user/' "$deploy" \
	|| fail "dev deploy does not install packaged user service sources"
grep -qF '/usr/bin/ryoku-host session ensure --user' "$deploy" \
	|| fail "dev deploy does not use the host provisioning path"
if grep -qF 'cp -a "$src/." "$runit_service_root/$name/"' "$deploy"; then
	fail "dev deploy still carries a second user service provisioning loop"
fi

login_services=(pipewire pipewire-pulse wireplumber)
is_login_service() {
	local candidate=${1%/}
	candidate=${candidate##*/}
	local name
	for name in "${login_services[@]}"; do
		[[ $candidate == "$name" ]] && return 0
	done
	return 1
}

for name in "${login_services[@]}"; do
	[[ -d $INIT/user/$name ]] || fail "login service is missing: $name"
done

while IFS= read -r svc; do
	[[ -x $svc/run ]] || fail "service run is not executable: $svc"
	head -1 "$svc/run" | grep -q '^#!/bin/sh$' || fail "run must be #!/bin/sh (Void dash): $svc/run"
	grep -qF '. "${RYOKU_INIT_LIB:-/usr/lib/ryoku/runit}/wait-for"' "$svc/run" \
		|| fail "run does not source the installed wait-for helper: $svc/run"
	grep -q '^load_session_env$' "$svc/run" || fail "run does not load the session env: $svc/run"
	if [[ $svc == "$INIT/user/"* ]]; then
		if is_login_service "$svc"; then
			[[ ! -e $svc/down ]] || fail "login service must not have a down marker: $svc"
		else
			[[ -f $svc/down && ! -s $svc/down ]] || fail "session service needs an empty down marker: $svc"
		fi
	fi
	if [[ -f $svc/finish ]]; then
		[[ -x $svc/finish ]] || fail "service finish is not executable: $svc"
		head -1 "$svc/finish" | grep -q '^#!/bin/sh$' || fail "finish must be #!/bin/sh: $svc/finish"
	fi
done < <(find "$INIT/user" "$INIT/system" -mindepth 1 -maxdepth 1 -type d | sort)

if command -v shellcheck >/dev/null 2>&1; then
	while IFS= read -r script; do
		shellcheck -s sh -e SC1091 "$script" || fail "shellcheck: $script"
	done < <(find "$INIT" -type f \( -name run -o -name finish -o -name wait-for -o -name xdg-dirs -o -name session-start \) | sort)
fi

for run in "$INIT"/user/*/run; do
	if grep -q '^wait_env ' "$run"; then
		grep -A1 '^wait_env ' "$run" | grep -q '^load_session_env$' \
			|| fail "env-gated service does not reload the envdir after its wait: $run"
	fi
done

grep -qF 'ryoku-host session start xdg-desktop-portal-gnome.service xdg-desktop-portal-gtk.service' "$ROOT/ryoku/niri/autostart.kdl" \
	|| fail "niri autostart does not use the host session entrypoint"
grep -qF 'ryoku-host session start xdg-desktop-portal-hyprland.service xdg-desktop-portal-gtk.service' "$ROOT/ryoku/hyprland/modules/autostart.lua" \
	|| fail "Hyprland autostart does not use the host session entrypoint"

# --- 4. roster integrity: every named service exists, every session user
# service is either in the roster or documented as on-demand in its comments.
roster=$INIT/session-services
[[ -f $roster ]] || fail "missing session roster: $roster"
while IFS= read -r name; do
	[[ -z $name || $name == '#'* ]] && continue
	[[ -d $INIT/user/$name ]] || fail "roster names a service that does not exist: $name"
done < "$roster"
for svc in "$INIT"/user/*/; do
	is_login_service "$svc" && continue
	name=$(basename "$svc")
	grep -qxF "$name" "$roster" || grep -qF "$name" "$roster" \
		|| fail "user service $name is neither in the roster nor documented as on-demand there"
done

# --- 5. parity: each translation carries its unit's observable contract.
# Environment= keys must be exported by the run script; the ExecStart binary
# must appear in it; Restart=on-failure units must have a finish that parks on
# clean exit; Type=oneshot finishes must park unconditionally.
parity_fail() { fail "parity: $*"; }

unit_env_keys() { grep -E '^Environment=' "$1" | sed 's/^Environment=//; s/=.*//'; }

check_parity() {
	local unit=$1 svc=$2
	local run=$svc/run finish=$svc/finish key

	while IFS= read -r key; do
		[[ -n $key ]] || continue
		grep -qE "export $key=" "$run" || parity_fail "$key from $unit not exported in $run"
	done < <(unit_env_keys "$unit")

	# the main binary of ExecStart= (last, non-prefixed line) must be invoked.
	local bin
	bin=$(grep -E '^ExecStart=' "$unit" | tail -1 | sed 's/^ExecStart=//; s/^@//; s/^-//' | awk '{print $1}')
	if [[ -n $bin ]]; then
		local base=${bin##*/}
		if [[ $base == systemctl ]]; then
			grep -v '^[[:space:]]*#' "$run" | grep -qE '(^|[[:space:]])(sv|ryoku-host svc) ' \
				|| parity_fail "systemctl ExecStart of $unit has no sv or ryoku-host svc call in $run"
		else
			grep -qF "$base" "$run" || parity_fail "ExecStart binary $base of $unit not found in $run"
		fi
	fi

	if grep -q '^Restart=on-failure' "$unit"; then
		[[ -f $finish ]] || parity_fail "$unit has Restart=on-failure but $svc has no finish"
		grep -q 'sv down' "$finish" || parity_fail "$finish must park via sv down (verified runit semantics)"
	fi
	if grep -q '^Type=oneshot' "$unit"; then
		grep -q 'sv down' "$run" || parity_fail "oneshot $run must park itself via sv down"
		[[ -f $finish ]] || parity_fail "oneshot $svc needs a parking finish for the failure path"
		grep -q 'sv down' "$finish" || parity_fail "oneshot $finish must park on failure"
	fi
	if grep -q '^ConditionEnvironment=' "$unit"; then
		local var
		var=$(grep '^ConditionEnvironment=' "$unit" | head -1 | sed 's/^ConditionEnvironment=//')
		grep -qF "wait_env $var" "$run" || parity_fail "ConditionEnvironment=$var of $unit not waited in $run"
	fi
	if grep -qE '^ConditionPathExists=' "$unit"; then
		grep -q 'wait_file\|ConditionPathExists' "$run" || \
			grep -qE '\-e |\[ -d |\[ -f ' "$run" || \
			parity_fail "ConditionPathExists of $unit not translated in $run"
	fi
}

check_parity "$ROOT/ryoku/shell/systemd/user/ryoku-shell.service" "$INIT/user/ryoku-shell"
check_parity "$ROOT/ryoku/shell/systemd/user/ryogami.service" "$INIT/user/ryogami"
check_parity "$ROOT/ryoku/shell/systemd/user/ryoku-idle.service" "$INIT/user/ryoku-idle"
check_parity "$ROOT/ryoku/shell/systemd/user/ryoku-clamshell.service" "$INIT/user/ryoku-clamshell"
check_parity "$ROOT/ryoku/shell/systemd/user/ryoku-eq.service" "$INIT/user/ryoku-eq"
check_parity "$ROOT/ryoku/shell/systemd/user/ryoku-bt-agent.service" "$INIT/user/ryoku-bt-agent"
check_parity "$ROOT/ryoku/shell/systemd/user/ryoku-bootstrap.service" "$INIT/user/ryoku-bootstrap"
check_parity "$ROOT/ryoku/rashin/systemd/ryoku-rashin.service" "$INIT/user/ryoku-rashin"
check_parity "$ROOT/ryoku/rashin/systemd/ryoku-prowl.service" "$INIT/user/ryoku-prowl"
check_parity "$ROOT/ryoku/palette-bridge/packaging/systemd/ryoku-palette-bridge.service" "$INIT/user/ryoku-palette-bridge"
check_parity "$ROOT/system/hardware/bluetooth/ryoku-bluetooth-reset.service" "$INIT/user/ryoku-bluetooth-reset"
# the loop service's binary set comes from the timer+service pair; check all three collectors appear
for tool in claude-usage codex-usage opencode-usage; do
	grep -qF "$tool" "$INIT/user/ryoku-ai-usage/run" || parity_fail "ai-usage loop lost $tool"
done
# boot guard + var-state: oneshot contract without a ConditionEnvironment
check_parity "$ROOT/ryoku/cli/systemd/ryoku-boot-guard.service" "$INIT/system/ryoku-boot-guard"
grep -q 'boot-guard' "$INIT/system/ryoku-boot-guard/run" || parity_fail "boot-guard run lost the ryoku boot-guard call"
# var-state must carry every tmpfiles path and mode
for entry in '/var/lib/ryoku 0755' '/var/lib/ryoku/boot 1777' 'greeter-primary 0666' 'greeter-numlock 0666'; do
	path=${entry% *}; mode=${entry#* }
	grep -qF "$path" "$INIT/system/ryoku-var-state/run" || parity_fail "var-state lost $path"
	grep -qF "$mode" "$INIT/system/ryoku-var-state/run" || parity_fail "var-state lost mode $mode for $path"
done
# network kill guard/disconnect: marker condition + the helper calls
grep -qF 'network-kill-switch.enabled' "$INIT/system/ryoku-network-kill-guard/run" || parity_fail "kill-guard lost its marker condition"
grep -qF 'ryoku-network-kill boot' "$INIT/system/ryoku-network-kill-guard/run" || parity_fail "kill-guard lost the boot call"
grep -qF 'nmcli networking off' "$INIT/system/ryoku-network-kill-disconnect/run" || parity_fail "kill-disconnect lost the nmcli call"
# env generator: every XDG dir the generator exports must be rendered
for dir in PICTURES DOWNLOAD DOCUMENTS MUSIC VIDEOS DESKTOP PUBLICSHARE TEMPLATES; do
	grep -qF "$dir" "$INIT/env/xdg-dirs" || parity_fail "xdg-dirs env renderer lost $dir"
done
# bt-agent: the systemctl restart became an sv restart of Void's service name
grep -qF 'sv restart bluetoothd' "$INIT/user/ryoku-bluetooth-reset/run" || parity_fail "bluetooth-reset lost the sv restart translation"

# --- 6. no systemd leakage in executable lines of the translation. Comments
# may name systemctl (they explain what was translated); code may not.
leaked=$(find "$INIT" -type f \( -name run -o -name finish \) -print0 | xargs -0 -I{} sh -c 'grep -Hn "systemctl\|systemd-run\|journalctl" "{}" 2>/dev/null' | grep -vE ':\s*#' || true)
[[ -z $leaked ]] || { echo "$leaked" >&2; fail "systemd command leaked into runit code (comments are fine)"; }

echo "void-init static checks passed"

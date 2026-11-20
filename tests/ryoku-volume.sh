#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
helper="$repo_root/system/hardware/audio/ryoku-volume"
nomarchy="$repo_root/ryoku/shell/quickshell/shell/modules/bar/barstyles/nomarchy/plugins/panels/audio/Panel.qml"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/config/ryoku"
export HOME="$tmp"
export XDG_CONFIG_HOME="$tmp/config"
export PATH="$tmp/bin:/usr/bin:/bin"
export WPCTL_LOG="$tmp/wpctl.log"
export WPCTL_CALLS_LOG="$tmp/wpctl-calls.log"
export JQ_LOG="$tmp/jq.log"
export SHELL_LOG="$tmp/ryoku-shell.log"
config="$XDG_CONFIG_HOME/ryoku/shell.json"
touch "$config"

fail() {
	echo "ryoku-volume-test: $*" >&2
	exit 1
}

cat >"$tmp/bin/jq" <<'PY'
#!/usr/bin/env python3
import json
import os
import sys

query, path = sys.argv[2:4]
with open(os.environ["JQ_LOG"], "a", encoding="utf-8") as log:
    log.write(" ".join(sys.argv[1:]) + "\n")
try:
    with open(path, encoding="utf-8") as stream:
        config = json.load(stream)
except (OSError, json.JSONDecodeError):
    sys.exit(1)

if query == ".qsbar.audioBoost == true":
    boosted = config.get("qsbar", {}).get("audioBoost") is True
    if boosted:
        print("true")
        sys.exit(0)
    sys.exit(1)

if query.startswith(".audio.volumeStep | select("):
    step = config.get("audio", {}).get("volumeStep")
    if isinstance(step, bool) or not isinstance(step, (int, float)):
        sys.exit(1)
    if not float(step).is_integer() or not 1 <= step <= 20:
        sys.exit(1)
    print(int(step))
    sys.exit(0)

sys.exit(2)
PY

cat >"$tmp/bin/wpctl" <<'SH'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$WPCTL_CALLS_LOG"
if [[ $1 == get-volume ]]; then
	awk -v volume="$MOCK_VOLUME" 'BEGIN { printf "Volume: %.4f\n", volume / 100 }'
elif [[ $1 == set-volume ]]; then
	printf '%s\n' "$@" >"$WPCTL_LOG"
else
	exit 2
fi
SH

cat >"$tmp/bin/ryoku-shell" <<'SH'
#!/usr/bin/env bash
printf '%s\n' "$@" >"$SHELL_LOG"
SH
chmod +x "$tmp/bin/jq" "$tmp/bin/wpctl" "$tmp/bin/ryoku-shell"

write_config() {
	printf '%s\n' "$1" >"$config"
}

expect_volume() {
	local current="$1" command="$2" expected="$3"
	export MOCK_VOLUME="$current"
	rm -f "$WPCTL_LOG"
	if [[ -n $command ]]; then
		"$helper" "$command" >/dev/null
	else
		"$helper" >/dev/null
	fi
	local got
	got="$(tail -n1 "$WPCTL_LOG")"
	[[ $got == "$expected" ]] || fail "$command at $current% produced $got, expected $expected"
}

write_config '{}'
expect_volume 65 up 67%
expect_volume 67 up 70%
expect_volume 70 down 67%
expect_volume 50 up 55%
expect_volume 50 '' 55%

write_config '{"audio":{"volumeStep":2}}'
expect_volume 51 up 53%
expect_volume 51 down 49%

write_config '{"audio":{"volumeStep":21}}'
expect_volume 50 up 55%
write_config '{"audio":{"volumeStep":"2"}}'
expect_volume 50 up 55%
write_config '{"audio":{"volumeStep":2.5}}'
expect_volume 50 up 55%

write_config '{"audio":{"volumeStep":2},"qsbar":{"audioBoost":true}}'
expect_volume 149 up 150%
grep -qx '1.5' <(sed -n '3p' "$WPCTL_LOG") || fail 'boosted volume cap was not passed to wpctl'
expect_volume 1 down 0%

rm -f "$WPCTL_LOG" "$WPCTL_CALLS_LOG" "$JQ_LOG" "$SHELL_LOG"
help="$($helper --help)"
[[ $help == *'Usage: ryoku-volume [up|down]'* ]] || fail '--help omitted the usage line'
[[ ! -e $WPCTL_CALLS_LOG && ! -e $JQ_LOG ]] || fail '--help queried or changed audio state'
help="$($helper -h)"
[[ $help == *'ryoku-volume set-step <1-20>'* ]] || fail '-h omitted set-step'
[[ ! -e $WPCTL_CALLS_LOG && ! -e $JQ_LOG ]] || fail '-h queried or changed audio state'

for step in 1 2 20; do
	: >"$SHELL_LOG"
	"$helper" set-step "$step" >/dev/null
	[[ $(<"$SHELL_LOG") == $'audio\nset-step\n'"$step" ]] || fail "set-step $step did not reach ryoku-shell"
	[[ ! -e $WPCTL_CALLS_LOG && ! -e $JQ_LOG ]] || fail 'set-step queried or changed output volume'
done
for step in 0 21 2.5 invalid; do
	if "$helper" set-step "$step" >/dev/null 2>&1; then
		fail "set-step accepted invalid value $step"
	fi
done
if "$helper" set-step >/dev/null 2>&1; then
	fail 'set-step accepted a missing value'
fi

# Nomarchy has no headless QML runtime test, so pin its keyboard path to the
# shared setting alongside the helper's executable behavior tests above.
grep -Eq 'root\.focusSection[[:space:]]*===[[:space:]]*"output"[[:space:]]*\?[[:space:]]*Ryoku\.Audio\.volumeStep[[:space:]]*:[[:space:]]*5' "$nomarchy" \
	|| fail 'Nomarchy keyboard adjustment must use the shared step only for output rows'

python_popup="$repo_root/ryoku/shell/quickshell/shell/modules/bar/barstyles/python/volume/VolumePopup.qml"
grep -Eq 'window\.activeTab[[:space:]]*===[[:space:]]*"outputs"[[:space:]]*\?[[:space:]]*Ryoku\.Audio\.volumeStep[[:space:]]*:[[:space:]]*5' "$python_popup" \
	|| fail 'Python volume popup must use the shared step only for output controls'

global_page="$repo_root/ryoku/hub/quickshell/pages/GlobalPage.qml"
grep -Fqx 'import "../Singletons"' "$global_page" \
	|| fail 'Hub Global page must import the Settings singleton used by resetDefaults'

echo 'ryoku-volume-test: helper behavior and volume settings paths passed'

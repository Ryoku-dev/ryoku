#!/usr/bin/env bash
# Regression test for the volume and brightness key steps: both helpers take
# their magnitude from shell.json (audio.step and display.brightnessStep), so
# the keys, the bar's wheel and the IPC verbs move by Ryoku Settings' number
# instead of a literal 5. A missing or bad value must fall back to 5, and
# ryoku-cmd-brightness must keep its explicit +N/-N form working.
set -euo pipefail

here="$(cd "$(dirname "$0")/.." && pwd)"
volume="$here/system/hardware/audio/ryoku-volume"
brightness="$here/ryoku/shell/scripts/ryoku-cmd-brightness"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
export XDG_CONFIG_HOME="$tmp/config"
mkdir -p "$XDG_CONFIG_HOME/ryoku" "$tmp/bin"

fail() { echo "FAIL: $1" >&2; exit 1; }

# store <json> -- write shell.json for the next press.
store() { printf '%s\n' "$1" >"$XDG_CONFIG_HOME/ryoku/shell.json"; }

# fakeWpctl <percent> -- a fake sink at that level; every set-volume call is
# recorded, one line per call.
fakeWpctl() {
	cat >"$tmp/bin/wpctl" <<EOF
#!/usr/bin/env bash
if [[ \$1 == get-volume ]]; then
	awk -v p="$1" 'BEGIN{printf "Volume: %.2f\n", p/100}'
else
	printf '%s\n' "\$*" >>"$tmp/wpctl.calls"
fi
EOF
	chmod +x "$tmp/bin/wpctl"
	: >"$tmp/wpctl.calls"
}

# volumePress <volume> <up|down> -- run one press, print the percentage it set.
volumePress() {
	fakeWpctl "$1"
	PATH="$tmp/bin:$PATH" "$volume" "$2"
	awk '{print $NF}' "$tmp/wpctl.calls" | tail -n 1
}

# 1. The shipped grid: step 5 lands on the five-point grid from any level.
store '{}'
[[ $(volumePress 43 up) == 45% ]] || fail "default up from 43 = $(volumePress 43 up), want 45%"
[[ $(volumePress 43 down) == 40% ]] || fail "default down from 43 = $(volumePress 43 down), want 40%"

# 2. A configured step moves the grid, including past the old 65 to 67 detour.
store '{"audio":{"step":10}}'
[[ $(volumePress 65 up) == 70% ]] || fail "step 10 up from 65 = $(volumePress 65 up), want 70%"
[[ $(volumePress 67 down) == 60% ]] || fail "step 10 down from 67 = $(volumePress 67 down), want 60%"

# 3. The ends clamp: 0 at the bottom, 100% without boost, 150% with it.
store '{"audio":{"step":10}}'
[[ $(volumePress 3 down) == 0% ]] || fail "down at the bottom = $(volumePress 3 down), want 0%"
[[ $(volumePress 95 up) == 100% ]] || fail "up at the 100% cap = $(volumePress 95 up), want 100%"
store '{"qsbar":{"audioBoost":true},"audio":{"step":10}}'
[[ $(volumePress 145 up) == 150% ]] || fail "boost up from 145 = $(volumePress 145 up), want 150%"
[[ $(volumePress 155 up) == 150% ]] || fail "boost up from 155 = $(volumePress 155 up), want 150%"

# 4. A bad store never stalls a keypress: a word falls back to 5, and a number
# outside the Hub's range clamps or floors into it (negative to 1, 99 to 25, a
# fraction to the whole below it).
for tc in '{"audio":{"step":0}}:41' '{"audio":{"step":-4}}:41' '{"audio":{"step":99}}:50' '{"audio":{"step":6.7}}:42' '{"audio":{"step":"loud"}}:45'; do
	store "${tc%:*}"
	want="${tc##*:}"
	got=$(volumePress 40 up)
	[[ $got == "$want%" ]] || fail "${tc%:*} stepped to $got, want $want%"
done

# 5. Brightness: panel-only fakes, recording the brightnessctl magnitude.
printf '#!/usr/bin/env bash\nprintf "%%s\\n" "$*" >>"%s"\n' "$tmp/backlight.calls" >"$tmp/bin/brightnessctl"
printf '#!/usr/bin/env bash\nexit 1\n' >"$tmp/bin/ryoku-hw-backlight"
printf '#!/usr/bin/env bash\nexit 0\n' >"$tmp/bin/ddcutil"
chmod +x "$tmp/bin/brightnessctl" "$tmp/bin/ryoku-hw-backlight" "$tmp/bin/ddcutil"
mkdir -p "$tmp/drm/card0-eDP-1"
echo connected >"$tmp/drm/card0-eDP-1/status"

# brightnessPress <up|down|+N|-N> -- run one press, print the brightnessctl args.
brightnessPress() {
	: >"$tmp/backlight.calls"
	PATH="$tmp/bin:$PATH" RYOKU_DRM_PATH="$tmp/drm" "$brightness" "$1"
	cat "$tmp/backlight.calls"
}

store '{"display":{"brightnessStep":7}}'
[[ $(brightnessPress up) == *'set 7%+'* ]] || fail "up with step 7 = $(brightnessPress up), want set 7%+"
[[ $(brightnessPress down) == *'set 7%-'* ]] || fail "down with step 7 = $(brightnessPress down), want set 7%-"

store '{}'
[[ $(brightnessPress up) == *'set 5%+'* ]] || fail "default up = $(brightnessPress up), want set 5%+"

# A bad brightness step clamps instead of stalling: negative to 1, past 25 to 25,
# and a fraction to the whole below it.
store '{"display":{"brightnessStep":-4}}'
[[ $(brightnessPress up) == *'set 1%+'* ]] || fail "negative step up = $(brightnessPress up), want set 1%+"
store '{"display":{"brightnessStep":99}}'
[[ $(brightnessPress down) == *'set 25%-'* ]] || fail "high step down = $(brightnessPress down), want set 25%-"
store '{"display":{"brightnessStep":7.9}}'
[[ $(brightnessPress up) == *'set 7%+'* ]] || fail "fractional step up = $(brightnessPress up), want set 7%+"
store '{"display":{"brightnessStep":"dim"}}'
[[ $(brightnessPress up) == *'set 5%+'* ]] || fail "word step up = $(brightnessPress up), want set 5%+"

# 6. The explicit form scripts already use keeps its own magnitude.
[[ $(brightnessPress +3) == *'set 3%+'* ]] || fail "explicit +3 = $(brightnessPress +3), want set 3%+"
[[ $(brightnessPress -3) == *'set 3%-'* ]] || fail "explicit -3 = $(brightnessPress -3), want set 3%-"

# 7. Nothing else is a press.
if PATH="$tmp/bin:$PATH" "$brightness" sideways >/dev/null 2>&1; then
	fail "a bad argument was accepted"
fi

echo "PASS: the volume and brightness keys step by the shell.json setting"

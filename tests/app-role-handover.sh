#!/usr/bin/env bash
# ryoku-app hands what follows -- to the app it launches for a role. A terminal
# runs it as a program, in the form that emulator takes. Every other role gets
# it as plain arguments, which is how a caller opens the file manager at a
# folder. This drives the real script against stub apps that print their argv.
set -euo pipefail

repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
script="$repo/ryoku/shell/scripts/ryoku-app"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/bin" "$work/config/ryoku"
for app in kitty alacritty wezterm nautilus thunar; do
	# shellcheck disable=SC2016 # the stub expands "$@" itself, when it runs
	printf '#!/bin/sh\nprintf "%%s" "%s"\nfor a in "$@"; do printf " [%%s]" "$a"; done\nprintf "\\n"\n' "$app" >"$work/bin/$app"
	chmod +x "$work/bin/$app"
done

fails=0
run() { PATH="$work/bin:$PATH" XDG_CONFIG_HOME="$work/config" HOME="$work" bash "$script" "$@" 2>&1; }
expect() {
	local want="$1" label="$2"
	shift 2
	local got
	got=$(run "$@" || true)
	if [[ $got == "$want" ]]; then
		echo "  ok: $label"
	else
		echo "  FAIL: $label"
		echo "    want: $want"
		echo "    got:  $got"
		fails=$((fails + 1))
	fi
}
store() { printf '%s\n' "$1" >"$work/config/ryoku/desktop.json"; }

echo "the shipped defaults"
rm -f "$work/config/ryoku/desktop.json"
expect "kitty" "the terminal opens bare" terminal
expect "kitty [btop]" "kitty takes the program as trailing argv" terminal -- btop
expect "nautilus" "the file manager opens bare" files
expect "nautilus [/home/someone/My Work]" "the file manager is handed a folder, spaces kept" files -- "/home/someone/My Work"

echo "the user's own choices"
store '{"desktop":{"apps":{"terminal":"alacritty","files":"thunar"}}}'
expect "alacritty [-e] [sh] [-c] [echo hi]" "another terminal takes the program after -e" terminal -- sh -c "echo hi"
expect "thunar [/srv/share]" "the chosen file manager is handed the folder, with no -e" files -- /srv/share
store '{"desktop":{"apps":{"terminal":"wezterm"}}}'
expect "wezterm [start] [--] [btop]" "wezterm takes the program after start --" terminal -- btop
store '{"desktop":{"apps":{"editor":"kitty -e nvim"}}}'
expect "kitty [-e] [nvim] [notes.md]" "an editor that lives in a terminal is handed the file" editor -- notes.md

if ((fails > 0)); then
	echo "app role handover: $fails failed"
	exit 1
fi
echo "app role handover: ok"

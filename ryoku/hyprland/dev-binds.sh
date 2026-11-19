#!/usr/bin/env bash
# add (on) or drop (off) the Ryoku shell keybinds on the live session so the
# real bindings can be exercised from a dev checkout. The compositor's Lua
# runtime owns binds (plain `hyprctl keyword` is rejected there), and binding
# a bound chord stacks a second handler, so every change is an explicit
# unbind then bind through `hyprctl eval`. `off` re-registers nothing: reload
# the config to restore the shipped bindings.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
# this script sits with the compositor config; the daemon binary is in the shell
# tree one directory over (it was beside this script before the wm provider move)
bin="$(cd "$here/../shell/ipc" && pwd)/ryoku-shell"

chords=("SUPER + Space" "SUPER + L" "SUPER + Escape" "SUPER + W" "SUPER + S" "SUPER + Tab" "SUPER + comma")
cmds=(
	"$bin launcher"
	"$bin lock"
	"$bin menu quick-settings"
	"$bin menu wallpaper"
	"$bin menu screenshot"
	"$bin overview"
	"$bin hub open"
)

case "${1:-on}" in
on)
	for i in "${!chords[@]}"; do
		hyprctl eval "hl.unbind(\"${chords[$i]}\"); hl.bind(\"${chords[$i]}\", hl.dsp.exec_cmd(\"${cmds[$i]}\"))" >/dev/null
	done
	echo "keybinds added. restore yours with: hyprctl reload"
	;;
off)
	for c in "${chords[@]}"; do
		hyprctl eval "hl.unbind(\"$c\")" >/dev/null
	done
	echo "keybinds removed. restore the shipped ones with: hyprctl reload"
	;;
*)
	echo "usage: dev-binds.sh [on|off]" >&2
	exit 1
	;;
esac

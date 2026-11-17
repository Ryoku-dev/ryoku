#!/usr/bin/env bash
set -euo pipefail

repo=${RYOKU_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

command -v jq >/dev/null || { echo "jq is required" >&2; exit 1; }
mkdir -p "$tmp/bin" "$tmp/config/ryoku" "$tmp/workdir"
for terminal in kitty alacritty; do
	cat >"$tmp/bin/$terminal" <<'STUB'
#!/usr/bin/env bash
printf '%s\0' "$PWD" "${0##*/}" "$@" >"$RYOKU_TERMINAL_ADAPTER_ARGS"
STUB
	chmod +x "$tmp/bin/$terminal"
done

export HOME="$tmp/home"
export XDG_CONFIG_HOME="$tmp/config"
export PATH="$tmp/bin:$repo/ryoku/shell/scripts:/usr/bin"
export RYOKU_TERMINAL_ADAPTER_ARGS="$tmp/args"

run_terminal() {
	local choice=$1
	shift
	printf '{"desktop":{"apps":{"terminal":"%s"}}}\n' "$choice" >"$tmp/config/ryoku/desktop.json"
	(cd "$tmp/workdir" && "$repo/ryoku/shell/scripts/ryoku-terminal" "$@")
}

run_terminal kitty
printf '%s\0kitty\0' "$tmp/workdir" >"$tmp/expected"
cmp -s "$tmp/expected" "$tmp/args" || {
	echo "ryoku-terminal did not use the Hub-selected Kitty terminal" >&2
	exit 1
}

run_terminal alacritty 'program with spaces' --flag
printf '%s\0alacritty\0-e\0program with spaces\0--flag\0' "$tmp/workdir" >"$tmp/expected"
cmp -s "$tmp/expected" "$tmp/args" || {
	echo "ryoku-terminal did not follow a Hub terminal change or preserve arguments" >&2
	exit 1
}

echo "ryoku-terminal adapter: passed"

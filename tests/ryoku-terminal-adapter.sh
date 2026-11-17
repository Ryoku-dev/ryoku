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
	(cd "$tmp/workdir" && "$repo/ryoku/shell/scripts/ryoku-terminal" -- "$@")
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

rm "$tmp/config/ryoku/desktop.json"
(cd "$tmp/workdir" && "$repo/ryoku/shell/scripts/ryoku-terminal")
printf '%s\0kitty\0' "$tmp/workdir" >"$tmp/expected"
cmp "$tmp/expected" "$tmp/args"

python3 - "$repo" "$tmp" <<'PY'
import configparser
import ctypes
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys

repo, tmp = map(Path, sys.argv[1:])
workdir = tmp / "spaces 'quotes' \"$dollar\"\nnew line"
workdir.mkdir()
arguments = ["program with spaces", "single'quote", 'double"quote', "$HOME",
             "$(touch injected)", "semi;colon", "new\nline", ""]
command = shlex.join(arguments)
adapter = repo / "ryoku/shell/scripts/ryoku-terminal"
store = tmp / "config/ryoku/desktop.json"
store.write_text(json.dumps({"desktop": {"apps": {"terminal": "kitty --single-instance"}}}))
expected = b"\0".join(os.fsencode(s) for s in
                      [str(workdir), "kitty", "--single-instance", *arguments]) + b"\0"

def check(argv):
    subprocess.run(argv, cwd=workdir, check=True)
    assert (tmp / "args").read_bytes() == expected
    assert not (workdir / "injected").exists()

check([str(adapter), command])
check([str(adapter), "--", *arguments])

helper = configparser.ConfigParser(interpolation=None)
helper.read(repo / "ryoku/apps/xfce4/helpers/ryoku-terminal.desktop")
entry = helper["Desktop Entry"]
assert entry["Type"] == "X-XFCE-Helper"
assert entry["X-XFCE-Category"] == "TerminalEmulator"
line = entry["X-XFCE-CommandsWithParameter"].rstrip(";")
line = line.replace("%B", str(adapter)).replace("%s", command)
glib = ctypes.CDLL("libglib-2.0.so.0")
argv = ctypes.POINTER(ctypes.c_char_p)()
argc = ctypes.c_int()
glib.g_shell_parse_argv.argtypes = [ctypes.c_char_p, ctypes.POINTER(ctypes.c_int),
                                  ctypes.POINTER(ctypes.POINTER(ctypes.c_char_p)), ctypes.c_void_p]
glib.g_shell_parse_argv.restype = ctypes.c_int
assert glib.g_shell_parse_argv(os.fsencode(line), ctypes.byref(argc), ctypes.byref(argv), None)
try:
    check([os.fsdecode(argv[i]) for i in range(argc.value)])
finally:
    glib.g_strfreev(argv)
result = subprocess.run([str(adapter), "'unterminated"], cwd=workdir,
                        capture_output=True)
assert result.returncode == 2
PY

echo "ryoku-terminal adapter: passed"

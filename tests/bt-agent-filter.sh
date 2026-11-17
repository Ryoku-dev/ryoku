#!/usr/bin/env bash
# ryoku-bt-agent must keep pairing prompts, errors and a device's bond changes in
# the journal while dropping the per-advertisement chatter bluetoothctl prints
# during any scan (#869: 400-600 lines a minute). The recording below has the
# shapes bluetoothctl really emits through a pipe: readline prompt markers, colour
# sequences and carriage-return redraws around every event.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
agent="$here/../system/hardware/bluetooth/ryoku-bt-agent"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fail() { echo "FAIL: $1" >&2; exit 1; }

E=$'\e'
P=$'\001'"${E}[0;94m"$'\002'"[bluetoothctl]> "$'\001'"${E}[0m"$'\002'
ev() { printf '\r%s[K%s[%s[0;93m%s%s[0m] %s\n%s' "$E" "" "$E" "$1" "$E" "$2" "$P"; }

{
  for i in $(seq 1 60); do
    ev CHG "Device 4C:87:5D:0A:0B:$(printf %02X "$i") RSSI: 0xffffffb4 (-76)"
    ev CHG "Device 4C:87:5D:0A:0B:$(printf %02X "$i") AdvertisingFlags:"
    ev CHG "Device 4C:87:5D:0A:0B:$(printf %02X "$i") ManufacturerData Key: 0x004c"
    ev CHG "Device 4C:87:5D:0A:0B:$(printf %02X "$i") ServiceData Key: 0000fd6f-0000-1000-8000-00805f9b34fb"
    ev CHG "Device 4C:87:5D:0A:0B:$(printf %02X "$i") TxPower: 0x0008 (8)"
    ev CHG "Device 4C:87:5D:0A:0B:$(printf %02X "$i") Name: Pair of Tags $i"
    ev NEW "Device 4C:87:5D:0A:0B:$(printf %02X "$i") Tag $i"
    ev DEL "Device 4C:87:5D:0A:0B:$(printf %02X "$i")"
    printf '%s\n' "$P"
  done
  printf '%s\n' "${P}Agent registered"
  ev CHG "Controller 00:1A:7D:DA:71:13 Discovering: yes"
  printf '%s[agent] Confirm passkey 123456 (yes/no): \n' "$P"
  printf '%s[agent] Request authorization\n' "$P"
  ev CHG "Device E4:17:D8:00:11:22 Paired: yes"
  ev CHG "Device E4:17:D8:00:11:22 Trusted: yes"
  ev CHG "Device E4:17:D8:00:11:22 Connected: yes"
  ev CHG "Device E4:17:D8:00:11:22 Connected: no"
  printf '%sFailed to pair: org.bluez.Error.AuthenticationFailed\n' "$P"
  printf 'Failed to register agent object: org.bluez.Error.AlreadyExists\n'
} >"$tmp/recording"

lines_in=$(wc -l <"$tmp/recording")
"$agent" --filter <"$tmp/recording" >"$tmp/out"

cat >"$tmp/want" <<'EOF'
Agent registered
[CHG] Controller 00:1A:7D:DA:71:13 Discovering: yes
[agent] Confirm passkey 123456 (yes/no): 
[agent] Request authorization
[CHG] Device E4:17:D8:00:11:22 Paired: yes
[CHG] Device E4:17:D8:00:11:22 Trusted: yes
[CHG] Device E4:17:D8:00:11:22 Connected: yes
[CHG] Device E4:17:D8:00:11:22 Connected: no
Failed to pair: org.bluez.Error.AuthenticationFailed
Failed to register agent object: org.bluez.Error.AlreadyExists
EOF

cmp -s "$tmp/out" "$tmp/want" || {
  diff "$tmp/want" "$tmp/out" >&2 || true
  fail "filter kept the wrong lines ($(wc -l <"$tmp/out") of $lines_in)"
}

echo "bt-agent-filter: $lines_in recorded lines -> $(wc -l <"$tmp/out"), all prompts and errors kept"

#!/usr/bin/env bash
# hermetic test for ryoku-bt-audio: the per-device Bluetooth codec memory.
#
# The codec order in 51-ryoku-bluetooth.conf is global (bluez5.codecs is a
# MONITOR property), and WirePlumber persists the bluetooth PROFILE but not the
# codec. So a user who prefers AAC on one device has to re-pick it on every
# reconnect. This helper is the memory that closes that gap, and these are the
# behaviours that make it trustworthy rather than annoying:
#
#   - a codec the device never offered is refused, because remembering one means
#     retrying a doomed switch on every single reconnect
#   - a device already on the wanted codec is left alone, because the switch
#     renegotiates the A2DP link and audibly drops audio
#   - apply only touches devices that have a remembered choice
#
# Both seams point at fixtures: RYOKU_BT_PACTL (a fake pactl that records what it
# was asked to do) and RYOKU_BT_CONFIG (the memory, inside the tmp dir, so the
# real ~/.config/ryoku/bluetooth.json is never read or written; asserted at the
# end). No PipeWire, no Bluetooth device, no root.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
helper="$here/../system/hardware/audio/ryoku-bt-audio"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fail() { echo "FAIL: $1" >&2; exit 1; }

command -v jq >/dev/null 2>&1 || { echo 'SKIP: jq is unavailable'; exit 0; }

# prove we never touch the user's real memory file
real="${XDG_CONFIG_HOME:-$HOME/.config}/ryoku/bluetooth.json"
real_state="absent"; [[ -e $real ]] && real_state="$(cksum <"$real")"

ADDR="AC:80:0A:11:22:33"
UND="AC_80_0A_11_22_33"
CARD="bluez_card.$UND"

# A fake pactl with the real output shapes. It records every send-message so the
# test can assert what the helper did, not merely that it exited 0.
cat >"$tmp/pactl" <<EOF
#!/usr/bin/env bash
set -u
log="$tmp/calls"
case "\$1" in
  subscribe)
    # Replay EVENTS, where a "pause" line stands for a quiet gap in the stream,
    # then exit the way pactl does when PipeWire goes away.
    while IFS= read -r ev; do
      if [[ \$ev == pause ]]; then sleep 1.5; else printf '%s\n' "\$ev"; fi
    done <"$tmp/EVENTS"
    exit 0 ;;
  list)
    if [[ \${2:-} == cards && \${3:-} == short ]]; then
      echo pass >>"$tmp/passes"
      [[ -e "$tmp/NO_CARD" ]] && exit 0
      printf '43\t$CARD\tmodule-bluez5-device.c\t\n'
      exit 0
    fi
    if [[ \${2:-} == cards ]]; then
      [[ -e "$tmp/NO_CARD" ]] && exit 0
      printf 'Card #43\n\tName: $CARD\n\tDriver: module-bluez5-device.c\n\tProperties:\n'
      printf '\t\tapi.bluez5.codec = "%s"\n' "\$(cat "$tmp/ACTIVE" 2>/dev/null || echo aac)"
      exit 0
    fi
    exit 0 ;;
  send-message)
    printf '%s %s %s\n' "\$2" "\$3" "\${4:-}" >>"\$log"
    case "\$3" in
      list-codecs) cat "$tmp/OFFERED" ;;
      switch-codec)
        # mimic the real thing: the switch takes effect
        printf '%s' "\${4//\\"/}" >"$tmp/ACTIVE" ;;
    esac
    exit 0 ;;
esac
exit 0
EOF
chmod +x "$tmp/pactl"

printf '[{"name":"sbc","description":"SBC"},{"name":"sbc_xq","description":"SBC-XQ"},{"name":"aac","description":"AAC"}]' >"$tmp/OFFERED"
printf 'aac' >"$tmp/ACTIVE"

export RYOKU_BT_PACTL="$tmp/pactl" RYOKU_BT_CONFIG="$tmp/bluetooth.json"
calls() { cat "$tmp/calls" 2>/dev/null || true; }
reset_calls() { : >"$tmp/calls"; }

# ---- cards / list ----------------------------------------------------------
out="$("$helper" cards)"
[[ $out == "$UND"$'\t'"$CARD" ]] || fail "cards should report the address and card, got: $out"

out="$("$helper" list "$ADDR")"
grep -qx "aac	active" <<<"$out" || fail "list should mark the active codec, got: $out"
grep -qx "sbc_xq	available" <<<"$out" || fail "list should show the offered codecs, got: $out"

# the colon form and the underscore form must be the same device
[[ "$("$helper" list "$UND")" == "$out" ]] || fail "an underscore address should resolve like a colon address"

# ---- refusing a codec the device never offered ------------------------------
reset_calls
rc=0; "$helper" set "$ADDR" ldac >/dev/null 2>&1 || rc=$?
[[ $rc -eq 2 ]] || fail "setting an unoffered codec should exit 2, got $rc"
grep -q switch-codec <<<"$(calls)" && fail "an unoffered codec must not reach switch-codec"
[[ -e $RYOKU_BT_CONFIG ]] && fail "an unoffered codec must not be remembered"

# ---- setting an offered codec -----------------------------------------------
reset_calls
"$helper" set "$ADDR" sbc_xq >/dev/null 2>&1 || fail "setting an offered codec should succeed"
grep -q 'switch-codec' <<<"$(calls)" || fail "set should switch the codec, calls: $(calls)"
[[ "$(jq -r --arg a "$UND" '.[$a].codec' "$RYOKU_BT_CONFIG")" == sbc_xq ]] \
  || fail "set should remember the codec, got: $(cat "$RYOKU_BT_CONFIG")"

# ---- a no-op stays a no-op (the switch drops audio) ------------------------
reset_calls
"$helper" apply >/dev/null 2>&1
grep -q switch-codec <<<"$(calls)" \
  && fail "apply switched a device that was already on the remembered codec"

# ---- apply puts it back after a reconnect changed it ------------------------
printf 'aac' >"$tmp/ACTIVE"          # the device came back on the global default
reset_calls
"$helper" apply >/dev/null 2>&1
grep -q 'switch-codec' <<<"$(calls)" || fail "apply should restore the remembered codec, calls: $(calls)"
[[ "$(cat "$tmp/ACTIVE")" == sbc_xq ]] || fail "apply should have switched to sbc_xq, got: $(cat "$tmp/ACTIVE")"

# ---- apply ignores devices with no remembered choice ------------------------
"$helper" forget "$ADDR" >/dev/null 2>&1 || fail "forget should succeed"
[[ "$(jq -r --arg a "$UND" '.[$a] // "gone"' "$RYOKU_BT_CONFIG")" == gone ]] \
  || fail "forget should drop the entry, got: $(cat "$RYOKU_BT_CONFIG")"
printf 'aac' >"$tmp/ACTIVE"
reset_calls
"$helper" apply >/dev/null 2>&1
grep -q switch-codec <<<"$(calls)" && fail "apply touched a device with no remembered codec"

# ---- no device connected is quiet, not an error ----------------------------
touch "$tmp/NO_CARD"
"$helper" apply >/dev/null 2>&1 || fail "apply with no connected card should still succeed"
[[ -z "$("$helper" cards)" ]] || fail "cards should be empty with no device"
rm -f "$tmp/NO_CARD"

# ---- watch collapses a burst of card events into one pass -------------------
# One reconnect emits several card events amid stream and sink noise. Each burst
# must cost one apply pass (each pass lists the cards once), and watch must end
# when the subscription does rather than hang.
cat >"$tmp/EVENTS" <<'EV'
Event 'new' on card #43
Event 'change' on sink #12
Event 'change' on card #43
Event 'new' on sink-input #90
Event 'change' on card #43
Event 'change' on card #43
pause
Event 'remove' on card #43
Event 'change' on sink #12
Event 'new' on card #44
EV
: >"$tmp/passes"
RYOKU_BT_WATCH_LOCK="$tmp/watch.lock" timeout 20 "$helper" watch >/dev/null 2>&1 \
  || fail "watch should exit cleanly when the subscription ends"
passes="$(wc -l <"$tmp/passes")"
[[ $passes -eq 3 ]] || fail "watch should run 3 passes (startup + one per burst), got $passes"

# ---- the headset mic switch is a saved WirePlumber setting ------------------
# `mic` talks to wpctl only: a fake one keeps the value and fails like the real
# one when WirePlumber is down, so what is asserted is the setting that was
# written and the answer that comes back, not that some command ran.
cat >"$tmp/wpctl" <<'EOF'
#!/usr/bin/env bash
[[ -e ${RYOKU_TEST_DIR:?}/WP_DOWN ]] && exit 1
[[ ${1:-} == settings ]] || exit 2
shift
save=0
[[ ${1:-} == --save ]] && { save=1; shift; }
key=$1
if [[ $# -ge 2 ]]; then
  printf '%s %s %s\n' "$save" "$key" "$2" >>"$RYOKU_TEST_DIR/wpctl.calls"
  printf '%s' "$2" >"$RYOKU_TEST_DIR/MIC"
  exit 0
fi
[[ $key == bluetooth.autoswitch-to-headset-profile ]] || { echo "Setting '$key' not found"; exit 0; }
echo "Value: $(cat "$RYOKU_TEST_DIR/MIC")"
EOF
chmod +x "$tmp/wpctl"
export RYOKU_TEST_DIR="$tmp" RYOKU_BT_WPCTL="$tmp/wpctl"
printf 'true' >"$tmp/MIC"

[[ "$("$helper" mic)" == on ]] || fail "mic should report the shipped default as on"
"$helper" mic off >/dev/null 2>&1 || fail "mic off should succeed"
[[ "$(cat "$tmp/wpctl.calls")" == "1 bluetooth.autoswitch-to-headset-profile false" ]] \
  || fail "mic off should save false for the autoswitch setting, got: $(cat "$tmp/wpctl.calls")"
[[ "$("$helper" mic status)" == off ]] || fail "mic should report off after mic off"
"$helper" mic on >/dev/null 2>&1 || fail "mic on should succeed"
[[ "$("$helper" mic)" == on ]] || fail "mic should report on after mic on"
touch "$tmp/WP_DOWN"
rc=0; "$helper" mic off >/dev/null 2>&1 || rc=$?
[[ $rc -eq 1 ]] || fail "mic off with WirePlumber down should exit 1, got $rc"
rc=0; "$helper" mic >/dev/null 2>&1 || rc=$?
[[ $rc -eq 1 ]] || fail "mic status with WirePlumber down should exit 1, got $rc"
rm -f "$tmp/WP_DOWN"
rc=0; "$helper" mic maybe >/dev/null 2>&1 || rc=$?
[[ $rc -eq 2 ]] || fail "mic with an unknown choice should exit 2, got $rc"

# ---- unknown verbs are refused ---------------------------------------------
for v in "" bogus --help; do
  rc=0; "$helper" "$v" >/dev/null 2>&1 || rc=$?
  [[ $rc -eq 2 ]] || fail "verb '$v' should exit 2, got $rc"
done

# ---- the user's real memory file was never touched -------------------------
now="absent"; [[ -e $real ]] && now="$(cksum <"$real")"
[[ $now == "$real_state" ]] || fail "the real $real was modified"

echo "bt-audio-codec: all checks passed"

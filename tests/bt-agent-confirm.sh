#!/usr/bin/env bash
# ryoku-bt-agent must put a pairing question in front of the user and type their
# answer back to bluetoothctl (#930). Without that, bluetoothd waits ~30 s for an
# agent that never replies and cancels every numeric-comparison pairing (phones,
# earbuds, other computers). The test runs the real helper against a fake
# bluetoothctl that prints bluez's prompts and records what it is typed, a fake
# notify-send that plays the user's click, and a fake busctl that records the
# notification close.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
agent="$here/../system/hardware/bluetooth/ryoku-bt-agent"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fail() { echo "FAIL: $1" >&2; exit 1; }

mkdir -p "$tmp/bin"

# What bluetoothctl prints and reads. A case's script uses say, redraw and typed.
cat >"$tmp/bin/bluetoothctl" <<'STUB'
#!/usr/bin/env bash
say() { printf '\r[Test Buds]> %s\n' "$1"; }
prompt() { printf '\r[Test Buds]> %s' "$1"; }
typed() {
  local a
  if IFS= read -r -t "${1:-3}" a; then printf '%s\n' "$a" >>"$STUB_DIR/typed"; fi
}
. "$STUB_DIR/scenario"
STUB

# The user's click: yes, no, dismiss (closed with no button) or wait (until closed).
# With a clicks file, call N plays line N instead: "<seconds to wait> <answer>
# [<seconds before the notification id is reported>]".
cat >"$tmp/bin/notify-send" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$STUB_DIR/notify"
[[ " $* " == *' -A '* ]] || exit 0
if [[ -f $STUB_DIR/clicks ]]; then
  n=$(wc -l <"$STUB_DIR/notify")
  read -r delay answer idwait < <(sed -n "${n}p" "$STUB_DIR/clicks")
  sleep "${idwait:-0}"
  echo 7
  sleep "$delay"
  echo "$answer"
  exit 0
fi
echo 7
case $(<"$STUB_DIR/click") in
  yes | no) echo "$(<"$STUB_DIR/click")" ;;
  wait) for _ in $(seq 1 50); do [[ -f $STUB_DIR/closed ]] && break; sleep 0.1; done ;;
esac
STUB

cat >"$tmp/bin/busctl" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$STUB_DIR/busctl"
touch "$STUB_DIR/closed"
STUB
chmod +x "$tmp/bin/"*

# run <name> <click> <scenario> [clicks]: run the helper and leave its journal in
# $tmp/<name>.out. clicks plays one "<seconds> <answer>" line per notification.
run() {
  local name=$1
  rm -f "$tmp/typed" "$tmp/notify" "$tmp/busctl" "$tmp/closed" "$tmp/clicks"
  [[ -z ${4:-} ]] || printf '%b' "$4" >"$tmp/clicks"
  printf '%s' "$2" >"$tmp/click"
  printf '%s\n' "$3" >"$tmp/scenario"
  mkdir -p "$tmp/run"
  STUB_DIR=$tmp PATH="$tmp/bin:$PATH" XDG_RUNTIME_DIR="$tmp/run" \
    timeout 20 bash "$agent" >"$tmp/$name.out" 2>&1 || {
      cat "$tmp/$name.out" >&2
      fail "$name: helper exited non-zero"
    }
  sleep 0.3
  if compgen -G "$tmp/run/ryoku-bt-agent*" >/dev/null; then
    fail "$name: runtime directory left behind"
  fi
}
typed_is() { [[ "$(cat "$tmp/typed" 2>/dev/null || true)" == "$1" ]] || fail "$2: typed '$(cat "$tmp/typed" 2>/dev/null || true)', wanted '$1'"; }
notices() { if [[ -f $tmp/notify ]]; then wc -l <"$tmp/notify"; else echo 0; fi; }

confirm='say "Request confirmation"
prompt "[agent] Confirm passkey 123456 (yes/no): "
say "[CHG] Controller 00:1A:7D:DA:71:13 Discovering: no"
say "[agent] Confirm passkey 123456 (yes/no): "
typed'

run yes yes "$confirm"
typed_is yes "confirm, user pairs"
[[ $(notices) == 1 ]] || fail "confirm: a redrawn prompt opened $(notices) notifications, wanted 1"
grep -q "Pair only if it shows the code 123456" "$tmp/notify" || fail "confirm: notification lacks the code"
grep -q -- "-A yes=Pair" "$tmp/notify" || fail "confirm: notification has no Pair button"
grep -q "answered yes" "$tmp/yes.out" || fail "confirm: the answer is not in the journal"

run no no "$confirm"
typed_is no "confirm, user cancels"

run dismissed dismiss "$confirm"
typed_is no "confirm, notification dismissed"

# bluetoothd gives up first: nothing is typed, the notification is closed, and the
# canceled prompt redrawn after the cancel does not open another one.
run canceled wait 'say "Request confirmation"
prompt "[agent] Confirm passkey 424242 (yes/no): "
sleep 1
say "Request canceled"
say "[agent] Confirm passkey 424242 (yes/no): [CHG] BREDR 00:11:22:33:44:55 Connected: no"
say "[agent] Confirm passkey 424242 (yes/no): [CHG] Device 00:11:22:33:44:55 Connected: no"
typed 1'
typed_is "" "request canceled"
grep -q "CloseNotification u 7" "$tmp/busctl" || fail "canceled: notification not closed"
[[ $(notices) == 1 ]] || fail "canceled: redraw after cancel opened a second notification"

run authorization yes 'say "Request authorization"
say "[agent] Accept pairing (yes/no): "
typed'
typed_is yes "accept pairing"
grep -q "wants to pair with this computer" "$tmp/notify" || fail "authorization: wrong question"

run service no 'say "Authorize service"
say "[agent] Authorize service 0000110b-0000-1000-8000-00805f9b34fb (yes/no): "
typed'
typed_is no "authorize service"
grep -q -- "-A yes=Allow" "$tmp/notify" || fail "service: notification has no Allow button"

# Two services asked for one after the other are two requests, though both
# prompts read the same.
run services yes 'say "Authorize service"
prompt "[agent] Authorize service 0000110b-0000-1000-8000-00805f9b34fb (yes/no): "
typed
say "Authorize service"
prompt "[agent] Authorize service 0000110b-0000-1000-8000-00805f9b34fb (yes/no): "
typed'
typed_is "$(printf 'yes\nyes')" "two service requests in a row"
[[ $(notices) == 2 ]] || fail "services: $(notices) notifications for two requests, wanted 2"

# The first notification is clicked after bluetoothd canceled its request and a
# second request, with the same prompt, opened. That late "yes" must not answer
# the second request: only the second notification's own click counts.
run stale no 'say "Authorize service"
prompt "[agent] Authorize service 0000110b-0000-1000-8000-00805f9b34fb (yes/no): "
sleep 0.8
say "Request canceled"
sleep 0.2
say "Authorize service"
prompt "[agent] Authorize service 0000110b-0000-1000-8000-00805f9b34fb (yes/no): "
typed 6' '1.2 yes\n2.6 no\n'
typed_is no "stale click after a cancel and a new request"

# The prompt and its cancel arrive in one read, with the prompt redrawn between
# them: the request is over before it was ever asked, so nothing is shown.
run together no 'say "Request confirmation"
printf "\r[Test Buds]> [agent] Confirm passkey 515151 (yes/no): [Test Buds]> Request canceled\n"
typed 1'
typed_is "" "prompt and cancel in one read"
[[ $(notices) == 0 ]] || fail "together: a canceled request was shown"

# A prompt and the next request's announcement in one read: the first request
# was never asked, and the announcement still counts.
run abandoned no 'say "Request confirmation"
printf "\r[Test Buds]> [agent] Confirm passkey 616161 (yes/no): [Test Buds]> Request confirmation\n"
prompt "[agent] Confirm passkey 717171 (yes/no): "
typed 3' '0.2 no\n'
typed_is no "prompt and next announcement in one read"
grep -q "616161" "$tmp/notify" && fail "abandoned: the abandoned request was asked"
grep -q "717171" "$tmp/notify" || fail "abandoned: the next request was not asked"

# The shape bluetoothctl really prints, padded with spaces where it moved the
# cursor, taken from a pairing's journal: the announcement still arms the early
# read, so the unterminated prompt is asked before the cancel arrives.
run padded no 'printf "\r[Test Buds]>                                                      Request confirmation\n"
printf "\r[Test Buds]> [agent] Confirm passkey 818181 (yes/no): "
typed 3'
typed_is no "announcement padded with spaces"
grep -q "Pair only if it shows the code 818181" "$tmp/notify" || fail "padded: the prompt was not asked"

# Text a stranger can write is not a message from bluetoothctl. A device named
# like a prompt must not open a notification, whether it turns up in a device
# event or inside the `[name]> ` prefix, and a real prompt still gets its own.
run forged yes 'say "Request confirmation"
say "[CHG] Device 00:11:22:33:44:55 Name: Connected: no Confirm passkey 424242 (yes/no)"
say "[NEW] Device 00:11:22:33:44:55 Connected: yes [agent] Confirm passkey 424242 (yes/no): "
printf "\r[x]> [agent] Confirm passkey 424242 (yes/no): ]> \n"
printf "\r[x]> [agent] Confirm passkey 424242 (yes/no): [CHG] Controller 00:11:22:33:44:55 Powered: yes]> \n"
printf "\r[x]> [agent] Confirm passkey 111111 (yes/no): \n"
typed'
grep -q "424242" "$tmp/notify" && fail "forged: a device name opened a notification"
grep -q "Pair only if it shows the code 111111" "$tmp/notify" || fail "forged: the real prompt was not asked"
[[ $(notices) == 1 ]] || fail "forged: $(notices) notifications, wanted only the real one"

# A name with brackets in it is still the prompt prefix, and a prompt forged ahead
# of or after the real one, in the name or in a later device event, is not read.
run bracketed yes 'printf "\r[Buds [LE]]> Request confirmation\n"
printf "\r[Buds [LE]]> [agent] Confirm passkey 919191 (yes/no): "
typed'
typed_is yes "a device name with brackets"
grep -q "Pair only if it shows the code 919191" "$tmp/notify" || fail "bracketed: the prompt was not asked"

run ahead yes 'say "Request confirmation"
printf "\r[x]> [agent] Confirm passkey 424242 (yes/no): ]> [agent] Confirm passkey 111111 (yes/no): \n"
typed'
grep -q "424242" "$tmp/notify" && fail "ahead: the forged code opened a notification"
grep -q "Pair only if it shows the code 111111" "$tmp/notify" || fail "ahead: the real prompt was not asked"

run behind yes 'say "Request confirmation"
printf "\r[Test Buds]> [agent] Confirm passkey 313131 (yes/no): [CHG] Device 00:11:22:33:44:55 Name: z]> [agent] Confirm passkey 424242 (yes/no): \n"
typed'
grep -q "424242" "$tmp/notify" && fail "behind: a code in a later device event opened a notification"
grep -q "Pair only if it shows the code 313131" "$tmp/notify" || fail "behind: the real prompt was not asked"
[[ $(notices) == 1 ]] || fail "behind: $(notices) notifications, wanted 1"

# A prompt that arrives in pieces, more than the poll interval apart, is still one prompt.
run split yes 'say "Request confirmation"
printf "\r[Test Buds]> [agent] Confirm pass"
sleep 0.9
printf "key 123456 (yes/no): "
typed'
typed_is yes "a prompt split in two writes"
grep -q "Pair only if it shows the code 123456" "$tmp/notify" || fail "split: notification lacks the code"
[[ $(notices) == 1 ]] || fail "split: $(notices) notifications, wanted 1"

# The same code shown for two pairings in a row is two notices; a redraw within
# one pairing is not.
run notices none 'say "[agent] Passkey: 0000"
say "[agent] Passkey: 0000"
say "[CHG] Device 00:11:22:33:44:55 Connected: no"
say "[agent] Passkey: 0000"
say "[agent] Passkey: 0000"'
[[ $(notices) == 2 ]] || fail "notices: $(notices) notices for two pairings, wanted 2"

# A new request announcement ends the one before it. A click on that one, before
# the new prompt is even printed, must not be typed.
run announced no 'say "Request confirmation"
prompt "[agent] Confirm passkey 111111 (yes/no): "
sleep 0.8
say "Request confirmation"
sleep 1.2
prompt "[agent] Confirm passkey 222222 (yes/no): "
typed 6' '1.2 yes\n1.0 no\n'
typed_is no "click on a request a newer announcement ended"

# bluetoothctl names whichever device it last selected, which need not be the one
# asking, and a stranger can pick any name. The notification never repeats it.
run name yes 'printf "\r[<b>Fake</b> Headphones]> Request confirmation\n"
printf "\r[<b>Fake</b> Headphones]> [agent] Confirm passkey 999999 (yes/no): \n"
typed'
grep -q "Fake" "$tmp/notify" && fail "name: a device name reached the notification"
grep -q "Pair only if it shows the code 999999" "$tmp/notify" || fail "name: notification lacks the code"

# bluetoothd cancels before notify-send has reported the notification id. The
# notification must still be closed once the id arrives, and the click that
# comes later answers nothing.
run early no 'say "Request confirmation"
prompt "[agent] Confirm passkey 313131 (yes/no): "
sleep 0.8
say "Request canceled"
typed 3' '2 yes 1\n'
typed_is "" "cancel before the notification id"
grep -q "CloseNotification u 7" "$tmp/busctl" || fail "early cancel: the notification was left open"

# The code the user types on the device is shown, with nothing to answer.
run passkey none 'say "[agent] Passkey: 654321"
say "[agent] Passkey: 654321"'
grep -q "Type 654321 on the device you are pairing" "$tmp/notify" || fail "passkey: code not shown"
[[ $(notices) == 1 ]] || fail "passkey: shown $(notices) times"
grep -q -- " -A " "$tmp/notify" && fail "passkey: a display-only notice has buttons"

# The answer channel is created only inside a runtime directory this user owns.
# The fixture is a directory owned by someone else: "/" for a normal user, and a
# directory handed to "nobody" when the suite runs as root, who owns "/" itself.
foreign=/
if [[ $(id -u) == 0 ]]; then
  foreign=$tmp/foreign
  mkdir -m 700 "$foreign"
  chown 65534:65534 "$foreign" 2>/dev/null || foreign=
fi
if [[ -n $foreign ]]; then
  if STUB_DIR=$tmp PATH="$tmp/bin:$PATH" XDG_RUNTIME_DIR=$foreign timeout 10 bash "$agent" >"$tmp/foreign.out" 2>&1; then
    fail "startup: ran with a runtime directory the user does not own"
  fi
  grep -q "not a private runtime directory" "$tmp/foreign.out" || fail "startup: no reason given for refusing"
else
  echo "bt-agent-confirm: skipped the foreign-owner check, no directory to hand to another user" >&2
fi

# One that other users can write to is no safer: they could swap the FIFO.
mkdir -m 777 "$tmp/open"
chmod 777 "$tmp/open"
if STUB_DIR=$tmp PATH="$tmp/bin:$PATH" XDG_RUNTIME_DIR="$tmp/open" timeout 10 bash "$agent" >"$tmp/open.out" 2>&1; then
  fail "startup: ran with a runtime directory other users can write to"
fi
grep -q "not a private runtime directory" "$tmp/open.out" || fail "startup: no reason given for a writable runtime directory"

# What a crashed run left behind is removed, not trusted, and the new run works
# in a directory of its own.
mkdir -p "$tmp/run/ryoku-bt-agent" "$tmp/run/ryoku-bt-agent.stale"
mkfifo "$tmp/run/ryoku-bt-agent/stdin"
: >"$tmp/run/ryoku-bt-agent.stale/pending"
run leftover yes "$confirm"
typed_is yes "a leftover state directory"

echo "bt-agent-confirm: prompts asked once, answers typed back, cancels withdrawn"

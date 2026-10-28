#!/usr/bin/env bash
# fixture test for ryoku-cmd-game-mode (the one-click competitive toggle). stubs
# the shell control plane and privileged helpers, points state + sysfs scan at a
# tmp dir, so the real compositor, wifi radio and power profile are never
# touched. verifies:
#   - toggle lifecycle
#   - compositor overrides and exact reversion
#   - the power profile flips to performance and the PRIOR one comes back, not a
#     guessed default
#   - the privileged system tune is applied on start and restored on stop
#   - wifi power-save delegated to the privileged ryoku-wifi-powersave via
#     pkexec (off on start, on on stop)
#   - refuses on battery, before touching anything
#   - clean no-op when there's no wifi device or a helper is missing
#
# helpers are addressed through their env seams (RYOKU_*_BIN) rather than by
# removing a stub from PATH. a bare PATH removal cannot express "absent": on a
# deployed machine `command -v` still finds the real helper in /usr/bin, so the
# absent-helper case silently tested nothing and failed for weeks.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
gm="$here/../ryoku/shell/scripts/ryoku-cmd-game-mode"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

bin="$tmp/bin"; mkdir -p "$bin"
calls="$tmp/calls.log"

logger() {  # name -> stub that records its argv and exits 0
  cat >"$bin/$1" <<EOF
#!/usr/bin/env bash
echo "$1 \$*" >>"$calls"
exit 0
EOF
}
logger pkexec
logger ryoku-wifi-powersave
logger ryoku-game-tune

printf '#!/usr/bin/env bash\necho "notify-send $*" >> "%s"\nexit 0\n' "$calls" >"$bin/notify-send"

# ryoku-shell stub with a real backing value. It exposes the same state and set
# calls as the daemon, so a wrong restore is visible rather than merely logged.
profile="$tmp/profile"; echo balanced >"$profile"
cat >"$bin/ryoku-shell" <<EOF
#!/usr/bin/env bash
echo "ryoku-shell \$*" >>"$calls"
case "\${1:-}:\${2:-}" in
  powerprofiles:state)
    jq -cn --arg active "\$(cat "$profile")" \
      '{active_profile:\$active,profiles:["power-saver","balanced","performance"]}'
    ;;
  powerprofiles:set)
    printf '%s\n' "\${3:-}" >"$profile"
    ;;
  *) exit 1 ;;
esac
EOF

# ryoku-idle owns the AC/battery question; exit 1 = on AC (permissive default).
ac="$tmp/on_ac"; : >"$ac"
cat >"$bin/ryoku-idle" <<EOF
#!/usr/bin/env bash
echo "ryoku-idle \$*" >>"$calls"
[[ -f "$ac" ]] && exit 1
exit 0
EOF

# A stubbed ryoku CLI reports compositor capabilities and models the observable
# decoration override. A caps line without live evaluation stands in for a
# compositor that must be left untouched.
deco="$tmp/decorations"
mk_ryoku() {  # path caps -> a ryoku stub whose `wm status` prints caps
  cat >"$1" <<EOF
#!/usr/bin/env bash
case "\$*" in
  "wm status") echo "Capabilities: $2" ;;
  "wm act decoration.gameMode on") touch "$deco" ;;
  "wm act decoration.gameMode off") rm -f "$deco" ;;
esac
EOF
  chmod +x "$1"
}
mk_ryoku "$bin/ryoku" "animations, configReload, cursorSet, liveConfigEval, windowRules"
chmod +x "$bin"/*

# fake sysfs with one wifi device (overridable per-test).
net="$tmp/net"; mkdir -p "$net/wlan0/wireless" "$net/eth0"
nonet="$tmp/nonet"; mkdir -p "$nonet/eth0"

export PATH="$bin:$PATH"
export RYOKU_STATE_PATH="$tmp/state"
export RYOKU_GAMEMODE_STATE_FILE="$tmp/state/game-mode.enabled"
export RYOKU_GAMEMODE_PROFILE_FILE="$tmp/state/game-mode.profile"
export RYOKU_NET_SYSFS="$net"
export RYOKU_WIFI_POWERSAVE_BIN="$bin/ryoku-wifi-powersave"
export RYOKU_GAME_TUNE_BIN="$bin/ryoku-game-tune"
export RYOKU_IDLE_BIN="$bin/ryoku-idle"
export RYOKU_SHELL_BIN="$bin/ryoku-shell"
export RYOKU_BIN="$bin/ryoku"
state="$RYOKU_GAMEMODE_STATE_FILE"

fail() { echo "FAIL: $1" >&2; exit 1; }
on() { [[ -f $state ]]; }

"$gm" status && fail "status reported on before any start"

# --- start: compositor override + profile + tune + wifi off -----------------
: >"$calls"
"$gm" start
on || fail "start did not persist the request"
[[ -e $deco ]] || fail "start did not apply the compositor override"
grep -qF 'ryoku-game-tune apply' "$calls" || fail "system tune not applied"
grep -qE 'pkexec .*ryoku-wifi-powersave off' "$calls" || fail "WiFi helper not asked to disable power-save"
[[ "$(cat "$profile")" == performance ]] || fail "profile did not actually reach performance"
"$gm" status || fail "status reported off while on"

# --- stop: reload + tune restored + the PRIOR profile back -----------------
: >"$calls"
"$gm" stop
on && fail "stop did not clear the request"
[[ ! -e $deco ]] || fail "stop did not restore the compositor"
grep -qF 'ryoku-game-tune restore' "$calls" || fail "system tune not restored"
grep -qE 'pkexec .*ryoku-wifi-powersave on' "$calls" || fail "WiFi helper not asked to restore power-save"
[[ "$(cat "$profile")" == balanced ]] ||
  fail "restored the wrong profile (want the prior 'balanced', got '$(cat "$profile")')"
"$gm" status && fail "status reported on after stop"

# --- the prior profile is whatever was there, not a hardcoded default ------
echo power-saver >"$profile"
"$gm" start
[[ "$(cat "$profile")" == performance ]] || fail "second start did not reach performance"
"$gm" stop
[[ "$(cat "$profile")" == power-saver ]] ||
  fail "did not restore a non-default prior profile (got '$(cat "$profile")')"

# --- toggle flips both ways ------------------------------------------------
"$gm" toggle; on || fail "toggle did not turn on"
"$gm" toggle; on && fail "toggle did not turn off"

# --- on battery: refuses, and touches nothing ------------------------------
rm -f "$ac"                      # ryoku-idle now reports discharging
echo balanced >"$profile"
: >"$calls"
"$gm" start && fail "start succeeded on battery"
on && fail "a refused start still persisted the request"
[[ ! -e $deco ]] || fail "a refused start changed the compositor"
grep -qF 'ryoku-game-tune' "$calls" && fail "applied the privileged tune on battery"
[[ "$(cat "$profile")" == balanced ]] || fail "changed the power profile on battery"
: >"$ac"                         # back on AC

# --- no ryoku-idle at all: treated as AC, matching the house default -------
: >"$calls"
RYOKU_IDLE_BIN="$tmp/nope" "$gm" start
on || fail "refused to start when the AC/battery helper is absent"
RYOKU_IDLE_BIN="$tmp/nope" "$gm" stop

# --- no wifi device: compositor still applies, wifi is a clean no-op -------
: >"$calls"
RYOKU_NET_SYSFS="$nonet" "$gm" start
on || fail "start failed on a no-WiFi host"
[[ -e $deco ]] || fail "compositor did not apply on a no-WiFi host"
grep -qF 'ryoku-wifi-powersave' "$calls" && fail "touched WiFi on a host with no WiFi device"
RYOKU_NET_SYSFS="$nonet" "$gm" stop

# --- helpers absent: the rest still applies --------------------------------
: >"$calls"
RYOKU_WIFI_POWERSAVE_BIN="$tmp/nope" RYOKU_GAME_TUNE_BIN="$tmp/nope" "$gm" start
[[ -e $deco ]] || fail "compositor did not apply when helpers were absent"
grep -qF 'ryoku-wifi-powersave' "$calls" && fail "tried to call an absent WiFi helper"
grep -qF 'ryoku-game-tune' "$calls" && fail "tried to call an absent tune helper"
RYOKU_WIFI_POWERSAVE_BIN="$tmp/nope" RYOKU_GAME_TUNE_BIN="$tmp/nope" "$gm" stop

# --- a compositor with no live config eval: power still boosts, the desktop is
#     left untouched (no eval on start, no reload on stop) --------------------
mk_ryoku "$tmp/ryoku-nolive" "animations, windowRules, workspaces"
: >"$calls"
RYOKU_BIN="$tmp/ryoku-nolive" "$gm" start
on || fail "start did not persist where the compositor has no live config eval"
[[ "$(cat "$profile")" == performance ]] ||
  fail "power did not boost without a compositor strip"
grep -qF 'ryoku-game-tune apply' "$calls" || fail "system tune skipped without a compositor strip"
: >"$calls"
RYOKU_BIN="$tmp/ryoku-nolive" "$gm" stop
[[ ! -e $deco ]] || fail "compositor without live config evaluation was changed"
on && fail "stop did not clear the request without a compositor strip"

# A failed provider action must not advertise Game Mode as on or tune hardware.
cat >"$tmp/ryoku-broken" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  "wm status") echo "Capabilities: configReload, liveConfigEval" ;;
  "wm act decoration.gameMode on") echo "eval rejected" >&2; exit 7 ;;
esac
EOF
chmod +x "$tmp/ryoku-broken"
: >"$calls"
RYOKU_BIN="$tmp/ryoku-broken" "$gm" start >"$tmp/failure.log" 2>&1 &&
  fail "start succeeded despite a rejected decoration strip"
on && fail "a rejected decoration strip left Game Mode on"
grep -qF 'eval rejected' "$tmp/failure.log" || fail "strip error was hidden"
grep -qF 'ryoku-game-tune apply' "$calls" &&
  fail "tuned hardware after the decoration strip failed"

# A failed compositor reload cannot strand the power, tune or Wi-Fi restores.
cat >"$tmp/ryoku-restore-fails" <<EOF
#!/usr/bin/env bash
case "\$*" in
  "wm status") echo "Capabilities: configReload, liveConfigEval" ;;
  "wm act decoration.gameMode on") touch "$deco" ;;
  "wm act decoration.gameMode off") echo "reload rejected" >&2; exit 7 ;;
esac
EOF
chmod +x "$tmp/ryoku-restore-fails"
echo balanced >"$profile"
RYOKU_BIN="$tmp/ryoku-restore-fails" "$gm" start
[[ "$(cat "$profile")" == performance ]] || fail "failed-reload setup did not boost power"
: >"$calls"
RYOKU_BIN="$tmp/ryoku-restore-fails" "$gm" stop >"$tmp/restore-failure.log" 2>&1 &&
  fail "stop reported success despite failed compositor restore"
on && fail "failed compositor restore left Game Mode on"
grep -qF 'reload rejected' "$tmp/restore-failure.log" || fail "restore error was hidden"
grep -qF 'ryoku-game-tune restore' "$calls" || fail "failed reload skipped tune restore"
grep -qF 'ryoku-shell powerprofiles set balanced' "$calls" || fail "failed reload skipped profile restore"
grep -qF 'ryoku-wifi-powersave on' "$calls" || fail "failed reload skipped Wi-Fi restore"
grep -qF 'Off - desktop restored' "$calls" &&
  fail "failed reload announced successful desktop restoration"

echo "game-mode: all checks passed"

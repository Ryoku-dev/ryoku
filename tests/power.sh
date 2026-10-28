#!/usr/bin/env bash
# hermetic test for ryoku-power: the battery charge-limit and PCIe ASPM levers.
# All four seams point at tmp fixtures -- RYOKU_POWER_SUPPLY_DIR (battery),
# RYOKU_ASPM_POLICY_FILE (policy), RYOKU_POWER_CONFIG (the desired-state JSON),
# and RYOKU_ASSUME_LAPTOP (the ryoku-hw-laptop gate) -- so no real sysfs is read
# or written and no root is needed: the tmp attribute files are writable, so the
# privileged writes go direct and never reach sudo. RYOKU_POWER_CONFIG is kept
# inside the tmp dir precisely so the real ~/.config/ryoku/power.json is never
# created or touched (asserted at the end).
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
helper="$here/../system/hardware/power/ryoku-power"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fail() { echo "FAIL: $1" >&2; exit 1; }

# snapshot the user's real config so we can prove this test never touches it.
realcfg="${XDG_CONFIG_HOME:-$HOME/.config}/ryoku/power.json"
realcfg_state="absent"; [[ -e $realcfg ]] && realcfg_state="$(cksum <"$realcfg")"

noaspm="$tmp/no-aspm"                  # a policy file that does not exist
nops="$tmp/no-ps"; mkdir -p "$nops"    # a power-supply dir with no battery
shell_stub="$tmp/ryoku-shell"
cat >"$shell_stub" <<'EOF'
#!/usr/bin/env bash
if [[ ${1:-}:${2:-} == powerprofiles:state ]]; then
  printf '%s\n' '{"active_profile":"balanced","profiles":["balanced","power-saver"]}'
else
  exit 1
fi
EOF
chmod +x "$shell_stub"

mk_batt() { # <ps-dir> [value]: <dir>/BAT0/charge_control_end_threshold
  mkdir -p "$1/BAT0"
  if [[ $# -ge 2 ]]; then
    printf '%s\n' "$2" >"$1/BAT0/charge_control_end_threshold"
  else                                 # empty file == the kernel's pre-write ENODATA
    : >"$1/BAT0/charge_control_end_threshold"
  fi
}
batt_file() { printf '%s/BAT0/charge_control_end_threshold' "$1"; }
mk_aspm() { mkdir -p "$(dirname "$1")"; printf '%s\n' "$2" >"$1"; }  # <file> <content>

# run the helper with all four seams pinned, so nothing leaks to real hardware.
rp() { # <ps> <aspm> <cfg> <laptop:1|0> [args...]
  local ps="$1" aspm="$2" cfg="$3" lap="$4"; shift 4
  RYOKU_POWER_SUPPLY_DIR="$ps" RYOKU_ASPM_POLICY_FILE="$aspm" \
    RYOKU_POWER_CONFIG="$cfg" RYOKU_ASSUME_LAPTOP="$lap" \
    RYOKU_CPU_DIR="$tmp/no-cpu" RYOKU_PLATFORM_PROFILE_FILE="$tmp/no-pp/platform_profile" \
    RYOKU_SHELL_BIN="$shell_stub" "$helper" "$@"
}

# build a fake CPU sysfs tree: <cpu-dir> <ncpus>. Values mirror the real
# amd-pstate-epp box so the boost ceiling math is exercised: highest_perf 196,
# nominal_perf 149, nominal_freq 4001 -> ceiling 5263060 kHz.
mk_cpu() {
  local root="$1" n="$2" i d
  for (( i = 0; i < n; i++ )); do
    d="$root/cpu$i/cpufreq"; mkdir -p "$d"
    printf 'performance powersave\n' >"$d/scaling_available_governors"
    printf 'default performance balance_performance balance_power power custom \n' \
      >"$d/energy_performance_available_preferences"
    printf 'powersave\n' >"$d/scaling_governor"
    printf 'power\n'      >"$d/energy_performance_preference"
    printf '4001000\n'    >"$d/scaling_max_freq"
    printf '4001000\n'    >"$d/cpuinfo_max_freq"
    printf '402786\n'     >"$d/cpuinfo_min_freq"
    mkdir -p "$root/cpu$i/acpi_cppc"
    printf '196\n'  >"$root/cpu$i/acpi_cppc/highest_perf"
    printf '149\n'  >"$root/cpu$i/acpi_cppc/nominal_perf"
    printf '4001\n' >"$root/cpu$i/acpi_cppc/nominal_freq"
  done
}

# run the helper with the CPU seams pinned and battery/aspm neutralised.
rpc() { # <cpu-dir> <pp-file|absent> <cfg> [args...]
  local cpu="$1" pp="$2" cfg="$3"; shift 3
  RYOKU_CPU_DIR="$cpu" RYOKU_PLATFORM_PROFILE_FILE="$pp" RYOKU_POWER_CONFIG="$cfg" \
    RYOKU_POWER_SUPPLY_DIR="$nops" RYOKU_ASPM_POLICY_FILE="$noaspm" \
    RYOKU_SHELL_BIN="$shell_stub" "$helper" "$@"
}

# --- charge-limit get: unset for an ENODATA-like read, the number once set ----
psA="$tmp/psA"; mk_batt "$psA"
cfgA="$tmp/cfgA/power.json"
[[ "$(rp "$psA" "$noaspm" "$cfgA" 1 charge-limit get)" == unset ]] \
  || fail "empty threshold did not read as unset"
rp "$psA" "$noaspm" "$cfgA" 1 charge-limit set 80
[[ "$(cat "$(batt_file "$psA")")" == 80 ]] || fail "set 80 did not write the sysfs attribute"
jq -e '.chargeLimit == 80' "$cfgA" >/dev/null || fail "set 80 did not persist to the config"
[[ "$(rp "$psA" "$noaspm" "$cfgA" 1 charge-limit get)" == 80 ]] || fail "get did not read back 80"

# --- charge-limit set: range + numeric validation, no write on rejection ------
psB="$tmp/psB"; mk_batt "$psB" 70
cfgB="$tmp/cfgB/power.json"
for good in 50 100; do
  rp "$psB" "$noaspm" "$cfgB" 1 charge-limit set "$good" \
    || fail "set rejected the in-range value $good"
  [[ "$(cat "$(batt_file "$psB")")" == "$good" ]] || fail "set $good did not write"
done
before="$(cat "$(batt_file "$psB")")"
for bad in 49 101 40 120 abc ""; do
  if rp "$psB" "$noaspm" "$cfgB" 1 charge-limit set "$bad" >/dev/null 2>&1; then
    fail "set accepted the invalid value '$bad'"
  fi
done
[[ "$(cat "$(batt_file "$psB")")" == "$before" ]] || fail "a rejected set still wrote the attribute"

# --- charge-limit clear: back to 100, key dropped, sibling preserved ----------
psC="$tmp/psC"; mk_batt "$psC" 80
cfgC="$tmp/cfgC/power.json"; mkdir -p "$(dirname "$cfgC")"
printf '{"chargeLimit":80,"aspm":"powersave"}\n' >"$cfgC"
rp "$psC" "$noaspm" "$cfgC" 1 charge-limit clear
[[ "$(cat "$(batt_file "$psC")")" == 100 ]] || fail "clear did not restore 100"
jq -e 'has("chargeLimit") | not' "$cfgC" >/dev/null || fail "clear left chargeLimit in the config"
jq -e '.aspm == "powersave"' "$cfgC" >/dev/null || fail "clear dropped the sibling aspm key"

# --- no battery: get is unsupported (non-zero), but apply is a clean no-op -----
cfgD="$tmp/no-cfgD/power.json"
if rp "$nops" "$noaspm" "$cfgD" 1 charge-limit get >/dev/null 2>&1; then
  fail "charge-limit get succeeded with no battery"
fi
rp "$nops" "$noaspm" "$cfgD" 1 apply || fail "apply errored on a machine with no knobs"

# --- aspm: get unwraps [selected], set validates against the offered list -----
aspmE="$tmp/aspmE/policy"; mk_aspm "$aspmE" '[default] performance powersave powersupersave'
cfgE="$tmp/cfgE/power.json"
[[ "$(rp "$nops" "$aspmE" "$cfgE" 1 aspm get)" == default ]] || fail "aspm get did not unwrap [default]"
if rp "$nops" "$aspmE" "$cfgE" 1 aspm set turbo >/dev/null 2>&1; then
  fail "aspm set accepted a policy absent from the option list"
fi
grep -q '\[default\]' "$aspmE" || fail "a rejected aspm set mutated the policy file"
rp "$nops" "$aspmE" "$cfgE" 1 aspm set powersave || fail "aspm set rejected an offered policy"
[[ "$(cat "$aspmE")" == powersave ]] || fail "aspm set did not write the policy"
jq -e '.aspm == "powersave"' "$cfgE" >/dev/null || fail "aspm set did not persist to the config"

# --- status --json: exactly four keys; charge_limit is -1 when unset/unsupported
psF="$tmp/psF"; mk_batt "$psF"
aspmF="$tmp/aspmF/policy"; mk_aspm "$aspmF" '[default] performance powersave'
cfgF="$tmp/cfgF/power.json"
json="$(rp "$psF" "$aspmF" "$cfgF" 1 status --json)"
jq -e . <<<"$json" >/dev/null || fail "status --json is not valid JSON"
keys="$(jq -r 'keys_unsorted | join(",")' <<<"$json")"
[[ $keys == "charge_limit,charge_limit_supported,aspm,aspm_supported" ]] \
  || fail "status --json keys are not exactly the four contracted ones: $keys"
jq -e '.charge_limit == -1 and .charge_limit_supported == true' <<<"$json" >/dev/null \
  || fail "unset charge_limit was not reported as -1"
json="$(rp "$nops" "$aspmF" "$cfgF" 1 status --json)"
jq -e '.charge_limit == -1 and .charge_limit_supported == false' <<<"$json" >/dev/null \
  || fail "unsupported charge_limit was not reported as -1"

# --- apply with no config file changes nothing (byte-identical) and exits 0 ----
psG="$tmp/psG"; mk_batt "$psG" 55
aspmG="$tmp/aspmG/policy"; mk_aspm "$aspmG" '[default] performance powersave'
cfgG="$tmp/no-cfgG/power.json"                 # absent
cp "$(batt_file "$psG")" "$tmp/psG.orig"; cp "$aspmG" "$tmp/aspmG.orig"
rp "$psG" "$aspmG" "$cfgG" 1 apply || fail "apply with no config errored"
cmp -s "$(batt_file "$psG")" "$tmp/psG.orig" || fail "apply with no config wrote the battery attribute"
cmp -s "$aspmG" "$tmp/aspmG.orig" || fail "apply with no config wrote the aspm policy"
[[ -e $cfgG ]] && fail "apply created a config file that did not exist"

# --- apply with a config converges the hardware, idempotently -----------------
psH="$tmp/psH"; mk_batt "$psH"
aspmH="$tmp/aspmH/policy"; mk_aspm "$aspmH" '[default] performance powersave powersupersave'
cfgH="$tmp/cfgH/power.json"; mkdir -p "$(dirname "$cfgH")"
printf '{"chargeLimit":75,"aspm":"powersave"}\n' >"$cfgH"
rp "$psH" "$aspmH" "$cfgH" 1 apply || fail "apply with a config errored"
[[ "$(cat "$(batt_file "$psH")")" == 75 ]] || fail "apply did not converge the charge limit"
[[ "$(cat "$aspmH")" == powersave ]] || fail "apply did not converge the aspm policy"
rp "$psH" "$aspmH" "$cfgH" 1 apply || fail "second apply errored"
[[ "$(cat "$(batt_file "$psH")")" == 75 ]] || fail "second apply drifted the charge limit"
[[ "$(cat "$aspmH")" == powersave ]] || fail "second apply drifted the aspm policy"

# --- the laptop gate: a desktop (RYOKU_ASSUME_LAPTOP=0) touches nothing --------
psI="$tmp/psI"; mk_batt "$psI" 90
aspmI="$tmp/aspmI/policy"; mk_aspm "$aspmI" '[default] performance powersave'
cfgI="$tmp/cfgI/power.json"; mkdir -p "$(dirname "$cfgI")"
printf '{"chargeLimit":60,"aspm":"powersave"}\n' >"$cfgI"
cp "$(batt_file "$psI")" "$tmp/psI.orig"; cp "$aspmI" "$tmp/aspmI.orig"
rp "$psI" "$aspmI" "$cfgI" 0 apply || fail "apply on a desktop was not a clean exit"
cmp -s "$(batt_file "$psI")" "$tmp/psI.orig" || fail "desktop apply wrote the battery attribute"
cmp -s "$aspmI" "$tmp/aspmI.orig" || fail "desktop apply wrote the aspm policy"

# --- capabilities: a CPU tree without platform_profile reports three knobs ----
cpuJ="$tmp/cpuJ"; mk_cpu "$cpuJ" 2
cfgJ="$tmp/cfgJ/power.json"
capsJ="$(rpc "$cpuJ" "$tmp/no-pp/platform_profile" "$cfgJ" capabilities --json)"
jq -e . <<<"$capsJ" >/dev/null || fail "capabilities --json is not valid JSON"
[[ "$(jq -r '.cpu | keys_unsorted | join(",")' <<<"$capsJ")" == "governor,epp,maxFreqPct" ]] \
  || fail "capabilities did not report exactly three cpu knobs without platform_profile"
jq -e '.cpu.governor.options == ["performance","powersave"]' <<<"$capsJ" >/dev/null \
  || fail "capabilities governor options wrong"
jq -e '.cpu.epp.options == ["default","performance","balance_performance","balance_power","power"]' <<<"$capsJ" >/dev/null \
  || fail "capabilities epp options did not drop the raw-bias 'custom' preference"
jq -e '.activeProfile == "balanced" and .profiles == ["balanced","power-saver"]' <<<"$capsJ" >/dev/null \
  || fail "capabilities did not use ryoku-shell's active profile and ordered profile list"
# maxFreqPct reports what someone actually asked for, never a ratio inferred from
# the driver's resting state. This used to assert 76, being 4001000 of a 5263060
# ceiling, which on amd-pstate-epp is only scaling_max_freq sitting at the nominal
# clock while boost rides the CPPC perf request: an unconstrained machine
# presented as three-quarters capped, one slider drag from becoming truly capped.
jq -e '.cpu.maxFreqPct == {min:20,max:100,current:100}' <<<"$capsJ" >/dev/null \
  || fail "an unconfigured machine should report maxFreqPct 100 (uncapped), not a derived ratio"
jq -e '(has("battery")|not) and (has("aspm")|not)' <<<"$capsJ" >/dev/null \
  || fail "capabilities emitted battery/aspm whose sources were absent"

# --- profile set: validates against live caps, persists, preserves siblings ---
cpuK="$tmp/cpuK"; mk_cpu "$cpuK" 2
ppK="$tmp/ppK/platform_profile"; mkdir -p "$(dirname "$ppK")"
printf 'quiet\n' >"$ppK"; printf 'quiet balanced performance\n' >"$tmp/ppK/platform_profile_choices"
cfgK="$tmp/cfgK/power.json"; mkdir -p "$(dirname "$cfgK")"
printf '{"chargeLimit":80}\n' >"$cfgK"

rpc "$cpuK" "$ppK" "$cfgK" profile set balanced governor powersave \
  || fail "profile set rejected a valid governor"
if rpc "$cpuK" "$ppK" "$cfgK" profile set balanced governor turbo >/dev/null 2>&1; then
  fail "profile set accepted a governor absent from scaling_available_governors"
fi
if rpc "$cpuK" "$ppK" "$cfgK" profile set balanced epp custom >/dev/null 2>&1; then
  fail "profile set accepted the filtered 'custom' epp"
fi
for bad in 10 19 101 150 abc ""; do
  if rpc "$cpuK" "$ppK" "$cfgK" profile set balanced maxFreqPct "$bad" >/dev/null 2>&1; then
    fail "profile set accepted an out-of-range maxFreqPct: '$bad'"
  fi
done
rpc "$cpuK" "$ppK" "$cfgK" profile set balanced maxFreqPct 60 \
  || fail "profile set rejected a valid maxFreqPct"
if rpc "$cpuK" "$ppK" "$cfgK" profile set nope governor powersave >/dev/null 2>&1; then
  fail "profile set accepted an unknown profile name"
fi
if rpc "$cpuK" "$ppK" "$cfgK" profile set balanced bogusKey x >/dev/null 2>&1; then
  fail "profile set accepted an unknown key"
fi
jq -e '.chargeLimit == 80' "$cfgK" >/dev/null || fail "profile set dropped the sibling chargeLimit"
jq -e '.profiles.balanced.governor == "powersave" and .profiles.balanced.maxFreqPct == 60' "$cfgK" >/dev/null \
  || fail "profile set did not persist governor + maxFreqPct"
jq -e '.governor == "powersave" and .maxFreqPct == 60' \
  <(rpc "$cpuK" "$ppK" "$cfgK" profile get balanced) >/dev/null \
  || fail "profile get did not read back the saved definition"

# --- profile clear: a knob set once must be removable -------------------------
# Without this there was no way back: `set <p> <key> ""` is rejected by the
# validators and nothing else removed a key, so a definition could only be undone
# by hand-editing power.json. These are re-applied at every login and every
# profile switch, so an undoable-only-by-hand cap is a trap.
rpc "$cpuK" "$ppK" "$cfgK" profile clear balanced maxFreqPct \
  || fail "profile clear rejected a valid key"
jq -e '(.profiles.balanced | has("maxFreqPct") | not) and .profiles.balanced.governor == "powersave"' "$cfgK" >/dev/null \
  || fail "profile clear <key> did not drop just that key"
if rpc "$cpuK" "$ppK" "$cfgK" profile clear balanced bogusKey >/dev/null 2>&1; then
  fail "profile clear accepted an unknown key"
fi
if rpc "$cpuK" "$ppK" "$cfgK" profile clear nope >/dev/null 2>&1; then
  fail "profile clear accepted an unknown profile"
fi
rpc "$cpuK" "$ppK" "$cfgK" profile clear balanced \
  || fail "profile clear rejected a whole profile"
jq -e '(.profiles | has("balanced") | not)' "$cfgK" >/dev/null \
  || fail "profile clear <profile> did not drop the whole definition"
jq -e '.chargeLimit == 80' "$cfgK" >/dev/null \
  || fail "profile clear dropped the sibling chargeLimit"

# --- apply-profile: maxFreqPct -> kHz on every cpu, absent keys skipped -------
cpuL="$tmp/cpuL"; mk_cpu "$cpuL" 2
ppL="$tmp/ppL/platform_profile"; mkdir -p "$(dirname "$ppL")"
printf 'quiet\n' >"$ppL"; printf 'quiet balanced performance\n' >"$tmp/ppL/platform_profile_choices"
cfgL="$tmp/cfgL/power.json"; mkdir -p "$(dirname "$cfgL")"
cat >"$cfgL" <<'JSON'
{"chargeLimit":80,"profiles":{
  "power-saver":{"maxFreqPct":20},
  "performance":{"governor":"performance","epp":"performance","maxFreqPct":100,"platformProfile":"performance"}
}}
JSON

# only maxFreqPct=20 -> 5263060*20/100 = 1052612 kHz; gov/epp/pp left untouched.
rpc "$cpuL" "$ppL" "$cfgL" apply-profile power-saver || fail "apply-profile power-saver errored"
for c in "$cpuL"/cpu[0-9]*; do
  [[ "$(cat "$c/cpufreq/scaling_max_freq")" == 1052612 ]] || fail "apply-profile 20% wrong kHz on $(basename "$c")"
  [[ "$(cat "$c/cpufreq/scaling_governor")" == powersave ]] || fail "apply-profile 20% touched governor (absent key)"
  [[ "$(cat "$c/cpufreq/energy_performance_preference")" == power ]] || fail "apply-profile 20% touched epp (absent key)"
done
[[ "$(cat "$ppL")" == quiet ]] || fail "apply-profile 20% touched platform_profile (absent key)"

# all four keys -> 100% = 5263060 kHz, plus governor/epp/platform_profile.
rpc "$cpuL" "$ppL" "$cfgL" apply-profile performance || fail "apply-profile performance errored"
for c in "$cpuL"/cpu[0-9]*; do
  [[ "$(cat "$c/cpufreq/scaling_max_freq")" == 5263060 ]] || fail "apply-profile 100% wrong kHz on $(basename "$c")"
  [[ "$(cat "$c/cpufreq/scaling_governor")" == performance ]] || fail "apply-profile did not write governor"
  [[ "$(cat "$c/cpufreq/energy_performance_preference")" == performance ]] || fail "apply-profile did not write epp"
done
[[ "$(cat "$ppL")" == performance ]] || fail "apply-profile did not write platform_profile"

rpc "$cpuL" "$ppL" "$cfgL" apply-profile balanced || fail "apply-profile of an undefined profile errored"
[[ "$(cat "$cpuL/cpu0/cpufreq/scaling_max_freq")" == 5263060 ]] \
  || fail "apply-profile of an undefined profile changed sysfs"

# --- SAFETY: apply with no profiles block writes no CPU knob and never escalates
cpuM="$tmp/cpuM"; mk_cpu "$cpuM" 2
ppM="$tmp/ppM/platform_profile"; mkdir -p "$(dirname "$ppM")"; printf 'quiet\n' >"$ppM"
printf 'quiet balanced performance\n' >"$tmp/ppM/platform_profile_choices"
# read-only write targets: any CPU write would fall through to the pkexec branch.
chmod 0444 "$cpuM"/cpu*/cpufreq/scaling_max_freq "$cpuM"/cpu*/cpufreq/scaling_governor \
  "$cpuM"/cpu*/cpufreq/energy_performance_preference "$ppM"
cfgM="$tmp/cfgM/power.json"; mkdir -p "$(dirname "$cfgM")"
printf '{"chargeLimit":80}\n' >"$cfgM"                 # no profiles block
stub="$tmp/stub"; mkdir -p "$stub"; marker="$tmp/pkexec-called"
printf '#!/usr/bin/env bash\ntouch "%s"\nexit 0\n' "$marker" >"$stub/pkexec"; chmod +x "$stub/pkexec"
PATH="$stub:$PATH" RYOKU_CPU_DIR="$cpuM" RYOKU_PLATFORM_PROFILE_FILE="$ppM" \
  RYOKU_POWER_CONFIG="$cfgM" RYOKU_POWER_SUPPLY_DIR="$nops" RYOKU_ASPM_POLICY_FILE="$noaspm" \
  RYOKU_ASSUME_LAPTOP=1 RYOKU_SHELL_BIN="$shell_stub" "$helper" apply \
  || fail "apply with no profiles block did not exit 0"
[[ ! -e $marker ]] || fail "apply with no profiles block invoked pkexec"
for c in "$cpuM"/cpu[0-9]*; do
  [[ "$(cat "$c/cpufreq/scaling_max_freq")" == 4001000 ]] || fail "apply with no profiles block wrote a CPU knob"
done

# --- authorization: a PATH-shadowing dev helper escalates only the packaged path
rule="$here/../system/hardware/power/47-ryoku-power.rules"
grep -Fq 'action.lookup("program") === "/usr/bin/ryoku-power"' "$rule" \
  || fail "power rule does not stay pinned to the packaged helper"
grep -Fq 'subject.local && subject.active && subject.isInGroup("wheel")' "$rule" \
  || fail "power rule no longer requires an active local administrator"

cpuN="$tmp/cpuN"; mk_cpu "$cpuN" 1
cfgN="$tmp/cfgN/power.json"; mkdir -p "$(dirname "$cfgN")"
printf '{"profiles":{"balanced":{"governor":"performance"}}}\n' >"$cfgN"
chmod 0444 "$cpuN/cpu0/cpufreq/scaling_governor"
stubN="$tmp/stubN"; mkdir -p "$stubN"; pkexec_argv="$tmp/pkexec-argv"
cat >"$stubN/pkexec" <<EOF
#!/usr/bin/env bash
printf '%s\n' "\$@" >"$pkexec_argv"
EOF
chmod +x "$stubN/pkexec"
install -m755 "$helper" "$stubN/ryoku-power"
install -m755 "$(dirname "$helper")/ryoku-hw-laptop" "$stubN/ryoku-hw-laptop"
PATH="$stubN:$PATH" RYOKU_CPU_DIR="$cpuN" \
  RYOKU_PLATFORM_PROFILE_FILE="$tmp/no-ppN/platform_profile" \
  RYOKU_POWER_CONFIG="$cfgN" RYOKU_POWER_SUPPLY_DIR="$nops" \
  RYOKU_ASPM_POLICY_FILE="$noaspm" RYOKU_SHELL_BIN="$shell_stub" \
  ryoku-power apply-profile balanced \
  || fail "shadow helper escalation did not exit 0"
mapfile -t pkexec_args <"$pkexec_argv"
[[ ${pkexec_args[0]:-} == /usr/bin/ryoku-power ]] \
  || fail "pkexec program did not match the policy path: ${pkexec_args[0]:-missing}"
[[ ${pkexec_args[1]:-}:${pkexec_args[2]:-} == apply-profile:balanced ]] \
  || fail "pkexec did not preserve the profile command"

# An already-converged profile needs no privileged write at all.
cpuO="$tmp/cpuO"; mk_cpu "$cpuO" 1
cfgO="$tmp/cfgO/power.json"; mkdir -p "$(dirname "$cfgO")"
printf '{"profiles":{"balanced":{"governor":"powersave","epp":"power"}}}\n' >"$cfgO"
chmod 0444 "$cpuO/cpu0/cpufreq/scaling_governor" \
  "$cpuO/cpu0/cpufreq/energy_performance_preference"
markerO="$tmp/pkexec-converged"
printf '#!/usr/bin/env bash\ntouch "%s"\nexit 0\n' "$markerO" >"$stubN/pkexec"
chmod +x "$stubN/pkexec"
PATH="$stubN:$PATH" RYOKU_CPU_DIR="$cpuO" \
  RYOKU_PLATFORM_PROFILE_FILE="$tmp/no-ppO/platform_profile" \
  RYOKU_POWER_CONFIG="$cfgO" RYOKU_POWER_SUPPLY_DIR="$nops" \
  RYOKU_ASPM_POLICY_FILE="$noaspm" RYOKU_SHELL_BIN="$shell_stub" \
  "$helper" apply-profile balanced \
  || fail "converged apply-profile did not exit 0"
[[ ! -e $markerO ]] || fail "converged apply-profile invoked pkexec"

# --- the real user config was never created or modified -----------------------
now="absent"; [[ -e $realcfg ]] && now="$(cksum <"$realcfg")"
[[ "$now" == "$realcfg_state" ]] || fail "the real $realcfg was created or modified"

echo "power: all checks passed"

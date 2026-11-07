#!/usr/bin/env bash
# Fixture test for ryoku-dev-perf-baseline --budget: feeds a recorded lab run
# (tests/perf-budget/orpheus-idle.json) and mutations of it through the budget
# comparison offline (--input), so nothing is measured. Checks that the
# recorded run passes the shipped budget (ryoku/shell/perf-budget.json), that
# each kind of overrun fails with exit 1 and a FAIL line, that the active bar
# style picks its own entry, that the lab allowances apply only to a container
# run or with --lab, and that a malformed budget or an unknown style or group
# exits 2.
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(cd "$here/.." && pwd)
tool=$root/bin/ryoku-dev-perf-baseline
budget=$root/ryoku/shell/perf-budget.json
fixture=$here/perf-budget/orpheus-idle.json

if ! command -v python3 >/dev/null 2>&1; then
  echo "SKIP: python3 required" >&2
  exit 0
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# mutate NAME SOURCE CODE: write $work/NAME.json from SOURCE after running the
# Python statements CODE on it (d is the document; rows, system and group(name)
# are shorthands).
mutate() {
  python3 - "$2" "$work/$1.json" "$3" <<'PY'
import json
import sys

src, dst, code = sys.argv[1:4]
with open(src) as f:
    d = json.load(f)
rows = d.get("rows")
system = d.get("system")


def group(name):
    return [r for r in rows if r["group"] == name]


exec(code)
with open(dst, "w") as f:
    json.dump(d, f)
PY
}

pass=0
fail=0
# check DESC WANT_RC PATTERN ARG...: run the tool with ARG..., expect exit
# status WANT_RC and a line of its output (stdout and stderr) matching PATTERN.
check() {
  local desc=$1 want=$2 pattern=$3 out rc=0
  shift 3
  out=$("$tool" "$@" 2>&1) || rc=$?
  if [[ $rc -eq $want ]] && grep -Eq -- "$pattern" <<<"$out"; then
    printf 'PASS %s\n' "$desc"
    pass=$((pass + 1))
  else
    printf 'FAIL %s\n  want exit %s and /%s/\n  got exit %s:\n%s\n' "$desc" "$want" "$pattern" "$rc" "$out" >&2
    fail=$((fail + 1))
  fi
}

row() {  # row CHECK VALUE RESULT: the budget table line for one check
  printf '^\\| %s \\| %s \\| [0-9.]+ \\| %s \\|' "$1" "$2" "$3"
}

# The recorded run passes, every check on its own line.
check "recorded Orpheus run passes" 0 'budget: 36 checks, 0 failed' --budget "$budget" --input "$fixture"
check "table has one line per check" 0 "$(row 'shell-ui PSS MB' 298.6 PASS)" --budget "$budget" --input "$fixture"
check "lab allowance applies to a container run" 0 "$(row 'shell-daemon wakeups/s' 39.3 PASS) lab allowance" \
  --budget "$budget" --input "$fixture"

# Lab allowances: off outside a container, forced on with --lab.
mutate bare "$fixture" 'system.pop("container")'
check "no lab allowance outside a container" 1 "$(row 'shell-daemon wakeups/s' 39.3 FAIL)" \
  --budget "$budget" --input "$work/bare.json"
check "--lab applies the allowances" 0 'budget: 36 checks, 0 failed' --budget "$budget" --input "$work/bare.json" --lab

# Each kind of overrun.
mutate pss "$fixture" 'group("shell-ui")[0]["pss"] = 900'
check "PSS over budget fails" 1 "$(row 'shell-ui PSS MB' 900.0 FAIL)" --budget "$budget" --input "$work/pss.json"
mutate gpu "$fixture" 'group("compositor")[0]["vram_mb"] = 400'
check "GPU memory (VRAM plus GTT) over budget fails" 1 "$(row 'compositor GPU MB' 410.0 FAIL)" \
  --budget "$budget" --input "$work/gpu.json"
mutate cpu "$fixture" 'group("shell-daemon")[0]["cpu_pct"] = 4'
check "CPU over budget fails" 1 "$(row 'shell-daemon CPU %' 4.00 FAIL)" --budget "$budget" --input "$work/cpu.json"
mutate wakeups "$fixture" 'group("wallpaper")[0]["wakeups_s"] = 24.4'
check "wakeups over budget fail" 1 "$(row 'wallpaper wakeups/s' 24.4 FAIL)" --budget "$budget" --input "$work/wakeups.json"
mutate launches "$fixture" 'system["spawns"] = {"per_min": {"wallpaper": 60.0, "other": 9.0}, "top": [["wallpaper: ryogami -> pactl", 60.0], ["other: bash -> ls", 9.0]]}'
check "a launch at idle fails its group" 1 "$(row 'wallpaper launches/min' 60.0 FAIL)" \
  --budget "$budget" --input "$work/launches.json"
check "base launches name who launched" 1 '^\| base launches/min \| 60\.0 \| 0\.0 \| FAIL \| wallpaper: ryogami -> pactl 60\.0/min \|' \
  --budget "$budget" --input "$work/launches.json"

# What cannot be measured fails rather than passes.
mutate untraced "$fixture" 'system.pop("spawns")'
check "launches not traced fail" 1 '^\| shell-ui launches/min \| n/a \| 0\.0 \| FAIL \| not measured' \
  --budget "$budget" --input "$work/untraced.json"

# Launch counts from a host-side trace of the same window.
mutate host-quiet "$fixture" 'd["rows"] = []; d["system"] = {"spawns": {"per_min": {"other": 3.0}, "top": []}}'
check "--spawns-from supplies the launch counts" 0 'budget: 36 checks, 0 failed' \
  --budget "$budget" --input "$work/untraced.json" --spawns-from "$work/host-quiet.json"
mutate host-busy "$fixture" 'd["rows"] = []; d["system"] = {"spawns": {"per_min": {"session-daemons": 2.0}, "top": [["session-daemons: sh -> bluetoothctl", 2.0]]}}'
check "--spawns-from launches fail" 1 "$(row 'session-daemons launches/min' 2.0 FAIL)" \
  --budget "$budget" --input "$work/untraced.json" --spawns-from "$work/host-busy.json"
mutate host-none "$fixture" 'd["rows"] = []; d["system"] = {}'
check "--spawns-from without launch counts is an error" 2 'has no launch counts' \
  --budget "$budget" --input "$work/untraced.json" --spawns-from "$work/host-none.json"

# Groups: unbudgeted, required and absent, optional and absent.
mutate unbudgeted "$fixture" 'rows.append({"pid": 9, "group": "visualizer-capture", "comm": "pw-record", "cmd": "pw-record", "pss": 3.0, "cpu_pct": 0.5, "wakeups_s": 50.0, "vram_mb": 0.0, "gtt_mb": 0.0})'
check "an unbudgeted base group fails" 1 '^\| visualizer-capture \| running \| none \| FAIL \| unbudgeted' \
  --budget "$budget" --input "$work/unbudgeted.json"
mutate nowall "$fixture" 'd["rows"] = [r for r in rows if r["group"] != "wallpaper"]'
check "a required group that is not running fails" 1 '^\| wallpaper \| absent \| running \| FAIL \|' \
  --budget "$budget" --input "$work/nowall.json"
mutate nobt "$fixture" 'd["rows"] = [r for r in rows if r["group"] != "session-daemons"]'
check "an optional group may be absent" 0 'budget: 31 checks, 0 failed' --budget "$budget" --input "$work/nobt.json"

# The active bar style selects its own entry: Kairos's compositor draws more
# GPU memory than Orpheus's, so 130 MB passes under Kairos and fails under Orpheus.
mutate kairos "$fixture" 'system["style"] = "kairos"; group("shell-ui")[0].update(pss=310.2, vram_mb=231.0, gtt_mb=20.1, cpu_pct=0.08, wakeups_s=8.0); group("compositor")[0].update(vram_mb=130.0, gtt_mb=0.0)'
check "Kairos is checked against the Kairos entry" 0 "$(row 'compositor GPU MB' 130.0 PASS)" \
  --budget "$budget" --input "$work/kairos.json"
check "--style overrides the run's style" 1 "$(row 'compositor GPU MB' 130.0 FAIL)" \
  --budget "$budget" --input "$work/kairos.json" --style orpheus
mutate nacre "$fixture" 'system["style"] = "nacre"'
check "a style without a budget is an error" 2 "no entry for bar style 'nacre'" --budget "$budget" --input "$work/nacre.json"
mutate nostyle "$fixture" 'system.pop("style")'
check "a run without a style is an error" 2 'records no active bar style' --budget "$budget" --input "$work/nostyle.json"

# A malformed budget exits 2 before anything is compared.
mutate bad-group "$budget" 'd["groups"]["shel-daemon"] = d["groups"].pop("shell-daemon")'
check "an unknown group in the budget is an error" 2 'names groups this script does not report: shel-daemon' \
  --budget "$work/bad-group.json" --input "$fixture"
mutate bad-metric "$budget" 'd["styles"]["kairos"]["groups"]["shell-ui"].pop("gpuMB")'
check "a group without every metric is an error" 2 'styles\.kairos\.groups\.shell-ui lacks gpuMB' \
  --budget "$work/bad-metric.json" --input "$fixture"
mutate bad-lab "$budget" 'd["groups"]["shell-daemon"]["lab"].pop("why")'
check "a lab allowance without its reason is an error" 2 'shell-daemon lacks lab\.why' \
  --budget "$work/bad-lab.json" --input "$fixture"
mutate bad-margins "$budget" 'd["margins"].pop("memoryPct")'
check "missing margins are an error" 2 'margins lack memoryPct' --budget "$work/bad-margins.json" --input "$fixture"
check "--spawns-from needs --input" 2 'spawns-from goes with --input' --budget "$budget" --spawns-from "$fixture"

printf '\n%d passed, %d failed\n' "$pass" "$fail"
[[ $fail -eq 0 ]]

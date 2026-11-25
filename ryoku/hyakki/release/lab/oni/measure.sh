#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# measure.sh SCENARIO OUT.json: measure what Oni costs in the lab as it stands.
#   SCENARIO is what the caller has already arranged:
#     motion   Oni awake and shown; measure.sh pokes it every 2 s (the control)
#     settled  awake and shown, left alone (the 30 s settled stop applies)
#     asleep   the ledger's last state is sleeping
#     covered  hidden by courtesy (a fullscreen window)
#     hidden   hidden by oni.hidden=true
#     off      oni.master=false: no oni-surface process may remain
#     strand-held       a strand is out and holding (its 30 s stop applies)
#     strand-retracted  the strand went home after a retract
#     perched  perched on a window, its strand curled (the daemon too must
#              stay within the settled budget)
#     detached after a detach: as settled, and no compositor or capture
#              process left under the daemon (the tracker is off)
#   Except for motion and off, it first waits until the ledger shows no commit
#   for ONI_MEASURE_QUIET seconds (5), giving up after ONI_MEASURE_TIMEOUT (180),
#   then samples for ONI_MEASURE_SECONDS (60). Before and after it reads, for
#   oni-surface and the oni daemon: the scope's CPUUsageNSec, utime+stime,
#   every thread's schedstat run time and timeslices, voluntary and
#   involuntary context switches, the ledger's commits and armed frame
#   callbacks, and Pss. OUT.json holds the numbers, the checks against the
#   proposed thresholds and "pass". Exit 0 when every check passes, 1 when one
#   fails, 2 on misuse. Refuses unless up.sh finished.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"

[ "$#" = 2 ] || { sed -n '3,25p' "$HERE/measure.sh" >&2; exit 2; }
case $1 in motion | settled | asleep | covered | hidden | off | strand-held | strand-retracted | perched | detached) ;; *) sed -n '3,25p' "$HERE/measure.sh" >&2; exit 2 ;; esac
lab_load
out=$2
case $out in *.json) ;; *) lab_die "OUT must end in .json" ;; esac
case $out in /*) ;; *) out=$PWD/$out ;; esac
case $out in "$ONI_LAB"/*) lab_die "OUT must be outside the lab, which down.sh removes" ;; esac
mkdir -p "$(dirname -- "$out")"
for v in "${ONI_MEASURE_SECONDS:-60}" "${ONI_MEASURE_QUIET:-5}" "${ONI_MEASURE_TIMEOUT:-180}"; do
    case $v in '' | *[!0-9.]*) lab_die "ONI_MEASURE_* must be seconds (got $v)" ;; esac
done
exec python3 "$LAB_KIT" measure "$ONI_LAB" "$1" "$out"

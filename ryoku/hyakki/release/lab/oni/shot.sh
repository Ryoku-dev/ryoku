#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# shot.sh OUT.png [GEOMETRY]: capture the lab's primary output, or the region
# GEOMETRY ("X,Y WxH" in global logical px, grim's -g form), into OUT.png and
# write OUT.json beside it with the compositor's outputs, windows, layer
# surfaces and the image's pixel statistics. Refuses unless up.sh finished, so
# it can never capture the live desktop.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
lab_load

usage() {
    printf 'usage: %s OUT.png ["X,Y WxH"]\n' "${0##*/}" >&2
    exit 2
}
[ "$#" = 1 ] || [ "$#" = 2 ] || usage
out=$1
region=${2:-}
case $out in *.png) ;; *) lab_die "OUT must end in .png" ;; esac
case $out in /*) ;; *) out=$PWD/$out ;; esac
mkdir -p "$(dirname -- "$out")"

if [ -n "$region" ]; then
    printf '%s\n' "$region" | grep -Eqx '[0-9]+,[0-9]+ [1-9][0-9]*x[1-9][0-9]*' ||
        lab_die "GEOMETRY must be \"X,Y WxH\" with non-negative integers (got $region)"
    lab_tool grim grim -g "$region" "$out"
else
    lab_tool grim grim -o "$LAB_OUTPUT" "$out"
fi
state=$(lab_state)
python3 "$LAB_KIT" shot "$out" "${out%.png}.json" "$LAB_ID" "$LAB_OUTPUT" "$region" "$state"

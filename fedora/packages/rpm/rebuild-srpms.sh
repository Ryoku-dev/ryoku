#!/usr/bin/env bash
set -euo pipefail
sources=${1:?SRPM directory}
out=${2:?new output directory}
chroot=${RYOKU_FEDORA_MOCK_ROOT:-fedora-44-x86_64}
[[ ! -e $out ]] || { echo "output already exists: $out" >&2; exit 1; }
mkdir -p "$out/RPMS" "$out/logs"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
shopt -s nullglob
sources=("$sources"/*.src.rpm)
((${#sources[@]})) || { echo 'no SRPMs to rebuild' >&2; exit 1; }
for srpm in "${sources[@]}"; do
  name=$(basename "$srpm" .src.rpm)
  result="$work/$name"
  status=0
  mock --root "$chroot" --isolation=simple --resultdir "$result" --rebuild "$srpm" || status=$?
  mkdir -p "$out/logs/$name"
  for logfile in "$result"/*.log; do cp "$logfile" "$out/logs/$name/"; done
  ((status == 0)) || exit "$status"
  for rpmfile in "$result"/*.rpm; do
    [[ $rpmfile != *.src.rpm ]] || continue
    cp "$rpmfile" "$out/RPMS/"
  done
  rm -rf "$result"
done

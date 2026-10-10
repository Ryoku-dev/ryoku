#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/../../.." && pwd)
out=$(realpath -m "${RYOKU_RPM_OUT:-$root/fedora/packages/rpm/out}")
srpm_input=${RYOKU_SRPM_IN:-}
local_build=${RYOKU_RPM_LOCAL:-0}
signing_key=${RYOKU_RPM_SIGNING_KEY:-}
die() { printf 'build-rpm-repo: %s\n' "$*" >&2; exit 1; }
for tool in rpmbuild createrepo_c cp find; do
  command -v "$tool" >/dev/null || die "missing $tool"
done
[[ $local_build == 0 || $local_build == 1 ]] || die 'RYOKU_RPM_LOCAL must be 0 or 1'
if [[ $local_build == 0 ]]; then
  [[ -n $signing_key ]] || die 'set RYOKU_RPM_SIGNING_KEY'
fi
if [[ -n $signing_key ]]; then
  for tool in gpg rpm rpmkeys rpmsign; do
    command -v "$tool" >/dev/null || die "missing $tool"
  done
fi

[[ ! -e $out ]] || die "output already exists: $out"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$out" "$work/RPMS"

if [[ -n $srpm_input ]]; then
  srpms=$(realpath -m "$srpm_input")
  [[ -d $srpms ]] || die "SRPM input is not a directory: $srpm_input"
else
  srpms="$work/SRPMS"
  RYOKU_SRPM_OUT="$srpms" "$root/fedora/packages/rpm/prepare-srpms.sh"
fi
[[ -f $srpms/release.json ]] || die "missing release metadata: $srpms/release.json"

shopt -s nullglob
source_rpms=("$srpms"/*.src.rpm)
((${#source_rpms[@]})) || die "no source RPMs found in $srpms"
for srpm in "${source_rpms[@]}"; do
  rpmbuild --define "_buildhost ryoku-builder" \
    --define "use_source_date_epoch_as_buildtime 1" \
    --define "clamp_mtime_to_source_date_epoch 1" \
    --define "_topdir $work" --define "_rpmdir $work/RPMS" --rebuild "$srpm"
done

mapfile -d '' built_rpms < <(find "$work/RPMS" -type f -name '*.rpm' -print0)
((${#built_rpms[@]})) || die 'no binary RPMs built'
for rpm_file in "${built_rpms[@]}"; do
  target="$out/${rpm_file##*/}"
  [[ ! -e $target ]] || die "duplicate RPM filename: ${target##*/}"
  cp "$rpm_file" "$target"
done

repo_rpms=("$out"/*.rpm)
if [[ -n $signing_key ]]; then
  gpg --armor --export "$signing_key" > "$work/key.asc"
  mkdir -p "$work/keydb"
  rpm --dbpath "$work/keydb" --initdb
  rpm --dbpath "$work/keydb" --import "$work/key.asc"
  for rpm_file in "${repo_rpms[@]}"; do
    rpmsign --define "_gpg_name $signing_key" --addsign "$rpm_file"
    signature=$(rpmkeys --dbpath "$work/keydb" --checksig "$rpm_file")
    [[ $signature == *"signatures OK"* ]] \
      || die "signature verification failed: $rpm_file"
  done
fi

createrepo_c "$out"
if [[ -n $signing_key ]]; then
  gpg --batch --yes --local-user "$signing_key" --armor --detach-sign \
    "$out/repodata/repomd.xml"
  gpg --verify "$out/repodata/repomd.xml.asc" "$out/repodata/repomd.xml"
fi
cp "$srpms/release.json" "$out/release.json"
printf 'Built repository at %s\n' "$out"

#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/../../.." && pwd)
git config --global --add safe.directory "$root" 2>/dev/null || true
git config --global --add safe.directory '*' 2>/dev/null || true
out=${RYOKU_RPM_OUT:-$root/fedora/packages/rpm/out}
die() { echo "build-rpm-repo: $*" >&2; exit 1; }
for tool in rpmbuild createrepo_c python3 tar gzip; do command -v "$tool" >/dev/null || die "missing $tool"; done
if [[ ${RYOKU_RPM_UNSIGNED:-0} != 1 ]]; then
  : "${RYOKU_RPM_SIGNING_KEY:?set the signing key fingerprint or RYOKU_RPM_UNSIGNED=1 for local tests}"
  [[ -z $(git -C "$root" status --porcelain) ]] || die 'signed releases require a clean checkout'
fi
[[ ! -e $out ]] || die "output already exists; use a new RYOKU_RPM_OUT to retain rollback packages"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$out"
RYOKU_SRPM_OUT="$out/SRPMS" RYOKU_RPM_LOCAL="${RYOKU_RPM_UNSIGNED:-0}" "$root/fedora/packages/rpm/prepare-srpms.sh"
for srpm in "$out"/SRPMS/*.src.rpm; do
  rpmbuild --define "_buildhost ryoku-builder" --define "use_source_date_epoch_as_buildtime 1" --define "clamp_mtime_to_source_date_epoch 1" --define "_topdir $work" --define "_rpmdir $out" --rebuild "$srpm"
done
cp "$out/SRPMS/release.json" "$out/release.json"
mapfile -d '' rpms < <(find "$out" -name '*.rpm' ! -path '*/SRPMS/*' -print0)
((${#rpms[@]})) || die 'no RPMs built'
if [[ ${RYOKU_RPM_UNSIGNED:-0} != 1 ]]; then
  gpg --armor --export "$RYOKU_RPM_SIGNING_KEY" > "$work/key.asc"
  rpm --dbpath "$work/keydb" --import "$work/key.asc"
  for rpmfile in "${rpms[@]}"; do
    rpmsign --define "_gpg_name $RYOKU_RPM_SIGNING_KEY" --addsign "$rpmfile"
    rpmkeys --dbpath "$work/keydb" --checksig "$rpmfile" | grep -q 'signatures OK' || die "signature verification failed: $rpmfile"
  done
fi
createrepo_c --excludes 'SRPMS/*' "$out"
if [[ ${RYOKU_RPM_UNSIGNED:-0} != 1 ]]; then
  gpg --batch --yes --local-user "$RYOKU_RPM_SIGNING_KEY" --armor --detach-sign "$out/repodata/repomd.xml"
  gpg --verify "$out/repodata/repomd.xml.asc" "$out/repodata/repomd.xml"
fi
printf 'Built repository at %s\n' "$out"

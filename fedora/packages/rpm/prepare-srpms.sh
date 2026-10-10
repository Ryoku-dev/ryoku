#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/../../.." && pwd)
out=$(realpath -m "${RYOKU_SRPM_OUT:-$root/fedora/packages/rpm/srpms}")
local_build=${RYOKU_RPM_LOCAL:-0}
die() { printf 'prepare-srpms: %s\n' "$*" >&2; exit 1; }
for tool in rpmbuild python3 tar gzip go git curl sha256sum; do
  command -v "$tool" >/dev/null || die "missing $tool"
done
[[ $local_build == 0 || $local_build == 1 ]] || die 'RYOKU_RPM_LOCAL must be 0 or 1'

git_cmd=(git -c "safe.directory=$root" -C "$root")
if [[ $local_build == 1 ]]; then
  pkgver=${RYOKU_PKGVER:-$("$root/bin/ryoku-release-version" --pkgver)}
  release=${RYOKU_RELEASE:-local-$pkgver}
  channel=${RYOKU_CHANNEL:-local}
else
  pkgver=${RYOKU_PKGVER:?set RYOKU_PKGVER}
  release=${RYOKU_RELEASE:?set RYOKU_RELEASE}
  channel=${RYOKU_CHANNEL:?set RYOKU_CHANNEL}
  [[ -z $("${git_cmd[@]}" status --porcelain) ]] \
    || die 'release sources require a clean checkout; use RYOKU_RPM_LOCAL=1 for local tests'
fi
[[ $pkgver =~ ^[0-9][A-Za-z0-9.]*$ ]] || die "invalid RPM package version: $pkgver"
[[ $("${git_cmd[@]}" rev-parse --is-shallow-repository) == false ]] \
  || die 'fetch full history before preparing release sources'

commit=${RYOKU_COMMIT:-$("${git_cmd[@]}" rev-parse HEAD)}
SOURCE_DATE_EPOCH=$("${git_cmd[@]}" show -s --format=%ct HEAD)
name=${RYOKU_NAME:-$(tr -d '[:space:]' < "$root/CODENAME")}
export SOURCE_DATE_EPOCH

[[ ! -e $out ]] || die "output already exists: $out"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/SOURCES" "$work/SPECS" "$work/tree/ryoku-$pkgver" "$out"

# Copy only paths in the index. Local builds still see edits to tracked files.
python3 - "$root" "$work/tree/ryoku-$pkgver" <<'PY'
import os
import pathlib
import shutil
import subprocess
import sys

root, dest = map(pathlib.Path, sys.argv[1:])
paths = subprocess.check_output([
    "git", "-c", f"safe.directory={root}", "-C", str(root),
    "ls-files", "-z", "--cached",
]).split(b"\0")
for raw in paths:
    if not raw:
        continue
    relative = pathlib.Path(os.fsdecode(raw))
    source = root / relative
    target = dest / relative
    if not source.exists() and not source.is_symlink():
        continue
    target.parent.mkdir(parents=True, exist_ok=True)
    if source.is_symlink():
        target.symlink_to(os.readlink(source))
    elif source.is_file():
        shutil.copy2(source, target)
PY

prowl_spec="$root/fedora/packages/rpm/prowl.spec"
prowl_commit=
prowl_sha256=
while read -r directive macro value rest; do
  [[ $directive == %global ]] || continue
  case "$macro" in
    prowl_commit) prowl_commit=$value ;;
    prowl_sha256) prowl_sha256=$value ;;
  esac
done < "$prowl_spec"
[[ $prowl_commit =~ ^[0-9a-f]{40}$ ]] || die 'prowl.spec has no valid prowl_commit'
[[ $prowl_sha256 =~ ^[0-9a-f]{64}$ ]] || die 'prowl.spec has no valid prowl_sha256'
prowl_archive="$work/prowl-$prowl_commit.tar.gz"
curl --fail --location --retry 3 \
  "https://github.com/neur0map/prowl/archive/$prowl_commit.tar.gz" \
  --output "$prowl_archive"
printf '%s  %s\n' "$prowl_sha256" "$prowl_archive" | sha256sum --check --status \
  || die "prowl source checksum mismatch for $prowl_commit"
mkdir -p "$work/tree/ryoku-$pkgver/.rpm-prowl"
tar -xzf "$prowl_archive" -C "$work/tree/ryoku-$pkgver/.rpm-prowl"
prowl_source="$work/tree/ryoku-$pkgver/.rpm-prowl/prowl-$prowl_commit"
[[ -f $prowl_source/go.mod ]] || die "prowl source archive has no go.mod"
(
  cd "$prowl_source"
  GOTOOLCHAIN=local \
    GOPATH="$work/prowl-gopath" \
    GOMODCACHE="$work/prowl-gomodcache" \
    GOCACHE="$work/prowl-gocache" \
    GOFLAGS='-mod=mod -modcacherw' \
    go mod vendor
)

# Resolve Go modules into each source payload so RPM builds stay offline.
while IFS= read -r -d '' gomod; do
  (cd "${gomod%/go.mod}" && GOTOOLCHAIN=local go mod vendor)
done < <(find "$work/tree/ryoku-$pkgver/ryoku" -name vendor -prune -o -name go.mod -print0)

python3 - "$work/tree/ryoku-$pkgver" <<'PYEXTRAS'
import importlib.machinery
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
extra = importlib.machinery.SourceFileLoader(
    "extra", str(root / "fedora/system/extras/ryoku-install-extra")
).load_module()
cache = root / ".rpm-extras"
cache.mkdir()
for version, url, digest, relative in extra.RELEASES.values():
    del version, relative
    (cache / digest).write_bytes(extra.download(url, digest))
PYEXTRAS

cat > "$work/tree/ryoku-$pkgver/.rpm-release" <<META
RELEASE=$release
NAME=$name
CHANNEL=$channel
VERSION=$pkgver
COMMIT=$commit
DATE=$(date -u -d "@$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ)
META

packages=()
for spec in "$root"/fedora/packages/rpm/*.spec; do
  packages+=("$(basename "$spec" .spec)")
done
while read -r path owners; do
  [[ -z $path || $path == \#* ]] && continue
  for owner in $owners; do
    [[ $owner == - || " ${packages[*]} " == *" $owner "* ]] \
      || die "source-owners names no spec: $owner"
  done
done < "$root/fedora/packages/rpm/source-owners"

source_tarball() {
  local package=$1 path owners archive="ryoku-$pkgver.tar.gz"
  local -a excludes=()
  while read -r path owners; do
    [[ -z $path || $path == \#* ]] && continue
    [[ " $owners " == *" $package "* ]] \
      || excludes+=("--exclude=ryoku-$pkgver/$path")
  done < "$root/fedora/packages/rpm/source-owners"
  [[ $package != prowl ]] || archive=ryoku-prowl-source.tar.gz
  mkdir -p "$work/SOURCES/$package"
  tar --anchored "${excludes[@]}" --sort=name --mtime="@$SOURCE_DATE_EPOCH" \
    --owner=0 --group=0 --numeric-owner -C "$work/tree" -cf - "ryoku-$pkgver" \
    | gzip -n > "$work/SOURCES/$package/$archive"
}

for package in "${packages[@]}"; do
  source_tarball "$package"
  target="$work/SPECS/$package.spec"
  if [[ $package == prowl ]]; then
    sed -e "s/^Release:.*/Release:        1%{?dist}/" \
      "$root/fedora/packages/rpm/$package.spec" > "$target"
  else
    sed -e "s/^Version:.*/Version:        $pkgver/" \
      -e "s/^Release:.*/Release:        1%{?dist}/" \
      "$root/fedora/packages/rpm/$package.spec" > "$target"
  fi
  rpmbuild --define "_topdir $work" \
    --define "_sourcedir $work/SOURCES/$package" \
    --define "_srcrpmdir $out" -bs "$target"
done

python3 - "$out/release.json" "$release" "$name" "$channel" "$pkgver" \
  "$commit" "$SOURCE_DATE_EPOCH" <<'PYMETA'
import datetime
import json
import pathlib
import sys

output = pathlib.Path(sys.argv[1])
timestamp = datetime.datetime.fromtimestamp(
    int(sys.argv[7]), datetime.timezone.utc
).strftime("%Y-%m-%dT%H:%M:%SZ")
data = {
    "schema": 1,
    "release": sys.argv[2],
    "name": sys.argv[3],
    "channel": sys.argv[4],
    "version": sys.argv[5],
    "commit": sys.argv[6],
    "date": timestamp,
}
output.write_text(json.dumps(data, separators=(",", ":")) + "\n")
PYMETA

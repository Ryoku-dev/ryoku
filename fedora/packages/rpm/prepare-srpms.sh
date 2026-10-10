#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/../../.." && pwd)
out=$(realpath -m "${RYOKU_SRPM_OUT:-$root/fedora/packages/rpm/srpms}")
channel=${RYOKU_CHANNEL:-testing}
die() { echo "prepare-srpms: $*" >&2; exit 1; }
for tool in rpmbuild python3 tar gzip go git patch; do command -v "$tool" >/dev/null || die "missing $tool"; done
git config --global --add safe.directory "$root" 2>/dev/null || true
git config --global --add safe.directory '*' 2>/dev/null || true
[[ ${RYOKU_RPM_REVISION:-1} =~ ^[1-9][0-9]*$ ]] || die 'RPM revision must be a positive integer'
if [[ ${RYOKU_RPM_LOCAL:-0} != 1 ]]; then
  [[ -z $(git -C "$root" status --porcelain) ]] || die 'release sources require a clean checkout; use RYOKU_RPM_LOCAL=1 for local tests'
fi
[[ $(git -C "$root" rev-parse --is-shallow-repository) == false ]] || die 'fetch full history for monotonic versions'
version="0.$(git -C "$root" rev-list --count HEAD)"
commit=$(git -C "$root" rev-parse HEAD)
SOURCE_DATE_EPOCH=$(git -C "$root" show -s --format=%ct HEAD)
export SOURCE_DATE_EPOCH
[[ ! -e $out ]] || die "output already exists: $out"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/SOURCES" "$work/SPECS" "$work/tree/ryoku-$version" "$out"
# Include current edits for local builds, while excluding ignored build products.
python3 - "$root" "$work/tree/ryoku-$version" <<'PY'
import os, pathlib, shutil, subprocess, sys
root, dest = map(pathlib.Path, sys.argv[1:])
paths = subprocess.check_output(['git', '-C', str(root), 'ls-files', '-z', '--cached', '--others', '--exclude-standard']).split(b'\0')
for raw in set(paths):
    if not raw: continue
    rel = pathlib.Path(os.fsdecode(raw))
    src, dst = root / rel, dest / rel
    if not src.exists() and not src.is_symlink(): continue
    dst.parent.mkdir(parents=True, exist_ok=True)
    if src.is_symlink(): dst.symlink_to(os.readlink(src))
    elif src.is_file(): shutil.copy2(src, dst)
PY
# Apply Fedora's host package-manager port only inside the RPM source tree.
patch --directory="$work/tree/ryoku-$version" --strip=1 --input="$root/fedora/packages/rpm/patches/0001-host-dnf-support.patch"
# Resolve pinned Go dependencies into the source payload. RPM builds then run
# offline, including modules whose development checkout has no vendor tree.
while IFS= read -r -d '' gomod; do
  (cd "${gomod%/go.mod}" && GOTOOLCHAIN=local go mod vendor)
done < <(find "$work/tree/ryoku-$version/ryoku" -name vendor -prune -o -name go.mod -print0)
python3 - "$work/tree/ryoku-$version" <<'PYEXTRAS'
import importlib.machinery, pathlib, sys
root = pathlib.Path(sys.argv[1])
extra = importlib.machinery.SourceFileLoader('extra', str(root / 'fedora/system/extras/ryoku-install-extra')).load_module()
cache = root / '.rpm-extras'
cache.mkdir()
for version, url, digest, relative in extra.RELEASES.values():
    (cache / digest).write_bytes(extra.download(url, digest))
PYEXTRAS
release=${RYOKU_RELEASE:-$(git -C "$root" describe --tags --match 'v[0-9]*' HEAD 2>/dev/null || echo "local-$version")}
cat > "$work/tree/ryoku-$version/.rpm-release" <<META
RELEASE=$release
NAME=$(cat "$root/CODENAME")
CHANNEL=$channel
VERSION=$version
COMMIT=$commit
DATE=$(date -u -d "@$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ)
META
packages=()
for spec in "$root"/fedora/packages/rpm/*.spec; do packages+=("$(basename "$spec" .spec)"); done
while read -r path owners; do
  [[ -z $path || $path == \#* ]] && continue
  for owner in $owners; do
    [[ $owner == - || " ${packages[*]} " == *" $owner "* ]] || die "source-owners names no spec: $owner"
  done
done < "$root/fedora/packages/rpm/source-owners"
# Each SRPM carries the tree without the heavy paths only other packages stage,
# so the build artifacts do not move the wallpapers twelve times.
source_tarball() {
  local package=$1 path owners excludes=()
  while read -r path owners; do
    [[ -z $path || $path == \#* ]] && continue
    [[ " $owners " == *" $package "* ]] || excludes+=("--exclude=ryoku-$version/$path")
  done < "$root/fedora/packages/rpm/source-owners"
  mkdir -p "$work/SOURCES/$package"
  tar --anchored "${excludes[@]}" --sort=name --mtime="@$SOURCE_DATE_EPOCH" --owner=0 --group=0 --numeric-owner \
    -C "$work/tree" -cf - "ryoku-$version" | gzip -n > "$work/SOURCES/$package/ryoku-$version.tar.gz"
}
for package in "${packages[@]}"; do
  source_tarball "$package"
  target="$work/SPECS/$package.spec"
  sed -e "s/^Version:.*/Version:        $version/" -e "s/^Release:.*/Release:        ${RYOKU_RPM_REVISION:-1}%{?dist}/" \
    "$root/fedora/packages/rpm/$package.spec" > "$target"
  rpmbuild --define "_topdir $work" --define "_sourcedir $work/SOURCES/$package" --define "_srcrpmdir $out" -bs "$target"
done
python3 - "$out" "$release" "$channel" "$version" "$commit" "$SOURCE_DATE_EPOCH" "$(cat "$root/CODENAME")" <<'PYMETA'
import datetime, hashlib, json, os, pathlib, sys
out = pathlib.Path(sys.argv[1])
data = dict(zip(('release', 'channel', 'version', 'commit'), sys.argv[2:6]))
data['date'] = datetime.datetime.fromtimestamp(int(sys.argv[6]), datetime.timezone.utc).isoformat()
data['name'] = sys.argv[7]
data['sequence'] = int(os.environ.get('GITHUB_RUN_NUMBER', '0'))
data['sources'] = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(out.glob('*.src.rpm'))}
(out / 'release.json').write_text(json.dumps(data, indent=2) + '\n')
PYMETA

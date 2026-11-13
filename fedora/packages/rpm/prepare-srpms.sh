#!/usr/bin/env bash
set -Eeuo pipefail

root=$(cd "$(dirname "$0")/../../.." && pwd)
out=$(realpath -m "${RYOKU_SRPM_OUT:-$root/fedora/packages/rpm/srpms}")
local_build=${RYOKU_RPM_LOCAL:-0}
rpm_revision=${RYOKU_RPM_REVISION:-1}
die() { printf 'prepare-srpms: %s\n' "$*" >&2; exit 1; }
current_package=setup
on_error() {
  local status=$?
  trap - ERR
  printf 'prepare-srpms: command failed while preparing %s at line %s: %s\n' \
    "$current_package" "${BASH_LINENO[0]:-unknown}" "$BASH_COMMAND" >&2
  exit "$status"
}
trap on_error ERR
for tool in rpmbuild rpmspec python3 tar gzip go git curl sha256sum find; do
  command -v "$tool" >/dev/null || die "missing $tool"
done
[[ $local_build == 0 || $local_build == 1 ]] || die 'RYOKU_RPM_LOCAL must be 0 or 1'
[[ $rpm_revision =~ ^[0-9][A-Za-z0-9._+~]*$ ]] \
  || die "invalid RYOKU_RPM_REVISION '$rpm_revision'"

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
[[ $pkgver =~ ^[0-9][A-Za-z0-9.]*$ ]] \
  || die "invalid RPM package version '$pkgver': expected a leading digit followed only by letters, digits, or dots"
[[ $("${git_cmd[@]}" rev-parse --is-shallow-repository) == false ]] \
  || die 'fetch full history before preparing release sources'

commit=${RYOKU_COMMIT:-$("${git_cmd[@]}" rev-parse HEAD)}
SOURCE_DATE_EPOCH=$("${git_cmd[@]}" show -s --format=%ct HEAD)
name=${RYOKU_NAME:-$(tr -d '[:space:]' < "$root/CODENAME")}
export SOURCE_DATE_EPOCH

[[ ! -e $out ]] || die "output already exists: $out"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
tree="$work/tree/ryoku-$pkgver"
mkdir -p "$work/SOURCES" "$work/SPECS" "$tree" "$out"
upstream_manifest="$out/upstream-rpms.tsv"
: > "$upstream_manifest"

# Release archives contain indexed files only. Local builds also include new,
# non-ignored files so a package can be proved before it is committed.
python3 - "$root" "$tree" "$local_build" <<'PY'
import os
import pathlib
import shutil
import subprocess
import sys

root, dest = map(pathlib.Path, sys.argv[1:3])
commands = [["ls-files", "-z", "--cached"]]
if sys.argv[3] == "1":
    commands.append(["ls-files", "-z", "--others", "--exclude-standard"])
paths = []
for arguments in commands:
    paths.extend(subprocess.check_output([
        "git", "-c", f"safe.directory={root}", "-C", str(root), *arguments,
    ]).split(b"\0"))
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

tree_go_vendored=0
vendor_ryoku_go_modules() {
  ((tree_go_vendored == 0)) || return 0
  while IFS= read -r -d '' gomod; do
    (cd "${gomod%/go.mod}" && GOTOOLCHAIN=local go mod vendor)
  done < <(find "$tree/ryoku" -name vendor -prune -o -name go.mod -print0)
  tree_go_vendored=1
  return 0
}

python3 - "$tree" <<'PYEXTRAS'
import importlib.machinery
import importlib.util
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
helper = root / "fedora/system/extras/ryoku-install-extra"
loader = importlib.machinery.SourceFileLoader("extra", str(helper))
spec = importlib.util.spec_from_file_location("extra", helper, loader=loader)
if spec is None or spec.loader is None:
    raise ImportError(f"could not load {helper}")
extra = importlib.util.module_from_spec(spec)
spec.loader.exec_module(extra)
cache = root / ".rpm-extras"
cache.mkdir()
for version, url, digest, relative in extra.RELEASES.values():
    del version, relative
    if url:  # vendored entries (geologica) read from the checkout, not a download
        (cache / digest).write_bytes(extra.download(url, digest))
PYEXTRAS

cat > "$tree/.rpm-release" <<META
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

rewrite_spec() {
  local source=$1 target=$2 version=$3 release_value=${4:-}
  python3 - "$source" "$target" "$version" "$release_value" <<'PY'
import pathlib
import re
import sys

source, target = map(pathlib.Path, sys.argv[1:3])
version, release = sys.argv[3:5]
text = source.read_text()
text, versions = re.subn(
    r"(?m)^Version:\s*.*$", f"Version:        {version}", text, count=1
)
if versions != 1:
    raise SystemExit(f"{source}: expected exactly one Version field")
if release:
    text, releases = re.subn(
        r"(?m)^Release:\s*.*$", f"Release:        {release}%{{?dist}}", text, count=1
    )
    if releases != 1:
        raise SystemExit(f"{source}: expected exactly one Release field")
target.write_text(text)
PY
}

source_tarball() {
  local package=$1 path owners
  local -a excludes=()
  while read -r path owners; do
    [[ -z $path || $path == \#* ]] && continue
    [[ " $owners " == *" $package "* ]] \
      || excludes+=("--exclude=ryoku-$pkgver/$path")
  done < "$root/fedora/packages/rpm/source-owners"
  tar --anchored "${excludes[@]}" --sort=name --mtime="@$SOURCE_DATE_EPOCH" \
    --owner=0 --group=0 --numeric-owner -C "$work/tree" -cf - "ryoku-$pkgver" \
    | gzip -n > "$work/SOURCES/$package/ryoku-$pkgver.tar.gz"
}

read_vendor_settings() {
  local spec=$1 directive macro value rest
  vendor_kind=
  vendor_dir=
  while read -r directive macro value rest; do
    [[ $directive == %global ]] || continue
    case "$macro" in
      ryoku_vendor) vendor_kind=$value ;;
      ryoku_vendor_dir) vendor_dir=$value ;;
    esac
  done < "$spec"
  [[ -z $vendor_kind || $vendor_kind == cargo || $vendor_kind == go ]] \
    || die "$(basename "$spec"): ryoku_vendor must be cargo or go"
  [[ $vendor_dir != /* && $vendor_dir != *..* ]] \
    || die "$(basename "$spec"): unsafe ryoku_vendor_dir '$vendor_dir'"
}

validate_expanded_spec() {
  local package=$1 spec=$2 expanded line lower
  local dependency_pattern='^(BuildRequires|Requires|Recommends|Suggests|Supplements|Enhances|Conflicts|Obsoletes)(\([^)]*\))?:'
  expanded=$(rpmspec -P "$spec") || die "$package: could not expand spec"
  while IFS= read -r line; do
    [[ $line =~ $dependency_pattern ]] || continue
    lower=${line,,}
    if [[ $lower == *"is not installed"* || $lower == *"not found"* ]]; then
      die "$package: expanded dependency is invalid: $line"
    fi
  done <<< "$expanded"
  return 0
}

record_upstream_rpms() {
  local package=$1 spec=$2 source_dir=$3 source_package input_digest
  source_package=$(rpmspec -q --srpm --qf '%{NAME}' "$spec") \
    || die "$package: could not resolve source RPM identity"
  [[ $source_package != *$'\n'* && -n $source_package ]] \
    || die "$package: invalid source RPM identity '$source_package'"
  input_digest=$(python3 - "$spec" "$source_dir" <<'PY'
import hashlib
import pathlib
import subprocess
import sys

spec = pathlib.Path(sys.argv[1])
source_dir = pathlib.Path(sys.argv[2])
expanded = subprocess.check_output(["rpmspec", "-P", str(spec)])
source_output = subprocess.check_output([
    "rpmspec", "-q", "--srpm", "--qf", "[%{SOURCE}\\n]", str(spec)
])
sources = sorted(filter(None, source_output.decode().splitlines()))
if len(sources) != len(set(sources)):
    raise SystemExit(f"{spec}: duplicate source filename")

digest = hashlib.sha256()
digest.update(b"expanded-spec\\0")
digest.update(expanded)
for name in sources:
    if pathlib.Path(name).name != name:
        raise SystemExit(f"{spec}: unsafe source filename {name!r}")
    source = source_dir / name
    if not source.is_file():
        raise SystemExit(f"{spec}: missing source file {name}")
    digest.update(b"source\\0")
    digest.update(name.encode())
    digest.update(b"\\0")
    with source.open("rb") as stream:
        while chunk := stream.read(1024 * 1024):
            digest.update(chunk)
print(digest.hexdigest())
PY
  ) || die "$package: could not digest upstream inputs"
  [[ $input_digest =~ ^[0-9a-f]{64}$ ]] \
    || die "$package: invalid upstream input digest '$input_digest'"
  rpmspec -q \
    --qf "$source_package\t%{NAME}\t%{EPOCHNUM}\t%{VERSION}\t%{RELEASE}\t%{ARCH}\t$input_digest\n" \
    "$spec" >> "$upstream_manifest" \
    || die "$package: could not resolve expected binary RPM identities"
}

prepare_spec_sources() {
  local package=$1 spec=$2 source_dir=$3
  local manifest="$root/fedora/packages/rpm/sources/$package"
  local expanded line number value local_name download_url actual expected
  local source_zero='' expected_vendor=''
  local checksum_pattern='^SHA256[[:space:]]+\(([^)]+)\)[[:space:]]*=[[:space:]]*([0-9a-f]{64})$'
  local -a identity=() unpacked=()
  declare -A checksums=() seen_checksums=() source_names=()

  if [[ -f $manifest ]]; then
    while IFS= read -r line || [[ -n $line ]]; do
      [[ -z $line || $line == \#* ]] && continue
      [[ $line =~ $checksum_pattern ]] \
        || die "$manifest: invalid checksum line: $line"
      local_name=${BASH_REMATCH[1]}
      [[ $local_name == "${local_name##*/}" && $local_name != . && $local_name != .. ]] \
        || die "$manifest: unsafe source name '$local_name'"
      [[ -z ${checksums[$local_name]+x} ]] \
        || die "$manifest: duplicate checksum for $local_name"
      checksums[$local_name]=${BASH_REMATCH[2]}
    done < "$manifest"
  fi

  read_vendor_settings "$spec"
  expanded=$(rpmspec -P "$spec") || die "$package: could not expand spec"
  if [[ -n $vendor_kind ]]; then
    command -v "$vendor_kind" >/dev/null || die "$package: missing $vendor_kind for vendoring"
    mapfile -t identity < <(rpmspec -q --srpm --qf '%{NAME}\n%{VERSION}\n' "$spec")
    ((${#identity[@]} >= 2)) || die "$package: could not read Name and Version"
    expected_vendor="${identity[0]}-${identity[1]}-vendor.tar.gz"
  fi

  while IFS= read -r line; do
    [[ $line =~ ^Source([0-9]*):[[:space:]]*(.+)$ ]] || continue
    number=${BASH_REMATCH[1]:-0}
    value=${BASH_REMATCH[2]}
    value=${value%"${value##*[![:space:]]}"}
    if [[ $value == https://* ]]; then
      if [[ $value == *#/* ]]; then
        local_name=${value##*#/}
        download_url=${value%%#/*}
      else
        download_url=${value%%#*}
        local_name=${download_url%%\?*}
        local_name=${local_name##*/}
      fi
      [[ $local_name == "${local_name##*/}" && -n $local_name && $local_name != . && $local_name != .. ]] \
        || die "$package: unsafe local name in Source$number: $value"
      [[ -z ${source_names[$local_name]+x} ]] \
        || die "$package: duplicate source filename $local_name"
      source_names[$local_name]=1
      expected=${checksums[$local_name]:-}
      [[ -n $expected ]] \
        || die "$package: no verified checksum for URL source $local_name in $manifest"
      curl --fail --location --retry 6 --retry-delay 10 --retry-max-time 180 --proto '=https' \
        "$download_url" --output "$source_dir/$local_name.part"
      actual=$(sha256sum "$source_dir/$local_name.part")
      actual=${actual%% *}
      [[ $actual == "$expected" ]] \
        || die "$package: checksum mismatch for $local_name (expected $expected, got $actual)"
      mv "$source_dir/$local_name.part" "$source_dir/$local_name"
      seen_checksums[$local_name]=1
    elif [[ $value == *://* ]]; then
      die "$package: Source$number must use HTTPS: $value"
    else
      local_name=${value##*/}
      [[ $value == "$local_name" && -n $local_name && $local_name != . && $local_name != .. ]] \
        || die "$package: Source$number must be a plain filename: $value"
      if [[ $number == 100 && -n $vendor_kind ]]; then
        [[ $local_name == "$expected_vendor" ]] \
          || die "$package: Source100 must be $expected_vendor"
      else
        [[ -f $source_dir/$local_name ]] \
          || die "$package: fetch hook did not produce Source$number $local_name"
      fi
    fi
    [[ $number != 0 ]] || source_zero="$source_dir/$local_name"
  done <<< "$expanded"

  [[ -n $source_zero ]] || die "$package: spec has no Source0"
  for local_name in "${!checksums[@]}"; do
    [[ -n ${seen_checksums[$local_name]+x} ]] \
      || die "$manifest: checksum has no matching URL source: $local_name"
  done

  if [[ -n $vendor_kind ]]; then
    [[ -f $source_zero ]] || die "$package: Source0 is missing for vendoring"
    local vendor_work="$work/vendor/$package" source_root module_root
    mkdir -p "$vendor_work/unpack"
    tar -xf "$source_zero" -C "$vendor_work/unpack" \
      || die "$package: could not unpack Source0 for vendoring"
    mapfile -d '' -t unpacked < <(find "$vendor_work/unpack" -mindepth 1 -maxdepth 1 -print0)
    ((${#unpacked[@]})) || die "$package: Source0 is empty"
    if ((${#unpacked[@]} == 1)) && [[ -d ${unpacked[0]} ]]; then
      source_root=${unpacked[0]}
    else
      source_root="$vendor_work/unpack"
    fi
    module_root=$source_root
    [[ -z $vendor_dir ]] || module_root="$source_root/$vendor_dir"
    [[ -d $module_root ]] || die "$package: vendoring directory does not exist: $vendor_dir"
    rm -rf "$module_root/vendor"
    if [[ $vendor_kind == cargo ]]; then
      [[ -f $module_root/Cargo.lock ]] || die "$package: cargo vendoring requires Cargo.lock"
      (
        cd "$module_root"
        CARGO_HOME="$vendor_work/cargo-home" cargo vendor --locked vendor
      )
    else
      [[ -f $module_root/go.mod && -f $module_root/go.sum ]] \
        || die "$package: Go vendoring requires go.mod and go.sum"
      (
        cd "$module_root"
        GOTOOLCHAIN=local \
          GOPATH="$vendor_work/gopath" \
          GOMODCACHE="$vendor_work/gomodcache" \
          GOCACHE="$vendor_work/gocache" \
          GOFLAGS='-mod=mod -modcacherw' \
          go mod vendor
      )
    fi
    [[ -d $module_root/vendor ]] || die "$package: vendoring produced no vendor directory"
    tar --sort=name --format=posix --mtime=@0 --owner=0 --group=0 \
      --numeric-owner \
      --pax-option='exthdr.name=%d/PaxHeaders/%f,delete=atime,delete=ctime' \
      -C "$module_root" -cf - vendor \
      | gzip -n > "$source_dir/$expected_vendor"
    rm -rf "$vendor_work"
  fi
  return 0
}

for package in "${packages[@]}"; do
  current_package=$package
  source_spec="$root/fedora/packages/rpm/$package.spec"
  target_spec="$work/SPECS/$package.spec"
  source_dir="$work/SOURCES/$package"
  hook="$root/fedora/packages/rpm/fetch/$package.sh"
  mkdir -p "$source_dir"

  if [[ -e $hook ]]; then
    [[ -x $hook ]] || die "$hook is not executable"
    RYOKU_FETCH_TREE="$tree" RYOKU_FETCH_SOURCES="$source_dir" "$hook" \
      || die "$package: fetch hook failed: $hook"
  fi

  if grep -Eq '^Source0:[[:space:]]+ryoku-%\{version\}\.tar\.gz[[:space:]]*$' "$source_spec"; then
    [[ ! -e $source_dir/.version ]] \
      || die "$package: .version is only valid for upstream specs"
    rewrite_spec "$source_spec" "$target_spec" "$pkgver" "$rpm_revision"
    validate_expanded_spec "$package" "$target_spec"
    vendor_ryoku_go_modules
    source_tarball "$package"
    prepare_spec_sources "$package" "$target_spec" "$source_dir"
  else
    upstream_version=
    if [[ -e $source_dir/.version ]]; then
      mapfile -t version_lines < "$source_dir/.version"
      ((${#version_lines[@]} == 1)) \
        || die "$package: fetch hook .version must contain exactly one line"
      upstream_version=${version_lines[0]}
      [[ $upstream_version =~ ^[0-9][A-Za-z0-9._+~]*$ ]] \
        || die "$package: fetch hook wrote invalid RPM Version '$upstream_version'"
      rm "$source_dir/.version"
    fi
    if [[ -n $upstream_version ]]; then
      rewrite_spec "$source_spec" "$target_spec" "$upstream_version"
    else
      cp "$source_spec" "$target_spec"
    fi
    validate_expanded_spec "$package" "$target_spec"
    prepare_spec_sources "$package" "$target_spec" "$source_dir"
    record_upstream_rpms "$package" "$target_spec" "$source_dir"
  fi

  rpmbuild --define "_topdir $work" \
    --define "_sourcedir $source_dir" \
    --define "_srcrpmdir $out" -bs "$target_spec" \
    || die "$package: failed to write source RPM"
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

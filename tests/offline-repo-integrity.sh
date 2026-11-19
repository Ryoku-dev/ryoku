#!/usr/bin/env bash
# offline-repo.sh must verify every cached package is a complete, readable archive
# before baking it into the [offline] repo. `pacman -Sw --needed` never re-fetches
# a file that already exists by name, and the cache is persistent, so a download
# truncated by one network hiccup sat corrupt in the cache and shipped in every
# ISO -- the offline pacstrap then died with a "truncated <pkg>" error, most
# likely on a large package. See the
# integrity loop in installation/iso/offline-repo.sh.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$here/.."
repo="$root/installation/iso/offline-repo.sh"
fail() { echo "FAIL: $1" >&2; exit 1; }

[[ -f $repo ]] || fail "offline-repo.sh not found"

# 1. the bake verifies package integrity with bsdtar, and does it BEFORE it builds
#    the db -- otherwise a truncated package would already be indexed and shipped.
grep -qE 'bsdtar -tf' "$repo" || fail "offline-repo.sh no longer verifies package integrity with bsdtar -tf"
vline=$(grep -nE 'bsdtar -tf' "$repo" | head -n1 | cut -d: -f1)
rline=$(grep -nE '^[[:space:]]*repo-add --quiet' "$repo" | head -n1 | cut -d: -f1)
[[ -n $vline && -n $rline ]] || fail "could not locate the integrity check and repo-add"
(( vline < rline )) || fail "integrity check (line $vline) runs after repo-add (line $rline); a truncated package would already be indexed"
# 2. a still-truncated package fails the build, it is never silently shipped.
grep -qE 'die "cache integrity' "$repo" || fail "offline-repo.sh must fail the build on a still-truncated package"

# 3. bsdtar -tf actually detects a truncated zstd package on this host (proves the
#    detection primitive the fix relies on). Skipped if bsdtar lacks zstd support.
command -v bsdtar >/dev/null 2>&1 || { echo "offline-repo-integrity: bsdtar absent; static checks OK"; exit 0; }
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
printf 'hello\n' >"$tmp/file"
if ! bsdtar -caf "$tmp/good.tar.zst" -C "$tmp" file 2>/dev/null; then
  echo "offline-repo-integrity: bsdtar has no zstd support here; static checks OK"
  exit 0
fi
bsdtar -tf "$tmp/good.tar.zst" >/dev/null 2>&1 || fail "a valid package failed the integrity check"
head -c 10 "$tmp/good.tar.zst" >"$tmp/trunc.tar.zst"
if bsdtar -tf "$tmp/trunc.tar.zst" >/dev/null 2>&1; then
  fail "a truncated package passed the integrity check"
fi

# 4. every package the per-vendor driver scripts install must be in the bake. The
#    bake carries its own literal list, so a package added to a driver script
#    otherwise only fails on a real offline install, with no network to recover
#    from. Kernel-derived names (${kb}-headers) come from the package lists and
#    are checked there, not here.
drivers="$root/system/hardware/drivers"
bake=$(cat "$repo")
missing=()
while read -r pkg; do
  [[ $pkg == *'$'* || $pkg == *'"'* || $pkg == '' ]] && continue
  grep -qF -- "$pkg" <<<"$bake" || missing+=("$pkg")
done < <(
  grep -hoE 'install_pkgs [a-z0-9 ._+-]+' "$drivers"/*.sh | sed 's/^install_pkgs //' | tr ' ' '\n'
  grep -hoE '(pkgs|base)\+?=\([^)]*\)' "$drivers"/*.sh | tr '()' '  ' | sed -E 's/(pkgs|base)\+?=//' | tr ' ' '\n'
)
(( ${#missing[@]} == 0 )) \
  || fail "driver packages missing from the offline bake (installation/iso/offline-repo.sh): ${missing[*]}"
# 5. both compositor variants must be in the bake. The TUI offers every
#    compositor it knows, and the offline desktop-set install (deploy.sh) names
#    ryoku-desktop-$RYOKU_COMPOSITOR against the baked repo only: a variant that
#    was never baked dies at "configure" with "target not found" and no network
#    to recover with (issue #260: the ISO shipped hyprland only).
for variant in ryoku-desktop-hyprland ryoku-desktop-niri; do
  grep -qF -- "$variant" <<<"$bake" \
    || fail "compositor variant '$variant' is not baked into the offline repo"
done
# 6. every package the offline desktop-set install can name must be in the bake's
#    completeness check, not only in its download list: the download staged
#    ryotunes out of the repo unnoticed and the ISO died at "configure". The
#    fresh-install apps come from deploy.sh; the file managers from choices.tsv.
required=$(sed -n '/^for req in/,/; do$/p' "$repo")
deploy="$root/installation/backend/lib/deploy.sh"
apps=$(sed -n 's/^[[:space:]]*for package in \(.*\); do$/\1/p' "$deploy" | grep -v '\$' | head -n1)
[[ -n $apps ]] || fail "could not find the fresh-install app loop in deploy.sh"
for pkg in $apps; do
  grep -qwF -- "$pkg" <<<"$required" \
    || fail "deploy.sh installs '$pkg' with the desktop set, but the offline bake never checks it is in the repo"
done
# shellcheck disable=SC2016
grep -qF '"${file_managers[@]}"' <<<"$required" \
  || fail "the offered file managers are not in the offline bake's completeness check"
# 7. a package whose file names its version without the epoch (ryotunes) must
#    still be staged: the RESOLVED keys carry the epochless version too.
# shellcheck disable=SC2016
grep -qF 'RESOLVED["$name ${ver#*:}"]=1' "$repo" \
  || fail "offline-repo.sh no longer matches epochless package filenames; ryotunes would be dropped from the repo"

echo "offline-repo-integrity: OK"

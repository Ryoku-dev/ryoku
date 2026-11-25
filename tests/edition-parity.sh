#!/usr/bin/env bash
set -euo pipefail

root=${RYOKU_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
gate="$root/bin/ryoku-edition-parity"
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT

fail() {
	printf 'edition-parity-test: %s\n' "$*" >&2
	exit 1
}

mkdir -p \
	"$fixture/release/packages/ryoku-desktop" \
	"$fixture/system/packages" \
	"$fixture/void/packages/srcpkgs/ryoku-desktop" \
	"$fixture/void/packages/srcpkgs/split-built" \
	"$fixture/fedora/packages/rpm"

cat > "$fixture/release/packages/ryoku-desktop/PKGBUILD" <<'PKGBUILD'
pkgname=('ryoku-desktop' 'split-built')
pkgver=1
depends=('versioned=1')
optdepends=()
PKGBUILD
printf 'base-package\n' > "$fixture/system/packages/base.packages"
printf '[test]\nhardware-package\n' > "$fixture/system/packages/hardware.packages"
printf 'pkgname=ryoku-desktop\n' > "$fixture/void/packages/srcpkgs/ryoku-desktop/template"
printf 'pkgname=split-built\n' > "$fixture/void/packages/srcpkgs/split-built/template"
printf 'Name: ryoku-desktop\n' > "$fixture/fedora/packages/rpm/ryoku-desktop.spec"
printf 'Name: split-built\n' > "$fixture/fedora/packages/rpm/split-built.spec"
printf 'arch\tvoid\tlanes\tnotes\nbase-package\tbase-package\tsystem\t\nhardware-package\thardware-package\thardware:test\t\n' \
	> "$fixture/void/packages/translations.tsv"
printf 'arch\tfedora\tlanes\tnotes\nbase-package\tbase-package\tsystem\t\nhardware-package\thardware-package\thardware:test\t\n' \
	> "$fixture/fedora/packages/translations.tsv"

run_failure() {
	if RYOKU_PATH="$fixture" "$gate" > "$fixture/output" 2>&1; then
		fail "$1"
	fi
}

run_success() {
	if ! RYOKU_PATH="$fixture" "$gate" > "$fixture/output" 2>&1; then
		cat "$fixture/output" >&2
		fail "$1"
	fi
}

run_failure 'a versioned dependency without a base-name row passed'
grep -q '^  versioned$' "$fixture/output" || fail 'versioned dependency did not map to its base name'
! grep -q 'versioned=1' "$fixture/output" || fail 'version constraint leaked into the missing package name'
printf 'versioned\tversioned\tdesktop\t\n' >> "$fixture/void/packages/translations.tsv"
printf 'versioned\tversioned\tdesktop\t\n' >> "$fixture/fedora/packages/translations.tsv"
run_success 'built split packages or a mapped versioned dependency failed'

rm "$fixture/fedora/packages/rpm/split-built.spec"
run_failure 'a split PKGBUILD package without a build artifact passed'
[[ $(grep -c '^  split-built$' "$fixture/output") -eq 1 ]] ||
	fail 'the missing Fedora split package was not reported'
printf 'Name: split-built\n' > "$fixture/fedora/packages/rpm/split-built.spec"
run_success 'restoring the built split package did not restore parity'

mkdir -p "$fixture/release/packages/new-arch-package"
printf 'pkgname=new-arch-package\npkgver=1\n' > "$fixture/release/packages/new-arch-package/PKGBUILD"
run_failure 'a new Arch package with no edition row passed'
[[ $(grep -c '^  new-arch-package$' "$fixture/output") -eq 2 ]] ||
	fail 'both editions did not report the new Arch package'
rm -rf "$fixture/release/packages/new-arch-package"

printf 'unavailable\t-\tdesktop\t\n' >> "$fixture/void/packages/translations.tsv"
run_failure 'a reasonless exclusion row passed'
grep -q '^  unavailable (line ' "$fixture/output" ||
	fail 'reasonless exclusion was not identified'

printf 'edition-parity: fixture cases passed\n'

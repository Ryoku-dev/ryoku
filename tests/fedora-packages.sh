#!/usr/bin/env bash
# Package closure, Fedora translation, and RPM dependency checks. This test is offline.
set -euo pipefail

ROOT=${RYOKU_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
PACKAGES="$ROOT/fedora/packages"
TABLE="$PACKAGES/translations.tsv"
RESOLVE="$ROOT/system/packages/resolve"
SPECS="$PACKAGES/rpm"

fail() { echo "fedora-packages: $*" >&2; exit 1; }

(($# == 0)) || fail "usage: ${0##*/}"
[[ -f $TABLE ]] || fail "missing translation table: $TABLE"
[[ -x $RESOLVE ]] || fail "resolver is not executable: $RESOLVE"
[[ $(IFS= read -r line < "$RESOLVE"; printf '%s' "$line") == '#!/bin/sh' ]] \
	|| fail "resolver must use #!/bin/sh"

package_lines() {
	awk '{ sub(/#.*/, ""); gsub(/^[[:space:]]+|[[:space:]]+$/, ""); if (length) print }' "$1"
}

strip_dep() {
	local name=${1%%:*}
	name=${name%%[<>=]*}
	printf '%s\n' "$name"
}

declare -A pkgbuilds=()
while IFS=$'\t' read -r pkg path; do
	[[ -n $pkg ]] || continue
	pkgbuilds[$pkg]=$path
done < <(
	for path in "$ROOT"/release/packages/*/PKGBUILD; do
		bash -c '
			source "$1" >/dev/null 2>&1
			for pkg in "${pkgname[@]:-}"; do
				[[ -n $pkg ]] && printf "%s\t%s\n" "$pkg" "$1"
			done
		' _ "$path"
	done
)

pkgbuild_field() {
	local path=$1 field=$2
	bash -c '
		source "$1" >/dev/null 2>&1
		case $2 in
			depends) printf "%s\n" "${depends[@]:-}" ;;
			optdepends) printf "%s\n" "${optdepends[@]:-}" ;;
		esac
	' _ "$path" "$field"
}

# shellcheck disable=SC2034 # the closure is written through namerefs
walk_closure() {
	local root=$1 hard_name=$2 opt_name=$3 seen_name=$4
	local -n hard_ref=$hard_name opt_ref=$opt_name seen_ref=$seen_name
	local -a queue=("$root")
	local raw name path

	while ((${#queue[@]})); do
		raw=${queue[0]}
		queue=("${queue[@]:1}")
		name=$(strip_dep "$raw")
		[[ -n $name ]] || continue
		hard_ref[$name]=1
		path=${pkgbuilds[$name]:-}
		[[ -n $path && -z ${seen_ref[$name]:-} ]] || continue
		seen_ref[$name]=1
		while IFS= read -r raw; do
			name=$(strip_dep "$raw")
			[[ -n $name ]] && queue+=("$name")
		done < <(pkgbuild_field "$path" depends)
		while IFS= read -r raw; do
			name=$(strip_dep "$raw")
			[[ -n $name ]] && opt_ref[$name]=1
		done < <(pkgbuild_field "$path" optdepends)
	done
}

# shellcheck disable=SC2034 # filled by walk_closure through namerefs
declare -A neutral_hard=() neutral_opt=() neutral_seen=() \
	niri_hard=() niri_opt=() niri_seen=() \
	hypr_hard=() hypr_opt=() hypr_seen=() \
	wayf_hard=() wayf_opt=() wayf_seen=()
walk_closure ryoku-desktop neutral_hard neutral_opt neutral_seen
# Rashin is optional on Arch, but this edition's desktop package still
# requires it, so its own dependencies belong to the desktop closure here.
walk_closure ryoku-rashin neutral_hard neutral_opt neutral_seen
walk_closure ryoku-desktop-niri niri_hard niri_opt niri_seen
walk_closure ryoku-desktop-hyprland hypr_hard hypr_opt hypr_seen
walk_closure ryoku-desktop-wayfire wayf_hard wayf_opt wayf_seen

declare -A closure=() expected_lanes=()
lane_order=(desktop dev system extra optional hyprland wayfire hardware:amd hardware:intel hardware:nvidia hardware:vm)

add_lane() {
	local pkg=$1 lane=$2 current=${expected_lanes[$1]:-}
	closure[$pkg]=1
	[[ $current == *"|$lane|"* ]] || expected_lanes[$pkg]="$current|$lane|"
}

boot_chain=(base base-devel linux linux-firmware mkinitcpio sudo btrfs-progs cryptsetup dosfstools efibootmgr limine plymouth snapper snap-pac limine-mkinitcpio-hook limine-snapper-sync)
declare -A boot=()
for pkg in "${boot_chain[@]}"; do boot[$pkg]=1; done
while IFS= read -r pkg; do
	if [[ -n ${boot[$pkg]:-} ]]; then add_lane "$pkg" system; else add_lane "$pkg" desktop; fi
done < <(package_lines "$ROOT/system/packages/base.packages")
while IFS= read -r pkg; do [[ -n ${closure[$pkg]:-} ]] || add_lane "$pkg" optional; done < <(package_lines "$ROOT/system/packages/phone.packages")
while IFS= read -r pkg; do add_lane "$pkg" dev; done < <(package_lines "$ROOT/system/packages/dev.packages")
while IFS= read -r pkg; do add_lane "$pkg" extra; done < <(package_lines "$ROOT/system/packages/aur.packages")
while IFS=$'\t' read -r section pkg; do
	add_lane "$pkg" "hardware:$section"
done < <(awk '
	{
		sub(/#.*/, "")
		gsub(/^[[:space:]]+|[[:space:]]+$/, "")
		if (!length) next
		if ($0 ~ /^\[/) { section = substr($0, 2, length($0) - 2); next }
		print section "\t" $0
	}
' "$ROOT/system/packages/hardware.packages")

for pkg in "${!neutral_hard[@]}" "${!niri_hard[@]}"; do add_lane "$pkg" desktop; done
for pkg in "${!hypr_hard[@]}" "${!hypr_opt[@]}"; do
	[[ -z ${neutral_hard[$pkg]:-}${neutral_opt[$pkg]:-}${niri_hard[$pkg]:-}${niri_opt[$pkg]:-} ]] \
		&& add_lane "$pkg" hyprland
done
for pkg in "${!wayf_hard[@]}" "${!wayf_opt[@]}"; do
	[[ -z ${neutral_hard[$pkg]:-}${neutral_opt[$pkg]:-}${niri_hard[$pkg]:-}${niri_opt[$pkg]:-} ]] \
		&& add_lane "$pkg" wayfire
done
for pkg in "${!neutral_opt[@]}" "${!niri_opt[@]}" "${!hypr_opt[@]}" "${!wayf_opt[@]}"; do
	closure[$pkg]=1
	[[ -n ${expected_lanes[$pkg]:-} ]] || add_lane "$pkg" optional
done
add_lane bash desktop
add_lane ryoku-keyring desktop
add_lane ryoku-palette-bridge desktop
add_lane udev desktop
for pkg in dualsensectl dbus hidapi libusb; do add_lane "$pkg" optional; done
# ryoku-gpu-lib32 installs the 32-bit GPU baseline on demand for games.
for pkg in lib32-mesa lib32-vulkan-icd-loader lib32-vulkan-nouveau; do add_lane "$pkg" optional; done
# The radio player remains in Arch's base set so ISO pacstrap installs it, but
# Fedora owns it through RPM weak dependencies and the default-on radio
# component. It must not re-enter the Fedora desktop lane as a hard package.
for pkg in mpv mpv-mpris; do expected_lanes[$pkg]='|optional|'; done


canonical_lanes() {
	local encoded=$1 lane out=
	for lane in "${lane_order[@]}"; do
		if [[ $encoded == *"|$lane|"* ]]; then
			[[ -z $out ]] || out+=,
			out+=$lane
		fi
	done
	printf '%s\n' "$out"
}

awk -F '\t' -v table="$TABLE" '
function bad(reason) {
	printf "%s: line %d: %s\n", table, NR, reason > "/dev/stderr"
	failed = 1
}
function package_name(name) { return name ~ /^[A-Za-z0-9][A-Za-z0-9+_.-]*$/ }
NR == 1 {
	if ($0 != "arch\tfedora\tlanes\tnotes") bad("bad header")
	next
}
/^[[:space:]]*#/ || /^[[:space:]]*$/ { next }
{
	if (NF != 4) { bad("expected four tab-separated columns"); next }
	if (!package_name($1)) bad("invalid Arch package name")
	if (seen[$1]++) bad("duplicate Arch package " $1)
	count = split($3, lanes, ",")
	delete row_lane
	for (i = 1; i <= count; i++) {
		if (lanes[i] !~ /^(desktop|dev|system|extra|optional|hyprland|wayfire|hardware:(amd|intel|nvidia|vm))$/)
			bad("invalid lane " lanes[i])
		if (row_lane[lanes[i]]++) bad("duplicate lane " lanes[i])
	}
	if ($2 == "-") {
		if ($4 == "") bad("unavailable mapping requires notes")
	} else {
		if ($2 ~ /^@/) bad("Fedora mappings must name RPM packages or use -")
		count = split($2, names, /[[:space:]]+/)
		for (i = 1; i <= count; i++) if (!package_name(names[i])) bad("invalid Fedora package " names[i])
	}
}
END { exit failed ? 1 : 0 }
' "$TABLE" || fail "translation table is malformed"

declare -A rows=() row_fedora=() mapped_rpms=()
while IFS=$'\t' read -r arch fedora row_lanes _; do
	[[ $arch == arch || $arch == '#'* || -z $arch ]] && continue
	rows[$arch]=1
	row_fedora[$arch]=$fedora
	if [[ $fedora != - ]]; then
		for pkg in $fedora; do mapped_rpms[$pkg]=1; done
	fi
	[[ -n ${closure[$arch]:-} ]] || fail "translation lies outside the Arch closure: $arch"
	expected=$(canonical_lanes "${expected_lanes[$arch]}")
	[[ $row_lanes == "$expected" ]] || fail "$arch lanes are '$row_lanes', expected '$expected'"
done < "$TABLE"
for pkg in "${!closure[@]}"; do
	[[ -n ${rows[$pkg]:-} ]] || fail "Arch closure package has no Fedora row: $pkg"
done
((${#rows[@]} == ${#closure[@]})) || fail "translation row count does not match closure"

spec_field() {
	local path=$1 field=$2
	awk -v field="$field" '
		$1 == field ":" {
			name = $2
			sub(/%\{\?_isa\}$/, "", name)
			print name
		}
	' "$path"
}

hard_dependency_satisfied() {
	local arch_dep=$1 expected_dep=$2 file_requirement
	[[ -n ${hard[$expected_dep]:-}${provided[$expected_dep]:-} ]] && return 0
	case $arch_dep in
		ffmpeg)
			file_requirement=/usr/bin/ffmpeg
			[[ -n ${hard[$file_requirement]:-} ]]
			;;
		*) return 1 ;;
	esac
}

declare -A spec_names=() spec_paths=() spec_refs=()
spec_count=0
for spec in "$SPECS"/*.spec; do
	[[ -f $spec ]] || continue
	((spec_count += 1))
	mapfile -t names < <(spec_field "$spec" Name)
	((${#names[@]} == 1)) || fail "spec must declare exactly one Name: $spec"
	name=${names[0]}
	[[ ${spec##*/} == "$name.spec" ]] || fail "spec file ${spec##*/} declares Name: $name"
	[[ -z ${spec_names[$name]:-} ]] || fail "duplicate RPM spec name: $name"
	spec_names[$name]=1
	spec_paths[$name]=$spec
	while IFS= read -r dep; do
		[[ -n $dep ]] && spec_refs[$dep]=1
	done < <({ spec_field "$spec" Requires; spec_field "$spec" Recommends; spec_field "$spec" Suggests; })
done
((spec_count > 0)) || fail "no RPM specs found under $SPECS"

for pkg in "${!spec_names[@]}"; do
	[[ -n ${mapped_rpms[$pkg]:-}${spec_refs[$pkg]:-} ]] \
		|| fail "RPM spec is not referenced by the translation table or another RPM: $pkg"
done

for spec_pkg in "${!spec_names[@]}"; do
	pkgbuild=${pkgbuilds[$spec_pkg]:-}
	[[ -n $pkgbuild ]] || continue

	declare -A hard=() weak=() provided=()
	while IFS= read -r dep; do [[ -n $dep ]] && hard[$dep]=1; done < <(spec_field "${spec_paths[$spec_pkg]}" Requires)
	while IFS= read -r dep; do [[ -n $dep ]] && weak[$dep]=1; done < <({
		spec_field "${spec_paths[$spec_pkg]}" Recommends
		spec_field "${spec_paths[$spec_pkg]}" Suggests
	})
	while IFS= read -r dep; do [[ -n $dep ]] && provided[$dep]=1; done < <(spec_field "${spec_paths[$spec_pkg]}" Provides)

	while IFS= read -r raw; do
		[[ -n $raw ]] || continue
		arch_dep=$(strip_dep "$raw")
		[[ -n ${row_fedora[$arch_dep]+set} ]] \
			|| fail "$spec_pkg PKGBUILD dependency has no translation: $arch_dep"
		mapping=${row_fedora[$arch_dep]}
		[[ $mapping != - ]] || continue
		for expected_dep in $mapping; do
			hard_dependency_satisfied "$arch_dep" "$expected_dep" \
				|| fail "$spec_pkg spec misses translated dependency $expected_dep (from $arch_dep)"
		done
	done < <(pkgbuild_field "$pkgbuild" depends)

	while IFS= read -r raw; do
		[[ -n $raw ]] || continue
		arch_dep=$(strip_dep "$raw")
		[[ -n ${row_fedora[$arch_dep]+set} ]] \
			|| fail "$spec_pkg PKGBUILD optional dependency has no translation: $arch_dep"
		mapping=${row_fedora[$arch_dep]}
		[[ $mapping != - ]] || continue
		for expected_dep in $mapping; do
			[[ -n ${hard[$expected_dep]:-}${weak[$expected_dep]:-}${provided[$expected_dep]:-} ]] \
				|| fail "$spec_pkg spec misses translated optional dependency $expected_dep (from $arch_dep)"
		done
	done < <(pkgbuild_field "$pkgbuild" optdepends)
	unset hard weak provided
done

check_resolve_output() {
	local out line sorted
	out=$($RESOLVE --table "$TABLE" "$@") || fail "resolve failed for: $*"
	while IFS= read -r line; do
		[[ -z $line || $line =~ ^[A-Za-z0-9][A-Za-z0-9+_.-]*$ ]] \
			|| fail "resolve printed a non-package value: $line"
	done <<< "$out"
	if [[ -n $out ]]; then
		sorted=$(printf '%s\n' "$out" | LC_ALL=C sort -u)
		[[ $out == "$sorted" ]] || fail "resolve output is not sorted and unique: $*"
	fi
	printf '%s' "$out"
}

for lane in "${lane_order[@]}"; do check_resolve_output --lane "$lane" >/dev/null; done
check_resolve_output >/dev/null

desktop=$(check_resolve_output --lane desktop)
dev=$(check_resolve_output --lane dev)
both=$(check_resolve_output --lane desktop --lane dev)
union=$(printf '%s\n%s\n' "$desktop" "$dev" | awk 'NF' | LC_ALL=C sort -u)
[[ $both == "$union" ]] || fail "repeated --lane arguments do not resolve their union"
grep -qxF fish <<< "$desktop" || fail "desktop lane did not translate fish"
grep -qxF ryoku <<< "$desktop" || fail "desktop lane omitted the Ryoku CLI"
grep -qxF gpk <<< "$desktop" || fail "desktop lane omitted the packaged gpk frontend"
grep -qxF snapper <<< "$desktop" || fail "desktop lane omitted snapper"

dropped=$(check_resolve_output --lane desktop --drop fish --drop chromium)
! grep -qxF fish <<< "$dropped" || fail "repeated --drop left the fish translation"
! grep -qxF chromium <<< "$dropped" || fail "repeated --drop left the chromium package"

fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
printf 'arch\tfedora\tlanes\tnotes\nbroken\tonly-three\tdesktop\n' > "$fixture/bad.tsv"
if "$RESOLVE" --table "$fixture/bad.tsv" >"$fixture/out" 2>"$fixture/err"; then
	fail "resolve accepted a malformed table"
fi
grep -q 'line 2' "$fixture/err" || fail "malformed-table error did not name the bad line"

# The installer pins the key it fetches; the ryoku-keyring RPM must ship that key.
key="$ROOT/keys/ryoku-release-key.pub.asc"
grep -qF 'keys/ryoku-release-key.pub.asc' "$SPECS/ryoku-keyring.spec" \
	|| fail "ryoku-keyring does not ship $key"
pinned=$(sed -n 's/^[[:space:]]*fedoraKeyFingerprint[[:space:]]*=[[:space:]]*"\([0-9A-F]*\)"$/\1/p' "$ROOT/ryoku-shell-installer/engine.go")
[[ -n $pinned ]] || fail "installer key fingerprint not found"
shipped=$(GNUPGHOME="$fixture" gpg --batch --with-colons --show-keys "$key" 2>/dev/null | awk -F: '$1 == "fpr" { print $10; exit }')
[[ $shipped == "$pinned" ]] || fail "ryoku-keyring key fingerprint '$shipped' does not match the installer's $pinned"
doctor_pinned=$(sed -n 's/^const ryokuReleaseKeyFingerprint = "\([0-9a-f]*\)"$/\1/p' "$ROOT/ryoku/cli/internal/doctor/reconcile_fedora_keyring.go")
[[ ${doctor_pinned^^} == "$pinned" ]] || fail "ryoku doctor checks for key '$doctor_pinned', not the installer's $pinned"
# ryoku doctor deletes each revoked fingerprint from the rpm keyring.
while IFS= read -r revoked || [[ -n $revoked ]]; do
	[[ -z $revoked || $revoked == '#'* ]] && continue
	[[ $revoked =~ ^[0-9A-Fa-f]{40}$ ]] || fail "ryoku-revoked entry is not a full fingerprint: $revoked"
	[[ ${revoked^^} != "$pinned" ]] || fail "ryoku-revoked lists the current release key $pinned"
done < "$ROOT/release/packages/ryoku-keyring/ryoku-revoked"

echo "fedora-packages: package closure, translations, and RPM dependencies are complete"

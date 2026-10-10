#!/usr/bin/env bash
# Package closure and Void translation checks. The default path is offline;
# --repo adds one live XBPS repository pass in a disposable container.
set -euo pipefail

ROOT=${RYOKU_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
PACKAGES="$ROOT/void/packages"
TABLE="$PACKAGES/translations.tsv"
RESOLVE="$PACKAGES/resolve"
SRCPKGS="$PACKAGES/srcpkgs"
MODE=${1:-}

fail() { echo "void-packages: $*" >&2; exit 1; }

case $MODE in
	''|--repo) ;;
	*) fail "usage: ${0##*/} [--repo]" ;;
esac

[[ -f $TABLE ]] || fail "missing translation table: $TABLE"
[[ -x $RESOLVE ]] || fail "resolver is not executable: $RESOLVE"
[[ $(head -n 1 "$RESOLVE") == '#!/bin/sh' ]] || fail "resolver must use #!/bin/sh"

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
	hypr_hard=() hypr_opt=() hypr_seen=()
walk_closure ryoku-desktop neutral_hard neutral_opt neutral_seen
walk_closure ryoku-desktop-niri niri_hard niri_opt niri_seen
walk_closure ryoku-desktop-hyprland hypr_hard hypr_opt hypr_seen

declare -A closure=() expected_lanes=()
lane_order=(desktop dev system extra optional hyprland hardware:amd hardware:intel hardware:nvidia hardware:vm)

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
for pkg in "${!neutral_opt[@]}" "${!niri_opt[@]}" "${!hypr_opt[@]}"; do
	closure[$pkg]=1
	[[ -n ${expected_lanes[$pkg]:-} ]] || add_lane "$pkg" optional
done
add_lane bash desktop
add_lane ryoku-keyring desktop
add_lane ryoku-palette-bridge desktop
add_lane udev desktop

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
	if ($0 != "arch\tvoid\tlanes\tnotes") bad("bad header")
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
		if (lanes[i] !~ /^(desktop|dev|system|extra|optional|hyprland|hardware:(amd|intel|nvidia|vm))$/)
			bad("invalid lane " lanes[i])
		if (row_lane[lanes[i]]++) bad("duplicate lane " lanes[i])
	}
	if ($2 == "@fetch") bad("@fetch mappings are no longer supported")
	if ($2 == "@repo" || $2 == "-") {
		if ($4 == "") bad("special mapping requires notes")
		if ($4 ~ /^repo=/) bad("special mapping cannot declare a repository")
		if ($2 == "@repo" && $4 != "Ryoku-owned package built from the checkout.")
			bad("@repo mapping has the wrong ownership note")
	} else {
		if ($4 != "" && $4 !~ /^repo=(ryoku|multilib|nonfree|multilib-nonfree)$/)
			bad("invalid repository declaration " $4)
		count = split($2, names, /[[:space:]]+/)
		for (i = 1; i <= count; i++) if (!package_name(names[i])) bad("invalid Void package " names[i])
	}
	if ($1 == "snapper" && ($2 != "-" || $4 != "Ryoku does not set up snapshots on Void."))
		bad("snapper must carry the Void snapshot note")
}
END { exit failed ? 1 : 0 }
' "$TABLE" || fail "translation table is malformed"

declare -A rows=() row_void=() template_refs=()
while IFS=$'\t' read -r arch void row_lanes notes; do
	[[ $arch == arch || $arch == '#'* || -z $arch ]] && continue
	rows[$arch]=1
	row_void[$arch]=$void
	if [[ $void == @repo ]]; then
		template_refs[$arch]=1
	elif [[ $notes == repo=ryoku ]]; then
		for pkg in $void; do template_refs[$pkg]=1; done
	fi
	[[ -n ${closure[$arch]:-} ]] || fail "translation lies outside the Arch closure: $arch"
	expected=$(canonical_lanes "${expected_lanes[$arch]}")
	[[ $row_lanes == "$expected" ]] || fail "$arch lanes are '$row_lanes', expected '$expected'"
done < "$TABLE"
for pkg in "${!closure[@]}"; do
	[[ -n ${rows[$pkg]:-} ]] || fail "Arch closure package has no Void row: $pkg"
done
((${#rows[@]} == ${#closure[@]})) || fail "translation row count does not match closure"

for pkg in "${!template_refs[@]}"; do
	[[ -f $SRCPKGS/$pkg/template ]] \
		|| fail "Ryoku repository mapping has no template: $pkg"
done

template_count=0
for template in "$SRCPKGS"/*/template; do
	[[ -f $template ]] || continue
	((template_count += 1))
	dir_pkg=${template%/template}
	dir_pkg=${dir_pkg##*/}
	template_pkg=$(bash -c 'source "$1"; printf "%s" "$pkgname"' _ "$template") \
		|| fail "could not read template: $template"
	[[ $template_pkg == "$dir_pkg" ]] \
		|| fail "template directory $dir_pkg declares pkgname=$template_pkg"
	[[ $template_pkg == ryoku-keyring || -n ${template_refs[$template_pkg]:-} ]] \
		|| fail "template is not referenced by @repo or repo=ryoku: $template_pkg"

	pkgbuild=${pkgbuilds[$template_pkg]:-}
	[[ -n $pkgbuild ]] || continue
	declare -A template_depends=()
	template_dep_list=$(bash -c 'source "$1"; printf "%s\n" ${depends:-}' _ "$template") \
		|| fail "could not read depends from $template"
	while IFS= read -r raw; do
		[[ -n $raw ]] || continue
		name=$(strip_dep "${raw#virtual?}")
		template_depends[$name]=1
	done <<< "$template_dep_list"
	template_provides=$(bash -c 'source "$1"; printf "%s\n" ${provides:-}' _ "$template") \
		|| fail "could not read provides from $template"

	while IFS= read -r raw; do
		arch_dep=$(strip_dep "$raw")
		[[ -n $arch_dep ]] || continue
		[[ -n ${row_void[$arch_dep]+set} ]] \
			|| fail "$template_pkg PKGBUILD dependency has no translation: $arch_dep"
		mapping=${row_void[$arch_dep]}
		case $mapping in
			-) continue ;;
			@repo) expected_list=$arch_dep ;;
			*) expected_list=$mapping ;;
		esac
		for expected_dep in $expected_list; do
			# A compositor variant is pulled by the base through its virtual;
			# xbps-src cannot build the reverse edge, so the variant omits it.
			if [[ $expected_dep == ryoku-desktop && $template_provides == *ryoku-desktop-compositor-* ]]; then
				continue
			fi
			[[ -n ${template_depends[$expected_dep]:-} ]] \
				|| fail "$template_pkg template misses translated dependency $expected_dep (from $arch_dep)"
		done
	done < <(pkgbuild_field "$pkgbuild" depends)
	unset template_depends
done
((template_count > 0)) || fail "no XBPS templates found under $SRCPKGS"

check_resolve_output() {
	local out line sorted
	out=$($RESOLVE "$@") || fail "resolve failed for: $*"
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
grep -qxF fish-shell <<< "$desktop" || fail "desktop lane did not translate fish"
! grep -q '^void-repo-' <<< "$desktop" || fail "desktop lane enabled an unnecessary Void repository"
grep -qxF ryoku <<< "$desktop" || fail "desktop lane omitted an @repo package"
grep -qxF prowl <<< "$desktop" || fail "desktop lane omitted a repo=ryoku package"
! grep -qxF gpk <<< "$desktop" || fail "desktop lane included the pacman-only gpk frontend"
! grep -qxF snapper <<< "$desktop" || fail "desktop lane included unsupported snapshot tooling"
amd=$(check_resolve_output --lane hardware:amd)
grep -qxF void-repo-multilib <<< "$amd" || fail "AMD 32-bit packages did not enable Void multilib"
! grep -q '^void-repo-.*nonfree$' <<< "$amd" || fail "AMD lane enabled an unnecessary nonfree repository"
intel=$(check_resolve_output --lane hardware:intel)
grep -qxF void-repo-multilib <<< "$intel" || fail "Intel 32-bit packages did not enable Void multilib"
grep -qxF void-repo-nonfree <<< "$intel" || fail "Intel microcode did not enable Void nonfree"
nvidia=$(check_resolve_output --lane hardware:nvidia)
# Void's nvidia package blacklists nouveau, and its open kernel modules only
# drive Turing and newer: installed blindly it leaves an older card with no
# driver at all. It stays out until the driver step picks a branch per GPU.
! grep -qE '^nvidia($|-libs)' <<< "$nvidia" ||
	fail "NVIDIA lane installs the proprietary driver before the Void driver step chooses a branch"
vm=$(check_resolve_output --lane hardware:vm)
grep -qxF void-repo-multilib <<< "$vm" || fail "VM 32-bit packages did not enable Void multilib"
dropped=$(check_resolve_output --lane desktop --drop fish --drop chromium)
! grep -qxF fish-shell <<< "$dropped" || fail "repeated --drop left the fish translation"
! grep -qxF chromium <<< "$dropped" || fail "repeated --drop left the chromium package"

session=$(check_resolve_output --lane hyprland --session)
while IFS= read -r pkg; do
	grep -qxF "$pkg" <<< "$session" || fail "--session omitted $pkg"
done < <(package_lines "$PACKAGES/sets/session.packages")
build=$(check_resolve_output --lane hyprland --build)
while IFS= read -r pkg; do
	grep -qxF "$pkg" <<< "$build" || fail "--build omitted $pkg"
done < <(package_lines "$PACKAGES/sets/build.packages")

fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
printf 'arch\tvoid\tlanes\tnotes\nbroken\tonly-three\tdesktop\n' > "$fixture/bad.tsv"
if "$RESOLVE" --table "$fixture/bad.tsv" >"$fixture/out" 2>"$fixture/err"; then
	fail "resolve accepted a malformed table"
fi
grep -q 'line 2' "$fixture/err" || fail "malformed-table error did not name the bad line"

if [[ $MODE == --repo ]]; then
	repo_rows=$(
		{
			awk -F '\t' '
				NR == 1 || /^#/ || $2 == "-" || $2 == "@repo" || $4 == "repo=ryoku" { next }
				{
					repository = "main"
					if ($4 ~ /^repo=/) repository = substr($4, 6)
					count = split($2, names, /[[:space:]]+/)
					for (i = 1; i <= count; i++) print repository "\t" names[i]
				}
			' "$TABLE"
			while IFS= read -r pkg; do printf 'main\t%s\n' "$pkg"; done < <(package_lines "$PACKAGES/sets/session.packages")
			while IFS= read -r pkg; do printf 'main\t%s\n' "$pkg"; done < <(package_lines "$PACKAGES/sets/build.packages")
		} | LC_ALL=C sort -u
	)
	resolved=$(check_resolve_output --session --build)
	docker run --rm \
		-e XBPS_ARCH=x86_64 \
		-e "BASE_REPOSITORY=${VOID_REPOSITORY:-https://repo-default.voidlinux.org/current}" \
		-e "REPO_ROWS=$repo_rows" \
		-e "RESOLVED=$resolved" \
		"${VOID_IMAGE:-ghcr.io/void-linux/void-glibc}" sh -eu -c '
		base=${BASE_REPOSITORY%/}
		for spec in \
			"main $base" \
			"multilib $base/multilib" \
			"nonfree $base/nonfree" \
			"multilib-nonfree $base/multilib/nonfree"; do
			set -- $spec
			if ! xbps-query -i -M --repository "$2" -Rs "" > "/tmp/repo-$1.raw"; then
				printf "void-packages: failed to fetch %s repository index\n" "$1" >&2
				exit 1
			fi
			awk "NF >= 2 { name = \$2; sub(/-[^-]*_[0-9]+$/, \"\", name); if (name != \"\") print name }" \
				"/tmp/repo-$1.raw" | LC_ALL=C sort -u > "/tmp/repo-$1"
			if [ ! -s "/tmp/repo-$1" ]; then
				printf "void-packages: %s repository index is empty\n" "$1" >&2
				exit 1
			fi
		done

		: > /tmp/ryoku-repository-errors
		for enabler in void-repo-multilib void-repo-nonfree void-repo-multilib-nonfree; do
			if ! grep -qxF "$enabler" /tmp/repo-main; then
				printf "void-packages: repository enabler is not in main: %s\n" "$enabler" >&2
				printf "%s\n" "$enabler" >> /tmp/ryoku-repository-errors
			fi
		done

		tab=$(printf "\t")
		printf "%s\n" "$REPO_ROWS" | while IFS="$tab" read -r declared pkg; do
			[ -n "$pkg" ] || continue
			actual=
			for repository in main multilib nonfree multilib-nonfree; do
				if grep -qxF "$pkg" "/tmp/repo-$repository"; then
					actual=$repository
					break
				fi
			done
			if [ -z "$actual" ]; then
				printf "void-packages: not in Void repositories: %s\n" "$pkg" >&2
				printf "%s\n" "$pkg" >> /tmp/ryoku-repository-errors
				continue
			fi
			if [ "$actual" != "$declared" ]; then
				printf "void-packages: %s declares %s, found in %s\n" "$pkg" "$declared" "$actual" >&2
				printf "%s\n" "$pkg" >> /tmp/ryoku-repository-errors
				continue
			fi
			case $actual in
				main) enabler= ;;
				multilib) enabler=void-repo-multilib ;;
				nonfree) enabler=void-repo-nonfree ;;
				multilib-nonfree) enabler=void-repo-multilib-nonfree ;;
			esac
			if [ -n "$enabler" ] && ! printf "%s\n" "$RESOLVED" | grep -qxF "$enabler"; then
				printf "void-packages: %s needs missing resolver output %s\n" "$pkg" "$enabler" >&2
				printf "%s\n" "$pkg" >> /tmp/ryoku-repository-errors
			fi
		done
		[ ! -s /tmp/ryoku-repository-errors ]
	' || fail "Void repository declarations or resolver enablers are wrong"
fi

echo "void-packages: package closure and translations are complete"

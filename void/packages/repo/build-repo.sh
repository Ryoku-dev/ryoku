#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=$(cd "$SCRIPT_DIR/../../.." && pwd)
SRCPKGS=$ROOT/void/packages/srcpkgs
WORK=${RYOKU_XBPS_WORK:-${TMPDIR:-/tmp}/ryoku-xbps-$UID}
OUT=${RYOKU_XBPS_OUT:-$SCRIPT_DIR/out}
ARCH=${RYOKU_XBPS_ARCH:-x86_64}
VOID_PACKAGES_REF=${RYOKU_VOID_PACKAGES_REF:-deb0bc286bd5e1fbbf191802c3f578caf4fcdd94}
VOID_PACKAGES_URL=${RYOKU_VOID_PACKAGES_URL:-https://github.com/void-linux/void-packages.git}
KEY=${RYOKU_XBPS_KEY:-}
KEYRING_DIR=${RYOKU_XBPS_KEYRING_DIR:-}
MIRROR_DIR=${RYOKU_XBPS_MIRROR:-}
PRODUCTION=${RYOKU_XBPS_PRODUCTION:-0}
SOURCE_ARCHIVE=${RYOKU_XBPS_SOURCE_ARCHIVE:-}
VP=$WORK/void-packages
HOSTDIR=$WORK/hostdir
ARCH_DIR=$OUT/$ARCH

log() { printf '\033[1;35m::\033[0m %s\n' "$*"; }
die() { printf 'build-repo.sh: error: %s\n' "$*" >&2; exit 1; }

[[ $EUID -ne 0 ]] || die "xbps-src builds must run as a non-root user"
for command in cmp git tar zstd sha256sum xbps-rindex; do
	command -v "$command" >/dev/null 2>&1 || die "required command not found: $command"
done
[[ -d $SRCPKGS ]] || die "template directory not found: $SRCPKGS"
[[ $PRODUCTION == 0 || $PRODUCTION == 1 ]] \
	|| die "RYOKU_XBPS_PRODUCTION must be 0 or 1"
if [[ $PRODUCTION == 1 ]]; then
	[[ -n $KEY ]] || die "production builds require RYOKU_XBPS_KEY"
	[[ -z $KEYRING_DIR ]] \
		|| die "production builds must use the committed ryoku-keyring plist"
fi
if [[ -n $KEY ]]; then
	[[ -r $KEY ]] || die "cannot read RYOKU_XBPS_KEY: $KEY"
else
	log "No RYOKU_XBPS_KEY was supplied; building an unsigned local repository"
fi

raw_version=${RYOKU_PKGVER:-$("$ROOT/bin/ryoku-release-version" --pkgver)}
VERSION=${raw_version//-/.}
VERSION=${VERSION//_/.}
[[ $VERSION =~ ^[0-9][A-Za-z0-9.]*$ ]] \
	|| die "version '$raw_version' cannot be made into an XBPS version"

prepared_archive=
if [[ -z $SOURCE_ARCHIVE && ${RYOKU_XBPS_WORKTREE:-0} == 1 ]]; then
	mkdir -p "$WORK/source"
	canonical_archive=$WORK/source/ryoku-worktree.tar.gz
	(
		cd "$ROOT"
		git ls-files -co --exclude-standard -z \
			| while IFS= read -r -d '' path; do
				case $path in
					void/packages/repo/out/*|void/packages/repo/work/*) continue ;;
				esac
				printf '%s\0' "$path"
			done \
			| LC_ALL=C sort -z \
			| tar --create --gzip --file="$canonical_archive" --null --no-recursion \
				--transform="flags=r;s,^,ryoku-$VERSION/," --files-from=-
	)
	source_hash=$(sha256sum "$canonical_archive" | awk '{print $1}')
	VERSION=$VERSION.wt${source_hash:0:10}
	prepared_archive=$WORK/source/ryoku-$VERSION.tar.gz
	(
		cd "$ROOT"
		git ls-files -co --exclude-standard -z \
			| while IFS= read -r -d '' path; do
				case $path in
					void/packages/repo/out/*|void/packages/repo/work/*) continue ;;
				esac
				printf '%s\0' "$path"
			done \
			| LC_ALL=C sort -z \
			| tar --create --gzip --file="$prepared_archive" --null --no-recursion \
				--transform="flags=r;s,^,ryoku-$VERSION/," --files-from=-
	)
	rm -f "$canonical_archive"
fi

: "${RYOKU_RELEASE:=local-$VERSION}"
: "${RYOKU_CHANNEL:=local}"
RYOKU_NAME=${RYOKU_NAME:-$(tr -d '[:space:]' < "$ROOT/CODENAME")}
RYOKU_COMMIT=${RYOKU_XBPS_COMMIT:-$(git -C "$ROOT" rev-parse HEAD 2>/dev/null || printf unknown)}
export RYOKU_RELEASE RYOKU_CHANNEL RYOKU_NAME
log "Release -> $RYOKU_NAME $RYOKU_RELEASE ($RYOKU_CHANNEL), package version $VERSION"

shopt -s nullglob
templates=("$SRCPKGS"/*/template)
((${#templates[@]})) || die "no templates found under $SRCPKGS"
declare -A template_for=() dependencies=() selected=() built=()
all_packages=()
for template in "${templates[@]}"; do
	metadata=$(bash -c 'source "$1"; printf "%s\t%s\n" "$pkgname" "${depends:-}"' _ "$template") \
		|| die "could not read template metadata: $template"
	pkg=${metadata%%$'\t'*}
	deps=${metadata#*$'\t'}
	[[ -n $pkg ]] || die "template has no pkgname: $template"
	[[ -z ${template_for[$pkg]:-} ]] || die "duplicate template for $pkg"
	template_for[$pkg]=$template
	dependencies[$pkg]=$deps
	all_packages+=("$pkg")
done

if [[ -n ${RYOKU_XBPS_PACKAGES:-} ]]; then
	for pkg in $RYOKU_XBPS_PACKAGES; do
		[[ -n ${template_for[$pkg]:-} ]] || die "unknown package in RYOKU_XBPS_PACKAGES: $pkg"
		selected[$pkg]=1
	done
else
	for pkg in "${all_packages[@]}"; do selected[$pkg]=1; done
fi

changed=1
while ((changed)); do
	changed=0
	for pkg in "${all_packages[@]}"; do
		[[ -n ${selected[$pkg]:-} ]] || continue
		for dep in ${dependencies[$pkg]}; do
			dep=${dep#virtual?}
			dep=${dep%%[\<\>\=]*}
			if [[ -n ${template_for[$dep]:-} && -z ${selected[$dep]:-} ]]; then
				selected[$dep]=1
				changed=1
			fi
		done
	done
done

if [[ -n ${selected[ryoku-keyring]:-} ]]; then
	key_source=$SRCPKGS/ryoku-keyring/files
	[[ $PRODUCTION == 1 || -z $KEYRING_DIR ]] || key_source=$KEYRING_DIR
	key_plists=("$key_source"/*.plist)
	((${#key_plists[@]})) || die "ryoku-keyring has no plist; run void/packages/repo/new-signing-key PRIVATE_KEY and add the production plist, or set RYOKU_XBPS_KEYRING_DIR for a development key"
fi

mkdir -p "$WORK" "$OUT" "$HOSTDIR"
if [[ -n $MIRROR_DIR ]]; then
	[[ -d $MIRROR_DIR ]] || die "RYOKU_XBPS_MIRROR is not a directory: $MIRROR_DIR"
	mirror_packages=("$MIRROR_DIR"/*.xbps)
	if ((${#mirror_packages[@]})); then
		mkdir -p "$HOSTDIR/binpkgs"
		cp -f "${mirror_packages[@]}" "$HOSTDIR/binpkgs/"
		host_packages=("$HOSTDIR/binpkgs"/*.xbps)
		rm -f "$HOSTDIR/binpkgs/$ARCH-repodata"
		XBPS_TARGET_ARCH=$ARCH xbps-rindex --add "${host_packages[@]}"
		log "Adopted ${#mirror_packages[@]} published package(s)"
	fi
fi
# Go's module cache may be read-only.
[[ ! -d $VP ]] || chmod -R u+w "$VP"
rm -rf "$VP"
mkdir -p "$WORK/source"
log "Cloning void-packages at $VOID_PACKAGES_REF"
git init -q "$VP"
git -C "$VP" remote add origin "$VOID_PACKAGES_URL"
git -C "$VP" fetch -q --depth=1 origin "$VOID_PACKAGES_REF"
git -C "$VP" checkout -q --detach FETCH_HEAD
cp -a "$SRCPKGS"/. "$VP/srcpkgs/"

release_metadata=$VP/srcpkgs/ryoku-desktop/files/ryoku-release.env
mkdir -p "$(dirname "$release_metadata")"
{
	printf 'RYOKU_RELEASE=%q\n' "$RYOKU_RELEASE"
	printf 'RYOKU_CHANNEL=%q\n' "$RYOKU_CHANNEL"
	printf 'RYOKU_NAME=%q\n' "$RYOKU_NAME"
	printf 'RYOKU_COMMIT=%q\n' "$RYOKU_COMMIT"
} > "$release_metadata"

if [[ -n $KEYRING_DIR ]]; then
	[[ -d $KEYRING_DIR ]] || die "RYOKU_XBPS_KEYRING_DIR is not a directory: $KEYRING_DIR"
	development_plists=("$KEYRING_DIR"/*.plist)
	rm -f "$VP/srcpkgs/ryoku-keyring/files/"*.plist
	if ((${#development_plists[@]})); then
		cp -a "${development_plists[@]}" "$VP/srcpkgs/ryoku-keyring/files/"
	fi
fi

archive=$WORK/source/ryoku-$VERSION.tar.gz
if [[ -n $prepared_archive ]]; then
	archive=$prepared_archive
elif [[ -n $SOURCE_ARCHIVE ]]; then
	[[ -r $SOURCE_ARCHIVE ]] || die "cannot read RYOKU_XBPS_SOURCE_ARCHIVE: $SOURCE_ARCHIVE"
	if [[ $SOURCE_ARCHIVE != "$archive" ]]; then
		cp "$SOURCE_ARCHIVE" "$archive"
	fi
else
	git -C "$ROOT" archive --format=tar.gz --prefix="ryoku-$VERSION/" -o "$archive" HEAD
fi
archive_sha=$(sha256sum "$archive" | awk '{print $1}')
# Remove obsolete monorepo archives before they consume hundreds of megabytes.
for stale in "$WORK"/source/ryoku-*.tar.gz; do
	[[ -e $stale && $stale != "$archive" ]] && rm -f "$stale"
done

for pkg in "${all_packages[@]}"; do
	template=${template_for[$pkg]}
	if grep -qF 'https://repo.ryoku.dev/sources/ryoku-${version}.tar.gz' "$template"; then
		stamped=$VP/srcpkgs/$pkg/template
		awk -v version="$VERSION" -v checksum="$archive_sha" '
			!version_done && /^version=/ { print "version=" version; version_done=1; next }
			!checksum_done && /^checksum=/ { print "checksum=" checksum; checksum_done=1; next }
			{ print }
		' "$stamped" > "$stamped.tmp"
		mv "$stamped.tmp" "$stamped"
		cache=$HOSTDIR/sources/$pkg-$VERSION
		for stale in "$HOSTDIR/sources/$pkg"-[0-9]*; do
			[[ -e $stale && $stale != "$cache" ]] && rm -rf "$stale"
		done
		mkdir -p "$cache"
		ln -f "$archive" "$cache/ryoku-$VERSION.tar.gz" 2>/dev/null \
			|| cp "$archive" "$cache/ryoku-$VERSION.tar.gz"
	fi
done

{
	printf '%s\n' 'XBPS_BUILD_ENVIRONMENT=ryoku-repository' 'XBPS_CHROOT_CMD=uchroot'
	# Reuse exact package versions already present in the persistent hostdir.
	printf '%s\n' 'XBPS_PRESERVE_PKGS=yes'
	printf 'XBPS_HOSTDIR=%q\n' "$HOSTDIR"
	printf 'export RYOKU_RELEASE=%q\n' "$RYOKU_RELEASE"
	printf 'export RYOKU_CHANNEL=%q\n' "$RYOKU_CHANNEL"
	printf 'export RYOKU_NAME=%q\n' "$RYOKU_NAME"
} >> "$VP/etc/conf"
# xbps-src needs etc/virtual even when the provider is built in the same run.
printf '%s\n' 'ryoku-desktop-compositor ryoku-desktop-niri' >> "$VP/etc/virtual"
log "Bootstrapping the pinned xbps-src masterdir"
(
	cd "$VP"
	./xbps-src binary-bootstrap
)

# Reuse exact package versions except the desktop package, whose release
# metadata belongs to this repository publication.
selected_count=${#selected[@]}
while ((${#built[@]} < selected_count)); do
	progress=0
	for pkg in "${all_packages[@]}"; do
		[[ -n ${selected[$pkg]:-} && -z ${built[$pkg]:-} ]] || continue
		ready=1
		for dep in ${dependencies[$pkg]}; do
			dep=${dep#virtual?}
			dep=${dep%%[\<\>\=]*}
			if [[ -n ${selected[$dep]:-} && -z ${built[$dep]:-} ]]; then
				ready=0
				break
			fi
		done
		((ready)) || continue
		log "Building $pkg"
		(
			cd "$VP"
			if [[ $pkg == ryoku-desktop ]]; then
				./xbps-src -f pkg "$pkg"
			else
				./xbps-src pkg "$pkg"
			fi
		)
		built[$pkg]=1
		progress=1
	done
	((progress)) || die "overlay dependency cycle among selected packages"
done

rm -rf "$ARCH_DIR"
mkdir -p "$ARCH_DIR"
for pkg in "${all_packages[@]}"; do
	[[ -n ${selected[$pkg]:-} ]] || continue
	stamped=$VP/srcpkgs/$pkg/template
	identity=$(bash -c 'source "$1"; printf "%s\t%s\n" "$version" "$revision"' _ "$stamped") \
		|| die "could not read built identity for $pkg"
	pkg_version=${identity%%$'\t'*}
	pkg_revision=${identity#*$'\t'}
	found=0
	while IFS= read -r -d '' package; do
		name=$(basename "$package")
		[[ ! -e $ARCH_DIR/$name ]] || die "duplicate built package filename: $name"
		cp "$package" "$ARCH_DIR/$name"
		found=1
	done < <(find "$HOSTDIR/binpkgs" -type f \
		-name "${pkg}-${pkg_version}_${pkg_revision}.*.xbps" -print0)
	((found)) || die "selected package was not built: ${pkg}-${pkg_version}_${pkg_revision}"
done
packages=("$ARCH_DIR"/*.xbps)
((${#packages[@]})) || die "xbps-src produced no selected packages"
if [[ $PRODUCTION == 1 && -n ${selected[ryoku-keyring]:-} ]]; then
	keyring_packages=("$ARCH_DIR"/ryoku-keyring-*.xbps)
	((${#keyring_packages[@]} == 1)) \
		|| die "production repository must contain one ryoku-keyring package"
	keyring_check=$WORK/keyring-check
	rm -rf "$keyring_check"
	mkdir -p "$keyring_check"
	tar -xf "${keyring_packages[0]}" -C "$keyring_check"
	committed_plists=("$SRCPKGS/ryoku-keyring/files/"*.plist)
	packaged_plists=("$keyring_check/var/db/xbps/keys/"*.plist)
	((${#packaged_plists[@]} == ${#committed_plists[@]})) \
		|| die "ryoku-keyring package does not contain exactly the committed plists"
	for committed in "${committed_plists[@]}"; do
		packaged="$keyring_check/var/db/xbps/keys/$(basename "$committed")"
		[[ -f $packaged ]] && cmp -s "$committed" "$packaged" \
			|| die "ryoku-keyring package does not contain committed plist $(basename "$committed")"
	done
	rm -rf "$keyring_check"
fi

log "Indexing ${#packages[@]} package(s)"
XBPS_TARGET_ARCH=$ARCH xbps-rindex --add "${packages[@]}"
XBPS_TARGET_ARCH=$ARCH xbps-rindex --remove-obsoletes "$ARCH_DIR"
XBPS_TARGET_ARCH=$ARCH xbps-rindex --hashcheck --clean "$ARCH_DIR"

if [[ -n $KEY ]]; then
	rm -f "$ARCH_DIR"/*.xbps.sig2
	log "Signing repository metadata and packages"
	XBPS_TARGET_ARCH=$ARCH xbps-rindex --privkey "$KEY" --sign \
		--signedby 'Ryoku Linux package repository' "$ARCH_DIR"
	XBPS_TARGET_ARCH=$ARCH xbps-rindex --privkey "$KEY" --sign-pkg "${packages[@]}"
	for package in "${packages[@]}"; do
		[[ -s $package.sig2 ]] || die "missing package signature: $(basename "$package").sig2"
	done
fi
[[ -s $ARCH_DIR/$ARCH-repodata ]] || die "$ARCH-repodata was not created"

printf '{"schema":1,"release":"%s","name":"%s","channel":"%s","version":"%s","commit":"%s","date":"%s"}\n' \
	"$RYOKU_RELEASE" "$RYOKU_NAME" "$RYOKU_CHANNEL" "$VERSION" "$RYOKU_COMMIT" \
	"$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$ARCH_DIR/release.json"

log "XBPS repository ready at $ARCH_DIR"

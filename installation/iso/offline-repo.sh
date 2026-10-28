#!/usr/bin/env bash
# Bake the full target package closure into a [offline] file:// repo so the
# installed system pacstraps with NO network (installation/backend/lib/offline.sh).
#
# Downloads (build host needs network + disk) every package a target can reach:
#   base.packages + every hardware.packages profile (amd/intel/nvidia microcode)
#   + dev.packages + the GPU-driver packages the per-vendor
#   scripts install + the Ryoku desktop set (ryoku-keyring, ryoku-desktop).
# Resolves the whole dependency closure against Arch core/extra/multilib and
# [ryoku] (the cachyos variant puts the generic [cachyos] repo above Arch's, as
# a CachyOS install orders it), into one repo with a repo-add db. TrustAll
# download over TLS; the installed target re-verifies against real keyrings on
# first -Syu.
#
# usage: offline-repo.sh <REPO_ROOT> <DEST_REPO_DIR> [RYOKU_REPO_URL]
# env:
#   RYOKU_OFFLINE_CACHE   persistent pkg cache (reused across builds; NOT wiped)
#   RYOKU_ARCH_MIRROR     Arch mirror base (default geo.mirror.pkgbuild.com)
#   RYOKU_OFFLINE_MEASURE 1 = resolve + count the closure, then stop (no download)
set -euo pipefail

REPO_ROOT=$1
DEST=$2
RYOKU_REPO_URL=${3:-https://repo.ryoku.dev/stable/x86_64}

ARCH_MIRROR=${RYOKU_ARCH_MIRROR:-https://geo.mirror.pkgbuild.com}
CACHE=${RYOKU_OFFLINE_CACHE:-$REPO_ROOT/installation/iso/offline-cache}
REPO_NAME=offline
VARIANT=${RYOKU_VARIANT:-plain}
CACHY_MIRROR=${RYOKU_CACHYOS_MIRROR:-https://mirror.cachyos.org/repo}

log() { printf '\033[1;36moffline-repo:\033[0m %s\n' "$*"; }
die() { printf 'offline-repo: error: %s\n' "$*" >&2; exit 1; }

command -v pacman >/dev/null 2>&1 || die "pacman not found"
command -v repo-add >/dev/null 2>&1 || die "repo-add not found (pacman package)"
case $VARIANT in
  plain | cachyos) ;;
  *) die "offline pacman closure supports only plain or cachyos, got: $VARIANT" ;;
esac


# read a plain one-per-line list, dropping comments + blanks.
read_list() { [[ -f $1 ]] && grep -vE '^[[:space:]]*(#|$)' "$1" || true; }
# read the packages under [section] in an INI file (matches lib/pacstrap.sh).
read_section() {
  awk -v sec="[$2]" '
    $0 == sec { f = 1; next }
    /^\[/ { f = 0 }
    f && NF && $0 !~ /^[[:space:]]*#/ { print }
  ' "$1"
}

pkgdir="$REPO_ROOT/system/packages"
hw="$pkgdir/hardware.packages"

# Every file manager the TUI offers, from the manifest it reads them from:
# deploy.sh installs the chosen one with the desktop set, and offline that
# transaction sees only this repo.
mapfile -t file_managers < <(awk -F'\t' '
  $1 == "choice" && $2 == "file-manager" && $5 != "-" { gsub(",", "\n", $5); print $5 }
' "$REPO_ROOT/installation/installerplan/choices.tsv")
(( ${#file_managers[@]} )) || die "no file-manager choices in installation/installerplan/choices.tsv"

# The base.packages and aur.packages names the CachyOS layer replaces (the stock
# kernel, the AUR's yay-bin): a CachyOS install never carries them
# (lib/cachyos.sh drops them), so neither does its closure.
replaced=()
[[ $VARIANT == cachyos ]] && mapfile -t replaced < <(read_list "$pkgdir/cachyos.replaces")
without_replaced() {
  awk 'NR == FNR { drop[$0] = 1; next } !drop[$0]' <(printf '%s\n' "${replaced[@]}") -
}
is_replaced() {
  local r
  for r in "${replaced[@]}"; do [[ $r == "$1" ]] && return 0; done
  return 1
}

# the full closure's top-level names (deps are pulled in by pacman -S).
mapfile -t PKGS < <(
  read_list "$pkgdir/base.packages"
  read_list "$pkgdir/dev.packages"
  # cachyos variant: the full CachyOS layer (kernel, settings, schedulers, proton).
  [[ $VARIANT == cachyos ]] && read_list "$pkgdir/cachyos.packages"
  # every hardware profile so any machine installs offline (the ISO carries all).
  read_section "$hw" amd
  read_section "$hw" intel
  read_section "$hw" nvidia
  # [vm] carries no packages today (it rides mesa + vulkan-icd-loader from base),
  # but read it anyway: a package added there later must land in the closure, not
  # discover at install time that only three of the four profiles were baked.
  read_section "$hw" vm
  # GPU-driver packages the per-vendor scripts (system/hardware/drivers/*.sh)
  # install. the mutually-conflicting nvidia MODULE packages are fetched
  # separately below (one -Sw each) so every variant lands in the repo; a single
  # resolve would keep only one. nvidia-utils/libva/lib32 don't conflict, so here.
  printf '%s\n' \
    mesa vulkan-radeon lib32-vulkan-radeon \
    intel-media-driver vpl-gpu-rt vulkan-intel lib32-vulkan-intel sof-firmware \
    nvidia-utils lib32-nvidia-utils libva-nvidia-driver \
    vulkan-icd-loader lib32-vulkan-icd-loader \
    broadcom-wl-dkms
  # The desktop set plus the hardware-only ASUS Aura provider: the target
  # installer selects asusctl only on a matching laptop. All compositor
  # variants bake because the TUI offers each one and an offline install has no
  # network to fetch a missing variant. The fresh-install apps bake beside
  # ryoku-rashin. The optional CachyOS layer remains network-only on plain.
  printf '%s\n' ryoku-keyring ryoku-desktop ryoku-desktop-hyprland ryoku-desktop-niri ryoku-desktop-wayfire asusctl \
    ryoku-rashin ryotunes ryomanager "${file_managers[@]}"
)
# dedupe, keep order.
mapfile -t PKGS < <(printf '%s\n' "${PKGS[@]}" | without_replaced | awk '!seen[$0]++')
log "closure has ${#PKGS[@]} top-level packages (deps resolved by pacman)"

# throwaway pacman config + db so the host's own pacman state is untouched.
# SigLevel = Never for the download: the DBs/packages come over TLS from official
# infra (verifying here would need every repo's key in a host keyring), and the
# installed target re-verifies everything against real keyrings on its first -Syu.
work=$(mktemp -d)
trap 'rm -rf "$work" 2>/dev/null || sudo rm -rf "$work"' EXIT
conf="$work/pacman.conf"
cat >"$conf" <<EOF
[options]
Architecture = x86_64
SigLevel = Never
LocalFileSigLevel = Never
ParallelDownloads = 10
EOF

# cachyos variant: the generic [cachyos] repo goes ABOVE Arch's, where a
# CachyOS install lists it (lib/cachyos.sh), so the closure takes CachyOS's
# build of everything it forks (pacman, mesa, the kernel layer). No CPU tier is
# baked: generic x86-64 builds install on any CPU, as CachyOS's own offline
# install does, and the installer writes the CPU's tier repos into the target,
# whose builds carry a higher pkgrel, so the first -Syu moves onto them.
if [[ $VARIANT == cachyos ]]; then
  printf '\n[cachyos]\nServer = %s/x86_64/cachyos\n' "$CACHY_MIRROR" >>"$conf"
fi

cat >>"$conf" <<EOF

[core]
Server = $ARCH_MIRROR/core/os/x86_64
[extra]
Server = $ARCH_MIRROR/extra/os/x86_64
[multilib]
Server = $ARCH_MIRROR/multilib/os/x86_64

[ryoku]
Server = $RYOKU_REPO_URL
EOF

# pacman -Sy/-Sw need root even with an isolated dbpath/cachedir. use sudo when
# not already root (the ISO build runs mkarchiso under sudo too); --dbpath and
# --cachedir keep the host's real pacman state untouched.
PAC=(pacman); (( EUID == 0 )) || PAC=(sudo pacman)

mkdir -p "$CACHE" "$work/db"
log "syncing repo databases (throwaway db)"
"${PAC[@]}" -Sy --config "$conf" --dbpath "$work/db" --noconfirm >/dev/null

# SERVED: every name the configured repos serve, mapped to the version pacman
# resolves it to: the first repo in config order that has the name, even when a
# later repo has a newer build. RESOLVED keeps every "name version" pacman
# resolved during this run, so a retry's db re-sync does not orphan what an
# earlier sync already fetched. Re-read after every db sync.
declare -A SERVED=() SERVED_REPO=() RESOLVED=()
read_served() {
  local repo name ver
  SERVED=()
  SERVED_REPO=()
  while read -r repo name ver _; do
    [[ -n ${SERVED[$name]:-} ]] && continue
    SERVED[$name]=$ver
    SERVED_REPO[$name]=$repo
    RESOLVED["$name $ver"]=1
    # A package file names its version without the epoch when the repo that
    # publishes it does (ryotunes is 1:1.2.3-1 in a ryotunes-1.2.3-1 file), and
    # stage_offline_repo matches by filename, so it dropped ryotunes unnoticed.
    RESOLVED["$name ${ver#*:}"]=1
  done < <("${PAC[@]}" -Sl --config "$conf" --dbpath "$work/db")
}
read_served
# Pacman resolves duplicate names from the first repository, regardless of
# version. Qualify every newer [ryoku] duplicate so the flat offline repository
# contains the same overrides `ryoku update` would select, without downgrades.
ryoku_pins=()
while read -r _repo name ver _; do
  first_repo=${SERVED_REPO[$name]:-}
  [[ -n $first_repo && $first_repo != ryoku ]] || continue
  comparison=$(vercmp "$ver" "${SERVED[$name]}") || die "could not compare repository versions for $name"
  (( comparison > 0 )) && ryoku_pins+=("$name")
done < <("${PAC[@]}" -Sl --config "$conf" --dbpath "$work/db" ryoku)
for name in "${ryoku_pins[@]}"; do
  PKGS+=("ryoku/$name")
done
log "qualified ${#ryoku_pins[@]} newer [ryoku] duplicate(s) in the offline closure"

# An aur.packages name a configured repo serves (CachyOS ships zen-browser-bin)
# is that repo's package: fetch its build like any other, so the box updates it
# from the repo, rather than building an AUR copy that shadows it. Only the
# rest is built from the AUR.
AUR_BUILD=()
while IFS= read -r n; do
  if [[ -n ${SERVED[$n]:-} ]]; then PKGS+=("$n"); else AUR_BUILD+=("$n"); fi
done < <(read_list "$pkgdir/aur.packages" | without_replaced)

# validate every name + resolve the whole closure BEFORE downloading a gigabyte.
#
# Check each name on its own first. pacman reports a whole-transaction failure as
# a bare "could not satisfy dependencies" with no offender named, and a virtual
# name with several providers fails exactly that way under --noconfirm: CachyOS
# split proton-cachyos into -slr and -native, both only *providing* the old name,
# and the ISO died with nothing to go on. Name the package and the reason here.
log "validating package names"
bad=()
for p in "${PKGS[@]}"; do
  "${PAC[@]}" -Si --config "$conf" --dbpath "$work/db" "$p" >/dev/null 2>&1 && continue
  # not a real package: say whether something merely provides the name
  providers=$("${PAC[@]}" -Ss --config "$conf" --dbpath "$work/db" "^${p}\$" 2>/dev/null | grep -c '^[a-z]' || true)
  if (( providers == 0 )) && "${PAC[@]}" -Sp --config "$conf" --dbpath "$work/db" "$p" >/dev/null 2>&1; then
    bad+=("$p (a virtual name; declare the concrete package that provides it)")
  else
    bad+=("$p (no such package in the configured repos)")
  fi
done
if (( ${#bad[@]} )); then
  printf 'offline-repo: unusable package name: %s\n' "${bad[@]}" >&2
  die "fix system/packages or the driver list in offline-repo.sh"
fi

log "resolving the dependency closure"
if ! resolved=$("${PAC[@]}" -Sp --config "$conf" --dbpath "$work/db" --print-format '%n' "${PKGS[@]}" 2>"$work/err"); then
  cat "$work/err" >&2
  die "closure resolution failed; fix system/packages or the driver list in offline-repo.sh"
fi
count=$(printf '%s\n' "$resolved" | grep -c . || true)
log "resolved closure: $count packages (with dependencies)"

if [[ ${RYOKU_OFFLINE_MEASURE:-0} == 1 ]]; then
  log "MEASURE mode: not downloading. resolved package count = $count"
  exit 0
fi

# drop_stale_cache: delete each cached file that has the filename of a build a
# repo serves but not that build's checksum. The AUR bake caches its builds as
# NAME-VER-REL-ARCH, and a repo can serve the very same filename
# (lib32-nvidia-580xx-utils is 580.178.04-1 in both the AUR and [cachyos]), so
# once a repo serves a name an earlier bake built, `pacman -Sw --needed` keeps
# the AUR file, fails its checksum against the db and aborts the transaction.
# The first repo in config order wins a filename, as it wins the name.
command -v bsdtar >/dev/null 2>&1 || die "bsdtar (libarchive) is required to check and verify cached packages"
drop_stale_cache() {
  local repo f sum
  local -A expect=()
  local -a check=() stale=()
  while read -r f sum; do
    [[ -z ${expect[$f]:-} ]] && expect[$f]=$sum
  done < <(
    sed -n 's/^\[\(.*\)\]$/\1/p' "$conf" | grep -vx options | while IFS= read -r repo; do
      bsdtar -xOf "$work/db/sync/$repo.db" '*/desc' \
        | awk '/^%FILENAME%$/ { getline; f = $0 } /^%SHA256SUM%$/ { getline; print f, $0 }'
    done
  )
  for f in "$CACHE"/*.pkg.tar.zst "$CACHE"/*.pkg.tar.xz; do
    [[ -e $f && -n ${expect[${f##*/}]:-} ]] && check+=("$f")
  done
  (( ${#check[@]} )) || return 0
  while read -r sum f; do
    [[ $sum == "${expect[${f##*/}]}" ]] || stale+=("$f")
  done < <(sha256sum "${check[@]}")
  (( ${#stale[@]} )) || return 0
  log "dropping ${#stale[@]} cached package(s) that are not the repo's build: ${stale[*]##*/}"
  rm -f "${stale[@]}"
}
drop_stale_cache

# download the whole closure into the persistent cache (--needed skips what a
# prior build already fetched, so a rebuild is a delta, not a re-download).
log "downloading the closure into $CACHE (this is the long pole; cached for reuse)"
"${PAC[@]}" -Sw --config "$conf" --dbpath "$work/db" --cachedir "$CACHE" --noconfirm --needed "${PKGS[@]}"

# the nvidia kernel-module packages all provide NVIDIA-MODULE and conflict
# pairwise, so a single resolve keeps only one. fetch each in its own pass so
# every variant lands in the repo and the installer's nvidia.sh can pick per-GPU
# offline. nvidia.sh installs each installed kernel's prebuilt
# <pkgbase>-nvidia-open (stock linux: nvidia-open), and nvidia-open-dkms when
# any kernel lacks one. those are what every supported card installs (Turing+,
# the only GPUs the open module and the current repos cover), so they are
# REQUIRED: a missing one is the driverless NVIDIA desktop from issue #30 and
# must fail the build, not slip through a warning. the cachyos variant installs
# linux-cachyos and linux-cachyos-lts and no stock kernel (cachyos.replaces).
if [[ $VARIANT == cachyos ]]; then
  nv_required=(linux-cachyos-nvidia-open linux-cachyos-lts-nvidia-open nvidia-open-dkms)
  nv_optional=()
else
  nv_required=(nvidia-open nvidia-open-dkms)
  nv_optional=(nvidia-open-lts)
fi
for v in "${nv_required[@]}"; do
  "${PAC[@]}" -Sw --config "$conf" --dbpath "$work/db" --cachedir "$CACHE" --noconfirm --needed "$v" \
    || die "required NVIDIA driver '$v' could not be fetched; an NVIDIA target would install to a driverless desktop. check the [extra] mirror and that the package still exists."
done
for v in "${nv_optional[@]}"; do
  "${PAC[@]}" -Sw --config "$conf" --dbpath "$work/db" --cachedir "$CACHE" --noconfirm --needed "$v" \
    || log "note: optional nvidia variant '$v' not fetched (absent from the current repos?); continuing"
done

# Pre-Turing NVIDIA (Maxwell/Pascal/Volta) and Kepler need the legacy driver
# branches. system/hardware/drivers/nvidia.sh installs them per-GPU and its
# messages already told the user they were "bundled in the offline repo" -- they
# never were, so a GTX 10xx (Pascal) installing offline landed on exactly the
# driverless desktop that nv_required exists to prevent.
#
# Baked, never installed by default: they provide NVIDIA-MODULE and so conflict
# with nvidia-open, and lib/offline.sh only auto-installs aur.packages.
nv_legacy=(
  nvidia-580xx-dkms lib32-nvidia-580xx-utils
  nvidia-470xx-dkms lib32-nvidia-470xx-utils
)
# Arch serves neither branch, so the plain variant BUILDS them like the rest of
# the AUR set; naming the dkms package and the lib32 base is enough, because
# makepkg emits a base's whole split set (the nvidia-580xx-utils base also yields
# nvidia-580xx-utils and opencl-nvidia-580xx). CachyOS serves both, patched for
# its kernels, so there a configured repo's build is fetched instead, one pass
# each like nv_required (they conflict with nvidia-utils), and required: a repo
# fetch has none of an AUR build's excuses for missing.
nv_legacy_fetch=()
nv_legacy_build=()
for v in "${nv_legacy[@]}"; do
  if [[ -n ${SERVED[$v]:-} ]]; then nv_legacy_fetch+=("$v"); else nv_legacy_build+=("$v"); fi
done
for v in "${nv_legacy_fetch[@]}"; do
  "${PAC[@]}" -Sw --config "$conf" --dbpath "$work/db" --cachedir "$CACHE" --noconfirm --needed "$v" \
    || die "legacy NVIDIA driver '$v' could not be fetched from the repo that serves it; a pre-Turing NVIDIA target would install to nouveau. check the mirror."
done

# pkgfile_in DIR NAME: which file in DIR is package NAME? A package filename is
# NAME-VER-REL-ARCH.pkg.tar.*, and VER and REL never contain '-', so stripping the
# three trailing '-' fields recovers NAME exactly. Matching that stem (rather than
# a "NAME-<digit>" glob) is correct for a version starting with a letter
# (libyuv-r2426+..., a git r<rev> build) and still tells a name apart from one it
# is a prefix of (nvidia-open vs nvidia-open-dkms).
pkgfile_in() {
  local dir=$1 want=$2 f base stem
  for f in "$dir/$want"-*.pkg.tar.*; do
    [[ -e $f ]] || continue
    base=${f##*/}; base=${base%.pkg.tar.*}            # NAME-VER-REL-ARCH
    stem=${base%-*}; stem=${stem%-*}; stem=${stem%-*} # drop ARCH, REL, VER
    [[ $stem == "$want" ]] && { printf '%s\n' "$f"; return 0; }
  done
  return 1
}

# Bundle the AUR toolset into the offline repo. Every ISO install is offline, so
# the AUR set an online install builds (voxtype, brand fonts, extra cursors, game
# controllers, localsend, the legacy NVIDIA branches, ...) never lands otherwise:
# system/packages/aur.packages is skipped on an offline install. Build each with
# makepkg on the networked build host (the target has no toolchain), pull its
# runtime deps into the closure so it resolves offline, and cache the result in
# $CACHE so a rebuild is a delta. This is how omarchy bakes its packages too.
#
# The bake itself is best-effort; COMPLETENESS IS ENFORCED AFTER IT, by checking
# the assembled repo for every name in aur.packages. That split is deliberate. It
# used to warn and continue everywhere, so a build host without base-devel
# published an ISO carrying zero AUR packages and said so only in a log line
# nobody reads (the last released image shipped without its UI font exactly that
# way). But making the bake itself fatal is worse: its body is a pile of
# best-effort probes whose non-zero exits are normal, and under errexit those
# aborted the build mid-bake with no message. So the probes stay tolerant and the
# outcome is checked once, where it can be checked honestly.
# RYOKU_OFFLINE_ALLOW_INCOMPLETE=1 downgrades that check for local builds that
# only need a bootable image.

bake_aur_set() {
  local aur_file="$REPO_ROOT/system/packages/aur.packages"
  local lax=${RYOKU_OFFLINE_ALLOW_INCOMPLETE:-0}
  [[ -f $aur_file ]] || die "system/packages/aur.packages is missing; the offline repo would ship without the AUR set"
  if ! { command -v makepkg && command -v git && command -v curl; } >/dev/null 2>&1; then
    [[ $lax == 1 ]] || die "AUR bake needs makepkg, git and curl (base-devel) on the build host; without them the ISO ships with no AUR packages. Install base-devel, or set RYOKU_OFFLINE_ALLOW_INCOMPLETE=1 for a local build."
    log "AUR bake: makepkg/git/curl missing; skipping (RYOKU_OFFLINE_ALLOW_INCOMPLETE=1)"
    return 0
  fi

  # the names no configured repo serves (AUR_BUILD, split off above), plus the
  # legacy NVIDIA branches no repo serves, which ride the same builder but are
  # never auto-installed; see nv_legacy above.
  local -a names=("${AUR_BUILD[@]}" "${nv_legacy_build[@]}")
  (( ${#names[@]} )) || die "no AUR packages resolved to bake"

  # makepkg refuses to run as root: build as an unprivileged user with passwordless
  # pacman so -s can sync build deps. a non-root build host builds as itself.
  local builder="" 
  local -a as=()
  if (( EUID == 0 )); then
    builder=ryoku-aurbuild
    id "$builder" &>/dev/null || useradd -m -s /bin/bash "$builder"
    printf '%s ALL=(ALL) NOPASSWD: /usr/bin/pacman\n' "$builder" >/etc/sudoers.d/99-ryoku-aurbuild
    chmod 0440 /etc/sudoers.d/99-ryoku-aurbuild
    as=(runuser -u "$builder" --)
  fi

  local bdir out n base src f dep
  bdir=$(mktemp -d); out="$bdir/out"; mkdir -p "$out"; chmod -R 0777 "$bdir"
  local -a built=() failed=() deps=()
  log "AUR bake: building ${#names[@]} package(s) into the offline repo"
  for n in "${names[@]}"; do
    # cache hit from a prior build (clear $CACHE to force a fresh build).
    if compgen -G "$CACHE/$n-*.pkg.tar.*" >/dev/null 2>&1; then built+=("$n(cached)"); continue; fi
    # split packages: the AUR git is keyed by PackageBase, not the pkgname. The
    # `|| base=""` matters: under `set -o pipefail` a lookup that finds nothing
    # (a transient RPC hiccup) makes the whole pipeline non-zero, which with
    # errexit aborts the build mid-bake with no message at all. Fall back to the
    # pkgname, which is the right guess for a non-split package anyway.
    base=$(curl -fsSL "https://aur.archlinux.org/rpc/v5/info?arg[]=$n" 2>/dev/null \
      | grep -oE '"PackageBase":[[:space:]]*"[^"]+"' | head -1 | sed -E 's/.*"([^"]+)"$/\1/') || base=""
    [[ -n $base ]] || base=$n
    src="$bdir/$base"
    # Retry with backoff: a release build died on "offline repo is missing
    # required packages: yay-bin" because AUR clones blip under the runner's
    # shared IP, and one quick retry still lost 1-2 random packages per run.
    # Back off so the AUR per-IP throttle window clears; the required-package
    # check below still fails the build if a package is genuinely absent.
    local attempt ok=0 reason="" delay
    for attempt in 1 2 3 4; do
      rm -rf "$src"
      if git clone -q --depth 1 "https://aur.archlinux.org/$base.git" "$src" 2>/dev/null \
        && [[ -f $src/PKGBUILD ]]; then
        (( EUID == 0 )) && chown -R "$builder:$builder" "$src"
        local -a envv=(env "PKGDEST=$out")
        [[ -n $builder ]] && envv=(env "HOME=/home/$builder" "PKGDEST=$out")
        if ( cd "$src" && "${as[@]}" "${envv[@]}" makepkg -s --noconfirm --skippgpcheck --needed >/dev/null 2>&1 ); then
          ok=1; break
        fi
        reason="build failed"
      else
        reason="could not fetch '$base' from the AUR"
      fi
      if (( attempt < 4 )); then
        delay=$(( attempt * 10 ))
        log "AUR bake: $n $reason; retry $attempt/3 in ${delay}s"
        sleep "$delay"
      else
        log "AUR bake: skip $n ($reason)"
      fi
    done
    if (( ok )); then built+=("$n"); else failed+=("$n"); fi
  done

  # move built packages into the cache and pull each one's runtime deps into the
  # closure, so it installs conflict-free from the baked repo on the target.
  shopt -s nullglob
  for f in "$out"/*.pkg.tar.*; do
    [[ $f == *.sig ]] && continue
    while IFS= read -r dep; do [[ -n $dep ]] && deps+=("$dep"); done \
      < <(bsdtar -xOf "$f" .PKGINFO 2>/dev/null | sed -n 's/^depend = //p' | sed 's/[<>=:].*//')
    cp -a "$f" "$CACHE"/
  done
  if (( ${#deps[@]} )); then
    # Some of these deps are themselves AUR packages that this very bake produced:
    # nvidia-580xx-dkms depends on nvidia-580xx-utils, which is another package
    # from the same pkgbase and is already sitting in the cache. pacman cannot
    # fetch those (`target not found`), so ask it only for the ones no built
    # package already provides.
    local -a want=()
    while IFS= read -r dep; do
      [[ -n $dep ]] || continue
      pkgfile_in "$CACHE" "$dep" >/dev/null && continue
      want+=("$dep")
    done < <(printf '%s\n' "${deps[@]}" | awk 'NF && !seen[$0]++')
    if (( ${#want[@]} )); then
      "${PAC[@]}" -Sw --config "$conf" --dbpath "$work/db" --cachedir "$CACHE" --noconfirm --needed "${want[@]}" \
        || log "AUR bake: note, some runtime deps did not fetch; the repo check below decides whether that matters"
    fi
  fi

  (( EUID == 0 )) && rm -f /etc/sudoers.d/99-ryoku-aurbuild
  rm -rf "$bdir"
  log "AUR bake: bundled ${#built[@]} (${built[*]:-none}); skipped ${#failed[@]} (${failed[*]:-none})"
}
# Called in a `||` context ON PURPOSE. That disables `set -e` inside the function,
# which is what this body needs: it is full of best-effort probes (an AUR RPC
# lookup that finds nothing, a clone of a package that moved) whose non-zero exit
# is normal and, under errexit + pipefail, aborted the whole build mid-bake with
# no message. Completeness is NOT enforced here for that reason: the repo check
# below asks the only question that matters -- is every package actually in the
# assembled repo -- and it runs with errexit intact, whatever this function did.
bake_aur_set || log "AUR bake: stopped early; the repo check below will catch anything missing"

# Integrity: every cached package must be a complete, readable archive before it
# goes into the repo. `pacman -Sw --needed` skips a file that already exists by
# name, and $CACHE is persistent, so a download truncated by one network hiccup
# (most likely on a big package) sits corrupt in
# the cache forever and ships in every ISO -- the "truncated <pkg>" error that
# bricks the offline pacstrap. bsdtar -tf reads the whole archive, so a short
# file fails here; delete it and re-fetch, bounded. A still-broken package fails
# the build now (a cheap rebuild), never at install time (a bricked ISO).
# Mirrors release/repo/build-repo.sh's bsdtar check.
log "verifying cached package integrity"
for _iattempt in 1 2 3; do
  corrupt=()
  for cpkg in "$CACHE"/*.pkg.tar.zst "$CACHE"/*.pkg.tar.xz; do
    [[ -e $cpkg ]] || continue
    bsdtar -tf "$cpkg" >/dev/null 2>&1 || corrupt+=("$cpkg")
  done
  if (( ${#corrupt[@]} == 0 )); then break; fi
  if (( _iattempt == 3 )); then
    die "cache integrity: package(s) still truncated after re-download: ${corrupt[*]##*/}. Remove them from $CACHE and re-run the bake (an AUR-built one needs a fresh makepkg)."
  fi
  log "cache integrity: re-downloading ${#corrupt[@]} truncated package(s): ${corrupt[*]##*/}"
  rm -f "${corrupt[@]}"
  "${PAC[@]}" -Sw --config "$conf" --dbpath "$work/db" --cachedir "$CACHE" --noconfirm --needed "${PKGS[@]}" || true
  for _v in "${nv_required[@]}" "${nv_optional[@]}" "${nv_legacy_fetch[@]}"; do
    "${PAC[@]}" -Sw --config "$conf" --dbpath "$work/db" --cachedir "$CACHE" --noconfirm --needed "$_v" 2>/dev/null || true
  done
done

# stage_offline_repo: reflink the cached packages the repo carries into DEST
# (btrfs COW, so no extra space). The cache persists across builds and can hold
# several builds of one name: an older version, or another repo's build after
# the repo a name resolves from changed (CachyOS above Arch), which can even be
# the newer one (Arch's dkms is ahead of CachyOS's). So a name a configured
# repo serves ships only as a build pacman resolved it to (RESOLVED); a name no
# repo serves is an AUR build and ships as cached, unless the CachyOS layer
# replaces it (yay-bin). Where that still leaves more than one build of a name
# (a retry's re-sync, an older AUR build), `repo-add -p` indexes the newest
# whatever order it reads them in.
stage_offline_repo() {
  local f base name url candidate pinned
  local -a keep=()
  for f in "$CACHE"/*.pkg.tar.zst "$CACHE"/*.pkg.tar.xz; do
    [[ -e $f ]] || continue
    base=${f##*/}; base=${base%.pkg.tar.*}; base=${base%-*} # NAME-VER-REL
    name=${base%-*-*}
    is_replaced "$name" && continue
    [[ -z ${SERVED[$name]:-} || -n ${RESOLVED["$name ${base#"$name"-}"]:-} ]] && keep+=("$f")
  done
  (( ${#keep[@]} )) || die "no packages in $CACHE after download"
  cp -a --reflink=auto "${keep[@]}" "$DEST"/

  # The configured repo order is deliberately the installed system's order,
  # where official/CachyOS repos precede [ryoku]. Resolve each override by its
  # qualified target, then replace any earlier-repo build staged above.
  for name in "${ryoku_pins[@]}"; do
    pinned=
    while IFS= read -r url; do
      candidate=${url%%\?*}; candidate=${candidate##*/}
      base=${candidate%.pkg.tar.*}; base=${base%-*}; base=${base%-*}; base=${base%-*}
      if [[ $base == "$name" ]]; then
        pinned="$CACHE/$candidate"
        break
      fi
    done < <("${PAC[@]}" -Sp --config "$conf" --dbpath "$work/db" --print-format '%l' "ryoku/$name")
    [[ -f $pinned ]] || die "qualified [ryoku] package '$name' was not downloaded"
    for f in "$DEST/$name"-*.pkg.tar.*; do
      [[ -e $f ]] || continue
      base=${f##*/}; base=${base%.pkg.tar.*}; base=${base%-*}; base=${base%-*}; base=${base%-*}
      [[ $base == "$name" ]] && rm -f "$f"
    done
    cp -a --reflink=auto "$pinned" "$DEST"/
  done
}

# assemble the [offline] repo, then build the db.
log "assembling the [offline] repo at $DEST"
rm -rf "$DEST"; mkdir -p "$DEST"
shopt -s nullglob
stage_offline_repo

# pkgfile_for / repo_has_pkg NAME: is package NAME baked into the repo? See
# pkgfile_in, defined above the AUR bake because the bake needs the same lookup
# against its cache.
pkgfile_for() { pkgfile_in "$DEST" "$1"; }
repo_has_pkg() { pkgfile_for "$1" >/dev/null; }
# Everything the installed system can reach has to be IN the repo, so check it
# rather than trust that the download and the AUR bake both did their jobs. The
# AUR set is included because these images are the only place it ever lands (an
# offline install never builds from the AUR), and a silent miss here is how the
# shipped ISO ended up without its UI font.
missing=()
aur_expected=()
mapfile -t aur_expected < <(read_list "$pkgdir/aur.packages" | without_replaced)
for req in "${nv_required[@]}" "${nv_legacy_fetch[@]}" nvidia-utils libva-nvidia-driver \
           mesa vulkan-radeon vulkan-intel vulkan-icd-loader \
           ryoku-keyring ryoku-desktop ryoku-desktop-hyprland ryoku-desktop-niri \
           ryoku-desktop-wayfire ryoku-rashin ryotunes ryomanager "${file_managers[@]}" \
           "${aur_expected[@]}"; do
  repo_has_pkg "$req" || missing+=("$req")
done
if (( ${#missing[@]} )); then
  [[ ${RYOKU_OFFLINE_ALLOW_INCOMPLETE:-0} == 1 ]] \
    || die "offline repo is missing required packages: ${missing[*]}. the install would leave a driverless or incomplete desktop."
  log "offline repo is missing ${missing[*]} (RYOKU_OFFLINE_ALLOW_INCOMPLETE=1)"
fi
# the AUR-built legacy NVIDIA branches are reported, not required: see the
# two-verdict note in bake_aur_set. Absent, those cards run on nouveau/mesa instead.
for req in "${nv_legacy_build[@]}"; do
  repo_has_pkg "$req" || log "note: legacy driver '$req' is not in the offline repo; pre-Turing/Kepler NVIDIA falls back to nouveau"
done
# repo-add builds offline.db(.tar.zst) + offline.files; lib/offline.sh globs it.
repo-add --quiet --prevent-downgrade "$DEST/$REPO_NAME.db.tar.zst" "$DEST"/*.pkg.tar.* >/dev/null

# verify the exact pacstrap transaction resolves and installs conflict-free from
# ONLY the baked repo. the installer folds the whole desktop set into one offline
# pacstrap (lib/offline.sh), so any two packages shipping the same path -- an
# upstream churn window like default-cursors taking over an icon file another
# package still owns -- makes pacman abort "conflicting files ... exists in
# filesystem", and offline there is no network to recover with: the ISO is a
# brick. catch it HERE, on the networked build host where a rebuild is cheap,
# not at install time. doubles as a closure-completeness check: every resolved
# dependency must be in the repo (a missing dep is the same dead end offline).
verify_offline_closure() {
  local vconf="$work/verify.conf" vdb="$work/verify-db"
  mkdir -p "$vdb"
  cat >"$vconf" <<EOF
[options]
Architecture = x86_64 x86_64_v3
SigLevel = Never
[$REPO_NAME]
SigLevel = Never
Server = file://$DEST
EOF
  "${PAC[@]}" -Sy --config "$vconf" --dbpath "$vdb" --noconfirm >/dev/null 2>&1 \
    || die "offline verify: cannot read the baked [$REPO_NAME] db at $DEST"

  # the installer's pacstrap set (lib/pacstrap.sh ryoku_pacstrap): base + dev +
  # [cachy] + BOTH microcodes (they never conflict) + the desktop set folded in
  # by ryoku_offline_pacstrap_extra. GPU drivers are the in-chroot driver step,
  # NOT pacstrap, and the nvidia module variants conflict pairwise, so they are
  # excluded here (repo_has_pkg above already proved them present in the repo).
  local -a pset=()
  mapfile -t pset < <( {
    read_list "$pkgdir/base.packages"
    read_list "$pkgdir/dev.packages"
    [[ $VARIANT == cachyos ]] && read_list "$pkgdir/cachyos.packages"
    read_section "$hw" vm
    # every variant is named so the compositor virtual is already satisfied:
    # an unqualified ryoku-desktop with several providers in the repo makes
    # pacman prompt for a choice, which a build script cannot answer.
    printf '%s\n' amd-ucode intel-ucode ryoku-keyring ryoku-desktop \
      ryoku-desktop-hyprland ryoku-desktop-niri ryoku-desktop-wayfire ryoku-rashin \
      ryotunes ryomanager "${file_managers[@]}"
  } | without_replaced | awk '!seen[$0]++' )

  local resolved
  if ! resolved=$("${PAC[@]}" -Sp --print-format '%n' --config "$vconf" --dbpath "$vdb" "${pset[@]}" 2>"$work/verify.err"); then
    log "offline verify: the pacstrap set does not yet resolve from the baked repo (a dependency is missing from the closure):
$(sed 's/^/  /' "$work/verify.err")"
    return 1
  fi

  # map each resolved name to its .pkg in the repo, then list every package's
  # files in ONE pass (pacman -Qlp = "pkgname /path", metadata excluded). a path
  # owned by more than one package is the pacstrap file conflict.
  local name f
  local -a pkgfiles=() vmissing=()
  while IFS= read -r name; do
    [[ -n $name ]] || continue
    if f=$(pkgfile_for "$name"); then
      pkgfiles+=("$f")
    else
      vmissing+=("$name")
    fi
  done <<<"$resolved"
  if (( ${#vmissing[@]} )); then
    log "offline verify: ${#vmissing[@]} resolved package(s) not yet in the baked repo: ${vmissing[*]}"
    return 1
  fi

  local conflicts
  conflicts=$(pacman -Qlp "${pkgfiles[@]}" 2>/dev/null | awk '$2 !~ /\/$/ {print $2" "$1}' | sort | awk '
    { p=$1; k=$2
      if (p!=q){ if(c>1) print q"  <=  "o; q=p; o=k; c=1; s=" "k" " }
      else if (index(s," "k" ")==0){ o=o", "k; c++; s=s k" " } }
    END{ if(c>1) print q"  <=  "o }')
  [[ -z $conflicts ]] || die "offline verify: the baked closure has file conflicts -- pacstrap would abort on the target, offline, with no way to recover:
$(printf '%s\n' "$conflicts" | sed 's/^/  /')
This is usually a transient upstream churn window (one package taking over a file another still ships). Re-run the bake once the mirrors settle, or pin/patch the offending package."

  log "offline verify: pacstrap set resolves ($(grep -c . <<<"$resolved") packages) with no file conflicts"
}
# A resolved dependency can be missing from the freshly baked repo when a mirror
# is mid-sync: its db lists a package before that package's file has propagated,
# so the closure reads as incomplete through no fault of ours. Retry with a db
# refresh and a re-download a few times, so a transient mirror window self-heals
# instead of bricking the release build; a genuinely absent package still fails.
for _vattempt in 1 2 3; do
  verify_offline_closure && break
  (( _vattempt == 3 )) && die "offline verify: the baked closure is still incomplete after $_vattempt attempts; a mirror is likely mid-sync (a dependency's db entry is ahead of its package file). Re-run the bake once the mirrors settle."
  log "offline verify: closure incomplete (attempt $_vattempt); re-syncing the db and re-downloading before retrying"
  "${PAC[@]}" -Sy --config "$conf" --dbpath "$work/db" --noconfirm >/dev/null
  read_served
  "${PAC[@]}" -Sw --config "$conf" --dbpath "$work/db" --cachedir "$CACHE" --noconfirm --needed "${PKGS[@]}"
  # restage from scratch, so the db is built from what this attempt stages
  # rather than added on top of the last attempt's.
  rm -rf "$DEST"; mkdir -p "$DEST"
  stage_offline_repo
  repo-add --quiet --prevent-downgrade "$DEST/$REPO_NAME.db.tar.zst" "$DEST"/*.pkg.tar.* >/dev/null
  sleep 20
done

# Drop every package file the db does not reference. The repo is assembled by
# copying the whole download cache, and that cache persists between local builds:
# an older build's packages sit there, get copied in, and repo-add indexes only
# the newest of each name, leaving the superseded files in the ISO as dead weight
# nobody can install. pacman's own version comparison already picked the winners,
# so trust the db and delete the rest. A CI runner starts with an empty cache and
# prunes nothing; a developer's tenth build stops shipping its first nine.
prune_superseded() {
  local db="$DEST/$REPO_NAME.db.tar.zst" keep dropped=0 f
  [[ -f $db ]] || return 0
  keep=$(bsdtar -xOf "$db" '*/desc' 2>/dev/null | awk '/^%FILENAME%$/{getline; print}')
  [[ -n $keep ]] || return 0
  for f in "$DEST"/*.pkg.tar.*; do
    [[ -f $f ]] || continue
    case "$f" in *.sig) continue ;; esac
    grep -qxF "${f##*/}" <<<"$keep" && continue
    rm -f "$f" "$f.sig"
    dropped=$((dropped + 1))
  done
  (( dropped )) && log "pruned $dropped superseded package(s) the db does not reference"
  return 0
}
prune_superseded

n=$(find "$DEST" -maxdepth 1 -name '*.pkg.tar.*' | wc -l)
sz=$(du -sh "$DEST" | cut -f1)
log "baked $n packages into the [offline] repo ($sz) at $DEST"

#!/usr/bin/env bash
# end-to-end test of the OFFLINE install path (installation/backend/lib/offline.sh
# wired into installation/backend/ryoku-install), under RYOKU_DRYRUN.
#
# Both Ryoku ISOs bake the whole package closure into a file:// [offline] repo so
# a machine with no network installs and boots. Nothing tested that. The install
# path had two ways to fail an offline user, both silent:
#
#   1. "offline" meant different things on each side. The TUI called an ISO offline
#      when the repo DIRECTORY existed; the backend needed a repo DB. A bake that
#      stopped before repo-add satisfied the first and not the second, so the
#      installer promised an offline install and the backend quietly took the
#      online path into a pacstrap with no mirror to reach.
#   2. RYOKU_ONLINE=0 with no usable repo sailed through preflight, because every
#      network step correctly skips itself when offline, and only failed at
#      pacstrap -- after the disk had already been wiped.
#
# So this pins both the happy path and the refusals, and it pins them where they
# matter: a refusal must land BEFORE the partition step, never after.
#
# Dry-run, so no disk is touched. Every network binary is replaced by a tripwire,
# which is what proves the offline path reaches the network zero times.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$here/.."
fail() { echo "FAIL: $1" >&2; exit 1; }

# The installer refuses to run without a compositor choice and the config dir
# the TUI derives for it; the offline path is compositor-agnostic, so Hyprland
# stands in.
export RYOKU_COMPOSITOR="${RYOKU_COMPOSITOR:-hyprland}"
export RYOKU_COMPOSITOR_CONFIG_DIR="${RYOKU_COMPOSITOR_CONFIG_DIR:-hypr}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# a baked repo is a directory with a repo db; an incomplete bake is the directory
# alone. these are the exact two states ryoku_offline_active distinguishes.
baked="$tmp/baked"; mkdir -p "$baked"; : >"$baked/offline.db"
nodb="$tmp/nodb"; mkdir -p "$nodb"

# tripwires: every command the install path could reach the network with. Each one
# records that it was called and fails, so a single touch is both visible and
# fatal instead of silently degrading.
trip="$tmp/bin"; mkdir -p "$trip"
for cmd in curl wget reflector getent rsync; do
  cat >"$trip/$cmd" <<EOF
#!/usr/bin/env bash
echo "NETWORK-TOUCH: $cmd \$*" >>"$tmp/touched"
exit 97
EOF
  chmod +x "$trip/$cmd"
done

# run_offline <repo> <online> [variant] [login-shell]: the backend in dry-run.
# Leaves output in $out and the exit code in $rc. PATH is prefixed with the
# tripwires.
run_offline() {
  rc=0
  out="$(PATH="$trip:$PATH" RYOKU_DRYRUN=1 RYOKU_REPO="$root" \
    RYOKU_VARIANT="${3:-plain}" RYOKU_LOGIN_SHELL="${4:-}" \
    RYOKU_ONLINE="$2" RYOKU_OFFLINE_REPO="$1" \
    RYOKU_DISK=/dev/vda RYOKU_PASSWORD_HASH='$6$fake$hash' \
    RYOKU_DISK_STRATEGY=whole RYOKU_ENCRYPT=0 RYOKU_SWAP_GIB=8 \
    bash "$root/installation/backend/ryoku-install" 2>&1)" || rc=$?
}

steps_of() { grep -oE '@@RYOKU_STEP [a-z]+' <<<"$out" | awk '{print $2}' | tr '\n' ' '; }

# ---- 1. a real baked repo installs, offline, start to finish ------------------
rm -f "$tmp/touched"
run_offline "$baked" 0
[[ $rc -eq 0 ]] || fail "offline install with a baked repo exited $rc: $out"
[[ "$(steps_of)" == "partition filesystems mount pacstrap configure bootloader " ]] \
  || fail "offline install step order is '$(steps_of)'"
[[ "$(grep -cF '@@RYOKU_DONE' <<<"$out")" -eq 1 ]] || fail "offline install did not signal done exactly once"

# The point of the whole exercise: no network, at all.
[[ ! -f $tmp/touched ]] || fail "the offline path touched the network: $(cat "$tmp/touched")"

# It must really be installing from the baked repo, not just skipping the network.
grep -q "offline: would write" <<<"$out" \
  || fail "offline install did not point pacstrap at the baked repo: $out"

# ---- 2. the desktop installs from the baked repo, staged AFTER the base --------
# ryoku-desktop pulls every shipped component. It is no longer folded into the
# base pacstrap (one bad desktop package there aborted the whole "lay the base
# system"); it installs as a separate chroot transaction from the SAME [offline]
# repo, so the base always lays and the desktop still ships, all with no network.
# both vars are read by the sourced offline.sh, which shellcheck does not follow.
# shellcheck disable=SC2034
# shellcheck source=/dev/null
( set +u; RYOKU_ONLINE=0 RYOKU_OFFLINE_REPO="$baked"
  source "$root/installation/backend/lib/offline.sh"
  extra="$(ryoku_offline_pacstrap_extra | tr '\n' ' ')"
  [[ $extra != *ryoku-desktop* ]] \
    || { echo "FAIL: desktop is still folded into the base pacstrap: '$extra'" >&2; exit 1; }
) || exit 1
# the section-1 run must still install the desktop set, from [offline], staged.
grep -qF "the baked [offline] repo" <<<"$out" \
  || fail "offline install did not stage the desktop set from the baked repo: $out"
grep -qF "ryoku-desktop" <<<"$out" \
  || fail "offline install did not install ryoku-desktop: $out"

# ---- 3. an unusable repo is refused BEFORE the disk is touched ---------------
# A bake that stopped before repo-add leaves the directory. That used to look
# offline-capable to the installer and online to the backend.
run_offline "$nodb" 0
[[ $rc -ne 0 ]] || fail "a repo with no db must not be accepted as an offline source"
[[ "$(steps_of)" == "" ]] || fail "refused too late: reached steps '$(steps_of)' (the disk is written in 'partition')"
grep -qi "no offline.db" <<<"$out" || fail "the refusal must name the missing db: $out"

run_offline "$tmp/absent" 0
[[ $rc -ne 0 ]] || fail "a missing offline repo must be refused"
[[ "$(steps_of)" == "" ]] || fail "missing-repo refusal reached steps '$(steps_of)'"
grep -qi "image is incomplete" <<<"$out" || fail "the refusal must tell the user what to do: $out"

# ---- 4. online installs are untouched ---------------------------------------
# The guard keys off RYOKU_ONLINE, so an online install must behave exactly as
# before -- including being free to use the network.
run_offline "" 1
[[ $rc -eq 0 ]] || fail "online install regressed: exit $rc: $out"
[[ "$(steps_of)" == "partition filesystems mount pacstrap configure bootloader " ]] \
  || fail "online install step order changed: '$(steps_of)'"

# ---- 5. in-chroot transactions resolve against the baked repo ALONE -----------
# pacman refuses a transaction outright -- "failed to prepare transaction (could
# not find database)" -- when ANY registered repo has no synced db, even if every
# target resolves from the one that does. In a fresh target that is core, extra,
# multilib, the CachyOS repos and [ryoku]: all registered, none synced. So the
# offline window installs against a config that registers [offline] and nothing
# else. Without this the desktop set, the GPU drivers and the AUR toolset all
# failed on real hardware while the base system looked fine.
target="$tmp/target"; mkdir -p "$target/etc/pacman.d"
sed "s#/mnt#$target#g" "$root/installation/backend/lib/offline.sh" >"$tmp/offline.sh"
# the stubs and RYOKU_OFFLINE_REPO are read by the sourced lib, which shellcheck
# does not follow (SC2329 on current shellcheck, SC2317 on the CI generation).
# shellcheck disable=SC2034,SC2329,SC2317
(
  set +u
  RYOKU_OFFLINE_REPO="$baked"
  log() { :; }
  run() { "$@"; }
  write_file() { cat >"$1"; }
  # shellcheck source=/dev/null
  source "$tmp/offline.sh"
  ryoku_offline_chroot_conf
) || fail "ryoku_offline_chroot_conf failed"
written="$target/etc/pacman.d/ryoku-offline.conf"
[[ -f $written ]] || fail "the offline window wrote no in-chroot pacman config"
repos=$(grep -oE '^\[[a-z0-9_-]+\]' "$written" | grep -v '^\[options\]$' || true)
[[ $repos == "[offline]" ]] || fail "the in-chroot offline config registers '$repos', not [offline] alone"
grep -qF "Server = file://$baked" "$written" || fail "the in-chroot offline config does not point at the baked repo"

# and every offline in-chroot install goes through it: offline.sh may not reach
# pacman directly at all, and deploy.sh's offline branch must use the wrapper
# (its online branch is free to use the target's own config).
while IFS= read -r line; do
  fail "installation/backend/lib/offline.sh installs in the chroot with an unrestricted pacman: $line"
done < <(grep -nE '^[^#]*arch-chroot /mnt pacman' "$root/installation/backend/lib/offline.sh" | grep -v -- '--config' || true)
grep -qE 'ryoku_offline_pacman -S .*"\$\{pkgs\[@\]\}"' "$root/installation/backend/lib/deploy.sh" \
  || fail "deploy.sh's offline branch does not install the desktop set through ryoku_offline_pacman"

# ---- 6. the install survives a baked desktop set older than the installer -----
# The ISO bakes the PUBLISHED ryoku-desktop, not this checkout, so a helper added
# to the repo today is missing from the target on an ISO built before the package
# published. A configure step that shells out to one of those helpers unguarded
# aborts the whole install: `ryoku-wifi-regdom set US` did exactly that, killing a
# freshly built ISO at the configure stage. Every in-chroot call to a ryoku-*
# helper must therefore be gated on it existing, or tolerate its failure.
while IFS=: read -r file lineno _; do
  # the statement, continuations folded in, and the dozen lines before it.
  stmt=$(sed -n "${lineno},$((lineno + 3))p" "$file" | sed -e :a -e '/\\$/{N;s/\\\n//;ta}' | head -1)
  ctx=$(sed -n "$(( lineno > 12 ? lineno - 12 : 1 )),${lineno}p" "$file")
  helper=$(grep -oE 'ryoku-[a-z][a-z-]*' <<<"$stmt" | head -1)
  case $stmt in
    *'||'*) continue ;;   # tolerates its own failure
  esac
  grep -qE "(-x /mnt/usr/bin/$helper|command -v $helper|chroot_has $helper)" <<<"$ctx" && continue
  fail "unguarded in-chroot call to $helper (a target whose baked desktop set predates it dies here): $file:$lineno"
done < <(grep -rnE '^[^#]*run arch-chroot /mnt ryoku-[a-z-]+' "$root/installation/backend/lib/" || true)

# ---- 7. CachyOS pacman.conf and update repositories match the CPU ------------
# The target gets CachyOS's own pacman.conf (CachyOS-Live-ISO pacman-more.conf)
# with the CPU tier's repositories above [cachyos], in the order
# cachyos-calamares' detect-architecture writes them.
# the sourced cachyos.sh calls these stubs; shellcheck can't see across source
# shellcheck disable=SC2329
(
  write_file() { cat >"$1"; }
  log() { :; }
  # shellcheck source=/dev/null
  source "$root/installation/backend/lib/cachyos.sh"

  tail=$'[cachyos]\n[core]\n[extra]\n[multilib]'
  declare -A want=(
    [v3]=$'[cachyos-v3]\n[cachyos-extra-v3]\n[cachyos-core-v3]\n'"$tail"
    [v4]=$'[cachyos-v4]\n[cachyos-extra-v4]\n[cachyos-core-v4]\n'"$tail"
    [znver4]=$'[cachyos-znver4]\n[cachyos-core-znver4]\n[cachyos-extra-znver4]\n'"$tail"
    [none]=$tail
  )
  for level in v3 v4 znver4 none; do
    conf="$tmp/cachyos-$level.conf"
    ryoku_cachyos_pacman_conf "$conf" "${level#none}"
    headers=$(grep -oE '^\[[a-z0-9_-]+\]' "$conf" | grep -v '^\[options\]$')
    [[ $headers == "${want[$level]}" ]] || fail "$level CachyOS repo order is '$headers'"
    for opt in 'Architecture = auto' Color VerbosePkgLists 'ParallelDownloads = 10' 'DownloadUser = alpm' \
               'SigLevel    = Required DatabaseOptional' \
               'NoExtract = etc/skel/.config/autostart/cachyos-hello.desktop'; do
      grep -qxF "$opt" "$conf" || fail "$level CachyOS pacman.conf lacks '$opt'"
    done
  done
  grep -qxF 'Include = /etc/pacman.d/cachyos-v4-mirrorlist' "$tmp/cachyos-znver4.conf" \
    || fail "znver4 repositories do not use the v4 mirrorlist"

  # znver4 goes to Zen 4/5 with AVX-512 only (detect-architecture's table).
  cpu() { printf 'cpu family\t: %s\nmodel\t\t: %s\nmodel name\t: x\nflags\t\t: fpu %s\n' "$1" "$2" "$3" >"$tmp/cpuinfo"; }
  cpu 25 97 'avx2 avx512f'; ryoku_cachyos_is_zen45 "$tmp/cpuinfo" || fail "Zen 4 (25/97) with AVX-512 not read as znver4"
  cpu 26 68 'avx2 avx512f'; ryoku_cachyos_is_zen45 "$tmp/cpuinfo" || fail "Zen 5 (26/68) with AVX-512 not read as znver4"
  cpu 25 33 'avx2 avx512f'; ! ryoku_cachyos_is_zen45 "$tmp/cpuinfo" || fail "Zen 3 (25/33) read as znver4"
  cpu 25 97 'avx2'; ! ryoku_cachyos_is_zen45 "$tmp/cpuinfo" || fail "a CPU without AVX-512 read as znver4"
  cpu 6 143 'avx2 avx512f'; ! ryoku_cachyos_is_zen45 "$tmp/cpuinfo" || fail "an Intel CPU read as znver4"
)

# ---- 8. a CachyOS install drops the names CachyOS replaces --------------------
# The stock kernel and headers join the drop list, which keeps them out of
# pacstrap and the update manifest after the CachyOS kernel pairs replace them.
# shellcheck disable=SC2329
(
  log() { :; }
  # shellcheck source=/dev/null
  source "$root/installation/backend/lib/choices.sh"
  # shellcheck source=/dev/null
  source "$root/installation/backend/lib/cachyos.sh"
  export RYOKU_REPO=$root
  RYOKU_DROP_PACKAGES=chromium
  ryoku_cachyos_drops
  [[ $RYOKU_DROP_PACKAGES == chromium,linux,linux-headers,yay-bin ]] || fail "CachyOS drop list is '$RYOKU_DROP_PACKAGES'"
  ryoku_cachyos_drops
  [[ $RYOKU_DROP_PACKAGES == chromium,linux,linux-headers,yay-bin ]] || fail "CachyOS drops are not idempotent: '$RYOKU_DROP_PACKAGES'"
)

# ---- 9. CachyOS identity is applied without a second first-login welcome -----
# cachyos-hooks cannot trigger itself in the pacstrap transaction that installs
# it. The edition path therefore brands the finished target explicitly and
# removes cachyos-hello's skel autostart after account creation. Plain Arch must
# do neither.
run_offline "$baked" 0 cachyos
[[ $rc -eq 0 ]] || fail "CachyOS identity dry run exited $rc: $out"
grep -qF 'run /usr/share/libalpm/scripts/cachyos-branding in the target' <<<"$out" \
  || fail "CachyOS install did not apply CachyOS branding: $out"
grep -qF 'remove etc/skel/.config/autostart/cachyos-hello.desktop from the target skel and created home directories' <<<"$out" \
  || fail "CachyOS install did not suppress CachyOS Welcome autostart: $out"

run_offline "$baked" 0 plain
[[ $rc -eq 0 ]] || fail "plain identity dry run exited $rc: $out"
! grep -qF '/usr/share/libalpm/scripts/cachyos-branding' <<<"$out" \
  || fail "plain install applied CachyOS branding: $out"
! grep -qF 'cachyos-hello.desktop' <<<"$out" \
  || fail "plain install suppressed a CachyOS-only autostart: $out"

# ---- 10. zsh installs seed the packaged startup file on every ISO flavour -----
for variant in plain cachyos; do
  run_offline "$baked" 0 "$variant" zsh
  [[ $rc -eq 0 ]] || fail "$variant zsh dry run exited $rc: $out"
  grep -qF 'seed /mnt/home/ryoku/.zshrc from /mnt/usr/share/ryoku/zsh/zshrc when missing' <<<"$out" \
    || fail "$variant zsh install did not plan the startup config seed: $out"

  target="$tmp/zsh-$variant"
  mkdir -p "$target/usr/share/ryoku/zsh" "$target/home/ryoku"
  cp "$root/ryoku/hub/backend/zshrc" "$target/usr/share/ryoku/zsh/zshrc"
  ROOT="$root" TARGET="$target" bash -c '
    # shellcheck source=/dev/null
    source "$ROOT/installation/backend/lib/common.sh"
    # shellcheck source=/dev/null
    source "$ROOT/installation/backend/lib/deploy.sh"
    RYOKU_TARGET_ROOT=$TARGET
    RYOKU_LOGIN_SHELL=zsh
    RYOKU_DRYRUN=
    RYOKU_ACCOUNT_NAMES=(ryoku)
    ryoku_seed_login_shell_config
  ' || fail "$variant zsh seed failed against the fake target"
  cmp "$root/ryoku/hub/backend/zshrc" "$target/home/ryoku/.zshrc" \
    || fail "$variant zsh seed did not copy the packaged template"

  printf 'custom zsh config\n' >"$target/home/ryoku/.zshrc"
  ROOT="$root" TARGET="$target" bash -c '
    # shellcheck source=/dev/null
    source "$ROOT/installation/backend/lib/common.sh"
    # shellcheck source=/dev/null
    source "$ROOT/installation/backend/lib/deploy.sh"
    RYOKU_TARGET_ROOT=$TARGET
    RYOKU_LOGIN_SHELL=zsh
    RYOKU_DRYRUN=
    RYOKU_ACCOUNT_NAMES=(ryoku)
    ryoku_seed_login_shell_config
  ' || fail "$variant existing zsh config check failed"
  [[ $(<"$target/home/ryoku/.zshrc") == 'custom zsh config' ]] \
    || fail "$variant zsh seed overwrote an existing config"
done

echo "install-offline: OK"

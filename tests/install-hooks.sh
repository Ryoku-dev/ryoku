#!/usr/bin/env bash
# Regression test for the two defects that broke the offline install for users:
#
#   1. The initramfs mask was laid at /mnt/etc/pacman.d/hooks/90-mkinitcpio-install.hook
#      BEFORE pacstrap. limine-mkinitcpio-hook ships that exact path, so pacman
#      aborted the whole transaction with "exists in filesystem" and no packages
#      were installed. The mask must land after pacstrap, and must move a
#      package-owned file aside rather than clobber it, so restore hands it back.
#
#   2. The retry paths ran `pacstrap -K --needed /mnt ...`. pacstrap parses only
#      short options (getopts ':C:cDGiKMNPU'), so a long option before the root
#      died with "pacstrap: invalid option -- '-'" and the real pacman error was
#      never surfaced. pacman flags belong after the root, where pacstrap passes
#      them through as targets.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$here/.."
fail() { echo "FAIL: $1" >&2; exit 1; }

# 1. ordering: the mask may not be laid before pacstrap runs.
inst="$root/installation/backend/ryoku-install"
mask_line=$(grep -n "ryoku_hooks_defer_mkinitcpio" "$inst" | grep -v '^\s*#' | head -n1 | cut -d: -f1)
strap_line=$(grep -n "^\s*ryoku_pacstrap$" "$inst" | head -n1 | cut -d: -f1)
[[ -n $mask_line && -n $strap_line ]] || fail "could not locate the mask and pacstrap calls in ryoku-install"
(( mask_line > strap_line )) \
  || fail "ryoku_hooks_defer_mkinitcpio (line $mask_line) runs before ryoku_pacstrap (line $strap_line); the mask collides with limine-mkinitcpio-hook's packaged hook"

# 1b. the reverse ordering for the hooks that do their damage DURING pacstrap:
# snap-pac (snapper with no config in the chroot) and 80-limine-efi-deploy
# (limine-install registering a stray NVRAM entry for a /boot it takes for the
# ESP). Those ship under /usr/share/libalpm/hooks, so masking them early is safe.
quiet_line=$(grep -n "^\s*ryoku_hooks_quiet$" "$inst" | head -n1 | cut -d: -f1)
[[ -n $quiet_line ]] || fail "could not locate the ryoku_hooks_quiet call in ryoku-install"
(( quiet_line < strap_line )) \
  || fail "ryoku_hooks_quiet (line $quiet_line) runs after ryoku_pacstrap (line $strap_line); the hooks then fire during pacstrap"

# 2. no pacstrap invocation may put a long option before the root argument.
while IFS= read -r line; do
  args=${line#*pacstrap }
  for tok in $args; do
    case $tok in
      /mnt|"\"\$newroot\"") break ;;
      --*) fail "pacstrap long option '$tok' precedes the root argument: $line" ;;
    esac
  done
done < <(grep -rhn "run pacstrap" "$root/installation/backend/lib/" || true)

# 3. behaviour: a package-owned hook survives mask + restore intact.
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
hooks="$tmp/etc/pacman.d/hooks"
mkdir -p "$hooks"
printf 'packaged limine hook\n' >"$hooks/90-mkinitcpio-install.hook"

# The lib writes to /mnt; source a copy pointed at the temp tree, with `run` and
# `log` stubbed, so the test needs neither root nor a real target.
sed "s#/mnt/etc/pacman.d/hooks#$hooks#g" "$root/installation/backend/lib/chroot.sh" >"$tmp/chroot.sh"
run() { "$@"; }
log() { :; }
die() { fail "$*"; }
# shellcheck source=/dev/null
source "$tmp/chroot.sh"
ryoku_hooks_defer_mkinitcpio
[[ -L $hooks/90-mkinitcpio-install.hook ]] \
  || fail "mask did not replace the hook with a /dev/null symlink"
[[ $(readlink "$hooks/90-mkinitcpio-install.hook") == /dev/null ]] \
  || fail "mask points somewhere other than /dev/null"
[[ -f $hooks/90-mkinitcpio-install.hook.ryoku-off ]] \
  || fail "the packaged hook was clobbered instead of moved aside"

ryoku_hooks_restore
[[ -f $hooks/90-mkinitcpio-install.hook && ! -L $hooks/90-mkinitcpio-install.hook ]] \
  || fail "restore did not hand the packaged hook back"
[[ $(cat "$hooks/90-mkinitcpio-install.hook") == "packaged limine hook" ]] \
  || fail "restored hook has the wrong contents"
[[ ! -e $hooks/90-mkinitcpio-install.hook.ryoku-off ]] \
  || fail "restore left the .ryoku-off copy behind"
[[ ! -e $hooks/60-mkinitcpio-remove.hook ]] \
  || fail "restore left the mkinitcpio remove mask behind"

# A package transaction can reclaim an override path while the install is still
# running. Keep that newer hook and discard only the parked stale copy.
ryoku_hooks_defer_mkinitcpio
printf 'newer limine hook\n' >"$tmp/new-hook"
mv -f "$tmp/new-hook" "$hooks/90-mkinitcpio-install.hook"
ryoku_hooks_restore
[[ $(cat "$hooks/90-mkinitcpio-install.hook") == "newer limine hook" ]] \
  || fail "restore replaced a hook written by a later package transaction"
[[ ! -e $hooks/90-mkinitcpio-install.hook.ryoku-off ]] \
  || fail "restore kept the parked hook after a package reclaimed its path"

# 4. the pacstrap-time offenders are masked and handed back.
printf 'local snap hook\n' >"$hooks/05-snap-pac-pre.hook"
for h in 05-snap-pac-pre.hook zz-snap-pac-post.hook 80-limine-efi-deploy.hook; do
  [[ " ${RYOKU_MASKED_HOOKS[*]} " == *" $h "* ]] \
    || fail "$h is not in RYOKU_MASKED_HOOKS"
done
ryoku_hooks_quiet
for h in 05-snap-pac-pre.hook zz-snap-pac-post.hook 80-limine-efi-deploy.hook; do
  [[ -L $hooks/$h && $(readlink "$hooks/$h") == /dev/null ]] \
    || fail "$h was not masked with a /dev/null symlink"
done
ryoku_hooks_restore
[[ $(cat "$hooks/05-snap-pac-pre.hook") == "local snap hook" ]] \
  || fail "restore did not preserve a local hook override"
for h in zz-snap-pac-post.hook 80-limine-efi-deploy.hook; do
  [[ ! -e $hooks/$h ]] || fail "restore left the $h mask behind"
done

echo "install-hooks: OK"

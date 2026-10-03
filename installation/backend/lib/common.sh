#!/usr/bin/env bash
# shared helpers for ryoku-install: logging, the dry-run wrapper, small utils
# every step lib leans on. sourced, never run directly.

# translation for everything log() and die() print, from the same catalog the
# desktop reads. sourced next to this file so the staged copy on the ISO keeps
# them together.
# shellcheck source=installation/backend/lib/i18n.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/i18n.sh"
i18n_init

# log: progress line on stdout. the TUI streams these into its scroll view.
# takes a format plus its arguments (`log 'locale: %s' "$RYOKU_LOCALE"`) so the
# sentence reaches the translator whole instead of pre-spliced. a lone literal
# with no arguments still works, and a stray % in one is printed as itself.
log() {
  local f=$1
  shift || true
  if (($#)); then
    printf '  %s\n' "$(tf "$f" "$@")"
  else
    printf '  %s\n' "$(t "$f")"
  fi
}

# step: staged-progress sentinel the TUI watches. ids, in order:
# partition, filesystems, mount, pacstrap, configure, bootloader. also records
# the current stage in RYOKU_STAGE so the ryoku-install ERR trap can name it;
# the printed sentinel bytes are unchanged.
# shellcheck disable=SC2034  # consumed by ryoku-install's exit trap, not here
step() { RYOKU_STAGE=$1; printf '@@RYOKU_STEP %s\n' "$1"; }

# die: abort with a stderr message + non-zero exit. same format-plus-arguments
# shape as log.
die() {
  local f=$1
  shift || true
  if (($#)); then
    printf 'ERROR: %s\n' "$(tf "$f" "$@")" >&2
  else
    printf 'ERROR: %s\n' "$(t "$f")" >&2
  fi
  exit 1
}

# run: execute a destructive/system command, or print it (prefixed DRYRUN:)
# when RYOKU_DRYRUN is set. plain argv only, no shell features. stdin is
# /dev/null on purpose: the install runs with no one at the keyboard, so a
# packaged script that asks a question (limine-mkinitcpio-hook's mkinitcpio
# wrapper, pacman's provider menu) reads EOF and moves on instead of blocking the
# install forever. Secrets go through run_secret, which keeps its stdin.
run() {
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    printf 'DRYRUN: %s\n' "$*"
    return 0
  fi
  "$@" </dev/null
}

# run_sh: same as run() for commands needing shell bits (pipes, redirects).
# pass a single string; printed verbatim under dry-run, else fed to bash -c.
run_sh() {
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    printf 'DRYRUN: %s\n' "$1"
    return 0
  fi
  bash -c "$1" </dev/null
}

# run_secret: command that reads a secret on stdin, redacted under dry-run.
# args: <label-for-dryrun> <command> [args...], secret on stdin.
run_secret() {
  local label=$1; shift
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    cat >/dev/null 2>&1 || true
    printf 'DRYRUN: %s\n' "$label"
    return 0
  fi
  "$@"
}

# write_file: write stdin to a path. dry-run prints target + content and skips
# the fs. parents are not created here.
write_file() {
  local path=$1 content
  content=$(cat)
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    printf 'DRYRUN: write %s:\n' "$path"
    printf '%s\n' "$content" | sed 's/^/        | /'
    return 0
  fi
  printf '%s\n' "$content" >"$path"
}

# append_file: same as write_file but appends. dry-run prints target + content
# and skips the fs. parents are not created here.
append_file() {
  local path=$1 content
  content=$(cat)
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    printf 'DRYRUN: append %s:\n' "$path"
    printf '%s\n' "$content" | sed 's/^/        | /'
    return 0
  fi
  printf '%s\n' "$content" >>"$path"
}

# ryoku_drop_pkg NAME: is NAME on the user's "remove from the install" list
# (RYOKU_DROP_PACKAGES, a comma-separated set the browser/apps pickers feed)?
# Empty set drops nothing, so a direct backend call keeps every package.
ryoku_drop_pkg() {
  [[ ",${RYOKU_DROP_PACKAGES:-}," == *,"$1,"* ]]
}

# ryoku_drop_from: read package names on stdin, print the ones the user kept.
# One filter point for every package-set reader (pacstrap, the AUR steps), so
# a drop never has to be honored twice in two styles.
ryoku_drop_from() {
  local p
  while IFS= read -r p; do
    [[ -n $p ]] || continue
    ryoku_drop_pkg "$p" || printf '%s\n' "$p"
  done
}

# deploy_dir: copy a source tree into a destination (dir-as-dir contents).
# missing sources -> skipped with a note in real mode; under dry-run the
# intended copy is always printed.
deploy_dir() {
  local src=$1 dst=$2
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    printf 'DRYRUN: mkdir -p %s && cp -rT %s %s\n' "$dst" "$src" "$dst"
    return 0
  fi
  [[ -d $src ]] || { log "skip: $src not present"; return 0; }
  mkdir -p "$dst"
  cp -rT "$src" "$dst"
}

# part_dev: Nth partition device for a disk. handles the nvme/mmc 'p' separator
# (nvme0n1 -> nvme0n1p2) vs plain disks (vda -> vda2).
part_dev() {
  local disk=$1 num=$2
  if [[ $disk == *[0-9] ]]; then
    printf '%sp%s' "$disk" "$num"
  else
    printf '%s%s' "$disk" "$num"
  fi
}

# part_num: trailing partition number of a partition device
# (nvme0n1p2 -> 2, sda2 -> 2, mmcblk0p1 -> 1). inverse of part_dev, used to
# register the right ESP partition with efibootmgr when its number is not 1.
part_num() {
  [[ $1 =~ ([0-9]+)$ ]] && printf '%s' "${BASH_REMATCH[1]}"
}

# dev_uuid: UUID of a block device. under dry-run the device doesn't exist, so
# return a readable placeholder instead. in real mode, command substitution
# ($(dev_uuid ...)) disables errexit, so a blkid that prints nothing (missing
# device, unformatted partition) would otherwise yield an empty UUID and an
# unbootable root=UUID=. fail non-zero on empty so consumers can catch it.
dev_uuid() {
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    printf '<UUID:%s>' "$1"
    return 0
  fi
  local uuid
  uuid=$(blkid -s UUID -o value "$1" 2>/dev/null) || true
  [[ -n $uuid ]] || return 1
  printf '%s' "$uuid"
}

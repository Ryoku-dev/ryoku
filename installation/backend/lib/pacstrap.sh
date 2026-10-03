#!/usr/bin/env bash
# shellcheck shell=bash
# Install the base system with pacstrap, then write fstab. package set =
# system/packages/base.packages + the per-profile section(s) of
# system/packages/hardware.packages (microcode + GPU drivers) + the developer
# toolchains in system/packages/dev.packages.

# read_section: prints the package lines under [section] in an INI-style file,
# skipping comments + blanks, stops at the next [section].
read_section() {
  awk -v sec="[$2]" '
    $0 == sec { f = 1; next }
    /^\[/ { f = 0 }
    f && NF && $0 !~ /^[[:space:]]*#/ { print }
  ' "$1"
}

# ryoku_ensure_keyring: make sure the live pacman keyring is ready before
# pacstrap. pacman-init.service is a oneshot that builds it at boot, so we block
# on `systemctl start` (returns only once it has finished, or immediately if it
# already ran), then populate if the keyring is still empty. without this,
# pacstrap fails to verify packages (public keyring not found / failed to
# install packages to new root).
ryoku_ensure_keyring() {
  # a oneshot's `start` blocks until it has finished, so this can't race the
  # service to pacstrap the way the old is-active poll did. failure is fine: the
  # populate fallback below still covers a keyring that never got built.
  run_sh 'systemctl start pacman-init.service 2>/dev/null || true'
  [[ -n "$(pacman-key --list-keys 2>/dev/null)" ]] && return 0
  log "initializing the pacman keyring"
  run pacman-key --init
  run pacman-key --populate archlinux
  # verify the rebuild produced keys; otherwise pacstrap dies later with a
  # cryptic "invalid or corrupted package (PGP signature)" AFTER the disk wipe.
  # fail here instead. (dry-run never really inits, so skip the check.)
  [[ -n ${RYOKU_DRYRUN:-} ]] && return 0
  [[ -n "$(pacman-key --list-keys 2>/dev/null)" ]] || \
    die "the pacman keyring is still empty after init + populate; package signatures cannot be verified. The live image's archlinux-keyring is broken; re-download or rewrite the ISO."
}

ryoku_pacstrap() {
  local base_file="$RYOKU_REPO/system/packages/base.packages"
  local hw_file="$RYOKU_REPO/system/packages/hardware.packages"
  [[ -f $base_file ]] || die 'missing package list: %s' "$base_file"
  local -a pkgs=()
  mapfile -t pkgs < <(grep -vE '^[[:space:]]*(#|$)' "$base_file")
  local -a sections=()
  case "$RYOKU_PROFILE" in
    amd) sections=(amd) ;;
    intel) sections=(intel) ;;
    amd-nvidia) sections=(amd intel nvidia) ;;
    vm) sections=(vm) ;;
    *) die 'unknown RYOKU_PROFILE: %s (want amd-nvidia|amd|intel|vm)' "$RYOKU_PROFILE" ;;
  esac

  local -a hw=()
  local sec
  for sec in "${sections[@]}"; do
    [[ -f $hw_file ]] && mapfile -t -O "${#hw[@]}" hw < <(read_section "$hw_file" "$sec")
  done
  (( ${#hw[@]} )) && pkgs+=("${hw[@]}")

  # dev toolchains ship with every machine (Go, Node/npm, Rust, Python, mise).
  local dev_file="$RYOKU_REPO/system/packages/dev.packages"
  local -a dev=()
  [[ -f $dev_file ]] && mapfile -t dev < <(grep -vE '^[[:space:]]*(#|$)' "$dev_file")
  (( ${#dev[@]} )) && pkgs+=("${dev[@]}")

  # cachyos variant: the full CachyOS layer (kernel, settings, schedulers,
  # proton), baked into the offline closure and pacstrapped in this same
  # transaction. plain installs stop at base + dev + drivers.
  if [[ ${RYOKU_VARIANT:-plain} == cachyos ]]; then
    local cachy_file="$RYOKU_REPO/system/packages/cachyos.packages"
    local -a cachy=()
    [[ -f $cachy_file ]] && mapfile -t cachy < <(grep -vE '^[[:space:]]*(#|$)' "$cachy_file")
    (( ${#cachy[@]} )) && pkgs+=("${cachy[@]}")
  fi

  # Broadcom wifi (BCM43xx) needs the out-of-tree wl driver; the in-kernel
  # b43/brcmsmac often can't associate. Arch dropped the prebuilt broadcom-wl,
  # so use broadcom-wl-dkms: the dkms hook builds wl.ko against the target's
  # linux-headers during this pacstrap (base ships dkms, base-devel and the
  # headers, so it works offline too). Add it only when a Broadcom network
  # controller (PCI vendor 14e4) is present. guard lspci's absence.
  if command -v lspci >/dev/null 2>&1 && [[ -n "$(lspci -d 14e4: 2>/dev/null)" ]]; then
    log "detected a Broadcom device (14e4:*); adding broadcom-wl-dkms to the pacstrap set"
    pkgs+=(broadcom-wl-dkms)
  fi

  # hook kept for a set the offline path may want folded into this transaction;
  # the desktop now installs separately (lib/deploy.sh), so it is empty today.
  if declare -f ryoku_offline_pacstrap_extra >/dev/null; then
    mapfile -t -O "${#pkgs[@]}" pkgs < <(ryoku_offline_pacstrap_extra)
  fi

  # the browser/apps pickers may have deselected shipped apps: filter the
  # assembled set here, the one place names become a transaction, so a
  # removed app never lands (and every list file stays the whole closure).
  local -a kept=()
  local p
  for p in "${pkgs[@]}"; do
    ryoku_drop_pkg "$p" || kept+=("$p")
  done
  (( ${#kept[@]} )) && pkgs=("${kept[@]}")

  ryoku_ensure_keyring
  log 'installing %d packages (profile=%s)' "${#pkgs[@]}" "$RYOKU_PROFILE"
  ryoku_pacstrap_install "${pkgs[@]}"

  log "writing /etc/fstab"
  run_sh "genfstab -U /mnt >> /mnt/etc/fstab"
}

# install the base package set. separated from ryoku_pacstrap so the retry paths
# are testable with stubs. two failure regimes, split by install type:
#   offline -- pacstrap draws only from the baked [offline] file:// repo, so a
#     failure is a closure defect (file conflict / corrupt package), never the
#     network. retry once with --needed, then surface the real pacman error; the
#     mirror fallback would only misreport a file conflict as a mirror problem.
#   online  -- a wifi drop, slow mirror, or corrupt download. drop the poisoned
#     cache, fall to the next mirror tier, and retry once with --needed (resuming
#     over the packages already installed, issue #21); a second failure lists the
#     tiers tried and the failing mirror URL.
ryoku_pacstrap_install() {
  local -a pkgs=("$@")
  # offline install: pacstrap from the baked file:// repo via the offline
  # pacman.conf (RYOKU_PACMAN_CONF), so the whole set installs with no network.
  local -a pconf=()
  [[ -n ${RYOKU_PACMAN_CONF:-} ]] && pconf=(-C "$RYOKU_PACMAN_CONF")
  # --noconfirm (a pacman flag, so it goes AFTER the root, where pacstrap passes
  # the rest through): without it pacman asks ":: Proceed with installation?" and,
  # when a virtual dependency has several providers in the baked repo, a
  # ":: There are N providers available for vulkan-driver" menu. A user reported an
  # install that answered that menu with its default and laid the legacy
  # nvidia-470xx driver, which then collided with the real driver in the configure
  # stage. The concrete providers are pinned in system/packages/hardware.packages,
  # so there is nothing left to ask.
  if run pacstrap "${pconf[@]}" -K /mnt --noconfirm "${pkgs[@]}"; then
    return 0
  fi

  # offline install: pacstrap draws ONLY from the baked [offline] file:// repo, so
  # a failure is never a network problem -- it is a defect in the baked closure (a
  # file conflict or a corrupt package). the mirror fallback below would be a
  # no-op that misreports the cause: users saw "across mirror tiers" for what was
  # really a "conflicting files ... exists in filesystem" abort. retry once with
  # --needed (resumes over what already extracted), then surface the REAL error.
  if declare -f ryoku_offline_active >/dev/null && ryoku_offline_active; then
    local olog; olog=$(mktemp) || olog=/dev/null
    log "pacstrap failed on the offline install; retrying once with --needed from the baked [offline] repo (no network involved)"
    if run pacstrap "${pconf[@]}" -K /mnt --noconfirm --needed "${pkgs[@]}" >"$olog" 2>&1; then
      [[ $olog == /dev/null ]] || { cat -- "$olog"; rm -f -- "$olog"; }
      return 0
    fi
    [[ $olog == /dev/null ]] || cat -- "$olog"
    local conflict="" corrupt="" conflict_msg=""
    [[ $olog == /dev/null ]] \
      || conflict=$(grep -aoE "[^ ]+ exists in filesystem" "$olog" 2>/dev/null | tail -n1) || conflict=""
    [[ $olog == /dev/null ]] \
      || corrupt=$(grep -aoE "File [^ ]+ is corrupted" "$olog" 2>/dev/null | tail -n1) || corrupt=""
    rm -f -- "$olog" 2>/dev/null || true
    # A corrupt package is not a closure defect: off the network the bad bytes
    # came from the download or the USB write, and the fix is a fresh medium,
    # not a bug report.
    if [[ -n $corrupt ]]; then
      die 'the offline install stopped on %s, which fails its recorded checksum. Every package lives on the disc, so the copy that reached the installer is bad: the ISO download or the USB write, most likely. Check the ISO checksum against the release page, rewrite the install medium, and run the installer again. If the checksum already matches, report it with /var/log/ryoku-install.log.' "$corrupt"
    fi
    [[ -n $conflict ]] && conflict_msg=$(tf ' File conflict: %s.' "$conflict")
    die 'the offline install could not lay the base system from the ISO'\''s baked package set.%s No network or mirror is involved (every package is on the disc), so this is either a defect in the baked closure or something the installer put at a path a package owns. Report the file conflict above with /var/log/ryoku-install.log; re-running the installer will not help.' "$conflict_msg"
  fi

  # online install: a wifi drop or corrupt download kills pacstrap with raw
  # pacman errors. drop the poisoned cache, regenerate the mirrorlist, and retry
  # with --needed (resume over installed packages) + --overwrite (adopt files a
  # package left orphaned when the first attempt died mid-extract, e.g. the
  # "gamemode ... exists in filesystem" conflict on a resumed install). issue #21.
  log "pacstrap failed (connection drop, slow mirror, or a package corrupt under load); clearing the target cache, dropping to the next mirror tier, and retrying once with --needed --overwrite"
  run_sh 'rm -f /mnt/var/cache/pacman/pkg/*.pkg.tar.* 2>/dev/null || true'
  ryoku_mirrors_fallback || true

  local paclog
  paclog=$(mktemp) || paclog=/dev/null
  if run pacstrap "${pconf[@]}" -K /mnt --noconfirm --needed --overwrite '*' "${pkgs[@]}" >"$paclog" 2>&1; then
    [[ $paclog == /dev/null ]] || { cat -- "$paclog"; rm -f -- "$paclog"; }
    return 0
  fi

  [[ $paclog == /dev/null ]] || cat -- "$paclog"
  local failinfo="" failinfo_msg=""
  [[ $paclog == /dev/null ]] \
    || failinfo=$(grep -aoE "failed retrieving file '?[^' ]+'? from [^ ]+" "$paclog" 2>/dev/null | tail -n1) || failinfo=""
  rm -f -- "$paclog" 2>/dev/null || true
  [[ -n $failinfo ]] && failinfo_msg=$(tf ' Last mirror error: %s.' "$failinfo")
  die 'pacstrap failed after retrying across mirror tiers (tried: %s).%s Check the connection (Wi-Fi can drop under sustained download) and re-run the installer.' "${RYOKU_MIRROR_TIERS_TRIED:-tier 1 (reflector)}" "$failinfo_msg"
}

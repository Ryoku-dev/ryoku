#!/usr/bin/env bash
# shellcheck shell=bash
# install the Ryoku desktop from the signed [ryoku] pacman repo, then materialize
# the per-user config. desktop = two packages now (ryoku-keyring, ryoku-desktop):
# the ryoku-desktop umbrella version-pins + pulls every monorepo component, so an
# old ISO survives package renames/additions. it carries binaries, hardware helper
# scripts, the Ryoku.Blobs QML plugin, the GPU udev rule, every ~/.config
# dotfile, all pacman. `ryoku materialize` then lays the dotfiles out from
# /usr/share/ryoku/config.
#
# The ISO keeps the full wallpaper library outside ryoku-desktop so normal
# package updates stay small. This stage seeds that library, delegates the
# account-owned desktop state to ryoku-host, and applies the machine-wide
# qylock and SDDM setup. It runs after chroot.sh through the dry-run wrappers
# and tolerates an offline partial install.

: "${RYOKU_TARGET_CHROOT:=arch-chroot}"

ryoku_deploy() {
  local u=$RYOKU_USERNAME h="/mnt/home/$RYOKU_USERNAME" account
  log 'deploying the Ryoku desktop for user %s' "$u"

  ryoku_deploy_repo
  ryoku_deploy_packages
  ryoku_seed_initcpio_hook
  ryoku_seed_keymap
  ryoku_seed_wallpapers "$h"
  ryoku_deploy_chown "$u"
  for account in "${RYOKU_ACCOUNT_NAMES[@]}"; do
    ryoku_deploy_seed "$account"
  done
  ryoku_deploy_qylock
}



# repo prep: add [ryoku] to /mnt/etc/pacman.conf, drop the live ISO's
# known-good mirrorlist into the target (the default mirrorlist ships every
# server commented out, so the chroot's `pacman -Sy` would otherwise have
# zero servers for core/extra), and import the release signing key so
# SigLevel=Required passes. all local, runs even offline; the repo stays
# configured for the next `ryoku update`.
ryoku_deploy_repo() {
  ryoku_repo_pacman_conf
  ryoku_repo_mirrorlist
  ryoku_repo_keyring
}

# pacman_conf: append [ryoku] once. release bucket lives under /stable/, so
# Server carries the prefix; the $arch default stays literal for pacman.
# RYOKU_REPO_SERVER / RYOKU_REPO_SIGLEVEL override the source (tests only, so a
# VM install can pull from a locally served repo instead of the public one).
ryoku_repo_pacman_conf() {
  local conf=/mnt/etc/pacman.conf
  local server="${RYOKU_REPO_SERVER:-https://repo.ryoku.dev/stable/\$arch}"
  local siglevel=${RYOKU_REPO_SIGLEVEL:-Required}
  if [[ -z ${RYOKU_DRYRUN:-} ]] && grep -q '^\[ryoku\]' "$conf" 2>/dev/null; then
    log '[ryoku] repository already present in %s' "$conf"
    return 0
  fi
  log 'adding the [ryoku] repository to %s' "$conf"
  append_file "$conf" <<EOF

[ryoku]
SigLevel = ${siglevel}
Server = ${server}
EOF
}

# mirrorlist: copy the live one over so the chroot's pacman -Sy can refresh
# core/extra. live one is what the ISO pacstrapped with, so known-good.
ryoku_repo_mirrorlist() {
  local src=/etc/pacman.d/mirrorlist dst=/mnt/etc/pacman.d/mirrorlist
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    printf 'DRYRUN: cp %s %s\n' "$src" "$dst"
    return 0
  fi
  [[ -s $src ]] || { log 'skip: no live mirrorlist at %s' "$src"; return 0; }
  mkdir -p "$(dirname "$dst")"
  cp "$src" "$dst"
}

# keyring: import the Ryoku release signing key into the target so pacman
# trusts [ryoku] before any signed pkg is fetched. key material ships in
# release/packages/ryoku-keyring. seed the three files, run pacman-key
# --populate (imports + locally signs into the trustdb), then drop the seeds
# so the ryoku-keyring package can install them without a file conflict.
# trustdb sticks.
ryoku_repo_keyring() {
  local kdir="$RYOKU_REPO/release/packages/ryoku-keyring"
  local kd=/mnt/usr/share/pacman/keyrings
  if [[ -z ${RYOKU_DRYRUN:-} && ! -f "$kdir/ryoku.gpg" ]]; then
    log 'warning: %s/ryoku.gpg missing; cannot trust [ryoku] (Ryoku packages will be skipped)' "$kdir"
    return 0
  fi
  log "importing the Ryoku release key into the target keyring"
  run install -Dm644 "$kdir/ryoku.gpg"     "$kd/ryoku.gpg"
  run install -Dm644 "$kdir/ryoku-trusted" "$kd/ryoku-trusted"
  run install -Dm644 "$kdir/ryoku-revoked" "$kd/ryoku-revoked"
  # a failed populate means pacman can never verify [ryoku] (SigLevel=Required):
  # the very next step then fails with a message that reads like a network
  # flake. stop HERE with the real cause instead; the seeds stay for a retry.
  run arch-chroot /mnt pacman-key --populate ryoku \
    || die "pacman-key --populate ryoku failed: the target cannot trust the [ryoku] repo, so the desktop set cannot install. Check /mnt/etc/pacman.d/gnupg and re-run the installer."
  run rm -f "$kd/ryoku.gpg" "$kd/ryoku-trusted" "$kd/ryoku-revoked"
}

# Print the installed packages for which [ryoku] serves a newer build. The
# qualified targets bypass pacman's first-repository rule without downgrading a
# package when an official or CachyOS repository has moved ahead.
ryoku_newer_repo_targets() {
  local installed_out served_out repo name version comparison
  local -A installed=()
  installed_out=$(arch-chroot /mnt pacman -Q --noconfirm) || return
  served_out=$(arch-chroot /mnt pacman -Sl --noconfirm ryoku) || return
  while read -r name version _; do
    [[ -n $name && -n $version ]] && installed["$name"]=$version
  done <<<"$installed_out"
  while read -r repo name version _; do
    [[ $repo == ryoku && -n ${installed[$name]:-} ]] || continue
    comparison=$(arch-chroot /mnt vercmp "$version" "${installed[$name]}") || return
    (( comparison > 0 )) && printf 'ryoku/%s\n' "$name"
  done <<<"$served_out"
}

# packages: pacman -S the desktop set from [ryoku] inside the chroot. The
# umbrella version-pins every monorepo component; hardware-only providers are
# added from the live machine's detector. Lends the live resolv.conf for DNS
# (target has none yet), same trick as aur.sh, restored after. needs net, so
# offline = skip.
# one retry covers a mid-download network flake. online, a failed install is
# fatal: no desktop without it and no ryoku CLI left to recover, so we stop
# loudly instead of booting a half-configured box.
ryoku_deploy_packages() {
  local -a pkgs=(ryoku-keyring ryoku-desktop "ryoku-desktop-$RYOKU_COMPOSITOR")
  local package
  # These apps are fresh-install defaults but remain independently removable.
  for package in ryoku-rashin ryotunes ryomanager; do
    ryoku_drop_pkg "$package" || pkgs+=("$package")
  done
  # The base set already carries Nautilus. Repeating the selected packages here
  # is harmless under --needed and also brings in Dolphin or Thunar when chosen.
  local -a file_packages=()
  local file_key="file-manager:$RYOKU_FILE_MANAGER"
  IFS=, read -ra file_packages <<<"${RYOKU_CHOICE_PACKAGES[$file_key]}"
  for package in "${file_packages[@]}"; do
    ryoku_drop_pkg "$package" || pkgs+=("$package")
  done
  local aura="$RYOKU_REPO/system/hardware/input/ryoku-hw-asus-aura"
  if [[ -x $aura ]] && "$aura"; then
    if [[ -z ${RYOKU_DRYRUN:-} ]] && arch-chroot /mnt pacman -Qq tlp >/dev/null 2>&1; then
      log "WARNING: skipping ASUS Aura lighting because TLP is installed"
    else
      pkgs+=(asusctl)
    fi
  fi
  local offline=0
  if [[ ${RYOKU_ONLINE:-1} != 1 ]] && declare -f ryoku_offline_active >/dev/null && ryoku_offline_active; then
    offline=1
  fi

  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    if (( offline )); then
      log "DRYRUN: arch-chroot /mnt pacman --config $RYOKU_OFFLINE_CHROOT_CONF -S --noconfirm --needed --noprogressbar ${pkgs[*]} (from the baked [offline] repo, no network)"
    else
      log "DRYRUN: arch-chroot /mnt pacman -Sy --noprogressbar"
      log "DRYRUN: arch-chroot /mnt pacman -S --noconfirm --needed --noprogressbar ${pkgs[*]} (one retry on a network flake)"
      log "DRYRUN: after installation, align installed packages that [ryoku] serves at a newer version"
      log "DRYRUN: would warn if $RYOKU_REPO/.payload version differs from pacman -Si ryoku-desktop"
    fi
    return 0
  fi

  if [[ ${RYOKU_ONLINE:-1} != 1 ]]; then
    if (( offline )); then
      # offline: from the baked [offline] repo, bound into the chroot by
      # ryoku_offline_chroot_on with its db already synced by pacstrap. Staged OUT
      # of the base pacstrap so a bad desktop package fails HERE, with a bootable
      # base already laid. Goes through ryoku_offline_pacman (the [offline]-ONLY
      # config): the target's own pacman.conf also registers core/extra/multilib,
      # the CachyOS repos and [ryoku], none of them synced, and pacman refuses the
      # whole transaction over that with "could not find database".
      log 'packages: installing the Ryoku desktop set from the baked [offline] repo: %s' "${pkgs[*]}"
      if ! ryoku_offline_pacman -S --noconfirm --needed --noprogressbar "${pkgs[@]}"; then
        # a re-run over a target a killed first attempt left half-extracted trips
        # "exists in filesystem"; retry once with --overwrite to adopt the orphaned
        # files (same recovery as pacstrap.sh), before giving up.
        log "offline desktop set install failed; retrying once with --overwrite to adopt any half-extracted files"
        if ! ryoku_offline_pacman -S --noconfirm --needed --noprogressbar --overwrite '*' "${pkgs[@]}"; then
          die 'the offline install laid the base system but could not install the Ryoku desktop set (%s) from the ISO'\''s baked [offline] repo. The base is bootable; check /var/log/ryoku-install.log for the failing package (a corrupt or conflicting package in the baked repo), then run '\''ryoku update'\'' once online.' "${pkgs[*]}"
        fi
      fi
      return 0
    fi
    log "packages: offline install with no baked desktop set; skipping (run 'ryoku update' once online)"
    return 0
  fi

  # lend the chroot DNS only when the target has none yet, then undo exactly that.
  local made_resolv=0
  if [[ ! -e /mnt/etc/resolv.conf ]] && cp -L /etc/resolv.conf /mnt/etc/resolv.conf 2>/dev/null; then
    made_resolv=1
  fi

  log 'installing the Ryoku desktop set: %s' "${pkgs[*]}"
  local rc=0
  # --noprogressbar: the TUI streams this output through a line viewport that the
  # \r-driven pacman bar would shred; the download still runs, just without the bar.
  if arch-chroot /mnt pacman -Sy --noprogressbar; then
    if ! arch-chroot /mnt pacman -S --noconfirm --needed --noprogressbar "${pkgs[@]}"; then
      log "desktop set install failed (a mid-download network flake, or files a killed first attempt left half-extracted); retrying once with --overwrite to adopt them"
      arch-chroot /mnt pacman -S --noconfirm --needed --noprogressbar --overwrite '*' "${pkgs[@]}" || rc=$?
    fi
  else
    rc=1
  fi
  if (( rc == 0 )); then
    case ${RYOKU_VARIANT:-} in
      plain | cachyos)
        local target_output
        local -a align_targets=()
        if ! target_output=$(ryoku_newer_repo_targets); then
          rc=1
        elif [[ -n $target_output ]]; then
          mapfile -t align_targets <<<"$target_output"
          log 'aligning newer [ryoku] package overrides: %s' "${align_targets[*]}"
          arch-chroot /mnt pacman -S --noconfirm --needed --noprogressbar "${align_targets[@]}" || rc=$?
        fi
        ;;
    esac
  fi

  if (( made_resolv == 1 )); then
    rm -f /mnt/etc/resolv.conf
  fi

  if (( rc != 0 )); then
    log "ERROR: could not install the Ryoku desktop set from [ryoku]"
    log "       (https://repo.ryoku.dev/stable). The desktop needs it and there is no"
    log "       ryoku CLI to recover with; check the network and re-run the installer."
    return 1
  fi
  ryoku_deploy_version_skew
  return 0
}

# version_skew: the ISO bakes a payload stamp ($RYOKU_REPO/.payload) with the
# VERSION it shipped. a long-lived ISO can lag the live [ryoku] repo; the install
# always pulls the repo's current ryoku-desktop, so a mismatch is informational,
# never fatal. compare the stamp's version to `pacman -Si ryoku-desktop`,
# normalizing separators and dropping the pkgrel; a missing stamp or query is a
# quiet no-op.
ryoku_deploy_version_skew() {
  local stamp="$RYOKU_REPO/.payload" baked repo baked_n repo_n
  [[ -f $stamp ]] || return 0
  baked=$(awk -F= '$1=="version"{print $2; exit}' "$stamp" 2>/dev/null || true)
  [[ -n $baked ]] || return 0
  repo=$(arch-chroot /mnt pacman -Si ryoku-desktop 2>/dev/null | awk '$1=="Version"{print $3; exit}' || true)
  [[ -n $repo ]] || return 0
  baked_n=${baked//[-_]/.}
  repo_n=${repo%-*}; repo_n=${repo_n//[-_]/.}
  [[ $baked_n == "$repo_n" ]] && return 0
  log 'WARNING: this ISO'\''s baked payload is version '\''%s'\'' but [ryoku] publishes ryoku-desktop '\''%s'\''. The installed desktop matches the repo; reinstall from a current ISO if anything looks off.' "$baked" "$repo"
}


# The HOOKS drop-in (chroot.sh) names these Ryoku hooks, and mkinitcpio aborts
# on a hook it cannot find, so the files have to be there before the bootloader
# step builds the images. ryoku-desktop owns them; this only covers an install
# that never got that set (offline with no baked desktop payload). Seeding a
# packaged path unowned is deliberate here: a box with no boot image is worse,
# and updater.ryokuOverwriteGlob lets the package adopt the copies later.
ryoku_seed_initcpio_hook() {
  local trim_src="$RYOKU_REPO/system/boot/mkinitcpio/install/ryoku-gpu-trim"
  local trim_dst=/mnt/usr/lib/initcpio/install/ryoku-gpu-trim
  local kind src dst seeded=
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    printf 'DRYRUN: install -Dm644 %s %s (only when the desktop set did not ship it)\n' "$trim_src" "$trim_dst"
    for kind in install hooks; do
      src="$RYOKU_REPO/system/boot/mkinitcpio/$kind/ryoku-console-keys"
      dst="/mnt/usr/lib/initcpio/$kind/ryoku-console-keys"
      printf 'DRYRUN: install -Dm644 %s %s (only when the desktop set did not ship it)\n' "$src" "$dst"
    done
    return 0
  fi

  if [[ ! -e $trim_dst && -f $trim_src ]]; then
    log 'seeding the ryoku-gpu-trim initramfs hook (the desktop set did not install it)'
    install -Dm644 "$trim_src" "$trim_dst"
  fi

  for kind in install hooks; do
    src="$RYOKU_REPO/system/boot/mkinitcpio/$kind/ryoku-console-keys"
    dst="/mnt/usr/lib/initcpio/$kind/ryoku-console-keys"
    if [[ ! -e $dst && -f $src ]]; then
      if [[ -z $seeded ]]; then
        log 'seeding the ryoku-console-keys initramfs hook (the desktop set did not install it)'
        seeded=1
      fi
      install -Dm644 "$src" "$dst"
    fi
  done
}

# seed the desktop keyboard layout into the neutral settings store, so the active
# compositor's `apply` renders it into the generated config: one seed works on any
# provider, present and future. desktop.json is user state no package ships, so
# this writes it directly (the later chown owns it); without it a non-us user gets
# a us session and a password typed there mismatches the install-time one.
ryoku_seed_keymap() {
  local store=/mnt/home/$RYOKU_USERNAME/.config/ryoku/desktop.json
  local xkbl=${RYOKU_XKB_LAYOUT:-} xkbv=${RYOKU_XKB_VARIANT:-}
  [[ -n $xkbl ]] || xkbl=$RYOKU_KEYMAP
  [[ $xkbl == us && -z $xkbv ]] && return 0   # the store default is already us
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log "DRYRUN: seed $store -> desktop.input.kbLayout=$xkbl kbVariant=$xkbv"
    return 0
  fi
  command -v jq >/dev/null 2>&1 || { log 'keyboard seed: skip (jq unavailable)'; return 0; }
  mkdir -p "$(dirname "$store")" || { log 'keyboard seed: skip (cannot create %s)' "$(dirname "$store")"; return 0; }
  local next
  if [[ -s $store ]]; then
    next=$(jq --arg l "$xkbl" --arg v "$xkbv" '. * {desktop:{input:{kbLayout:$l,kbVariant:$v}}}' "$store") || { log 'keyboard seed: skip (%s is not valid JSON)' "$store"; return 0; }
  else
    next=$(jq -n --arg l "$xkbl" --arg v "$xkbv" '{desktop:{input:{kbLayout:$l,kbVariant:$v}}}') || { log 'keyboard seed: skip (jq failed)'; return 0; }
  fi
  [[ -n $next ]] || { log 'keyboard seed: skip (empty jq result)'; return 0; }
  printf '%s\n' "$next" >"$store"
  log 'seeded keyboard layout into the store: %s%s' "$xkbl" "${xkbv:+ ($xkbv)}"
}

# The wallpaper library is an ISO payload, not a package payload. Keeping this
# installer-only avoids adding hundreds of megabytes to every desktop update.
ryoku_seed_wallpapers() {
  local home=$1
  log 'seeding the ISO wallpaper library into %s' "$home"
  deploy_dir "$RYOKU_REPO/ryoku/assets/wallpapers" "$home/Pictures/Wallpapers"
}

ryoku_narrate_seed_self() {
  [[ -n ${RYOKU_DRYRUN:-} ]] || return 0
  local user=$1 rice=$2 rashin=$3 hidden=${4:-} dropped=${5:-}
  local shell=$6 init=$7 locale=$8
  [[ -n $hidden ]] || hidden=none
  [[ -n $dropped ]] || dropped=none
  log 'DRYRUN: users seed-self account=%s rice=%s rashin=%s hidden-apps=%s shell=%s init=%s locale=%s provisioned-ledger=%s' \
    "$user" "$rice" "$rashin" "$hidden" "$shell" "$init" "$locale" "$dropped"
}

ryoku_deploy_seed() {
  local user=$1 shell=/usr/bin/fish id
  local -a args=(users seed-self --rice default)
  case ${RYOKU_LOGIN_SHELL:-fish} in
    zsh) shell=/usr/bin/zsh ;;
    bash) shell=/usr/bin/bash ;;
  esac
  [[ ${RYOKU_RASHIN:-1} == 1 ]] && args+=(--rashin)
  if [[ -n ${RYOKU_HIDE_APPS:-} ]]; then
    local -a hidden=()
    IFS=, read -ra hidden <<<"$RYOKU_HIDE_APPS"
    for id in "${hidden[@]}"; do args+=(--hide-app "$id"); done
  fi
  ryoku_narrate_seed_self "$user" default "${RYOKU_RASHIN:-1}" \
    "${RYOKU_HIDE_APPS:-}" "${RYOKU_DROP_PACKAGES:-}" "$shell" systemd \
    "${RYOKU_LOCALE:-C.UTF-8}"
  if [[ ! -x /mnt/usr/bin/ryoku-host && -z ${RYOKU_DRYRUN:-} ]]; then
    log 'account provisioning: skipped (ryoku-host not installed)'
    return 0
  fi
  log 'provisioning the Ryoku desktop for %s' "$user"
  run "$RYOKU_TARGET_CHROOT" /mnt runuser -u "$user" -- \
    env -i "HOME=/home/$user" "USER=$user" "LOGNAME=$user" "SHELL=$shell" \
    "PATH=/usr/local/bin:/usr/bin:/bin" "LANG=${RYOKU_LOCALE:-C.UTF-8}" \
    "RYOKU_HOST_INIT=systemd" "RYOKU_DROP_PACKAGES=${RYOKU_DROP_PACKAGES:-}" \
    /usr/bin/ryoku-host "${args[@]}"
}


# Set the selected file manager in both places desktop applications read:
# xdg-mime owns inode/directory, while the neutral desktop store owns the files
# launcher role. Install-time only, like ryoku_default_browser.
ryoku_default_file_manager() {
  local u=$RYOKU_USERNAME choice=${RYOKU_FILE_MANAGER:-nautilus}
  local desktop command store next d
  case $choice in
    nautilus)
      command=nautilus
      for d in org.gnome.Nautilus.desktop nautilus.desktop; do
        [[ -f /mnt/usr/share/applications/$d ]] && { desktop=$d; break; }
      done
      ;;
    dolphin) command=dolphin; desktop=org.kde.dolphin.desktop ;;
    thunar) command=thunar; desktop=thunar.desktop ;;
  esac
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log 'DRYRUN: set %s as the default file manager (inode/directory + desktop.apps.files) for %s if installed' "$choice" "$u"
    return 0
  fi
  if [[ -z ${desktop:-} || ! -f /mnt/usr/share/applications/$desktop ]]; then
    log 'default file manager: %s is not installed; leaving the role unchanged' "$choice"
    return 0
  fi
  if "$RYOKU_TARGET_CHROOT" /mnt runuser -u "$u" -- \
    env "HOME=/home/$u" "USER=$u" "LOGNAME=$u" \
    xdg-mime default "$desktop" inode/directory 2>/dev/null; then
    log 'default file manager: set %s (%s) for %s' "$choice" "$desktop" "$u"
  else
    log 'default file manager: warning, could not set %s for %s (continuing)' "$choice" "$u"
  fi
  store="/mnt/home/$u/.config/ryoku/desktop.json"
  if command -v jq >/dev/null 2>&1; then
    if [[ -s $store ]]; then
      next=$(jq --arg files "$command" '. * {desktop:{apps:{files:$files}}}' "$store") || next=
    else
      mkdir -p "$(dirname "$store")"
      next=$(jq -n --arg files "$command" '{desktop:{apps:{files:$files}}}') || next=
    fi
    if [[ -n $next ]]; then
      printf '%s\n' "$next" >"$store"
      chown "$u:$u" "$store" 2>/dev/null || true
      log 'file-manager role: desktop.apps.files -> %s' "$command"
    else
      log 'file-manager role: skip (could not merge %s)' "$store"
    fi
  fi
}


# The account provisioner installs qylock's user half. The installer still
# applies the machine-wide SDDM greeter from the ISO payload.
ryoku_deploy_qylock() {
  log "deploying qylock bundle + SDDM clockwork theme"
  deploy_dir "$RYOKU_REPO/ryoku/lockscreen/qylock" /mnt/usr/share/ryoku/qylock

  # stage the two installers in the chroot, run, drop. mkarchiso copies the
  # payload with --no-preserve=mode, so every script the bundle executes
  # directly needs its bit back here.
  run cp "$RYOKU_REPO/ryoku/lockscreen/sddm/setup" /mnt/root/ryoku-sddm-setup
  run cp "$RYOKU_REPO/ryoku/lockscreen/install-qylock" /mnt/root/ryoku-install-qylock
  run chmod 755 /mnt/root/ryoku-sddm-setup /mnt/root/ryoku-install-qylock \
    /mnt/usr/share/ryoku/qylock/sddm-host/install-greeter-theme
  local env="RYOKU_QYLOCK_BUNDLE=/usr/share/ryoku/qylock RYOKU_QYLOCK_MODE=live SUDO_USER=$RYOKU_USERNAME RYOKU_DRYRUN=${RYOKU_DRYRUN:-}"
  # shellcheck disable=SC2086  # env assignments are intentionally word-split
  run "$RYOKU_TARGET_CHROOT" /mnt env $env /root/ryoku-sddm-setup
  # shellcheck disable=SC2086
  run "$RYOKU_TARGET_CHROOT" /mnt env $env /root/ryoku-install-qylock
  run rm -f /mnt/root/ryoku-sddm-setup /mnt/root/ryoku-install-qylock
}

ryoku_deploy_chown() {
  local u=$1
  log 'fixing ownership of /home/%s' "$u"
  run "$RYOKU_TARGET_CHROOT" /mnt chown -R "$u:$u" "/home/$u"
}

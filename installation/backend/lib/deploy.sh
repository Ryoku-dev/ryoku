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
# a few user-data bits no package owns yet are still seeded from the repo
# payload here: brand assets + wallpapers (shell reads them from $HOME),
# ~/.npmrc prefix, the nvim default-editor registration, qylock + the SDDM
# theme. runs in the "configure" stage after chroot.sh, through the dry-run
# wrappers, and tolerates offline / partial installs (the desktop set then
# turns up on the first `ryoku update`).

ryoku_deploy() {
  local u=$RYOKU_USERNAME
  local h="/mnt/home/$u"
  log 'deploying the Ryoku desktop for user %s' "$u"

  ryoku_deploy_repo              # [ryoku] stanza + mirrorlist + keyring trust (local)
  ryoku_deploy_packages          # pacman -S the desktop set (needs net)
  ryoku_seed_initcpio_hook       # the HOOKS-named trim hook, if that set never came
  ryoku_seed_keymap              # chosen kb_layout into the neutral store
  ryoku_deploy_chown "$u"        # the store seed creates ~/.config as root; hand
                                 # it to the user before materialize writes in it
  ryoku_deploy_materialize "$u"  # `ryoku materialize` as the user
  ryoku_deploy_seed "$h"         # unpackaged: brand, wallpapers, ~/.npmrc
  ryoku_deploy_chown "$u"        # own root-seeded files before the user steps
  ryoku_deploy_qylock            # qylock writes user files as the now-owning user
}

# deploy_file: one config file, mode 644, parents made.
deploy_file() {
  local src=$1 dst=$2
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    printf 'DRYRUN: install -Dm644 %s %s\n' "$src" "$dst"
    return 0
  fi
  [[ -f $src ]] || { log 'skip: %s not present' "$src"; return 0; }
  install -Dm644 "$src" "$dst"
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

# materialize: lay the base config (/usr/share/ryoku/config, owned by
# ryoku-desktop) into ~/.config by running `ryoku materialize` as the user.
# HOME/USER/LOGNAME forced because runuser keeps root's env (same dance as
# aur.sh). skipped when the `ryoku` CLI is absent (offline / partial pkgs).
ryoku_deploy_materialize() {
  local u=$1
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log "DRYRUN: arch-chroot /mnt runuser -u $u -- env HOME=/home/$u ryoku materialize"
    return 0
  fi
  if [[ ! -x /mnt/usr/bin/ryoku ]]; then
    log "materialize: skipped (ryoku CLI not installed)"
    return 0
  fi
  log 'materializing the Ryoku config into /home/%s/.config' "$u"
  arch-chroot /mnt runuser -u "$u" -- env "HOME=/home/$u" "USER=$u" "LOGNAME=$u" \
    ryoku materialize \
    || log "materialize: warning, ryoku materialize failed (continuing)"
}

# the HOOKS drop-in (chroot.sh) names ryoku-gpu-trim, and mkinitcpio aborts on a
# hook it cannot find, so the file has to be there before the bootloader step
# builds the images. ryoku-desktop owns it; this only covers the install that
# never got that set (offline with no baked desktop payload). Seeding a packaged
# path unowned is deliberate here -- a box with no boot image is worse -- and
# updater.ryokuOverwriteGlob lets the package adopt the copy later.
ryoku_seed_initcpio_hook() {
  local src="$RYOKU_REPO/system/boot/mkinitcpio/install/ryoku-gpu-trim"
  local dst=/mnt/usr/lib/initcpio/install/ryoku-gpu-trim
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    printf 'DRYRUN: install -Dm644 %s %s (only when the desktop set did not ship it)\n' "$src" "$dst"
    return 0
  fi
  [[ -e $dst ]] && return 0            # ryoku-desktop shipped it: leave the owned file
  [[ -f $src ]] || return 0            # nothing to seed; chroot.sh already dropped the name
  log 'seeding the ryoku-gpu-trim initramfs hook (the desktop set did not install it)'
  install -Dm644 "$src" "$dst"
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

# seed the user-data nothing else owns: brand assets + wallpapers (shell
# reads them from $HOME), ~/.npmrc prefix. from the repo payload; missing
# sources are fine.
ryoku_deploy_seed() {
  local h=$1
  log 'seeding brand assets, wallpapers, decor art, and ~/.npmrc into %s' "$h"
  deploy_dir "$RYOKU_REPO/ryoku/assets/brand" "$h/.local/share/ryoku/assets/brand"
  # ship a wallpaper set so a fresh install has something to pick from;
  # ryoku-shell picks one at random on first start.
  deploy_dir "$RYOKU_REPO/ryoku/assets/wallpapers" "$h/Pictures/Wallpapers"
  # the decor art the Decor/Placard components render, beside Wallpapers and
  # livewalls so a user can see and swap it. `ryoku doctor` keeps it current.
  deploy_dir "$RYOKU_REPO/ryoku/assets/ryodecors" "$h/Pictures/ryodecors"
  deploy_file "$RYOKU_REPO/ryoku/apps/npm/npmrc" "$h/.npmrc"
}

# qylock: install the lockscreen bundle + the SDDM clockwork theme. not yet
# packaged, so the bundle ships in the payload and its two installers run
# in the chroot.
ryoku_deploy_qylock() {
  log "deploying qylock bundle + SDDM clockwork theme"
  deploy_dir "$RYOKU_REPO/ryoku/lockscreen/qylock" /mnt/usr/share/ryoku/qylock

  # stage the two installers in the chroot, run, drop.
  run cp "$RYOKU_REPO/ryoku/lockscreen/sddm/setup" /mnt/root/ryoku-sddm-setup
  run cp "$RYOKU_REPO/ryoku/lockscreen/install-qylock" /mnt/root/ryoku-install-qylock
  run chmod 755 /mnt/root/ryoku-sddm-setup /mnt/root/ryoku-install-qylock
  local env="RYOKU_QYLOCK_BUNDLE=/usr/share/ryoku/qylock SUDO_USER=$RYOKU_USERNAME RYOKU_DRYRUN=${RYOKU_DRYRUN:-}"
  # shellcheck disable=SC2086  # env assignments are intentionally word-split
  run arch-chroot /mnt env $env /root/ryoku-sddm-setup
  # shellcheck disable=SC2086
  run arch-chroot /mnt env $env /root/ryoku-install-qylock
  run rm -f /mnt/root/ryoku-sddm-setup /mnt/root/ryoku-install-qylock
}

ryoku_deploy_chown() {
  local u=$1
  log 'fixing ownership of /home/%s' "$u"
  run arch-chroot /mnt chown -R "$u:$u" "/home/$u"
}

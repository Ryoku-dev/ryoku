#!/usr/bin/env bash
# shellcheck shell=bash
# configure the freshly installed system: locale, console keymap, timezone,
# hostname, primary user, sudo, initramfs HOOKS, and (if encrypting) crypttab.
# files under /mnt are written directly; anything that must run in the target
# goes through arch-chroot.

ryoku_configure() {
  ryoku_cfg_locale
  ryoku_cfg_keymap
  ryoku_cfg_timezone
  ryoku_cfg_hostname
  ryoku_cfg_user
  ryoku_cfg_sudo
  ryoku_cfg_initramfs
  ryoku_cfg_crypttab
}

ryoku_cfg_locale() {
  log 'locale: %s' "$RYOKU_LOCALE"
  # dots escaped so en_US.UTF-8 can only match its own line.
  run sed -i "s|^#\(${RYOKU_LOCALE//./\\.} \)|\1|" /mnt/etc/locale.gen
  # a locale locale.gen does not list (a manual RYOKU_LOCALE, a slimmed file)
  # would generate nothing and leave every tool warning "cannot set locale";
  # append it so locale-gen builds exactly what locale.conf names. only when
  # the target has the source definition: locale-gen is set -e, so appending a
  # bogus name (localedef exits 4) would abort the install after the wipe,
  # strictly worse than the warning it fixes.
  if [[ -z ${RYOKU_DRYRUN:-} && $RYOKU_LOCALE == *.* ]] \
    && ! grep -q "^${RYOKU_LOCALE} " /mnt/etc/locale.gen; then
    if [[ -f /mnt/usr/share/i18n/locales/${RYOKU_LOCALE%%.*} ]]; then
      log 'locale: %s not listed in locale.gen, appending' "$RYOKU_LOCALE"
      printf '%s %s\n' "$RYOKU_LOCALE" "${RYOKU_LOCALE##*.}" >>/mnt/etc/locale.gen
    else
      log 'warn: %s has no source definition in the target; not appending (locale-gen would fail the install)' "$RYOKU_LOCALE"
    fi
  fi
  write_file /mnt/etc/locale.conf <<EOF
LANG=$RYOKU_LOCALE
EOF
  run arch-chroot /mnt locale-gen
}

ryoku_cfg_keymap() {
  log 'console keymap: %s' "$RYOKU_KEYMAP"
  write_file /mnt/etc/vconsole.conf <<EOF
KEYMAP=$RYOKU_KEYMAP
EOF
  # X11/Xwayland and the SDDM greeter never read the console keymap, so set the
  # XKB layout too. the TUI derives it from the chosen keymap (RYOKU_XKB_LAYOUT);
  # a manual install without it falls back to the keymap name. skip a plain us
  # layout (the X.Org default) so unchanged installs write no file.
  local xkbl=${RYOKU_XKB_LAYOUT:-} xkbv=${RYOKU_XKB_VARIANT:-}
  [[ -n $xkbl ]] || xkbl=$RYOKU_KEYMAP
  if [[ $xkbl != us || -n $xkbv ]]; then
    log 'X11 keyboard layout: %s%s' "$xkbl" "${xkbv:+ ($xkbv)}"
    run install -d /mnt/etc/X11/xorg.conf.d
    write_file /mnt/etc/X11/xorg.conf.d/00-keyboard.conf <<EOF
Section "InputClass"
    Identifier "system-keyboard"
    MatchIsKeyboard "on"
    Option "XkbLayout" "$xkbl"
    Option "XkbVariant" "$xkbv"
EndSection
EOF
  fi
}

ryoku_cfg_timezone() {
  local tz=$RYOKU_TIMEZONE
  if [[ $tz == auto ]]; then
    if [[ ${RYOKU_ONLINE:-1} != 1 ]]; then
      # offline install: IP geolocation can't resolve, so skip the curl (which
      # would only time out) and use UTC. saves ~20s on every offline install.
      log "timezone: offline install, using UTC (auto needs network to geolocate)"
      tz=UTC
    elif [[ -n ${RYOKU_DRYRUN:-} ]]; then
      printf 'DRYRUN: curl -fsSL https://ipinfo.io/timezone\n'
      tz='<auto-timezone>'
    else
      # configure runs with the network up, so geolocation still resolves
      # even when the installer's timezone screen ran before Wi-Fi joined.
      # two providers for resilience; first known zone wins.
      local url
      for url in "https://ipinfo.io/timezone" "http://ip-api.com/line?fields=timezone"; do
        tz=$(curl -fsSL --max-time 10 "$url" 2>/dev/null | tr -d '[:space:]') || tz=""
        if [[ -n $tz && -e /mnt/usr/share/zoneinfo/$tz ]]; then break; fi
        tz=""
      done
    fi
  fi
  if [[ -z ${RYOKU_DRYRUN:-} && ( -z $tz || ! -e /mnt/usr/share/zoneinfo/$tz ) ]]; then
    log 'warn: timezone '\''%s'\'' is empty or unknown; falling back to UTC' "${tz:-empty}"
    tz=UTC
  fi
  log 'timezone: %s' "$tz"
  run ln -sf "/usr/share/zoneinfo/$tz" /mnt/etc/localtime
  run arch-chroot /mnt hwclock --systohc
  run arch-chroot /mnt systemctl enable systemd-timesyncd.service
}

ryoku_cfg_hostname() {
  log 'hostname: %s' "$RYOKU_HOSTNAME"
  write_file /mnt/etc/hostname <<EOF
$RYOKU_HOSTNAME
EOF
  # No /etc/hosts write: that file is owned by the `filesystem` package, so
  # editing it makes pacman drop a .pacnew on every filesystem upgrade. The
  # stock nss-myhostname (default in /etc/nsswitch.conf) already resolves
  # localhost and the machine hostname, so the hand-written table is redundant.
}

ryoku_cfg_user() {
  local login_shell="/usr/bin/$RYOKU_LOGIN_SHELL"
  if [[ -z ${RYOKU_DRYRUN:-} && ! -x /mnt$login_shell ]]; then
    die "login shell %s is missing from the target; refusing to create %s with an unusable shell" "$login_shell" "$RYOKU_USERNAME"
  fi
  log 'user: %s (wheel,video,input; shell %s)' "$RYOKU_USERNAME" "$login_shell"
  # video -> write panel backlight (with 90-ryoku-backlight.rules); input ->
  # read game controllers / input devices without a per-login logind grant.
  run arch-chroot /mnt useradd -m -G wheel,video,input -s "$login_shell" "$RYOKU_USERNAME"
  # same password on the user + root, so sudo (wheel) and su both work
  # with what the installer collected. hashes go in on stdin (chpasswd -e
  # reads name:hash) and never hit the logs.
  printf '%s:%s\n' "$RYOKU_USERNAME" "$RYOKU_PASSWORD_HASH" | run_secret \
    "arch-chroot /mnt chpasswd -e (user:hash via stdin)" \
    arch-chroot /mnt chpasswd -e
  printf 'root:%s\n' "$RYOKU_PASSWORD_HASH" | run_secret \
    "arch-chroot /mnt chpasswd -e (root:hash via stdin)" \
    arch-chroot /mnt chpasswd -e
}

ryoku_cfg_sudo() {
  log "sudo: wheel group"
  write_file /mnt/etc/sudoers.d/10-wheel <<'EOF'
%wheel ALL=(ALL:ALL) ALL
EOF
  run chmod 0440 /mnt/etc/sudoers.d/10-wheel
}

ryoku_cfg_initramfs() {
  log "mkinitcpio HOOKS drop-in (/etc/mkinitcpio.conf.d/ryoku.conf)"
  local src="$RYOKU_REPO/system/boot/mkinitcpio/ryoku.conf"
  local trim_hook="$RYOKU_REPO/system/boot/mkinitcpio/install/ryoku-gpu-trim"
  local console_keys_install="$RYOKU_REPO/system/boot/mkinitcpio/install/ryoku-console-keys"
  local console_keys_hook="$RYOKU_REPO/system/boot/mkinitcpio/hooks/ryoku-console-keys"
  local content
  if [[ -f $src ]]; then
    content=$(<"$src")
  else
    content='HOOKS=(base udev ryoku-console-keys plymouth keyboard autodetect ryoku-gpu-trim microcode modconf kms keymap consolefont block encrypt resume filesystems fsck)'
  fi
  # 'encrypt' hook only matters for a LUKS root; strip it on the HOOKS line
  # only, so the word "encrypted" anywhere in the comments above survives.
  [[ ${RYOKU_ENCRYPT:-} != 1 ]] && content=$(printf '%s\n' "$content" | sed -E '/^HOOKS=/ s/ encrypt\b//')

  # ryoku-gpu-trim (a mkinitcpio install hook) keeps the denylisted nouveau, and
  # the ~100 MiB of GSP firmware it drags in, out of every kernel image.
  # ryoku-desktop owns the hook and deploy.sh seeds it when that set never
  # installs; a HOOKS entry mkinitcpio cannot find aborts the build, so drop the
  # name when even the repo has no copy to deliver.
  if [[ ! -f $trim_hook ]]; then
    log 'warning: %s missing; leaving ryoku-gpu-trim out of HOOKS' "$trim_hook"
    content=$(printf '%s\n' "$content" | sed -E '/^HOOKS=/ s/ ryoku-gpu-trim\b//')
  fi

  # Plymouth needs vconsole.conf in the image to use the installer's keyboard
  # layout, and it starts early enough that a runtime hook must set the console
  # lock state. Drop the name if the repo cannot deliver the complete pair.
  if [[ ! -f $console_keys_install || ! -f $console_keys_hook ]]; then
    log 'warning: ryoku-console-keys hook pair missing (%s, %s); leaving ryoku-console-keys out of HOOKS' \
      "$console_keys_install" "$console_keys_hook"
    content=$(printf '%s\n' "$content" | sed -E '/^HOOKS=/ s/ ryoku-console-keys\b//')
  fi

  run mkdir -p /mnt/etc/mkinitcpio.conf.d
  write_file /mnt/etc/mkinitcpio.conf.d/ryoku.conf <<<"$content"

  # NVIDIA early KMS (the mkinitcpio MODULES drop-in) is written later by
  # system/hardware/drivers/nvidia.sh (ryoku_drivers), and only when the module
  # actually built -- so a driver that fails to build can't force a broken
  # initramfs here.
}

ryoku_cfg_crypttab() {
  [[ ${RYOKU_ENCRYPT:-} == 1 ]] || return 0
  local luks_uuid
  luks_uuid=$(dev_uuid "$LUKS_PART") || die 'crypttab: could not read the LUKS UUID of %s (blkid returned nothing); refusing to write a crypttab the system cannot unlock at boot.' "$LUKS_PART"
  log 'crypttab: root -> UUID=%s' "$luks_uuid"
  write_file /mnt/etc/crypttab <<EOF
root UUID=$luks_uuid none luks
EOF
}

# pacman hooks that misbehave inside the install chroot get masked for the
# duration and restored before we finish. A /dev/null symlink is pacman's
# documented way to disable a hook by name. All of these ship under
# /usr/share/libalpm/hooks, so a mask in /etc/pacman.d/hooks never collides with
# a packaged file and can therefore be laid BEFORE pacstrap -- which is where
# they do their damage:
#   snap-pac runs `snapper` on every transaction, but the chroot has no snapper
#   config or D-Bus, so it aborts with "fatal library error, lookup self".
#   80-limine-efi-deploy (limine-mkinitcpio-hook) runs limine-install, which at
#   pacstrap time has no /etc/default/limine yet: it autodetects ESP_PATH as
#   /boot -- on the alongside layout our XBOOTLDR, not an ESP -- and registers a
#   stray "Limine" NVRAM entry pointing there. The bootloader step installs the
#   loader itself; the hook is restored for later upgrades.
RYOKU_MASKED_HOOKS=(05-snap-pac-pre.hook zz-snap-pac-post.hook 80-limine-efi-deploy.hook)

# mkinitcpio's install hooks rebuild the initramfs on every kernel/module
# package. The drivers and AUR steps would each trigger one, and the bootloader
# step's single explicit `mkinitcpio -P` (a direct command, unaffected by hook
# masking) produces the one correct image, so mask them for the rest of the
# install. Masked after pacstrap, never before: limine-mkinitcpio-hook ships
# /etc/pacman.d/hooks/90-mkinitcpio-install.hook, and a mask sitting at that
# path when pacstrap tries to extract it aborts the transaction with "exists in
# filesystem".
RYOKU_MKINITCPIO_HOOKS=(90-mkinitcpio-install.hook 60-mkinitcpio-remove.hook)

# A masked hook that a package owns is moved aside, not overwritten, so restore
# hands the packaged file back instead of deleting it.
ryoku_hooks_defer_mkinitcpio() {
  run mkdir -p /mnt/etc/pacman.d/hooks
  local h p
  for h in "${RYOKU_MKINITCPIO_HOOKS[@]}"; do
    p=/mnt/etc/pacman.d/hooks/$h
    log 'deferring the initramfs build to the bootloader step: masking %s' "$h"
    if [[ -f $p && ! -L $p ]]; then
      run mv -f "$p" "$p.ryoku-off"
    fi
    run ln -sf /dev/null "$p"
  done
}

ryoku_hooks_quiet() {
  run mkdir -p /mnt/etc/pacman.d/hooks
  local h
  for h in "${RYOKU_MASKED_HOOKS[@]}"; do
    log 'masking pacman hook for the install: %s' "$h"
    run ln -sf /dev/null "/mnt/etc/pacman.d/hooks/$h"
  done
}

ryoku_hooks_restore() {
  local h p
  for h in "${RYOKU_MASKED_HOOKS[@]}" "${RYOKU_MKINITCPIO_HOOKS[@]}"; do
    p=/mnt/etc/pacman.d/hooks/$h
    [[ -n ${RYOKU_DRYRUN:-} || -L $p || -f $p.ryoku-off ]] || continue
    log 'restoring pacman hook: %s' "$h"
    [[ -L $p ]] && run rm -f "$p"
    [[ -f $p.ryoku-off ]] && run mv -f "$p.ryoku-off" "$p"
  done
  return 0
}

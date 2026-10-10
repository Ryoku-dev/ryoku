#!/usr/bin/env bash

void_kernel_packages() {
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    printf '%s\n' linux
    return 0
  fi
  xbps-query -r /mnt -l 2>/dev/null | awk '
    $1 == "ii" {
      p=$2; sub(/-[0-9][^-]*(_[0-9]+)?$/, "", p)
      if (p ~ /^linux[0-9]+\.[0-9]+$/) print p
    }
  ' | sort -u
}

void_kernel_cmdline() {
  local root_ref cmdline
  if [[ ${RYOKU_ENCRYPT:-} == 1 ]]; then
    local luks_uuid
    luks_uuid=$(dev_uuid "$LUKS_PART") || die 'could not read LUKS UUID from %s' "$LUKS_PART"
    root_ref=/dev/mapper/root
    cmdline="root=$root_ref rootflags=subvol=@ rw rd.luks.uuid=luks-$luks_uuid"
  else
    local root_uuid
    root_uuid=$(dev_uuid "$ROOT_DEV") || die 'could not read root UUID from %s' "$ROOT_DEV"
    root_ref=UUID=$root_uuid
    cmdline="root=$root_ref rootflags=subvol=@ rw"
  fi
  [[ -z ${RYOKU_KERNEL_CMDLINE_EXTRA:-} ]] || cmdline+=" $RYOKU_KERNEL_CMDLINE_EXTRA"
  printf '%s' "$cmdline"
}

void_seed_limine_config() {
  run mkdir -p /mnt/etc/ryoku /mnt/usr/local/bin

  local branding=$RYOKU_REPO/system/boot/limine/limine.conf
  [[ -f $branding ]] || die 'Limine branding is missing: %s' "$branding"
  run cp "$branding" /mnt/etc/ryoku/limine-branding.conf

  local helper=$RYOKU_REPO/system/boot/limine/ryoku-windows-entry
  if [[ -f $helper ]]; then
    run cp "$helper" /mnt/usr/local/bin/ryoku-windows-entry
    run chmod 0755 /mnt/usr/local/bin/ryoku-windows-entry
  fi
}

void_bootloader() {
  local cmdline
  cmdline=$(void_kernel_cmdline)
  log 'kernel cmdline: %s quiet splash' "$cmdline"
  run mkdir -p /mnt/etc/kernel
  write_file /mnt/etc/kernel/cmdline <<EOF
$cmdline quiet splash
EOF

  void_seed_limine_config
  void_prepare_existing_entry

  if [[ -n ${RYOKU_DRYRUN:-} || -d /mnt/usr/share/plymouth/themes/ryoku ]]; then
    void_chroot plymouth-set-default-theme ryoku || true
  fi

  local -a kernels=()
  mapfile -t kernels < <(void_kernel_packages)
  ((${#kernels[@]})) || die "no installed Void kernel series was found for dracut"
  local kernel
  for kernel in "${kernels[@]}"; do
    log 'building dracut initramfs for %s' "$kernel"
    void_chroot xbps-reconfigure -f "$kernel"
  done

  local hook=/etc/kernel.d/post-install/50-ryoku-limine
  if [[ -z ${RYOKU_DRYRUN:-} && ! -x /mnt$hook ]]; then
    die 'packaged Limine kernel hook is missing: %s' "$hook"
  fi
  if ! run env RYOKU_LIMINE_CMDLINE="$cmdline quiet splash" \
    chroot /mnt "$hook"; then
    die 'Limine kernel hook failed while building the boot menu'
  fi

  case ${RYOKU_FIRMWARE_MODE:-uefi} in
    bios) void_install_limine_bios ;;
    uefi) void_install_limine_uefi ;;
    *) die 'unknown firmware mode: %s' "$RYOKU_FIRMWARE_MODE" ;;
  esac
}

void_prepare_existing_entry() {
  run rm -f /mnt/etc/ryoku/limine-extra.conf
  [[ ${RYOKU_DISK_STRATEGY:-} == alongside ]] || return 0
  local mode=${RYOKU_RESOLVED_ESP_MODE:-${RYOKU_ESP_MODE:-shared}}
  if [[ -n ${RYOKU_DRYRUN:-} && $mode != dedicated ]]; then
    void_install_alongside_stage1 "$(part_dev "$RYOKU_DISK" 1)"
    return 0
  fi
  local kind=${RYOKU_PF_ESP_KIND:-none} loader=${RYOKU_PF_ESP_BOOT:-none}
  local esp=${RYOKU_PF_ESP:-} partuuid title=
  [[ -n $esp && $kind != none ]] || return 0
  if [[ $kind == windows ]]; then
    title=Windows
    loader=/EFI/Microsoft/Boot/bootmgfw.efi
  elif [[ -n $loader && $loader != none && $loader != - ]]; then
    title='Linux (existing)'
  fi
  if [[ -n $title ]]; then
    partuuid=$(blkid -s PARTUUID -o value "$esp" 2>/dev/null || true)
    [[ -n $partuuid ]] || die 'could not read PARTUUID for the existing ESP %s' "$esp"
    write_file /mnt/etc/ryoku/limine-extra.conf <<EOF
/$title
    protocol: efi_chainload
    image_path: guid($partuuid):$loader
EOF
  fi
  if [[ $mode != dedicated ]]; then
    void_install_alongside_stage1 "$esp"
  fi
}

void_install_alongside_stage1() {
  local esp=$1 uuid partnum
  log 'installing the static Limine stage on the shared ESP %s' "$esp"
  run mkdir -p /mnt/efi /mnt/var/backups/ryoku
  run mount "$esp" /mnt/efi
  run_sh "tar -C /mnt/efi -cf /mnt/var/backups/ryoku/shared-esp-before-ryoku.tar ."
  run mkdir -p /mnt/efi/EFI/ryoku
  run cp /mnt/usr/share/limine/BOOTX64.EFI /mnt/efi/EFI/ryoku/BOOTX64.EFI
  run cp /mnt/usr/share/limine/BOOTX64.EFI /mnt/boot/ryoku-limine.efi
  write_file /mnt/efi/EFI/ryoku/limine.conf <<EOF
timeout: 0
default_entry: 1
/Ryoku
    protocol: efi_chainload
    image_path: fslabel(${RYOKU_ALONGSIDE_BOOT_LABEL:-RYOKUBOOT}):/ryoku-limine.efi
EOF
  if [[ -n ${RYOKU_DRYRUN:-} || ! -e /mnt/efi/EFI/BOOT/BOOTX64.EFI ]]; then
    run mkdir -p /mnt/efi/EFI/BOOT
    run cp /mnt/usr/share/limine/BOOTX64.EFI /mnt/efi/EFI/BOOT/BOOTX64.EFI
  fi
  uuid=$(dev_uuid "$esp") || die 'could not read filesystem UUID for shared ESP %s' "$esp"
  append_file /mnt/etc/fstab <<EOF
UUID=$uuid	/efi	vfat	defaults,nofail,noatime	0 2
EOF
  partnum=$(part_num "$esp")
  run efibootmgr --create --disk "$RYOKU_DISK" --part "$partnum" --label Ryoku \
    --loader '\EFI\ryoku\BOOTX64.EFI' --unicode || \
    log 'warning: firmware rejected the Ryoku NVRAM entry; the fallback loader remains available'
  run umount /mnt/efi
}

void_install_limine_uefi() {
  local mode=${RYOKU_RESOLVED_ESP_MODE:-${RYOKU_ESP_MODE:-shared}}
  if [[ ${RYOKU_DISK_STRATEGY:-} == alongside && $mode != dedicated ]]; then
    return 0
  fi
  log "installing Limine UEFI loader and removable fallback"
  run mkdir -p /mnt/boot/EFI/BOOT /mnt/boot/EFI/ryoku
  run cp /mnt/usr/share/limine/BOOTX64.EFI /mnt/boot/EFI/BOOT/BOOTX64.EFI
  run cp /mnt/usr/share/limine/BOOTX64.EFI /mnt/boot/EFI/ryoku/BOOTX64.EFI
  local partnum
  partnum=$(part_num "$ESP_DEV")
  run efibootmgr --create --disk "$RYOKU_DISK" --part "$partnum" --label Ryoku \
    --loader '\EFI\ryoku\BOOTX64.EFI' --unicode || \
    log 'warning: firmware rejected the Ryoku NVRAM entry; EFI/BOOT/BOOTX64.EFI remains bootable'
  local helper=$RYOKU_REPO/system/boot/limine/ryoku-windows-entry
  if [[ -z ${RYOKU_DRYRUN:-} && -x $helper ]]; then
    "$helper" sync /mnt/boot/limine.conf >/dev/null 2>&1 || true
  fi
}

void_install_limine_bios() {
  [[ ${RYOKU_DISK_STRATEGY:-} == whole ]] \
    || die "legacy BIOS installation supports whole-disk mode only"
  local bios_part=${RYOKU_BIOS_PART:-} partnum
  [[ -n $bios_part || -n ${RYOKU_DRYRUN:-} ]] \
    || die "the BIOS boot partition was not created"
  partnum=$(part_num "${bios_part:-$(part_dev "$RYOKU_DISK" 2)}")
  log 'installing Limine BIOS stage to %s using GPT BIOS partition %s' "$RYOKU_DISK" "$partnum"
  run mkdir -p /mnt/boot/limine
  run cp /mnt/usr/share/limine/limine-bios.sys /mnt/boot/limine/limine-bios.sys
  void_chroot limine bios-install "$RYOKU_DISK" "$partnum"
}

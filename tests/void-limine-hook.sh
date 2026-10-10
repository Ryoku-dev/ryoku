#!/usr/bin/env bash
set -euo pipefail

repo=${RYOKU_PATH:-$(cd "$(dirname "$0")/.." && pwd)}
hook=$repo/system/boot/limine/50-ryoku-limine
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

absent_boot=$tmp/absent/boot
RYOKU_LIMINE_BOOT_DIR=$absent_boot \
RYOKU_LIMINE_CONF=$absent_boot/limine.conf \
RYOKU_LIMINE_BRANDING_FILE=$tmp/missing-branding.conf \
  "$hook"
[[ ! -e $absent_boot/limine.conf ]]

boot=$tmp/boot
mkdir -p "$boot" "$tmp/etc"
: >"$boot/vmlinuz-6.12.30_1"
: >"$boot/initramfs-6.12.30_1.img"
: >"$boot/vmlinuz-6.6.90_1"
: >"$boot/initramfs-6.6.90_1.img"
: >"$boot/vmlinuz-6.1.1_1"

cat >"$tmp/branding.conf" <<'EOF'
# Comments from Arch's source menu are not valid Void menu branding.
timeout: 7
default_entry: 1
interface_branding: Ryoku Test
/Ryoku Linux
    protocol: efi
EOF
cat >"$tmp/extra.conf" <<'EOF'
/Windows
    protocol: efi_chainload
    image_path: guid(abc):/EFI/Microsoft/Boot/bootmgfw.efi
EOF

RYOKU_LIMINE_BOOT_DIR=$boot \
RYOKU_LIMINE_CONF=$boot/limine.conf \
RYOKU_LIMINE_BRANDING_FILE=$tmp/branding.conf \
RYOKU_LIMINE_EXTRA_FILE=$tmp/extra.conf \
RYOKU_WINDOWS_HELPER=/nonexistent \
RYOKU_LIMINE_LOADER=/nonexistent \
RYOKU_LIMINE_CMDLINE='root=UUID=test rw rd.luks.uuid=luks-crypt' \
  "$hook"

grep -q '^interface_branding: Ryoku Test$' "$boot/limine.conf"
! grep -q '^[[:space:]]*#' "$boot/limine.conf"
grep -q '^/Ryoku Linux 6.12.30_1$' "$boot/limine.conf"
grep -q '^/Ryoku Linux 6.6.90_1$' "$boot/limine.conf"
! grep -q '6.1.1_1' "$boot/limine.conf"
grep -q 'path: boot():/vmlinuz-6.12.30_1' "$boot/limine.conf"
grep -q 'module_path: boot():/initramfs-6.12.30_1.img' "$boot/limine.conf"
grep -q 'rd.luks.uuid=luks-crypt' "$boot/limine.conf"
grep -q '^/Windows$' "$boot/limine.conf"
first=$(grep '^/Ryoku Linux ' "$boot/limine.conf" | head -n1)
[[ $first == '/Ryoku Linux 6.12.30_1' ]]

printf 'root=UUID=from-file rw console=ttyS0\n' >"$tmp/etc/kernel-cmdline"
unset RYOKU_LIMINE_CMDLINE
RYOKU_LIMINE_BOOT_DIR=$boot \
RYOKU_LIMINE_CONF=$boot/limine.conf \
RYOKU_LIMINE_BRANDING_FILE=$tmp/branding.conf \
RYOKU_LIMINE_EXTRA_FILE=/nonexistent \
RYOKU_WINDOWS_HELPER=/nonexistent \
RYOKU_LIMINE_LOADER=/nonexistent \
RYOKU_LIMINE_CMDLINE_FILE=$tmp/etc/kernel-cmdline \
  "$hook" linux6.12 6.12.30_1
grep -q 'cmdline: root=UUID=from-file rw console=ttyS0' "$boot/limine.conf"
! grep -q '^/Windows$' "$boot/limine.conf"

rm "$boot/initramfs-6.12.30_1.img"
RYOKU_LIMINE_BOOT_DIR=$boot \
RYOKU_LIMINE_CONF=$boot/limine.conf \
RYOKU_LIMINE_BRANDING_FILE=$tmp/branding.conf \
RYOKU_LIMINE_EXTRA_FILE=/nonexistent \
RYOKU_WINDOWS_HELPER=/nonexistent \
RYOKU_LIMINE_LOADER=/nonexistent \
RYOKU_LIMINE_CMDLINE_FILE=$tmp/etc/kernel-cmdline \
  "$hook" linux6.12 6.12.30_1
! grep -q '6.12.30_1' "$boot/limine.conf"
grep -q '6.6.90_1' "$boot/limine.conf"

fresh=$tmp/fresh
fresh_boot=$fresh/boot
mkdir -p "$fresh_boot"
: >"$fresh_boot/vmlinuz-6.12.30_1"
: >"$fresh_boot/initramfs-6.12.30_1.img"
printf 'current-loader\n' >"$fresh/BOOTX64.EFI"

RYOKU_LIMINE_BOOT_DIR=$fresh_boot \
RYOKU_LIMINE_CONF=$fresh_boot/limine.conf \
RYOKU_LIMINE_BRANDING_FILE=$tmp/branding.conf \
RYOKU_LIMINE_EXTRA_FILE=/nonexistent \
RYOKU_WINDOWS_HELPER=/nonexistent \
RYOKU_LIMINE_LOADER=$fresh/BOOTX64.EFI \
RYOKU_LIMINE_SHARED_ESP=$fresh/shared \
RYOKU_LIMINE_CMDLINE='root=UUID=fresh rw' \
  "$hook"
[[ -f $fresh_boot/limine.conf ]]
[[ ! -e $fresh_boot/EFI/BOOT/BOOTX64.EFI ]]

mkdir -p "$fresh_boot/EFI/BOOT"
printf 'stale-loader\n' >"$fresh_boot/EFI/BOOT/BOOTX64.EFI"
RYOKU_LIMINE_BOOT_DIR=$fresh_boot \
RYOKU_LIMINE_CONF=$fresh_boot/limine.conf \
RYOKU_LIMINE_BRANDING_FILE=$tmp/branding.conf \
RYOKU_LIMINE_EXTRA_FILE=/nonexistent \
RYOKU_WINDOWS_HELPER=/nonexistent \
RYOKU_LIMINE_LOADER=$fresh/BOOTX64.EFI \
RYOKU_LIMINE_SHARED_ESP=$fresh/shared \
RYOKU_LIMINE_CMDLINE='root=UUID=fresh rw' \
  "$hook"
cmp "$fresh/BOOTX64.EFI" "$fresh_boot/EFI/BOOT/BOOTX64.EFI"

printf 'void limine hook: ok\n'

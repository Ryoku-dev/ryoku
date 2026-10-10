#!/usr/bin/env bash
set -euo pipefail

repo=${RYOKU_PATH:-$(cd "$(dirname "$0")/.." && pwd)}
hook=$repo/system/boot/limine/50-ryoku-limine
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

assert_contains() {
  local needle=$1 file=$2
  grep -Fq -- "$needle" "$file" || {
    printf 'missing %q in %s\n' "$needle" "$file" >&2
    return 1
  }
}

assert_not_contains() {
  local needle=$1 file=$2
  if grep -Fq -- "$needle" "$file"; then
    printf 'unexpected %q in %s\n' "$needle" "$file" >&2
    return 1
  fi
}

base_path=$PATH
no_lss_bin=$tmp/no-lss-bin
mkdir -p "$no_lss_bin"
for command_name in bash mkdir dirname mktemp rm awk sort cat chmod mv cp; do
  ln -s "$(command -v "$command_name")" "$no_lss_bin/$command_name"
done

absent_boot=$tmp/absent/boot
PATH=$no_lss_bin \
RYOKU_LIMINE_BOOT_DIR=$absent_boot \
RYOKU_LIMINE_CONF=$absent_boot/limine.conf \
RYOKU_LIMINE_BRANDING_FILE=$tmp/missing-branding.conf \
  "$hook"
[[ ! -e $absent_boot/limine.conf ]]

boot=$tmp/boot
modules=$tmp/modules
mkdir -p "$boot" "$modules/6.12.30_1" "$modules/6.6.90_1" "$modules/6.14.1_1"
: >"$boot/vmlinuz-6.14.1_1"
: >"$boot/initramfs-6.14.1_1.img"
rm -rf "$modules/6.14.1_1"
: >"$boot/vmlinuz-6.12.30_1"
: >"$boot/initramfs-6.12.30_1.img"
: >"$boot/vmlinuz-6.6.90_1"
: >"$boot/initramfs-6.6.90_1.img"
: >"$boot/vmlinuz-6.1.1_1"

cat >"$tmp/branding.conf" <<'EOF'
# Comments from Arch's source menu are not valid Void menu branding.
timeout: 7
default_entry: 1
remember_last_entry: yes
interface_branding: Ryoku Test
/Ryoku Linux
    protocol: efi
EOF
cat >"$tmp/extra.conf" <<'EOF'
/Windows
    protocol: efi_chainload
    image_path: guid(abc):/EFI/Microsoft/Boot/bootmgfw.efi
EOF
cat >"$boot/limine.conf" <<'EOF'
timeout: 2
default_entry: 1
/Ryoku Linux 6.0.1_1
    protocol: linux
    path: boot():/vmlinuz-6.0.1_1
EOF

PATH=$no_lss_bin \
RYOKU_LIMINE_BOOT_DIR=$boot \
RYOKU_LIMINE_CONF=$boot/limine.conf \
RYOKU_LIMINE_BRANDING_FILE=$tmp/branding.conf \
RYOKU_LIMINE_EXTRA_FILE=$tmp/extra.conf \
RYOKU_WINDOWS_HELPER=/nonexistent \
RYOKU_LIMINE_LOADER=/nonexistent \
RYOKU_MODULES_DIR=$modules \
RYOKU_LIMINE_CMDLINE='root=/dev/mapper/root rootflags=subvol=@ rw rd.luks.uuid=luks-test' \
  "$hook"

conf=$boot/limine.conf
assert_contains 'interface_branding: Ryoku Test' "$conf"
assert_contains 'default_entry: Ryoku Linux/linux6.12' "$conf"
[[ $(grep -c '^default_entry:' "$conf") -eq 1 ]]
[[ $(grep -c '^/Ryoku Linux$' "$conf") -eq 1 ]]
if [[ $(awk '/^\/Ryoku Linux$/ { getline; print; exit }' "$conf") != '  //linux6.12' ]]; then
  printf 'the Ryoku Linux directory must be followed directly by its first kernel entry\n' >&2
  exit 1
fi
assert_contains '  //linux6.12' "$conf"
assert_contains '  comment: Kernel version: 6.12.30_1' "$conf"
assert_contains '  comment: kernel-id=linux6.12' "$conf"
assert_contains '  //linux6.6' "$conf"
assert_not_contains '6.14.1_1' "$conf"
assert_not_contains '6.1.1_1' "$conf"
assert_not_contains '/Ryoku Linux 6.0.1_1' "$conf"
assert_not_contains '//Snapshots' "$conf"
assert_contains 'path: boot():/vmlinuz-6.12.30_1' "$conf"
assert_contains 'module_path: boot():/initramfs-6.12.30_1.img' "$conf"
assert_contains 'cmdline: root=/dev/mapper/root rootflags=subvol=@ rw rd.luks.uuid=luks-test' "$conf"
assert_contains '/Windows' "$conf"
if grep -q '^[[:space:]]*#' "$conf"; then
  printf 'branding comments leaked into %s\n' "$conf" >&2
  exit 1
fi
line_new=$(grep -n '^  //linux6\.12$' "$conf")
line_old=$(grep -n '^  //linux6\.6$' "$conf")
[[ ${line_new%%:*} -lt ${line_old%%:*} ]]

fallback=$tmp/fallback
mkdir -p "$fallback/boot" "$fallback/modules/custom+debug"
: >"$fallback/boot/vmlinuz-custom+debug"
: >"$fallback/boot/initramfs-custom+debug.img"
PATH=$no_lss_bin \
RYOKU_LIMINE_BOOT_DIR=$fallback/boot \
RYOKU_LIMINE_CONF=$fallback/boot/limine.conf \
RYOKU_LIMINE_BRANDING_FILE=$tmp/branding.conf \
RYOKU_LIMINE_EXTRA_FILE=/nonexistent \
RYOKU_WINDOWS_HELPER=/nonexistent \
RYOKU_LIMINE_LOADER=/nonexistent \
RYOKU_MODULES_DIR=$fallback/modules \
RYOKU_LIMINE_CMDLINE='root=UUID=fallback rw' \
  "$hook"
assert_contains 'default_entry: Ryoku Linux/linux-custom_debug' "$fallback/boot/limine.conf"
assert_contains '  //linux-custom_debug' "$fallback/boot/limine.conf"

fake_bin=$tmp/fake-bin
mkdir -p "$fake_bin" "$tmp/etc/snapper/configs"
: >"$tmp/etc/snapper/configs/root"
flock_log=$tmp/flock.log
sync_log=$tmp/sync.log
lock_state_log=$tmp/lock-state.log

cat >"$fake_bin/flock" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$RYOKU_TEST_FLOCK_LOG"
exec /usr/bin/flock "$@"
EOF
cat >"$fake_bin/limine-snapper-sync" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$RYOKU_TEST_SYNC_LOG"
if /usr/bin/flock -n "$RYOKU_LIMINE_LOCK_FILE" -c true; then
  printf 'unlocked\n' >>"$RYOKU_TEST_LOCK_STATE_LOG"
  exit 90
fi
printf 'held\n' >>"$RYOKU_TEST_LOCK_STATE_LOG"
EOF
chmod +x "$fake_bin/flock" "$fake_bin/limine-snapper-sync"

common_snapshot_env=(
  PATH="$fake_bin:$base_path"
  RYOKU_LIMINE_BOOT_DIR="$boot"
  RYOKU_LIMINE_CONF="$conf"
  RYOKU_LIMINE_BRANDING_FILE="$tmp/branding.conf"
  RYOKU_LIMINE_EXTRA_FILE="$tmp/extra.conf"
  RYOKU_WINDOWS_HELPER=/nonexistent
  RYOKU_LIMINE_LOADER=/nonexistent
  RYOKU_MODULES_DIR="$modules"
  RYOKU_SNAPPER_ROOT_CONFIG="$tmp/etc/snapper/configs/root"
  RYOKU_LIMINE_LOCK_FILE="$tmp/boot-partition.lock"
  RYOKU_TEST_FLOCK_LOG="$flock_log"
  RYOKU_TEST_SYNC_LOG="$sync_log"
  RYOKU_TEST_LOCK_STATE_LOG="$lock_state_log"
  RYOKU_LIMINE_CMDLINE='root=UUID=test rootflags=subvol=@ rw'
)

env "${common_snapshot_env[@]}" "$hook"
assert_contains '  //Snapshots' "$conf"
assert_contains '--no-mutex' "$sync_log"
assert_contains 'held' "$lock_state_log"
assert_not_contains 'unlocked' "$lock_state_log"
assert_contains '--timeout=30' "$flock_log"
assert_contains '--unlock' "$flock_log"

cat >"$conf" <<'EOF'
timeout: 7
default_entry: Ryoku Linux/linux6.12
/Ryoku Linux

  //linux6.12
  protocol: linux
  path: boot():/vmlinuz-6.12.30_1
  module_path: boot():/initramfs-6.12.30_1.img
  cmdline: root=UUID=test rootflags=subvol=@ rw

  //Snapshots
  ### Auto-generated by limine-snapper-sync
  comment: 1 / 1 snapshots
  ///2026-10-10 02:53:22
  comment: 41 pre update
  ////linux6.12
  comment: Kernel version: 6.12.30_1
  protocol: linux
  path: boot():/machine/limine_history/vmlinuz
  module_path: boot():/machine/limine_history/initramfs
  cmdline: root=UUID=test rootflags=subvol=/@snapshots/41/snapshot rw

/Old flat neighbor
  protocol: linux
EOF

extract_snapshots() {
  awk '
    !found {
      if ($0 ~ /^[[:space:]]*\/\/Snapshots[[:space:]]*$/) {
        found = 1
        print
      }
      next
    }
    /^[[:space:]]*\/[^\/]/ { exit }
    /^[[:space:]]*\/\/[^\/]/ { exit }
    { print }
  ' "$1"
}

extract_snapshots "$conf" >"$tmp/snapshots.before"
mkdir -p "$modules/6.14.1_1"
env "${common_snapshot_env[@]}" "$hook"
extract_snapshots "$conf" >"$tmp/snapshots.after-add"
cmp "$tmp/snapshots.before" "$tmp/snapshots.after-add"
assert_contains 'default_entry: Ryoku Linux/linux6.14' "$conf"
assert_contains '  //linux6.14' "$conf"
assert_not_contains '/Old flat neighbor' "$conf"

rm "$boot/initramfs-6.14.1_1.img"
env "${common_snapshot_env[@]}" "$hook"
extract_snapshots "$conf" >"$tmp/snapshots.after-remove"
cmp "$tmp/snapshots.before" "$tmp/snapshots.after-remove"
assert_contains 'default_entry: Ryoku Linux/linux6.12' "$conf"
assert_not_contains '6.14.1_1' "$conf"

printf 'root=UUID=from-file rootflags=subvol=@ rw console=ttyS0\n' >"$tmp/etc/kernel-cmdline"
file_cmdline_env=("${common_snapshot_env[@]}")
unset 'file_cmdline_env[${#file_cmdline_env[@]}-1]'
file_cmdline_env+=(RYOKU_LIMINE_CMDLINE_FILE="$tmp/etc/kernel-cmdline")
env "${file_cmdline_env[@]}" "$hook" linux6.12 6.12.30_1
assert_contains 'cmdline: root=UUID=from-file rootflags=subvol=@ rw console=ttyS0' "$conf"

fresh=$tmp/fresh
fresh_boot=$fresh/boot
mkdir -p "$fresh_boot" "$fresh/modules/6.12.30_1"
: >"$fresh_boot/vmlinuz-6.12.30_1"
: >"$fresh_boot/initramfs-6.12.30_1.img"
printf 'current-loader\n' >"$fresh/BOOTX64.EFI"

PATH=$no_lss_bin \
RYOKU_LIMINE_BOOT_DIR=$fresh_boot \
RYOKU_LIMINE_CONF=$fresh_boot/limine.conf \
RYOKU_LIMINE_BRANDING_FILE=$tmp/branding.conf \
RYOKU_LIMINE_EXTRA_FILE=/nonexistent \
RYOKU_WINDOWS_HELPER=/nonexistent \
RYOKU_LIMINE_LOADER=$fresh/BOOTX64.EFI \
RYOKU_LIMINE_SHARED_ESP=$fresh/shared \
RYOKU_MODULES_DIR=$fresh/modules \
RYOKU_LIMINE_CMDLINE='root=UUID=fresh rootflags=subvol=@ rw' \
  "$hook"
[[ -f $fresh_boot/limine.conf ]]
[[ ! -e $fresh_boot/EFI/BOOT/BOOTX64.EFI ]]

mkdir -p "$fresh_boot/EFI/BOOT"
printf 'stale-loader\n' >"$fresh_boot/EFI/BOOT/BOOTX64.EFI"
PATH=$no_lss_bin \
RYOKU_LIMINE_BOOT_DIR=$fresh_boot \
RYOKU_LIMINE_CONF=$fresh_boot/limine.conf \
RYOKU_LIMINE_BRANDING_FILE=$tmp/branding.conf \
RYOKU_LIMINE_EXTRA_FILE=/nonexistent \
RYOKU_WINDOWS_HELPER=/nonexistent \
RYOKU_LIMINE_LOADER=$fresh/BOOTX64.EFI \
RYOKU_LIMINE_SHARED_ESP=$fresh/shared \
RYOKU_MODULES_DIR=$fresh/modules \
RYOKU_LIMINE_CMDLINE='root=UUID=fresh rootflags=subvol=@ rw' \
  "$hook"
cmp "$fresh/BOOTX64.EFI" "$fresh_boot/EFI/BOOT/BOOTX64.EFI"

printf 'void limine hook: ok\n'

#!/usr/bin/env bash
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=tests/install-dryrun-harness.sh
source "$here/install-dryrun-harness.sh"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
extra_users=$tmp/extra-users.tsv
printf 'alice\tAlice Example\tstandard\taudio\tx\t1073741824\nbob\tBob Example\tadmin\tdevices,virtualization,containers\tx\t0\n' >"$extra_users"
fail() {
  printf 'installer edition parity: %s\n' "$*" >&2
  exit 1
}

# These are the only host-adapter differences accepted by the neutral contract.
# Arch uses arch-chroot and first-login XDG autostart; Void uses chroot and
# initializes XDG directories eagerly for Turnstile. Account roles and
# permissions use the same shared mapping. Boot and package-manager commands
# are outside the neutral manifest.
declare -A adapter_allowlist=(
  [arch:target-runner]=arch-chroot
  [void:target-runner]=chroot
  [arch:xdg-driver]=first-login
  [void:xdg-driver]=installer
  [arch:extra-groups]=''
  [void:extra-groups]=''
)

answers=(
  RYOKU_DISK_STRATEGY=whole
  RYOKU_PROFILE=vm
  RYOKU_COMPOSITOR=niri
  RYOKU_COMPOSITOR_CONFIG_DIR=niri
  RYOKU_BROWSER=firefox
  RYOKU_LOGIN_SHELL=fish
  RYOKU_KEYMAP=de
  RYOKU_XKB_LAYOUT=de
  RYOKU_XKB_VARIANT=
  RYOKU_LOCALE=en_US.UTF-8
  RYOKU_TIMEZONE=auto
  RYOKU_ONLINE=1
  RYOKU_FIRMWARE_MODE=uefi
  RYOKU_ESP_MODE=shared
  RYOKU_SUBVOL_SNAPSHOTS=1
  RYOKU_EXTRA_USERS_FILE="$extra_users"
)

for edition in arch void; do
  install_dryrun_run "$edition" "$tmp/$edition.out" "${answers[@]}" \
    || fail "$edition niri dry run failed"
done

require_fixed() {
  local edition=$1 needle=$2 label=$3
  grep -qF -- "$needle" "$tmp/$edition.out" \
    || fail "$edition dry run omitted $label"
}

normalize_groups() {
  local edition=$1 groups=$2 group extra normalized=
  local -a kept=()
  for group in ${groups//,/ }; do
    for extra in ${adapter_allowlist[$edition:extra-groups]}; do
      [[ $group != "$extra" ]] || continue 2
    done
    kept+=("$group")
  done
  mapfile -t kept < <(printf '%s\n' "${kept[@]}" | LC_ALL=C sort -u)
  normalized=$(IFS=,; printf '%s' "${kept[*]}")
  printf '%s\n' "$normalized"
}

write_contract() {
  local edition=$1
  local out=$tmp/$edition.out contract=$tmp/$edition.contract
  local runner=${adapter_allowlist[$edition:target-runner]}
  local user_line groups normalized_groups materialize_count

  materialize_count=$(grep -cF "DRYRUN: $runner /mnt runuser -u ryoku -- env HOME=/home/ryoku ryoku materialize" "$out" || true)
  [[ $materialize_count == 1 ]] || fail "$edition planned $materialize_count materialize calls"
  require_fixed "$edition" 'desktop.input.kbLayout=de kbVariant=' 'the neutral keymap seed'
  require_fixed "$edition" 'DRYRUN: write /mnt/home/ryoku/.local/state/ryoku/default-rice-pending:' 'the default-rice marker'
  grep -A1 -F 'DRYRUN: write /mnt/home/ryoku/.local/state/ryoku/default-rice-pending:' "$out" \
    | grep -qF '        | default' || fail "$edition default-rice marker did not select default"
  require_fixed "$edition" "$INSTALL_DRYRUN_ROOT/ryoku/assets/wallpapers /mnt/home/ryoku/Pictures/Wallpapers" 'the seeded wallpaper set'
  require_fixed "$edition" 'deploying qylock bundle + SDDM clockwork theme' 'qylock and SDDM theme selection'
  require_fixed "$edition" 'RYOKU_QYLOCK_BUNDLE=/usr/share/ryoku/qylock RYOKU_QYLOCK_MODE=live SUDO_USER=ryoku' 'the qylock installer environment'
  grep -qF 'welcome-seen' "$out" && fail "$edition pre-marked the welcome tour as seen"
  grep -qF 'keys-hint-seen' "$out" && fail "$edition pre-marked the intro hint as seen"
  require_fixed "$edition" 'DRYRUN: curl -fsSL https://ipinfo.io/timezone' 'online auto-timezone geolocation'
  require_fixed "$edition" 'timezone: <auto-timezone>' 'the resolved auto timezone'
  require_fixed "$edition" '/usr/share/zoneinfo/<auto-timezone> /mnt/etc/localtime' 'the timezone link'

  user_line=$(grep -m1 -E 'DRYRUN: .* useradd -M -U -d /home/ryoku -s /usr/bin/fish -G [^ ]+ ryoku$' "$out") \
    || fail "$edition omitted the primary-user creation plan"
  groups=${user_line#* -G }
  groups=${groups%% ryoku}
  normalized_groups=$(normalize_groups "$edition" "$groups")
  require_fixed "$edition" 'DRYRUN: btrfs quota enable /mnt/home' 'Btrfs home quotas'
  require_fixed "$edition" 'DRYRUN: btrfs subvolume create /mnt/home/alice' 'the standard account home subvolume'
  require_fixed "$edition" 'useradd -M -U -d /home/alice -s /usr/bin/fish -G input,video,audio -c Alice Example alice' 'the standard account plan'
  require_fixed "$edition" 'DRYRUN: btrfs qgroup limit 1073741824 /mnt/home/alice' 'the standard account storage limit'
  require_fixed "$edition" 'DRYRUN: btrfs subvolume create /mnt/home/bob' 'the administrator home subvolume'
  require_fixed "$edition" 'useradd -M -U -d /home/bob -s /usr/bin/fish -G wheel,input,video,kvm,libvirt,docker -c Bob Example bob' 'the administrator account plan'
  require_fixed "$edition" "DRYRUN: ${adapter_allowlist[$edition:target-runner]} /mnt passwd -l root" 'the locked root account'

  grep -Eq '(^| )xdg-user-dirs( |$)' "$out" \
    || fail "$edition package plan omitted xdg-user-dirs"
  case ${adapter_allowlist[$edition:xdg-driver]} in
    installer)
      require_fixed "$edition" 'XDG_CONFIG_HOME=/home/ryoku/.config LANG=en_US.UTF-8 xdg-user-dirs-update' 'XDG directory initialization'
      ;;
    first-login)
      grep -qF 'xdg-user-dirs-update' "$out" \
        && fail "$edition unexpectedly changed the allow-listed XDG initialization path"
      ;;
  esac

  {
    printf 'materialize\tuser=ryoku home=/home/ryoku\n'
    printf 'keymap\tlayout=de variant=\n'
    printf 'default-rice-pending\tdefault\n'
    printf 'wallpapers\tshipped-set -> /home/ryoku/Pictures/Wallpapers\n'
    printf 'lockscreen\tqylock live\n'
    printf 'greeter\tSDDM clockwork\n'
    printf 'intro\twelcome-and-keys-unseen\n'
    printf 'groups\t%s\n' "$normalized_groups"
    printf 'xdg-dirs\tDesktop Documents Downloads Music Pictures Public Templates Videos\n'
    printf 'timezone\t<auto-timezone> -> /etc/localtime\n'
  } >"$contract"
}

write_contract arch
write_contract void
if ! cmp -s "$tmp/arch.contract" "$tmp/void.contract"; then
  diff -u "$tmp/arch.contract" "$tmp/void.contract" >&2 || true
  fail "Arch and void neutral end-state contracts differ"
fi

# Exercise each edition's timezone step directly so the offline check does not
# depend on a baked package repository being present in the source checkout.
run_offline_timezone() {
  local edition=$1 output=$2
  (
    # shellcheck source=installation/backend/lib/common.sh
    source "$INSTALL_DRYRUN_ROOT/installation/backend/lib/common.sh"
    RYOKU_DRYRUN=1
    RYOKU_ONLINE=0
    RYOKU_TIMEZONE=auto
    case $edition in
      arch)
        # shellcheck source=installation/backend/lib/chroot.sh
        source "$INSTALL_DRYRUN_ROOT/installation/backend/lib/chroot.sh"
        ryoku_cfg_timezone
        ;;
      void)
        # shellcheck source=void/iso/backend/lib/configure.sh
        source "$INSTALL_DRYRUN_ROOT/void/iso/backend/lib/configure.sh"
        void_chroot() { run chroot /mnt "$@"; }
        void_cfg_timezone
        ;;
    esac
  ) >"$output"
}

for edition in arch void; do
  run_offline_timezone "$edition" "$tmp/$edition-offline.out"
  grep -qF 'timezone: offline install, using UTC' "$tmp/$edition-offline.out" \
    || fail "$edition did not explain the offline auto-timezone fallback"
  grep -qF '/usr/share/zoneinfo/UTC /mnt/etc/localtime' "$tmp/$edition-offline.out" \
    || fail "$edition did not link UTC for offline auto timezone"
  ! grep -qF 'DRYRUN: curl -fsSL' "$tmp/$edition-offline.out" \
    || fail "$edition attempted timezone geolocation while offline"
done

timezone_check=$(
  (
    # shellcheck source=installation/backend/lib/common.sh
    source "$INSTALL_DRYRUN_ROOT/installation/backend/lib/common.sh"
    ryoku_validate_timezone UTC-3
    printf 'normalized=%s\n' "$RYOKU_VALIDATED_TIMEZONE"
    ryoku_validate_timezone America/Sao_Paulo
    printf 'region=%s\n' "$RYOKU_VALIDATED_TIMEZONE"
  )
)
grep -qF 'UTC-3 maps to Etc/GMT+3 because Etc/GMT signs are reversed' <<<"$timezone_check" \
  || fail "UTC-3 did not explain the inverted Etc/GMT sign"
grep -qF 'normalized=Etc/GMT+3' <<<"$timezone_check" \
  || fail "UTC-3 did not normalize to Etc/GMT+3"
grep -qF 'region=America/Sao_Paulo' <<<"$timezone_check" \
  || fail "a zoneinfo Region/City was not accepted"

if invalid_timezone=$(
  (
    # shellcheck source=installation/backend/lib/common.sh
    source "$INSTALL_DRYRUN_ROOT/installation/backend/lib/common.sh"
    ryoku_validate_timezone UTC-15
  ) 2>&1
); then
  fail "an unknown timezone was accepted"
fi
grep -qF "unknown timezone 'UTC-15'" <<<"$invalid_timezone" \
  || fail "an unknown timezone did not produce a useful error"

for edition in arch void; do
  if install_dryrun_run "$edition" "$tmp/$edition-invalid-timezone.out" \
    "${answers[@]}" RYOKU_TIMEZONE=UTC-15; then
    fail "$edition accepted an unknown timezone"
  fi
  grep -qF "unknown timezone 'UTC-15'" "$tmp/$edition-invalid-timezone.out" \
    || fail "$edition did not show the timezone error"
  ! grep -qF '@@RYOKU_STEP partition' "$tmp/$edition-invalid-timezone.out" \
    || fail "$edition touched the disk before rejecting the timezone"
done

for edition in arch void; do
  install_dryrun_run "$edition" "$tmp/$edition-offset-timezone.out" \
    "${answers[@]}" RYOKU_TIMEZONE=UTC-3
  grep -qF 'UTC-3 maps to Etc/GMT+3 because Etc/GMT signs are reversed' \
    "$tmp/$edition-offset-timezone.out" \
    || fail "$edition did not explain the fixed-offset mapping"
  grep -qF '/usr/share/zoneinfo/Etc/GMT+3 /mnt/etc/localtime' \
    "$tmp/$edition-offset-timezone.out" \
    || fail "$edition did not install the normalized fixed-offset zone"
done

printf 'installer edition parity: ok\n'

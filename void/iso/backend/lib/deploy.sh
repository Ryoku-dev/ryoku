#!/usr/bin/env bash
# Install the packaged desktop, provision runit, SDDM, and the user's config.

void_desktop_install() {
  local -a args=(--lane desktop --lane dev)
  local lane
  while IFS= read -r lane; do args+=(--lane "$lane"); done < <(void_profile_lanes)
  local -a drop_args=()
  mapfile -t drop_args < <(void_resolver_drop_args)
  args+=("${drop_args[@]}")

  local -a packages=()
  mapfile -t packages < <(void_resolve "${args[@]}")
  packages+=(ryoku-keyring ryoku-desktop "ryoku-desktop-$RYOKU_COMPOSITOR")
  mapfile -t packages < <(printf '%s\n' "${packages[@]}" | awk 'NF && !seen[$0]++')
  void_keep_uninstalled_packages packages

  log 'installing Ryoku desktop from the offline XBPS repository (%d packages)' "${#packages[@]}"
  ((${#packages[@]})) || return 0
  run env XBPS_ARCH=x86_64 \
    xbps-install -r /mnt -R "$RYOKU_OFFLINE_REPO" -i -Sy "${packages[@]}"
}

void_deploy_user() {
  local user=$RYOKU_USERNAME
  local home=/mnt/home/$user

  ryoku_seed_keymap
  ryoku_deploy_chown "$user"
  ryoku_deploy_materialize "$user"
  ryoku_seed_provisioned "$user"
  ryoku_deploy_seed "$home"
  ryoku_deploy_chown "$user"
  ryoku_deploy_qylock

  log 'provisioning Turnstile user services for %s' "$user"
  run "$RYOKU_TARGET_CHROOT" /mnt chpst -u "$user:$user" \
    env "HOME=/home/$user" "USER=$user" "LOGNAME=$user" \
    ryoku-host session ensure --user
}

void_services() {
  log "enabling the Void desktop services"
  run chroot /mnt ryoku-host session ensure --system
  local service
  for service in ryoku-var-state ryoku-boot-guard ryoku-network-kill-guard \
    ryoku-network-kill-disconnect dbus turnstiled polkitd NetworkManager \
    bluetoothd power-profiles-daemon socklog-unix nanoklogd sddm; do
    void_enable_service "$service"
  done
  if [[ ${RYOKU_ENABLE_SERIAL:-0} == 1 ]]; then
    if [[ -n ${RYOKU_DRYRUN:-} || ! -e /mnt/etc/sv/agetty-ttyS0 ]]; then
      run cp -a /mnt/etc/sv/agetty-generic /mnt/etc/sv/agetty-ttyS0
    fi
    void_enable_service agetty-ttyS0
  fi
}
void_default_browser() {
  local user=$RYOKU_USERNAME browser=${RYOKU_BROWSER:-firefox} desktop command
  case $browser in
    chromium) desktop=chromium.desktop; command=chromium ;;
    zen) desktop=zen.desktop; command=zen ;;
    *) desktop=firefox.desktop; command=firefox ;;
  esac
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log 'DRYRUN: set %s as the xdg and Ryoku desktop browser for %s' "$browser" "$user"
    return 0
  fi
  if [[ ! -f /mnt/usr/share/applications/$desktop ]]; then
    log 'default browser: %s is not installed; leaving the role unchanged' "$browser"
    return 0
  fi
  if chroot /mnt runuser -u "$user" -- env HOME="/home/$user" USER="$user" LOGNAME="$user" \
    xdg-mime default "$desktop" x-scheme-handler/http x-scheme-handler/https text/html; then
    log 'default browser: set %s for %s' "$browser" "$user"
  else
    log 'default browser: could not set xdg handlers for %s; continuing' "$browser"
  fi
  run chroot /mnt env TARGET_USER="$user" BROWSER_CMD="$command" sh -c '
    store=/home/$TARGET_USER/.config/ryoku/desktop.json
    mkdir -p "${store%/*}"
    tmp=$store.tmp
    if [ -s "$store" ]; then
      jq --arg browser "$BROWSER_CMD" ". * {desktop:{apps:{browser:\$browser}}}" "$store" >"$tmp"
    else
      jq -n --arg browser "$BROWSER_CMD" "{desktop:{apps:{browser:\$browser}}}" >"$tmp"
    fi
    mv "$tmp" "$store"
    chown "$TARGET_USER:$TARGET_USER" "$store"
  '
}

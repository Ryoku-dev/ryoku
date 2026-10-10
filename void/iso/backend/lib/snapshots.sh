#!/usr/bin/env bash

void_snapshot_limine_defaults() {
  local source=$RYOKU_REPO/system/boot/limine/default.conf
  [[ -r $source ]] || die 'Limine defaults are missing: %s' "$source"

  local defaults key
  defaults=$(awk '/^(TARGET_OS_NAME|ESP_PATH|MAX_SNAPSHOT_ENTRIES|SNAPSHOT_FORMAT_CHOICE)=/ { print }' "$source")
  for key in TARGET_OS_NAME ESP_PATH MAX_SNAPSHOT_ENTRIES SNAPSHOT_FORMAT_CHOICE; do
    [[ $defaults == *"$key="* ]] || die 'Limine defaults are missing %s: %s' "$key" "$source"
  done

  run mkdir -p /mnt/etc/default
  printf '%s\n' "$defaults" | write_file /mnt/etc/default/limine
}

void_snapshot_service() {
  local service=$1
  if [[ -z ${RYOKU_DRYRUN:-} && ! -d /mnt/etc/sv/$service ]]; then
    die 'snapshot service is missing from the target: %s' "$service"
  fi
  void_enable_service "$service"
}

void_snapshots() {
  if [[ ${RYOKU_SUBVOL_SNAPSHOTS:-1} != 1 ]]; then
    ryoku_snapshots_disabled
    return 0
  fi

  log "configuring snapper and Limine snapshot boot entries"
  ryoku_snap_config
  ryoku_snap_updatedb
  void_snapshot_limine_defaults
  void_snapshot_service snapper-cleanup
  void_snapshot_service limine-snapper-sync
}

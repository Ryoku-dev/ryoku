#!/usr/bin/env bash
# Run the shared hardware scripts inside the Void target with all siblings intact.

void_drivers() {
  local source=$RYOKU_REPO/system/hardware/drivers
  local target=/mnt/root/ryoku-drivers
  [[ -d $source ]] || die 'driver payload is missing: %s' "$source"
  log "installing drivers for the detected hardware"
  run rm -rf "$target"
  deploy_dir "$source" "$target"

  local script
  for script in amd intel nvidia vulkan; do
    [[ -f $source/$script.sh ]] || continue
    if ! run timeout 900 chroot /mnt \
      env RYOKU_DRYRUN="${RYOKU_DRYRUN:-}" bash "/root/ryoku-drivers/$script.sh"; then
      log 'drivers: %s setup failed or timed out; continuing so the system remains bootable' "$script"
    fi
  done

  void_gpu_mode
  run rm -rf "$target"
}

void_gpu_mode() {
  [[ -n ${RYOKU_GPU_MODE:-} ]] || return 0
  local mode
  case $RYOKU_GPU_MODE in
    offload) mode=hybrid ;;
    sync) mode=performance ;;
    vfio) mode=passthrough ;;
    *) log 'GPU mode: ignoring unknown value %s' "$RYOKU_GPU_MODE"; return 0 ;;
  esac
  if [[ -z ${RYOKU_COMPOSITOR_GPU_PIN:-} ]]; then
    log 'GPU mode: skipped because the selected window manager owns device selection'
    return 0
  fi
  local user=$RYOKU_USERNAME
  run chroot /mnt runuser -u "$user" -- \
    env HOME="/home/$user" USER="$user" LOGNAME="$user" RYOKU_WM="$RYOKU_COMPOSITOR" \
    ryoku-gpu mode "$mode" || log 'GPU mode: applying %s failed; continuing' "$mode"
}

#!/usr/bin/env bash
set -u

dri_root="${INIR_DRI_ROOT:-/dev/dri}"
sysfs_root="${INIR_SYSFS_ROOT:-/sys}"

if [[ "${INIR_SKIP_GRAPHICS_PREFLIGHT:-0}" == "1" ]]; then
    printf 'Graphics preflight skipped by INIR_SKIP_GRAPHICS_PREFLIGHT=1\n'
    exit 0
fi

shopt -s nullglob
render_nodes=("$dri_root"/renderD*)
usable_nodes=()
for node in "${render_nodes[@]}"; do
    if [[ -r "$node" && -w "$node" ]]; then
        usable_nodes+=("$node")
    fi
done

if [[ ${#usable_nodes[@]} -eq 0 ]]; then
    printf 'No accessible DRM render node was found under %s.\n' "$dri_root" >&2
    printf 'Void must already have working kernel/firmware and Mesa or vendor graphics support before installing iNiR.\n' >&2
    exit 2
fi

virtio_without_virgl=0
for node in "${usable_nodes[@]}"; do
    render_name="$(basename -- "$node")"
    sys_render="$sysfs_root/class/drm/$render_name"
    device="$(readlink -f "$sys_render/device" 2>/dev/null || true)"

    # VirtIO exposes negotiated feature bits in sysfs. Bit 0 is
    # VIRTIO_GPU_F_VIRGL; without it Niri cannot use this VM graphics path.
    feature_files=()
    if [[ -n "$device" && -d "$device" ]]; then
        feature_files=("$device"/virtio*/features)
    fi

    if [[ ${#feature_files[@]} -eq 0 ]]; then
        printf 'DRM render node available: %s\n' "$node"
        exit 0
    fi

    for feature_file in "${feature_files[@]}"; do
        [[ -r "$feature_file" ]] || continue
        first_bit="$(head -c 1 "$feature_file" 2>/dev/null || true)"
        if [[ "$first_bit" == "1" ]]; then
            printf 'VirtIO VirGL render node available: %s\n' "$node"
            exit 0
        fi
        if [[ "$first_bit" == "0" ]]; then
            virtio_without_virgl=1
        fi
    done
done

if [[ $virtio_without_virgl -eq 1 ]]; then
    printf 'VirtIO GPU has no VirGL support. Niri cannot use this VM renderer configuration.\n' >&2
    printf 'Enable 3D acceleration with virtio-vga-gl or virtio-gpu-gl; lavapipe/software EGL is not a supported fallback here.\n' >&2
    exit 3
fi

printf 'DRM render node available.\n'

# SPDX-License-Identifier: GPL-3.0-only
# Shared helpers for the lab scripts. The scripts source this file inside
# their own process; never source it, or the lab.env file it reads, into an
# interactive or persistent shell.

lab_die() {
    printf '%s: %s\n' "${0##*/}" "$*" >&2
    exit 1
}

# ONI_LAB_ID selects one of several labs that may run at once: unset or empty
# is the default lab, /dev/shm/oni-lab; ONI_LAB_ID=13 is /dev/shm/oni-lab-13.
# Every socket, bus, port, scope and file of a lab lives under its directory
# or carries its scope tag, so two labs share nothing, and down.sh of one
# never matches the other's processes. An explicit ONI_LAB must agree with
# ONI_LAB_ID when both are set. ONI_LAB is exported so every script and kit a
# lab script starts addresses the same lab.
if [ -n "${ONI_LAB_ID:-}" ]; then
    case $ONI_LAB_ID in
        *[!a-z0-9]*) lab_die "ONI_LAB_ID may only hold lowercase letters and digits (got $ONI_LAB_ID)" ;;
    esac
    [ "${#ONI_LAB_ID}" -le 3 ] || lab_die "ONI_LAB_ID is longer than 3 bytes; ONI_LAB would pass 20"
    [ -z "${ONI_LAB:-}" ] || [ "$ONI_LAB" = "/dev/shm/oni-lab-$ONI_LAB_ID" ] ||
        lab_die "ONI_LAB=$ONI_LAB disagrees with ONI_LAB_ID=$ONI_LAB_ID (/dev/shm/oni-lab-$ONI_LAB_ID)"
    ONI_LAB=/dev/shm/oni-lab-$ONI_LAB_ID
fi
ONI_LAB=${ONI_LAB:-/dev/shm/oni-lab}
export ONI_LAB

# Hyprland's IPC socket is $XDG_RUNTIME_DIR/hypr/<signature>/.socket2.sock;
# the signature alone is about 60 bytes and sun_path holds 107.
lab_check_path() {
    case $ONI_LAB in
        /dev/shm/?*) ;;
        *) lab_die "ONI_LAB must be a directory under /dev/shm (got $ONI_LAB)" ;;
    esac
    case $ONI_LAB in
        *[!A-Za-z0-9/._-]* | */../* | */.. | *//*) lab_die "ONI_LAB may only hold letters, digits, '/', '.', '_' and '-'" ;;
    esac
    [ "${#ONI_LAB}" -le 20 ] || lab_die "ONI_LAB is longer than 20 bytes; Hyprland's socket path would overflow"
}

lab_check_path
LAB_ENV=$ONI_LAB/lab.env
LAB_LOG=$ONI_LAB/log
# compositor runtime: host compositor sockets, nested compositor sockets and IPC
LAB_WRT=$ONI_LAB/w
# client runtime: the app's bus, daemon socket and locks
LAB_RT=$ONI_LAB/rt
# oni-surface's ledger: surface.jsonl belongs to the running surface, earlier
# runs are kept as surface-N.jsonl
LAB_LEDGERS=$ONI_LAB/ledger
LAB_LEDGER=$LAB_LEDGERS/surface.jsonl
LAB_KIT=${HERE:?}/labkit.py

# Refuse unless up.sh finished: a missing env file would point grim, wtype or
# the pointer at the live desktop.
lab_load() {
    [ -f "$LAB_ENV" ] || lab_die "no lab at $ONI_LAB (missing $LAB_ENV); run up.sh niri|hyprland first"
    # shellcheck source=/dev/null
    . "$LAB_ENV"
    [ "${LAB_READY:-0}" = 1 ] || lab_die "lab at $ONI_LAB is not ready (up.sh did not finish)"
    case $LAB_DISPLAY in
        "$LAB_WRT"/*) ;;
        *) lab_die "lab display $LAB_DISPLAY is not inside $LAB_WRT" ;;
    esac
    [ -S "$LAB_DISPLAY" ] || lab_die "lab display $LAB_DISPLAY is gone"
}

# lab.env is append-only; a later line overrides an earlier one when sourced.
lab_env_put() {
    printf "%s='%s'\n" "$1" "$2" >>"$LAB_ENV"
}

# lab_run ROLE MEMORYMAX COMMAND...: every lab process runs in its own
# transient user scope with a memory ceiling, no swap, CPU accounting for
# measure.sh and the highest OOM-kill priority, so a runaway probe dies before
# anything of the live session.
lab_run() {
    _role=$1 _mem=$2
    shift 2
    systemd-run --user --scope --quiet --collect \
        --unit="oni-lab-${LAB_ID:?}-$_role" \
        -p MemoryMax="$_mem" -p MemorySwapMax=0 -p CPUAccounting=yes \
        -- choom -n 1000 -- "$@"
}

lab_rand() {
    od -An -N4 -tx1 /dev/urandom | tr -d ' \n'
}

# lab_tool ROLE COMMAND...: a short Wayland client (grim, wtype, the pointer)
# that sees only the nested compositor. The pointer's absolute extent is the
# union of the enabled outputs.
lab_tool() {
    _role=$1
    shift
    lab_run "$_role-$(lab_rand)" 256M env -i PATH=/usr/bin:/bin \
        XDG_RUNTIME_DIR="$LAB_WRT" WAYLAND_DISPLAY="$LAB_DISPLAY" \
        ONI_PTR_W="$LAB_EXTENT_W" ONI_PTR_H="$LAB_EXTENT_H" "$@"
}

lab_niri() {
    lab_run "niri-msg-$(lab_rand)" 64M env -i PATH=/usr/bin:/bin NIRI_SOCKET="$LAB_NIRI_SOCKET" niri msg "$@"
}

lab_hyprctl() {
    lab_run "hyprctl-$(lab_rand)" 64M env -i PATH=/usr/bin:/bin XDG_RUNTIME_DIR="$LAB_WRT" \
        hyprctl -i "$LAB_HYPR_INSTANCE" "$@"
}

# lab_app ROLE MEMORYMAX COMMAND...: start a process under test in its own
# scope and session. Nothing from the caller leaks in: HOME, every XDG
# directory, TMPDIR, the session bus and the runtime directory are the lab's
# own, and PATH starts with the lab's refusal stubs. LAB_WM_ENV names the
# nested compositor's IPC (NIRI_SOCKET or HYPRLAND_INSTANCE_SIGNATURE). GIO
# stays on local files so the private bus never activates gvfsd and its FUSE
# mount. NAME=VALUE words before COMMAND are added to that environment.
lab_app() {
    _role=$1 _mem=$2
    shift 2
    if [ -n "${ONI_LAB_VK_DRIVER_FILES:-}" ]; then
        case $ONI_LAB_VK_DRIVER_FILES in
            /nix/store/*/share/vulkan/icd.d/*.json) ;;
            *) lab_die "the lab Vulkan driver must be a Nix-store ICD manifest" ;;
        esac
        [ -f "$ONI_LAB_VK_DRIVER_FILES" ] || lab_die "the lab Vulkan ICD manifest is missing"
        set -- "VK_DRIVER_FILES=$ONI_LAB_VK_DRIVER_FILES" "$@"
    fi
    if [ -n "${ONI_LAB_SOFTWARE_MESA:-}" ]; then
        case $ONI_LAB_SOFTWARE_MESA in
            /nix/store/*-mesa-*) ;;
            *) lab_die "the lab software Mesa must be a Nix-store output" ;;
        esac
        _egl=$ONI_LAB_SOFTWARE_MESA/share/glvnd/egl_vendor.d/50_mesa.json
        [ -f "$_egl" ] && [ -d "$ONI_LAB_SOFTWARE_MESA/lib/dri" ] &&
            [ -d "$ONI_LAB_SOFTWARE_MESA/lib/gbm" ] || lab_die "the lab Mesa driver set is incomplete"
        set -- "__EGL_VENDOR_LIBRARY_FILENAMES=$_egl" \
            "LIBGL_DRIVERS_PATH=$ONI_LAB_SOFTWARE_MESA/lib/dri" \
            "GBM_BACKENDS_PATH=$ONI_LAB_SOFTWARE_MESA/lib/gbm" \
            LIBGL_ALWAYS_SOFTWARE=true GALLIUM_DRIVER=llvmpipe "$@"
    fi
    lab_run "$_role" "$_mem" setsid -w env -i HOME="$ONI_LAB/home" USER="${USER:-oni}" LOGNAME="${USER:-oni}" \
        LANG=C.UTF-8 PATH="$ONI_LAB/bin:/usr/bin:/bin" TMPDIR="$ONI_LAB/tmp" \
        XDG_CONFIG_HOME="$ONI_LAB/cfg" XDG_STATE_HOME="$ONI_LAB/state" \
        XDG_CACHE_HOME="$ONI_LAB/cache" XDG_DATA_HOME="$ONI_LAB/data" \
        XDG_DATA_DIRS=/usr/local/share:/usr/share \
        XDG_RUNTIME_DIR="$LAB_RT" WAYLAND_DISPLAY="$LAB_DISPLAY" XDG_SESSION_TYPE=wayland \
        XDG_CURRENT_DESKTOP="$LAB_DESKTOP" "$LAB_WM_ENV" \
        DBUS_SESSION_BUS_ADDRESS="unix:path=$LAB_RT/bus" DBUS_SYSTEM_BUS_ADDRESS="unix:path=$LAB_RT/bus" \
        GIO_USE_VFS=local GVFS_DISABLE_FUSE=1 \
        HYAKKI_DAEMON="${HYAKKI_BIN:?}/oni" HYAKKI_SURFACE="$HYAKKI_BIN/oni-surface" \
        "$@"
}

# lab_hyakki ARG...: the hyakki command line inside the lab, as the app sees it.
lab_hyakki() {
    lab_app "cli-$(lab_rand)" 256M "$HYAKKI_BIN/hyakki" "$@"
}

# lab_daemon [NAME=VALUE...]: start the oni daemon in its own scope and wait
# for its socket. NAME=VALUE words are lab hooks (ctl.sh checks them); with
# any, the daemon also gets HYAKKI_ONI_LAB=1.
lab_daemon() {
    [ "$#" = 0 ] || set -- HYAKKI_ONI_LAB=1 "$@"
    lab_app "oni-$(lab_rand)" 1536M "$@" "$HYAKKI_BIN/oni" >>"$LAB_LOG/oni.log" 2>&1 </dev/null &
    _n=200
    while [ "$_n" -gt 0 ]; do
        [ -S "$LAB_RT/hyakki/oni.sock" ] && return 0
        _n=$((_n - 1))
        sleep 0.1
    done
    return 1
}

# lab_surface [NAME=VALUE...]: start oni-surface with the lab hooks on, in a
# fresh surface scope, and return once its ledger has a line. The previous
# run's ledger is kept as surface-N.jsonl.
lab_surface() {
    mkdir -p "$LAB_LEDGERS"
    if [ -e "$LAB_LEDGER" ]; then
        _k=1
        while [ -e "$LAB_LEDGERS/surface-$_k.jsonl" ]; do _k=$((_k + 1)); done
        mv "$LAB_LEDGER" "$LAB_LEDGERS/surface-$_k.jsonl"
    fi
    lab_app "surface-$(lab_rand)" 512M HYAKKI_ONI_LAB=1 HYAKKI_ONI_LEDGER="$LAB_LEDGER" "$@" \
        "$HYAKKI_BIN/oni-surface" >>"$LAB_LOG/oni-surface.log" 2>&1 </dev/null &
    _n=100
    while [ "$_n" -gt 0 ]; do
        [ -s "$LAB_LEDGER" ] && return 0
        _n=$((_n - 1))
        sleep 0.1
    done
    return 1
}

# lab_stop COMM [SIGNAL]: signal this lab's processes named COMM (oni or
# oni-surface) and wait until they are gone; oni-surface's instance lock is
# free once this returns.
lab_stop() {
    python3 "$LAB_KIT" stop "$LAB_ID" "$1" "${2:-TERM}"
}

# lab_state: the compositor's outputs (with logical position), windows and,
# where the compositor lists them, layer-shell surfaces as one JSON object.
lab_state() {
    if [ "$LAB_WM" = niri ]; then
        _o=$(lab_niri -j outputs) || return 1
        _w=$(lab_niri -j windows) || return 1
        _l=$(lab_niri -j layers 2>/dev/null) || _l=null
    else
        _o=$(lab_hyprctl -j monitors all) || return 1
        _w=$(lab_hyprctl -j clients) || return 1
        _l=$(lab_hyprctl -j layers 2>/dev/null) || _l=null
    fi
    python3 "$LAB_KIT" state "$LAB_WM" "$_o" "$_w" "$_l"
}

# lab_extent: "W H" of the union of the enabled outputs, the virtual
# pointer's absolute extent.
lab_extent() {
    lab_state | python3 "$LAB_KIT" extent
}

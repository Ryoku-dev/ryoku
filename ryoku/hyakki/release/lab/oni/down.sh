#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# down.sh: stop every process whose environment names the lab, prove that no
# lab process and no lab scope is left and that the live session's systemd
# user environment is unchanged, then remove $ONI_LAB.
#   ONI_LAB_SAVE=DIR  copy the lab's logs (and Hyprland's instance log, which
#                     Hyprland deletes at exit) into DIR first.
#   ONI_LAB_KEEP=1    stop the processes but keep $ONI_LAB.
set -u
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"

env_value() {
    [ -f "$LAB_ENV" ] && sed -n "s/^$1='\\(.*\\)'\$/\\1/p" "$LAB_ENV" | tail -n 1
}
userenv_before=$(env_value LAB_USERENV_MD5)
lab_id=$(env_value LAB_ID)

if [ -n "${ONI_LAB_SAVE:-}" ] && [ -d "$ONI_LAB" ]; then
    mkdir -p "$ONI_LAB_SAVE"
    [ -d "$LAB_LOG" ] && cp -a "$LAB_LOG/." "$ONI_LAB_SAVE/"
    [ -f "$LAB_ENV" ] && cp "$LAB_ENV" "$ONI_LAB_SAVE/lab.env"
    for f in "$LAB_WRT"/hypr/*/hyprland.log; do
        [ -f "$f" ] && cp "$f" "$ONI_LAB_SAVE/hyprland-instance.log"
    done
    [ -f "$ONI_LAB/state/hyakki/oni.log" ] && cp "$ONI_LAB/state/hyakki/oni.log" "$ONI_LAB_SAVE/oni-daemon.log"
    [ -d "$ONI_LAB/ledger" ] && cp -a "$ONI_LAB/ledger" "$ONI_LAB_SAVE/ledger"
fi

# A process belongs to the lab when its runtime, display, home, XDG
# directories or compositor IPC point inside $ONI_LAB, or when it runs in one
# of this lab's scopes: oni marks itself non-dumpable, so its environ is
# unreadable even to its own user. The caller's own ancestry is never touched.
python3 - "$ONI_LAB" "$lab_id" <<'PY'
import os, signal, sys, time

mark, lab_id = sys.argv[1], sys.argv[2]
scope = f"/oni-lab-{lab_id}-" if lab_id else None
keys = {b"XDG_RUNTIME_DIR", b"WAYLAND_DISPLAY", b"HOME", b"NIRI_SOCKET",
        b"XDG_CONFIG_HOME", b"XDG_STATE_HOME", b"XDG_CACHE_HOME", b"XDG_DATA_HOME"}
m = mark.encode()

ancestors = set()
p = os.getpid()
while p > 1:
    ancestors.add(p)
    try:
        p = int(open(f"/proc/{p}/stat").read().rsplit(")", 1)[1].split()[1])
    except (OSError, ValueError, IndexError):
        break

def lab_pids():
    found = []
    for d in os.listdir("/proc"):
        if not d.isdigit() or int(d) in ancestors:
            continue
        if scope:
            try:
                if scope in open(f"/proc/{d}/cgroup").read():
                    found.append(int(d))
                    continue
            except OSError:
                continue
        try:
            env = open(f"/proc/{d}/environ", "rb").read().split(b"\0")
        except OSError:
            continue
        for entry in env:
            k, _, v = entry.partition(b"=")
            if k in keys and (v == m or v.startswith(m + b"/")):
                found.append(int(d))
                break
    return found

def alive(pid):
    try:
        st = open(f"/proc/{pid}/stat").read().rsplit(")", 1)[1].split()[0]
    except OSError:
        return False
    return st != "Z"

stopped = 0
for _ in range(4):
    victims = lab_pids()
    if not victims:
        break
    stopped += len(victims)
    for pid in victims:
        try:
            os.kill(pid, signal.SIGTERM)
        except OSError:
            pass
    deadline = time.monotonic() + 3
    while time.monotonic() < deadline and any(alive(p) for p in victims):
        time.sleep(0.1)
    for pid in victims:
        if alive(pid):
            try:
                os.kill(pid, signal.SIGKILL)
            except OSError:
                pass
    time.sleep(0.5)
left = [p for p in lab_pids() if alive(p)]
print(f"down.sh: signalled {stopped} lab processes; {len(left)} left")
for p in left:
    try:
        print("  left:", p, open(f"/proc/{p}/cmdline", "rb").read().replace(b"\0", b" ").decode(errors="replace"))
    except OSError:
        pass
sys.exit(1 if left else 0)
PY
reaped=$?

grep -F " $ONI_LAB/" /proc/mounts | while read -r _ mnt _; do
    fusermount3 -u "$mnt" 2>/dev/null || fusermount -u "$mnt" 2>/dev/null || true
done

units=0
if [ -n "$lab_id" ]; then
    systemctl --user reset-failed "oni-lab-$lab_id-*" >/dev/null 2>&1 || true
    units=$(systemctl --user list-units --all --plain --no-legend "oni-lab-$lab_id-*" 2>/dev/null | grep -c . || true)
    printf 'down.sh: %s lab scopes left\n' "$units"
fi

if [ "${ONI_LAB_KEEP:-0}" != 1 ] && [ "$reaped" = 0 ]; then
    rm -rf "$ONI_LAB"
fi

userenv_after=$(systemctl --user show-environment | md5sum | cut -d' ' -f1)
envst=0
if [ -n "$userenv_before" ]; then
    if [ "$userenv_before" = "$userenv_after" ]; then
        printf 'down.sh: live session environment unchanged (%s)\n' "$userenv_after"
    else
        printf 'down.sh: LIVE SESSION ENVIRONMENT CHANGED: %s -> %s\n' "$userenv_before" "$userenv_after" >&2
        envst=1
    fi
else
    printf 'down.sh: no recorded session environment to compare (now %s)\n' "$userenv_after"
fi
[ "$reaped" = 0 ] && [ "$units" = 0 ] && [ "$envst" = 0 ]

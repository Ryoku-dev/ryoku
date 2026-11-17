#!/usr/bin/env python3
"""List processes with the kernel's tick-exact start time.

The Tasks card must be able to tell the process the user confirmed from one that
later reused its pid. `ps etimes` is whole seconds, which leaves a two-second
window; /proc/<pid>/stat keeps the start time in clock ticks, so the scan and
the end helper can compare exactly.

The output is one tab-separated row per process:

    user  pid  name  cpu-seconds  elapsed-seconds  rss-kib  start-epoch
"""

from __future__ import annotations

import os
import pwd
import time

TICKS = os.sysconf("SC_CLK_TCK")
PAGE_KIB = os.sysconf("SC_PAGE_SIZE") // 1024


def boot_time() -> int:
    with open("/proc/stat", "r", encoding="utf-8") as handle:
        for line in handle:
            if line.startswith("btime "):
                return int(line.split()[1])
    raise OSError("no boot time")


def owner(uid: int, cache: dict[int, str]) -> str:
    if uid not in cache:
        try:
            cache[uid] = pwd.getpwuid(uid).pw_name
        except KeyError:
            cache[uid] = str(uid)
    return cache[uid]


def fields(pid: int) -> tuple[str, list[bytes]] | None:
    try:
        with open(f"/proc/{pid}/stat", "rb") as handle:
            data = handle.read()
    except OSError:
        return None
    head, separator, tail = data.rpartition(b") ")
    open_paren = head.find(b"(")
    if not separator or open_paren < 0:
        return None
    name = " ".join(head[open_paren + 1:].decode("utf-8", "replace").split())
    return name, tail.split()


def status(pid: int) -> tuple[int, int] | None:
    uid = -1
    rss = 0
    try:
        with open(f"/proc/{pid}/status", "r", encoding="utf-8", errors="replace") as handle:
            for line in handle:
                if line.startswith("Uid:"):
                    uid = int(line.split()[1])
                elif line.startswith("VmRSS:"):
                    rss = int(line.split()[1])
    except (OSError, ValueError, IndexError):
        return None
    return uid, rss


def main() -> None:
    boot = boot_time()
    now = time.time()
    names: dict[int, str] = {}
    out = []
    for entry in os.listdir("/proc"):
        if not entry.isdigit():
            continue
        pid = int(entry)
        parsed = fields(pid)
        if parsed is None:
            continue
        name, values = parsed
        state = status(pid)
        if state is None or not name or len(values) < 22:
            continue
        uid, rss = state
        try:
            cpu_ticks = int(values[11]) + int(values[12])
            start_ticks = int(values[19])
        except ValueError:
            continue
        started = boot + start_ticks / TICKS
        elapsed = max(0, int(now - started))
        cpu = cpu_ticks // TICKS
        out.append("\t".join([
            owner(uid, names),
            str(pid),
            name,
            str(cpu),
            str(elapsed),
            str(rss or int(values[21]) * PAGE_KIB),
            f"{started:.6f}",
        ]))
    print("\n".join(out))


if __name__ == "__main__":
    main()

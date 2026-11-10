#!/usr/bin/env python3
"""End a process only if it is still the one the user confirmed.

A pid alone is not an identity: Linux reuses pids, and the check and the signal
must not straddle that reuse. The pidfd pins the process the kernel gave us, so
the signal cannot reach a different process that takes the same pid later.
"""

from __future__ import annotations

import os
import signal
import sys

WINDOW = 2.0


def start_time(pid: int) -> float:
    with open(f"/proc/{pid}/stat", "r", encoding="utf-8") as handle:
        fields = handle.read().rsplit(")", 1)[1].split()
    ticks = int(fields[19])
    with open("/proc/stat", "r", encoding="utf-8") as handle:
        for line in handle:
            if line.startswith("btime "):
                boot = int(line.split()[1])
                break
        else:
            raise OSError("no boot time")
    return boot + ticks / os.sysconf("SC_CLK_TCK")


def main() -> int:
    if len(sys.argv) != 3:
        return 2
    pid = int(sys.argv[1])
    expected = float(sys.argv[2])
    try:
        pidfd = os.pidfd_open(pid)
    except (ProcessLookupError, PermissionError):
        return 1
    try:
        try:
            seen = start_time(pid)
        except (OSError, IndexError, ValueError):
            return 1
        if abs(seen - expected) > WINDOW:
            return 1
        try:
            signal.pidfd_send_signal(pidfd, signal.SIGTERM)
        except ProcessLookupError:
            return 1
    finally:
        os.close(pidfd)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

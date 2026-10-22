#!/usr/bin/env python3
import re
import statistics
import sys
from pathlib import Path

PATTERNS = {
    "desktop_open_ms": re.compile(r"\[perf\]\[DesktopContextMenu\] open latency ms=(\d+)"),
    "desktop_build_ms": re.compile(r"\[perf\]\[DesktopContextMenu\] build model .* ms=(\d+)"),
    "context_load_ms": re.compile(r"\[perf\]\[ContextMenuPage\] load source=[^ ]+ ms=(\d+)"),
    "context_save_ms": re.compile(r"\[perf\]\[ContextMenuPage\] save queued ms=(\d+)"),
    "tray_update_groups_ms": re.compile(r"\[perf\]\[TrayMenu\] updateGroups ms=(\d+)"),
    "tray_open_update_ms": re.compile(r"\[perf\]\[TrayMenu\] open/update latency ms=(\d+)"),
}


def percentile(sorted_values, p):
    if not sorted_values:
        return None
    if len(sorted_values) == 1:
        return sorted_values[0]
    rank = (len(sorted_values) - 1) * p
    lo = int(rank)
    hi = min(lo + 1, len(sorted_values) - 1)
    frac = rank - lo
    return sorted_values[lo] + (sorted_values[hi] - sorted_values[lo]) * frac


def summarize(name, values):
    if not values:
        return None
    vals = sorted(values)
    return {
        "name": name,
        "count": len(vals),
        "min": vals[0],
        "p50": percentile(vals, 0.50),
        "p95": percentile(vals, 0.95),
        "max": vals[-1],
        "mean": statistics.fmean(vals),
    }


def parse_text(text):
    buckets = {k: [] for k in PATTERNS}
    for line in text.splitlines():
        for key, pat in PATTERNS.items():
            m = pat.search(line)
            if m:
                buckets[key].append(int(m.group(1)))
    return buckets


def print_table(stats):
    print("metric,count,min,p50,p95,max,mean")
    for item in stats:
        print(
            f"{item['name']},{item['count']},{item['min']},"
            f"{item['p50']:.2f},{item['p95']:.2f},{item['max']},{item['mean']:.2f}"
        )


def main():
    if len(sys.argv) != 2:
        print("Usage: menu_perf_stats.py <log-file>")
        return 2

    path = Path(sys.argv[1])
    if not path.exists():
        print(f"File not found: {path}")
        return 2

    text = path.read_text(encoding="utf-8", errors="replace")
    buckets = parse_text(text)
    stats = []
    for key, values in buckets.items():
        summary = summarize(key, values)
        if summary is not None:
            stats.append(summary)

    if not stats:
        print("No known [perf] menu metrics found in the log.")
        return 1

    print_table(stats)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

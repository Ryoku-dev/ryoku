#!/usr/bin/env python3
"""Fail when `timeout` wraps a command that may read the terminal without --foreground.

GNU timeout runs its command in a new process group. From there sudo cannot hide the password
(it shows on screen) and any read from the terminal (sudo, pacman/yay `[Y/n]`, read) never
returns, so the command looks hung. `timeout --foreground` keeps it in the terminal's group.
"""
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
READS_TERMINAL = {
    "sudo", "pkexec", "pacman", "yay", "paru", "makepkg", "dnf", "apt", "apt-get",
    "xbps-install", "zypper", "nixos-rebuild", "eval", "read",
}
TIMEOUT = re.compile(
    r"(?:^|[;&|(`\s])timeout\s+((?:-[-\w=]+\s+)*)\S+\s+(?:env\s+(?:\w+=\S*\s+)*)?(\S+)(\s+-c\b)?"
)


def shell_files():
    yield ROOT / "setup"
    yield ROOT / "scripts" / "inir"
    for base in ("sdata", "scripts", "distro"):
        yield from sorted((ROOT / base).rglob("*.sh"))


def violations(path: Path):
    for number, line in enumerate(path.read_text(errors="replace").splitlines(), 1):
        if line.lstrip().startswith("#"):
            continue
        for match in TIMEOUT.finditer(line):
            options, command, dash_c = match.group(1), match.group(2).strip("\"'"), match.group(3)
            if "--foreground" in options or re.search(r"(^|\s)-f\b", options):
                continue
            if command in READS_TERMINAL or (command in ("bash", "sh") and dash_c):
                yield number, line.strip()


def main(argv):
    files = [Path(arg) for arg in argv] or [p for p in shell_files() if p.is_file()]
    found = [(f, n, text) for f in files for n, text in violations(f)]
    for f, n, text in found:
        shown = f.relative_to(ROOT) if f.is_relative_to(ROOT) else f
        print(f"{shown}:{n}: timeout without --foreground around a command that reads the terminal: {text}")
    if found:
        return 1
    print(f"TTY timeouts: {len(files)} shell files keep terminal prompts in the foreground")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

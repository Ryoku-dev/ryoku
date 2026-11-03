#!/usr/bin/env python3
"""List what a change takes away from the distributions it was not written for.

iNiR runs on Arch, Fedora, Debian, Void and Nix. Support for one of them never changes what the others run:
a command swapped for one init system or package manager, or an option removed because one distro lacks it,
breaks everyone else silently. This lists, for a branch or a pull request:

  GONE     a system command (init, session, package manager) a file ran before and no longer runs anywhere
  OPTION   a Config option or default that disappeared
  DEFAULT  a default value that changed (information: existing configs keep theirs)

Usage:
  scripts/check-distro-parity.py                  # this branch against origin/prerelease
  scripts/check-distro-parity.py BASE [HEAD]      # any two revisions (HEAD defaults to HEAD)
  scripts/check-distro-parity.py --pr N           # a pull request on GitHub (fetches refs/pull/N/head)

Exit 1 when anything is GONE or an OPTION went away: each line needs an answer for every other distro.
"""
import json
import re
import subprocess
import sys

INIT = {"systemctl", "loginctl", "journalctl", "systemd-run", "systemd-inhibit", "busctl", "sv", "runsvdir",
        "chpst", "turnstiled", "dinitctl", "rc-service", "rc-update", "elogind-inhibit"}
PACKAGES = {"pacman", "yay", "paru", "checkupdates", "paccache", "makepkg", "dnf", "rpm", "apt", "apt-get",
            "dpkg", "zypper", "flatpak", "nix", "nix-env", "nixos-rebuild", "home-manager",
            "xbps-install", "xbps-remove", "xbps-query", "xbps-reconfigure"}
TOOLS = INIT | PACKAGES
VERBS = {"poweroff", "reboot", "halt", "suspend", "hibernate", "hybrid-sleep", "suspend-then-hibernate", "start",
         "stop", "restart", "reload", "enable", "disable", "mask", "unmask", "status", "show", "is-active",
         "is-enabled", "show-environment", "import-environment", "set-environment", "daemon-reload", "kill",
         "lock-session", "terminate-session", "up", "down", "install", "remove", "update", "upgrade", "search"}
SOURCE = re.compile(r"\.(qml|js|sh|py|kdl|service|nix|fish|bash|zsh)$|^(setup|scripts/inir)$")
SKIP = re.compile(r"^(docs/|translations/|CHANGELOG|README|scripts/(check|test)-)")
TOKEN = re.compile(r"[A-Za-z0-9_./+:@-]+")
COMMENT = re.compile(r"^\s*(#|//|\*|/\*)")


def git(*args: str, check: bool = True) -> str:
    run = subprocess.run(["git", *args], capture_output=True, text=True)
    if check and run.returncode != 0:
        sys.exit(f"git {' '.join(args)}: {run.stderr.strip()}")
    return run.stdout


def show(rev: str, path: str) -> str:
    return git("show", f"{rev}:{path}", check=False)


def calls(line: str) -> set:
    """The system commands a line runs, keyed with their verb: `systemctl poweroff`, `yay`."""
    if COMMENT.match(line):
        return set()
    words = TOKEN.findall(line)
    out = set()
    for i, word in enumerate(words):
        name = word.rsplit("/", 1)[-1]
        if name not in TOOLS:
            continue
        verb = next((w for w in words[i + 1:i + 4] if not w.startswith("-")), "")
        out.add(f"{name} {verb}" if verb in VERBS else name)
    return out


def diff_lines(base: str, head: str) -> dict:
    """{path: (removed lines, added lines)} for source files."""
    out, path = {}, None
    for line in git("diff", "-U0", "--no-color", base, head).splitlines():
        if line.startswith("+++ "):
            path = line[6:] if line.startswith("+++ b/") else None
            if path and (SKIP.match(path) or not SOURCE.search(path)):
                path = None
            if path:
                out.setdefault(path, ([], []))
        elif line.startswith("--- "):
            old = line[6:] if line.startswith("--- a/") else None
            if old and not SKIP.match(old) and SOURCE.search(old):
                out.setdefault(old, ([], []))
                path = old
        elif path and line.startswith("-") and not line.startswith("---"):
            out[path][0].append(line[1:])
        elif path and line.startswith("+") and not line.startswith("+++"):
            out[path][1].append(line[1:])
    return out


def gone_commands(base: str, head: str, files: dict) -> list:
    """A command a file no longer runs. Another file adding the same command proves nothing about this
    caller (the installer's `systemctl reboot` never rebooted from the session screen), so it is only a hint."""
    added_in = {}
    for path, (_removed, added) in files.items():
        for line in added:
            for key in calls(line):
                added_in.setdefault(key, set()).add(path)
    findings = []
    for path, (removed, _added) in sorted(files.items()):
        after = set()
        for line in show(head, path).splitlines():
            after |= calls(line)
        lost = set()
        for line in removed:
            lost |= calls(line) - after
        for key in sorted(lost):
            elsewhere = sorted(added_in.get(key, set()) - {path})
            hint = f"  (added in {', '.join(elsewhere[:2])}{', …' if len(elsewhere) > 2 else ''})" if elsewhere else ""
            findings.append(("GONE", path, key + hint))
    return findings


def leaves(node, prefix=""):
    if isinstance(node, dict):
        for key, value in node.items():
            yield from leaves(value, f"{prefix}.{key}" if prefix else key)
    else:
        yield prefix, node


def options(base: str, head: str) -> list:
    findings = []
    prop = re.compile(r"^\s*(?:readonly\s+)?property\s+\S+\s+(\w+)\s*:")
    before = {m.group(1) for l in show(base, "modules/common/Config.qml").splitlines() if (m := prop.match(l))}
    after_text = show(head, "modules/common/Config.qml")
    for name in sorted(before):
        if not re.search(rf"\b{re.escape(name)}\b", after_text):
            findings.append(("OPTION", "modules/common/Config.qml", name))
    try:
        old = dict(leaves(json.loads(show(base, "defaults/config.json") or "{}")))
        new = dict(leaves(json.loads(show(head, "defaults/config.json") or "{}")))
    except ValueError:
        return findings
    for key in sorted(old):
        if key not in new:
            findings.append(("OPTION", "defaults/config.json", key))
        elif old[key] != new[key]:
            findings.append(("DEFAULT", "defaults/config.json", f"{key}: {json.dumps(old[key])} -> {json.dumps(new[key])}"))
    return findings


def main() -> int:
    args = sys.argv[1:]
    if args[:1] in (["-h"], ["--help"]):
        print(__doc__)
        return 0
    if args[:1] == ["--pr"] and len(args) == 2:
        ref = f"refs/remotes/origin/pr/{args[1]}"
        git("fetch", "--quiet", "origin", f"+refs/pull/{args[1]}/head:{ref}")
        git("fetch", "--quiet", "origin", "prerelease")
        head = ref
        base = git("merge-base", "origin/prerelease", head).strip()
    else:
        head = args[1] if len(args) > 1 else "HEAD"
        base = git("merge-base", args[0] if args else "origin/prerelease", head).strip()

    findings = gone_commands(base, head, diff_lines(base, head)) + options(base, head)
    blocking = [f for f in findings if f[0] in ("GONE", "OPTION")]
    for kind, path, what in findings:
        print(f"{kind:8} {path}: {what}")
    if blocking:
        print(f"\n{len(blocking)} path(s) other distributions may have lost. For each, show where Arch, Fedora, "
              "Debian, Void and Nix get it now, or keep the old path behind a capability check.")
        return 1
    print("No system command or option lost." if not findings else "\nNothing lost; DEFAULT lines are for reading.")
    return 0


if __name__ == "__main__":
    sys.exit(main())

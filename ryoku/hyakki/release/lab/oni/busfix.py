#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""busfix.py: what the lab's D-Bus fixtures (mprisfix.py, notifyfix.py and
tabwin.py) share. Imported, never run.

lab_address(env)   the session bus address a fixture may use: it must be a
                   unix socket under /dev/shm (the lab's $LAB_RT/bus or a
                   selftest's own bus). Anything else, including no address,
                   the user's /run/user/UID/bus or an abstract socket, is
                   refused, so a fixture started by mistake outside the lab
                   never registers a name on the live session.
connect(address)   a message-bus connection to exactly that address (never
                   Gio.bus_get, which could autolaunch or pick the live bus).
PrivateBus(dir)    for selftests: a dbus-daemon with its own config (no
                   service directories, so nothing is ever activated)
                   listening at DIR/bus, started with a clean environment and
                   stopped on exit.
"""
import os
import subprocess
import time

ROOT = "/dev/shm/"
CONFIG = """<!DOCTYPE busconfig PUBLIC "-//freedesktop//DTD D-BUS Bus Configuration 1.0//EN"
 "http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
<busconfig>
  <type>session</type>
  <listen>unix:path={path}</listen>
  <auth>EXTERNAL</auth>
  <policy context="default">
    <allow send_destination="*" eavesdrop="true"/>
    <allow eavesdrop="true"/>
    <allow own="*"/>
  </policy>
</busconfig>
"""


class Refused(Exception):
    pass


def lab_address(env=None):
    env = os.environ if env is None else env
    address = env.get("DBUS_SESSION_BUS_ADDRESS", "")
    if not address.startswith("unix:path=" + ROOT) or "," in address or ";" in address:
        raise Refused("DBUS_SESSION_BUS_ADDRESS must be a unix:path= socket under %s (got %r); "
                      "the fixtures never use the live session bus" % (ROOT, address))
    path = address[len("unix:path="):]
    if "/../" in path or path.endswith("/..") or not os.path.isabs(path):
        raise Refused("bus path %r is not a plain absolute path" % path)
    return address


def connect(address):
    from gi.repository import Gio

    flags = Gio.DBusConnectionFlags.AUTHENTICATION_CLIENT | Gio.DBusConnectionFlags.MESSAGE_BUS_CONNECTION
    return Gio.DBusConnection.new_for_address_sync(address, flags, None, None)


class PrivateBus:
    def __init__(self, directory):
        self.dir = directory
        self.path = os.path.join(directory, "bus")
        self.address = "unix:path=" + self.path
        self.proc = None

    def __enter__(self):
        if not self.dir.startswith(ROOT):
            raise Refused("a selftest bus lives under %s" % ROOT)
        conf = os.path.join(self.dir, "bus.conf")
        with open(conf, "w", encoding="utf-8") as fh:
            fh.write(CONFIG.format(path=self.path))
        self.proc = subprocess.Popen(
            ["dbus-daemon", "--config-file=" + conf, "--nofork", "--nopidfile"],
            env={"PATH": "/usr/bin:/bin", "HOME": self.dir}, stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL, stderr=open(os.path.join(self.dir, "bus.log"), "w"))
        deadline = time.monotonic() + 5
        while not os.path.exists(self.path):
            if self.proc.poll() is not None or time.monotonic() > deadline:
                raise RuntimeError("the private dbus-daemon did not start, see %s/bus.log" % self.dir)
            time.sleep(0.02)
        return self

    def env(self, **extra):
        """A clean child environment that sees only this bus."""
        out = {"PATH": "/usr/bin:/bin", "HOME": self.dir, "LANG": "C.UTF-8",
               "XDG_RUNTIME_DIR": self.dir, "DBUS_SESSION_BUS_ADDRESS": self.address,
               "GIO_USE_VFS": "local", "GVFS_DISABLE_FUSE": "1"}
        out.update(extra)
        return out

    def __exit__(self, *exc):
        if self.proc and self.proc.poll() is None:
            self.proc.terminate()
            try:
                self.proc.wait(5)
            except subprocess.TimeoutExpired:
                self.proc.kill()
                self.proc.wait()
        return False


def wait_line(proc, want, timeout=10.0):
    """Read proc's stdout until a line equals WANT; False on exit or timeout."""
    import selectors

    sel = selectors.DefaultSelector()
    sel.register(proc.stdout, selectors.EVENT_READ)
    deadline = time.monotonic() + timeout
    buf = b""
    try:
        while time.monotonic() < deadline:
            if not sel.select(max(0.0, deadline - time.monotonic())):
                break
            chunk = os.read(proc.stdout.fileno(), 4096)
            if not chunk:
                return False
            buf += chunk
            lines = buf.split(b"\n")
            buf = lines.pop()
            if any(line.decode(errors="replace").strip() == want for line in lines):
                return True
        return False
    finally:
        sel.unregister(proc.stdout)


def read_jsonl(path):
    import json

    try:
        with open(path, encoding="utf-8") as fh:
            return [json.loads(line) for line in fh if line.strip()]
    except FileNotFoundError:
        return []

#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""mprisfix.py: the lab's media player. It plays nothing; it is an MPRIS
player on the lab's private session bus that records every call, so Phase 13
can prove "pause the music" reached a player without touching a real one.

mprisfix.py serve --log FILE [--name ID] [--status Playing|Paused|Stopped]
    Connect to DBUS_SESSION_BUS_ADDRESS, which must be a socket under
    /dev/shm (busfix.lab_address; the live session bus is refused), own
    org.mpris.MediaPlayer2.ID (default "onilab") and serve
    /org/mpris/MediaPlayer2 with the org.mpris.MediaPlayer2 and
    org.mpris.MediaPlayer2.Player interfaces. PlaybackStatus starts at
    --status (default Playing). PlayPause, Play, Pause and Stop change it and
    emit PropertiesChanged; Next, Previous, Seek, SetPosition, OpenUri, Raise
    and Quit change nothing (CanQuit and CanRaise are false). Setting Volume
    stores it. Each method call and property write is appended to FILE as
    one JSON line:
      {"t", "sender", "interface", "member", "args", "before", "after"}
    where before/after are PlaybackStatus. Prints "ready" once the name is
    owned; exits 1 if another owner holds it, 0 on SIGTERM or SIGINT.
mprisfix.py selftest
    Start a private dbus-daemon under /dev/shm, serve there and drive the
    player as a client; exit 0 when every check passes.
"""
import json
import os
import signal
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.dont_write_bytecode = True  # the lab writes nothing into the repository
sys.path.insert(0, HERE)
import busfix  # noqa: E402

PATH = "/org/mpris/MediaPlayer2"
ROOT_IFACE = "org.mpris.MediaPlayer2"
PLAYER_IFACE = "org.mpris.MediaPlayer2.Player"
XML = """<node>
  <interface name="org.mpris.MediaPlayer2">
    <method name="Raise"/>
    <method name="Quit"/>
    <property name="CanQuit" type="b" access="read"/>
    <property name="CanRaise" type="b" access="read"/>
    <property name="HasTrackList" type="b" access="read"/>
    <property name="Identity" type="s" access="read"/>
    <property name="SupportedUriSchemes" type="as" access="read"/>
    <property name="SupportedMimeTypes" type="as" access="read"/>
  </interface>
  <interface name="org.mpris.MediaPlayer2.Player">
    <method name="Next"/>
    <method name="Previous"/>
    <method name="Pause"/>
    <method name="PlayPause"/>
    <method name="Stop"/>
    <method name="Play"/>
    <method name="Seek"><arg name="Offset" type="x" direction="in"/></method>
    <method name="SetPosition">
      <arg name="TrackId" type="o" direction="in"/><arg name="Position" type="x" direction="in"/>
    </method>
    <method name="OpenUri"><arg name="Uri" type="s" direction="in"/></method>
    <signal name="Seeked"><arg name="Position" type="x"/></signal>
    <property name="PlaybackStatus" type="s" access="read"/>
    <property name="Rate" type="d" access="read"/>
    <property name="Metadata" type="a{sv}" access="read"/>
    <property name="Volume" type="d" access="readwrite"/>
    <property name="Position" type="x" access="read"/>
    <property name="MinimumRate" type="d" access="read"/>
    <property name="MaximumRate" type="d" access="read"/>
    <property name="CanGoNext" type="b" access="read"/>
    <property name="CanGoPrevious" type="b" access="read"/>
    <property name="CanPlay" type="b" access="read"/>
    <property name="CanPause" type="b" access="read"/>
    <property name="CanSeek" type="b" access="read"/>
    <property name="CanControl" type="b" access="read"/>
  </interface>
</node>"""
STATUSES = ("Playing", "Paused", "Stopped")
TRACK = "/dev/ryoku/oni/lab/track/1"


def serve(log, name, status):
    from gi.repository import Gio, GLib

    try:
        address = busfix.lab_address()
    except busfix.Refused as e:
        print("mprisfix.py: %s" % e, file=sys.stderr)
        return 2
    conn = busfix.connect(address)
    node = Gio.DBusNodeInfo.new_for_xml(XML)
    state = {"status": status, "volume": 0.5}
    loop = GLib.MainLoop()
    rc = [0]

    def write(sender, iface, member, args, before):
        line = {"t": time.time(), "sender": sender, "interface": iface, "member": member, "args": args,
                "before": before, "after": state["status"]}
        with open(log, "a", encoding="utf-8") as fh:
            fh.write(json.dumps(line) + "\n")

    def changed(props):
        conn.emit_signal(None, PATH, "org.freedesktop.DBus.Properties", "PropertiesChanged",
                         GLib.Variant("(sa{sv}as)", (PLAYER_IFACE, props, [])))

    def props(iface):
        if iface == ROOT_IFACE:
            return {"CanQuit": GLib.Variant("b", False), "CanRaise": GLib.Variant("b", False),
                    "HasTrackList": GLib.Variant("b", False), "Identity": GLib.Variant("s", "Oni lab player"),
                    "SupportedUriSchemes": GLib.Variant("as", []), "SupportedMimeTypes": GLib.Variant("as", [])}
        return {"PlaybackStatus": GLib.Variant("s", state["status"]), "Rate": GLib.Variant("d", 1.0),
                "Metadata": GLib.Variant("a{sv}", {"mpris:trackid": GLib.Variant("o", TRACK),
                                                   "xesam:title": GLib.Variant("s", "Lab tone")}),
                "Volume": GLib.Variant("d", state["volume"]), "Position": GLib.Variant("x", 0),
                "MinimumRate": GLib.Variant("d", 1.0), "MaximumRate": GLib.Variant("d", 1.0),
                "CanGoNext": GLib.Variant("b", True), "CanGoPrevious": GLib.Variant("b", True),
                "CanPlay": GLib.Variant("b", True), "CanPause": GLib.Variant("b", True),
                "CanSeek": GLib.Variant("b", False), "CanControl": GLib.Variant("b", True)}

    def method(connection, sender, path, iface, member, params, invocation):
        before = state["status"]
        if iface == PLAYER_IFACE:
            nxt = {"PlayPause": "Paused" if before == "Playing" else "Playing", "Play": "Playing",
                   "Pause": "Paused" if before == "Playing" else before, "Stop": "Stopped"}.get(member, before)
            state["status"] = nxt
        write(sender, iface, member, params.unpack() if params is not None else [], before)
        if state["status"] != before:
            changed({"PlaybackStatus": GLib.Variant("s", state["status"])})
        invocation.return_value(None)

    def get_property(connection, sender, path, iface, prop):
        return props(iface).get(prop)

    def set_property(connection, sender, path, iface, prop, value):
        if iface != PLAYER_IFACE or prop != "Volume":
            return False
        before = state["status"]
        state["volume"] = max(0.0, min(1.0, value.unpack()))
        write(sender, "org.freedesktop.DBus.Properties", "Set", [iface, prop, state["volume"]], before)
        changed({"Volume": GLib.Variant("d", state["volume"])})
        return True

    for info in node.interfaces:
        conn.register_object(PATH, info, method, get_property, set_property)

    def acquired(*_):
        print("ready", flush=True)

    def lost(*_):
        print("mprisfix.py: org.mpris.MediaPlayer2.%s is owned by someone else" % name, file=sys.stderr)
        rc[0] = 1
        loop.quit()

    Gio.bus_own_name_on_connection(conn, "org.mpris.MediaPlayer2." + name,
                                   Gio.BusNameOwnerFlags.DO_NOT_QUEUE, acquired, lost)
    for sig in (signal.SIGTERM, signal.SIGINT):
        GLib.unix_signal_add(GLib.PRIORITY_DEFAULT, sig, lambda *_: (loop.quit(), False)[1])
    loop.run()
    return rc[0]


def selftest():
    import shutil
    import subprocess
    import tempfile

    from gi.repository import Gio, GLib

    tmp = tempfile.mkdtemp(prefix="mprisfix-", dir="/dev/shm")
    me = os.path.abspath(__file__)
    log = os.path.join(tmp, "mpris.jsonl")
    fails = []

    def check(label, ok, value=None):
        print(("ok   " if ok else "FAIL ") + label + ("" if ok else "  (%r)" % (value,)), flush=True)
        if not ok:
            fails.append(label)

    try:
        with busfix.PrivateBus(tmp) as bus:
            r = subprocess.run([sys.executable, me, "serve", "--log", log],
                               env=bus.env(DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/0/bus"),
                               capture_output=True, text=True, timeout=10)
            check("a bus outside /dev/shm is refused", r.returncode == 2 and not os.path.exists(log), r.stderr)
            proc = subprocess.Popen([sys.executable, me, "serve", "--log", log], env=bus.env(),
                                    stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                    stderr=open(os.path.join(tmp, "serve.log"), "w"))
            check("the player owns its name and prints ready", busfix.wait_line(proc, "ready"))
            second = subprocess.run([sys.executable, me, "serve", "--log", log], env=bus.env(),
                                    capture_output=True, text=True, timeout=10)
            check("a second player for the same name exits 1", second.returncode == 1, second)

            conn = busfix.connect(bus.address)
            seen = []
            conn.signal_subscribe(None, "org.freedesktop.DBus.Properties", "PropertiesChanged", "/org/mpris/MediaPlayer2",
                                  None, Gio.DBusSignalFlags.NONE, lambda *a: seen.append(a[-1].unpack()))
            dest = "org.mpris.MediaPlayer2.onilab"

            def call(iface, member, args=None, sig=None):
                return conn.call_sync(dest, PATH, iface, member, args, GLib.VariantType(sig) if sig else None,
                                      Gio.DBusCallFlags.NONE, 5000, None)

            def status():
                return call("org.freedesktop.DBus.Properties", "Get", GLib.Variant("(ss)", (PLAYER_IFACE, "PlaybackStatus")),
                            "(v)").unpack()[0]

            def pump():
                ctx = GLib.MainContext.default()
                deadline = time.monotonic() + 1
                while time.monotonic() < deadline:
                    while ctx.pending():
                        ctx.iteration(False)
                    time.sleep(0.02)

            names = conn.call_sync("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus",
                                   "ListNames", None, None, Gio.DBusCallFlags.NONE, 5000, None).unpack()[0]
            check("the player is listed on the private bus", dest in names, names)
            check("it starts Playing", status() == "Playing")
            call(PLAYER_IFACE, "PlayPause")
            check("PlayPause pauses", status() == "Paused")
            call(PLAYER_IFACE, "PlayPause")
            check("PlayPause again plays", status() == "Playing")
            call(PLAYER_IFACE, "Pause")
            call(PLAYER_IFACE, "Pause")
            check("Pause is idempotent", status() == "Paused")
            call("org.freedesktop.DBus.Properties", "Set",
                 GLib.Variant("(ssv)", (PLAYER_IFACE, "Volume", GLib.Variant("d", 0.25))))
            vol = call("org.freedesktop.DBus.Properties", "Get", GLib.Variant("(ss)", (PLAYER_IFACE, "Volume")),
                       "(v)").unpack()[0]
            check("Volume is stored", vol == 0.25, vol)
            every = call("org.freedesktop.DBus.Properties", "GetAll", GLib.Variant("(s)", (ROOT_IFACE,)),
                         "(a{sv})").unpack()[0]
            check("the root interface names the lab player", every.get("Identity") == "Oni lab player", every)
            call(ROOT_IFACE, "Quit")
            check("Quit is recorded but does not stop the player (CanQuit is false)", proc.poll() is None)
            pump()
            check("PropertiesChanged reports each status change",
                  [s[1].get("PlaybackStatus") for s in seen if "PlaybackStatus" in s[1]]
                  == ["Paused", "Playing", "Paused"], seen)
            rows = busfix.read_jsonl(log)
            check("every call is recorded with the status before and after",
                  [(r["member"], r["before"], r["after"]) for r in rows]
                  == [("PlayPause", "Playing", "Paused"), ("PlayPause", "Paused", "Playing"),
                      ("Pause", "Playing", "Paused"), ("Pause", "Paused", "Paused"),
                      ("Set", "Paused", "Paused"), ("Quit", "Paused", "Paused")], rows)
            check("only PlayPause was counted as PlayPause",
                  sum(1 for r in rows if r["member"] == "PlayPause") == 2)
            check("records carry the caller's unique name", all(r["sender"].startswith(":") for r in rows), rows)
            proc.terminate()
            try:
                proc.wait(5)
            except subprocess.TimeoutExpired:
                proc.kill()
            check("SIGTERM exits 0", proc.returncode == 0, proc.returncode)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("mprisfix selftest: %s" % ("FAIL: " + ", ".join(fails) if fails else "all passed"))
    return 1 if fails else 0


def main(argv):
    if argv[:1] == ["selftest"]:
        return selftest()
    if argv[:1] != ["serve"]:
        print(__doc__.split("\n\n")[1], file=sys.stderr)
        return 2
    log, name, status, i = None, "onilab", "Playing", 1
    while i < len(argv):
        if argv[i] in ("--log", "--name", "--status") and i + 1 < len(argv):
            log, name, status = {"--log": (argv[i + 1], name, status), "--name": (log, argv[i + 1], status),
                                 "--status": (log, name, argv[i + 1])}[argv[i]]
            i += 2
            continue
        print("mprisfix.py: unknown argument %s" % argv[i], file=sys.stderr)
        return 2
    if not log or not os.path.isabs(log) or status not in STATUSES or not name.replace("_", "").isalnum():
        print("usage: mprisfix.py serve --log /abs/FILE [--name ID] [--status Playing|Paused|Stopped]", file=sys.stderr)
        return 2
    return serve(log, name, status)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

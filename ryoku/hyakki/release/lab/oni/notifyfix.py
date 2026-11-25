#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""notifyfix.py: the lab's notification server. It shows nothing; it owns
org.freedesktop.Notifications on the lab's private session bus and records
every notification, so Phase 13 can prove a hidden Oni still announces a
reminder without reaching the desktop's real notification daemon.

notifyfix.py serve --log FILE
    Connect to DBUS_SESSION_BUS_ADDRESS, which must be a socket under
    /dev/shm (busfix.lab_address; the live session bus is refused), own
    org.freedesktop.Notifications and serve /org/freedesktop/Notifications:
      Notify(susssasa{sv}i) -> u   ids count up from 1; a known replaces_id
                                   is reused
      CloseNotification(u)         emits NotificationClosed(id, 3)
      GetCapabilities() -> as      ["body"]
      GetServerInformation() -> (ssss)
                                   ("Oni lab notifications", "oni-lab",
                                    "1", "1.2")
    Each call is appended to FILE as one JSON line {"t", "sender", "member",
    ...}; a Notify line also holds "app_name", "replaces_id", "icon",
    "summary", "body", "actions", "hints" (basic values as they are, any
    other value as its type signature, so no image bytes are kept),
    "timeout" and the "id" returned. Prints "ready" once the name is owned;
    exits 1 if another owner holds it, 0 on SIGTERM or SIGINT.
notifyfix.py selftest
    Start a private dbus-daemon under /dev/shm, serve there and send
    notifications as a client; exit 0 when every check passes.
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

NAME = "org.freedesktop.Notifications"
PATH = "/org/freedesktop/Notifications"
XML = """<node>
  <interface name="org.freedesktop.Notifications">
    <method name="Notify">
      <arg name="app_name" type="s" direction="in"/>
      <arg name="replaces_id" type="u" direction="in"/>
      <arg name="app_icon" type="s" direction="in"/>
      <arg name="summary" type="s" direction="in"/>
      <arg name="body" type="s" direction="in"/>
      <arg name="actions" type="as" direction="in"/>
      <arg name="hints" type="a{sv}" direction="in"/>
      <arg name="expire_timeout" type="i" direction="in"/>
      <arg name="id" type="u" direction="out"/>
    </method>
    <method name="CloseNotification"><arg name="id" type="u" direction="in"/></method>
    <method name="GetCapabilities"><arg name="capabilities" type="as" direction="out"/></method>
    <method name="GetServerInformation">
      <arg name="name" type="s" direction="out"/><arg name="vendor" type="s" direction="out"/>
      <arg name="version" type="s" direction="out"/><arg name="spec_version" type="s" direction="out"/>
    </method>
    <signal name="NotificationClosed"><arg name="id" type="u"/><arg name="reason" type="u"/></signal>
    <signal name="ActionInvoked"><arg name="id" type="u"/><arg name="action_key" type="s"/></signal>
  </interface>
</node>"""
BASIC = set("bynqiuxtdsog")


def hint(value):
    sig = value.get_type_string()
    return value.unpack() if sig in BASIC else "<%s>" % sig


def serve(log):
    from gi.repository import Gio, GLib

    try:
        address = busfix.lab_address()
    except busfix.Refused as e:
        print("notifyfix.py: %s" % e, file=sys.stderr)
        return 2
    conn = busfix.connect(address)
    node = Gio.DBusNodeInfo.new_for_xml(XML)
    state = {"next": 1, "open": set()}
    loop = GLib.MainLoop()
    rc = [0]

    def write(line):
        line = dict({"t": time.time()}, **line)
        with open(log, "a", encoding="utf-8") as fh:
            fh.write(json.dumps(line) + "\n")

    def method(connection, sender, path, iface, member, params, invocation):
        if member == "Notify":
            app, replaces, icon, summary, body, actions, _hints, timeout = (
                params.get_child_value(i) for i in range(8))
            replaces = replaces.get_uint32()
            if replaces and replaces in state["open"]:
                nid = replaces
            else:
                nid = state["next"]
                state["next"] += 1
            state["open"].add(nid)
            hints = {}
            for i in range(_hints.n_children()):
                entry = _hints.get_child_value(i)
                hints[entry.get_child_value(0).get_string()] = hint(entry.get_child_value(1).get_variant())
            write({"sender": sender, "member": member, "app_name": app.get_string(), "replaces_id": replaces,
                   "icon": icon.get_string(), "summary": summary.get_string(), "body": body.get_string(),
                   "actions": actions.unpack(), "hints": hints, "timeout": timeout.get_int32(), "id": nid})
            invocation.return_value(GLib.Variant("(u)", (nid,)))
        elif member == "CloseNotification":
            nid = params.unpack()[0]
            write({"sender": sender, "member": member, "id": nid})
            if nid in state["open"]:
                state["open"].discard(nid)
                conn.emit_signal(None, PATH, NAME, "NotificationClosed", GLib.Variant("(uu)", (nid, 3)))
            invocation.return_value(None)
        elif member == "GetCapabilities":
            write({"sender": sender, "member": member})
            invocation.return_value(GLib.Variant("(as)", (["body"],)))
        elif member == "GetServerInformation":
            write({"sender": sender, "member": member})
            invocation.return_value(GLib.Variant("(ssss)", ("Oni lab notifications", "oni-lab", "1", "1.2")))
        else:
            invocation.return_dbus_error("org.freedesktop.DBus.Error.UnknownMethod", member)

    conn.register_object(PATH, node.interfaces[0], method, None, None)

    def acquired(*_):
        print("ready", flush=True)

    def lost(*_):
        print("notifyfix.py: %s is owned by someone else" % NAME, file=sys.stderr)
        rc[0] = 1
        loop.quit()

    Gio.bus_own_name_on_connection(conn, NAME, Gio.BusNameOwnerFlags.DO_NOT_QUEUE, acquired, lost)
    for sig in (signal.SIGTERM, signal.SIGINT):
        GLib.unix_signal_add(GLib.PRIORITY_DEFAULT, sig, lambda *_: (loop.quit(), False)[1])
    loop.run()
    return rc[0]


def selftest():
    import shutil
    import subprocess
    import tempfile

    from gi.repository import Gio, GLib

    tmp = tempfile.mkdtemp(prefix="notifyfix-", dir="/dev/shm")
    me = os.path.abspath(__file__)
    log = os.path.join(tmp, "notify.jsonl")
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
            r = subprocess.run([sys.executable, me, "serve", "--log", log], env=bus.env(DBUS_SESSION_BUS_ADDRESS=""),
                               capture_output=True, text=True, timeout=10)
            check("no bus address is refused", r.returncode == 2, r.stderr)
            proc = subprocess.Popen([sys.executable, me, "serve", "--log", log], env=bus.env(),
                                    stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                    stderr=open(os.path.join(tmp, "serve.log"), "w"))
            check("the server owns org.freedesktop.Notifications and prints ready", busfix.wait_line(proc, "ready"))
            second = subprocess.run([sys.executable, me, "serve", "--log", log], env=bus.env(),
                                    capture_output=True, text=True, timeout=10)
            check("a second server exits 1", second.returncode == 1, second)

            conn = busfix.connect(bus.address)
            closed = []
            conn.signal_subscribe(NAME, NAME, "NotificationClosed", PATH, None, Gio.DBusSignalFlags.NONE,
                                  lambda *a: closed.append(a[-1].unpack()))

            def call(member, args=None, sig=None):
                return conn.call_sync(NAME, PATH, NAME, member, args, GLib.VariantType(sig) if sig else None,
                                      Gio.DBusCallFlags.NONE, 5000, None).unpack()

            def notify(summary, body, replaces=0, hints=None):
                return call("Notify", GLib.Variant("(susssasa{sv}i)", (
                    "Oni", replaces, "", summary, body, [], hints or {}, -1)), "(u)")[0]

            caps = call("GetCapabilities", None, "(as)")[0]
            check("capabilities are body only", caps == ["body"], caps)
            info = call("GetServerInformation", None, "(ssss)")
            check("server information names the lab", info[0] == "Oni lab notifications", info)
            a = notify("Reminder", "stretch", hints={"urgency": GLib.Variant("y", 1),
                                                    "image-data": GLib.Variant("(iiibiiay)", (1, 1, 4, True, 8, 4, b"\0\0\0\0"))})
            b = notify("Time's up.", "")
            c = notify("Reminder", "stretch again", replaces=a)
            d = notify("Reminder", "unknown id", replaces=99)
            check("ids count up; a known replaces_id is reused; an unknown one is not",
                  (a, b, c, d) == (1, 2, 1, 3), (a, b, c, d))
            call("CloseNotification", GLib.Variant("(u)", (2,)))
            ctx = GLib.MainContext.default()
            deadline = time.monotonic() + 1
            while time.monotonic() < deadline and not closed:
                while ctx.pending():
                    ctx.iteration(False)
                time.sleep(0.02)
            check("CloseNotification emits NotificationClosed with reason 3", closed == [(2, 3)], closed)
            rows = busfix.read_jsonl(log)
            notes = [r for r in rows if r["member"] == "Notify"]
            check("every Notify is recorded with its words and returned id",
                  [(r["summary"], r["body"], r["id"]) for r in notes]
                  == [("Reminder", "stretch", 1), ("Time's up.", "", 2), ("Reminder", "stretch again", 1),
                      ("Reminder", "unknown id", 3)], notes)
            check("hints keep basic values and only the type of anything else (no image bytes)",
                  notes[0]["hints"] == {"urgency": 1, "image-data": "<(iiibiiay)>"}, notes[0]["hints"])
            check("the other calls are recorded in order",
                  [r["member"] for r in rows if r["member"] != "Notify"]
                  == ["GetCapabilities", "GetServerInformation", "CloseNotification"], rows)
            proc.terminate()
            try:
                proc.wait(5)
            except subprocess.TimeoutExpired:
                proc.kill()
            check("SIGTERM exits 0", proc.returncode == 0, proc.returncode)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("notifyfix selftest: %s" % ("FAIL: " + ", ".join(fails) if fails else "all passed"))
    return 1 if fails else 0


def main(argv):
    if argv[:1] == ["selftest"]:
        return selftest()
    if len(argv) != 3 or argv[0] != "serve" or argv[1] != "--log" or not os.path.isabs(argv[2]):
        print("usage: notifyfix.py serve --log /abs/FILE | notifyfix.py selftest", file=sys.stderr)
        return 2
    return serve(argv[2])


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""appwin.py: Phase 15's fixture windows, GTK 4, one process per window. Each
writes what happened to it into a file outside the window, so the lab reads
what landed without a screenshot. None touches the clipboard or runs anything.

appwin.py target --app-id ID --title TITLE --state FILE.json [--cmd FIFO]
                 [--variant pin]
    The window Hands works in. A text view (with --variant pin: one password
    entry, title "Enter PIN") and a Mark button. FILE.json is rewritten whole
    (temporary file, rename) after every change:
        {"pid", "app_id", "title", "text", "marks", "focus", "variant"}
    "focus" counts the times the window gained keyboard focus. --cmd is a
    FIFO the lab writes lines into: "title TEXT" retitles the window (title
    theft), "quit" closes it. Prints "ready" once mapped.
appwin.py sentinel --log FILE.jsonl
    dev.ryoku.OniSentinel: a window on the lab's own desktop that appends one
    line per input event it receives: {"t", "ev"} with ev key, button,
    motion, enter, leave or scroll. It never records which key: only that
    one arrived. Hands' isolation steps hold it to zero lines.
appwin.py files --log FILE.jsonl [ARG...]
    dev.ryoku.OniFiles: the lab's file manager. Appends {"t", "argv",
    "cwd"} once, then shows a window naming its first argument. Phase 15's
    show_file must reach it with a folder, never a file.
appwin.py selftest
    The pure helpers (argument parsing, the state file, the command parser),
    no display.
"""
import json
import os
import sys
import time

sys.dont_write_bytecode = True

TARGET_ID = "dev.ryoku.OniTarget"
SENTINEL_ID = "dev.ryoku.OniSentinel"
FILES_ID = "dev.ryoku.OniFiles"
PIN_TITLE = "Enter PIN"
MARK_LABEL = "Mark"


def usage():
    raise SystemExit(__doc__)


def parse(argv):
    """(mode, options, rest) or SystemExit."""
    if not argv:
        usage()
    mode, args, opts, rest = argv[0], argv[1:], {}, []
    known = {"target": ("--app-id", "--title", "--state", "--cmd", "--variant"),
             "sentinel": ("--log",), "files": ("--log",), "selftest": ()}
    if mode not in known:
        usage()
    i = 0
    while i < len(args):
        a = args[i]
        if a in known[mode] and i + 1 < len(args) and mode != "files":
            opts[a[2:]] = args[i + 1]
            i += 2
            continue
        if mode == "files" and a == "--log" and "log" not in opts and i + 1 < len(args):
            opts["log"] = args[i + 1]
            i += 2
            continue
        if mode == "files":
            rest.append(a)
            i += 1
            continue
        usage()
    need = {"target": ("app-id", "title", "state"), "sentinel": ("log",), "files": ("log",), "selftest": ()}[mode]
    if any(k not in opts for k in need):
        usage()
    for k in ("state", "log", "cmd"):
        if k in opts and not os.path.isabs(opts[k]):
            raise SystemExit(f"appwin.py: --{k} must be absolute")
    if opts.get("variant") not in (None, "pin"):
        usage()
    return mode, opts, rest


def write_state(path, state):
    tmp = path + ".part"
    with open(tmp, "w", encoding="utf-8") as fh:
        json.dump(state, fh)
    os.replace(tmp, path)


def append(path, obj):
    with open(path, "a", encoding="utf-8") as fh:
        fh.write(json.dumps(obj) + "\n")


def command(line):
    """('title', TEXT) | ('quit', None) | None for a FIFO line."""
    line = line.rstrip("\n")
    if line == "quit":
        return ("quit", None)
    if line.startswith("title ") and len(line) > 6:
        return ("title", line[6:][:200])
    return None


def gtk():
    os.environ.setdefault("GDK_BACKEND", "wayland")
    os.environ.setdefault("GSK_RENDERER", "cairo")
    os.environ.setdefault("GTK_A11Y", "none")
    import gi
    gi.require_version("Gtk", "4.0")
    from gi.repository import Gio, GLib, Gtk
    return Gio, GLib, Gtk


def run_target(o):
    Gio, GLib, Gtk = gtk()
    app_id, variant = o["app-id"], o.get("variant")
    GLib.set_prgname(app_id)
    title = PIN_TITLE if variant == "pin" else o["title"]
    st = {"pid": os.getpid(), "app_id": app_id, "title": title, "text": "", "marks": 0, "focus": 0, "variant": variant}

    def save():
        write_state(o["state"], st)

    def activate(app):
        win = Gtk.ApplicationWindow(application=app, title=title)
        win.set_default_size(640, 320)
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        if variant == "pin":
            entry = Gtk.PasswordEntry()
            entry.connect("changed", lambda e: (st.update(text=e.get_text()), save()))
            box.append(Gtk.Label(label="Account PIN"))
            box.append(entry)
            focus_to = entry
        else:
            view = Gtk.TextView(wrap_mode=Gtk.WrapMode.WORD_CHAR, monospace=True, vexpand=True)
            buf = view.get_buffer()
            buf.connect("changed", lambda b: (st.update(text=b.get_text(b.get_start_iter(), b.get_end_iter(), False)),
                                              save()))
            box.append(view)
            focus_to = view
        mark = Gtk.Button(label=MARK_LABEL)
        mark.connect("clicked", lambda *_: (st.update(marks=st["marks"] + 1), save()))
        box.append(mark)
        win.set_child(box)
        win.connect("notify::is-active", lambda w, _p: (st.update(focus=st["focus"] + 1), save())
                    if w.is_active() else None)
        win.connect("map", lambda *_: print("ready", flush=True))
        if o.get("cmd"):
            fd = os.open(o["cmd"], os.O_RDWR | os.O_NONBLOCK)  # read-write: no EOF while the lab has no writer
            pending = [""]

            def on_cmd(_fd, _cond):
                try:
                    pending[0] += os.read(fd, 4096).decode(errors="replace")
                except BlockingIOError:
                    return True
                while "\n" in pending[0]:
                    line, pending[0] = pending[0].split("\n", 1)
                    c = command(line)
                    if c and c[0] == "title":
                        win.set_title(c[1])
                        st["title"] = c[1]
                        save()
                    elif c and c[0] == "quit":
                        win.close()
                return True
            GLib.unix_fd_add_full(GLib.PRIORITY_DEFAULT, fd, GLib.IOCondition.IN, on_cmd)
        save()
        win.present()
        focus_to.grab_focus()

    app = Gtk.Application(application_id=app_id, flags=Gio.ApplicationFlags.NON_UNIQUE)
    app.connect("activate", activate)
    return app.run([sys.argv[0]])


def run_sentinel(o):
    Gio, GLib, Gtk = gtk()
    GLib.set_prgname(SENTINEL_ID)
    log = o["log"]

    def ev(kind):
        append(log, {"t": time.time(), "ev": kind})

    def activate(app):
        win = Gtk.ApplicationWindow(application=app, title="Oni lab sentinel")
        win.set_default_size(360, 240)
        area = Gtk.Box(hexpand=True, vexpand=True)
        area.set_focusable(True)
        keys = Gtk.EventControllerKey()
        keys.connect("key-pressed", lambda *_: ev("key") or False)
        keys.connect("key-released", lambda *_: ev("key") or False)
        win.add_controller(keys)
        click = Gtk.GestureClick(button=0)
        click.connect("pressed", lambda *_: ev("button"))
        click.connect("released", lambda *_: ev("button"))
        area.add_controller(click)
        motion = Gtk.EventControllerMotion()
        motion.connect("motion", lambda *_: ev("motion"))
        motion.connect("enter", lambda *_: ev("enter"))
        motion.connect("leave", lambda *_: ev("leave"))
        area.add_controller(motion)
        scroll = Gtk.EventControllerScroll(flags=Gtk.EventControllerScrollFlags.BOTH_AXES)
        scroll.connect("scroll", lambda *_: ev("scroll") or False)
        area.add_controller(scroll)
        area.append(Gtk.Label(label="sentinel", hexpand=True))
        win.set_child(area)
        win.connect("map", lambda *_: print("ready", flush=True))
        win.present()
        area.grab_focus()

    open(log, "a").close()
    app = Gtk.Application(application_id=SENTINEL_ID, flags=Gio.ApplicationFlags.NON_UNIQUE)
    app.connect("activate", activate)
    return app.run([sys.argv[0]])


def run_files(o, rest):
    append(o["log"], {"t": time.time(), "argv": rest, "cwd": os.getcwd(), "pid": os.getpid()})
    Gio, GLib, Gtk = gtk()
    GLib.set_prgname(FILES_ID)

    def activate(app):
        win = Gtk.ApplicationWindow(application=app, title="Files")
        win.set_default_size(480, 200)
        win.set_child(Gtk.Label(label=os.path.basename((rest or ["?"])[0].rstrip("/")) or "/"))
        win.present()

    app = Gtk.Application(application_id=FILES_ID, flags=Gio.ApplicationFlags.NON_UNIQUE)
    app.connect("activate", activate)
    return app.run([sys.argv[0]])


def selftest():
    checks = []

    def check(name, ok, value=None):
        checks.append({"name": name, "pass": bool(ok), "value": value})

    m, o, _ = parse(["target", "--app-id", TARGET_ID, "--title", "Kansai notes", "--state", "/dev/shm/x.json"])
    check("parse: target with its three options", m == "target" and o["app-id"] == TARGET_ID, o)
    m, o, r = parse(["files", "--log", "/dev/shm/f.jsonl", "/home/x/HandsFiles", "--select"])
    check("parse: files keeps every argument after --log as the logged argv",
          r == ["/home/x/HandsFiles", "--select"] and o["log"] == "/dev/shm/f.jsonl", r)
    bad = [["target", "--app-id", "a", "--title", "t", "--state", "rel.json"], ["target", "--app-id", "a"],
           ["sentinel"], ["bogus"], ["target", "--app-id", "a", "--title", "t", "--state", "/x", "--variant", "x"]]
    refused = 0
    for b in bad:
        try:
            parse(b)
        except SystemExit:
            refused += 1
    check("parse refuses a relative state, a missing option, an unknown mode or variant", refused == len(bad), refused)
    check("command reads title and quit, nothing else",
          command("title Changed\n") == ("title", "Changed") and command("quit") == ("quit", None)
          and command("title ") is None and command("rm -rf ~") is None)
    p = f"/dev/shm/appwin-selftest-{os.getpid()}.json"
    write_state(p, {"text": "a"})
    write_state(p, {"text": "b"})
    ok = json.load(open(p)) == {"text": "b"} and not os.path.exists(p + ".part")
    os.unlink(p)
    check("write_state replaces the file whole", ok)
    ok = all(c["pass"] for c in checks)
    print(json.dumps({"pass": ok, "checks": checks}, indent=2))
    return 0 if ok else 1


def main(argv):
    mode, o, rest = parse(argv)
    if mode == "selftest":
        return selftest()
    if mode == "target":
        return run_target(o)
    if mode == "sentinel":
        return run_sentinel(o)
    return run_files(o, rest)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

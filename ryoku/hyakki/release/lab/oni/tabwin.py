#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""tabwin.py: the lab's browser, one GTK 4 window holding a notebook of tabs
(app id dev.ryoku.OniBrowser). It loads nothing; a tab is a label naming the
address it was asked for. It exists so Phase 13 can prove "open a new tab"
through the browser's own launcher, as decision 1 requires: Oni sends no key
chord to any window.

tabwin.py --out /abs/DIR [--new-tab] [URL...]
    Like a real browser it is single-instance on the session bus. The first
    launch opens the window with one tab per URL given (one blank tab when
    none is). A later launch hands its command line to the running window
    over the bus and exits with the window's answer:
      a URL (about:blank included)  opens it in a new tab, which becomes the
                                    active one
      --new-tab                     opens a blank new tab
      nothing                       only raises the window; no tab is added
    Every change is written to DIR/tabs.json at once (a temporary file renamed
    over it):
      {"count": N, "active": I, "tabs": [{"title", "url"}], "launches": L,
       "remote": R, "seq": S, "pid": P}
    and every command line the window handled is appended to
    DIR/tabs-requests.jsonl as {"t", "argv", "remote", "action", "added",
    "count"} with action start, new_tab, raise or refused. --out given by a
    later launch is ignored: the first launch owns DIR. It prints "ready"
    once its window is mapped and exits when the compositor closes it.
    It needs a session bus under /dev/shm (busfix.lab_address): without one
    it could not be single-instance, so it refuses to start.
tabwin.py selftest
    Start a private dbus-daemon and a gtk4-broadwayd display under /dev/shm,
    run the window there and drive it with further launches; exit 0 when
    every check passes. Touches no session bus and no compositor.

The lab's desktop entry for it is written by launchfix.py install; its
Exec is "python3 /abs/tabwin.py --out /abs/DIR %u".
"""
import json
import os
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.dont_write_bytecode = True  # the lab writes nothing into the repository
sys.path.insert(0, HERE)
import busfix  # noqa: E402

APP_ID = "dev.ryoku.OniBrowser"
TITLE = "Oni lab browser"
BLANK = "about:blank"


def parse(argv):
    """argv without the program: (out, new_tab, urls, error)."""
    out, new_tab, urls, i = None, False, [], 0
    while i < len(argv):
        a = argv[i]
        if a == "--out":
            if i + 1 >= len(argv):
                return None, False, [], "--out needs a directory"
            out = argv[i + 1]
            i += 2
            continue
        if a == "--new-tab":
            new_tab = True
        elif a == "--":
            urls.extend(argv[i + 1:])
            break
        elif a.startswith("-"):
            return None, False, [], "unknown option %s" % a
        elif a:
            urls.append(a)
        i += 1
    return out, new_tab, urls, None


def title_of(url):
    if url == BLANK:
        return "New tab"
    rest = url.split("://", 1)[-1]
    return rest.split("/", 1)[0] or url


def run(argv):
    out, _, _, err = parse(argv[1:])
    if err or not out or not os.path.isabs(out) or not os.path.isdir(out):
        sys.exit("usage: tabwin.py --out /abs/existing/DIR [--new-tab] [URL...]" + (" (%s)" % err if err else ""))
    try:
        busfix.lab_address()
    except busfix.Refused as e:
        sys.exit("tabwin.py: %s" % e)
    os.environ.setdefault("GDK_BACKEND", "wayland")
    os.environ.setdefault("GSK_RENDERER", "cairo")
    os.environ.setdefault("GTK_A11Y", "none")

    import gi

    gi.require_version("Gtk", "4.0")
    from gi.repository import Gio, GLib, Gtk

    GLib.set_prgname(APP_ID)
    state = {"out": out, "win": None, "book": None, "tabs": [], "launches": 0, "remote": 0, "seq": 0}

    def save():
        book = state["book"]
        state["seq"] += 1
        value = {"count": len(state["tabs"]), "active": book.get_current_page() if book else -1,
                 "tabs": [{"title": t, "url": u} for t, u in state["tabs"]],
                 "launches": state["launches"], "remote": state["remote"], "seq": state["seq"],
                 "pid": os.getpid()}
        path = os.path.join(state["out"], "tabs.json")
        with open(path + ".part", "w", encoding="utf-8") as fh:
            json.dump(value, fh)
        os.replace(path + ".part", path)

    def record(args, remote, action, added):
        line = {"t": time.time(), "argv": args, "remote": remote, "action": action, "added": added,
                "count": len(state["tabs"])}
        with open(os.path.join(state["out"], "tabs-requests.jsonl"), "a", encoding="utf-8") as fh:
            fh.write(json.dumps(line) + "\n")

    def add_tab(url):
        book = state["book"]
        page = Gtk.Label(label=url, selectable=False)
        state["tabs"].append((title_of(url), url))
        n = book.append_page(page, Gtk.Label(label=title_of(url)))
        book.set_current_page(n)

    def window():
        win = Gtk.ApplicationWindow(application=app, title=TITLE)
        win.set_default_size(760, 420)
        book = Gtk.Notebook(scrollable=True)
        state["win"], state["book"] = win, book
        win.set_child(book)
        book.connect("switch-page", lambda *_: GLib.idle_add(lambda: (save(), False)[1]))
        win.connect("map", lambda *_: print("ready", flush=True))
        win.connect("close-request", lambda *_: (app.quit(), False)[1])
        return win

    def command_line(app_, cmdline):
        args = list(cmdline.get_arguments())
        remote = cmdline.get_is_remote()
        _, new_tab, urls, err = parse(args[1:])
        state["launches"] += 1
        if remote:
            state["remote"] += 1
        if err:
            record(args, remote, "refused", 0)
            save()
            cmdline.printerr_literal("tabwin.py: %s\n" % err) if hasattr(cmdline, "printerr_literal") else None
            return 2
        first = state["win"] is None
        wanted = urls + ([BLANK] if new_tab else [])
        if first:
            win = window()
            for u in wanted or [BLANK]:
                add_tab(u)
            action = "start"
            win.present()
        else:
            for u in wanted:
                add_tab(u)
            action = "new_tab" if wanted else "raise"
            state["win"].present()
        record(args, remote, action, len(wanted or [BLANK]) if first else len(wanted))
        save()
        return 0

    app = Gtk.Application(application_id=APP_ID, flags=Gio.ApplicationFlags.HANDLES_COMMAND_LINE)
    app.connect("command-line", command_line)
    try:
        app.register(None)
    except GLib.Error as e:
        sys.exit("tabwin.py: could not register on the session bus: %s" % e.message)
    if app.get_dbus_connection() is None and not app.get_is_remote():
        sys.exit("tabwin.py: no session bus; refusing to start a second, unconnected browser")
    return app.run(argv)


def selftest():
    import socket
    import subprocess
    import tempfile

    tmp = tempfile.mkdtemp(prefix="tabwin-", dir="/dev/shm")
    fails = []

    def check(name, ok, value=None):
        print(("ok   " if ok else "FAIL ") + name + ("" if ok else "  (%r)" % (value,)), flush=True)
        if not ok:
            fails.append(name)

    def tabs():
        try:
            with open(os.path.join(tmp, "out", "tabs.json"), encoding="utf-8") as fh:
                return json.load(fh)
        except (OSError, ValueError):
            return {}

    def wait_for(pred, timeout=10.0):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            v = tabs()
            if pred(v):
                return v
            time.sleep(0.05)
        return tabs()

    os.makedirs(os.path.join(tmp, "out"))
    me = os.path.abspath(__file__)
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    display = ":%d" % (60 + os.getpid() % 30)
    broadway = None
    try:
        with busfix.PrivateBus(tmp) as bus:
            env = bus.env(GDK_BACKEND="broadway", BROADWAY_DISPLAY=display, GSK_RENDERER="cairo",
                          GTK_A11Y="none")
            broadway = subprocess.Popen(["gtk4-broadwayd", "--address", "127.0.0.1", "--port", str(port), display],
                                        env=env, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                                        stderr=open(os.path.join(tmp, "broadway.log"), "w"))
            deadline = time.monotonic() + 5
            # gtk4-broadwayd names its socket after display number + 1
            sock = os.path.join(tmp, "broadway%d.socket" % (int(display[1:]) + 1))
            while not os.path.exists(sock) and time.monotonic() < deadline and broadway.poll() is None:
                time.sleep(0.02)
            check("gtk4-broadwayd listens in the private runtime directory", os.path.exists(sock), sock)

            out = os.path.join(tmp, "out")
            refused = subprocess.run([sys.executable, me, "--out", out],
                                     env=dict(env, DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/0/bus"),
                                     capture_output=True, text=True, timeout=10)
            check("a bus outside /dev/shm is refused before anything starts",
                  refused.returncode != 0 and "never use the live session bus" in refused.stderr
                  and not os.path.exists(os.path.join(out, "tabs.json")), refused.stderr)

            primary = subprocess.Popen([sys.executable, me, "--out", out], env=env,
                                       stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                       stderr=open(os.path.join(tmp, "tabwin.log"), "w"))
            check("the first launch maps its window and prints ready", busfix.wait_line(primary, "ready", 15))
            v = wait_for(lambda v: v.get("count") == 1)
            check("the first launch with no URL holds one blank tab",
                  v.get("count") == 1 and v["tabs"][0]["url"] == BLANK and v.get("active") == 0
                  and v.get("launches") == 1 and v.get("remote") == 0, v)

            def launch(*args):
                return subprocess.run([sys.executable, me, "--out", out] + list(args), env=env,
                                      capture_output=True, text=True, timeout=15)

            r = launch(BLANK)
            v = wait_for(lambda v: v.get("count") == 2)
            check("a launch with about:blank (the reflex new-tab argv) adds one tab and exits 0",
                  r.returncode == 0 and v.get("count") == 2 and v.get("active") == 1 and v.get("remote") == 1, (r, v))
            r = launch("https://example.org/a")
            v = wait_for(lambda v: v.get("count") == 3)
            check("a launch with a site opens it in a new, active tab",
                  r.returncode == 0 and v.get("count") == 3 and v.get("active") == 2
                  and v["tabs"][2] == {"title": "example.org", "url": "https://example.org/a"}, v)
            r = launch("--new-tab")
            v = wait_for(lambda v: v.get("count") == 4)
            check("--new-tab opens a blank tab", r.returncode == 0 and v.get("count") == 4, v)
            seq = v.get("seq")
            r = launch()
            v = wait_for(lambda v: v.get("launches") == 5)
            check("a bare launch only raises: no tab is added",
                  r.returncode == 0 and v.get("count") == 4 and v.get("launches") == 5 and v.get("seq", 0) > seq, v)
            r = launch("--bogus")
            time.sleep(0.3)
            v = tabs()
            check("an unknown option is refused by the launcher itself; the window adds nothing",
                  r.returncode != 0 and "unknown option" in r.stderr and v.get("count") == 4
                  and v.get("launches") == 5, (r.returncode, r.stderr, v))
            reqs = busfix.read_jsonl(os.path.join(out, "tabs-requests.jsonl"))
            check("every handled command line is recorded in order",
                  [q["action"] for q in reqs] == ["start", "new_tab", "new_tab", "new_tab", "raise"]
                  and [q["remote"] for q in reqs] == [False, True, True, True, True]
                  and reqs[1]["argv"][-1] == BLANK and [q["count"] for q in reqs] == [1, 2, 3, 4, 4], reqs)
            check("only one browser process holds the window", primary.poll() is None)
            primary.terminate()
            try:
                primary.wait(5)
            except subprocess.TimeoutExpired:
                primary.kill()
            check("the window exits on SIGTERM", primary.poll() is not None)
    finally:
        if broadway and broadway.poll() is None:
            broadway.terminate()
            broadway.wait(5)
        import shutil

        shutil.rmtree(tmp, ignore_errors=True)
    print("tabwin selftest: %s" % ("FAIL: " + ", ".join(fails) if fails else "all passed"))
    return 1 if fails else 0


if __name__ == "__main__":
    if sys.argv[1:2] == ["selftest"]:
        sys.exit(selftest())
    sys.exit(run(sys.argv))

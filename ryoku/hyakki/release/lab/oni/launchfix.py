#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""launchfix.py: the lab's launcher catalogue. Phase 13 lets Oni open apps
by name; the lab gives it two desktop entries of its own, so no real app is
ever started:

  dev.ryoku.OniNotes.desktop    "Lindqvist Notes", a text editor: Exec runs
                                textwin.py dev.ryoku.OniNotes "Lindqvist
                                notes" OUT/notes.txt (the window writes its
                                text there)
  dev.ryoku.OniBrowser.desktop  "Oni Lab Browser", Categories WebBrowser,
                                the http/https handler: Exec runs
                                tabwin.py --out OUT %u (a launch with a URL,
                                about:blank included, opens a new tab in the
                                running window; see tabwin.py)

launchfix.py install --data-home DIR --config-home DIR --out DIR [--here DIR]
    Write both entries to DATA_HOME/applications/ and make the browser the
    default for x-scheme-handler/http, x-scheme-handler/https and text/html
    in CONFIG_HOME/mimeapps.list. Every directory must be under /dev/shm.
    --here is where textwin.py and tabwin.py live (default: beside this
    file). OUT must exist. phase13.sh runs it with the lab's own XDG
    directories, as lab_app gives them to the daemon:
        python3 launchfix.py install --data-home "$ONI_LAB/data" \\
            --config-home "$ONI_LAB/cfg" --out "$ONI_LAB/out"
    so the catalogue reads $ONI_LAB/data/applications/*.desktop and
    $ONI_LAB/cfg/mimeapps.list. An existing mimeapps.list that differs is
    refused, never overwritten.
launchfix.py remove --data-home DIR ID
    Delete DATA_HOME/applications/ID.desktop (Phase 13's "an uninstalled app
    is skipped" step); exit 1 if it was not there.
launchfix.py selftest
    Install into a scratch directory under /dev/shm and check the entries
    parse as desktop entries, resolve as the default browser, and that their
    Exec lines (field codes handled as the reflex launcher does) map a notes
    window and add browser tabs on a private bus and gtk4-broadwayd display.
    It also builds scoperefuse.c and checks the systemd-run refusal: exit 1,
    the real tool's failure line, the log line, and the wrapped command never
    run. Exit 0 when every check passes.

The systemd-run stub: up.sh builds refuse.c for systemctl, ryoku,
ryoku-shell and ryoku-host; Phase 13 needs scoperefuse.c built the same way
as $ONI_LAB/bin/systemd-run (cc -O2 -Wall -Wextra -Werror
-DREFUSED_LOG="\\"$LAB_LOG/refused.log\\"").
"""
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.dont_write_bytecode = True  # the lab writes nothing into the repository
sys.path.insert(0, HERE)
import busfix  # noqa: E402

NOTES = "dev.ryoku.OniNotes"
BROWSER = "dev.ryoku.OniBrowser"
MIME = ("x-scheme-handler/http", "x-scheme-handler/https", "text/html")


def quote(arg):
    """One Exec argument (Desktop Entry spec, "The Exec key")."""
    if arg and not any(c in arg for c in " \t\n\"'\\><~|&;$*?#()`="):
        return arg
    return '"' + "".join("\\" + c if c in '"`$\\' else c for c in arg) + '"'


def escape(value):
    """A desktop-file string value: backslashes doubled (spec "Possible value types")."""
    return value.replace("\\", "\\\\")


def entries(here, out):
    notes_exec = " ".join(quote(a) for a in ["python3", os.path.join(here, "textwin.py"), NOTES, "Lindqvist notes",
                                             os.path.join(out, "notes.txt")])
    browser_exec = " ".join(quote(a) for a in ["python3", os.path.join(here, "tabwin.py"), "--out", out]) + " %u"
    return {
        NOTES: "\n".join([
            "[Desktop Entry]", "Type=Application", "Name=Lindqvist Notes", "GenericName=Text Editor",
            "Comment=The lab's launch target, a plain text window",
            "Keywords=notes;notepad;text;", "Exec=" + escape(notes_exec), "TryExec=python3",
            "Icon=accessories-text-editor", "Terminal=false", "Categories=Utility;TextEditor;",
            "StartupWMClass=" + NOTES, ""]),
        BROWSER: "\n".join([
            "[Desktop Entry]", "Type=Application", "Name=Oni Lab Browser", "GenericName=Web Browser",
            "Comment=The lab's browser, a window of tabs that loads nothing",
            "Keywords=browser;web;internet;", "Exec=" + escape(browser_exec), "TryExec=python3",
            "Icon=web-browser", "Terminal=false", "Categories=Network;WebBrowser;",
            "MimeType=" + "".join(m + ";" for m in MIME), "StartupWMClass=" + BROWSER, ""]),
    }


def mimeapps():
    return "[Default Applications]\n" + "".join("%s=%s.desktop\n" % (m, BROWSER) for m in MIME)


def under_shm(*paths):
    for p in paths:
        if not os.path.isabs(p) or not os.path.realpath(p).startswith(busfix.ROOT):
            raise SystemExit("launchfix.py: %s is not an absolute path under %s" % (p, busfix.ROOT))


def write(path, text):
    with open(path + ".part", "w", encoding="utf-8") as fh:
        fh.write(text)
    os.replace(path + ".part", path)


def install(data_home, config_home, out, here):
    under_shm(data_home, config_home, out)
    if not os.path.isdir(out):
        raise SystemExit("launchfix.py: OUT %s does not exist" % out)
    for name in ("textwin.py", "tabwin.py"):
        if not os.path.isfile(os.path.join(here, name)):
            raise SystemExit("launchfix.py: no %s in %s" % (name, here))
    mime = os.path.join(config_home, "mimeapps.list")
    if os.path.exists(mime):
        with open(mime, encoding="utf-8") as fh:
            if fh.read() != mimeapps():
                raise SystemExit("launchfix.py: %s exists and differs; not overwriting it" % mime)
    apps = os.path.join(data_home, "applications")
    os.makedirs(apps, exist_ok=True)
    os.makedirs(config_home, exist_ok=True)
    paths = []
    for app_id, text in entries(here, out).items():
        path = os.path.join(apps, app_id + ".desktop")
        write(path, text)
        paths.append(path)
    write(mime, mimeapps())
    for p in paths + [mime]:
        print(p)
    return 0


def remove(data_home, app_id):
    under_shm(data_home)
    if app_id not in (NOTES, BROWSER):
        raise SystemExit("launchfix.py: %s is not one of the lab's entries" % app_id)
    path = os.path.join(data_home, "applications", app_id + ".desktop")
    try:
        os.unlink(path)
    except FileNotFoundError:
        return 1
    return 0


def reflex_argv(exec_line, url=None):
    """The argv the reflex launcher builds from an Exec value: %u/%U/%f/%F
    become the one URL (appended when there is none), every other field code
    is dropped, %% is a literal %."""
    from gi.repository import GLib

    args = GLib.shell_parse_argv(exec_line)[1]
    out, placed = [], False
    for a in args:
        if a in ("%u", "%U", "%f", "%F"):
            if url is not None:
                out.append(url)
            placed = True
        elif len(a) == 2 and a[0] == "%" and a != "%%":
            continue
        else:
            out.append(a.replace("%%", "%"))
    if url is not None and not placed:
        out.append(url)
    return out


def selftest():
    import json
    import shutil
    import socket
    import subprocess
    import tempfile
    import time

    import gi

    gi.require_version("Gio", "2.0")
    from gi.repository import Gio

    try:
        gi.require_version("GioUnix", "2.0")
        from gi.repository import GioUnix

        DesktopAppInfo = GioUnix.DesktopAppInfo
    except (ValueError, ImportError):
        DesktopAppInfo = Gio.DesktopAppInfo

    tmp = tempfile.mkdtemp(prefix="launchfix-", dir="/dev/shm")
    fails = []

    def check(label, ok, value=None):
        print(("ok   " if ok else "FAIL ") + label + ("" if ok else "  (%r)" % (value,)), flush=True)
        if not ok:
            fails.append(label)

    procs = []
    try:
        data, cfg, out = (os.path.join(tmp, d) for d in ("data", "cfg", "out"))
        os.makedirs(out)
        me = os.path.abspath(__file__)
        r = subprocess.run([sys.executable, me, "install", "--data-home", data, "--config-home", cfg, "--out", out],
                           capture_output=True, text=True, timeout=10)
        check("install writes two entries and mimeapps.list", r.returncode == 0 and len(r.stdout.split()) == 3, r)
        r2 = subprocess.run([sys.executable, me, "install", "--data-home", data, "--config-home", cfg, "--out", out],
                            capture_output=True, text=True, timeout=10)
        check("a second install is idempotent", r2.returncode == 0, r2.stderr)
        r3 = subprocess.run([sys.executable, me, "install", "--data-home", "/tmp/x", "--config-home", cfg,
                             "--out", out], capture_output=True, text=True, timeout=10)
        check("a directory outside /dev/shm is refused", r3.returncode != 0 and not os.path.exists("/tmp/x"), r3.stderr)
        if shutil.which("desktop-file-validate"):
            for app_id in (NOTES, BROWSER):
                v = subprocess.run(["desktop-file-validate", os.path.join(data, "applications", app_id + ".desktop")],
                                   capture_output=True, text=True)
                check("desktop-file-validate accepts %s" % app_id, v.returncode == 0 and not v.stdout.strip(), v.stdout)
        notes = DesktopAppInfo.new_from_filename(os.path.join(data, "applications", NOTES + ".desktop"))
        browser = DesktopAppInfo.new_from_filename(os.path.join(data, "applications", BROWSER + ".desktop"))
        check("both entries parse", notes is not None and browser is not None)
        check("the notes entry is named Lindqvist Notes and runs textwin.py",
              notes.get_name() == "Lindqvist Notes" and reflex_argv(notes.get_commandline())
              == ["python3", os.path.join(HERE, "textwin.py"), NOTES, "Lindqvist notes", os.path.join(out, "notes.txt")],
              notes.get_commandline())
        check("the browser entry is a WebBrowser taking %u",
              "WebBrowser" in (browser.get_categories() or "") and browser.get_commandline().endswith(" %u")
              and browser.get_startup_wm_class() == BROWSER, browser.get_commandline())

        # the default-browser lookup reads the environment once, so ask in a child that sees only the lab dirs
        look = subprocess.run([sys.executable, "-c", "from gi.repository import Gio\n"
                               "for s in ('https','http'):\n"
                               "    a = Gio.AppInfo.get_default_for_uri_scheme(s); print(a.get_id() if a else None)"],
                              env={"PATH": "/usr/bin:/bin", "HOME": tmp, "XDG_DATA_HOME": data, "XDG_CONFIG_HOME": cfg,
                                   "XDG_DATA_DIRS": os.path.join(tmp, "none"), "XDG_CONFIG_DIRS": os.path.join(tmp, "none"),
                                   "GIO_USE_VFS": "local", "DBUS_SESSION_BUS_ADDRESS": "disabled:"},
                              capture_output=True, text=True, timeout=10)
        check("the lab browser is the http and https default",
              look.stdout.split() == [BROWSER + ".desktop"] * 2, (look.stdout, look.stderr))

        sock = socket.socket()
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
        sock.close()
        display = ":%d" % (30 + os.getpid() % 25)
        with busfix.PrivateBus(tmp) as bus:
            env = bus.env(GDK_BACKEND="broadway", BROADWAY_DISPLAY=display, GSK_RENDERER="cairo", GTK_A11Y="none",
                          XDG_DATA_HOME=data, XDG_CONFIG_HOME=cfg)
            bw = subprocess.Popen(["gtk4-broadwayd", "--address", "127.0.0.1", "--port", str(port), display],
                                  env=env, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                                  stderr=open(os.path.join(tmp, "broadway.log"), "w"))
            procs.append(bw)
            deadline = time.monotonic() + 5
            bsock = os.path.join(tmp, "broadway%d.socket" % (int(display[1:]) + 1))
            while not os.path.exists(bsock) and time.monotonic() < deadline:
                time.sleep(0.02)
            n = subprocess.Popen(reflex_argv(notes.get_commandline()), env=env, stdin=subprocess.DEVNULL,
                                 stdout=subprocess.PIPE, stderr=open(os.path.join(tmp, "notes.log"), "w"))
            procs.append(n)
            check("the notes entry's Exec maps a text window", busfix.wait_line(n, "ready", 15))
            check("the window's text file starts empty", open(os.path.join(out, "notes.txt")).read() == "")
            b = subprocess.Popen(reflex_argv(browser.get_commandline(), "about:blank"), env=env,
                                 stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                 stderr=open(os.path.join(tmp, "browser.log"), "w"))
            procs.append(b)
            check("the browser entry's Exec with about:blank maps the browser", busfix.wait_line(b, "ready", 15))
            again = subprocess.run(reflex_argv(browser.get_commandline(), "about:blank"), env=env,
                                   capture_output=True, text=True, timeout=15)
            site = subprocess.run(reflex_argv(browser.get_commandline(), "https://example.org/"), env=env,
                                  capture_output=True, text=True, timeout=15)
            deadline = time.monotonic() + 5
            tabs = {}
            while time.monotonic() < deadline:
                try:
                    tabs = json.load(open(os.path.join(out, "tabs.json")))
                except (OSError, ValueError):
                    pass
                if tabs.get("count") == 3:
                    break
                time.sleep(0.05)
            check("launching the entry again opens new tabs in the running browser",
                  again.returncode == 0 and site.returncode == 0 and tabs.get("count") == 3
                  and tabs.get("remote") == 2 and tabs["tabs"][2]["url"] == "https://example.org/", tabs)
        r = subprocess.run([sys.executable, me, "remove", "--data-home", data, NOTES], timeout=10)
        r2 = subprocess.run([sys.executable, me, "remove", "--data-home", data, NOTES], timeout=10)
        check("remove deletes an entry once", r.returncode == 0 and r2.returncode == 1
              and not os.path.exists(os.path.join(data, "applications", NOTES + ".desktop")))

        # scoperefuse.c, the lab's bin/systemd-run
        bindir = os.path.join(tmp, "bin")
        os.makedirs(bindir)
        refused = os.path.join(tmp, "refused.log")
        cc = subprocess.run(["cc", "-O2", "-Wall", "-Wextra", "-Werror", '-DREFUSED_LOG="%s"' % refused,
                             "-o", os.path.join(bindir, "systemd-run"), os.path.join(HERE, "scoperefuse.c")],
                            capture_output=True, text=True, timeout=60)
        check("scoperefuse.c builds warning-free", cc.returncode == 0, cc.stderr)
        marker = os.path.join(tmp, "ran")
        run = subprocess.run(["systemd-run", "--user", "--scope", "--collect", "--", "touch", marker],
                             env={"PATH": bindir + ":/usr/bin:/bin"}, capture_output=True, text=True, timeout=10)
        check("the stub exits 1 with systemd-run's own failure line",
              run.returncode == 1 and run.stderr == "Failed to start transient scope unit: Access denied\n", run)
        check("the wrapped command never runs", not os.path.exists(marker))
        check("the refusal is logged in refuse.c's format",
              open(refused).read() == "refused: systemd-run --user --scope --collect -- touch %s\n" % marker,
              open(refused).read() if os.path.exists(refused) else None)
    finally:
        for p in reversed(procs):
            if p.poll() is None:
                p.terminate()
                try:
                    p.wait(5)
                except subprocess.TimeoutExpired:
                    p.kill()
                    p.wait()
        shutil.rmtree(tmp, ignore_errors=True)
    print("launchfix selftest: %s" % ("FAIL: " + ", ".join(fails) if fails else "all passed"))
    return 1 if fails else 0


def main(argv):
    if argv[:1] == ["selftest"]:
        return selftest()
    opts, rest, i = {}, [], 1
    while i < len(argv):
        if argv[i] in ("--data-home", "--config-home", "--out", "--here") and i + 1 < len(argv):
            opts[argv[i]] = argv[i + 1]
            i += 2
        else:
            rest.append(argv[i])
            i += 1
    if argv[:1] == ["install"] and not rest and {"--data-home", "--config-home", "--out"} <= set(opts):
        return install(opts["--data-home"], opts["--config-home"], opts["--out"], opts.get("--here", HERE))
    if argv[:1] == ["remove"] and len(rest) == 1 and set(opts) == {"--data-home"}:
        return remove(opts["--data-home"], rest[0])
    print("usage: launchfix.py install --data-home DIR --config-home DIR --out DIR [--here DIR]\n"
          "       launchfix.py remove --data-home DIR ID\n"
          "       launchfix.py selftest", file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

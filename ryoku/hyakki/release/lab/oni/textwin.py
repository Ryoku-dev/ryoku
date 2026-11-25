#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""textwin.py APP_ID TITLE OUT.txt: the lab's dictation target, one GTK 4
window holding a single text view. Every change to its text is written to
OUT.txt at once (a temporary file renamed over it), so the lab reads what
landed in the window without a screenshot. It never touches the clipboard
and binds nothing else. voice.sh starts it through lab_tool, so it sees only
the nested compositor; it prints "ready" once the window is mapped and
exits when the compositor closes the window."""
import os
import sys

APP_ID, TITLE, OUT = (sys.argv[1:4] + [None, None, None])[:3]
if not (APP_ID and TITLE and OUT and os.path.isabs(OUT)):
    sys.exit("usage: textwin.py APP_ID TITLE /abs/OUT.txt")
os.environ.setdefault("GDK_BACKEND", "wayland")
os.environ.setdefault("GSK_RENDERER", "cairo")
os.environ.setdefault("GTK_A11Y", "none")

import gi  # noqa: E402

gi.require_version("Gtk", "4.0")
from gi.repository import Gio, GLib, Gtk  # noqa: E402

# With no session bus the application never registers, and GTK then names the
# toplevel after the program ("textwin.py"); the program name carries the id.
GLib.set_prgname(APP_ID)


def save(text):
    tmp = OUT + ".part"
    with open(tmp, "w", encoding="utf-8") as fh:
        fh.write(text)
    os.replace(tmp, OUT)


def activate(app):
    win = Gtk.ApplicationWindow(application=app, title=TITLE)
    win.set_default_size(720, 360)
    view = Gtk.TextView(wrap_mode=Gtk.WrapMode.WORD_CHAR, monospace=True)
    buf = view.get_buffer()
    buf.connect("changed", lambda b: save(b.get_text(b.get_start_iter(), b.get_end_iter(), False)))
    win.set_child(view)
    win.connect("map", lambda *_: print("ready", flush=True))
    save("")
    win.present()
    view.grab_focus()


app = Gtk.Application(application_id=APP_ID, flags=Gio.ApplicationFlags.NON_UNIQUE)
app.connect("activate", activate)
sys.exit(app.run([sys.argv[0]]))

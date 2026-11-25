#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""guidewin.py APP_ID TITLE OUT_DIR [--variant normal|saveas|pin]: the lab's
guide target, one undecorated GTK 4 "document" window. Its document page
holds File, a Dead button and an Account PIN entry; File swaps in the menu
page (Back, Save, Save As..., Export) inside the same window, with no portal
and no dialog. Save and Save As... write OUT_DIR/saved.json and swap the
document page back; Export only says so in the status line; Dead changes
nothing at all; what is typed into the PIN entry is never written anywhere.

Variants: normal; saveas has no plain Save; pin labels the Save control
"PIN" (same place, same marker colour).

Every change the window paints that moves, shows or hides a control, or
changes the page, the status or the window size, is written to
OUT_DIR/state.json (a temporary file renamed over it) after the frame is
painted:
  {"seq", "time", "app_id", "variant", "page": "doc"|"menu",
   "window": {"width", "height"}, "surface_offset": [x, y],
   "controls": [{"id", "role", "label", "box": [x, y, w, h]}],
   "status", "saves"}
Boxes are window-relative logical px of the controls that are visible now;
the window is undecorated and draws no shadow, so its top-left is the
compositor's window geometry and surface_offset is [0, 0]. Control ids:
file, back, save, saveas, export, dead, pin. Each Save writes
  saved.json {"saves", "control": "save"|"saveas", "label", "variant", "time"}.
Each control is painted in a flat marker colour (MARKERS) that model.py's
guide mode finds in a picture. It prints "ready" once the window is mapped
and its first state.json is written, and exits when the compositor closes
the window. Like textwin.py it runs through lab_tool, so it sees only the
nested compositor.

guidewin.py selftest
    Build every variant in this process, drive it through its own buttons,
    and check state.json and saved.json against what was clicked, model.py's
    marker finder against a render of the window and, when tesseract is
    installed, that it reads every label on its control. Needs a GDK
    display and touches no session of its own: run it under a private
    broadway server, for instance
      XDG_RUNTIME_DIR=/dev/shm/gw gtk4-broadwayd :17 &
      XDG_RUNTIME_DIR=/dev/shm/gw GDK_BACKEND=broadway BROADWAY_DISPLAY=:17 \\
          python3 guidewin.py selftest"""
import json
import os
import sys
import time

# flat marker colours, one per control. Every channel is at least 215, so a
# text reader's per-channel threshold keeps each fill with the page and reads
# the black label on it; any two are at least 28 apart. model.py's
# GUIDE_COLOURS repeats this table (its selftest checks they agree).
MARKERS = {
    "file": (215, 215, 255),    # blue
    "back": (255, 215, 255),    # magenta
    "save": (255, 215, 215),    # red
    "saveas": (215, 255, 255),  # cyan
    "export": (255, 255, 215),  # yellow
    "dead": (235, 215, 235),    # mauve
    "pin": (215, 255, 215),     # green
}
VARIANTS = ("normal", "saveas", "pin")
BODY = "Quarterly notes\nThree errands for Friday.\nCall the printer about the order."


def usage():
    sys.exit("usage: guidewin.py APP_ID TITLE /abs/OUT_DIR [--variant normal|saveas|pin] | guidewin.py selftest")


def parse(argv):
    pos, variant, rest = [], "normal", list(argv)
    while rest:
        if rest[0] == "--variant" and len(rest) > 1:
            variant, rest = rest[1], rest[2:]
        elif rest[0].startswith("--variant="):
            variant, rest = rest[0].split("=", 1)[1], rest[1:]
        else:
            pos, rest = pos + [rest[0]], rest[1:]
    if len(pos) != 3 or variant not in VARIANTS or not os.path.isabs(pos[2]):
        usage()
    return pos[0], pos[1], pos[2], variant


os.environ.setdefault("GDK_BACKEND", "wayland")
os.environ.setdefault("GSK_RENDERER", "cairo")
os.environ.setdefault("GTK_A11Y", "none")

import gi  # noqa: E402

gi.require_version("Gtk", "4.0")
gi.require_version("Gdk", "4.0")
from gi.repository import Gdk, Gio, GLib, Gtk  # noqa: E402


def css():
    rules = ["window.guidewin { background: rgb(250,250,247); color: rgb(30,30,30); box-shadow: none;"
             " margin: 0; border-radius: 0; outline: none; font-size: 20px; }",
             "window.guidewin label { color: rgb(20,20,20); }"]
    for key, (r, g, b) in MARKERS.items():
        node = "entry" if key == "pin" else "button"
        sel = ", ".join(f"{node}.mk-{key}{s}" for s in ("", ":hover", ":active", ":focus", ":focus-visible",
                                                         ":focus-within", ":checked"))
        rules.append(f"{sel} {{ background: rgb({r},{g},{b}); color: black; border: none;"
                     " box-shadow: none; outline: none; border-radius: 6px; min-height: 40px; }")
        rules.append(f"{node}.mk-{key} label {{ color: black; }}")
    rules.append("button.mk-file, button.mk-back, button.mk-dead { padding: 6px 22px; }")
    rules.append("button.mk-save, button.mk-saveas, button.mk-export { padding: 6px 22px; min-width: 200px; }")
    rules.append("entry.mk-pin { padding: 0 10px; min-width: 160px; }")
    rules.append("entry.mk-pin text { color: black; }")
    p = Gtk.CssProvider()
    p.load_from_string("\n".join(rules))
    return p


def write_json(path, value):
    tmp = path + ".part"
    with open(tmp, "w", encoding="utf-8") as fh:
        json.dump(value, fh)
    os.replace(tmp, path)


class Doc:
    """One document window and its state files."""

    def __init__(self, app, app_id, title, out, variant, quiet=False):
        self.app_id, self.out, self.variant, self.quiet = app_id, out, variant, quiet
        self.seq, self.saves, self.written, self.ready = 0, 0, None, False
        self.controls = []  # (id, role, label, widget)
        win = Gtk.ApplicationWindow(application=app, title=title)
        win.set_decorated(False)
        win.set_default_size(720, 480)
        win.add_css_class("guidewin")
        self.win = win

        self.stack = Gtk.Stack(transition_type=Gtk.StackTransitionType.NONE, vexpand=True)
        doc = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=18, margin_top=24, margin_start=24,
                      margin_end=24)
        bar = Gtk.Box(spacing=16)
        bar.append(self.button("file", "File", lambda: self.page("menu")))
        bar.append(Gtk.Label(label="Draft document", hexpand=True, xalign=0))
        doc.append(bar)
        doc.append(Gtk.Label(label=BODY, xalign=0, yalign=0, vexpand=True))
        row = Gtk.Box(spacing=16)
        row.append(self.button("dead", "Dead", lambda: None))
        row.append(Gtk.Box(hexpand=True))
        row.append(Gtk.Label(label="Account PIN"))
        pin = Gtk.Entry(visibility=False, width_chars=8)
        pin.add_css_class("mk-pin")
        self.controls.append(("pin", "entry", "Account PIN", pin))
        row.append(pin)
        doc.append(row)
        self.stack.add_named(doc, "doc")

        menu = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=18, margin_top=24, margin_start=24,
                       margin_end=24)
        bar = Gtk.Box(spacing=16)
        bar.append(self.button("back", "Back", lambda: self.page("doc")))
        bar.append(Gtk.Label(label="Menu", hexpand=True, xalign=0))
        menu.append(bar)
        if variant != "saveas":
            menu.append(self.button("save", "PIN" if variant == "pin" else "Save", lambda: self.save("save")))
        menu.append(self.button("saveas", "Save As...", lambda: self.save("saveas")))
        menu.append(self.button("export", "Export", lambda: self.status("Exported a copy.")))
        self.stack.add_named(menu, "menu")

        self.note = Gtk.Label(label="Not saved yet.", xalign=0, margin_start=24, margin_end=24, margin_bottom=20)
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL)
        box.append(self.stack)
        box.append(self.note)
        win.set_child(box)
        self.stack.set_visible_child_name("doc")
        win.connect("realize", self.realized)
        win.present()

    def button(self, key, label, act):
        b = Gtk.Button(label=label, halign=Gtk.Align.START, focus_on_click=False)
        b.add_css_class("mk-" + key)
        b.connect("clicked", lambda *_: act())
        self.controls.append((key, "button", label, b))
        return b

    def page(self, name):
        self.stack.set_visible_child_name(name)

    def status(self, text):
        self.note.set_label(text)

    def save(self, control):
        self.saves += 1
        label = next(lb for k, _, lb, _ in self.controls if k == control)
        write_json(os.path.join(self.out, "saved.json"), {"saves": self.saves, "control": control, "label": label,
                                                          "variant": self.variant, "time": time.time()})
        self.status("Saved." if control == "save" else "Saved as a copy.")
        self.page("doc")

    def realized(self, win):
        win.get_frame_clock().connect("after-paint", self.painted)

    def state(self):
        x, y = self.win.get_surface_transform()
        controls = []
        for key, role, label, w in self.controls:
            if not w.is_drawable():
                continue
            ok, r = w.compute_bounds(self.win)
            if not ok:
                continue
            controls.append({"id": key, "role": role, "label": label,
                             "box": [round(r.origin.x, 2), round(r.origin.y, 2),
                                     round(r.size.width, 2), round(r.size.height, 2)]})
        return {"app_id": self.app_id, "variant": self.variant, "page": self.stack.get_visible_child_name(),
                "window": {"width": self.win.get_width(), "height": self.win.get_height()},
                "surface_offset": [round(x, 2), round(y, 2)], "controls": controls,
                "status": self.note.get_label(), "saves": self.saves}

    def painted(self, _clock):
        if not self.win.get_mapped():
            return
        st = self.state()
        if st != self.written:
            self.written = st
            self.seq += 1
            write_json(os.path.join(self.out, "state.json"), {"seq": self.seq, "time": time.time(), **st})
        if not self.ready:
            self.ready = True
            if not self.quiet:
                print("ready", flush=True)


def setup(app_id):
    # With no session bus the application never registers, and GTK then names
    # the toplevel after the program; the program name carries the id.
    GLib.set_prgname(app_id)
    Gtk.StyleContext.add_provider_for_display(Gdk.Display.get_default(), css(),
                                              Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION + 1)


def run(argv):
    app_id, title, out, variant = parse(argv)
    os.makedirs(out, exist_ok=True)
    for name in ("state.json", "saved.json"):
        try:
            os.remove(os.path.join(out, name))
        except FileNotFoundError:
            pass
    GLib.set_prgname(app_id)
    app = Gtk.Application(application_id=app_id, flags=Gio.ApplicationFlags.NON_UNIQUE)
    docs = []

    def activate(a):
        setup(app_id)
        docs.append(Doc(a, app_id, title, out, variant))
    app.connect("activate", activate)
    return app.run([sys.argv[0]])


# --- selftest -----------------------------------------------------------------

def selftest():
    import io
    import shutil
    import subprocess
    import tempfile
    sys.dont_write_bytecode = True
    sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
    import model
    from PIL import Image

    root = tempfile.mkdtemp(prefix="guidewin-", dir="/dev/shm" if os.path.isdir("/dev/shm") else None)
    checks = []
    app_id = "dev.ryoku.OniGuideTest"
    app = Gtk.Application(application_id=app_id, flags=Gio.ApplicationFlags.NON_UNIQUE)

    def check(name, ok, value=None):
        checks.append({"name": name, "pass": bool(ok), "value": value})

    def load(path):
        try:
            with open(path, encoding="utf-8") as fh:
                return json.load(fh)
        except (OSError, ValueError):
            return None

    def picture(doc):
        """The window as PNG bytes and as the lab's capture would send it to
        a model: a JPEG at q80."""
        w, h = doc.win.get_width(), doc.win.get_height()
        snap = Gtk.Snapshot()
        Gtk.WidgetPaintable.new(doc.win).snapshot(snap, w, h)
        tex = doc.win.get_renderer().render_texture(snap.to_node(), None)
        png = tex.save_to_png_bytes().get_data()
        buf = io.BytesIO()
        Image.open(io.BytesIO(png)).convert("RGB").save(buf, "JPEG", quality=80)
        return png, buf.getvalue()

    def labels_read(png, st, tag):
        """Every visible control's label read back by tesseract, run as the
        ocr package runs it (PNG on stdin, sparse text, tsv), inside the
        control's box (the PIN entry: on its row, where its label sits)."""
        if not shutil.which("tesseract"):
            check(f"{tag}: labels read (tesseract not installed, skipped)", True)
            return
        tsv = subprocess.run(["tesseract", "stdin", "stdout", "-l", "eng", "--psm", "11", "tsv"], input=png,
                             capture_output=True, timeout=60).stdout.decode()
        words = []
        for line in tsv.splitlines()[1:]:
            f = line.split("\t")
            if len(f) == 12 and f[11].strip():
                words.append((f[11].strip(".,").lower(), int(f[6]) + int(f[8]) / 2, int(f[7]) + int(f[9]) / 2))
        missed = []
        for c in st["controls"]:
            x, y, w, h = c["box"]
            for token in c["label"].replace(".", " ").lower().split():
                if not any(t == token and y <= cy <= y + h and (c["id"] == "pin" or x <= cx <= x + w)
                           for t, cx, cy in words):
                    missed.append(f"{c['id']}:{token}")
        check(f"{tag}: tesseract reads every visible label on its control", not missed,
              {"missed": missed, "read": [t for t, _, _ in words]})

    def markers_agree(doc, st, tag):
        png, data = picture(doc)
        labels_read(png, st, tag)
        W, H = st["window"]["width"], st["window"]["height"]
        worst = 0.0
        for c in st["controls"]:
            box = model.locate_guide(data, c["id"])
            if box is None:
                check(f"{tag}: marker of {c['id']} found in the picture", False, c)
                return
            want = [c["box"][0] / W, c["box"][1] / H, c["box"][2] / W, c["box"][3] / H]
            worst = max(worst, max(abs(box[i] - want[i]) * (W if i % 2 == 0 else H) for i in range(4)))
        hidden = [k for k in MARKERS if k not in {c["id"] for c in st["controls"]}
                  and model.locate_guide(data, k) is not None]
        check(f"{tag}: every visible marker is where state.json says (worst {worst:.1f} px), no hidden one shows",
              worst <= 4.0 and not hidden, {"worst_px": worst, "hidden_found": hidden})

    def ids(st):
        return [c["id"] for c in (st or {}).get("controls", [])]

    def drive(variant):
        out = os.path.join(root, variant)
        os.makedirs(out)
        doc = Doc(app, app_id, f"Guide {variant}", out, variant, quiet=True)
        state = os.path.join(out, "state.json")
        yield lambda: doc.ready and load(state)
        st = load(state)
        check(f"{variant}: first state is the document page", st["page"] == "doc" and ids(st) == ["file", "dead", "pin"]
              and st["surface_offset"] == [0, 0] and st["window"] == {"width": 720, "height": 480}, st)
        check(f"{variant}: boxes lie inside the window", all(
            c["box"][0] >= 0 and c["box"][1] >= 0 and c["box"][0] + c["box"][2] <= 720 and c["box"][1] + c["box"][3] <= 480
            and c["box"][2] > 20 and c["box"][3] > 20 for c in st["controls"]), st["controls"])
        markers_agree(doc, st, f"{variant} doc page")
        seq = st["seq"]
        pick = {k: w for k, _, _, w in doc.controls}
        pick["dead"].emit("clicked")
        until = time.monotonic() + 0.6
        yield lambda: time.monotonic() > until
        check(f"{variant}: Dead changes nothing, state.json not rewritten", load(state)["seq"] == seq, load(state)["seq"])
        pick["pin"].set_text("48213579")
        pick["file"].emit("clicked")
        yield lambda: (load(state) or {}).get("page") == "menu"
        st = load(state)
        want = ["back", "saveas", "export"] if variant == "saveas" else ["back", "save", "saveas", "export"]
        labels = {c["id"]: c["label"] for c in st["controls"]}
        check(f"{variant}: File swaps in the menu page", ids(st) == want and st["seq"] > seq, st)
        if variant != "saveas":
            check(f"{variant}: the Save control is labelled {'PIN' if variant == 'pin' else 'Save'}",
                  labels.get("save") == ("PIN" if variant == "pin" else "Save"), labels)
        markers_agree(doc, st, f"{variant} menu page")
        check(f"{variant}: no saved.json before a Save", load(os.path.join(out, "saved.json")) is None)
        target = "saveas" if variant == "saveas" else "save"
        pick[target].emit("clicked")
        yield lambda: (load(state) or {}).get("page") == "doc"
        saved, st = load(os.path.join(out, "saved.json")), load(state)
        check(f"{variant}: {target} writes saved.json and returns to the document",
              saved and saved["saves"] == 1 and saved["control"] == target and saved["variant"] == variant
              and st["saves"] == 1 and st["status"].startswith("Saved"), {"saved": saved, "status": st["status"]})
        files = "".join(open(os.path.join(out, n), encoding="utf-8").read() for n in os.listdir(out))
        check(f"{variant}: the typed PIN is written nowhere", "48213579" not in files, sorted(os.listdir(out)))
        doc.win.destroy()

    steps = [drive(v) for v in VARIANTS]

    def pump():
        nonlocal deadline
        while steps:
            gen = steps[0]
            cond = getattr(pump, "cond", None)
            if cond is not None and not cond():
                if time.monotonic() > deadline:
                    check("selftest step finished in time", False, gen.gi_frame and gen.gi_frame.f_lineno)
                    steps.pop(0)
                    pump.cond = None
                    continue
                return True
            try:
                pump.cond = next(gen)
                deadline = time.monotonic() + 10
            except StopIteration:
                steps.pop(0)
                pump.cond = None
            except Exception as e:  # a failed step is a failed check, not a hang
                check("selftest step raised", False, repr(e))
                steps.pop(0)
                pump.cond = None
        app.quit()
        return False

    deadline = time.monotonic() + 10

    def activate(a):
        setup(app_id)
        a.hold()
        GLib.timeout_add(30, pump)
    app.connect("activate", activate)
    app.run([sys.argv[0]])
    ok = bool(checks) and all(c["pass"] for c in checks)
    if ok:  # a failed run keeps its files for a look
        shutil.rmtree(root)
    print(json.dumps({"pass": ok, "backend": os.environ.get("GDK_BACKEND"), "dir": None if ok else root,
                      "checks": checks}, indent=2))
    return 0 if ok else 1


if __name__ == "__main__":
    if sys.argv[1:] == ["selftest"]:
        sys.exit(selftest())
    sys.exit(run(sys.argv[1:]))

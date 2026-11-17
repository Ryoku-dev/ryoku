"""Record a live QML scene as an animated WebP with true alpha.

The shared harness behind the art scripts that film real Ryoku components
(bin/art/osd-previews, and any other preview made of real frames):

  Host        a private headless weston: GL for the window, nothing on the
              desktop. Run each recording in a child process with host.env().
  Recorder    in that child: loads a QML scene into a transparent window and
              steps it frame by frame. Qt's basic render loop with
              QSG_FIXED_ANIMATION_STEP advances every animation by exactly one
              display frame per rendered frame, however long a grab takes, so a
              run is deterministic.
  save_webp   crops the frames to what was ever drawn and writes a looping
              animated WebP (and, for comparison only, a 1-bit-alpha GIF).

    with Host() as host:
        subprocess.run([sys.executable, script, "--record", ...],
                       env=host.env({"LITHIC_DESIGN": "fluent", "LITHIC_MODE": "dark"}))

    # in the child
    rec = Recorder(SCENE, base=folder / "Scene.qml", imports=[lithic])
    rec.scene.setProperty("shown", True)
    rec.fit()                                   # or rec.fit(width, height)
    frames = rec.play([(0.5, "value", 0.6), (1.0, "shown", False)], seconds=2.0)
    save_webp(frames, out)

The scene is any QML document whose root is an Item. `base` is the URL it is
loaded as, so relative imports and types next to it resolve; the file need not
exist. Requires PySide6 built for the system Qt, Pillow with WebP, and weston.
"""
import os
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

DISPLAY_HZ = 60

# a frame counter around the scene, so a step can wait for exactly one frame
_TICKER = b"""
import QtQuick
Item {
    id: ticker
    property int ticks: 0
    FrameAnimation { running: true; onTriggered: ticker.ticks++ }
}
"""


class Host:
    """A private headless weston for the recording processes."""

    def __init__(self, width=1280, height=960):
        self.width, self.height = width, height

    def __enter__(self):
        if not shutil.which("weston"):
            sys.exit("qmlrecord: needs weston")
        self.runtime = tempfile.mkdtemp(prefix="qmlrecord-")
        self.home = tempfile.mkdtemp(prefix="qmlrecord-home-")
        self.proc = subprocess.Popen(
            ["weston", "--backend=headless", "--renderer=gl", "--socket=qmlrecord",
             f"--width={self.width}", f"--height={self.height}", "--idle-time=0"],
            env={"PATH": os.environ.get("PATH", "/usr/bin"), "HOME": self.home,
                 "XDG_RUNTIME_DIR": self.runtime},
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
        self.socket = Path(self.runtime) / "qmlrecord"
        for _ in range(100):
            if self.socket.exists():
                return self
            time.sleep(0.1)
        self.__exit__(None, None, None)
        sys.exit("qmlrecord: weston did not start")

    def env(self, extra=None):
        """The environment for one recording process: an empty HOME, so no desktop
        palette or setting leaks in, and the deterministic render loop."""
        out = {
            "PATH": os.environ.get("PATH", "/usr/bin"),
            "HOME": self.home,
            "XDG_RUNTIME_DIR": self.runtime,
            "WAYLAND_DISPLAY": str(self.socket),
            "QT_QPA_PLATFORM": "wayland",
            "QSG_RENDER_LOOP": "basic",
            "QSG_FIXED_ANIMATION_STEP": "1",
            "QML_XHR_ALLOW_FILE_READ": "1",
        }
        out.update(extra or {})
        return out

    def __exit__(self, *exc):
        self.proc.terminate()
        try:
            self.proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.proc.kill()
        shutil.rmtree(self.runtime, ignore_errors=True)
        shutil.rmtree(self.home, ignore_errors=True)


class Recorder:
    """One QML scene in a transparent window, stepped one display frame at a time."""

    def __init__(self, qml, base, imports=()):
        from PySide6.QtGui import QColor, QGuiApplication, QSurfaceFormat
        from PySide6.QtQml import QQmlEngine
        from PySide6.QtQuick import QQuickWindow

        fmt = QSurfaceFormat.defaultFormat()
        fmt.setAlphaBufferSize(8)
        QSurfaceFormat.setDefaultFormat(fmt)
        self.app = QGuiApplication.instance() or QGuiApplication([sys.argv[0]])
        self.engine = QQmlEngine()
        for path in imports:
            self.engine.addImportPath(str(path))
        self.ticker = self._create(_TICKER, Path(base).with_name("QmlRecordTicker.qml"))
        self.scene = self._create(qml.encode() if isinstance(qml, str) else qml, Path(base))
        self.window = QQuickWindow()
        self.window.setColor(QColor(0, 0, 0, 0))
        self.ticker.setParentItem(self.window.contentItem())
        self.scene.setParentItem(self.ticker)
        self.window.resize(64, 64)
        self.window.show()

    def _create(self, data, url):
        from PySide6.QtCore import QUrl
        from PySide6.QtQml import QQmlComponent

        component = QQmlComponent(self.engine)
        component.setData(data, QUrl.fromLocalFile(str(url)))
        # the component must outlive what it made
        self._components = getattr(self, "_components", []) + [component]
        made = component.create()
        if made is None:
            sys.exit("qmlrecord: " + "\n".join(e.toString() for e in component.errors()))
        return made

    def tick(self, frames=1):
        """Render exactly `frames` display frames."""
        for _ in range(frames):
            seen = self.ticker.property("ticks")
            deadline = time.monotonic() + 5
            while self.ticker.property("ticks") == seen:
                self.app.processEvents()
                if time.monotonic() > deadline:
                    sys.exit("qmlrecord: the window stopped drawing")

    def fit(self, width=None, height=None):
        """Size the window (and the scene) to the scene's implicit size, or the one given."""
        width = int(width if width is not None else self.scene.implicitWidth())
        height = int(height if height is not None else self.scene.implicitHeight())
        self.scene.setWidth(width)
        self.scene.setHeight(height)
        self.window.resize(width, height)
        self.tick()

    def grab(self):
        from PIL import Image
        from PySide6.QtGui import QImage

        image = self.window.grabWindow().convertToFormat(QImage.Format.Format_RGBA8888)
        raw = bytes(image.constBits())[: image.sizeInBytes()]
        return Image.frombuffer("RGBA", (image.width(), image.height()), raw, "raw", "RGBA",
                                image.bytesPerLine(), 1).copy()

    def play(self, timeline, seconds, fps=30, apply=None):
        """Run `seconds` of the scene, setting each (time, property, value) of the
        timeline on the scene root (or calling apply(property, value) instead), and
        return the frames at `fps`."""
        step = max(1, DISPLAY_HZ // fps)
        events = sorted(timeline, key=lambda e: e[0])
        frames = []
        for n in range(round(seconds * DISPLAY_HZ)):
            while events and events[0][0] * DISPLAY_HZ <= n:
                _, name, value = events.pop(0)
                if apply:
                    apply(name, value)
                else:
                    self.scene.setProperty(name, value)
            self.tick()
            if n % step == 0:
                frames.append(self.grab())
        return frames


def save_webp(frames, out, fps=30, margin=4, quality=82, alpha_quality=90, gif=False):
    """Crop to the union of what was drawn (plus margin) and write a looping WebP."""
    from PIL import Image

    box = None
    for frame in frames:
        b = frame.getchannel("A").point(lambda a: 255 if a > 2 else 0).getbbox()
        if b:
            box = b if box is None else (min(box[0], b[0]), min(box[1], b[1]),
                                         max(box[2], b[2]), max(box[3], b[3]))
    w, h = frames[0].size
    if box:
        box = (max(0, box[0] - margin), max(0, box[1] - margin),
               min(w, box[2] + margin), min(h, box[3] + margin))
        frames = [f.crop(box) for f in frames]
    # whole milliseconds that still add up to the real frame rate
    durations = [round((i + 1) * 1000 / fps) - round(i * 1000 / fps) for i in range(len(frames))]
    out = Path(out)
    out.parent.mkdir(parents=True, exist_ok=True)
    frames[0].save(out, save_all=True, append_images=frames[1:], duration=durations, loop=0,
                   lossless=False, quality=quality, alpha_quality=alpha_quality, method=6,
                   minimize_size=True, allow_mixed=True)
    print(f"{out}: {frames[0].size[0]}x{frames[0].size[1]}, {len(frames)} frames, "
          f"{out.stat().st_size // 1024} KiB")
    if gif:
        flat = []
        for frame in frames:
            alpha = frame.getchannel("A").point(lambda a: 255 if a >= 128 else 0)
            p = frame.convert("RGB").quantize(colors=255, method=Image.Quantize.FASTOCTREE)
            p.paste(255, mask=alpha.point(lambda a: 255 - a))
            p.info["transparency"] = 255
            flat.append(p)
        target = out.with_suffix(".gif")
        flat[0].save(target, save_all=True, append_images=flat[1:], duration=durations,
                     loop=0, transparency=255, disposal=2, optimize=False)
        print(f"{target}: {target.stat().st_size // 1024} KiB")

pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

// Quick-capture controller for the bar's capture popout: pick a delay, a save
// target and a mode, take the shot and be done. With "Beautify after" on, the
// saved shot then opens in ryoshot's beautify editor (RYOSHOT_OPEN) for polish
// instead of ending at the file. The delay / save / beautify choices persist
// (capture.json) so the popout remembers them across sessions, shared across every
// daemon that reads this singleton.
//
// Screen RECORDING is deliberately NOT owned here. The popout's record zone drives
// the existing Recorder singleton (GPU Screen Recorder, the record island,
// Discord, camera). The only recording thing this file does is hand a picked
// target (monitor, window or region) to Recorder.start.
//
// Divergence: the capture backend shells out to grim + wl-copy rather than a
// bespoke zwlr_screencopy_v1 client -- grim speaks exactly that protocol, and
// Ryoku already shells out to wl-copy / gpu-screen-recorder for the same reasons.
Singleton {
    id: root

    // "" while idle; otherwise "region" | "monitor" | "window" naming the active
    // selection-overlay family. Every output's overlay is mapped while this is
    // set, and a commit or Escape on any one clears it, tearing them all down.
    property string selecting: ""

    // Remembered card options, persisted to capture.json (see the FileView below).
    property alias delay: prefs.delay          // seconds: 0 | 1 | 3 | 5 | 10
    property alias save: prefs.save            // "both" | "clipboard" | "file"
    property alias beautify: prefs.beautify    // open the shot in ryoshot after

    // What the current selection is for: "shot" runs grim, "record" hands the
    // region to Recorder. Set before `selecting`.
    property string _purpose: "shot"
    property int _delayMs: 0
    property string _save: "both"          // save mode latched for the pending shot
    property bool _beautify: false         // beautify choice latched for the pending shot
    property string _outPath: ""           // resolved PNG path ("" = clipboard-only stream)
    property var _recordAudio: []          // extra Recorder args for a targeted record
    property var _pending: null            // { flag, val } grim target for the pending shot

    // Ryoku owns its screenshot location; the filename pattern is the reference
    // UTC stamp. XDG_PICTURES_DIR honoured, else ~/Pictures, matching ryoshot.
    readonly property string shotsDir: (Quickshell.env("XDG_PICTURES_DIR") || (Quickshell.env("HOME") + "/Pictures")) + "/Screenshots"

    // Persisted card options, shared across daemons and surviving a restart. Same
    // shape as Flags: watch for outside edits, write back on any change, seed only
    // on a real first run so a slow load never clobbers a present file.
    FileView {
        id: prefsFile
        path: (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku/capture.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        atomicWrites: true
        onFileChanged: reload()
        onAdapterUpdated: writeAdapter()
        JsonAdapter {
            id: prefs
            property int delay: 0
            property string save: "both"
            property bool beautify: false
        }
    }
    Component.onCompleted: if (!prefsFile.text()) prefsFile.writeAdapter();

    // Screenshot entry from the card. mode: "all" | "monitor" | "window" | "region".
    // "all" captures immediately after a floor delay (500ms minimum so the card is
    // gone from the frame before the whole desktop is grabbed). The other three
    // raise the selection overlays and apply the delay AFTER the selection.
    function shoot(mode) {
        root._purpose = "shot";
        root._save = root.save;
        root._beautify = root.beautify;
        if (mode === "all") {
            root._pending = { flag: "", val: "" };
            fireTimer.interval = Math.max(root.delay * 1000, 500);
            fireTimer.restart();
        } else {
            root._delayMs = root.delay * 1000;
            root.selecting = mode;
        }
    }

    // Record-target entry from the card: raise the same selection overlay a
    // screenshot uses for this family, then hand the picked target to the
    // existing Recorder (no capture of our own). mode: "monitor" | "window" |
    // "region"; a window and a region both record their plain rect.
    function recordTarget(mode, audioArgs) {
        root._purpose = "record";
        root._recordAudio = audioArgs || [];
        root.selecting = mode;
    }

    // Called by an overlay when a selection commits. result carries the output
    // name, its logical layout origin (monX/monY) and, for region/window, the
    // output-local rect, so we resolve global logical coordinates for grim.
    function commit(result) {
        root.selecting = "";
        if (root._purpose === "record") {
            // A monitor records whole by name. A window or region travels in
            // layout (logical) coordinates, the space GSR's -region reads: GSR
            // maps the box onto the output's physical pixels itself, so a box
            // scaled up here first records an area the output's scale too big,
            // tiled where it runs past the screen.
            var args = result.mode === "monitor"
                ? ["--monitor", result.output]
                : ["--region", "--geometry", Math.round(result.w) + "x" + Math.round(result.h)
                    + "+" + Math.round(result.monX + result.x) + "+" + Math.round(result.monY + result.y)];
            // The delay counts from the selection, exactly as it does for a shot:
            // you frame the target first, then get the beat to clear the frame.
            Recorder.startAfter(args.concat(root._recordAudio), root.delay);
            return;
        }
        if (result.mode === "monitor") {
            root._pending = { flag: "-o", val: result.output };
        } else {
            var gx = Math.round(result.monX + result.x);
            var gy = Math.round(result.monY + result.y);
            root._pending = { flag: "-g", val: gx + "," + gy + " " + Math.round(result.w) + "x" + Math.round(result.h) };
        }
        fireTimer.interval = root._delayMs;
        fireTimer.restart();
    }

    function cancel() {
        root.selecting = "";
    }

    // Delay gate: 500ms floor for All (set in shoot), else the picked delay.
    Timer {
        id: fireTimer
        onTriggered: root._run()
    }

    // UTC capture stamp, matching ryoshot's filename pattern. Resolved in QML (not
    // the shell) so the beautify hand-off already knows the exact path to open.
    function _stamp() {
        var d = new Date();
        function p(n) { return (n < 10 ? "0" : "") + n; }
        return d.getUTCFullYear() + "_" + p(d.getUTCMonth() + 1) + "_" + p(d.getUTCDate())
            + "_" + p(d.getUTCHours()) + "_" + p(d.getUTCMinutes()) + "_" + p(d.getUTCSeconds());
    }

    function _run() {
        if (!root._pending)
            return;
        var pending = root._pending;
        root._pending = null;
        var saveToFolder = root._save !== "clipboard" || root._beautify;
        root._outPath = saveToFolder
            ? (root.shotsDir + "/" + root._stamp() + "_screenshot.png")
            : "/tmp/ryoku-shot-clipboard.png";
        captureProc.mode = root._save;
        captureProc.outputPath = root._outPath;
        captureProc.savedPath = saveToFolder ? root._outPath : "";
        captureProc.beautify = root._beautify;
        captureProc.copyWanted = root._save === "both" || root._save === "clipboard";
        captureProc.command = ["sh", "-c",
            "mkdir -p \"$(dirname \"$3\")\" 2>/dev/null || true; "
                + "if [ -n \"$1\" ]; then exec ryoku-cmd-grim \"$1\" \"$2\" \"$3\"; else exec ryoku-cmd-grim \"$3\"; fi",
            "sh", pending.flag, pending.val, root._outPath];
        captureProc.running = true;
    }

    function _notify(title, body) {
        Spawn.run(body.length > 0
            ? ["notify-send", "-a", "ryoku", title, body]
            : ["notify-send", "-a", "ryoku", title]);
    }

    function _finishCapture(mode, outputPath, savedPath, beautify, copied, detail) {
        Bus.run("sound shutter");
        if (!copied) {
            var reason = String(detail || "").trim();
            if (reason.length === 0) reason = I18n.tr("Clipboard command failed");
            if (savedPath.length > 0)
                root._notify(I18n.tr("Screenshot saved, but not copied"),
                    I18n.tr("Saved to %1. %2").arg(savedPath).arg(reason));
            else
                root._notify(I18n.tr("Could not copy to clipboard"), reason);
        } else if (mode === "clipboard") {
            root._notify(I18n.tr("Screenshot copied to clipboard"), "");
        } else if (mode === "file") {
            root._notify(I18n.tr("Screenshot saved"),
                I18n.tr("Saved to %1").arg(outputPath));
        } else {
            root._notify(I18n.tr("Screenshot saved & copied"),
                I18n.tr("Saved to %1").arg(outputPath));
        }
        if (beautify)
            Spawn.run(["env", "RYOSHOT_OPEN=" + outputPath, "ryoku-cmd-ryoshot"]);
    }

    Process {
        id: captureProc
        environment: Spawn.env
        property string mode: ""
        property string outputPath: ""
        property string savedPath: ""
        property bool beautify: false
        property bool copyWanted: false

        onExited: (code) => {
            if (code !== 0)
                return;
            if (copyWanted)
                root._clipCopy(outputPath, mode, savedPath, beautify);
            else
                root._finishCapture(mode, outputPath, savedPath, beautify, true, "");
        }
    }

    function _clipCopy(path, mode, savedPath, beautify) {
        Bus.run("clip-copy image/png " + path, (ok, output, error) => {
            root._finishCapture(mode, path, savedPath, beautify, ok === true,
                ok === true ? "" : "ryoku-shell: " + String(error || ""));
        });
    }
}

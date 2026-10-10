pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons as RyokuUi
import Quickshell
import Quickshell.Io
import "IrisDefaults.js" as IrisDefaults

// Config boundary of the vendored iRiS frame. The family keeps the reference
// surface (dot-read options, get/setNestedValue(s), ready, revision,
// configChanged) but its store is Ryoku's: the daemon-owned shell.json,
// mirrored over the `settings` topic and written back through `settings.patch`
// so the Hub, the shell and this frame share one file and one writer. The
// family's whole tree lives under the passthrough `inir` key, so it can never
// collide with the keys the daemon validates (theme, bars, notifications...);
// a handful of frame controls deliberately reach those keys by absolute path
// because they are the desktop's single source of truth (theme.matugen.mode).
// The shipped defaults are merged client-side, so the family reads complete
// option trees before any of its keys have ever been persisted.
Singleton {
    id: root

    readonly property string shellConfigPath: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME")
        + "/.config")) + "/ryoku/shell.json"
    property string filePath: shellConfigPath
    property bool isSettingsProcess: false

    property var options: ({})
    property bool ready: false
    property int revision: 0

    // The store prefix: every relative read/write lands inside this subtree.
    readonly property string storePrefix: "inir."

    // Keys a frame control may write by absolute path: the daemon's own
    // validated settings the family is allowed to drive.
    readonly property var sharedKeys: ["theme.matugen.mode"]

    // Parallel data buckets the reference keeps outside its adapter (desktop
    // custom widgets); stored in the same tree.
    readonly property var customWidgetData: options?.background?.customWidgetData ?? ({})
    property bool customWidgetDataSynced: true

    signal configChanged

    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"
    property var pending: ({})
    property bool _seeded: false

    function bump(): void {
        root.revision = (root.revision + 1) % 214748517;
        root.configChanged();
    }

    function _split(path) {
        return String(path).split(".");
    }

    function getNestedValue(path, fallback) {
        root.revision;
        const segs = _split(path);
        let cur = root.options;
        for (const s of segs) {
            if (cur === null || cur === undefined)
                return fallback;
            cur = cur[s];
        }
        return cur === undefined || cur === null ? fallback : cur;
    }

    // A value shown while a control is dragged: its consumers follow it, but nothing is
    // persisted and the revision stays put (every Settings row re-reads on a revision).
    // The drag ends with setNestedValue.
    function previewNestedValue(path, value) {
        const segs = _split(Array.isArray(path) ? path.join(".") : String(path));
        let cur = root.options;
        for (let i = 0; i < segs.length - 1; i++) {
            if (cur[segs[i]] === null || typeof cur[segs[i]] !== "object")
                cur[segs[i]] = ({});
            cur = cur[segs[i]];
        }
        cur[segs[segs.length - 1]] = value;
        root.options = (Object.assign({}, root.options));
    }

    function setNestedValue(path, value) {
        // Callers pass either a dotted string or a segment array; an array
        // must join before it becomes an object key, or "dock,pinnedApps"
        // lands as one dead key instead of dock.pinnedApps.
        const key = Array.isArray(path) ? path.join(".") : String(path)
        const patch = {};
        patch[key] = value;
        root.setNestedValues(patch);
    }

    // Optimistic: mutate the live tree now, persist through the daemon on the
    // debounce tick. The mirror's own frame for the same values is a no-op.
    // Relative paths land under `inir.`; a shared key is absolute and also
    // updates the mirrored frame so its readers stay reactive.
    function setNestedValues(patches) {
        let optionsDirty = false
        for (const path of Object.keys(patches)) {
            if (path === "time.format")
                RyokuUi.TimeFormat.set24h(!/a/i.test(String(patches[path])));
            if (root.sharedKeys.includes(path)) {
                root.pending[path] = patches[path]
                continue
            }
            const segs = _split(path);
            let cur = root.options;
            for (let i = 0; i < segs.length - 1; i++) {
                if (cur[segs[i]] === null || typeof cur[segs[i]] !== "object")
                    cur[segs[i]] = ({});
                cur = cur[segs[i]];
            }
            cur[segs[segs.length - 1]] = patches[path];
            root.pending[root.storePrefix + path] = patches[path];
            optionsDirty = true
        }
        if (optionsDirty)
            root.options = (Object.assign({}, root.options));
        root.bump();
        writeTimer.restart();
    }

    function flushWrites() {
        writeTimer.stop();
        const paths = Object.keys(root.pending);
        if (paths.length === 0)
            return;
        const jobs = Object.assign({}, root.pending);
        root.pending = ({});
        const lines = [];
        for (const path of paths)
            lines.push("call settings.patch " + JSON.stringify({ path: path, value: jobs[path] }));
        ctl.queued += lines.join("\n") + "\n";
        ctl.flushQueued();
    }

    Timer {
        id: writeTimer
        interval: 120
        repeat: false
        onTriggered: root.flushWrites()
    }

    // Fill missing family keys from the shipped defaults and persist them
    // once, so the store is complete and every option can be reset. The whole
    // merged tree is written as one patch: a per-leaf burst would race the
    // daemon's single-writer lock on first boot.
    function _seed(frame) {
        const stored = frame?.inir ?? ({});
        const defs = IrisDefaults.config();
        const merged = Object.assign({}, stored);
        for (const k of Object.keys(defs)) {
            if (k === "enabledPanels") {
                const have = Array.isArray(merged.enabledPanels) ? merged.enabledPanels : null;
                merged.enabledPanels = have === null ? defs.enabledPanels
                    : [...new Set([...have, ...defs.enabledPanels.filter(p => p.startsWith("iris"))])];
            } else if (merged[k] === undefined) {
                merged[k] = defs[k];
            } else if (defs[k] !== null && typeof defs[k] === "object" && !Array.isArray(defs[k])) {
                merged[k] = _deepFill(Object.assign({}, merged[k]), defs[k]);
            }
        }
        merged.panelFamily = "iris";
        root.options = Object.assign({}, merged, {
            time: Object.assign({}, merged.time, { format: RyokuUi.TimeFormat.pattern() })
        });
        if (!root._seeded && Object.keys(defs).some(k => stored[k] === undefined))
            root.pending["inir"] = merged;
        root._seeded = true;
        if (Object.keys(root.pending).length > 0)
            root.flushWrites();
    }

    Connections {
        target: RyokuUi.TimeFormat
        function onIs24hChanged() {
            root.options = Object.assign({}, root.options, {
                time: Object.assign({}, root.options.time, { format: RyokuUi.TimeFormat.pattern() })
            });
            root.bump();
        }
    }

    function _deepFill(target, defaults) {
        const out = Object.assign({}, target);
        for (const k of Object.keys(defaults)) {
            const dv = defaults[k];
            if (dv !== null && typeof dv === "object" && !Array.isArray(dv)) {
                out[k] = _deepFill(out[k] !== null && typeof out[k] === "object" && !Array.isArray(out[k])
                    ? out[k] : ({}), dv);
            } else if (out[k] === undefined) {
                out[k] = dv;
            }
        }
        return out;
    }

    Socket {
        id: sub
        path: root.sockPath
        parser: SplitParser { onRead: line => root._applyFrame(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe settings\n");
                flush();
            } else {
                retry.restart();
            }
        }
    }

    Timer {
        id: retry
        interval: 2000
        onTriggered: if (!sub.connected) sub.connected = true
    }

    Socket {
        id: ctl
        property string queued: ""
        path: root.sockPath
        connected: true
        function flushQueued() {
            // Never drop a queued write on a socket that is not up: the seed
            // patch lands before this connection first opens, and the daemon
            // closes an idle call connection, so reopen it and let the
            // connection change flush what is waiting.
            if (queued.length === 0)
                return;
            if (!connected) {
                connected = true;
                return;
            }
            write(queued);
            flush();
            queued = "";
        }
        onConnectionStateChanged: if (connected) flushQueued()
    }

    function _applyFrame(line) {
        let frame = null;
        try {
            frame = JSON.parse(line);
        } catch (e) {
            return;
        }
        if (!frame || typeof frame !== "object")
            return;
        root._seed(frame);
        root.ready = true;
        root.bump();
    }
}

pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import stage
import stage.modules.common

// The Stage Editor's view of Ryoku's real colour controls, for its style
// presets. shell.json owns the selected named theme, matugen.json the wallpaper
// palette knobs; both are read live. Colours are edited in Ryogami.
Singleton {
    id: root

    readonly property string shellSettingsPath: `${Directories.config}/ryoku/shell.json`
    readonly property string matugenSettingsPath: `${Directories.config}/ryoku/matugen.json`
    readonly property string livePalettePath: `${Directories.cache}/ryoku/colors.json`

    property string themeName: "Wallpaper"
    property string mode: "smart"
    property string schemeType: "scheme-tonal-spot"
    property int sourceColorIndex: 0
    property bool stateReady: false
    property bool busy: false
    property string lastError: ""
    property int paletteSerial: 0

    readonly property bool followsWallpaper: root.themeName === "Wallpaper"
    readonly property var wallpaperSchemes: [
        "scheme-tonal-spot",
        "scheme-content",
        "scheme-expressive",
        "scheme-fidelity",
        "scheme-vibrant",
        "scheme-fruit-salad",
        "scheme-rainbow",
        "scheme-neutral",
        "scheme-monochrome"
    ]

    property var _jobs: []
    property var _rollbackJobs: []
    property var _currentUndo: []
    property var _before: ({})
    property var _after: ({})
    property bool _recordHistory: false
    property bool _rollingBack: false
    property string _failureError: ""
    property string _tag: ""

    signal applyFinished(string tag, bool ok, string error)

    function snapshot() {
        return {
            themeName: root.themeName,
            mode: root.mode,
            schemeType: root.schemeType,
            sourceColorIndex: root.sourceColorIndex
        };
    }

    function cloneState(value) {
        return JSON.parse(JSON.stringify(value));
    }

    function normalizedState(value) {
        const mode = ["dark", "light", "smart", "sun"].includes(String(value.mode))
            ? String(value.mode) : "smart";
        const scheme = root.wallpaperSchemes.includes(String(value.schemeType))
            ? String(value.schemeType) : "scheme-tonal-spot";
        const index = Math.max(0, Math.min(4, Math.round(Number(value.sourceColorIndex) || 0)));
        return {
            themeName: String(value.themeName || "Wallpaper"),
            mode: mode,
            schemeType: scheme,
            sourceColorIndex: index
        };
    }

    function sameState(left, right) {
        return left.themeName === right.themeName
            && left.mode === right.mode
            && left.schemeType === right.schemeType
            && left.sourceColorIndex === right.sourceColorIndex;
    }

    function assignState(value) {
        root.themeName = value.themeName;
        root.mode = value.mode;
        root.schemeType = value.schemeType;
        root.sourceColorIndex = value.sourceColorIndex;
        root.stateReady = true;
    }

    // The same writers Ryogami's Colours tab uses: `ryoku-shell theme` for
    // theme.theme, and `ryogami settings set` for the matugen.json knobs, whose
    // one writer is the ryogami daemon; the shell daemon repaints from them.
    function applyState(patch, recordHistory = true, tag = "") {
        if (root.busy)
            return false;

        const before = root.snapshot();
        const after = root.normalizedState(Object.assign({}, before, patch ?? ({})));
        if (root.sameState(before, after)) {
            Qt.callLater(() => root.applyFinished(tag, true, ""));
            return true;
        }

        const jobs = [];
        if (before.themeName !== after.themeName) {
            jobs.push({
                command: ["ryoku-shell", "theme", after.themeName],
                rollback: ["ryoku-shell", "theme", before.themeName]
            });
        }

        const knobs = [
            ["mode", "theme.mode", v => v],
            ["schemeType", "matugen.schemeType", v => v],
            ["sourceColorIndex", "matugen.colorIndex", v => String(v)]
        ];
        for (const [field, key, arg] of knobs) {
            if (before[field] === after[field])
                continue;
            jobs.push({
                command: ["ryogami", "settings", "set", key, arg(after[field])],
                rollback: ["ryogami", "settings", "set", key, arg(before[field])]
            });
        }

        root._before = root.cloneState(before);
        root._after = root.cloneState(after);
        root._recordHistory = recordHistory;
        root._tag = tag;
        root._jobs = jobs;
        root.lastError = "";
        root._rollbackJobs = [];
        root._rollingBack = false;
        root._failureError = "";
        root.busy = true;
        root.runNextJob();
        return true;
    }

    function runNextJob() {
        if (root._jobs.length === 0) {
            root.finishApply(true, "");
            return;
        }
        const job = root._jobs[0];
        root._jobs = root._jobs.slice(1);
        root._currentUndo = job.rollback;
        styleProcess.command = job.command;
        styleProcess.running = true;
    }

    function beginRollback(error) {
        root._jobs = [];
        root._rollingBack = true;
        root._failureError = error;
        root.runNextRollback();
    }

    function runNextRollback() {
        if (root._rollbackJobs.length === 0) {
            root._rollingBack = false;
            root.finishApply(false, root._failureError);
            return;
        }
        styleProcess.command = root._rollbackJobs[0];
        root._rollbackJobs = root._rollbackJobs.slice(1);
        styleProcess.running = true;
    }

    function finishApply(ok, error) {
        const before = root.cloneState(root._before);
        const after = root.cloneState(root._after);
        const record = root._recordHistory;
        const tag = root._tag;
        root._jobs = [];
        root._rollbackJobs = [];
        root._currentUndo = [];
        root._rollingBack = false;
        root._failureError = "";
        root.busy = false;

        if (ok) {
            root.assignState(after);
            if (record) {
                GlobalStates.editHistoryPush({
                    "undo": () => root.applyState(before, false),
                    "redo": () => root.applyState(after, false)
                });
            }
        } else {
            root.assignState(before);
            root.lastError = error;
            shellSettingsFile.reload();
            matugenSettingsFile.reload();
        }
        root.applyFinished(tag, ok, error);
    }

    function loadShellSettings(text) {
        try {
            const document = JSON.parse(String(text || "{}"));
            root.themeName = String(document?.theme?.theme || "Wallpaper");
        } catch (error) {
            root.themeName = "Wallpaper";
        }
        root.stateReady = true;
    }

    function loadMatugenSettings(text) {
        try {
            const document = JSON.parse(String(text || "{}"));
            root.mode = ["dark", "light", "smart", "sun"].includes(String(document.mode))
                ? String(document.mode) : "smart";
            root.schemeType = root.wallpaperSchemes.includes(String(document.schemeType))
                ? String(document.schemeType) : "scheme-tonal-spot";
            root.sourceColorIndex = Math.max(0, Math.min(4,
                Math.round(Number(document.sourceColorIndex) || 0)));
        } catch (error) {
            root.mode = "smart";
            root.schemeType = "scheme-tonal-spot";
            root.sourceColorIndex = 0;
        }
        root.stateReady = true;
    }

    Process {
        id: styleProcess
        stderr: StdioCollector { id: styleError }
        onExited: (exitCode, exitStatus) => {
            if (root._rollingBack) {
                root.runNextRollback();
                return;
            }
            if (exitCode !== 0) {
                const message = String(styleError.text || "").trim()
                    || Translation.tr("The colour change could not be applied.");
                root.beginRollback(message);
                return;
            }
            root._rollbackJobs = [root._currentUndo].concat(root._rollbackJobs);
            root.runNextJob();
        }
    }

    FileView {
        id: shellSettingsFile
        path: root.shellSettingsPath
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.loadShellSettings(text())
        onLoadFailed: root.loadShellSettings("")
    }

    FileView {
        id: matugenSettingsFile
        path: root.matugenSettingsPath
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.loadMatugenSettings(text())
        onLoadFailed: root.loadMatugenSettings("")
    }

    // PresetTransition waits for this serial. The actual shell palette is
    // already loaded by Ryoku's Theme singleton; Stage only observes its file.
    FileView {
        id: livePaletteFile
        path: root.livePalettePath
        watchChanges: true
        printErrors: false
        onFileChanged: {
            reload();
            root.paletteSerial++;
        }
        onLoaded: root.paletteSerial++
    }

    IpcHandler {
        target: "theme"
        function toggleLightDark(): void { root.toggleLightDark(); }
        function reapplyTheme(): void { root.reapplyTheme(); }
    }
}

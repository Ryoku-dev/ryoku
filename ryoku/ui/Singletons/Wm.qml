pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.WindowManager
import Quickshell.Wayland

// Capability-shaped view of the window manager. The daemon provider owns the
// workspace and window state; Wayland protocols enrich it with handles and
// protocol-only flags. Consumers ask caps.*, never which compositor is running.
Singleton {
    id: root

    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    // The daemon publishes a full snapshot on every provider frame, tagged with
    // a per-section version. Each section is copied into a backing property only
    // when its own version moves, so a window drag rebinds the window-derived
    // lists and leaves the workspace join, the keyboard feed, and the outputs
    // list untouched. Derived read-only properties below depend on the backing
    // property, which is exactly what makes Qt skip their re-evaluation.
    property var _v: null

    property bool _ready: false
    property var _caps: ({})
    property string _workspaceModel: "fixed"
    property string _provider: ""
    property var _configFiles: []
    property string _focusedOutput: ""
    property var _outputs: []
    property bool _overviewOpen: false
    property string _keyboardLayout: ""
    property var _keyboardLayouts: []
    property var _winResidue: []
    property var _wsResidue: []

    readonly property bool ready: root._ready

    // Every capability key is always present as a boolean, so read Wm.caps.<name>
    // directly with no undefined guard.
    readonly property var caps: root._caps
    readonly property string workspaceModel: root._workspaceModel
    // The running window manager as the provider names itself, for display
    // only: behaviour asks caps, never this.
    readonly property string provider: root._provider

    readonly property string focusedOutput: root._focusedOutput
    readonly property var outputs: root._outputs
    readonly property var configFiles: root._configFiles

    // The compositor's native overview is open (niri). False on a compositor
    // with no overview event, so a blur gated on it simply stays off there.
    readonly property bool overviewOpen: root._overviewOpen

    // Active xkb layout name (human string) and the configured layout list.
    readonly property string keyboardLayout: root._keyboardLayout
    readonly property var keyboardLayouts: root._keyboardLayouts

    // Bound at declaration so the Wayland registry binds early; the lists fill in
    // asynchronously after connect.
    readonly property var _windowsets: WindowManager.windowsets
    property var _toplevelModel: ToplevelManager.toplevels
    readonly property var toplevels: root._toplevelModel ? root._toplevelModel.values : []

    function outputByName(name) {
        const outs = root.outputs;
        for (let i = 0; i < outs.length; i++)
            if (outs[i].name === name)
                return outs[i];
        return null;
    }

    // The compositor's scale factor for an output, so a surface can convert
    // its logical rectangle back to physical pixels. 1 until the daemon has
    // reported that output.
    function outputScale(name) {
        const o = root.outputByName(name);
        return o && typeof o.scale === "number" && o.scale > 0 ? o.scale : 1;
    }

    // The provider owns workspace identity and state. ext-workspace-v1 enriches
    // matching names with protocol-only flags; it is a fallback only while the
    // provider list is empty.
    readonly property var workspaces: {
        const sets = root._windowsets || [];
        const byName = {};
        for (let i = 0; i < sets.length; i++)
            byName[sets[i].name] = sets[i];

        const residue = root._wsResidue || [];
        const out = [];
        if (residue.length > 0) {
            for (let i = 0; i < residue.length; i++) {
                const r = residue[i];
                const s = byName[r.name] || null;
                out.push({
                    id: r.id !== undefined && r.id !== "" ? String(r.id) : r.name,
                    name: r.name,
                    active: r.active === true,
                    urgent: s ? s.urgent === true : false,
                    canActivate: s ? s.canActivate === true : root.caps.workspaces === true,
                    windows: r.windows || 0,
                    occupied: (r.windows || 0) > 0,
                    fullscreen: r.fullscreen === true,
                    special: r.special === true,
                    output: r.output || "",
                    layout: r.layout || ""
                });
            }
            return out;
        }

        for (let i = 0; i < sets.length; i++) {
            const s = sets[i];
            out.push({
                id: s.name,
                name: s.name,
                active: s.active === true,
                urgent: s.urgent === true,
                canActivate: s.canActivate === true,
                windows: 0,
                occupied: false,
                fullscreen: false,
                special: false,
                output: "",
                layout: ""
            });
        }
        return out;
    }

    function workspaceById(id) {
        const list = root.workspaces;
        for (let i = 0; i < list.length; i++)
            if (list[i].id === id)
                return list[i];
        return null;
    }

    function workspaceByName(name) {
        const list = root.workspaces;
        for (let i = 0; i < list.length; i++)
            if (list[i].name === name)
                return list[i];
        return null;
    }

    // The workspace shown on the focused output.
    readonly property var focusedWorkspace: {
        const list = root.workspaces;
        for (let i = 0; i < list.length; i++)
            if (list[i].active && list[i].output === root.focusedOutput)
                return list[i];
        for (let i = 0; i < list.length; i++)
            if (list[i].active)
                return list[i];
        return null;
    }

    // True when the workspace shown on `name` holds a fullscreen window.
    function outputHasFullscreen(name) {
        const o = root.outputByName(name);
        if (!o || !o.activeWorkspace)
            return false;
        const ws = root.workspaceByName(o.activeWorkspace);
        return !!ws && ws.fullscreen === true;
    }

    // Daemon window residue (identity, workspace, focus order, geometry) joined to
    // the foreign-toplevel handle for capture and activate. Keyed on
    // (appId, title, output); tied same-key windows pair in list order. Two
    // identical-title windows of one app on one output can swap handles.
    readonly property var windows: {
        const tls = root.toplevels;
        const buckets = ({});
        for (let i = 0; i < tls.length; i++) {
            const t = tls[i];
            const k = (t.appId || "") + "\u0000" + (t.title || "") + "\u0000"
                + (t.screens && t.screens.length ? t.screens[0].name : "");
            (buckets[k] || (buckets[k] = [])).push(t);
        }
        const cursor = ({});
        const out = [];
        for (let i = 0; i < root._winResidue.length; i++) {
            const w = root._winResidue[i];
            const k = (w.appId || "") + "\u0000" + (w.title || "") + "\u0000" + (w.output || "");
            const arr = buckets[k];
            const at = cursor[k] || 0;
            const tl = (arr && at < arr.length) ? arr[at] : null;
            cursor[k] = at + 1;
            out.push({
                id: w.id,
                appId: w.appId || "",
                title: w.title || "",
                workspace: w.workspace || "",
                output: w.output || "",
                focusOrder: (typeof w.focusOrder === "number" ? w.focusOrder : -1),
                floating: w.floating === true,
                x: w.x || 0,
                y: w.y || 0,
                width: w.width || 0,
                height: w.height || 0,
                toplevel: tl
            });
        }
        return out;
    }

    // The foreign-toplevel activated flag marks the focused window on every
    // compositor; focusOrder is only meaningful where caps.focusHistory is set,
    // so it is the fallback.
    readonly property var focusedWindow: {
        const list = root.windows;
        for (let i = 0; i < list.length; i++)
            if (list[i].toplevel && list[i].toplevel.activated)
                return list[i];
        for (let i = 0; i < list.length; i++)
            if (list[i].focusOrder === 0)
                return list[i];
        return null;
    }

    function windowsOnWorkspace(name) {
        const list = root.windows;
        const out = [];
        for (let i = 0; i < list.length; i++)
            if (list[i].workspace === name)
                out.push(list[i]);
        return out;
    }

    // ---- actions ----
    // Each names a neutral action, gates on the capability action.go maps it to
    // (empty capability = always allowed), and rides the daemon `wm.act` call.
    function _act(action, cap, args) {
        if (cap !== "" && root.caps[cap] !== true)
            return;
        root._call("wm.act", { action: action, args: args });
    }

    // Request/response variant: cb receives the wm.act result string, "" on a
    // gated-out call or failure. Used where an action reports state to restore.
    property var _pending: ({})
    property int _nextId: 0
    function _actResult(action, cap, args, cb) {
        if (cap !== "" && root.caps[cap] !== true) {
            if (cb) cb("");
            return;
        }
        const id = ++root._nextId;
        root._pending[id] = cb;
        root._call("wm.act", { action: action, args: args, id: id });
    }

    function focusWindow(id) { root._act("window.focus", "", [String(id)]); }
    function closeWindow(id) { root._act("window.close", "", [String(id)]); }
    function focusApp(appId) { root._act("app.focus", "", [String(appId)]); }
    function floatWindow(id) { root._act("window.float", "windowFloat", [String(id)]); }
    function moveWindowToWorkspace(id, ws) { root._act("window.moveToWorkspace", "workspaces", [String(id), String(ws)]); }

    function focusWorkspace(ws) { root._act("workspace.focus", "workspaces", [String(ws)]); }
    function cycleWorkspace(delta) { root._act("workspace.cycle", "workspaces", [String(delta)]); }
    function moveWorkspaceToOutput(ws, output) { root._act("workspace.moveToOutput", "workspaceMoveToOutput", [String(ws), String(output)]); }
    function toggleSpecialWorkspace(name) { root._act("workspace.toggleSpecial", "specialWorkspace", [String(name || "")]); }

    function cycleKeyboardLayout() { root._act("keyboard.cycleLayout", "keyboardLayoutSwitch", []); }
    function setCursor(theme, size) { root._act("cursor.set", "cursorSet", [String(theme), String(size)]); }
    function reloadConfig(scope) { root._act("config.reload", "configReload", [String(scope || "")]); }
    function setOutputPower(output, on) { root._act("output.power", "outputPower", [String(output), on ? "on" : "off"]); }
    function toggleOverview() { root._act("overview.toggle", "nativeOverview", []); }
    function enterSubmap(name) { root._act("submap.enter", "submap", [String(name)]); }
    function resetSubmap() { root._act("submap.reset", "submap", []); }

    // Live exclusive tweaks. screenShader takes a shader name; focusFollowsMouse
    // sets a follow mode and reports the previous one to cb for the caller to
    // restore.
    function setScreenShader(name) { root._act("decoration.screenShader", "screenShader", [String(name || "")]); }
    function setFocusFollowsMouse(mode, cb) { root._actResult("input.focusFollowsMouse", "liveConfigEval", [String(mode)], cb); }
    function setWorkspaceLayout(ws, layout) { root._act("workspace.layout", "tiledLayout", [String(ws), String(layout)]); }

    // ---- transport ----
    // Copy a section into its backing property only when its version moved, so
    // the derived lists that depend on it rebind only when that section really
    // changed. The first frame ever (and any frame from a pre-version daemon,
    // which sends no versions map) copies everything, so caps and the model are
    // live before the ready frame lands, exactly as the whole-frame swap did.
    function _apply(line) {
        try {
            const frame = JSON.parse(line);
            if (!frame || typeof frame !== "object" || Array.isArray(frame))
                return;
            const v = frame.versions;
            if (!v || root._v === null) {
                root._v = {};
                root._ready = frame.ready === true;
                root._caps = frame.caps || ({});
                root._workspaceModel = frame.workspaceModel || "fixed";
                root._provider = frame.provider || "";
                root._configFiles = frame.configFiles || [];
                root._focusedOutput = frame.focusedOutput || "";
                root._outputs = frame.outputs || [];
                root._overviewOpen = frame.overviewOpen === true;
                root._keyboardLayout = frame.keyboardLayout || "";
                root._keyboardLayouts = frame.keyboardLayouts || [];
                root._winResidue = frame.windows || [];
                root._wsResidue = frame.workspaces || [];
                return;
            }
            const old = root._v;
            if (v.ready !== old.ready) {
                root._ready = frame.ready === true;
                root._caps = frame.caps || ({});
                root._workspaceModel = frame.workspaceModel || "fixed";
                root._provider = frame.provider || "";
                root._configFiles = frame.configFiles || [];
            }
            if (v.windows !== old.windows) root._winResidue = frame.windows || [];
            if (v.workspaces !== old.workspaces) root._wsResidue = frame.workspaces || [];
            if (v.focus !== old.focus) root._focusedOutput = frame.focusedOutput || "";
            if (v.outputs !== old.outputs) root._outputs = frame.outputs || [];
            if (v.keyboard !== old.keyboard) {
                root._keyboardLayout = frame.keyboardLayout || "";
                root._keyboardLayouts = frame.keyboardLayouts || [];
            }
            if (v.overview !== old.overview) root._overviewOpen = frame.overviewOpen === true;
            root._v = v;
        } catch (e) {
        }
    }

    function _call(method, args) {
        ctl.queued += "call " + method + " " + JSON.stringify(args) + "\n";
        if (ctl.connected)
            ctl.flushQueued();
        else
            ctl.connected = true;
    }

    function _reply(line) {
        try {
            const r = JSON.parse(line);
            if (r && r.id !== undefined && root._pending[r.id] !== undefined) {
                const cb = root._pending[r.id];
                delete root._pending[r.id];
                if (cb) cb((r.ok && typeof r.result === "string") ? r.result : "");
            }
        } catch (e) {
        }
    }

    Socket {
        id: sub
        path: root.sockPath
        parser: SplitParser { onRead: line => root._apply(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe wm\n");
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
        path: root.sockPath
        property string queued: ""
        parser: SplitParser { onRead: line => root._reply(line) }
        function flushQueued() {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }
        onConnectionStateChanged: if (connected) flushQueued()
    }
}

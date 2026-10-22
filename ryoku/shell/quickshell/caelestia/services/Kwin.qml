pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Caelestia
import Caelestia.Config
import Ryoku.Ui.Singletons as Ryoku

// The window and workspace layer every Caelestia module reads. The name is the one
// the tree already uses; the data now comes from the neutral Ryoku.Wm singleton, so
// it works on whichever compositor Ryoku runs on. Windows and workspaces keep the
// shape the QML expects: a window has address, class, title, workspace {id, name,
// uuid}, output and geometry; workspaces are numbered by position, one-based, with
// the provider's own key as their id.
Singleton {
    id: root

    readonly property var _wm: Ryoku.Wm

    // Regular workspaces in provider order, then the special ones kept apart so the
    // one-based index of a regular workspace never shifts.
    readonly property var _normal: (root._wm.workspaces || []).filter(w => !w.special)
    readonly property var _special: (root._wm.workspaces || []).filter(w => w.special)

    // Hyprland numbers its workspaces and only keeps the ones in use, so its number is
    // the index, and the strip runs from 1 to the shown count (or the page holding the
    // highest in use);
    // an index that is not there yet is created when it is focused. A provider with
    // opaque ids numbers its workspaces in the order it lists them.
    readonly property bool _numbered: root._normal.length > 0 && root._normal.every(w => /^[1-9][0-9]*$/.test(String(w.id)))
    readonly property var _slots: {
        if (!root._numbered)
            return root._normal;
        const byId = {};
        const page = Math.max(1, GlobalConfig.bar.workspaces.shown);
        let top = page;
        for (const w of root._normal) {
            byId[w.id] = w;
            top = Math.max(top, Number(w.id));
        }
        // whole pages, so the strip keeps its size past the first page (6 shows 6 to 10)
        top = Math.ceil(top / page) * page;
        const out = [];
        for (let n = 1; n <= top; n++)
            out.push(byId[String(n)] ?? { id: String(n), name: String(n) });
        return out;
    }

    readonly property var workspaces: root._slots.map((w, i) => ({
        id: String(w.id),
        uuid: String(w.id),
        name: w.name,
        index: i + 1,
        active: w.active === true,
        urgent: w.urgent === true,
        windows: w.windows || 0,
        occupied: (w.windows || 0) > 0,
        fullscreen: w.fullscreen === true,
        output: w.output || "",
        monitor: w.output || ""
    }))

    // Special workspaces carry negative ids, so a window in one never matches a regular
    // workspace and each special keeps its own.
    readonly property var specialWorkspaces: root._special.map((w, i) => ({
        id: -(100 + i),
        uuid: String(w.id),
        name: root._specialName(w),
        active: w.active === true,
        windows: w.windows || 0,
        output: w.output || "",
        monitor: w.output || "",
        lastIpcObject: { windows: w.windows || 0 }
    }))

    readonly property string activeOutputName: root._wm.focusedOutput || ""
    readonly property int activeWsId: {
        const active = root._wm.focusedWorkspace;
        return active ? root.indexForId(String(active.id)) : -1;
    }
    readonly property var activeByOutput: {
        const out = ({});
        for (const o of (root._wm.outputs || [])) {
            const idx = o.activeWorkspace ? root.indexForId(String(o.activeWorkspace)) : -1;
            if (idx > 0)
                out[o.name] = idx;
        }
        return out;
    }

    readonly property var windowList: (root._wm.windows || []).map(w => root._mapWindow(w))
    readonly property var activeWindow: {
        const focused = root._wm.focusedWindow;
        if (!focused)
            return null;
        const address = String(focused.id);
        return root.windowList.find(w => w.address === address) ?? null;
    }
    readonly property var toplevels: ({ values: root.windowList })
    readonly property var activeToplevel: root.activeWindow
    readonly property var focusedWorkspace: ({ id: root.activeWsId, name: root.activeWsId.toString() })

    // shortcut: the layer publishes no swipe progress, so these hold their idle
    // values, wire them when the window manager seam grows those readings.
    readonly property string pendingFocusAddress: ""
    // The window a dock thumbnail points at; modules/highlight outlines it.
    property string highlightedAddress: ""
    readonly property real swipeOffset: 0
    readonly property var swipeOffsetByOutput: ({})

    readonly property bool capsLock: CUtils.capsLock
    readonly property bool numLock: CUtils.numLock
    readonly property string defaultKbLayout: ""
    readonly property string kbLayoutFull: root._wm.keyboardLayout || ""
    readonly property string kbLayout: root._shortLabel(root.kbLayoutFull)

    // The callers branch on this to pick a dispatcher dialect; the string forms
    // below are the ones dispatch() reads, so it stays false.
    readonly property bool usingLua: false

    // shortcut: Hyprland-style options and the device list have no reading in the
    // seam, so this answers every call with nothing, replace it with real calls
    // when game mode and the device page are wired.
    readonly property QtObject extras: QtObject {
        readonly property var options: ({})
        readonly property var devices: ({ keyboards: [], mice: [], touchpads: [] })
        function message(request: string): void {}
        function batchMessage(requests: var): void {}
        function applyOptions(options: var): void {}
        function refreshDevices(): void {}
    }
    readonly property var options: root.extras.options
    readonly property var devices: root.extras.devices

    property bool hadKeyboard: false
    property string lastSpecialWorkspace: ""

    readonly property var monitors: (root._wm.outputs || []).map((o, i) => root._mapMonitor(o, i))
    readonly property var focusedMonitor: {
        const name = root.activeOutputName;
        const list = root.monitors;
        return list.find(m => m.name === name) ?? list.find(m => m.focused) ?? list[0] ?? null;
    }

    signal configReloaded

    function _shortLabel(label: string): string {
        return (label || "").replace(/\s*\([^)]*\)/, "").replace(/[^a-z]/gi, "").slice(0, 3).toLowerCase();
    }

    function indexForId(id: string): int {
        const list = root._slots;
        for (let i = 0; i < list.length; i++)
            if (String(list[i].id) === String(id))
                return i + 1;
        return -1;
    }

    function uuidForIndex(index: int): string {
        const w = root._slots[index - 1];
        return w ? String(w.id) : "";
    }

    function _specialName(w: var): string {
        return w.name.startsWith("special:") ? w.name : "special:" + w.name;
    }

    function _mapWindow(w: var): var {
        const t = w.toplevel;
        const wsId = String(w.workspace || "");
        const idx = root.indexForId(wsId);
        const specialIdx = idx > 0 ? -1 : root._special.findIndex(sw => String(sw.id) === wsId);
        const special = specialIdx >= 0 ? root._special[specialIdx] : null;
        const hidden = special !== null && special.name.replace("special:", "") === "minimized";
        const win = {
            address: String(w.id),
            "class": w.appId,
            initialClass: w.appId,
            title: w.title,
            pid: w.pid || 0,
            workspace: {
                id: idx > 0 ? idx : (special ? -(100 + specialIdx) : -1),
                name: special ? root._specialName(special) : (idx > 0 ? root._slots[idx - 1].name : ""),
                uuid: wsId
            },
            workspaceUuid: wsId,
            output: w.output,
            monitor: w.output,
            floating: w.floating === true,
            skipTaskbar: w.skipTaskbar === true,
            fullscreen: t ? t.fullscreen === true : false,
            maximized: t ? t.maximized === true : false,
            minimized: hidden || (t ? t.minimized === true : false),
            pinned: false,
            x: w.x,
            y: w.y,
            width: w.width,
            height: w.height,
            at: [w.x, w.y],
            size: [w.width, w.height],
            focusHistoryId: w.focusOrder,
            toplevel: t
        };
        // A copy, so the object never points back at itself.
        win.lastIpcObject = Object.assign({}, win);
        return win;
    }

    function _mapMonitor(o: var, index: int): var {
        const activeIdx = o.activeWorkspace ? root.indexForId(String(o.activeWorkspace)) : -1;
        const special = root._special.find(w => w.active === true && w.output === o.name) ?? null;
        const mon = {
            id: index,
            name: o.name,
            focused: o.name === root.activeOutputName,
            scale: o.scale ?? 1,
            x: o.x ?? 0,
            y: o.y ?? 0,
            width: o.width ?? 0,
            height: o.height ?? 0,
            activeWorkspace: {
                id: activeIdx > 0 ? activeIdx : 1,
                name: activeIdx > 0 ? root._slots[activeIdx - 1].name : "",
                toplevels: { values: root.windowList.filter(w => w.workspace.id === activeIdx) }
            },
            specialWorkspace: {
                name: special ? root._specialName(special) : "",
                toplevels: { values: special ? root.windowList.filter(w => w.workspace.uuid === String(special.id)) : [] }
            }
        };
        mon.lastIpcObject = Object.assign({}, mon);
        return mon;
    }

    function focusWindow(address: string): void {
        root._wm.focusWindow(address);
    }

    function closeWindow(address: string): void {
        root._wm.closeWindow(address);
    }

    // shortcut: stacking is not a seam action, so raise does nothing and sending a
    // window to a named output is not offered.
    function raiseWindow(address: string): void {}
    function highlightWindow(address: string): void { root.highlightedAddress = address; }
    function clearHighlight(): void { root.highlightedAddress = ""; }
    function sendToOutput(address: string, outputName: string): void {}

    // -1 is the current workspace, -2 every workspace (not offered).
    function setWindowDesktop(address: string, desktopId: int): void {
        const target = desktopId === -1 ? String(root._wm.focusedWorkspace?.id ?? "") : root.uuidForIndex(desktopId);
        if (target)
            root._wm.moveWindowToWorkspace(address, target);
    }

    function windowsForWorkspace(workspace: var, includeAll: bool): var {
        return root.filterWindows(root.windowList, workspace, "", includeAll ?? true);
    }

    function activeWorkspaceFor(screenName: string): int {
        const perOutput = (screenName && root.activeByOutput) ? root.activeByOutput[screenName] : 0;
        return perOutput > 0 ? perOutput : (root.activeWsId > 0 ? root.activeWsId : 1);
    }

    function activeWorkspaceUuidFor(screenName: string): string {
        return root.uuidForIndex(root.activeWorkspaceFor(screenName));
    }

    function isWindowOnWorkspace(win: var, wsTarget: var, includeAll: bool): bool {
        if (!win)
            return false;
        const incAll = includeAll ?? true;
        const wsId = win.workspace?.id ?? -1;
        const wsUuid = win.workspaceUuid || win.workspace?.uuid || "";

        if (wsId <= 0 && !wsUuid)
            return incAll;

        if (typeof wsTarget === "number" && wsTarget > 0)
            return wsId === wsTarget || (wsUuid !== "" && wsUuid === root.uuidForIndex(wsTarget));
        if (typeof wsTarget === "string" && wsTarget.length > 0)
            return wsUuid === wsTarget || (wsId > 0 && wsId === root.indexForId(wsTarget));
        return true;
    }

    function filterWindows(list: var, wsTarget: var, screenName: string, includeAllWorkspaces: bool): var {
        const source = list || root.windowList || [];
        const incAll = includeAllWorkspaces ?? true;
        return source.filter(w => {
            if (wsTarget !== undefined && wsTarget !== null && !root.isWindowOnWorkspace(w, wsTarget, incAll))
                return false;
            if (screenName && Quickshell.screens.length > 1) {
                const out = w.output || w.monitor;
                if (out && out !== screenName)
                    return false;
            }
            return true;
        });
    }

    // The same tuning the Ryoku shell's game mode runs (Hyprland live config).
    function setGameMode(on: bool): void {
        Quickshell.execDetached(["ryoku-cmd-game-mode", on ? "start" : "stop"]);
    }

    function cursorOutputName(): string {
        return root.activeOutputName;
    }

    // State arrives pushed, so there is nothing to refresh.
    function refreshWindows(): void {}
    function setActiveOutputName(outputName: string): void {}

    function switchToWorkspace(wsId: var, screenName: string): void {
        root._wm.focusWorkspace(typeof wsId === "number" ? root.uuidForIndex(wsId) : String(wsId));
    }

    // shortcut: the workspace set is fixed, so adding and removing do nothing.
    function createWorkspace(name: string): void {}
    function removeWorkspace(id: string): void {}

    // The Wm action is a toggle, so a request for the state we are already in is
    // dropped.
    function setDesktop(index: int): void {
        root._wm.focusWorkspace(root.uuidForIndex(index));
    }

    function nextDesktop(): void {
        root._wm.cycleWorkspace(1);
    }

    function previousDesktop(): void {
        root._wm.cycleWorkspace(-1);
    }

    function isIgnoredWindow(win: var): bool {
        if (!win)
            return true;
        const cls = String(win["class"] ?? "");
        if (cls === "quickshell")
            return true;
        const ignored = GlobalConfig.bar.workspaces.ignoredTags;
        if (!ignored || ignored.length === 0)
            return false;
        return ignored.includes(cls.replace(/\*$/, ""));
    }

    function hasFullscreen(): bool {
        const wins = root.windowList;
        const activeWs = root.activeWsId;
        for (let i = 0; i < wins.length; i++) {
            if (wins[i].fullscreen === true && !wins[i].minimized) {
                const winWs = wins[i].workspace?.id ?? -1;
                if (activeWs !== -1 && winWs !== -1 && winWs !== activeWs)
                    continue;
                return true;
            }
        }
        return false;
    }

    function hasFullscreenOn(screenName: string): bool {
        if (!screenName)
            return hasFullscreen();
        const wins = root.windowList;
        const activeWs = root.activeWsId;
        for (let i = 0; i < wins.length; i++) {
            if (!wins[i].fullscreen || wins[i].minimized)
                continue;
            if (wins[i].output !== screenName)
                continue;
            const winWs = wins[i].workspace?.id ?? -1;
            if (activeWs !== -1 && winWs !== -1 && winWs !== activeWs)
                continue;
            return true;
        }
        return false;
    }

    function hasWindowOverlapping(screenName: string, x: real, y: real, width: real, height: real, focusedOnly: bool): bool {
        const wins = root.windowList;
        const activeWin = root.activeWindow;
        const activeAddr = activeWin ? String(activeWin.address ?? "") : "";
        const screenWsId = root.activeWorkspaceFor(screenName);
        const activeWinWsId = activeWin?.workspace?.id ?? -1;
        const activeOnThisWs = activeWinWsId === -1 || screenWsId === -1 || activeWinWsId === screenWsId;
        const isActiveScreen = screenName && activeWin && activeWin.output === screenName && activeOnThisWs;
        const applyFocusedOnly = focusedOnly && isActiveScreen && activeAddr.length > 0;

        for (let i = 0; i < wins.length; i++) {
            const win = wins[i];
            if (win.minimized === true)
                continue;
            const winWsId = win.workspace?.id ?? -1;
            if (screenWsId !== -1 && winWsId !== -1 && winWsId !== screenWsId)
                continue;
            if (applyFocusedOnly && String(win.address) !== activeAddr)
                continue;
            if (win.x < x + width && win.x + win.width > x && win.y < y + height && win.y + win.height > y)
                return true;
        }
        return false;
    }

    function windowHidesDesktopWidgets(screenName: string, hideOnAll: bool): bool {
        const wins = root.windowList;

        const isMaximizedOnWs = (win, outName) => {
            if (win.minimized === true || (!win.maximized && !win.fullscreen))
                return false;
            const winWs = win.workspace?.id ?? -1;
            const activeWs = root.activeWorkspaceFor(outName);
            return activeWs === -1 || winWs === -1 || winWs === activeWs;
        };

        if (hideOnAll)
            return wins.some(w => isMaximizedOnWs(w, w.output || screenName));
        return wins.some(w => (screenName === "" || w.output === screenName) && isMaximizedOnWs(w, screenName));
    }

    // A window address may arrive with a leading 0x or two; match it on the digits.
    function _resolveAddress(raw: string): string {
        const bare = raw.trim().replace(/^(0x)+/i, "");
        const hit = root.windowList.find(w => w.address.replace(/^(0x)+/i, "") === bare);
        return hit ? hit.address : raw.trim();
    }

    // Reads the request forms the QML sends and routes each to a seam action.
    function dispatch(request: string): void {
        if (request.startsWith("workspace ")) {
            const ws = request.split(" ").slice(1).join(" ");
            if (/^r[+-]\d+$/.test(ws)) {
                if (ws.charAt(1) === "+")
                    root.nextDesktop();
                else
                    root.previousDesktop();
            } else if (ws.startsWith("special:")) {
                root._wm.toggleSpecialWorkspace(ws.slice("special:".length));
            } else {
                root.switchToWorkspace(ws);
            }
            return;
        }

        if (request.startsWith("focuswindow address:")) {
            root.focusWindow(root._resolveAddress(request.slice("focuswindow address:".length)));
            return;
        }

        if (request.startsWith("closewindow address:")) {
            root.closeWindow(root._resolveAddress(request.slice("closewindow address:".length)));
            return;
        }

        const moveMatch = request.match(/^movetoworkspace\s+(\S+),address:/);
        if (moveMatch) {
            const desktopId = parseInt(moveMatch[1], 10);
            const addr = root._resolveAddress(request.slice(moveMatch[0].length));
            if (!isNaN(desktopId))
                root.setWindowDesktop(addr, desktopId);
            return;
        }

        if (request === "dpms off" || request === "dpms on") {
            for (const o of (root._wm.outputs || []))
                root._wm.setOutputPower(o.name, request === "dpms on");
            return;
        }

        if (request.startsWith("togglespecialworkspace"))
            root._wm.toggleSpecialWorkspace(request.slice("togglespecialworkspace".length).trim());
    }

    function cycleSpecialWorkspace(direction: string): void {
        const openSpecials = root._special.filter(w => (w.windows ?? 0) > 0).map(w => root._specialName(w));
        if (openSpecials.length === 0)
            return;

        const activeSpecial = root.focusedMonitor?.specialWorkspace?.name ?? "";
        if (!activeSpecial) {
            const target = openSpecials.includes(root.lastSpecialWorkspace) ? root.lastSpecialWorkspace : openSpecials[0];
            root.dispatch(`workspace ${target}`);
            return;
        }

        const currentIndex = openSpecials.indexOf(activeSpecial);
        const nextIndex = currentIndex === -1 ? 0
            : (direction === "next")
                ? (currentIndex + 1) % openSpecials.length
                : (currentIndex - 1 + openSpecials.length) % openSpecials.length;
        root.dispatch(`workspace ${openSpecials[nextIndex]}`);
    }

    function monitorNames(): list<string> {
        return root.monitors.map(m => m.name);
    }

    function monitorFor(screen: ShellScreen): var {
        return root.monitors.find(m => m.name === screen.name) ?? root._mapMonitor({ name: screen.name }, -1);
    }

    function refreshDevices(): void {
        root.extras.refreshDevices();
    }

    function listSpecialWorkspaces(): string {
        return root._special.filter(w => (w.windows ?? 0) > 0).map(w => root._specialName(w)).join("\n");
    }

    function getFocusedMonitor(): string {
        const m = root.focusedMonitor;
        if (!m)
            return "null";
        return JSON.stringify({ id: m.id, name: m.name, focused: m.focused, activeWorkspace: m.activeWorkspace, specialWorkspace: m.specialWorkspace }, null, 2);
    }

    function listMonitors(): string {
        return root.monitorNames().join(", ");
    }

    onCapsLockChanged: {
        if (!GlobalConfig.utilities.toasts.capsLockChanged)
            return;
        Toaster.toast(
            capsLock ? qsTr("Caps lock enabled") : qsTr("Caps lock disabled"),
            capsLock ? qsTr("Caps lock is currently enabled") : qsTr("Caps lock is currently disabled"),
            capsLock ? "keyboard_capslock_badge" : "keyboard_capslock"
        );
    }

    onNumLockChanged: {
        if (!GlobalConfig.utilities.toasts.numLockChanged)
            return;
        Toaster.toast(
            numLock ? qsTr("Num lock enabled") : qsTr("Num lock disabled"),
            numLock ? qsTr("Num lock is currently enabled") : qsTr("Num lock is currently disabled"),
            numLock ? "looks_one" : "timer_1"
        );
    }

    onKbLayoutFullChanged: {
        if (hadKeyboard && GlobalConfig.utilities.toasts.kbLayoutChanged)
            Toaster.toast(qsTr("Keyboard layout changed"), qsTr("Layout changed to: %1").arg(kbLayoutFull), "keyboard");
        hadKeyboard = kbLayoutFull.length > 0;
    }

    IpcHandler {
        function refreshDevices(): void {
            root.refreshDevices();
        }

        function cycleSpecialWorkspace(direction: string): void {
            root.cycleSpecialWorkspace(direction);
        }

        function listSpecialWorkspaces(): string {
            return root.listSpecialWorkspaces();
        }

        function getFocusedMonitor(): string {
            return root.getFocusedMonitor();
        }

        function listMonitors(): string {
            return root.listMonitors();
        }

        target: "kwin"
    }

    IpcHandler {
        function refreshDevices(): void {
            root.refreshDevices();
        }

        function cycleSpecialWorkspace(direction: string): void {
            root.cycleSpecialWorkspace(direction);
        }

        function listSpecialWorkspaces(): string {
            return root.listSpecialWorkspaces();
        }

        function getFocusedMonitor(): string {
            return root.getFocusedMonitor();
        }

        function listMonitors(): string {
            return root.listMonitors();
        }

        target: "hypr"
    }
}

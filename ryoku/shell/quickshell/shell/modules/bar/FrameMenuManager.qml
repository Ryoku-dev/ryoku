pragma ComponentBehavior: Bound

import QtQuick
import "framebars/MenuState.js" as MenuState
import Ryoku.FrameBars
import shell.services
import "popouts"

// The single per-monitor frame menu manager. It repeats the configured menu
// records and owns which record is active at each anchor for THIS monitor.
// Opening a second menu at a busy anchor replaces the first; each monitor keeps
// its own slice of state, so the same anchor stays independent across monitors.
// shell.qml unions each open menu's trigger and body rects (masks) into the one
// overlay input mask; released regions fall through to applications.
Item {
    id: root

    anchors.fill: parent

    required property string monitorName
    required property real scale
    required property var group
    // Clearance from the screen edge to the inside of each rail, per edge. A
    // surface grows out of its own anchor's edge, so it must clear THAT rail;
    // a single top-rail figure tucked every left and right body under the side
    // rails.
    required property var railClearances
    readonly property real frameThickness: root.clearanceFor(root.barEdge)
    function clearanceFor(anchor) {
        const edge = anchor.indexOf("top") === 0 ? "top"
            : anchor.indexOf("bottom") === 0 ? "bottom"
            : anchor;
        const value = root.railClearances ? root.railClearances[edge] : undefined;
        return typeof value === "number" ? value : 0;
    }
    property bool active: true

    // A folder bar style (topBar) is a single sash on one screen edge with no
    // other rails, so frame menus fold to barEdge or its matching corner.
    property bool topBar: false
    property string barEdge: "top"   // the edge the folded bar sits on: top | bottom
    function mapAnchor(a) {
        if (!root.topBar || !a) return a;
        if (root.barEdge === "bottom") {
            if (a === "left") return "bottom-left";
            if (a === "right") return "bottom-right";
            if (a === "top") return "bottom";
            if (a === "top-left") return "bottom-left";
            if (a === "top-right") return "bottom-right";
            return a;
        }
        if (a === "left") return "top-left";
        if (a === "right") return "top-right";
        if (a === "bottom") return "top";
        if (a === "bottom-left") return "top-left";
        if (a === "bottom-right") return "top-right";
        return a;
    }

    readonly property var menus: {
        const src = Config.normalizedFrameBars.menus || ({});
        const out = [];
        for (const id in src) {
            // "screenshot" migrated from an in-band menu to the Super+S capture
            // card (a surface). Ignore any lingering screenshot_menu in a config
            // the doctor has not yet reconciled, so the surface -- not the retired
            // menu -- answers `menu screenshot`.
            if (id === "screenshot") continue;
            out.push(Object.assign({ id: id, kind: "menu" }, src[id]));
        }
        return out;
    }
    readonly property var surfaces: {
        const out = [];
        out.push(
            { id: "voice", kind: "voice", anchor: "bottom", minWidth: 380 },
            { id: "keyring", kind: "keyring", anchor: "top", minWidth: 420 },
            // polkit asks for an administrator password: same island as the
            // keyring prompt, and modal for the same reason.
            { id: "polkit", kind: "polkit", anchor: "top", minWidth: 420 },
            // the rail spectrum's music card: a pointer-only popout, so it
            // joins the surfaces without taking the keyboard.
            { id: "music", kind: "music", anchor: "left", minWidth: 264 },
            // the rail bluetooth widget's device card: a left-anchored sliding
            // card, opening and dismissing exactly like the music card.
            { id: "bluetooth", kind: "bluetooth", anchor: "left", minWidth: 272 },
            // the rail battery + network widgets' cards: left-anchored sliding
            // cards like the others; network takes on-demand keyboard for a
            // Wi-Fi password, battery is pointer-only.
            { id: "battery", kind: "battery", anchor: "left", minWidth: 244 },
            { id: "network", kind: "network", anchor: "left", minWidth: 250 },
            // the rail system-monitor widget's card: CPU / memory / temp gauges
            // on a left-anchored sliding card like the others.
            { id: "sysmon", kind: "sysmon", anchor: "left", minWidth: 260 },
            // the rail speaker/mic widgets' card: the audio mixer -- output,
            // input, per-app playback and capture -- on a left-anchored
            // sliding card like the others, pointer-only (faders, no keys).
            { id: "audio", kind: "audio", anchor: "left", minWidth: 300 },
            // The rail capture card: a left-anchored sliding card like the
            // other rail cards. Pointer-only.
            { id: "screenshot", kind: "screenshot", anchor: "left", minWidth: 300 }
        );
        return out;
    }
    readonly property var records: {
        const all = root.menus.concat(root.surfaces);
        return root.topBar ? all.map(r => Object.assign({}, r, { anchor: root.mapAnchor(r.anchor) })) : all;
    }

    // { [monitor]: { [anchor]: record } }, driven by the pure MenuState model.
    property var menuState: ({})
    signal surfaceClosed(string id, var context)
    signal pluginUnpinRequested(string pluginId)

    readonly property bool anyOpen: {
        const mon = menuState[monitorName];
        if (!mon) return false;
        for (const a in mon) if (mon[a]) return true;
        return false;
    }
    // Ryoku-owned credential and plugin surfaces keep outside-click dismissal
    // and keyboard focus. Pointer-only cards and voice stay passive.
    readonly property bool surfaceModal: {
        const mon = menuState[monitorName];
        if (!mon) return false;
        for (const anchor in mon)
            if (mon[anchor] && mon[anchor].kind && mon[anchor].kind !== "menu" && mon[anchor].id !== "voice" && mon[anchor].id !== "music" && mon[anchor].id !== "bluetooth" && mon[anchor].id !== "battery" && mon[anchor].id !== "network" && mon[anchor].id !== "sysmon" && mon[anchor].id !== "audio" && mon[anchor].id !== "screenshot") return true;
        return false;
    }
    // Keyboard focus the frame raises while something is open (contract 05 sec
    // 4): OnDemand for the network menu, None for every pointer-only menu.
    // Ryoku's own modal surfaces take Exclusive; the voice toast takes none.
    readonly property string keyboardMode: {
        const mon = menuState[monitorName];
        if (!mon) return "none";
        for (const anchor in mon) {
            const rec = mon[anchor];
            if (!rec) continue;
            if (rec.id === "network") return "ondemand";
            if (rec.kind && rec.kind !== "menu" && rec.id !== "voice" && rec.id !== "music" && rec.id !== "bluetooth" && rec.id !== "battery" && rec.id !== "network" && rec.id !== "sysmon" && rec.id !== "audio" && rec.id !== "screenshot") return "exclusive";
        }
        return "none";
    }

    // The single reference menu (kind "menu") open on this monitor, or null.
    // Ryoku-owned credential, voice, and plugin surfaces remain popouts.
    readonly property var activeMenu: {
        const mon = menuState[monitorName];
        if (!mon) return null;
        for (const a in mon)
            if (mon[a] && mon[a].kind === "menu") return mon[a];
        return null;
    }

    // Latched chrome-band source: the open menu's resting panel rect + anchor,
    // held through the close so the band retracts along the right edge instead
    // of vanishing. A side menu (left/right) slides; an edge/corner menu scales.
    property var chromeRest: ({ x: 0, y: 0, w: 0, h: 0 })
    property string chromeAnchor: ""
    property bool chromeSide: true
    property string chromeOwner: ""
    property var pendingChrome: ({})
    property string waitingChromeId: ""
    readonly property bool chromeSwitchPending: !!(root.pendingChrome && root.pendingChrome.id)
    Timer {
        id: chromeSwitchTimer
        interval: root.chromeSide ? Motion.sidebarExit : Motion.diagonal
        onTriggered: {
            const next = root.pendingChrome;
            root.pendingChrome = ({});
            if (!next || !next.id || !root.activeMenu || root.activeMenu.id !== next.id)
                return;
            root.applyChromeSource(next);
        }
    }
    // 0 closed .. 1 open. Full-height sidebars follow iNiR's window-local
    // slide: their already-sized panel translates from beyond the screen edge,
    // entering over 400 ms and leaving over 200 ms. Compact menus retain the
    // frame's edge/corner scale.
    property real chromeReveal: 0
    readonly property bool anyVisible: root.anyOpen || root.chromeReveal > 0
    readonly property bool chromeEntering: !!root.activeMenu
        && !root.chromeSwitchPending && root.waitingChromeId === ""
    Behavior on chromeReveal {
        NumberAnimation {
            duration: root.chromeSide
                ? (root.chromeEntering ? Motion.sidebarEnter : Motion.sidebarExit)
                : Motion.diagonal
            easing.type: root.chromeSide ? Easing.BezierSpline : Motion.diagonalCurve
            easing.bezierCurve: root.chromeEntering
                ? Motion.sidebarEnterCurve : Motion.sidebarExitCurve
        }
    }
    property real chromeOpacity: 0
    Behavior on chromeOpacity {
        NumberAnimation {
            duration: !root.chromeSide ? 0
                : Math.round((root.chromeEntering ? Motion.sidebarEnter : Motion.sidebarExit) * 0.7)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: root.chromeEntering
                ? Motion.sidebarFadeInCurve : Motion.sidebarFadeOutCurve
        }
    }

    function applyChromeSource(source) {
        root.chromeOwner = source.id;
        root.chromeAnchor = source.anchor;
        root.chromeRest = { x: source.x, y: source.y, w: source.w, h: source.h };
        root.chromeSide = source.side;
        root.chromeReveal = root.chromeOpacity = 1;
    }
    // A retained body may still be incubating when it replaces a menu on
    // another edge. Retract the outgoing panel now, but do not expose an empty
    // incoming panel; body readiness will call setChromeSource with its rect.
    function prepareChromeSource(id, anchor) {
        if (root.chromeOwner === "" || root.chromeOwner === id || root.chromeAnchor === anchor)
            return;
        chromeSwitchTimer.stop();
        root.pendingChrome = ({});
        root.waitingChromeId = id;
        root.chromeReveal = root.chromeOpacity = 0;
    }
    function setChromeSource(id, anchor, x, y, w, h, isSide) {
        const source = { id: id, anchor: anchor, x: x, y: y, w: w, h: h, side: isSide };
        if (root.waitingChromeId !== "" && root.waitingChromeId !== id)
            root.waitingChromeId = "";
        if (root.waitingChromeId === id && root.chromeReveal > 0.004) {
            root.pendingChrome = source;
            root.waitingChromeId = "";
            root.chromeReveal = root.chromeOpacity = 0;
            chromeSwitchTimer.restart();
            return;
        }
        if (root.waitingChromeId === id)
            root.waitingChromeId = "";
        if (root.chromeOwner === ""
                || (root.chromeOwner === id && root.chromeAnchor === anchor)
                || root.chromeAnchor === anchor || root.chromeReveal <= 0.004) {
            chromeSwitchTimer.stop();
            root.pendingChrome = ({});
            root.applyChromeSource(source);
            return;
        }
        if (root.chromeSwitchPending && root.pendingChrome.id === id) {
            root.pendingChrome = source;
            return;
        }
        root.pendingChrome = source;
        root.chromeReveal = root.chromeOpacity = 0;
        chromeSwitchTimer.restart();
    }
    onActiveMenuChanged: {
        if (!root.activeMenu) {
            chromeSwitchTimer.stop();
            root.pendingChrome = ({});
            root.waitingChromeId = "";
            root.chromeReveal = root.chromeOpacity = 0;
        } else if (root.waitingChromeId !== ""
                && root.waitingChromeId !== root.activeMenu.id) {
            root.waitingChromeId = "";
        }
    }

    // The panel rect shared by the shell chrome and clipped menu body. A
    // full-height sidebar keeps its resting size and translates as one unit,
    // so its QML subtree never relayouts during the animation. Compact edge and
    // corner menus still scale from their anchor origin.
    readonly property var chromePanel: {
        const r = root.chromeRest;
        const p = root.chromeReveal;
        const a = root.chromeAnchor;
        const rr = r.x + r.w;
        const rb = r.y + r.h;
        const cx = r.x + r.w / 2;
        const wp = r.w * p;
        const hp = r.h * p;
        if (root.chromeSide && (a === "left" || a.indexOf("-left") >= 0))
            return { anchor: a, x: r.x - (r.x + r.w) * (1 - p), y: r.y, w: r.w, h: r.h };
        if (root.chromeSide && (a === "right" || a.indexOf("-right") >= 0))
            return { anchor: a, x: r.x + (root.width - r.x) * (1 - p), y: r.y, w: r.w, h: r.h };
        if (a === "left")         return { anchor: a, x: r.x,         y: r.y,     w: wp, h: r.h };
        if (a === "right")        return { anchor: a, x: rr - wp,     y: r.y,     w: wp, h: r.h };
        if (a === "top")          return { anchor: a, x: cx - wp / 2, y: r.y,     w: wp, h: hp };
        if (a === "bottom")       return { anchor: a, x: cx - wp / 2, y: rb - hp, w: wp, h: hp };
        if (a === "top-left")     return { anchor: a, x: r.x,         y: r.y,     w: wp, h: hp };
        if (a === "top-right")    return { anchor: a, x: rr - wp,     y: r.y,     w: wp, h: hp };
        if (a === "bottom-left")  return { anchor: a, x: r.x,         y: rb - hp, w: wp, h: hp };
        if (a === "bottom-right") return { anchor: a, x: rr - wp,     y: rb - hp, w: wp, h: hp };
        return { anchor: "", x: r.x, y: r.y, w: 0, h: 0 };
    }

    // per-anchor trigger (owner) and body (Popout) mask rects, in window
    // coordinates. Zero rects for idle anchors contribute nothing to the mask.
    readonly property var masks: {
        const out = ({});
        const list = MenuCatalog.anchors();
        for (let k = 0; k < list.length; ++k) {
            const a = list[k];
            const rec = MenuState.activeAt(menuState, monitorName, a);
            const t = rec && rec.trigger ? rec.trigger : null;
            out[a] = {
                tx: t ? t.x : 0, ty: t ? t.y : 0, tw: t ? t.width : 0, th: t ? t.height : 0,
                bx: 0, by: 0, bw: 0, bh: 0
            };
        }
        const n = menuRepeater.count;
        for (let i = 0; i < n; ++i) {
            const fm = menuRepeater.itemAt(i);
            if (!fm || fm.maskW <= 0 || fm.maskH <= 0 || !out[fm.anchor]) continue;
            out[fm.anchor].bx = fm.maskX;
            out[fm.anchor].by = fm.maskY;
            out[fm.anchor].bw = fm.maskW;
            out[fm.anchor].bh = fm.maskH;
        }
        const plugin = pluginPopouts.first;
        if (plugin && plugin.pinned && out[plugin.edge]) {
            out[plugin.edge].tx = plugin.triggerX;
            out[plugin.edge].ty = plugin.triggerY;
            out[plugin.edge].tw = plugin.triggerW;
            out[plugin.edge].th = plugin.triggerH;
            out[plugin.edge].bx = plugin.maskX;
            out[plugin.edge].by = plugin.maskY;
            out[plugin.edge].bw = plugin.maskW;
            out[plugin.edge].bh = plugin.maskH;
        }
        if (dockPreviewHost.maskW > 0 && dockPreviewHost.maskH > 0 && out[dockPreviewHost.edge]) {
            out[dockPreviewHost.edge].bx = dockPreviewHost.maskX;
            out[dockPreviewHost.edge].by = dockPreviewHost.maskY;
            out[dockPreviewHost.edge].bw = dockPreviewHost.maskW;
            out[dockPreviewHost.edge].bh = dockPreviewHost.maskH;
        }
        if (musicHoverHost.maskW > 0 && musicHoverHost.maskH > 0 && out[musicHoverHost.edge]) {
            out[musicHoverHost.edge].bx = musicHoverHost.maskX;
            out[musicHoverHost.edge].by = musicHoverHost.maskY;
            out[musicHoverHost.edge].bw = musicHoverHost.maskW;
            out[musicHoverHost.edge].bh = musicHoverHost.maskH;
        }
        return out;
    }

    // The dock preview body rect on its own, so the overlay mask can catch it in
    // every bar style. The per-anchor `masks` above only reaches the input region
    // through railRegion (sumi); an islands/qsbar bar uses dragRegion, so without
    // this the live tiles are unreachable and the strip melts as you reach for it.
    readonly property var dockMask: ({
        x: dockPreviewHost.maskX,
        y: dockPreviewHost.maskY,
        w: dockPreviewHost.maskW,
        h: dockPreviewHost.maskH
    })

    // Music card body rect on its own, like dockMask, so it catches input in
    // every bar style.
    readonly property var musicMask: ({
        x: musicHoverHost.maskX,
        y: musicHoverHost.maskY,
        w: musicHoverHost.maskW,
        h: musicHoverHost.maskH
    })

    // A centred plugin popout hangs off no screen edge, so it cannot ride the
    // per-anchor `masks` above (they are keyed by edge). Carry its body rect on
    // its own, the way dockMask does, so the modal catches input in every bar
    // style instead of clicking straight through to the desktop.
    readonly property var pluginMask: {
        const pop = pluginPopouts.first;
        const live = pop && pop.centered && pop.maskW > 0 && pop.maskH > 0;
        return ({
            x: live ? pop.maskX : 0,
            y: live ? pop.maskY : 0,
            w: live ? pop.maskW : 0,
            h: live ? pop.maskH : 0
        });
    }

    function activeIdAt(anchor) {
        const rec = MenuState.activeAt(menuState, monitorName, anchor);
        return rec ? rec.id : "";
    }
    function alongAt(anchor) {
        const rec = MenuState.activeAt(menuState, monitorName, anchor);
        return rec && rec.along !== undefined ? rec.along : -1;
    }
    // The live record active at this anchor when its id matches lets a delegate
    // read dynamic fields such as voice's `off`; static records do not carry it.
    function openRecordAt(anchor, id) {
        const rec = MenuState.activeAt(root.menuState, root.monitorName, anchor);
        return rec && rec.id === id ? rec : null;
    }

    // The anchor where a record id is currently open on this monitor, or "" if
    // it is closed. A record's live anchor can differ from its config anchor
    // because a rail widget opens its popout on the rail's own edge.
    function liveAnchorFor(id) {
        const mon = root.menuState[root.monitorName];
        if (!mon) return "";
        for (const anchor in mon) if (mon[anchor] && mon[anchor].id === id) return anchor;
        return "";
    }
    // The screen edge (top/bottom/left/right) nearest a point, so a popout welds
    // to the rail edge its trigger widget hugs.
    function edgeNearest(x, y) {
        const d = { top: y, bottom: root.height - y, left: x, right: root.width - x };
        let best = "top";
        for (const e in d) if (d[e] < d[best]) best = e;
        return best;
    }

    // The rail edge a trigger widget sits ON. A rail is a thin strip along one
    // screen edge (railClearances is its inside depth), so a widget within a
    // rail's depth welds its popout to THAT rail -- even a left-rail widget at
    // the very bottom, which edgeNearest would mis-assign to the bottom edge and
    // melt with the wrong (dipping, centre-narrowing) geometry. Only edges that
    // carry a live rail (clearance > 0) qualify; otherwise fall back to nearest.
    // Corner ownership decides the order. A horizontal rail spans the full width
    // and owns both shared corners (Bar.qml gives it leadInset 0); a vertical one
    // is inset to fit BETWEEN them. So a point in a shared corner belongs to the
    // horizontal rail, and top/bottom must be tested first -- testing left first
    // stole every top/bottom widget sitting within a side rail's depth and opened
    // its menu welded to the wrong edge.
    function edgeForTrigger(x, y) {
        const c = root.railClearances || ({});
        const cl = c.left || 0, cr = c.right || 0, ct = c.top || 0, cb = c.bottom || 0;
        if (ct > 0 && y <= ct) return "top";
        if (cb > 0 && root.height - y <= cb) return "bottom";
        if (cl > 0 && x <= cl) return "left";
        if (cr > 0 && root.width - x <= cr) return "right";
        return root.edgeNearest(x, y);
    }

    // Single-open invariant (contract 05 sec 4): before revealing a menu or
    // surface, close every other open one on this monitor, so at most one is
    // ever shown per frame. Daemon-lifecycle toasts (voice, keyring) are exempt
    // and are neither closed here nor closed by opening a menu.
    function closeOtherUsers(state, keepId) {
        const mon = state[root.monitorName];
        if (!mon) return state;
        let next = state;
        for (const anchor in mon) {
            const rec = mon[anchor];
            if (!rec || rec.id === keepId || rec.id === "voice" || rec.id === "keyring") continue;
            root.surfaceClosed(rec.id, rec);
            next = MenuState.closeAt(next, root.monitorName, anchor);
        }
        return next;
    }
    function openSurface(id, ownerRect, requestedMonitor, context) {
        if (requestedMonitor !== undefined && requestedMonitor !== "" && requestedMonitor !== root.monitorName) return;
        const voiceOff = id === "voice-off";
        const surfaceID = voiceOff ? "voice" : id;
        if (surfaceID.indexOf("plugin:") === 0) {
            root.openPlugin(surfaceID.substring(7));
            return;
        }
        const rec = MenuState.recordFor(root.records, surfaceID);
        if (!rec) return;
        // Asking again for the surface that already owns this anchor closes it:
        // a bar button and its command both read as one toggle. Keyring and
        // voice are daemon lifecycle surfaces, not user toggles: a fresh prompt
        // or a re-show must replace the live record, never dismiss it.
        const daemonOwned = surfaceID === "keyring" || surfaceID === "voice";
        // Where this surface is currently open. Its live anchor may differ from
        // the config anchor because a rail widget welds its popout to its edge.
        // Find it wherever it lives so asking again closes it.
        const openAnchor = root.liveAnchorFor(surfaceID);
        if (!daemonOwned && openAnchor !== "") {
            root.closeAt(openAnchor);
            return;
        }
        let local;
        if (ownerRect && ownerRect.width > 0 && ownerRect.height > 0) {
            local = root.mapFromGlobal(ownerRect.x, ownerRect.y);
            if (local.x < 0 || local.y < 0 || local.x >= root.width || local.y >= root.height) return;
        } else {
            local = { x: root.width / 2, y: root.height / 2 };
            ownerRect = { width: 1, height: 1 };
        }
        // A surface opened from a rail widget grows out of THAT rail's edge, so
        // the popout stays welded to its trigger wherever the user placed the
        // rail. The edge is the screen side the trigger hugs; a corner config
        // anchor (wallpaper, recording) collapses to that edge. A command or IPC
        // open carries no widget rect, so it keeps the record's config anchor.
        const cx = local.x + ownerRect.width / 2;
        const cy = local.y + ownerRect.height / 2;
        const realTrigger = ownerRect.width > 1 || ownerRect.height > 1;
        const anchor = realTrigger ? root.edgeForTrigger(cx, cy) : rec.anchor;
        const horiz = anchor.indexOf("top") === 0 || anchor.indexOf("bottom") === 0;
        const along = horiz ? cx : cy;
        const trigger = { x: local.x, y: local.y, width: ownerRect.width, height: ownerRect.height };
        let base = daemonOwned ? root.menuState : root.closeOtherUsers(root.menuState, surfaceID);
        const occupied = MenuState.activeAt(base, root.monitorName, anchor);
        if (occupied && occupied.id !== surfaceID) {
            root.surfaceClosed(occupied.id, occupied);
            base = MenuState.closeAt(base, root.monitorName, anchor);
        }
        root.menuState = MenuState.open(base, root.monitorName,
            Object.assign({}, rec, { id: surfaceID, anchor: anchor, along: along, trigger: trigger,
                off: voiceOff, promptId: surfaceID === "keyring" && context ? context.promptId : undefined }));
    }
    function openMenu(id, ownerRect) {
        root.openSurface(id, ownerRect, root.monitorName);
    }
    function openMenuAt(id, x, y) {
        root.openSurface(id, { x: x, y: y, width: 1, height: 1 }, root.monitorName);
    }
    function openPlugin(pluginID) {
        if (pluginID === "") return;
        const id = "plugin:" + pluginID;
        if (root.activeIdAt("top") === id) {
            root.closeAt("top");
            return;
        }
        const base = root.closeOtherUsers(root.menuState, id);
        const rec = { id: id, kind: "plugin", anchor: "top", along: root.width / 2,
            trigger: { x: root.width / 2, y: root.height / 2, width: 1, height: 1 } };
        root.menuState = MenuState.open(base, root.monitorName, rec);
    }
    function closeSurface(id, requestedMonitor) {
        if (requestedMonitor !== undefined && requestedMonitor !== "" && requestedMonitor !== root.monitorName) return;
        if (id === "" || id === undefined) {
            root.closeAll();
            return;
        }
        if (id.indexOf("plugin:") === 0) {
            if (root.activeIdAt("top") === id) root.closeAt("top");
            return;
        }
        root.closeMenu(id === "voice-off" ? "voice" : id);
    }
    function retireKeyringPrompt(promptId) {
        const active = MenuState.activeAt(root.menuState, root.monitorName, "top");
        if (active && active.id === "keyring" && active.promptId !== promptId) root.closeAt("top");
    }

    function closeAt(anchor) {
        const active = MenuState.activeAt(root.menuState, root.monitorName, anchor);
        if (!active) return;
        root.surfaceClosed(active.id, active);
        root.menuState = MenuState.closeAt(root.menuState, root.monitorName, anchor);
    }
    function closeMenu(id) {
        const anchor = root.liveAnchorFor(id);
        if (anchor !== "") root.closeAt(anchor);
    }
    function closeAll() {
        const mon = root.menuState[root.monitorName];
        if (!mon) return;
        for (const anchor in mon) root.closeAt(anchor);
    }

    onActiveChanged: if (!root.active) root.closeAll()

    Repeater {
        id: menuRepeater
        model: root.records
        delegate: FrameMenu {
            required property var modelData
            // A record renders wherever it is open (its live anchor), which a
            // rail widget pins to its own edge; the config anchor is only the
            // resting fallback while the record is closed.
            readonly property string liveAnchor: root.liveAnchorFor(modelData.id)
            // Latched through the close, the same way the manager holds
            // chromeRest and Popout holds heldAlong. Snapping straight back to
            // the config anchor moved the resting rect mid-close, and that moved
            // rect re-entered pushChrome and drove the reveal back to 1 -- the
            // band stranded open as an empty panel on the config anchor's edge,
            // long after the menu it belonged to was gone.
            property string heldAnchor: ""
            onLiveAnchorChanged: if (liveAnchor !== "") heldAnchor = liveAnchor
            readonly property string effectiveAnchor: liveAnchor !== "" ? liveAnchor
                : (heldAnchor !== "" ? heldAnchor : modelData.anchor)
            group: root.group
            frameThickness: root.clearanceFor(effectiveAnchor)
            clearances: root.railClearances
            radius: Theme.radiusWindow
            smoothing: 0
            s: root.scale
            active: root.active
            manager: root
            record: modelData
            anchor: effectiveAnchor
            menuOpen: root.active && liveAnchor !== ""
            openRecord: liveAnchor !== "" ? root.openRecordAt(liveAnchor, modelData.id) : null
            triggerAlong: liveAnchor !== "" ? root.alongAt(liveAnchor) : -1
            onRequestClose: root.closeMenu(modelData.id)
        }
    }

    PluginPopouts {
        id: pluginPopouts
        group: root.group
        s: root.scale
        active: root.active
        frameThickness: root.frameThickness
        radius: Theme.radiusWindow
        smoothing: 0
        pinnedId: {
            const id = root.activeIdAt("top");
            return id.indexOf("plugin:") === 0 ? id.substring(7) : "";
        }
        onUnpinRequested: pluginId => root.pluginUnpinRequested(pluginId)
    }

    // The dock's hover window-preview strip: one self-hovering Popout per
    // monitor, driven by the DockPreview singleton the dock writes on icon
    // hover. Not a menu-state surface; its body mask is unioned above so the
    // live tiles are clickable over the desktop hole.
    DockPreviewPopout {
        id: dockPreviewHost
        group: root.group
        s: root.scale
        active: root.active
        frameThickness: root.clearanceFor(DockPreview.edge)
    }

    // The dock's now-playing card, driven by MusicPreview on chip hover.
    MusicHoverPopout {
        id: musicHoverHost
        group: root.group
        s: root.scale
        active: root.active
        frameThickness: root.clearanceFor(MusicPreview.edge)
    }
    onPluginUnpinRequested: pluginId => root.closeSurface("plugin:" + pluginId, root.monitorName)
}

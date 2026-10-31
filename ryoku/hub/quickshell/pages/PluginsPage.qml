pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import ".."
import "../schema/PluginsPage.js" as Schema
import "../Singletons"

// Plugins: every Hyprland compositor plugin on this machine in one place. One
// tab per plugin; above its settings sits a status card from
// `ryoku-hub desktop plugins list`: where the copy came from, its version, whether
// it matches the running Hyprland (a plugin is ABI-locked to the exact
// compositor build, so an Arch bump can leave a copy that no longer loads), and
// the actions that fix it: Rebuild here from upstream, Docs, Remove. "Add from
// git" clones any repository with a hyprpm.toml, builds the plugin the user
// picks, and lays it beside the bundled ones; its settings are the config keys
// detected in the .so, typed by the compositor once it loads. Settings ride the
// hypr draft like every Hyprland page; Save writes settings.lua and reloads.
Item {
    id: pg
    property var hub

    readonly property string pTitle: I18n.tr("Plugins")
    readonly property string pEyebrow: I18n.tr("COMPOSITOR")
    readonly property string pBlurb: I18n.tr("Compositor plugins: what runs here, their settings, and new ones.")

    function hv(path) { return pg.hub ? pg.hub.hyprVal(path) : undefined }
    function cv(path) { return pg.hub ? pg.hub.hyprCommittedVal(path) : undefined }

    // ── the roster ───────────────────────────────────────────────────────────
    property var roster: ({ compositor: {}, toolchain: { ok: true, missing: [] }, plugins: [] })
    property bool loaded: false
    // the action in flight: "rebuild:<id>", "rebuild:all", "inspect", "add", "remove:<id>"
    property string busy: ""
    property var log: []
    property string notice: ""
    property bool addOpen: false
    property string addUrl: ""
    property var inspected: null
    property var picked: ({})

    readonly property var extras: (pg.roster.plugins || []).filter(function (p) { return !p.bundled; })
    readonly property var stale: (pg.roster.plugins || []).filter(function (p) { return p.status === "stale" && p.rebuildable; })
    readonly property int runningCount: (pg.roster.plugins || []).filter(function (p) { return p.loaded; }).length
    readonly property bool canBuild: pg.roster.toolchain ? pg.roster.toolchain.ok === true : false

    function refresh() { listProc.running = true }
    function infoFor(id) {
        var ps = pg.roster.plugins || [];
        for (var i = 0; i < ps.length; i++) if (ps[i].id === id) return ps[i];
        return null;
    }
    // the plugin a tab shows: bundled tabs come from the schema, an added
    // plugin's tab is its name
    function idForTab(tab) {
        for (var i = 0; i < Schema.plugins.length; i++) if (Schema.plugins[i].tab === tab) return Schema.plugins[i].id;
        for (var j = 0; j < pg.extras.length; j++) if (pg.extras[j].name === tab) return pg.extras[j].id;
        return "";
    }
    readonly property var current: pg.infoFor(pg.idForTab(sp.tab))
    // a plugin just added: its tab opens once the roster carries it
    property string pendingTab: ""

    // ── rows ─────────────────────────────────────────────────────────────────
    function humanize(s) {
        var t = s.replace(/[_-]+/g, " ").trim();
        return t.charAt(0).toUpperCase() + t.slice(1);
    }
    // an added plugin's rows: its Enabled switch, then one control per config
    // key the backend detected, typed by the compositor (a switch for a bool, a
    // stepper for an int, a slider for a float, a field for anything else)
    readonly property var extraRows: {
        var out = [];
        for (var i = 0; i < pg.extras.length; i++) {
            var p = pg.extras[i], base = "wm.hyprland.plugins.extra." + p.id;
            out.push({ tab: p.name, group: I18n.tr("PLUGIN"), key: base + ".enabled", label: I18n.tr("Enabled"),
                       desc: I18n.tr("Loads %1 into the compositor, applies on Save").arg(p.name), ctl: "sw", src: "desktop.json" });
            var ss = p.settings || [];
            for (var j = 0; j < ss.length; j++) {
                var s = ss[j], parts = s.key.split(":");
                var row = { tab: p.name, key: base + ".config." + s.key, label: pg.humanize(parts[parts.length - 1]),
                            group: parts.length > 2 ? parts.slice(1, -1).join(" \u00b7 ").toUpperCase() : I18n.tr("SETTINGS"),
                            desc: "plugin:" + s.key + (s.type ? ", " + s.type : "") + (s.default !== undefined && s.default !== null ? ", default " + s.default : ""),
                            src: "desktop.json", when: {} };
                row.when[base + ".enabled"] = [true];
                var d = Number(s.default);
                switch (s.type) {
                case "bool": row.ctl = "sw"; break;
                case "int": row.ctl = "step"; row.lo = Math.min(0, isNaN(d) ? 0 : d); row.hi = Math.max(100, isNaN(d) ? 0 : d * 4); break;
                case "float":
                    row.ctl = "slid";
                    if (!isNaN(d) && d >= 0 && d <= 1) { row.lo = 0; row.hi = 1; row.pct = true; }
                    else { row.lo = 0; row.hi = Math.max(10, isNaN(d) ? 0 : d * 4); }
                    break;
                default: row.ctl = "text";
                }
                out.push(row);
            }
        }
        return out;
    }
    // what a detected key holds until the user sets it: the compositor's default
    readonly property var extraDefaults: {
        var m = {};
        for (var i = 0; i < pg.extras.length; i++) {
            var p = pg.extras[i], ss = p.settings || [];
            for (var j = 0; j < ss.length; j++)
                if (ss[j].default !== undefined && ss[j].default !== null) m["wm.hyprland.plugins.extra." + p.id + ".config." + ss[j].key] = ss[j].default;
        }
        return m;
    }
    readonly property var settingsSchema: {
        ProviderSchema.revision;
        return ProviderSchema.rowsFor("plugins").concat(pg.extraRows);
    }

    // draft/committed are flat maps off the hypr store (dotted keys), the shape
    // the settings sheet reads. draft depends on hyprVal, so an edit rebuilds it.
    readonly property var draft: {
        var d = {};
        if (pg.hub)
            for (var i = 0; i < pg.settingsSchema.length; i++) {
                var k = pg.settingsSchema[i].key;
                if (!k) continue;
                var v = pg.hv(k);
                d[k] = v === undefined ? pg.extraDefaults[k] : v;
            }
        return d;
    }
    readonly property var committed: {
        var d = {};
        if (pg.hub)
            for (var i = 0; i < pg.settingsSchema.length; i++) {
                var k = pg.settingsSchema[i].key;
                if (!k) continue;
                var v = pg.cv(k);
                d[k] = v === undefined ? pg.extraDefaults[k] : v;
            }
        return d;
    }

    // ── actions ──────────────────────────────────────────────────────────────
    function rebuild(id) {
        if (pg.busy !== "" || !pg.canBuild) return;
        pg.busy = "rebuild:" + id;
        pg.log = [];
        pg.notice = "";
        actProc.command = id === "all"
            ? ["ryoku-hub", "desktop", "plugins", "rebuild", "--stale"]
            : ["ryoku-hub", "desktop", "plugins", "rebuild", id];
        actProc.running = true;
    }
    function inspect() {
        var url = pg.addUrl.trim();
        if (pg.busy !== "" || url === "" || !pg.canBuild) return;
        pg.busy = "inspect";
        pg.log = [];
        pg.notice = "";
        pg.inspected = null;
        pg.picked = ({});
        actProc.command = ["ryoku-hub", "desktop", "plugins", "add", "--inspect", url];
        actProc.running = true;
    }
    function addPicked() {
        var ids = [];
        for (var k in pg.picked) if (pg.picked[k]) ids.push(k);
        if (pg.busy !== "" || !pg.inspected || ids.length === 0) return;
        pg.busy = "add";
        pg.log = [];
        pg.notice = "";
        actProc.command = ["ryoku-hub", "desktop", "plugins", "add", pg.inspected.repo].concat(ids);
        actProc.running = true;
    }
    function remove(id) {
        if (pg.busy !== "") return;
        pg.busy = "remove:" + id;
        pg.log = [];
        pg.notice = "";
        actProc.command = ["ryoku-hub", "desktop", "plugins", "remove", id];
        actProc.running = true;
    }
    function openDocs(url) { if (url) Spawn.runApp(["xdg-open", url], { label: "browser" }); }

    // the backend's verdict on an action, from its JSON on stdout
    function finish(code, out) {
        var kind = pg.busy;
        pg.busy = "";
        var r = null;
        try { r = JSON.parse(out); } catch (e) {}
        if (code !== 0 || !r) {
            pg.notice = pg.log.length ? pg.log[pg.log.length - 1] : I18n.tr("The action failed; see the log.");
            pg.refresh();
            return;
        }
        if (kind === "inspect") {
            pg.inspected = r;
            var pk = {};
            for (var i = 0; i < r.plugins.length; i++)
                if (!r.plugins[i].bundled && (r.plugins.length === 1 || !r.plugins[i].installed)) pk[r.plugins[i].id] = r.plugins.length === 1;
            pg.picked = pk;
            pg.log = [];
            pg.notice = r.plugins.length === 0 ? I18n.tr("No plugin declared in that repository.") : "";
            return;
        }
        if (kind.indexOf("remove:") === 0) {
            if (pg.hub) pg.hub.hyprDrop("wm.hyprland.plugins.extra." + r.removed);
            pg.notice = I18n.tr("Removed %1").arg(r.removed);
            pg.refresh();
            return;
        }
        // rebuild / add
        var built = r.built || [], failed = Object.keys(r.failed || {});
        var parts = [];
        if (built.length) parts.push(I18n.tr("Built: %1").arg(built.join(", ")));
        if (r.skipped && r.skipped.length) parts.push(I18n.tr("Up to date: %1").arg(r.skipped.length));
        for (var f = 0; f < failed.length; f++) parts.push(failed[f] + ": " + r.failed[failed[f]]);
        pg.notice = parts.join("  \u00b7  ");
        if (kind === "add" && pg.hub) {
            // a plugin just added is one the user wants running: enable it in the
            // draft, and Save loads it (the roster then types its settings)
            for (var b = 0; b < built.length; b++) pg.hub.hyprEdit("wm.hyprland.plugins.extra." + built[b] + ".enabled", true);
            if (built.length) { pg.addOpen = false; pg.inspected = null; pg.addUrl = ""; pg.pendingTab = built[0]; }
        }
        pg.refresh();
    }

    Process {
        id: listProc
        command: ["ryoku-hub", "desktop", "plugins", "list"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try { pg.roster = JSON.parse(this.text); pg.loaded = true; }
                catch (e) { console.log("plugins: list parse failed: " + e); }
                pg.settleTab();
            }
        }
    }
    Process {
        id: actProc
        stdout: StdioCollector { id: actOut }
        stderr: SplitParser { onRead: (line) => { if (line.trim() !== "") pg.log = pg.log.concat([line]); } }
        onExited: (code) => pg.finish(code, actOut.text)
    }
    // a Save reloads the compositor, which loads what the draft turned on: read
    // the roster back once that has settled
    Connections {
        target: pg.hub
        function onSavePage() { afterSave.restart(); }
    }
    Timer { id: afterSave; interval: 1500; onTriggered: pg.refresh() }
    // the open tab follows the roster: a plugin just added opens, a plugin just
    // removed hands over to the first tab instead of leaving an empty sheet
    function settleTab() {
        var tabs = sp.tabs;
        if (pg.pendingTab !== "") {
            var p = pg.infoFor(pg.pendingTab);
            if (p && tabs.indexOf(p.name) >= 0) { sp.tab = p.name; pg.pendingTab = ""; }
        }
        if (tabs.length && tabs.indexOf(sp.tab) < 0) sp.tab = tabs[0];
    }

    function statusLabel(p) {
        if (!p) return "";
        switch (p.status) {
        case "running": return I18n.tr("RUNNING");
        case "ready": return p.enabled ? I18n.tr("ON AT START") : I18n.tr("READY");
        case "stale": return I18n.tr("NEEDS REBUILD");
        case "failed": return I18n.tr("FAILED TO LOAD");
        case "unreachable": return I18n.tr("COMPOSITOR OFFLINE");
        case "missing": return I18n.tr("NOT INSTALLED");
        }
        return "";
    }
    function sourceLabel(p) {
        if (!p || !p.installed) return "";
        switch (p.source) {
        case "package": return p.package ? p.package + " " + p.version : p.version;
        case "built": return I18n.tr("built here %1").arg(p.version);
        case "hyprpm": return I18n.tr("hyprpm");
        }
        return p.version;
    }

    function focusKey(k) { sp.focusKey(k) }
    SchemaPage {
        id: sp
        anchors.fill: parent
        schema: pg.settingsSchema
        draft: pg.draft
        defaults: pg.committed
        advanced: pg.hub ? pg.hub.advanced : false
        title: pg.pTitle
        eyebrow: pg.pEyebrow
        blurb: pg.pBlurb
        query: pg.hub ? pg.hub.query : ""
        onEdited: (k, v) => { if (pg.hub) pg.hub.hyprEdit(k, v); }
        onPickRequested: (r) => { if (pg.hub) pg.hub.openPick(r); }

        // ── above the settings: the compositor line, the plugin's card, add ──
        Column {
            width: parent.width
            spacing: Tokens.s3

            // the compositor line: what every plugin here is locked to
            Item {
                width: parent.width
                height: 28
                Row {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s2
                    Rectangle { width: 4; height: 4; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("HYPRLAND") + " " + (pg.roster.compositor.version || "?")
                              + (pg.roster.compositor.commit ? "  \u00b7  " + String(pg.roster.compositor.commit).slice(0, 7) : "")
                              + "  \u00b7  " + (pg.runningCount === 1 ? I18n.tr("%1 PLUGIN RUNNING").arg(pg.runningCount) : I18n.tr("%1 PLUGINS RUNNING").arg(pg.runningCount))
                        color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                        font.letterSpacing: 0.4
                    }
                }
                Row {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s2
                    Btn {
                        visible: pg.stale.length > 0
                        compact: true
                        armed: pg.busy === "" && pg.canBuild
                        text: I18n.tr("REBUILD STALE") + " (" + pg.stale.length + ")"
                        onAct: pg.rebuild("all")
                    }
                    Btn {
                        compact: true
                        primary: pg.addOpen
                        armed: pg.busy === ""
                        text: pg.addOpen ? I18n.tr("CLOSE") : I18n.tr("+ ADD FROM GIT")
                        onAct: pg.addOpen = !pg.addOpen
                    }
                    IconBtn {
                        anchors.verticalCenter: parent.verticalCenter
                        glyph: "\u21bb"
                        armed: pg.busy === ""
                        onAct: pg.refresh()
                    }
                }
            }

            // no toolchain: say what is missing once, above everything
            Text {
                visible: pg.loaded && !pg.canBuild
                width: parent.width
                wrapMode: Text.WordWrap
                text: I18n.tr("Plugins cannot be built on this machine. Missing build tools: %1.")
                    .arg(((pg.roster.toolchain && pg.roster.toolchain.missing) || []).join(", "))
                color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
            }

            // the card for the plugin whose tab is open
            Rectangle {
                id: card
                readonly property var p: pg.current
                width: parent.width
                visible: card.p !== null
                height: visible ? cardCol.implicitHeight + Tokens.s4 * 2 : 0
                radius: Tokens.radius
                color: "transparent"
                border.width: Tokens.border
                border.color: Tokens.line

                Column {
                    id: cardCol
                    anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
                    spacing: Tokens.s2

                    Item {
                        width: parent.width
                        height: 24
                        Row {
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s3
                            Rectangle {
                                anchors.verticalCenter: parent.verticalCenter
                                width: 8; height: 8; radius: 4
                                color: !card.p ? Tokens.inkFaint
                                     : card.p.status === "running" ? Tokens.sun
                                     : (card.p.status === "stale" || card.p.status === "failed" || card.p.status === "unreachable") ? Tokens.alert
                                     : card.p.status === "missing" ? Tokens.inkFaint : Tokens.inkMuted
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: card.p ? card.p.name : ""
                                color: Tokens.ink; font.family: Tokens.ui
                                font.pixelSize: Tokens.fRow; font.weight: Font.Medium
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: card.p ? card.p.id : ""
                                color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                            }
                        }
                        // the status pill: running inverts, trouble reads in the alert red
                        Rectangle {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            readonly property bool on: card.p && card.p.status === "running"
                            readonly property bool bad: card.p && (card.p.status === "stale" || card.p.status === "failed" || card.p.status === "unreachable")
                            width: pill.implicitWidth + 16
                            height: 20
                            radius: Tokens.radius
                            color: on ? Tokens.bone : "transparent"
                            border.width: on ? 0 : Tokens.border
                            border.color: bad ? Tokens.alert : Tokens.line
                            Text {
                                id: pill
                                anchors.centerIn: parent
                                text: pg.statusLabel(card.p)
                                color: parent.on ? Tokens.inkOnBone : (parent.bad ? Tokens.alert : Tokens.inkFaint)
                                font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                                font.weight: Font.Medium; font.letterSpacing: 0.6
                            }
                        }
                    }

                    Text {
                        width: parent.width
                        text: card.p ? I18n.tr(card.p.desc) : ""
                        visible: text !== ""
                        wrapMode: Text.WordWrap
                        color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    }
                    Text {
                        width: parent.width
                        text: card.p ? card.p.detail : ""
                        wrapMode: Text.WordWrap
                        color: card.p && (card.p.status === "stale" || card.p.status === "failed" || card.p.status === "unreachable") ? Tokens.alert : Tokens.ink
                        font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    }
                    // provenance is file-truth, so mono
                    Text {
                        width: parent.width
                        visible: card.p && card.p.installed
                        text: card.p ? (pg.sourceLabel(card.p)
                              + (card.p.builtAt ? "  \u00b7  " + String(card.p.builtAt).slice(0, 10) : "")
                              + "  \u00b7  " + (pg.hub ? pg.hub.tildePath(card.p.path) : card.p.path)) : ""
                        elide: Text.ElideMiddle
                        color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                    }

                    Row {
                        spacing: Tokens.s2
                        Btn {
                            visible: card.p && card.p.docs !== ""
                            compact: true
                            text: I18n.tr("DOCS")
                            onAct: pg.openDocs(card.p.docs)
                        }
                        Btn {
                            visible: card.p && !!card.p.sounds
                            compact: true
                            text: I18n.tr("SOUNDS")
                            onAct: pg.openDocs(card.p.sounds)
                        }
                        Btn {
                            visible: card.p && !!card.p.userSounds
                            compact: true
                            text: I18n.tr("MY SOUNDS")
                            onAct: pg.openDocs(card.p.userSounds)
                        }
                        Btn {
                            visible: card.p && !!card.p.shippedSounds
                            compact: true
                            text: I18n.tr("SHIPPED")
                            onAct: pg.openDocs(card.p.shippedSounds)
                        }
                        Btn {
                            visible: card.p && card.p.rebuildable
                            compact: true
                            primary: card.p && (card.p.status === "stale" || card.p.status === "missing")
                            armed: pg.busy === "" && pg.canBuild
                            text: pg.busy === ("rebuild:" + (card.p ? card.p.id : "")) ? I18n.tr("BUILDING\u2026")
                                : (card.p && card.p.status === "missing" ? I18n.tr("BUILD HERE") : I18n.tr("REBUILD"))
                            onAct: pg.rebuild(card.p.id)
                        }
                        Btn {
                            visible: card.p && card.p.removable && card.p.source === "built"
                            compact: true
                            armed: pg.busy === ""
                            text: I18n.tr("REMOVE")
                            onAct: pg.remove(card.p.id)
                        }
                    }
                }
            }

            // add from git: paste a repository, see what it offers, pick, build
            Rectangle {
                visible: pg.addOpen
                width: parent.width
                height: visible ? addCol.implicitHeight + Tokens.s4 * 2 : 0
                radius: Tokens.radius
                color: Tokens.tint5
                border.width: Tokens.border
                border.color: Tokens.line

                Column {
                    id: addCol
                    anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
                    spacing: Tokens.s3

                    Text {
                        width: parent.width
                        wrapMode: Text.WordWrap
                        text: I18n.tr("A Hyprland plugin repository (one with a hyprpm.toml). It is cloned and built on this machine against your Hyprland, then loaded into the compositor: add only repositories you trust.")
                        color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    }
                    Row {
                        width: parent.width
                        spacing: Tokens.s2
                        Field {
                            id: urlField
                            width: parent.width - inspectBtn.width - Tokens.s2
                            placeholder: "https://github.com/hyprwm/hyprland-plugins"
                            tabular: true
                            text: pg.addUrl
                            onEdited: (v) => pg.addUrl = v
                            onAccepted: pg.inspect()
                        }
                        Btn {
                            id: inspectBtn
                            anchors.verticalCenter: parent.verticalCenter
                            armed: pg.busy === "" && pg.canBuild && pg.addUrl.trim() !== ""
                            text: pg.busy === "inspect" ? I18n.tr("FETCHING\u2026") : I18n.tr("LOOK UP")
                            onAct: pg.inspect()
                        }
                    }
                    // what the repository offers; already-bundled plugins are named, not offered
                    Column {
                        visible: pg.inspected !== null
                        width: parent.width
                        spacing: Tokens.s1
                        Repeater {
                            model: pg.inspected ? pg.inspected.plugins : []
                            delegate: Item {
                                id: offer
                                required property var modelData
                                width: parent.width
                                height: 30
                                readonly property bool on: pg.picked[offer.modelData.id] === true
                                Row {
                                    anchors.left: parent.left
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: Tokens.s3
                                    Sw {
                                        anchors.verticalCenter: parent.verticalCenter
                                        on: offer.on
                                        enabled: !offer.modelData.bundled
                                        onToggled: (v) => { var p = Object.assign({}, pg.picked); p[offer.modelData.id] = v; pg.picked = p; }
                                    }
                                    Text {
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: offer.modelData.id
                                        color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fSmall
                                    }
                                    Text {
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: offer.modelData.bundled ? I18n.tr("bundled with Ryoku") : (offer.modelData.installed ? I18n.tr("installed") : (offer.modelData.desc || ""))
                                        color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                        elide: Text.ElideRight
                                    }
                                }
                            }
                        }
                        Btn {
                            primary: true
                            armed: pg.busy === "" && Object.keys(pg.picked).some(function (k) { return pg.picked[k]; })
                            text: pg.busy === "add" ? I18n.tr("BUILDING\u2026") : I18n.tr("BUILD AND ADD")
                            onAct: pg.addPicked()
                        }
                    }
                }
            }

            // the last action's verdict and, while something builds, its log
            Text {
                visible: pg.notice !== ""
                width: parent.width
                wrapMode: Text.WordWrap
                text: pg.notice
                color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
            }
            Rectangle {
                visible: pg.busy !== "" || (pg.log.length > 0 && pg.notice === "")
                width: parent.width
                height: visible ? logText.implicitHeight + Tokens.s3 * 2 : 0
                radius: Tokens.radius
                color: Tokens.paperLift
                border.width: Tokens.border
                border.color: Tokens.lineSoft
                Text {
                    id: logText
                    anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s3 }
                    text: pg.log.length ? pg.log.slice(-8).join("\n") : I18n.tr("Working\u2026")
                    wrapMode: Text.WrapAnywhere
                    color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                }
            }
        }
    }
}

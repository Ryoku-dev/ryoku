pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Installed add-on management. RyoStore owns discovery and installation;
// Settings keeps plugin enablement, placement, live options, updates/removal,
// and bundle component removal. The page is full-bleed because every action
// applies immediately instead of participating in the shared Save/Revert flow.
Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true

    // ── installed-management state ──────────────────────────────────────────
    property var plugins: []        // discover.sh --all: [{ id, dir, manifest, placement }]
    property var catalog: []         // normalized RyoStore plugin items, for update detection
    property string selId: ""        // "" = master list; else the open plugin's id
    property string busyId: ""        // id currently installing/removing
    property bool confirmRemove: false // the destructive-confirm plate is up
    property bool loaded: false        // discover.sh has answered at least once
    property string tab: "Plugins"
    property var bundles: []
    property bool bundlesLoaded: false

    // hub.query carries the rail search text; empty means no filter.
    readonly property string query: (pg.hub && pg.hub.query) ? ("" + pg.hub.query) : ""

    // the selected plugin, re-derived from selId so it stays fresh across a
    // refresh (the process round-trip swaps the whole plugins array).
    readonly property var sel: {
        for (var i = 0; i < pg.plugins.length; i++)
            if (pg.plugins[i].id === pg.selId)
                return pg.plugins[i];
        return null;
    }

    // the master list, filtered by the rail search.
    readonly property var shown: {
        var q = pg.query.trim().toLowerCase();
        if (q === "")
            return pg.plugins;
        return pg.plugins.filter(function (p) {
            var m = p.manifest || {};
            var name = ("" + (m.name || p.id || "")).toLowerCase();
            return name.indexOf(q) >= 0 || ("" + p.id).toLowerCase().indexOf(q) >= 0;
        });
    }
    readonly property var shownBundles: {
        const q = pg.query.trim().toLowerCase();
        if (q === "")
            return pg.bundles;
        return pg.bundles.filter(bundle => {
            const parts = (bundle.metadata || {}).items || [];
            const hay = [bundle.id, bundle.name, bundle.summary, bundle.description]
                .concat(parts.map(part => part.name + " " + (part.summary || "")))
                .join(" ").toLowerCase();
            return hay.indexOf(q) !== -1;
        });
    }

    readonly property string shellDir: Quickshell.env("RYOKU_SHELL_DIR")
    readonly property string script: (pg.shellDir && pg.shellDir.length > 0)
        ? pg.shellDir + "/quickshell/plugins/discover.sh"
        : (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/quickshell/plugins/discover.sh"

    // Plugin host id -> readable label, and back.
    function hostLabel(key) {
        return key === "framePopout" ? I18n.tr("Frame popout")
            : key === "desktopWidget" ? I18n.tr("Desktop widget")
            : key === "topbarGlyph" ? I18n.tr("Bar")
            : key === "sidebarCard" ? I18n.tr("Sidebar card") : key;
    }
    function hostKey(label) {
        return label === I18n.tr("Frame popout") ? "framePopout"
            : label === I18n.tr("Desktop widget") ? "desktopWidget"
            : label === I18n.tr("Bar") ? "topbarGlyph"
            : label === I18n.tr("Sidebar card") ? "sidebarCard" : label;
    }

    function refresh() { listProc.running = false; listProc.running = true; }
    function loadCatalog() { catProc.running = false; catProc.running = true; }
    function loadBundles() { bundleProc.running = false; bundleProc.running = true; }
    function browseStore() {
        Quickshell.execDetached(["ryostore", "open", pg.tab === "Bundles" ? "bundles" : "plugins"]);
    }
    function removeBundle(id, item) {
        const scope = item ? ["item", id, item] : ["bundle", id];
        Spawn.run(["kitty", "--class", "ryostore", "-e", "ryostore-install", "remove"].concat(scope));
    }
    function catalogEntry(id) {
        for (var i = 0; i < pg.catalog.length; i++)
            if (pg.catalog[i].id === id)
                return pg.catalog[i];
        return null;
    }
    // semver compare: 1 if a>b, 0 equal, -1 if a<b. missing parts = 0.
    function cmpSemver(a, b) {
        var pa = String(a || "0").split(".").map(function (n) { return parseInt(n, 10) || 0; });
        var pb = String(b || "0").split(".").map(function (n) { return parseInt(n, 10) || 0; });
        for (var i = 0; i < Math.max(pa.length, pb.length); i++) {
            var x = pa[i] || 0, y = pb[i] || 0;
            if (x !== y)
                return x < y ? -1 : 1;
        }
        return 0;
    }
    // an installed plugin -> the newer catalogue version, or "" when up to date
    // / unknown. Drives the Update marker and the detail's Update button.
    function updateFor(pl) {
        var inst = (pl && pl.manifest && pl.manifest.version) ? pl.manifest.version : "";
        var ce = pg.catalogEntry(pl ? pl.id : "");
        var avail = ce ? (ce.version || "") : "";
        if (!inst || !avail)
            return "";
        return pg.cmpSemver(avail, inst) > 0 ? avail : "";
    }
    function install(id) {
        if (!id)
            return;
        pg.busyId = id;
        installProc.command = ["ryostore", "internal", "install-guest", "plugins", id];
        installProc.running = true;
    }
    function place(id, field, a, b, c, d) {
        if (!id)
            return;
        var args = ["ryoku-plugins-place", id, field];
        for (var v of [a, b, c, d])
            if (v !== undefined)
                args.push("" + v);
        placeProc.command = args;
        placeProc.running = true;
    }
    function setSetting(id, key, value) {
        if (!id)
            return;
        var obj = {};
        obj[key] = value;
        settingsProc.command = ["ryoku-plugins-place", id, "settings", JSON.stringify(obj)];
        settingsProc.running = true;
    }
    // remove drops the whole plugins.json entry (forget) then deletes the
    // data-dir plugin via the symlink-safe backend (a dev plugin is a symlink
    // into the checkout; rm -rf would gut the repo).
    function removePlugin(id) {
        if (!id)
            return;
        pg.busyId = id;
        pg.place(id, "forget");
        rmProc.command = ["ryostore", "internal", "remove-guest", "plugins", id];
        rmProc.running = true;
    }

    Component.onCompleted: { pg.refresh(); pg.loadCatalog(); pg.loadBundles(); }

    Process {
        id: listProc
        command: ["bash", pg.script, "--all"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try { pg.plugins = JSON.parse(text || "[]"); } catch (e) { pg.plugins = []; }
                pg.loaded = true;
            }
        }
    }
    Process { id: placeProc; onExited: pg.refresh() }
    Process { id: settingsProc; onExited: pg.refresh() }
    Process { id: rmProc; onExited: { pg.busyId = ""; pg.selId = ""; pg.confirmRemove = false; pg.refresh(); } }
    Process {
        id: catProc
        command: ["ryostore", "catalog", "--category", "plugins"]
        stdout: StdioCollector {
            onStreamFinished: {
                try { pg.catalog = (JSON.parse(text || "{}").items) || []; }
                catch (e) { pg.catalog = []; }
            }
        }
    }
    Process { id: installProc; onExited: { pg.busyId = ""; pg.refresh(); pg.loadCatalog(); } }
    Process {
        id: bundleProc
        command: ["ryostore", "catalog", "--category", "bundles"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const items = (JSON.parse(text || "{}").items) || [];
                    pg.bundles = items.filter(item =>
                        item.category === "bundles" && Number(item.installedCount || 0) > 0);
                } catch (e) {
                    pg.bundles = [];
                }
                pg.bundlesLoaded = true;
            }
        }
    }

    // ── head: eyebrow, Fraunces title, blurb (matches every page) ───────────
    Column {
        id: head
        anchors.top: parent.top
        anchors.topMargin: Tokens.s6
        // the head sits on the body's grid: left-inset and body-wide, so the
        // title starts over the first card column instead of floating centred
        x: Tokens.s6
        width: Math.max(320, pg.width - Tokens.s6 * 2 - Tokens.s3)
        // the register row sits off the title: a rule over a 32px
        // title needs more than the gap between two lines of body text
        spacing: Tokens.s3

        Row {
            // the register row holds a fixed box, so the rule and the seal keep
            // their distance from the title on every page
            height: Tokens.s5
            spacing: Tokens.s2
            Rectangle {
                width: 16; height: 1; color: Tokens.ink
                anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: "力"; color: Tokens.ink; font.family: Tokens.jp
                font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: I18n.tr("ADD-ONS"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Item {
            width: parent.width
            height: title.implicitHeight
            Text {
                id: title
                anchors.left: parent.left
                text: I18n.tr("Add-ons")
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Tokens.fTitle
            }
            Btn {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("BROWSE RYOSTORE")
                onAct: pg.browseStore()
            }
        }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("Installed plugins and bundles. RyoStore installs new ones.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
    }

    Tabs {
        id: tabs
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: head.bottom
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s4
        options: ["Plugins", "Bundles"]
        current: pg.tab
        onChose: label => { pg.tab = label; pg.selId = ""; }
    }

    // ── the body: a Loader swaps master (list) and detail (one plugin) ──────
    Loader {
        id: body
        anchors {
            left: parent.left; right: parent.right
            top: tabs.bottom; bottom: parent.bottom
            leftMargin: Tokens.s6; rightMargin: Tokens.s6
            topMargin: Tokens.s5; bottomMargin: Tokens.s6
        }
        sourceComponent: pg.tab === "Bundles" ? bundleComp : (pg.selId === "" ? masterComp : detailComp)
        onLoaded: {
            if (!item)
                return;
            item.opacity = 0;
            fade.restart();
        }
    }
    // content exchange -> swap token; nothing travels, so a plain fade is right.
    NumberAnimation {
        id: fade
        target: body.item; property: "opacity"; to: 1
        duration: Tokens.swap; easing.type: Tokens.ease
    }

    // ── master: the installed list ──────────────────────────────────────────
    Component {
        id: masterComp

        Item {
            id: master

            // section head: dot + PLUGINS + leader + count + refresh.
            Item {
                id: sect
                anchors { left: parent.left; right: parent.right; top: parent.top }
                height: 32

                Row {
                    id: sectLabel
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s2
                    Rectangle {
                        width: 4; height: 4; color: Tokens.ink
                        anchors.verticalCenter: parent.verticalCenter
                    }
                    Text {
                        text: I18n.tr("PLUGINS"); color: Tokens.ink; font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackMark
                        anchors.verticalCenter: parent.verticalCenter
                    }
                }

                Row {
                    id: sectActions
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s3

                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        // an entry count is file-truth chrome, so mono.
                        text: (pg.plugins.length === 1 ? I18n.tr("%1 PLUGIN") : I18n.tr("%1 PLUGINS")).arg(pg.plugins.length)
                        color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                    }
                    // Re-scan installed plugins after external RyoStore changes.
                    IconBtn {
                        anchors.verticalCenter: parent.verticalCenter
                        glyph: "\u21bb"
                        onAct: pg.refresh()
                    }
                }

                Rectangle {
                    anchors.left: sectLabel.right; anchors.right: sectActions.left
                    anchors.leftMargin: Tokens.s3; anchors.rightMargin: Tokens.s3
                    anchors.verticalCenter: parent.verticalCenter
                    height: 1; color: Tokens.lineSoft
                }
            }

            Flickable {
                id: flick
                anchors {
                    left: parent.left; right: parent.right
                    top: sect.bottom; bottom: parent.bottom
                    topMargin: Tokens.s4; bottomMargin: Tokens.s4
                }
                contentWidth: width
                contentHeight: Math.max(col.height, height)
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                WheelScroll { }

                CardColumns {

                id: col
            // a body of cards fills the measure and splits into balanced columns
            width: flick.width - Tokens.s3
                    spacing: Tokens.s2

                    Repeater {
                        model: pg.shown

                        delegate: Rectangle {
                            id: card
                            required property var modelData
                            readonly property var man: card.modelData.manifest || ({})
                            readonly property var place: card.modelData.placement || ({})
                            readonly property bool on: card.place.enabled === true
                            readonly property string host: (card.place.host)
                                ? card.place.host
                                : ((card.man.defaults && card.man.defaults.host) ? card.man.defaults.host : "framePopout")
                            readonly property int settingsCount: (card.man.metadata && card.man.metadata.settings)
                                ? card.man.metadata.settings.length : 0
                            readonly property string upd: pg.updateFor(card.modelData)

                            width: col.colWidth
                            height: 64
                            radius: Tokens.radius
                            color: ch.hovered ? Tokens.tint5 : "transparent"
                            border.width: Tokens.border
                            border.color: ch.hovered ? Tokens.lineStrong : Tokens.line
                            Behavior on color { ColorAnimation { duration: Tokens.snap } }
                            Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

                            HoverHandler { id: ch; cursorShape: Qt.PointingHandCursor }
                            TapHandler { onTapped: pg.selId = card.modelData.id }

                            // name + meta.
                            Column {
                                anchors.left: parent.left; anchors.leftMargin: Tokens.s4
                                anchors.right: right.left; anchors.rightMargin: Tokens.s3
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s1

                                Text {
                                    width: parent.width
                                    text: card.man.name || card.modelData.id
                                    color: Tokens.ink; font.family: Tokens.ui
                                    font.pixelSize: Tokens.fRow; font.weight: Font.Medium
                                    elide: Text.ElideRight
                                }
                                Text {
                                    width: parent.width
                                    text: pg.hostLabel(card.host)
                                        + (card.settingsCount > 0 ? "  ·  " + (card.settingsCount === 1 ? I18n.tr("%1 setting") : I18n.tr("%1 settings")).arg(card.settingsCount) : "")
                                    color: Tokens.inkMuted; font.family: Tokens.ui
                                    font.pixelSize: Tokens.fMicro
                                    elide: Text.ElideRight
                                }
                            }

                            // right cluster: update marker, status chip, caret.
                            Row {
                                id: right
                                anchors.right: parent.right; anchors.rightMargin: Tokens.s4
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s3

                                // a version string is file-truth, so mono. This
                                // only flags availability; the action is in detail.
                                Text {
                                    anchors.verticalCenter: parent.verticalCenter
                                    visible: card.upd !== ""
                                    text: I18n.tr("UPDATE %1").arg(card.upd)
                                    color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                                }

                                // status: enabled inverts (the ON member of a set).
                                Rectangle {
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: pip.implicitWidth + 16
                                    height: 20
                                    radius: Tokens.radius
                                    color: card.on ? Tokens.bone : "transparent"
                                    border.width: card.on ? 0 : Tokens.border
                                    border.color: Tokens.line
                                    Text {
                                        id: pip
                                        anchors.centerIn: parent
                                        text: card.on ? I18n.tr("ON") : I18n.tr("OFF")
                                        color: card.on ? Tokens.inkOnBone : Tokens.inkFaint
                                        font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                                        font.weight: Font.Medium; font.letterSpacing: 0.6
                                    }
                                }

                                Text {
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: "\u203a"
                                    color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                                }
                            }
                        }
                    }
                }
            }

            // empty state, gated on load so it does not flash before data lands.
            Text {
                anchors.centerIn: flick
                visible: pg.loaded && pg.plugins.length === 0
                text: I18n.tr("No add-ons installed. Browse RyoStore to install one.")
                color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
            }
            // no-results state, when a search filters everything out.
            Text {
                anchors.centerIn: flick
                visible: pg.plugins.length > 0 && pg.shown.length === 0
                text: I18n.tr("No add-ons match your search.")
                color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
            }

        }
    }

    Component {
        id: bundleComp

        Item {
            Item {
                id: bundleHead
                anchors { left: parent.left; right: parent.right; top: parent.top }
                height: Tokens.ctlH
                Text {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: (pg.bundles.length === 1 ? I18n.tr("%1 BUNDLE") : I18n.tr("%1 BUNDLES")).arg(pg.bundles.length)
                    color: Tokens.inkFaint
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny
                }
                IconBtn {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    glyph: "\u21bb"
                    onAct: pg.loadBundles()
                }
            }

            Flickable {
                id: bundleFlick
                anchors { left: parent.left; right: parent.right; top: bundleHead.bottom; bottom: parent.bottom; topMargin: Tokens.s3 }
                contentWidth: width
                contentHeight: bundleList.implicitHeight
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                WheelScroll { }

                Column {
                    id: bundleList
                    width: bundleFlick.width - Tokens.s3
                    spacing: Tokens.s3

                    Repeater {
                        model: pg.shownBundles
                        delegate: Rectangle {
                            id: bundleCard
                            required property var modelData
                            readonly property var parts: (modelData.metadata || {}).items || []
                            width: bundleList.width
                            implicitHeight: bundleBody.implicitHeight + Tokens.s4 * 2
                            color: "transparent"
                            radius: Tokens.radius
                            border.width: Tokens.border
                            border.color: Tokens.line

                            Column {
                                id: bundleBody
                                anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
                                spacing: Tokens.s2

                                Row {
                                    width: parent.width
                                    spacing: Tokens.s3
                                    Column {
                                        width: Math.max(0, parent.width - removeAll.width - Tokens.s3)
                                        Text {
                                            width: parent.width
                                            text: bundleCard.modelData.name || bundleCard.modelData.id
                                            color: Tokens.ink
                                            font.family: Tokens.display
                                            font.pixelSize: Tokens.fRow
                                            elide: Text.ElideRight
                                        }
                                        Text {
                                            width: parent.width
                                            text: I18n.tr("%1 / %2 INSTALLED").arg(Number(bundleCard.modelData.installedCount || 0)).arg(Number(bundleCard.modelData.totalCount || bundleCard.parts.length))
                                            color: Tokens.inkMuted
                                            font.family: Tokens.mono
                                            font.pixelSize: Tokens.fTiny
                                        }
                                    }
                                    Btn {
                                        id: removeAll
                                        text: I18n.tr("REMOVE BUNDLE")
                                        onAct: pg.removeBundle(bundleCard.modelData.id, "")
                                    }
                                }

                                Repeater {
                                    model: bundleCard.parts
                                    delegate: Item {
                                        id: partRow
                                        required property var modelData
                                        width: bundleBody.width
                                        height: Tokens.rowH
                                        Text {
                                            anchors.left: parent.left
                                            anchors.verticalCenter: parent.verticalCenter
                                            width: Math.max(0, parent.width - state.width - removeOne.width - Tokens.s5)
                                            text: partRow.modelData.name + (partRow.modelData.summary ? "  ·  " + partRow.modelData.summary : "")
                                            color: Tokens.ink
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fSmall
                                            elide: Text.ElideRight
                                        }
                                        Text {
                                            id: state
                                            anchors.right: removeOne.left
                                            anchors.rightMargin: Tokens.s3
                                            anchors.verticalCenter: parent.verticalCenter
                                            text: partRow.modelData.installed === true ? I18n.tr("INSTALLED") : I18n.tr("ABSENT")
                                            color: partRow.modelData.installed === true ? Tokens.ink : Tokens.inkFaint
                                            font.family: Tokens.mono
                                            font.pixelSize: Tokens.fTiny
                                        }
                                        Btn {
                                            id: removeOne
                                            anchors.right: parent.right
                                            anchors.verticalCenter: parent.verticalCenter
                                            text: I18n.tr("REMOVE")
                                            visible: partRow.modelData.installed === true
                                            onAct: pg.removeBundle(bundleCard.modelData.id, partRow.modelData.name)
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }

            Text {
                anchors.centerIn: parent
                visible: pg.bundlesLoaded && pg.bundles.length === 0
                text: I18n.tr("No bundle components are installed.")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }
        }
    }

    // ── detail: one plugin's placement + settings ───────────────────────────
    Component {
        id: detailComp

        Item {
            id: detail
            property var pendingImageField: null

            readonly property var sel: pg.sel || ({})
            readonly property var man: detail.sel.manifest || ({})
            readonly property var place: detail.sel.placement || ({})
            readonly property var schema: (detail.man.metadata && detail.man.metadata.settings) || []
            readonly property bool enabled: detail.place.enabled === true
            readonly property string host: (detail.place.host)
                ? detail.place.host
                : ((detail.man.defaults && detail.man.defaults.host) ? detail.man.defaults.host : "framePopout")
            readonly property var hosts: (detail.man.hosts || []).filter(function (h) {
                return h === "framePopout" || h === "desktopWidget"
                    || h === "topbarGlyph" || h === "sidebarCard";
            })
            readonly property string upd: pg.updateFor(detail.sel)

            // ── header: back + name + version .... update + remove ──
            Item {
                id: hdr
                anchors { left: parent.left; right: parent.right; top: parent.top }
                height: 40

                IconBtn {
                    id: backBtn
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    glyph: "\u2039"
                    onAct: pg.selId = ""
                }

                Row {
                    id: acts
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s3

                    // Update: only when the catalogue carries a newer version.
                    Btn {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: detail.upd !== ""
                        text: pg.busyId === detail.sel.id ? I18n.tr("UPDATING") : I18n.tr("UPDATE %1").arg(detail.upd)
                        armed: pg.busyId === ""
                        onAct: pg.install(detail.sel.id)
                    }
                    // Remove: destructive, so it arms the confirm plate first.
                    Btn {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("REMOVE")
                        onAct: pg.confirmRemove = true
                    }
                }

                Text {
                    id: nameT
                    anchors.left: backBtn.right; anchors.leftMargin: Tokens.s3
                    anchors.right: verT.left; anchors.rightMargin: Tokens.s2
                    anchors.verticalCenter: parent.verticalCenter
                    text: (detail.man.name) ? detail.man.name : (detail.sel.id || "")
                    color: Tokens.ink; font.family: Tokens.ui
                    font.pixelSize: Tokens.fValue; font.weight: Font.Medium
                    elide: Text.ElideRight
                }
                Text {
                    id: verT
                    anchors.right: acts.left; anchors.rightMargin: Tokens.s4
                    anchors.verticalCenter: parent.verticalCenter
                    visible: text !== ""
                    text: (detail.man.version) ? ("v" + detail.man.version) : ""
                    color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                }
            }

            Flickable {
                id: dflick
                anchors {
                    left: parent.left; right: parent.right
                    top: hdr.bottom; bottom: parent.bottom
                    topMargin: Tokens.s5
                }
                contentWidth: width
                contentHeight: Math.max(dcol.height, height)
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                WheelScroll { }

                Column {
                    id: dcol
                    width: dflick.width - Tokens.s3
                    spacing: Tokens.s5

                    // A plugin Ryoku did not write says so first: the same
                    // warning the Store and QS Bar Settings print.
                    Rectangle {
                        width: parent.width
                        visible: detail.man.official !== true
                        height: visible ? communityRow.implicitHeight + Tokens.s4 * 2 : 0
                        radius: Tokens.radius
                        color: "transparent"
                        border.width: Tokens.border
                        border.color: Tokens.lineStrong
                        Row {
                            id: communityRow
                            anchors {
                                left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter
                                leftMargin: Tokens.s4; rightMargin: Tokens.s4
                            }
                            spacing: Tokens.s3
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: "warning"
                                color: Tokens.inkDim
                                font.family: "Material Symbols Rounded"
                                font.pixelSize: Tokens.fRow
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                width: parent.width - Tokens.fRow - parent.spacing
                                text: I18n.tr("Community plugin. Ryoku does not review or maintain it: it runs inside your shell with your permissions, so inspect its code before you trust it.")
                                color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                wrapMode: Text.WordWrap
                            }
                        }
                    }

                    // ── Placement: enable, host, and where a popout sits ──
                    Column {
                        width: parent.width
                        spacing: Tokens.s3

                        Item {
                            width: parent.width; height: 20
                            Row {
                                id: pHead
                                anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s2
                                Rectangle { width: 4; height: 4; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
                                Text {
                                    text: I18n.tr("PLACEMENT"); color: Tokens.ink; font.family: Tokens.ui
                                    font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                                    font.letterSpacing: Tokens.trackMark
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                            }
                            Rectangle {
                                anchors.left: pHead.right; anchors.leftMargin: Tokens.s3
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                height: 1; color: Tokens.lineSoft
                            }
                        }

                        // Enabled: runs it on the desktop, or keeps it dormant.
                        Item {
                            width: parent.width; height: 30
                            Text {
                                anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("Enabled")
                                color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                            }
                            Sw {
                                anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                on: detail.enabled
                                onToggled: (v) => pg.place(detail.sel.id, "enabled", v ? "true" : "false")
                            }
                        }

                        // Show as: only when the plugin offers more than one home.
                        Item {
                            width: parent.width; height: 30
                            visible: detail.hosts.length > 1
                            Text {
                                anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("Show as")
                                color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                            }
                            Seg {
                                anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                options: detail.hosts.map(function (h) { return pg.hostLabel(h); })
                                current: pg.hostLabel(detail.host)
                                onChose: (label) => pg.place(detail.sel.id, "host", pg.hostKey(label))
                            }
                        }

                        // ── frame-popout placement editor ──
                        // Screen-proportioned stage: drag the chip anywhere on
                        // it. Dropped in the middle third of both axes the
                        // popout is centred (a modal surface); dropped near an
                        // edge it docks there, and the thirds along that edge
                        // pick start | center | end. Edits write live via
                        // ryoku-plugins-place. Desktop widgets do not come
                        // through here (they are placed on the wallpaper), so
                        // this renders for framePopout only.
                        Item {
                            id: placer
                            width: parent.width
                            readonly property real stageH: 202
                            implicitHeight: placer.stageH + 28
                                + (centreNote.visible ? centreNote.height + Tokens.s2 : 0)
                            visible: detail.enabled && detail.host === "framePopout"

                            readonly property var fp: (detail.place && detail.place.framePopout) ? detail.place.framePopout : ({})
                            readonly property string edge: placer.fp.edge ? placer.fp.edge : "right"
                            readonly property string align: placer.fp.align ? placer.fp.align : "start"
                            // live mirrors so a drop snaps instantly, before the
                            // ryoku-plugins-place round-trip refreshes the truth.
                            property string edgeLocal: placer.edge
                            property string alignLocal: placer.align
                            onEdgeChanged: placer.edgeLocal = placer.edge
                            onAlignChanged: placer.alignLocal = placer.align
                            readonly property bool centered: placer.edgeLocal === "center"
                            readonly property bool vertical: placer.edgeLocal === "left" || placer.edgeLocal === "right"
                            function hoverW() { return placer.fp.hoverW ? placer.fp.hoverW : 320; }
                            function hoverH() { return placer.fp.hoverH ? placer.fp.hoverH : 16; }
                            // thirds of an axis: start | center | end.
                            function third(fraction) {
                                return fraction < 1 / 3 ? "start" : fraction < 2 / 3 ? "center" : "end";
                            }
                            function alongPos(span, size) {
                                var m = 10;
                                return placer.alignLocal === "start" ? m
                                     : placer.alignLocal === "end" ? span - size - m
                                     : (span - size) / 2;
                            }
                            function chipX() {
                                var m = 10;
                                return placer.centered ? (stage.width - chip.width) / 2
                                     : placer.edgeLocal === "left" ? m
                                     : placer.edgeLocal === "right" ? stage.width - chip.width - m
                                     : placer.alongPos(stage.width, chip.width);
                            }
                            function chipY() {
                                var m = 10;
                                return placer.centered ? (stage.height - chip.height) / 2
                                     : placer.edgeLocal === "top" ? m
                                     : placer.edgeLocal === "bottom" ? stage.height - chip.height - m
                                     : placer.alongPos(stage.height, chip.height);
                            }
                            // the middle third of both axes is the screen centre;
                            // anywhere else docks to whichever edge is nearest.
                            function dropEdge(fx, fy) {
                                if (placer.third(fx) === "center" && placer.third(fy) === "center")
                                    return "center";
                                var d = { left: fx, right: 1 - fx, top: fy, bottom: 1 - fy };
                                var best = "top";
                                for (var e in d) if (d[e] < d[best]) best = e;
                                return best;
                            }
                            function dropAlign(edge, fx, fy) {
                                return edge === "center" ? "center"
                                     : (edge === "left" || edge === "right") ? placer.third(fy)
                                     : placer.third(fx);
                            }

                            Rectangle {
                                id: stage
                                anchors.horizontalCenter: parent.horizontalCenter
                                anchors.top: parent.top
                                width: Math.min(placer.width, placer.stageH * 1.6)   // ~16:10
                                height: placer.stageH
                                radius: Tokens.radius
                                color: "transparent"   // no gradient; depth is the hairline
                                border.width: Tokens.border
                                border.color: Tokens.line
                                clip: true

                                // faint inner screen frame, so it reads as "your display".
                                Rectangle {
                                    anchors.fill: parent; anchors.margins: 8
                                    radius: Tokens.radius; color: "transparent"
                                    border.width: Tokens.border; border.color: Tokens.lineSoft
                                }

                                Text {
                                    anchors.top: parent.top; anchors.left: parent.left; anchors.margins: 10
                                    text: I18n.tr("LIVE PLACEMENT")
                                    color: Tokens.inkFaint; font.family: Tokens.mono
                                    font.pixelSize: Tokens.fTiny; font.weight: Font.Medium
                                    font.letterSpacing: 2
                                }

                                // the centre drop zone: land the chip here for a
                                // popout that floats in the middle of the screen.
                                Rectangle {
                                    id: centreZone
                                    x: parent.width / 3
                                    y: parent.height / 3
                                    width: parent.width / 3
                                    height: parent.height / 3
                                    radius: Tokens.radius
                                    color: placer.centered ? Tokens.tint5 : "transparent"
                                    Canvas {
                                        anchors.fill: parent
                                        onPaint: {
                                            var ctx = getContext("2d");
                                            ctx.reset();
                                            ctx.strokeStyle = Tokens.inkFaint.toString();
                                            ctx.lineWidth = 1;
                                            ctx.setLineDash([4, 3]);
                                            ctx.strokeRect(0.5, 0.5, width - 1, height - 1);
                                        }
                                        onWidthChanged: requestPaint()
                                        onHeightChanged: requestPaint()
                                    }
                                    Text {
                                        anchors.centerIn: parent
                                        visible: !placer.centered
                                        text: I18n.tr("centre")
                                        color: Tokens.inkFaint; font.family: Tokens.mono
                                        font.pixelSize: Tokens.fTiny; font.letterSpacing: 1.5
                                    }
                                }

                                // the popout body: drag it anywhere on the stage;
                                // where it lands picks the edge and the align.
                                Rectangle {
                                    id: chip
                                    width: placer.vertical ? 64 : 96
                                    height: placer.vertical ? 96 : 60
                                    radius: Tokens.radius
                                    color: Tokens.paperLift
                                    border.width: Tokens.border
                                    border.color: Tokens.ink
                                    x: placer.chipX()
                                    y: placer.chipY()

                                    Text {
                                        anchors.centerIn: parent
                                        text: I18n.tr("popout")
                                        color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                                    }

                                    MouseArea {
                                        anchors.fill: parent
                                        cursorShape: Qt.PointingHandCursor
                                        drag.target: chip
                                        drag.minimumX: 0
                                        drag.maximumX: stage.width - chip.width
                                        drag.minimumY: 0
                                        drag.maximumY: stage.height - chip.height
                                        onReleased: {
                                            var fx = (chip.x + chip.width / 2) / stage.width;
                                            var fy = (chip.y + chip.height / 2) / stage.height;
                                            var e = placer.dropEdge(fx, fy);
                                            var a = placer.dropAlign(e, fx, fy);
                                            placer.edgeLocal = e;                     // snap instantly
                                            placer.alignLocal = a;
                                            chip.x = Qt.binding(function () { return placer.chipX(); });
                                            chip.y = Qt.binding(function () { return placer.chipY(); });
                                            pg.place(detail.sel.id, "framePopout", e, a, placer.hoverW(), placer.hoverH());
                                        }
                                    }
                                }
                            }

                            // edge selector: the four edges plus the screen centre.
                            Seg {
                                id: edgeSeg
                                anchors.horizontalCenter: parent.horizontalCenter
                                anchors.top: stage.bottom
                                anchors.topMargin: Tokens.s2
                                options: ["Center", "Top", "Right", "Bottom", "Left"]
                                current: placer.edgeLocal.charAt(0).toUpperCase() + placer.edgeLocal.slice(1)
                                onChose: (label) => {
                                    var e = label.toLowerCase();
                                    var a = e === "center" ? "center" : placer.alignLocal;
                                    placer.edgeLocal = e;
                                    placer.alignLocal = a;
                                    pg.place(detail.sel.id, "framePopout", e, a, placer.hoverW(), placer.hoverH());
                                }
                            }

                            // a centred popout has no edge to hover, and it shares
                            // the middle of the screen with the shell's own
                            // surfaces. Only one surface is open at a time, so this
                            // is a note, not a restriction.
                            Text {
                                id: centreNote
                                anchors.top: edgeSeg.bottom
                                anchors.topMargin: Tokens.s2
                                anchors.left: parent.left
                                anchors.right: parent.right
                                visible: placer.centered
                                wrapMode: Text.WordWrap
                                text: I18n.tr("A centred popout has no hover edge: open it with its keybind, or with ryoku-shell plugin <id>. It shares the middle of the screen with the sidebars (Super+Escape and Super+S), and the shell shows one surface at a time, so they take turns instead of overlapping.")
                                color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                            }
                        }

                        Item {
                            id: sidebarPlacement
                            width: parent.width
                            visible: detail.enabled && detail.host === "sidebarCard"
                            implicitHeight: visible ? sidebarPlacementCol.implicitHeight : 0

                            readonly property var card: (detail.place && detail.place.sidebarCard)
                                ? detail.place.sidebarCard : ({})
                            readonly property var defaults: (detail.man.defaults && detail.man.defaults.sidebar)
                                ? detail.man.defaults.sidebar : ({})
                            readonly property string side: sidebarPlacement.card.side
                                || sidebarPlacement.defaults.side || "left"
                            readonly property string tabName: sidebarPlacement.card.tab
                                || sidebarPlacement.defaults.tab || "Plugins"
                            readonly property int order: sidebarPlacement.card.order !== undefined
                                ? Number(sidebarPlacement.card.order)
                                : (sidebarPlacement.defaults.order !== undefined
                                    ? Number(sidebarPlacement.defaults.order) : 10)
                            readonly property string label: sidebarPlacement.card.label
                                || ((detail.man.defaults || {}).label) || detail.man.name || detail.sel.id
                            readonly property string glyph: sidebarPlacement.card.glyph
                                || ((detail.man.defaults || {}).glyph) || "extension"

                            function commit(side, tab, order) {
                                pg.place(detail.sel.id, "sidebarCard", side, tab, order,
                                    sidebarPlacement.label, sidebarPlacement.glyph);
                            }

                            Column {
                                id: sidebarPlacementCol
                                width: parent.width
                                spacing: Tokens.s3

                                Item {
                                    width: parent.width
                                    height: 30
                                    Text {
                                        anchors.left: parent.left
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: I18n.tr("Side")
                                        color: Tokens.inkDim
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fBody
                                    }
                                    Seg {
                                        anchors.right: parent.right
                                        anchors.verticalCenter: parent.verticalCenter
                                        options: [I18n.tr("Left"), I18n.tr("Right")]
                                        current: sidebarPlacement.side === "right"
                                            ? I18n.tr("Right") : I18n.tr("Left")
                                        onChose: label => sidebarPlacement.commit(
                                            label === I18n.tr("Right") ? "right" : "left",
                                            sidebarPlacement.tabName, sidebarPlacement.order)
                                    }
                                }

                                Item {
                                    width: parent.width
                                    height: 30
                                    Text {
                                        anchors.left: parent.left
                                        anchors.right: sidebarTab.left
                                        anchors.rightMargin: Tokens.s3
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: I18n.tr("Tab")
                                        color: Tokens.inkDim
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fBody
                                    }
                                    Field {
                                        id: sidebarTab
                                        anchors.right: parent.right
                                        anchors.verticalCenter: parent.verticalCenter
                                        width: 220
                                        text: sidebarPlacement.tabName
                                        placeholder: I18n.tr("Plugins")
                                        onCommitted: value => sidebarPlacement.commit(
                                            sidebarPlacement.side,
                                            String(value).trim() || "Plugins",
                                            sidebarPlacement.order)
                                    }
                                }

                                Item {
                                    width: parent.width
                                    height: 30
                                    Text {
                                        anchors.left: parent.left
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: I18n.tr("Order")
                                        color: Tokens.inkDim
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fBody
                                    }
                                    Row {
                                        anchors.right: parent.right
                                        anchors.verticalCenter: parent.verticalCenter
                                        spacing: Tokens.s2
                                        Text {
                                            anchors.verticalCenter: parent.verticalCenter
                                            text: sidebarPlacement.order
                                            color: Tokens.ink
                                            font.family: Tokens.mono
                                            font.pixelSize: Tokens.fBody
                                        }
                                        Step {
                                            value: sidebarPlacement.order
                                            from: -100
                                            to: 100
                                            stepBy: 1
                                            onModified: value => sidebarPlacement.commit(
                                                sidebarPlacement.side,
                                                sidebarPlacement.tabName, value)
                                        }
                                    }
                                }
                            }
                        }

                        // desktop-widget hint: those are placed on the wallpaper.
                        Text {
                            width: parent.width
                            visible: detail.enabled && detail.host === "desktopWidget"
                            text: I18n.tr("Desktop widgets are moved, resized and hidden on the wallpaper. Drag the tile, or right-click it for its menu.")
                            color: Tokens.inkMuted; font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                        }
                    }

                    // ── Settings: the plugin's own fields, from its schema ──
                    Column {
                        width: parent.width
                        spacing: Tokens.s3
                        visible: detail.schema.length > 0

                        Item {
                            width: parent.width; height: 20
                            Row {
                                id: sHead
                                anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s2
                                Rectangle { width: 4; height: 4; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
                                Text {
                                    text: I18n.tr("SETTINGS"); color: Tokens.ink; font.family: Tokens.ui
                                    font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                                    font.letterSpacing: Tokens.trackMark
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                            }
                            Rectangle {
                                anchors.left: sHead.right; anchors.leftMargin: Tokens.s3
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                height: 1; color: Tokens.lineSoft
                            }
                        }

                        // The settings form: a generic renderer that turns the
                        // plugin's declared schema into native controls. Nothing
                        // here is hardcoded per plugin; the type -> control switch
                        // (default arm = text, so unknown types stay editable) and
                        // a live local value mirror carry every field. Each change
                        // fires setSetting() -> plugins.json.
                        Column {
                            id: form
                            width: parent.width
                            spacing: Tokens.s4

                            property var values: detail.place.settings || ({})
                            property var _local: ({})
                            onValuesChanged: form._local = JSON.parse(JSON.stringify(form.values || {}))
                            Component.onCompleted: form._local = JSON.parse(JSON.stringify(form.values || {}))

                            function _val(field) {
                                if (form._local && form._local[field.key] !== undefined)
                                    return form._local[field.key];
                                return field.default;
                            }
                            function _set(key, value) {
                                var n = JSON.parse(JSON.stringify(form._local || {}));
                                n[key] = value;
                                form._local = n;
                                pg.setSetting(detail.sel.id, key, value);
                            }
                            function _choiceLabel(field, val) {
                                var os = field.options || [];
                                for (var i = 0; i < os.length; i++)
                                    if (String(os[i].value) === String(val))
                                        return os[i].label;
                                return String(val);
                            }
                            function _choiceKeyOf(field, label) {
                                var os = field.options || [];
                                for (var i = 0; i < os.length; i++)
                                    if (os[i].label === label)
                                        return os[i].value;
                                return label;
                            }

                            Repeater {
                                model: detail.schema

                                delegate: Column {
                                    id: fieldWrap
                                    required property var modelData
                                    required property int index
                                    width: form.width
                                    spacing: Tokens.s3

                                    readonly property string grp: modelData.group || ""
                                    readonly property bool startsGroup: fieldWrap.index === 0
                                        || ((detail.schema[fieldWrap.index - 1].group || "") !== fieldWrap.grp)

                                    // group header (grotesk section caps + hairline),
                                    // once per distinct group; blank group = none.
                                    Item {
                                        width: parent.width
                                        height: 16
                                        visible: fieldWrap.startsGroup && fieldWrap.grp.length > 0
                                        Text {
                                            id: gHead
                                            anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                            text: fieldWrap.grp
                                            color: Tokens.inkMuted; font.family: Tokens.ui
                                            font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                                            font.letterSpacing: Tokens.trackMark
                                            font.capitalization: Font.AllUppercase
                                        }
                                        Rectangle {
                                            anchors.left: gHead.right; anchors.leftMargin: Tokens.s3
                                            anchors.right: parent.right
                                            anchors.verticalCenter: parent.verticalCenter
                                            height: 1; color: Tokens.lineSoft
                                        }
                                    }

                                    Loader {
                                        width: parent.width
                                        sourceComponent: {
                                            switch (fieldWrap.modelData.type) {
                                            case "choice": return cChoice;
                                            case "toggle": return cToggle;
                                            case "slider": return cSlider;
                                            case "image": return cImage;
                                            default: return cText;   // text + anything unknown
                                            }
                                        }
                                        onLoaded: item.field = fieldWrap.modelData
                                    }
                                }
                            }

                            // ── field control templates ──

                            Component {
                                id: cToggle
                                Item {
                                    id: ct
                                    property var field: ({})
                                    width: form.width
                                    implicitHeight: 30
                                    Text {
                                        anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                        anchors.right: ctSw.left; anchors.rightMargin: Tokens.s3
                                        elide: Text.ElideRight
                                        text: ct.field.label || ct.field.key
                                        color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                                    }
                                    Sw {
                                        id: ctSw
                                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                        on: form._val(ct.field) === true || form._val(ct.field) === "true"
                                        onToggled: (v) => form._set(ct.field.key, v)
                                    }
                                }
                            }

                            // choice: a seg for <=4 options, a wrapped chip band
                            // for more (both invert the selected member; neither
                            // needs an overlay, so the form stays self-contained).
                            Component {
                                id: cChoice
                                Column {
                                    id: cc
                                    property var field: ({})
                                    width: form.width
                                    spacing: Tokens.s2
                                    readonly property var _labels: (cc.field.options || []).map(function (o) { return o.label; })
                                    readonly property string _curLabel: form._choiceLabel(cc.field, String(form._val(cc.field)))
                                    readonly property bool _few: cc._labels.length <= 4

                                    Item {
                                        visible: cc._few
                                        width: cc.width
                                        height: 30
                                        Text {
                                            anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                            anchors.right: segFew.left; anchors.rightMargin: Tokens.s3
                                            elide: Text.ElideRight
                                            text: cc.field.label || cc.field.key
                                            color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                                        }
                                        Seg {
                                            id: segFew
                                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                            options: cc._labels
                                            current: cc._curLabel
                                            onChose: (label) => form._set(cc.field.key, form._choiceKeyOf(cc.field, label))
                                        }
                                    }
                                    Text {
                                        visible: !cc._few
                                        width: cc.width
                                        text: cc.field.label || cc.field.key
                                        color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                                    }
                                    Chips {
                                        visible: !cc._few
                                        width: cc.width
                                        options: cc._labels
                                        current: cc._curLabel
                                        onChose: (label) => form._set(cc.field.key, form._choiceKeyOf(cc.field, label))
                                    }
                                }
                            }

                            // slider: reuses the integer-domain Slid by mapping the
                            // field's range onto its step count, so decimals and
                            // step survive; the numeral is the live readout.
                            Component {
                                id: cSlider
                                Item {
                                    id: cs
                                    property var field: ({})
                                    width: form.width
                                    implicitHeight: 30
                                    readonly property real _from: cs.field.min !== undefined ? cs.field.min : 0
                                    readonly property real _to: cs.field.max !== undefined ? cs.field.max : 1
                                    readonly property int _dec: cs.field.decimals !== undefined ? cs.field.decimals : 2
                                    readonly property real _step: cs.field.step !== undefined ? cs.field.step : (cs._dec === 0 ? 1 : 0.01)
                                    readonly property real _v: Number(form._val(cs.field))
                                    readonly property int _steps: Math.max(1, Math.round((cs._to - cs._from) / cs._step))

                                    Text {
                                        anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                        anchors.right: csNum.left; anchors.rightMargin: Tokens.s3
                                        elide: Text.ElideRight
                                        text: cs.field.label || cs.field.key
                                        color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                                    }
                                    Text {
                                        id: csNum
                                        anchors.right: csTrack.left; anchors.rightMargin: Tokens.s3
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: cs._dec === 0 ? ("" + Math.round(cs._v)) : cs._v.toFixed(cs._dec)
                                        color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                                    }
                                    Slid {
                                        id: csTrack
                                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                        width: 220
                                        from: 0
                                        to: cs._steps
                                        value: Math.round((cs._v - cs._from) / cs._step)
                                        onModified: (iv) => {
                                            var actual = cs._from + iv * cs._step;
                                            form._set(cs.field.key, cs._dec === 0 ? Math.round(actual) : Number(actual.toFixed(cs._dec)));
                                        }
                                    }
                                }
                            }

                            Component {
                                id: cText
                                Item {
                                    id: cx
                                    property var field: ({})
                                    width: form.width
                                    implicitHeight: 30
                                    Text {
                                        anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                        anchors.right: cxBox.left; anchors.rightMargin: Tokens.s3
                                        elide: Text.ElideRight
                                        text: cx.field.label || cx.field.key
                                        color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                                    }
                                    Field {
                                        id: cxBox
                                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                        width: 220
                                        placeholder: cx.field.placeholder || ""
                                        text: String(form._val(cx.field) || "")
                                        // commit on editing-finished only, like every field.
                                        onCommitted: (v) => form._set(cx.field.key, v)
                                    }
                                }
                            }

                            // image: a labelled box that opens the system file
                            // chooser (through the XDG portal); stores a file:// URL.
                            Component {
                                id: cImage
                                Item {
                                    id: ci
                                    property var field: ({})
                                    width: form.width
                                    implicitHeight: 30
                                    readonly property string cur: String(form._val(ci.field) || "")
                                    Text {
                                        anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                        anchors.right: ciBox.left; anchors.rightMargin: Tokens.s3
                                        elide: Text.ElideRight
                                        text: ci.field.label || ci.field.key
                                        color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                                    }
                                    Rectangle {
                                        id: ciBox
                                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                        width: 220; height: 30
                                        radius: Tokens.radius
                                        color: ciHov.hovered ? Tokens.tint10 : "transparent"
                                        border.width: Tokens.border
                                        border.color: ciHov.hovered ? Tokens.lineStrong : Tokens.line
                                        Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                        Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
                                        Text {
                                            anchors.left: parent.left; anchors.leftMargin: 9
                                            anchors.right: parent.right; anchors.rightMargin: 9
                                            anchors.verticalCenter: parent.verticalCenter
                                            elide: Text.ElideLeft
                                            text: ci.cur.length === 0 ? I18n.tr("Choose image\u2026") : ci.cur.replace(/^.*\//, "")
                                            color: ci.cur.length === 0 ? Tokens.inkMuted : Tokens.ink
                                            font.family: ci.cur.length === 0 ? Tokens.ui : Tokens.mono
                                            font.pixelSize: ci.cur.length === 0 ? 12 : 11
                                        }
                                        HoverHandler { id: ciHov; cursorShape: Qt.PointingHandCursor }
                                        TapHandler { onTapped: { detail.pendingImageField = ci.field; ciDlg.open(); } }
                                    }
                                }
                            }
                        }
                    }

                    // sibling empty state, agreeing with the form's own gate.
                    Text {
                        width: parent.width
                        visible: detail.schema.length === 0
                        text: I18n.tr("This add-on has no configurable settings.")
                        color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    }
                }
            }

            // image-type plugin option picker; lifted to the detail root so it
            // overlays as a full modal (the delegate row itself is only 30px tall).
            // The tapped field is captured in pendingImageField, like SchemaPage.
            PickFile {
                id: ciDlg
                title: I18n.tr("Choose an image")
                onPicked: (p) => { if (detail.pendingImageField) form._set(detail.pendingImageField.key, "" + p); ciDlg.active = false; }
                onCanceled: ciDlg.active = false
            }
        }
    }

    // ── destructive confirm: a bone plate, 2px border, an unambiguous verb ──
    // No red anywhere: inversion and the word carry the weight (DESIGN.md
    // sections 1 and 4). The scrim has no fill (translucency is banned); the
    // plate reads as an overlay by being bone on black.
    MouseArea {
        id: confirmScrim
        anchors.fill: parent
        visible: pg.confirmRemove
        z: 100
        onClicked: pg.confirmRemove = false

        Rectangle {
            id: plate
            anchors.centerIn: parent
            width: 380
            height: plateCol.implicitHeight + Tokens.s5 * 2
            radius: Tokens.radius
            color: Tokens.bone
            border.width: 2
            border.color: Tokens.inkOnBone

            // absorb clicks inside the plate so they do not dismiss it.
            MouseArea { anchors.fill: parent }

            Column {
                id: plateCol
                anchors.centerIn: parent
                width: parent.width - Tokens.s5 * 2
                spacing: Tokens.s4

                Text {
                    width: parent.width
                    text: I18n.tr("Remove %1?").arg(pg.sel && pg.sel.manifest && pg.sel.manifest.name
                        ? pg.sel.manifest.name : (pg.sel ? pg.sel.id : I18n.tr("add-on")))
                    color: Tokens.inkOnBone; font.family: Tokens.ui
                    font.pixelSize: Tokens.fValue; font.weight: Font.Medium
                    wrapMode: Text.WordWrap
                }
                Text {
                    width: parent.width
                    text: I18n.tr("This deletes the add-on and its settings from your desktop. You can reinstall it from RyoStore.")
                    color: Tokens.inkOnBoneDim; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                }
                Row {
                    anchors.right: parent.right
                    spacing: Tokens.s3

                    Rectangle {
                        width: cancelT.implicitWidth + 30; height: 32; radius: Tokens.radius
                        color: cancelH.hovered ? Tokens.lineOnBone : "transparent"
                        border.width: Tokens.border; border.color: Tokens.inkOnBoneDim
                        Behavior on color { ColorAnimation { duration: Tokens.snap } }
                        Text {
                            id: cancelT
                            anchors.centerIn: parent
                            text: I18n.tr("CANCEL"); color: Tokens.inkOnBone
                            font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                            font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                        }
                        HoverHandler { id: cancelH; cursorShape: Qt.PointingHandCursor }
                        TapHandler { onTapped: pg.confirmRemove = false }
                    }
                    // the committed verb: black on bone, inversion within inversion.
                    Rectangle {
                        width: rmT.implicitWidth + 30; height: 32; radius: Tokens.radius
                        color: rmH.hovered ? Tokens.inkOnBoneDim : Tokens.inkOnBone
                        Behavior on color { ColorAnimation { duration: Tokens.snap } }
                        Text {
                            id: rmT
                            anchors.centerIn: parent
                            text: I18n.tr("REMOVE"); color: Tokens.bone
                            font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                            font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                        }
                        HoverHandler { id: rmH; cursorShape: Qt.PointingHandCursor }
                        TapHandler { onTapped: pg.removePlugin(pg.selId) }
                    }
                }
            }
        }
    }
}

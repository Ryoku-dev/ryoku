pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Qt.labs.folderlistmodel
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import ".."
import "../LauncherSettings.js" as LauncherSettings

Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true

    readonly property var keys: [
        "variant", "radius", "bgBlur", "weatherUnit", "heroImage",
        "heroStrength", "heroPosX", "heroPosY", "showWeather", "showGreeting",
        "resultSettleMs", "horizonMode", "horizonColor", "overlayOpacity", "hiddenApps",
        "columns", "matWidth", "sectionOrder", "hiddenSections", "pinnedApps"
    ]
    readonly property var factory: ({
            "variant": "hero", "radius": 16, "bgBlur": 2, "weatherUnit": "auto", "heroImage": "",
            "heroStrength": 0.6, "heroPosX": 0.5, "heroPosY": 0.5,
            "showWeather": true, "showGreeting": true, "resultSettleMs": 360,
            "horizonMode": "auto", "horizonColor": "#ffc777", "overlayOpacity": 0.94, "hiddenApps": [],
            "columns": 8, "matWidth": 144, "sectionOrder": [], "hiddenSections": [], "pinnedApps": []
        })
    readonly property string launcherRoot: {
        var shellDir = String(Quickshell.env("RYOKU_SHELL_DIR") || "");
        if (shellDir.length > 0)
            return shellDir + "/quickshell/shell/modules/launcher";
        // Deployed Hub QML loads from a qrc namespace, so Qt.resolvedUrl gives no
        // usable file path; read the launcher the shell installs beside us.
        var cfg = String(Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config"));
        return cfg + "/quickshell/shell/modules/launcher";
    }
    property var catalog: ({ version: 0, fallback: "", variants: [] })
    // Launchers from RyoStore, installed or not: the catalogue says which can be
    // worn here and which to fetch first. Install stays in RyoStore; this page
    // shows their state, offers their updates, and wears what is yours.
    property var storeLaunchers: []
    property string updatingId: ""
    readonly property string storeRoot: (Quickshell.env("XDG_STATE_HOME")
        || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku/store"

    property var draft: pg.clone(pg.factory)
    property var committed: pg.clone(pg.factory)
    property bool loaded: false

    function clone(o) {
        var r = {};
        for (var k in o)
            r[k] = o[k];
        return r;
    }
    function catalogVariants() {
        return pg.catalog && Array.isArray(pg.catalog.variants)
            ? pg.catalog.variants : [];
    }
    // Every launcher Super+Space can open here: the shipped ones, then each one
    // installed from RyoStore that runs on this desktop.
    function wearableVariants() {
        return pg.catalogVariants().concat(pg.storeLaunchers.filter(function (entry) {
            return entry.installed && !entry.unavailable;
        }));
    }
    function variantEntry(id) {
        var variants = pg.wearableVariants();
        var requested = String(id || "");
        for (var i = 0; i < variants.length; i++)
            if (String(variants[i].id || "") === requested)
                return variants[i];
        var defaultId = String(pg.catalog && pg.catalog.default || "");
        for (var j = 0; j < variants.length; j++)
            if (String(variants[j].id || "") === defaultId)
                return variants[j];
        return variants.length > 0 ? variants[0]
            : ({ id: "", name: "", description: "", preview: "", capabilities: [] });
    }
    // A Store launcher runs from its installed view; the store's view path is
    // checked before it becomes a file path.
    function storeViewDir(item) {
        var view = String((item.metadata || {}).view || "");
        var pattern = new RegExp("^launcher-views/" + item.id + "/[a-f0-9]{64}$");
        return pattern.test(view) ? pg.storeRoot + "/" + view : "";
    }
    // Its Settings preview is the Preview.qml in that view.
    function storePreviewUrl(item) {
        var dir = pg.storeViewDir(item);
        return (item.metadata || {}).preview === "Preview.qml" && dir.length > 0
            ? "file://" + dir + "/Preview.qml" : "";
    }
    // What Settings offers for a Store launcher comes from the settings.json in
    // its view (see LauncherSettings.js); a shipped launcher's comes from
    // catalog.json.
    property var storeSettings: LauncherSettings.parse("")
    function activeSections() {
        return pg.variantEntry(pg.draft.variant).store ? pg.storeSettings.sections : [];
    }
    function browseStore() {
        Spawn.runApp(["ryostore", "open", "launchers"], { label: "ryostore" });
    }
    // Settings owns updates (docs/store.md): re-running the install replaces the
    // launcher in place, and the shell swaps the new view in without a reload.
    function updateLauncher(id) {
        if (!id || pg.updatingId !== "")
            return;
        pg.updatingId = id;
        launcherUpdate.command = ["ryostore", "install", "launchers", id];
        launcherUpdate.running = true;
    }
    function refreshStoreLaunchers() {
        storeCatalog.running = false;
        storeCatalog.running = true;
    }
    function activeCapabilities() {
        var entry = pg.variantEntry(pg.draft.variant);
        if (entry.store)
            return pg.storeSettings.capabilities;
        return Array.isArray(entry.capabilities) ? entry.capabilities : [];
    }
    function supports(capability) {
        return pg.activeCapabilities().indexOf(capability) !== -1;
    }
    function same(a, b) { return String(a) === String(b); }
    // JsonAdapter hands lists back as array-likes; keep plain arrays in the draft.
    function norm(v) {
        return (v && typeof v === "object" && v.length !== undefined)
            ? Array.prototype.slice.call(v).map(String) : v;
    }
    function listOf(d, k) { return pg.norm(d[k]) || []; }
    function hiddenList(d) { return pg.listOf(d, "hiddenApps"); }
    property bool hiddenOpen: false
    property bool pinnedOpen: false
    function basename(p) { return ("" + p).replace(/^.*\//, ""); }

    readonly property int dirtyCount: {
        if (!pg.loaded)
            return 0;
        var n = 0;
        for (var i = 0; i < pg.keys.length; i++)
            if (!pg.same(pg.draft[pg.keys[i]], pg.committed[pg.keys[i]]))
                n++;
        return n;
    }
    readonly property bool dirty: pg.dirtyCount > 0

    function edit(k, v) {
        var d = pg.clone(pg.draft);
        d[k] = v;
        pg.draft = d;
    }
    function adopt() {
        var d = {}, c = {};
        for (var i = 0; i < pg.keys.length; i++) {
            var k = pg.keys[i];
            d[k] = pg.norm(cfgA[k]);
            c[k] = pg.norm(cfgA[k]);
        }
        pg.draft = d;
        pg.committed = c;
    }
    function adoptExternal() {
        var d = {}, c = {};
        for (var i = 0; i < pg.keys.length; i++) {
            var k = pg.keys[i];
            if (pg.same(pg.draft[k], pg.committed[k])) {
                d[k] = pg.norm(cfgA[k]);
                c[k] = pg.norm(cfgA[k]);
            } else {
                d[k] = pg.draft[k];
                c[k] = pg.norm(cfgA[k]);
            }
        }
        pg.draft = d;
        pg.committed = c;
    }
    function save() {
        for (var i = 0; i < pg.keys.length; i++)
            cfgA[pg.keys[i]] = pg.draft[pg.keys[i]];
        cfg.writeAdapter();
        pg.committed = pg.clone(pg.draft);
    }
    function revert() { pg.draft = pg.clone(pg.committed); }
    function resetDefaults() { pg.draft = pg.clone(pg.factory); }

    function unitLabel(k) { return k === "C" ? "\u00b0C" : k === "F" ? "\u00b0F" : "Auto"; }
    function unitKey(l) { return l === "\u00b0C" ? "C" : l === "\u00b0F" ? "F" : "auto"; }
    function horizonModeLabel(k) { return k === "fixed" ? "Fixed" : k === "off" ? "Off" : "Palette"; }
    function horizonModeKey(l) { return l === "Fixed" ? "fixed" : l === "Off" ? "off" : "auto"; }

    FileView {
        id: catalogFile
        path: pg.launcherRoot + "/catalog.json"
        blockLoading: true
        printErrors: true
        onLoaded: pg.catalog = JSON.parse(text())
    }

    Process {
        id: storeCatalog
        command: ["ryostore", "catalog", "--category", "launchers"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const catalog = JSON.parse(this.text || "{}");
                    // A launcher written for another compositor that you have
                    // not installed can neither run nor be fetched here, so it
                    // is not offered.
                    pg.storeLaunchers = (catalog.items || [])
                        .filter(item => item.category === "launchers"
                            && !(item.unavailable === true && item.installed !== true))
                        .map(item => ({
                            id: item.id,
                            name: item.name || item.id,
                            description: item.summary || item.description || "",
                            store: true,
                            previewUrl: item.installed === true ? pg.storePreviewUrl(item) : "",
                            art: item.art || "",
                            capabilities: [],
                            viewDir: item.installed === true ? pg.storeViewDir(item) : "",
                            installed: item.installed === true,
                            version: item.version || "",
                            installedVersion: item.installedVersion || "",
                            updateAvailable: item.updateAvailable === true,
                            unavailable: item.unavailable === true,
                            unavailableEdition: item.unavailableEdition === true,
                            unavailableReason: item.unavailableReason || "",
                            requiredWindowManager: item.requiredWindowManager || "",
                            downloadPaused: item.downloadPaused === true,
                            downloadPauseReason: item.downloadPauseReason || ""
                        }));
                } catch (e) {
                    pg.storeLaunchers = [];
                }
            }
        }
    }

    // An update installs into your own data tree, so it runs in the background
    // with no terminal; any outcome re-reads the shelf so a failed update keeps
    // its UPDATE button instead of leaving a dead one.
    Process {
        id: launcherUpdate
        onExited: {
            pg.updatingId = "";
            pg.refreshStoreLaunchers();
        }
    }

    FileView {
        id: storeSettingsFile
        path: {
            var entry = pg.variantEntry(pg.draft.variant);
            return entry.store && entry.viewDir ? entry.viewDir + "/settings.json" : "";
        }
        printErrors: false
        onPathChanged: pg.storeSettings = LauncherSettings.parse("")
        onLoaded: pg.storeSettings = LauncherSettings.parse(text())
        onLoadFailed: pg.storeSettings = LauncherSettings.parse("")
    }

    FileView {
        id: cfg
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/launcher.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        atomicWrites: true
        onFileChanged: reload()
        onLoaded: { if (!pg.loaded) { pg.adopt(); pg.loaded = true; } else pg.adoptExternal(); }
        onLoadFailed: { if (!pg.loaded) { pg.adopt(); pg.loaded = true; } }

        JsonAdapter {
            id: cfgA
            property string variant: "hero"
            property real radius: 16
            property int bgBlur: 2
            property string weatherUnit: "auto"
            property string heroImage: ""
            property real heroStrength: 0.6
            property real heroPosX: 0.5
            property real heroPosY: 0.5
            property bool showWeather: true
            property bool showGreeting: true
            property int resultSettleMs: 360
            property string horizonMode: "auto"
            property string horizonColor: "#ffc777"
            property real overlayOpacity: 0.94
            property var hiddenApps: []
            property int columns: 8
            property int matWidth: 144
            property var sectionOrder: []
            property var hiddenSections: []
            property var pinnedApps: []
        }
    }

    Column {
        id: head
        anchors.top: parent.top
        anchors.topMargin: Tokens.s6
        // the head sits on the body's grid, so the title starts over the first card
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
                text: UiPreferences.brandMark("\u529b"); color: Tokens.ink; font.family: Tokens.jp
                font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: I18n.tr("DESKTOP"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Text {
            text: I18n.tr("App Launcher"); color: Tokens.ink
            font.family: Tokens.display; font.pixelSize: Tokens.fTitle
        }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("The command palette on Super+Space; nothing saves until you do.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
    }

    Preview {
        id: preview
        anchors { left: parent.left; right: parent.right; top: head.bottom }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6; anchors.topMargin: Tokens.s5
        height: 226
        label: {
            var name = String(pg.variantEntry(pg.draft.variant).name || I18n.tr("Launcher"));
            return I18n.tr("%1 PREVIEW").arg(name.toUpperCase());
        }
        tag: variantPreview.item
            ? (variantPreview.item.implicitWidth + " × " + variantPreview.item.implicitHeight)
            : ""
        live: true

        Item {
            id: previewStage
            anchors.fill: parent
            clip: true

            readonly property var entry: pg.variantEntry(pg.draft.variant)

            Loader {
                id: variantPreview
                anchors.centerIn: parent
                width: item ? item.implicitWidth : 0
                height: item ? item.implicitHeight : 0
                scale: item && item.implicitWidth > 0 && item.implicitHeight > 0
                    ? Math.min(0.72, Math.min(
                        (previewStage.width - 24) / item.implicitWidth,
                        (previewStage.height - 12) / item.implicitHeight))
                    : 1
                source: {
                    var entry = previewStage.entry;
                    if (entry.store)
                        return entry.previewUrl || "";
                    var file = String(entry.preview || "");
                    return file.length > 0
                        ? "file://" + pg.launcherRoot + "/" + file : "";
                }
                // A Store preview may skip the editing half of the contract.
                onLoaded: {
                    if ("settings" in item)
                        item.settings = Qt.binding(function () { return pg.draft; });
                    if (item.editRequested)
                        item.editRequested.connect(function (key, value) {
                            pg.edit(key, value);
                        });
                }
            }

            // A Store launcher that ships no Preview.qml shows its store art.
            Image {
                anchors.fill: parent
                anchors.margins: Tokens.s3
                visible: previewStage.entry.store === true && variantPreview.source.toString() === ""
                source: visible ? previewStage.entry.art : ""
                asynchronous: true
                fillMode: Image.PreserveAspectFit
            }
        }
    }

    Flickable {
        id: flick
        anchors {
            left: parent.left; right: parent.right
            top: preview.bottom; bottom: bar.top
            leftMargin: Tokens.s6; rightMargin: Tokens.s6
            topMargin: Tokens.s5; bottomMargin: Tokens.s4
        }
        contentWidth: width
        contentHeight: col.height + Tokens.s5
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        CardColumns {

        id: col
            // a body of cards fills the measure and splits into balanced columns
            width: flick.width - Tokens.s3
            spacing: Tokens.s5
            fillTo: flick.height

            SettingCard {
                // The primary choice of the page takes the band across the
                // columns: the shipped launchers and every one in RyoStore wrap
                // into as many rows of tiles as they need.
                property bool fullWidth: true
                width: col.colWidth
                title: I18n.tr("LAUNCHER")

                Item {
                    width: parent.width
                    height: styleBody.height + Tokens.s3 + Tokens.s4
                    Column {
                        id: styleBody
                        anchors { left: parent.left; right: parent.right; top: parent.top }
                        anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s3
                        spacing: Tokens.s3
                        // The height is computed, not measured: a Flow reports its
                        // post-wrap height a polish pass late, and CardColumns would
                        // place the next card over this one's second row.
                        Flow {
                            id: styleRow
                            width: parent.width
                            spacing: Tokens.s2
                            readonly property string worn: String(pg.variantEntry(pg.draft.variant).id || "")
                            readonly property int perRow: Math.max(2, Math.floor((width + Tokens.s2) / 210))
                            readonly property int tileH: 64
                            readonly property int rowCount: Math.max(1, Math.ceil(styleRep.count / perRow))
                            height: rowCount * tileH + (rowCount - 1) * spacing
                            Repeater {
                                id: styleRep
                                model: pg.catalogVariants().concat(pg.storeLaunchers)
                                delegate: Rectangle {
                                    id: styleTile
                                    required property var modelData
                                    readonly property bool fromStore: styleTile.modelData.store === true
                                    // A shipped launcher is always yours; a Store one
                                    // once it is installed and runs on this compositor.
                                    readonly property bool applyable: !styleTile.fromStore
                                        || (styleTile.modelData.installed && !styleTile.modelData.unavailable)
                                    readonly property bool on: styleTile.applyable && styleRow.worn === styleTile.modelData.id
                                    // offered only where re-running the install can succeed
                                    readonly property bool updatable: styleTile.fromStore
                                        && styleTile.modelData.updateAvailable === true
                                        && styleTile.modelData.version.length > 0
                                        && !styleTile.modelData.unavailable && !styleTile.modelData.downloadPaused
                                    // the launcher's own blurb when it is yours to wear,
                                    // otherwise the honest reason it is not
                                    readonly property string subText: {
                                        const item = styleTile.modelData;
                                        if (styleTile.updatable) {
                                            const from = item.installedVersion.length > 0 ? item.installedVersion : item.version;
                                            return I18n.tr("%1 \u2192 %2").arg(from).arg(item.version);
                                        }
                                        if (styleTile.applyable)
                                            return String(item.description || "");
                                        if (item.unavailable) {
                                            const wm = String(item.requiredWindowManager).toUpperCase();
                                            const tag = item.unavailableEdition ? I18n.tr("Not on this system")
                                                : wm.length > 0 ? I18n.tr("%1 only").arg(wm) : I18n.tr("Unavailable");
                                            return item.unavailableReason.length > 0 ? tag + " \u00b7 " + item.unavailableReason : tag;
                                        }
                                        if (item.downloadPaused)
                                            return item.downloadPauseReason.length > 0
                                                ? I18n.tr("Under construction") + " \u00b7 " + item.downloadPauseReason
                                                : I18n.tr("Under construction");
                                        return I18n.tr("Available \u00b7 install from RyoStore");
                                    }

                                    objectName: "launcher-style-" + styleTile.modelData.id
                                    width: Math.floor((styleRow.width - (styleRow.perRow - 1) * Tokens.s2) / styleRow.perRow)
                                    height: styleRow.tileH
                                    radius: Tokens.radius
                                    opacity: styleTile.applyable ? 1.0 : 0.55
                                    color: styleTile.on ? Tokens.bone : (styleMouse.containsMouse ? Tokens.tint5 : "transparent")
                                    border.width: Tokens.border
                                    border.color: styleTile.on ? Tokens.bone : Tokens.line
                                    Behavior on color { ColorAnimation { duration: Tokens.snap } }

                                    Column {
                                        anchors {
                                            left: parent.left
                                            right: updateBtn.visible ? updateBtn.left : parent.right
                                            margins: Tokens.s3; rightMargin: updateBtn.visible ? Tokens.s2 : Tokens.s3
                                            verticalCenter: parent.verticalCenter
                                        }
                                        spacing: 3
                                        Row {
                                            spacing: Tokens.s2
                                            Text {
                                                text: String(styleTile.modelData.name || styleTile.modelData.id).toUpperCase()
                                                color: styleTile.on ? Tokens.inkOnBone : Tokens.inkDim
                                                font.family: Tokens.ui
                                                font.pixelSize: 12
                                                font.weight: Font.Medium
                                                font.letterSpacing: Tokens.trackLabel
                                            }
                                            // marks community work apart from the
                                            // launchers the shell ships
                                            Text {
                                                visible: styleTile.fromStore
                                                anchors.verticalCenter: parent.verticalCenter
                                                text: I18n.tr("STORE")
                                                color: styleTile.on ? Tokens.inkOnBoneDim : Tokens.inkFaint
                                                font.family: Tokens.mono
                                                font.pixelSize: Tokens.fTiny
                                                font.letterSpacing: Tokens.trackLabel
                                            }
                                        }
                                        Text {
                                            width: parent.width
                                            text: styleTile.subText
                                            color: styleTile.on ? Tokens.inkOnBoneDim : Tokens.inkFaint
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fTiny
                                            elide: Text.ElideRight
                                        }
                                    }
                                    MouseArea {
                                        id: styleMouse
                                        anchors.fill: parent
                                        hoverEnabled: true
                                        preventStealing: true
                                        cursorShape: Qt.PointingHandCursor
                                        onClicked: styleTile.applyable ? pg.edit("variant", styleTile.modelData.id) : pg.browseStore()
                                    }
                                    // Declared after the MouseArea so it stacks above it:
                                    // the tile wears the launcher on click, and the update
                                    // must not be stolen by that handler.
                                    Btn {
                                        id: updateBtn
                                        anchors { right: parent.right; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                                        visible: styleTile.updatable
                                        compact: true
                                        armed: pg.updatingId === ""
                                        text: pg.updatingId === styleTile.modelData.id ? I18n.tr("UPDATING") : I18n.tr("UPDATE")
                                        onAct: pg.updateLauncher(styleTile.modelData.id)
                                    }
                                }
                            }
                        }
                        Btn {
                            text: I18n.tr("BROWSE RYOSTORE")
                            onAct: pg.browseStore()
                        }
                    }
                }
            }

            SettingCard {
                width: col.colWidth
                title: I18n.tr("PALETTE")
                visible: pg.supports("shape") || pg.supports("background")

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    visible: pg.supports("shape")
                    label: I18n.tr("Corner radius")
                    desc: I18n.tr("Rounds the palette corners; inner cards 4 px tighter.")
                    unit: "px"
                    value: String(pg.draft.radius)
                    def: String(pg.committed.radius)
                    changed: !pg.same(pg.draft.radius, pg.committed.radius)
                    source: "launcher.json"
                    controlWidth: 58
                    Step {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        value: Number(pg.draft.radius) || 0
                        from: 0; to: 28
                        onModified: (v) => pg.edit("radius", v)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    visible: pg.supports("background")
                    divider: pg.supports("shape")
                    label: I18n.tr("Backdrop frost")
                    desc: I18n.tr("Frosts the frozen desktop behind the result drawer.")
                    unit: "px"
                    value: String(pg.draft.bgBlur)
                    def: String(pg.committed.bgBlur)
                    changed: !pg.same(pg.draft.bgBlur, pg.committed.bgBlur)
                    source: "launcher.json"
                    controlWidth: 58
                    Step {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        value: Number(pg.draft.bgBlur) || 0
                        from: 0; to: 30
                        onModified: (v) => pg.edit("bgBlur", v)
                    }
                }
            }

            SettingCard {
                id: overlayCard
                width: col.colWidth
                title: I18n.tr("OVERLAY")
                visible: pg.supports("overlay")

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    label: I18n.tr("Overlay opacity")
                    desc: I18n.tr("How opaque the backdrop behind the launcher is.")
                    unit: "%"
                    value: String(Math.round((Number(pg.draft.overlayOpacity) || 0) * 100))
                    def: String(Math.round((Number(pg.committed.overlayOpacity) || 0) * 100))
                    changed: !pg.same(pg.draft.overlayOpacity, pg.committed.overlayOpacity)
                    source: "launcher.json"
                    controlWidth: Math.min(240, Math.max(160, Math.round(overlayCard.width * 0.34)))
                    Slid {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        width: parent.width
                        value: Math.round((Number(pg.draft.overlayOpacity) || 0) * 100)
                        from: 30; to: 100
                        onModified: (v) => pg.edit("overlayOpacity", v / 100)
                    }
                }
            }

            SettingCard {
                id: hiddenCard
                width: col.colWidth
                title: I18n.tr("HIDDEN APPS")
                visible: pg.supports("hiddenApps")

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    footH: 32
                    label: I18n.tr("Apps hidden from the launcher")
                    desc: I18n.tr("They stay installed; only the launcher stops listing them.")
                    unit: I18n.tr("hidden")
                    value: String(pg.hiddenList(pg.draft).length)
                    def: String(pg.hiddenList(pg.committed).length)
                    changed: !pg.same(pg.hiddenList(pg.draft), pg.hiddenList(pg.committed))
                    source: "launcher.json"
                    controlWidth: 190
                    Row {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: Tokens.s2
                        Btn {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: pg.hiddenList(pg.draft).length > 0
                            text: I18n.tr("SHOW ALL")
                            onAct: pg.edit("hiddenApps", [])
                        }
                        Btn {
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("CHOOSE")
                            onAct: pg.hiddenOpen = true
                        }
                    }
                }
            }

            SettingCard {
                id: layoutCard
                width: col.colWidth
                title: I18n.tr("GRID")
                visible: pg.supports("grid")

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    label: I18n.tr("Columns")
                    desc: I18n.tr("Apps per row. Fewer fit on a narrow screen.")
                    value: String(pg.draft.columns)
                    def: String(pg.committed.columns)
                    changed: !pg.same(pg.draft.columns, pg.committed.columns)
                    source: "launcher.json"
                    controlWidth: 58
                    Step {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        value: Number(pg.draft.columns) || 8
                        from: 3; to: 16; stepBy: 1
                        onModified: (v) => pg.edit("columns", v)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Tile width")
                    desc: I18n.tr("Width of each app tile; the grid is columns times this.")
                    unit: "px"
                    value: String(pg.draft.matWidth)
                    def: String(pg.committed.matWidth)
                    changed: !pg.same(pg.draft.matWidth, pg.committed.matWidth)
                    source: "launcher.json"
                    controlWidth: 76
                    Step {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        value: Number(pg.draft.matWidth) || 144
                        from: 90; to: 240; stepBy: 6
                        onModified: (v) => pg.edit("matWidth", v)
                    }
                }
            }

            SettingCard {
                id: sectionsCard
                width: col.colWidth
                title: I18n.tr("SECTIONS")
                visible: pg.supports("sections") && pg.activeSections().length > 0

                SectionsEditor {
                    anchors.left: parent.left; anchors.right: parent.right
                    known: pg.activeSections()
                    order: pg.listOf(pg.draft, "sectionOrder")
                    hidden: pg.listOf(pg.draft, "hiddenSections")
                    onEdited: (order, hidden) => {
                        pg.edit("sectionOrder", order);
                        pg.edit("hiddenSections", hidden);
                    }
                }
            }

            SettingCard {
                id: favoritesCard
                width: col.colWidth
                title: I18n.tr("FAVORITES")
                visible: pg.supports("pinnedApps")

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    footH: 32
                    label: I18n.tr("Pinned apps")
                    desc: I18n.tr("Listed first in every section and in search, marked with a star.")
                    unit: I18n.tr("pinned")
                    value: String(pg.listOf(pg.draft, "pinnedApps").length)
                    def: String(pg.listOf(pg.committed, "pinnedApps").length)
                    changed: !pg.same(pg.listOf(pg.draft, "pinnedApps"), pg.listOf(pg.committed, "pinnedApps"))
                    source: "launcher.json"
                    controlWidth: 190
                    Row {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: Tokens.s2
                        Btn {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: pg.listOf(pg.draft, "pinnedApps").length > 0
                            text: I18n.tr("CLEAR")
                            onAct: pg.edit("pinnedApps", [])
                        }
                        Btn {
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("CHOOSE")
                            onAct: pg.pinnedOpen = true
                        }
                    }
                }
            }

            SettingCard {
                width: col.colWidth
                title: I18n.tr("RESULT MOTION")
                visible: pg.supports("results")

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    label: I18n.tr("Type settle")
                    desc: I18n.tr("Pause before the results fade in.")
                    unit: "ms"
                    value: String(pg.draft.resultSettleMs)
                    def: String(pg.committed.resultSettleMs)
                    changed: !pg.same(pg.draft.resultSettleMs, pg.committed.resultSettleMs)
                    source: "launcher.json"
                    controlWidth: 58
                    Step {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        value: Number(pg.draft.resultSettleMs) || 360
                        from: 120; to: 700
                        onModified: (v) => pg.edit("resultSettleMs", v)
                    }
                }
            }

            SettingCard {
                width: col.colWidth
                title: I18n.tr("HERO")
                visible: pg.supports("hero")

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    label: I18n.tr("Show greeting")
                    desc: I18n.tr("Time-of-day greeting above the hero clock.")
                    def: pg.committed.showGreeting ? I18n.tr("ON") : I18n.tr("OFF")
                    changed: !pg.same(pg.draft.showGreeting, pg.committed.showGreeting)
                    source: "launcher.json"
                    controlWidth: 54
                    Sw {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        on: !!pg.draft.showGreeting
                        onToggled: (v) => pg.edit("showGreeting", v)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Show weather")
                    desc: I18n.tr("Weather and temperature on the hero; off shows the date.")
                    def: pg.committed.showWeather ? I18n.tr("ON") : I18n.tr("OFF")
                    changed: !pg.same(pg.draft.showWeather, pg.committed.showWeather)
                    source: "launcher.json"
                    controlWidth: 54
                    Sw {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        on: !!pg.draft.showWeather
                        onToggled: (v) => pg.edit("showWeather", v)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    block: true
                    label: I18n.tr("Weather units")
                    desc: I18n.tr("Temperature scale on the hero; Auto follows your locale.")
                    def: I18n.tr(pg.unitLabel(pg.committed.weatherUnit))
                    changed: !pg.same(pg.draft.weatherUnit, pg.committed.weatherUnit)
                    source: "launcher.json"
                    Seg {
                        anchors.left: parent.left; anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        options: ["Auto", "\u00b0C", "\u00b0F"]
                        current: pg.unitLabel(pg.draft.weatherUnit)
                        onChose: (l) => pg.edit("weatherUnit", pg.unitKey(l))
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    block: true
                    label: I18n.tr("Solar line")
                    desc: I18n.tr("Warm line under the clock: Palette, Fixed or Off.")
                    def: I18n.tr(pg.horizonModeLabel(pg.committed.horizonMode))
                    changed: !pg.same(pg.draft.horizonMode, pg.committed.horizonMode)
                    source: "launcher.json"
                    Seg {
                        anchors.left: parent.left; anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        options: ["Palette", "Fixed", "Off"]
                        current: pg.horizonModeLabel(pg.draft.horizonMode)
                        onChose: (l) => pg.edit("horizonMode", pg.horizonModeKey(l))
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    visible: pg.draft.horizonMode === "fixed"
                    divider: true
                    block: true
                    label: I18n.tr("Line colour")
                    desc: I18n.tr("Colour of the solar line when it is set to Fixed.")
                    def: String(pg.committed.horizonColor)
                    changed: !pg.same(pg.draft.horizonColor, pg.committed.horizonColor)
                    source: "launcher.json"
                    ColorField {
                        anchors.left: parent.left
                        anchors.verticalCenter: parent.verticalCenter
                        width: Math.min(260, parent.width)
                        value: String(pg.draft.horizonColor)
                        onChosen: (v) => pg.edit("horizonColor", v)
                    }
                }
            }

            SettingCard {
                id: heroImgCard
                width: col.colWidth
                title: I18n.tr("HERO IMAGE")
                visible: pg.supports("hero")

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    footH: 32
                    label: I18n.tr("Image")
                    desc: I18n.tr("Wide banner behind the palette; drag to reframe it.")
                    changed: !pg.same(pg.draft.heroImage, pg.committed.heroImage)
                    source: "launcher.json"
                    Item {
                        anchors.fill: parent
                        Text {
                            anchors.left: parent.left
                            anchors.right: heroActs.left
                            anchors.rightMargin: Tokens.s3
                            anchors.verticalCenter: parent.verticalCenter
                            elide: Text.ElideMiddle
                            text: (pg.draft.heroImage || "").length > 0 ? pg.basename(pg.draft.heroImage) : I18n.tr("Shipped art")
                            color: (pg.draft.heroImage || "").length > 0 ? Tokens.inkDim : Tokens.inkFaint
                            font.family: (pg.draft.heroImage || "").length > 0 ? Tokens.mono : Tokens.ui
                            font.pixelSize: (pg.draft.heroImage || "").length > 0 ? 12 : Tokens.fSmall
                        }
                        Row {
                            id: heroActs
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s2
                            Btn {
                                anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("CHANGE")
                                onAct: pg.openPicker()
                            }
                            Btn {
                                anchors.verticalCenter: parent.verticalCenter
                                visible: (pg.draft.heroImage || "").length > 0
                                text: I18n.tr("USE SHIPPED ART")
                                onAct: pg.edit("heroImage", "")
                            }
                        }
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Strength")
                    desc: I18n.tr("How visible the hero image is.")
                    unit: "%"
                    value: String(Math.round((Number(pg.draft.heroStrength) || 0) * 100))
                    def: String(Math.round((Number(pg.committed.heroStrength) || 0) * 100))
                    changed: !pg.same(pg.draft.heroStrength, pg.committed.heroStrength)
                    source: "launcher.json"
                    controlWidth: Math.min(240, Math.max(160, Math.round(heroImgCard.width * 0.34)))
                    Slid {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        width: parent.width
                        value: Math.round((Number(pg.draft.heroStrength) || 0) * 100)
                        from: 0; to: 100
                        onModified: (v) => pg.edit("heroStrength", v / 100)
                    }
                }
            }
        }
    }

    Rectangle {
        id: bar
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        height: 60
        color: "transparent"
        Rectangle {
            anchors { left: parent.left; right: parent.right; top: parent.top }
            height: 1; color: Tokens.line
        }

        Row {
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            Rectangle {
                id: dot
                anchors.verticalCenter: parent.verticalCenter
                width: 6; height: 6; radius: 3
                antialiasing: false
                color: pg.dirty ? Tokens.ink : "transparent"
                border.width: pg.dirty ? 0 : Tokens.border
                border.color: Tokens.inkFaint

                SequentialAnimation on opacity {
                    running: pg.dirty
                    loops: Animation.Infinite
                    NumberAnimation { to: 0.3; duration: 600; easing.type: Easing.InOutSine }
                    NumberAnimation { to: 1.0; duration: 600; easing.type: Easing.InOutSine }
                    onStopped: dot.opacity = 1
                }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: pg.dirty
                    ? (pg.dirtyCount === 1
                        ? I18n.tr("%1 CHANGE \u00b7 PREVIEWING \u00b7 NOT SAVED").arg(pg.dirtyCount)
                        : I18n.tr("%1 CHANGES \u00b7 PREVIEWING \u00b7 NOT SAVED").arg(pg.dirtyCount))
                    : I18n.tr("SAVED \u00b7 LIVE ON YOUR DESKTOP")
                color: pg.dirty ? Tokens.ink : Tokens.inkMuted
                font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
            }
        }

        Row {
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("RESET TO DEFAULTS")
                onAct: pg.resetDefaults()
            }
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: 1; height: 20; color: Tokens.line
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("REVERT")
                armed: pg.dirty
                onAct: pg.revert()
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("SAVE")
                primary: true
                armed: pg.dirty
                onAct: pg.save()
            }
        }
    }

    property bool pickerOpen: false
    readonly property string home: Quickshell.env("HOME") || ""
    property url pickerFolder: "file://" + Paths.picturesDir
    function openPicker() {
        pg.pickerFolder = "file://" + Paths.picturesDir;
        pg.pickerOpen = true;
    }
    function gotoDir(sub) { pg.pickerFolder = "file://" + pg.home + (sub.length ? "/" + sub : ""); }

    AppSetPicker {
        anchors.fill: parent
        visible: pg.hiddenOpen
        z: 110
        hidden: pg.hiddenList(pg.draft)
        onChanged: (ids) => pg.edit("hiddenApps", ids)
        onDismissed: pg.hiddenOpen = false
    }

    AppSetPicker {
        anchors.fill: parent
        visible: pg.pinnedOpen
        z: 110
        title: I18n.tr("Pinned apps")
        countText: I18n.tr("%1 pinned")
        invert: false
        hidden: pg.listOf(pg.draft, "pinnedApps")
        onChanged: (ids) => pg.edit("pinnedApps", ids)
        onDismissed: pg.pinnedOpen = false
    }

    Item {
        id: pickerLayer
        anchors.fill: parent
        visible: pg.pickerOpen
        z: 100

        MouseArea { anchors.fill: parent; hoverEnabled: true; onClicked: pg.pickerOpen = false }

        FolderListModel {
            id: fm
            folder: pg.pickerFolder
            showDirs: true
            showDirsFirst: true
            showDotAndDotDot: false
            showHidden: false
            nameFilters: ["*.png", "*.jpg", "*.jpeg", "*.bmp"]
            sortField: FolderListModel.Name
        }

        Rectangle {
            id: panel
            anchors.centerIn: parent
            width: Math.min(parent.width - 80, 900)
            height: Math.min(parent.height - 60, 560)
            radius: Tokens.radius
            color: Tokens.paperLift
            border.width: Tokens.border
            border.color: Tokens.lineStrong
            MouseArea { anchors.fill: parent; onClicked: {} }

            Text {
                id: ptitle
                anchors { left: parent.left; top: parent.top; margins: Tokens.s4 }
                text: I18n.tr("CHOOSE A HERO IMAGE")
                color: Tokens.ink; font.family: Tokens.ui
                font.pixelSize: 10; font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
            }
            Text {
                anchors.left: ptitle.left
                anchors.top: ptitle.bottom
                anchors.topMargin: Tokens.s1
                anchors.right: pclose.left
                anchors.rightMargin: Tokens.s3
                elide: Text.ElideLeft
                text: ("" + pg.pickerFolder).replace("file://", "").replace(pg.home, "~")
                color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 11
            }
            IconBtn {
                id: pclose
                anchors { right: parent.right; top: parent.top; margins: Tokens.s3 }
                glyph: "\u2715"
                onAct: pg.pickerOpen = false
            }

            Row {
                id: pnav
                anchors { left: parent.left; right: parent.right; top: ptitle.bottom }
                anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s5
                spacing: Tokens.s2

                Btn { text: I18n.tr("\u2191 UP"); onAct: pg.pickerFolder = fm.parentFolder }
                Btn { text: I18n.tr("HOME"); onAct: pg.gotoDir("") }
                Btn { text: I18n.tr("PICTURES"); onAct: pg.pickerFolder = "file://" + Paths.picturesDir }
                Btn { text: I18n.tr("DOWNLOADS"); onAct: pg.gotoDir("Downloads") }
                Btn { text: I18n.tr("STORE"); onAct: pg.pickerFolder = "file://" + Paths.picturesDir + "/ryoku-launchers" }
            }

            GridView {
                id: grid
                anchors {
                    left: parent.left; right: parent.right
                    top: pnav.bottom; bottom: pfoot.top
                    leftMargin: Tokens.s3; rightMargin: Tokens.s3
                    topMargin: Tokens.s3; bottomMargin: Tokens.s2
                }
                clip: true
                readonly property int cols: Math.max(3, Math.floor(width / 190))
                cellWidth: Math.floor(width / cols)
                cellHeight: Math.round(cellWidth * 0.72)
                cacheBuffer: 1200
                boundsBehavior: Flickable.StopAtBounds
                model: fm
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                WheelScroll { }

                delegate: Item {
                    id: tile
                    required property string fileName
                    required property url fileUrl
                    required property bool fileIsDir
                    width: grid.cellWidth
                    height: grid.cellHeight

                    Rectangle {
                        anchors.fill: parent
                        anchors.margins: 4
                        radius: Tokens.radius
                        color: tile.fileIsDir && th.hovered ? Tokens.bone : (th.hovered ? Tokens.tint5 : "transparent")
                        border.width: Tokens.border
                        border.color: th.hovered ? (tile.fileIsDir ? Tokens.bone : Tokens.ink) : Tokens.line
                        clip: true
                        Behavior on color { ColorAnimation { duration: Tokens.snap } }
                        Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

                        Column {
                            visible: tile.fileIsDir
                            anchors.centerIn: parent
                            width: parent.width - Tokens.s5
                            spacing: Tokens.s2
                            Text {
                                anchors.horizontalCenter: parent.horizontalCenter
                                text: I18n.tr("DIR")
                                color: th.hovered ? Tokens.inkOnBone : Tokens.inkMuted
                                font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                                font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                            }
                            Text {
                                width: parent.width
                                horizontalAlignment: Text.AlignHCenter
                                elide: Text.ElideMiddle
                                text: tile.fileName
                                color: th.hovered ? Tokens.inkOnBone : Tokens.inkDim
                                font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            }
                        }

                        Image {
                            visible: !tile.fileIsDir
                            anchors.fill: parent
                            anchors.margins: 1
                            asynchronous: true
                            cache: true
                            fillMode: Image.PreserveAspectCrop
                            sourceSize: Qt.size(Math.ceil(parent.width * 1.4), Math.ceil(parent.height * 1.4))
                            source: tile.fileIsDir ? "" : tile.fileUrl
                        }

                        Rectangle {
                            visible: !tile.fileIsDir && th.hovered
                            anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
                            height: 20
                            color: Tokens.paperLift
                            Text {
                                anchors.fill: parent
                                anchors.leftMargin: Tokens.s2
                                anchors.rightMargin: Tokens.s2
                                verticalAlignment: Text.AlignVCenter
                                elide: Text.ElideMiddle
                                text: tile.fileName
                                color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                            }
                        }

                        HoverHandler { id: th; cursorShape: Qt.PointingHandCursor }
                        TapHandler {
                            onTapped: {
                                if (tile.fileIsDir) {
                                    pg.pickerFolder = tile.fileUrl;
                                } else {
                                    pg.edit("heroImage", "" + tile.fileUrl);
                                    pg.pickerOpen = false;
                                }
                            }
                        }
                    }
                }
            }

            Text {
                anchors.centerIn: grid
                visible: fm.status === FolderListModel.Ready && fm.count === 0
                text: I18n.tr("NO IMAGES OR FOLDERS HERE")
                color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 12; font.letterSpacing: 2
            }

            Item {
                id: pfoot
                anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
                height: 52
                Rectangle {
                    anchors { left: parent.left; right: parent.right; top: parent.top }
                    height: 1; color: Tokens.lineSoft
                }
                Btn {
                    anchors.right: parent.right
                    anchors.rightMargin: Tokens.s4
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("CANCEL")
                    onAct: pg.pickerOpen = false
                }
            }
        }
    }
}

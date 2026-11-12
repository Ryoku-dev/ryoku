import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import Ryoku.FrameBars
import "../Singletons"
import "../components"
import "../components/bar"
import "../schema/bar.js" as Schema
import "../ShellIndex.js" as ShellIndex

// bar styles and complete desktops, where the bar shows, and the options of the one in use
Item {
    id: root

    required property var hub
    property string pendingRow: ""
    // a search landing owns the scroll position, so the gallery leaves it alone
    property bool landed: false

    readonly property var active: catalog.activeCard
    readonly property string activeName: root.active ? root.active.name : catalog.active
    readonly property string editor: catalog.editorOf(root.active)
    readonly property bool ownWindow: root.editor === "" && catalog.hasOwnWindow(root.active)

    readonly property var nacreDefaults: NacreConfig.defaultConfig()
    readonly property var railDefaults: FrameBars.defaultConfig().rails.left
    readonly property var nacreBind: ({ store: "settings", key: "nacre", fallback: root.nacreDefaults })
    readonly property var framesBind: ({ store: "settings", key: "frameBars", fallback: FrameBars.defaultConfig() })
    readonly property var nacre: NacreConfig.normalize(Changes.value(root.nacreBind))
    readonly property var frames: FrameBars.normalize(Changes.value(root.framesBind), BarCatalog, MenuCatalog)

    function fallbackFor(key) {
        if (key.indexOf("nacre.") === 0)
            return root.nacreDefaults[key.slice(6)];
        if (key.indexOf("frameBars.rails.left.") === 0)
            return root.railDefaults[key.slice(21)];
        return undefined;
    }

    function withFallback(bind) {
        if (!bind || bind.fallback !== undefined)
            return bind;
        var d = root.fallbackFor(bind.key);
        return d === undefined ? bind : Object.assign({}, bind, { fallback: d });
    }

    function prepared(row) {
        if (!row.bind)
            return row;
        var bind = root.withFallback(row.bind);
        var out = Object.assign({}, row, { bind: bind });
        if (out["default"] === undefined && bind.fallback !== undefined)
            out["default"] = bind.fallback;
        if (row.when)
            out.when = { bind: root.withFallback(row.when.bind), is: row.when.is };
        return out;
    }

    function screenRow(screen) {
        var name = String(screen.name || "");
        var model = String(screen.model || "");
        var label = model.length > 0 ? model : name;
        return {
            id: "show-bar-" + name, section: "displays", label: label,
            changeLabel: I18n.tr("Bar on %1").arg(label),
            detail: I18n.tr("%1, %2 × %3").arg(name).arg(screen.width).arg(screen.height),
            control: "toggle", bind: { store: "settings", key: "displays.bar." + name, fallback: true }, "default": true
        };
    }

    readonly property var displayRows: Quickshell.screens.filter(s => String(s.name || "") !== "").map(s => root.screenRow(s))

    readonly property var ownWindowRows: root.ownWindow ? [{
        id: "own-settings-open", section: "options", label: "Open its settings",
        detail: I18n.tr("%1 keeps its options in a window of its own").arg(root.activeName),
        control: "handoff", command: catalog.ownWindowCommand(root.active), actionText: "Open"
    }] : []

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.filter(r => r.template !== true && (!r.editor || r.editor === root.editor))
            .map(r => root.prepared(r))
            .concat(root.displayRows)
            .concat(root.ownWindowRows)
    })

    readonly property var stateLine: !root.active ? ""
        : root.active.category === "subshells" ? I18n.tr("In use: %1, a complete desktop").arg(root.activeName)
        : I18n.tr("In use: %1").arg(root.activeName)

    function sectionIndex(id) {
        for (var i = 0; i < page.layout.length; i++)
            if (page.layout[i].id === id)
                return i;
        return -1;
    }

    // search lands on drawn rows; a row the page draws itself, or a section named in a deep link, lands on its section
    function focusRow(rowId) {
        if (!rowId)
            return;
        root.landed = true;
        if (page.layout.length === 0) {
            root.pendingRow = rowId;
            return;
        }
        var target = rowId === "show-bar" && root.displayRows.length > 0 ? root.displayRows[0].id : rowId;
        if (page.sectionOfRow(target)) {
            page.focusRow(target);
            return;
        }
        var row = Schema.rows.find(r => r.id === rowId);
        // a row of the style in use's own options waits a moment for the style's editor to be
        // known (the shells index arrives after the page opens), then lands on its section
        if (row && row.editor && row.editor !== root.editor && !editorWait.over) {
            editorWait.rowId = rowId;
            editorWait.restart();
            return;
        }
        editorWait.over = false;
        var at = root.sectionIndex(row ? row.section : rowId);
        if (at >= 0)
            page.chooseSection(at);
    }
    Timer {
        id: editorWait
        property string rowId: ""
        property bool over: false
        interval: 3000
        onTriggered: {
            editorWait.over = true;
            root.focusRow(editorWait.rowId);
        }
    }
    onEditorChanged: {
        if (editorWait.running) {
            editorWait.stop();
            Qt.callLater(root.focusRow, editorWait.rowId);
        }
    }

    // a card's detail modal: opened by a click or Enter, it shows the style large
    property string detailId: ""
    readonly property var detailCard: catalog.card(detailId)
    function openDetail(id) {
        root.detailId = id;
        detailDialog.open();
    }

    StyleDetailDialog {
        id: detailDialog
        parent: root.hub.overlayItem
        card: root.detailCard
        styles: catalog
        onAct: (styleId, action) => root.act(styleId, action)
    }

    function act(id, action) {
        var c = catalog.card(id);
        if (!c)
            return;
        if (action === "use")
            catalog.use(id);
        else if (action === "install" || action === "update")
            catalog.install(id, action);
        else if (action === "settings")
            catalog.openOwnWindow(c);
        else if (action === "options")
            page.chooseSection(root.sectionIndex("options"));
    }

    // lane edits for the sectioned bar: from "" is a widget off the bar
    function moveNacre(widgetId, fromLane, fromIndex, toLane, toIndex) {
        var next = toLane === "" ? NacreConfig.remove(root.nacre, widgetId)
            : NacreConfig.move(root.nacre, widgetId, fromLane, toLane, toIndex);
        Changes.set(root.nacreBind, next, I18n.tr("Bar layout"), "bar");
    }

    // edge bar edits; adding a widget also shows the edge bar
    function moveRail(widgetId, fromZone, fromIndex, toZone, toIndex) {
        var cfg = root.frames, next;
        if (toZone === "")
            next = FrameBars.removeWidget(cfg, "left", fromZone, fromIndex);
        else if (fromZone === "") {
            next = FrameBars.addWidget(cfg, "left", toZone, widgetId, BarCatalog);
            var list = next.rails.left[toZone];
            var placed = list.indexOf(widgetId);
            if (placed >= 0 && toIndex < placed) {
                list.splice(placed, 1);
                list.splice(Math.max(0, toIndex), 0, widgetId);
            }
            next.rails.left.enabled = true;
        } else {
            var target = fromZone === toZone && fromIndex < toIndex ? toIndex - 1 : toIndex;
            next = FrameBars.moveWidget(cfg, "left", fromZone, fromIndex, "left", toZone, target, BarCatalog);
        }
        Changes.set(root.framesBind, next, I18n.tr("Edge bar widgets"), "bar");
    }

    function nacreLabel(id) {
        switch (id) {
        case "brand": return I18n.tr("Ryoku mark");
        case "media": return I18n.tr("Media");
        case "activeWindow": return I18n.tr("Active window");
        case "clock": return I18n.tr("Clock");
        case "workspaces": return I18n.tr("Workspaces");
        case "resources": return I18n.tr("Resources");
        case "connectivity": return I18n.tr("Connections");
        case "audio": return I18n.tr("Sound");
        case "battery": return I18n.tr("Battery");
        case "notifications": return I18n.tr("Notifications");
        case "tray": return I18n.tr("Tray");
        case "weather": return I18n.tr("Weather");
        case "utils": return I18n.tr("Recording");
        }
        return id;
    }

    function railLabel(id) {
        switch (id) {
        case "audio-input": return I18n.tr("Microphone");
        case "audio-output": return I18n.tr("Volume");
        case "battery": return I18n.tr("Battery");
        case "bluetooth": return I18n.tr("Bluetooth");
        case "clock": return I18n.tr("Clock");
        case "dock": return I18n.tr("Dock");
        case "workspaces": return I18n.tr("Workspaces");
        case "lock": return I18n.tr("Lock");
        case "logout": return I18n.tr("Log out");
        case "music": return I18n.tr("Music");
        case "network": return I18n.tr("Network");
        case "notifications": return I18n.tr("Notifications");
        case "quick-settings": return I18n.tr("Quick settings");
        case "sysmon": return I18n.tr("System monitor");
        case "shutdown": return I18n.tr("Shut down");
        case "tray": return I18n.tr("Tray");
        case "vpn": return I18n.tr("VPN");
        }
        return id;
    }

    function contributed(kind) {
        var list = ShellIndex.contributions(catalog.index, kind);
        return list.map(p => p.name === p.itemName ? p.name : I18n.tr("%1 from %2").arg(p.name).arg(p.itemName)).join(", ");
    }

    function partDetail(lead, kind) {
        var extra = root.contributed(kind);
        return extra.length > 0 ? lead + ". " + I18n.tr("Your styles add: %1").arg(extra) : lead;
    }

    BarStyles { id: catalog }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "bar"
        schema: root.schema
        stateLine: root.stateLine
        bodies: ({ style: galleryBody, options: root.editor === "nacre" ? lanesBody : null, parts: partsBody })
        footers: ({ options: root.editor === "frame-rails" ? railBody : root.editor === "" && !root.ownWindow ? noOptionsBody : null })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    Component {
        id: galleryBody
        StyleGallery {
            styles: catalog
            onAct: (styleId, action) => root.act(styleId, action)
            onDetailRequested: styleId => root.openDetail(styleId)
            onRevealActive: (top, bottom) => {
                if (root.landed)
                    return;
                var y0 = mapToItem(page, 0, top).y, y1 = mapToItem(page, 0, bottom).y;
                if (y1 > page.height)
                    page.scrollTo(y0 - (page.height - (y1 - y0)) / 2);
            }
        }
    }

    Component {
        id: lanesBody
        Column {
            spacing: LithicTokens.space.s2
            Label {
                width: parent.width
                text: I18n.tr("Drag a widget between the bar's sections, or onto Not on the bar to take it off. Each widget also has a menu.")
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            LaneEditor {
                width: parent.width
                sideBySide: true
                overlayParent: root.hub.overlayItem
                labelFor: id => root.nacreLabel(id)
                lanes: [
                    { id: "left", title: I18n.tr("Left"), moveText: I18n.tr("Move to the left"), addText: I18n.tr("Add on the left"), items: root.nacre.islands.left },
                    { id: "center", title: I18n.tr("Middle"), moveText: I18n.tr("Move to the middle"), addText: I18n.tr("Add in the middle"), items: root.nacre.islands.center },
                    { id: "right", title: I18n.tr("Right"), moveText: I18n.tr("Move to the right"), addText: I18n.tr("Add on the right"), items: root.nacre.islands.right }
                ]
                spare: NacreConfig.unused(root.nacre)
                spareTitle: I18n.tr("Not on the bar")
                spareEmpty: I18n.tr("Every widget is on the bar")
                onMoved: (widgetId, fromLane, fromIndex, toLane, toIndex) => root.moveNacre(widgetId, fromLane, fromIndex, toLane, toIndex)
            }
        }
    }

    Component {
        id: railBody
        Column {
            spacing: LithicTokens.space.s2
            Label {
                width: parent.width
                text: I18n.tr("Widgets on the edge bar, from the top of the screen down. Drag them between the parts, or use each widget's menu.")
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            LaneEditor {
                width: parent.width
                overlayParent: root.hub.overlayItem
                labelFor: id => root.railLabel(id)
                lanes: [
                    { id: "top", title: I18n.tr("Top"), moveText: I18n.tr("Move to the top"), addText: I18n.tr("Add at the top"), items: root.frames.rails.left.top },
                    { id: "center", title: I18n.tr("Middle"), moveText: I18n.tr("Move to the middle"), addText: I18n.tr("Add in the middle"), items: root.frames.rails.left.center },
                    { id: "bottom", title: I18n.tr("Bottom"), moveText: I18n.tr("Move to the bottom"), addText: I18n.tr("Add at the bottom"), items: root.frames.rails.left.bottom }
                ]
                spare: BarCatalog.ids().filter(id => {
                    var e = BarCatalog.entry(id);
                    var rail = root.frames.rails.left;
                    return e && e.axes.indexOf("vertical") >= 0 && rail.top.concat(rail.center, rail.bottom).indexOf(id) < 0;
                })
                spareTitle: I18n.tr("Not on the edge bar")
                spareEmpty: I18n.tr("Every widget that fits is on the edge bar")
                onMoved: (widgetId, fromLane, fromIndex, toLane, toIndex) => root.moveRail(widgetId, fromLane, fromIndex, toLane, toIndex)
            }
        }
    }

    Component {
        id: noOptionsBody
        Label {
            text: I18n.tr("%1 has no options of its own. Its look follows your colours and the settings on the other pages.").arg(root.activeName)
            typeRole: "detail"
            inkRole: "fg3"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    Component {
        id: partsBody
        Slab {
            Column {
                width: parent.width

                HandoffRow {
                    width: parent.width
                    first: true
                    glyph: "arrow_forward"
                    text: I18n.tr("Dock")
                    detail: root.partDetail(I18n.tr("Choose the dock and edit it on Desktop & dock"), "dock")
                    onTriggered: root.hub.navigate("desktop-dock", "dock-design")
                }
                HandoffRow {
                    width: parent.width
                    first: false
                    glyph: "arrow_forward"
                    text: I18n.tr("Desktop widgets")
                    detail: root.partDetail(I18n.tr("Place and arrange widgets on Desktop & dock"), "desktopWidgets")
                    onTriggered: root.hub.navigate("desktop-dock", "edit-widgets")
                }
                HandoffRow {
                    width: parent.width
                    first: false
                    glyph: "arrow_forward"
                    text: I18n.tr("App launcher")
                    detail: root.partDetail(I18n.tr("Choose the launcher and how it looks"), "launcher")
                    onTriggered: root.hub.navigate("launcher", "")
                }
            }
        }
    }
}

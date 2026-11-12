import QtQuick
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/desktop-dock"
import "../schema/desktop-dock.js" as Schema
import "../ShellIndex.js" as ShellIndex
import ".."

// the desktop's own parts: the editor hand-offs, widgets per display, the dock,
// and what shows while the desktop reloads
Item {
    id: root

    required property var hub
    property string pendingRow: ""
    property Item reloadBody: null
    property var storeDocks: []
    // every dock RyoStore knows by name, installed or not
    property var storeNames: ({})

    function screenRow(screen) {
        var name = String(screen.name || "");
        var model = String(screen.model || "");
        var label = model.length > 0 ? model : name;
        return {
            id: "show-widgets-" + name, section: "widgets", label: label,
            changeLabel: I18n.tr("Widgets on %1").arg(label),
            detail: I18n.tr("%1, %2 × %3").arg(name).arg(screen.width).arg(screen.height),
            control: "toggle", bind: { store: "settings", key: "displays.widgets." + name, fallback: true }, "default": true
        };
    }

    readonly property var displayRows: Quickshell.screens.filter(s => String(s.name || "") !== "").map(s => root.screenRow(s))

    // a stored dock that nothing here provides stays listed for the whole visit,
    // so choosing another one does not rebuild the row under the keyboard
    readonly property string currentDock: String(Changes.value(Schema.row("dock-design").bind) || "ryoku")
    property var strayDocks: []
    onCurrentDockChanged: root.keepStray()
    function keepStray() {
        var id = root.currentDock;
        if (root.strayDocks.indexOf(id) < 0 && !root.knownDocks().some(d => d.value === id))
            root.strayDocks = root.strayDocks.concat([id]);
    }

    function knownDocks() {
        var out = [{ label: "Ryoku", value: "ryoku" }, { label: "ii", value: "ii" }];
        var seen = { ryoku: true, ii: true };
        ShellIndex.contributions(shells.index, "dock").forEach(d => {
            if (seen[d.id])
                return;
            seen[d.id] = true;
            out.push({ label: d.name === d.itemName ? d.name : I18n.tr("%1 from %2").arg(d.name).arg(d.itemName), value: d.id });
        });
        root.storeDocks.forEach(d => {
            if (seen[d.id])
                return;
            seen[d.id] = true;
            out.push({ label: d.name, value: d.id });
        });
        return out;
    }

    // every dock that can draw: the two built in, those bar styles bring, and those installed from RyoStore
    readonly property var dockOptions: {
        var out = root.knownDocks();
        root.strayDocks.forEach(id => {
            if (!out.some(d => d.value === id))
                out.push({ label: id === "none" ? I18n.tr("None")
                    : root.storeNames[id] ? I18n.tr("%1 (not installed)").arg(root.storeNames[id])
                    : I18n.tr("A dock that is not installed"), value: id });
        });
        return out;
    }

    readonly property bool dockOn: Changes.value(Schema.row("dock-show").bind) === true

    // the active bar style may draw a dock of its own instead
    readonly property var dockOwner: {
        var item = ShellIndex.item(shells.index, String(Settings.get("barStyle") || shells.index.fallback || ""));
        if (!item || !Array.isArray(item.owns) || item.owns.indexOf("dock") < 0)
            return null;
        var key = String((item.ownsSetting || {}).dock || "");
        return key === "" || Settings.get(key) !== false ? item : null;
    }

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.filter(r => r.template !== true)
            .map(r => r.id === "dock-design" ? Object.assign({}, r, { options: root.dockOptions }) : r)
            .concat(root.displayRows)
    })

    readonly property string stateLine: {
        var off = root.displayRows.filter(r => Changes.value(r.bind) === false).length;
        var dock = root.dockOwner ? I18n.tr("%1 draws its own dock.").arg(root.dockOwner.name || root.dockOwner.id)
            : root.dockOn ? I18n.tr("The dock is on.") : I18n.tr("The dock is off.");
        var widgets = off === 0 ? I18n.tr("Widgets show on every display.")
            : off === root.displayRows.length ? I18n.tr("Widgets are off on every display.")
            : off === 1 ? I18n.tr("Widgets are off on 1 display.") : I18n.tr("Widgets are off on %1 displays.").arg(off);
        return widgets + " " + dock;
    }

    function own(id) {
        var r = Schema.row(id);
        return !!r && r.template === true;
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (rowId === "show-widgets") {
            if (root.displayRows.length > 0)
                page.focusRow(root.displayRows[0].id);
            return;
        }
        if (!root.own(rowId)) {
            page.focusRow(rowId);
            return;
        }
        root.pendingRow = rowId;
        landRetry.tries = 0;
        landRetry.restart();
    }

    // the reload rows sit under More options; open it, wait for it to unfold, then land
    Timer {
        id: landRetry
        property int tries: 0
        interval: 60
        repeat: true
        onTriggered: {
            var id = root.pendingRow;
            if (root.reloadBody && !root.reloadBody.unfolded) {
                root.reloadBody.openMore();
                interval = LithicTokens.motion.unfold + 40;
                return;
            }
            interval = 60;
            if (page.rowItems[id]) {
                stop();
                root.pendingRow = "";
                page.land(id);
            } else if (++tries > 30) {
                stop();
            }
        }
    }

    ShellIndexFile { id: shells }

    Process {
        id: dockCatalogue
        command: ["ryostore", "catalog", "--category", "docks"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var docks = (JSON.parse(this.text || "{}").items || []).filter(i => i.category === "docks");
                    var names = {};
                    docks.forEach(i => names[String(i.id)] = String(i.name || i.id));
                    root.storeNames = names;
                    root.storeDocks = docks.filter(i => i.installed === true).map(i => ({ id: String(i.id), name: String(i.name || i.id) }));
                } catch (e) {
                    root.storeDocks = [];
                }
            }
        }
    }

    Component.onCompleted: {
        Changes.ensureStore("reloadcover", Qt.resolvedUrl("../components/desktop-dock/ReloadCoverStore.qml"));
        root.keepStray();
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "desktop-dock"
        schema: root.schema
        stateLine: root.stateLine
        bodies: ({ reload: reloadSection })
        footers: ({ dock: root.dockOwner ? ownedDock : null })
    }

    Component {
        id: ownedDock
        Label {
            text: I18n.tr("%1, the bar style in use, draws its own dock, so these settings wait until you use a style without one.").arg(root.dockOwner ? (root.dockOwner.name || root.dockOwner.id) : "")
            typeRole: "detail"
            inkRole: "fg3"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    Component {
        id: reloadSection
        ReloadCover {
            id: cover
            readonly property bool unfolded: cover.opened
            host: page
            Component.onCompleted: root.reloadBody = cover
            Component.onDestruction: if (root.reloadBody === cover) root.reloadBody = null
        }
    }
}

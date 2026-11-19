import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

Column {
    id: root
    readonly property bool wide: true
    property var row: null
    property var value: []
    property var commit: null
    property var store: null
    property var windows: []
    property var request: null
    property bool adding: false
    property string error: ""
    property int generation: 0
    readonly property bool busy: root.request !== null
    readonly property bool editing: root.adding
    readonly property bool live: root.visible && root.enabled && (!root.store || root.store.active !== false)
    readonly property var apps: Array.from(root.value || []).filter((id, index, all) => id && all.indexOf(id) === index)
    readonly property var available: root.windows.filter(app => root.apps.indexOf(app.id) < 0)
    width: LithicTokens.size.settingsControlMax
    spacing: LithicTokens.space.s2

    function remove(id) {
        if (root.commit)
            root.commit(root.apps.filter(app => app !== id));
    }
    function add(id) {
        if (root.apps.indexOf(id) < 0 && root.commit)
            root.commit(root.apps.concat([id]));
        root.adding = false;
    }
    function openApps() {
        if (!root.live || root.busy)
            return;
        root.adding = true;
        root.error = "";
        var version = ++root.generation;
        root.request = Changes.run(["ryoku", "wm", "state"], (ok, out, err) => {
            if (version !== root.generation)
                return;
            root.request = null;
            if (!root.live)
                return;
            if (!ok) {
                root.error = I18n.tr("Running apps could not be read") + (err ? ": " + String(err).trim() : "");
                return;
            }
            try {
                var frame = JSON.parse(out);
                var seen = ({});
                var apps = [];
                Array.from(frame.windows || []).forEach(window => {
                    var id = String(window.appId || "").trim();
                    if (!id || Object.prototype.hasOwnProperty.call(seen, id))
                        return;
                    seen[id] = true;
                    apps.push({ id: id, title: String(window.title || "") });
                });
                apps.sort((a, b) => a.id.localeCompare(b.id));
                root.windows = apps;
            } catch (e) {
                root.error = I18n.tr("The running app list could not be read");
            }
        }, { timeout: 10000 });
    }
    function closeApps() {
        ++root.generation;
        var pending = root.request;
        root.request = null;
        root.adding = false;
        if (pending)
            pending.running = false;
    }
    onLiveChanged: {
        if (!root.live)
            root.closeApps();
    }
    Component.onDestruction: root.closeApps()

    Repeater {
        model: root.apps
        delegate: CutRect {
            id: appRow
            required property string modelData
            width: Math.min(root.width, LithicTokens.size.settingsFieldMax)
            height: Math.max(40, appName.implicitHeight + 2 * LithicTokens.space.s2)
            cut: Tone.rowCut
            fill: Lithic.field
            strokeColor: Lithic.edge
            strokeWidth: LithicTokens.size.hairline
            Label {
                id: appName
                anchors.left: parent.left
                anchors.leftMargin: LithicTokens.space.s2
                anchors.right: removeButton.left
                anchors.rightMargin: LithicTokens.space.s2
                anchors.verticalCenter: parent.verticalCenter
                text: appRow.modelData
                typeRole: "monoSm"
                elide: Text.ElideRight
            }
            IconPlaque {
                id: removeButton
                anchors.right: parent.right
                anchors.rightMargin: LithicTokens.space.s1
                anchors.verticalCenter: parent.verticalCenter
                small: true
                kind: "quiet"
                glyph: "close"
                accessibleName: I18n.tr("Remove %1 from quiet places").arg(appRow.modelData)
                onClicked: root.remove(appRow.modelData)
            }
        }
    }
    Label {
        visible: root.apps.length === 0
        width: parent.width
        text: I18n.tr("No apps are kept quiet")
        typeRole: "detail"
        inkRole: "fg3"
    }
    Plaque {
        small: true
        kind: "line"
        glyph: "add"
        text: I18n.tr("Add an app")
        busy: root.live && root.busy
        enabled: !root.busy
        onClicked: root.openApps()
    }
    Column {
        visible: root.adding
        width: Math.min(parent.width, LithicTokens.size.settingsFieldMax)
        spacing: LithicTokens.space.s2
        Repeater {
            model: root.available
            delegate: Column {
                id: candidate
                required property var modelData
                width: parent.width
                spacing: LithicTokens.space.s1
                Plaque {
                    width: parent.width
                    small: true
                    kind: "quiet"
                    glyph: "add"
                    text: candidate.modelData.id
                    onClicked: root.add(candidate.modelData.id)
                }
                Label {
                    visible: candidate.modelData.title !== ""
                    width: parent.width
                    text: candidate.modelData.title
                    typeRole: "detail"
                    inkRole: "fg3"
                    elide: Text.ElideRight
                }
            }
        }
        Label {
            visible: !root.busy && root.error === "" && root.available.length === 0
            width: parent.width
            text: I18n.tr("No other running apps to add")
            typeRole: "detail"
            inkRole: "fg3"
        }
        Plaque {
            small: true
            kind: "quiet"
            glyph: "close"
            text: I18n.tr("Close app list")
            onClicked: root.closeApps()
        }
    }
    Label {
        visible: root.error !== ""
        width: parent.width
        text: root.error
        typeRole: "detail"
        ink: Lithic.jasper
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}

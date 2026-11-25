import QtQuick
import QtQuick.Window
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/screen-capture"
import "../schema/hyakki.js" as Schema

Item {
    id: root

    required property var hub
    property string pendingRow: ""
    readonly property var store: Changes.stores["hyakki"] || null
    readonly property bool loaded: root.store !== null && root.store.loaded
    readonly property bool switching: root.store !== null && root.store.switching !== ""
    readonly property bool windowActive: Window.active
    readonly property string stateLine: root.loaded && root.store.version !== ""
        ? I18n.tr("Hyakki %1").arg(root.store.version) : ""
    readonly property string statusLine: !root.loaded ? I18n.tr("Reading Hyakki's status.")
        : root.switching ? (root.store.switching === "on" ? I18n.tr("Turning Hyakki on") : I18n.tr("Turning Hyakki off and removing it"))
        : !root.store.installed ? I18n.tr("Not installed. Turning it on installs Hyakki and asks for your password")
        : !root.store.enabled ? I18n.tr("Off. Hyakki runs nothing until you turn it on")
        : root.store.running ? I18n.tr("On and running now; it stops by itself when idle")
        : I18n.tr("On. It starts when you ask something and stops again when idle")

    readonly property var schema: ({
        title: Schema.title, lead: Schema.lead, sections: Schema.sections, related: Schema.related,
        rows: Schema.rows.map(function (row) {
            if (row.id === "hyakki-enabled")
                return Object.assign({}, row, { detail: root.statusLine });
            if (row.command && (!root.loaded || !root.store.enabled || root.switching))
                return Object.assign({}, row, { unavailable: I18n.tr("Turn Hyakki on to open its native settings and app") });
            return row;
        })
    })

    function focusRow(id) {
        if (!id)
            return;
        if (!root.loaded || page.layout.length === 0) {
            root.pendingRow = id;
            return;
        }
        root.pendingRow = "";
        page.focusRow(id);
    }

    function revisited() {
        if (root.store && !root.switching)
            root.store.refresh();
    }

    function request(on) {
        if (!root.visible || !root.loaded || root.switching || root.store.readError !== "")
            return;
        if (on) {
            root.store.turn(true);
            return;
        }
        root.hub.confirm({
            title: I18n.tr("Turn Hyakki off?"),
            text: I18n.tr("Hyakki stops and its app is removed. Your conversations and settings stay."),
            actionText: I18n.tr("Turn off")
        }, function () {
            if (root.visible && !root.switching)
                root.store.turn(false);
        });
    }

    onWindowActiveChanged: if (root.windowActive && root.visible) root.revisited()
    onVisibleChanged: if (root.visible) root.revisited()
    onLoadedChanged: if (root.loaded && root.pendingRow !== "") Qt.callLater(root.focusRow, root.pendingRow)
    Component.onCompleted: {
        var had = !!Changes.stores["hyakki"];
        Changes.ensureStore("hyakki", Qt.resolvedUrl("../components/hyakki/HyakkiStore.qml"));
        if (had)
            root.revisited();
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "hyakki"
        schema: root.schema
        stateLine: root.stateLine
        controls: ({ "hyakki-enabled": switchControl })
        footers: ({ hyakki: root.store && (root.store.message !== "" || root.store.readError !== "") ? statusFooter : null })
        onLayoutChanged: if (root.loaded && root.pendingRow !== "") Qt.callLater(root.focusRow, root.pendingRow)
    }

    Component {
        id: switchControl
        Row {
            property var row: null
            function activate() {
                if (root.loaded)
                    root.request(!root.store.enabled);
            }
            spacing: LithicTokens.space.s2
            Working {
                anchors.verticalCenter: parent.verticalCenter
                visible: !root.loaded || (root.switching && switchClock.working)
                active: visible
            }
            BladeProgress {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.switching && switchClock.slow
                width: 96
                active: visible
                indeterminate: true
            }
            Plaque {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.switching && switchClock.slow
                enabled: root.store !== null && !root.store.cancelRequested
                kind: "quiet"
                small: true
                text: root.store && root.store.cancelRequested ? I18n.tr("Stopping") : I18n.tr("Cancel")
                onClicked: root.store.cancel()
            }
            Toggle {
                id: toggle
                anchors.verticalCenter: parent.verticalCenter
                enabled: root.loaded && !root.switching && root.store.readError === ""
                checked: root.switching ? root.store.switching === "on" : root.loaded && root.store.enabled
                Accessible.name: I18n.tr("Use Hyakki")
                onToggled: {
                    var want = toggle.checked;
                    toggle.checked = Qt.binding(() => root.switching ? root.store.switching === "on" : root.loaded && root.store.enabled);
                    root.request(want);
                }
            }
        }
    }

    Component {
        id: statusFooter
        Row {
            width: parent ? parent.width : implicitWidth
            spacing: LithicTokens.space.s3
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.max(0, parent.width - retry.width - parent.spacing)
                text: root.store.message || root.store.readError
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: retry
                anchors.verticalCenter: parent.verticalCenter
                enabled: !root.switching
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: {
                    if (root.store.message !== "" && root.store.readError === "")
                        root.request(root.store.lastAsk === "on");
                    else
                        root.store.refresh();
                }
            }
        }
    }

    BusyClock {
        id: switchClock
        running: root.visible && root.switching
    }
}

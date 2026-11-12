import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/screen-capture"
import "../schema/rashin.js" as Schema

// turning Rashin on or off, what it is doing, and opening it; reading its status
// never starts it, so this page costs Rashin nothing while it is off
Item {
    id: root

    required property var hub
    property string pendingRow: ""

    readonly property var store: Changes.stores["rashin"] || null
    readonly property bool loaded: root.store !== null && root.store.loaded
    readonly property string state: root.loaded ? root.store.state : ""
    readonly property bool switching: root.store !== null && root.store.switching !== ""

    function routeText() {
        var r = root.store.askRoute;
        if (r === "auto")
            return I18n.tr("Picks a free model for each question");
        if (r === "auto:fast")
            return I18n.tr("Picks the fastest free model");
        if (r === "auto:smart")
            return I18n.tr("Picks the strongest free model");
        return I18n.tr("Uses %1").arg(r);
    }

    function enabledDetail() {
        var s = root.store;
        if (s.switching === "on")
            return s.installed ? I18n.tr("Turning Rashin on")
                               : I18n.tr("Installing Rashin. Enter your password when asked; this can take a minute");
        if (s.switching === "off")
            return I18n.tr("Turning Rashin off and removing it");
        switch (root.state) {
        case "absent": return I18n.tr("Not installed. Turning it on installs Rashin and asks for your password");
        case "off": return I18n.tr("Off. Rashin runs nothing at all until you turn it on");
        case "idle": return I18n.tr("On. It starts when you ask something and stops again when idle");
        case "running": return I18n.tr("On and running now; it stops by itself when idle");
        }
        return "";
    }

    function prepared(row) {
        var out = Object.assign({}, row);
        var s = root.store;
        switch (row.id) {
        case "rashin-enabled":
            out.detail = root.enabledDetail();
            break;
        case "status-ask":
            out.detail = s.running ? I18n.tr("%1. %2 free providers ready").arg(root.routeText()).arg(s.providers) : root.routeText();
            break;
        case "status-agent":
            var who = s.harness !== "" ? s.harness.charAt(0).toUpperCase() + s.harness.slice(1) : I18n.tr("The first coding agent it finds");
            out.detail = s.running ? I18n.tr("%1. %2 coding agents installed").arg(who).arg(s.harnesses) : who;
            break;
        case "status-index":
            out.detail = s.running ? I18n.tr("%1 indexes of your system and projects").arg(s.indexes)
                                   : I18n.tr("Your system and projects; the counts show while Rashin runs");
            break;
        }
        return out;
    }

    function shown(row) {
        if (!root.loaded)
            return false;
        if (row.section === "status")
            return root.store.installed;
        if (row.section === "open")
            return root.store.installed && root.store.enabled && !root.switching;
        return true;
    }

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.filter(r => root.shown(r)).map(r => root.prepared(r))
    })

    readonly property string stateLine: root.loaded && root.store.installed && root.store.version !== ""
        ? I18n.tr("Rashin %1").arg(root.store.version) : ""

    function request(on) {
        if (!root.store || root.switching)
            return;
        if (on) {
            root.store.turn(true);
            return;
        }
        root.hub.confirm({
            title: I18n.tr("Turn Rashin off?"),
            text: I18n.tr("Rashin stops, its app is removed and nothing of it runs until you turn it on again. Your chats and settings stay."),
            actionText: I18n.tr("Turn off")
        }, function () { root.store.turn(false); });
    }

    function sectionIndex(id) {
        for (var i = 0; i < page.layout.length; i++)
            if (page.layout[i].id === id)
                return i;
        return -1;
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (page.layout.length === 0 || !root.loaded) {
            root.pendingRow = rowId;
            return;
        }
        // a drawn row or section is the frame's to land on
        if (page.landingRow(rowId) !== "" || page.layout.some(s => s.id === rowId)) {
            page.focusRow(rowId);
            return;
        }
        // a status or open row or section of a Rashin that is off lands on the switch
        page.focusRow("rashin-enabled");
    }

    Component.onCompleted: {
        var had = !!Changes.stores["rashin"];
        Changes.ensureStore("rashin", Qt.resolvedUrl("../components/rashin/RashinStore.qml"));
        if (had)
            Changes.stores["rashin"].refresh();
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "rashin"
        schema: root.schema
        stateLine: root.stateLine
        bodies: ({ rashin: root.loaded ? null : loadingBody })
        // a switch that failed says so first; a status re-read that failed says the page may be out of date
        footers: ({ rashin: root.store && root.store.error !== "" ? errorFooter
                          : root.loaded && root.store.readError !== "" ? readFooter : null })
        controls: ({ "rashin-enabled": switchControl })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    Component {
        id: loadingBody
        Column {
            spacing: LithicTokens.space.s2
            Row {
                visible: !root.store || root.store.readError === ""
                spacing: LithicTokens.space.s2
                Working {
                    anchors.verticalCenter: parent.verticalCenter
                    active: parent.visible
                }
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Reading Rashin's status.")
                    typeRole: "body"
                    inkRole: "fg2"
                }
            }
            Label {
                visible: root.store !== null && root.store.readError !== ""
                width: parent.width
                text: root.store ? root.store.readError + " " + I18n.tr("Check that the rashin command works in a terminal, then try again.") : ""
                typeRole: "body"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                visible: root.store !== null && root.store.readError !== ""
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: root.store.refresh()
            }
        }
    }

    Component {
        id: switchControl
        Row {
            id: ctl
            property var row: null
            function activate() {
                if (root.store && !root.switching)
                    root.request(!root.store.enabled);
            }
            spacing: LithicTokens.space.s2
            Working {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.switching && switchClock.working
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
                enabled: !root.store.cancelRequested
                kind: "quiet"
                small: true
                text: root.store && root.store.cancelRequested ? I18n.tr("Stopping") : I18n.tr("Cancel")
                onClicked: root.store.cancel()
            }
            Toggle {
                id: toggle
                anchors.verticalCenter: parent.verticalCenter
                enabled: !root.switching
                checked: root.switching ? root.store.switching === "on" : (root.store !== null && root.store.enabled)
                Accessible.name: I18n.tr("Use Rashin")
                onToggled: {
                    var want = toggle.checked;
                    toggle.checked = Qt.binding(() => root.switching ? root.store.switching === "on" : (root.store !== null && root.store.enabled));
                    root.request(want);
                }
            }
        }
    }

    Component {
        id: errorFooter
        Row {
            spacing: LithicTokens.space.s3
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, parent.width - retrySwitch.width - LithicTokens.space.s3)
                text: root.store.message
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: retrySwitch
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: root.store.turn(root.store.lastAsk === "on")
            }
        }
    }

    Component {
        id: readFooter
        Row {
            spacing: LithicTokens.space.s3
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, parent.width - retryRead.width - LithicTokens.space.s3)
                text: root.store.readError + " " + I18n.tr("What this page shows may be out of date. Check that the rashin command works in a terminal, then try again.")
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: retryRead
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: root.store.refresh()
            }
        }
    }

    BusyClock {
        id: switchClock
        running: root.switching
    }
}

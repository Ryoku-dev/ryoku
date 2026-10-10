pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

// Remotes: the distant fleet. Saved SSH hosts and VPS from ~/.ssh/config and
// ryoport's own book, each carrying a live state and, on demand, a full health
// probe. The list is the console; a berth opens on the right. A PuTTY that also
// tells you how the box is doing.
Item {
    id: rem

    property bool active: false
    signal newRemote()
    signal editRemote(string alias)

    // keyboard-first, like a real terminal client: arrows walk the list, Enter
    // drops into a session. Filtering typing lands in the search field itself.
    Keys.onUpPressed: rem.moveSelection(-1)
    Keys.onDownPressed: rem.moveSelection(1)
    Keys.onReturnPressed: if (Remotes.selectedAlias.length > 0) Remotes.connect(Remotes.selectedAlias)
    Keys.onEnterPressed: if (Remotes.selectedAlias.length > 0) Remotes.connect(Remotes.selectedAlias)
    function moveSelection(dir) {
        var list = rem.shown;
        if (list.length === 0) return;
        var idx = -1;
        for (var i = 0; i < list.length; i++)
            if (list[i].alias === Remotes.selectedAlias) { idx = i; break; }
        idx = Math.max(0, Math.min(list.length - 1, idx + dir));
        Remotes.select(list[idx].alias);
    }
    Shortcut {
        sequences: ["/", "Ctrl+K"]
        enabled: rem.active
        onActivated: search.grabFocus()
    }

    // ---- head --------------------------------------------------------------
    PageHead {
        id: header
        anchors { top: parent.top; left: parent.left; right: parent.right }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6; anchors.topMargin: Tokens.s5
        eyebrow: I18n.tr("FLEET")
        title: I18n.tr("Remotes")
        blurb: I18n.tr("Saved hosts and VPS, reachable at a glance. Connect in a tap, or read a live health probe before you do.")
    }

    // ---- toolbar -----------------------------------------------------------
    Item {
        id: toolbar
        anchors { top: header.bottom; left: parent.left; right: parent.right }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6; anchors.topMargin: Tokens.s3
        height: 40

        Field {
            id: search
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: 280
            toolbar: true
            placeholder: I18n.tr("Filter hosts")
            onEdited: (v) => rem.query = v
        }

        Row {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: String(Remotes.upCount).padStart(2, "0") + I18n.tr(" UP · ") + String(Remotes.hostCount).padStart(2, "0") + I18n.tr(" HOSTS")
                color: Tokens.inkMuted
                font.family: Tokens.mono; font.pixelSize: 10; font.letterSpacing: 1.2
            }
            Btn { anchors.verticalCenter: parent.verticalCenter; text: I18n.tr("PROBE ALL"); onAct: Remotes.probeAll() }
            Btn { anchors.verticalCenter: parent.verticalCenter; text: I18n.tr("NEW"); primary: true; onAct: rem.newRemote() }
        }
    }

    property string query: ""
    function match(h) {
        if (rem.query.length === 0) return true;
        var q = rem.query.toLowerCase();
        return (h.alias || "").toLowerCase().indexOf(q) >= 0
            || (h.hostName || "").toLowerCase().indexOf(q) >= 0
            || (h.group || "").toLowerCase().indexOf(q) >= 0;
    }
    readonly property var shown: {
        var out = [];
        for (var i = 0; i < Remotes.hosts.length; i++)
            if (rem.match(Remotes.hosts[i])) out.push(Remotes.hosts[i]);
        return out;
    }

    // ---- body: list left, eye-candy + keys right ---------------------------
    Item {
        id: main
        anchors { top: toolbar.bottom; left: parent.left; right: parent.right; bottom: parent.bottom }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s4; anchors.bottomMargin: Tokens.s5

        readonly property real gCol: (width - (Spans.cols - 1) * Tokens.s2) / Spans.cols
        readonly property real leftW: 6 * gCol + 5 * Tokens.s2
        readonly property int seamW: Tokens.s5

        Item {
            id: leftCol
            anchors { left: parent.left; top: parent.top; bottom: parent.bottom }
            width: main.leftW

            Flickable {
                anchors.fill: parent
                contentHeight: grid.implicitHeight + Tokens.s3
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }

                Column {
                    id: grid
                    width: parent.width
                    spacing: Tokens.s2
                    Repeater {
                        model: rem.shown
                        RemoteTile {
                            required property var modelData
                            width: grid.width
                            host: modelData
                            onTapped: Remotes.select(modelData.alias)
                            onConnect: Remotes.connect(modelData.alias)
                        }
                    }
                }
            }

            Empty {
                anchors.centerIn: parent
                width: parent.width
                visible: rem.shown.length === 0 && !Remotes.loading
                caption: !Remotes.engineOk
                    ? I18n.tr("The remote engine (ryossh) is not installed. Build it from ryoku/apps/ryovm/remote.")
                    : Remotes.hosts.length === 0
                        ? I18n.tr("No remotes yet. Add a VPS with NEW, or drop hosts in ~/.ssh/config.")
                        : I18n.tr("No host matches that filter.")
            }
        }

        Rectangle {
            anchors.left: leftCol.right
            anchors.leftMargin: main.seamW / 2
            anchors { top: parent.top; bottom: parent.bottom }
            anchors.topMargin: Tokens.s2; anchors.bottomMargin: Tokens.s2
            width: 1
            color: Tokens.line
        }

        Item {
            id: rightCol
            anchors.left: leftCol.right
            anchors.leftMargin: main.seamW
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.bottom: parent.bottom

            RemoteDetail {
                anchors.fill: parent
                alias: Remotes.selectedAlias
                opacity: Remotes.selectedAlias.length > 0 ? 1 : 0
                visible: opacity > 0
                Behavior on opacity { NumberAnimation { duration: Tokens.swap } }
                onEdit: (a) => rem.editRemote(a)
            }

            // nothing berthed: the poster and the key toolkit fill the column.
            Column {
                anchors.fill: parent
                spacing: Tokens.s4
                opacity: Remotes.selectedAlias.length === 0 ? 1 : 0
                visible: opacity > 0
                Behavior on opacity { NumberAnimation { duration: Tokens.swap } }

                Rectangle {
                    width: parent.width
                    height: 150
                    radius: Tokens.radius
                    color: "transparent"
                    border.width: Tokens.border
                    border.color: Tokens.line
                    Ticks { color: Tokens.line }

                    Column {
                        anchors.fill: parent
                        anchors.margins: Tokens.s4
                        spacing: Tokens.s2
                        Row {
                            spacing: Tokens.s2
                            Text { text: "//"; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro }
                            Text {
                                text: I18n.tr("KEYS_"); color: Tokens.ink
                                font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                                font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                            }
                            Text {
                                text: "鍵"; color: Tokens.inkFaint; font.family: Tokens.jp; font.pixelSize: 12
                                anchors.verticalCenter: parent.verticalCenter
                            }
                        }
                        Text {
                            text: (Remotes.keysData.agent ? Remotes.keysData.agent.length : 0) + I18n.tr(" in the agent · ")
                                + (Remotes.keysData.files ? Remotes.keysData.files.length : 0) + I18n.tr(" on disk")
                            color: Tokens.inkMuted
                            font.family: Tokens.mono; font.pixelSize: 11
                        }
                        Repeater {
                            model: Remotes.keysData.files ? Remotes.keysData.files.slice(0, 3) : []
                            Text {
                                required property var modelData
                                width: rightCol.width - Tokens.s5
                                elide: Text.ElideRight
                                text: (modelData.type || I18n.tr("KEY")) + "  " + (modelData.comment || modelData.path || "")
                                color: Tokens.inkFaint
                                font.family: Tokens.mono; font.pixelSize: 10
                            }
                        }
                    }
                }
            }
        }
    }

    Component.onCompleted: Remotes.loadKeys()
}

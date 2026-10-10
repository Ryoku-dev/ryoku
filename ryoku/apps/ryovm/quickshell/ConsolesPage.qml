pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

// Consoles: the experimental berths. Serial cables, telnet logins, RDP desktops
// and VNC screens, each opened through a mature open-source client -- picocom,
// inetutils telnet, FreeRDP, TigerVNC -- so ryoport stays the harbour and the
// protocol clients stay the ships. Profiles live in ryoport's own book; any
// password lives only in the login keyring and is handed to the child at
// launch, never written down.
Item {
    id: con

    property bool active: false
    property string selectedId: ""
    property string askPwId: ""          // set while a SAVE PASSWORD field is open
    signal newConsole(string kind)
    signal editConsole(string id)

    readonly property var kinds: [
        { key: "serial", name: I18n.tr("Serial"), jp: "直列", hint: I18n.tr("console cable over /dev/ttyUSB*"), tool: "picocom", pkg: "pacman -S picocom" },
        { key: "telnet", name: I18n.tr("Telnet"), jp: "端末", hint: I18n.tr("legacy network consoles (plaintext)"), tool: "telnet", pkg: "pacman -S inetutils" },
        { key: "rdp", name: I18n.tr("RDP"), jp: "遠隔", hint: I18n.tr("Windows remote desktop"), tool: "xfreerdp", pkg: "pacman -S freerdp" },
        { key: "vnc", name: I18n.tr("VNC"), jp: "画面", hint: I18n.tr("VNC screens on any port"), tool: "vncviewer", pkg: "pacman -S tigervnc" }
    ]
    readonly property var selected: {
        void Remotes.consolesRev;
        for (var i = 0; i < Remotes.consoles.length; i++)
            if (Remotes.consoles[i].id === selectedId) return Remotes.consoles[i];
        return null;
    }

    // keyboard-first, like the remotes page: arrows walk the list, Enter opens.
    Keys.onUpPressed: moveSelection(-1)
    Keys.onDownPressed: moveSelection(1)
    Keys.onReturnPressed: if (selected !== null) Remotes.openConsole(selectedId)
    Keys.onEnterPressed: if (selected !== null) Remotes.openConsole(selectedId)
    function moveSelection(dir) {
        var list = Remotes.consoles;
        if (list.length === 0) return;
        var idx = 0;
        for (var i = 0; i < list.length; i++)
            if (list[i].id === selectedId) { idx = i; break; }
        idx = Math.max(0, Math.min(list.length - 1, idx + dir));
        selectedId = list[idx].id;
    }

    PageHead {
        id: header
        anchors { top: parent.top; left: parent.left; right: parent.right }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6; anchors.topMargin: Tokens.s5
        eyebrow: I18n.tr("EXPERIMENTAL")
        title: I18n.tr("Consoles")
        blurb: I18n.tr("Serial, telnet, RDP, and VNC berths opened through proven open-source clients. Passwords stay in your keyring.")
    }

    Item {
        id: body
        anchors { top: header.bottom; left: parent.left; right: parent.right; bottom: parent.bottom }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s4; anchors.bottomMargin: Tokens.s5

        // ---- left: saved berths ---------------------------------------------
        Item {
            id: leftCol
            anchors { left: parent.left; top: parent.top; bottom: parent.bottom }
            width: Math.max(300, Math.min(400, parent.width * 0.36))

            Text {
                id: listHead
                anchors { top: parent.top; left: parent.left; right: parent.right }
                text: I18n.tr("SAVED BERTHS")
                color: Tokens.inkMuted
                font.family: Tokens.ui; font.pixelSize: 9
                font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
            }
            ListView {
                id: berthList
                anchors { top: listHead.bottom; topMargin: Tokens.s2; left: parent.left; right: parent.right; bottom: newRow.top }
                anchors.bottomMargin: Tokens.s2
                clip: true
                model: Remotes.consoles
                spacing: Tokens.s2
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                delegate: Rectangle {
                    id: berth
                    required property var modelData
                    width: berthList.width
                    height: 58
                    color: con.selectedId === modelData.id ? Tokens.bone : "transparent"
                    border.width: Tokens.border
                    border.color: con.selectedId === modelData.id ? Tokens.bone : Tokens.lineSoft
                    antialiasing: false
                    Column {
                        anchors.left: parent.left
                        anchors.leftMargin: Tokens.s3
                        anchors.right: berthState.left
                        anchors.rightMargin: Tokens.s2
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 3
                        Text {
                            width: parent.width
                            elide: Text.ElideRight
                            text: berth.modelData.name
                            color: con.selectedId === berth.modelData.id ? Tokens.inkOnBone : Tokens.ink
                            font.family: Tokens.ui; font.pixelSize: 12
                            font.weight: Font.Medium
                        }
                        Text {
                            width: parent.width
                            elide: Text.ElideRight
                            text: Remotes.consoleTarget(berth.modelData)
                            color: con.selectedId === berth.modelData.id ? Tokens.inkOnBone : Tokens.inkMuted
                            opacity: 0.8
                            font.family: Tokens.mono; font.pixelSize: 9
                        }
                    }
                    Text {
                        id: berthState
                        anchors.right: parent.right
                        anchors.rightMargin: Tokens.s3
                        anchors.verticalCenter: parent.verticalCenter
                        text: Remotes.consoleStateOf(berth.modelData).toUpperCase()
                        color: Tokens.inkFaint
                        font.family: Tokens.mono; font.pixelSize: 8
                    }
                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: con.selectedId = berth.modelData.id
                        onDoubleClicked: Remotes.openConsole(berth.modelData.id)
                    }
                }
            }
            Row {
                id: newRow
                anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
                spacing: Tokens.s2
                Repeater {
                    model: con.kinds
                    Btn {
                        required property var modelData
                        text: "+ " + modelData.name.toUpperCase()
                        onAct: con.newConsole(modelData.key)
                    }
                }
            }
            Empty {
                anchors { top: listHead.bottom; topMargin: Tokens.s2; bottom: newRow.top; bottomMargin: Tokens.s2; left: parent.left; right: parent.right }
                visible: Remotes.consoles.length === 0
                caption: I18n.tr("No consoles yet. Pick a protocol on the right.")
            }
        }

        // ---- right: selected berth, or the protocol poster -------------------
        Item {
            id: rightCol
            anchors { left: leftCol.right; leftMargin: Tokens.s5; top: parent.top; right: parent.right; bottom: parent.bottom }

            Rectangle {
                anchors { left: parent.left; top: parent.top; bottom: parent.bottom }
                anchors.topMargin: Tokens.s2; anchors.bottomMargin: Tokens.s2
                width: 1
                color: Tokens.line
            }

            Column {
                id: berthPane
                anchors { left: parent.left; leftMargin: Tokens.s5; right: parent.right; top: parent.top }
                anchors.rightMargin: Tokens.s2
                spacing: Tokens.s4
                visible: con.selected !== null

                Row {
                    width: parent.width
                    spacing: Tokens.s3
                    Column {
                        width: parent.width - ann.width - openBtn.width - editBtn.width - Tokens.s3 * 3
                        spacing: 3
                        Text {
                            width: parent.width
                            elide: Text.ElideRight
                            text: con.selected ? con.selected.name : ""
                            color: Tokens.ink
                            font.family: Tokens.display; font.pixelSize: 26
                        }
                        Text {
                            text: con.selected ? Remotes.consoleTarget(con.selected) : ""
                            color: Tokens.inkMuted
                            font.family: Tokens.mono; font.pixelSize: 10
                        }
                    }
                    Annunciator {
                        id: ann
                        anchors.verticalCenter: parent.verticalCenter
                        label: con.selected ? Remotes.consoleStateOf(con.selected).toUpperCase() : ""
                        lit: con.selected !== null && Remotes.consoleStateOf(con.selected) === "up"
                        warn: con.selected !== null && Remotes.consoleStateOf(con.selected) === "down"
                        tileW: 74
                    }
                    Btn {
                        id: openBtn
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("OPEN")
                        primary: true
                        onAct: if (con.selected) Remotes.openConsole(con.selected.id)
                    }
                    Btn {
                        id: editBtn
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("EDIT")
                        onAct: if (con.selected) con.editConsole(con.selected.id)
                    }
                }

                // why the berth is dark, when it is dark
                Rectangle {
                    width: parent.width
                    visible: con.selected !== null && Remotes.consoleStateOf(con.selected) !== "up"
                    height: whyCol.implicitHeight + Tokens.s4 * 2
                    color: "transparent"
                    border.width: Tokens.border
                    border.color: Tokens.line
                    antialiasing: false
                    Column {
                        id: whyCol
                        anchors { fill: parent; margins: Tokens.s4 }
                        spacing: Tokens.s2
                        Text {
                            width: parent.width
                            text: con.selected
                                ? I18n.tr("client: %1").arg(Remotes.consoleClientOf(con.selected.kind))
                                : ""
                            color: Tokens.inkMuted
                            font.family: Tokens.mono; font.pixelSize: 10
                        }
                        Text {
                            width: parent.width
                            wrapMode: Text.WordWrap
                            text: {
                                if (!con.selected) return "";
                                var r = Remotes.consoleReachOf(con.selected.id);
                                if (!r) return I18n.tr("not probed yet");
                                if (r.up === true) return "";
                                return r.error || I18n.tr("the port did not answer");
                            }
                            color: Tokens.ink
                            font.family: Tokens.ui; font.pixelSize: 11
                        }
                        Row {
                            spacing: Tokens.s2
                            visible: con.selected !== null && con.selected.auth !== "password" && con.askPwId !== con.selected.id
                            Btn {
                                text: I18n.tr("SAVE PASSWORD")
                                onAct: con.askPwId = con.selected ? con.selected.id : ""
                            }
                        }
                        Row {
                            width: parent.width
                            spacing: Tokens.s2
                            visible: con.selected !== null && con.selected.auth !== "password" && con.askPwId === con.selected.id
                            Field {
                                id: pwField
                                width: parent.width - savePwBtn.width - cancelPwBtn.width - Tokens.s2 * 2
                                secret: true
                                tabular: true
                                placeholder: I18n.tr("password -> keyring (never a file)")
                                onAccepted: savePwBtn.saveIt()
                            }
                            Btn {
                                id: savePwBtn
                                text: I18n.tr("SAVE")
                                primary: true
                                armed: pwField.text.length > 0
                                function saveIt() {
                                    if (con.selected && pwField.text.length > 0) {
                                        Remotes.setConsolePass(con.selected.id, pwField.text);
                                        pwField.clear();
                                        con.askPwId = "";
                                    }
                                }
                                onAct: saveIt()
                            }
                            Btn {
                                id: cancelPwBtn
                                text: I18n.tr("CANCEL")
                                onAct: { con.askPwId = ""; }
                            }
                        }
                    }
                }

                // credential tell + teardown
                Row {
                    width: parent.width
                    spacing: Tokens.s2
                    visible: con.selected !== null
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: con.selected && con.selected.auth === "password"
                            ? I18n.tr("password saved in the keyring")
                            : I18n.tr("no saved password")
                        color: Tokens.inkFaint
                        font.family: Tokens.mono; font.pixelSize: 9
                    }
                    Item { width: parent.width - 340; height: 1 }
                    Btn {
                        visible: con.selected !== null && con.selected.auth === "password"
                        text: I18n.tr("FORGET")
                        compact: true
                        onAct: if (con.selected) Remotes.clearConsolePass(con.selected.id)
                    }
                    Btn {
                        text: I18n.tr("REMOVE")
                        compact: true
                        onAct: { if (con.selected) { Remotes.removeConsole(con.selected.id); con.selectedId = ""; } }
                    }
                }
            }

            // nothing berthed: the protocol poster fills the column.
            Column {
                anchors { left: parent.left; leftMargin: Tokens.s5; right: parent.right; top: parent.top }
                spacing: Tokens.s3
                visible: con.selected === null

                Text {
                    text: I18n.tr("PROTOCOLS_")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui; font.pixelSize: 9
                    font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                }
                Repeater {
                    model: con.kinds
                    Rectangle {
                        id: proto
                        required property var modelData
                        width: rightCol.width - Tokens.s5
                        height: 66
                        color: "transparent"
                        border.width: Tokens.border
                        border.color: Tokens.line
                        antialiasing: false
                        Ticks { color: Tokens.line }
                        Row {
                            anchors { fill: parent; leftMargin: Tokens.s4; rightMargin: Tokens.s3; topMargin: Tokens.s3; bottomMargin: Tokens.s3 }
                            spacing: Tokens.s3
                            Column {
                                width: parent.width - addBtn.width - Tokens.s3
                                spacing: 2
                                Row {
                                    spacing: Tokens.s2
                                    Text {
                                        text: proto.modelData.name
                                        color: Tokens.ink
                                        font.family: Tokens.ui; font.pixelSize: 12
                                        font.weight: Font.DemiBold
                                        anchors.verticalCenter: parent.verticalCenter
                                    }
                                    Text {
                                        text: proto.modelData.jp
                                        color: Tokens.inkFaint
                                        font.family: Tokens.jp; font.pixelSize: 11
                                        anchors.verticalCenter: parent.verticalCenter
                                    }
                                    Text {
                                        text: I18n.tr("CLIENT MISSING")
                                        visible: !Remotes.consoleCaps[proto.modelData.key]
                                        color: Tokens.ink
                                        font.family: Tokens.mono; font.pixelSize: 8
                                        anchors.verticalCenter: parent.verticalCenter
                                    }
                                }
                                Text {
                                    width: parent.width
                                    elide: Text.ElideRight
                                    text: proto.modelData.hint
                                    color: Tokens.inkMuted
                                    font.family: Tokens.ui; font.pixelSize: 10
                                }
                                Text {
                                    text: Remotes.consoleCaps[proto.modelData.key]
                                        ? proto.modelData.tool
                                        : proto.modelData.pkg
                                    color: Tokens.inkFaint
                                    font.family: Tokens.mono; font.pixelSize: 9
                                }
                            }
                            Btn {
                                id: addBtn
                                anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("NEW")
                                primary: true
                                onAct: con.newConsole(proto.modelData.key)
                            }
                        }
                    }
                }
                Text {
                    width: parent.width
                    wrapMode: Text.WordWrap
                    text: I18n.tr("Serial and telnet logins stay interactive in the terminal window; RDP and VNC passwords are fetched from your keyring at launch and never touch argv or a file.")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                }
            }
        }
    }

    Component.onCompleted: Remotes.loadConsoles()
    onActiveChanged: if (active) Remotes.loadConsoles()
}

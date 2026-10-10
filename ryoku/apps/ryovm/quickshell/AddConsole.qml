pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

// The console manifest: add or amend one berth. The shape follows what the
// protocol needs -- a serial berth is a device and a baud rate; a network berth
// is an address and a port; RDP adds user and domain. A password typed here
// goes straight through ryossh into the login keyring; the profile JSON never
// carries the secret, only the flag that one is saved.
Item {
    id: sheet

    property bool open: false
    property string editId: ""
    property string kind: "serial"       // the default tab when adding
    property bool hadPassword: false
    property bool clearPassword: false
    signal closed()

    anchors.fill: parent
    visible: opacity > 0.01
    opacity: open ? 1 : 0
    z: 100
    readonly property bool editing: editId.length > 0
    readonly property bool serial: kind === "serial"
    readonly property bool valid: nameF.text.trim().length > 0 && (
        serial ? devF.text.trim().indexOf("/dev/") === 0 && parseInt(baudF.text) >= 30
               : addrF.text.trim().length > 0 && !/\s/.test(addrF.text.trim()))

    onOpenChanged: if (open) sheet.prefill()
    function prefill() {
        var c = null;
        if (sheet.editing) c = Remotes.consoleById(sheet.editId);
        if (c) kind = c.kind;
        nameF.text = c ? (c.name || "") : "";
        devF.text = c && c.device ? c.device : "/dev/ttyUSB0";
        baudF.text = c && c.baud ? String(c.baud) : "115200";
        addrF.text = c && c.address ? c.address : "";
        portF.text = c && c.port ? String(c.port) : defaultPort();
        userF.text = c && c.user ? c.user : "";
        domainF.text = c && c.domain ? c.domain : "";
        fullToggle.checked = !!(c && c.fullscreen);
        pwF.text = "";
        sheet.hadPassword = !!(c && c.auth === "password");
        sheet.clearPassword = false;
        Qt.callLater(nameF.grabFocus);
    }
    function defaultPort() {
        if (kind === "telnet") return "23";
        if (kind === "rdp") return "3389";
        if (kind === "vnc") return "5900";
        return "0";
    }
    function save() {
        if (!sheet.valid) return;
        var obj = {
            id: sheet.editing ? sheet.editId : "",
            kind: kind,
            name: nameF.text.trim()
        };
        if (sheet.serial) {
            obj.device = devF.text.trim();
            obj.baud = parseInt(baudF.text) || 115200;
        } else {
            obj.address = addrF.text.trim();
            obj.port = parseInt(portF.text) || parseInt(defaultPort());
            obj.user = userF.text.trim();
            if (kind === "rdp") obj.domain = domainF.text.trim();
            obj.fullscreen = fullToggle.checked;
        }
        Remotes.addConsole(obj, pwF.text, sheet.hadPassword && sheet.clearPassword);
        sheet.closed();
    }

    Keys.onEscapePressed: (e) => { if (sheet.open) { sheet.closed(); e.accepted = true; } }
    focus: open

    component LabelText: Text {
        color: Tokens.inkMuted
        font.family: Tokens.mono; font.pixelSize: 9; font.letterSpacing: 1.3
    }

    // scrim: only an outside click (or Esc) dismisses the sheet
    MouseArea {
        anchors.fill: parent
        onClicked: sheet.closed()
    }

    Rectangle {
        anchors.centerIn: parent
        width: Math.min(480, sheet.width - Tokens.s5 * 2)
        height: Math.min(Tokens.s6 + header.implicitHeight + Tokens.s3 + fields.implicitHeight + Tokens.s3 + footer.implicitHeight + Tokens.s6, sheet.height - Tokens.s5 * 2)
        radius: Tokens.radius
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        antialiasing: false
        MouseArea { anchors.fill: parent }          // a click inside never dismisses
        Ticks { color: Tokens.line }

        Column {
            id: header
            anchors { top: parent.top; left: parent.left; right: parent.right }
            anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6; anchors.topMargin: Tokens.s6
            spacing: Tokens.s3
            Row {
                spacing: Tokens.s2
                Text { text: "//"; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro; anchors.verticalCenter: parent.verticalCenter }
                Text {
                    text: (sheet.editing ? I18n.tr("EDIT") : I18n.tr("NEW")) + I18n.tr(" CONSOLE")
                    color: Tokens.ink
                    font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                    font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                    anchors.verticalCenter: parent.verticalCenter
                }
                Text { text: "接続"; color: Tokens.inkFaint; font.family: Tokens.jp; font.pixelSize: 12; anchors.verticalCenter: parent.verticalCenter }
            }
            Rectangle { width: parent.width; height: 1; color: Tokens.lineSoft }
        }

        Flickable {
            id: flick
            anchors { top: header.bottom; bottom: footer.top; left: parent.left; right: parent.right }
            anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6
            anchors.topMargin: Tokens.s3; anchors.bottomMargin: Tokens.s3
            contentWidth: width
            contentHeight: fields.implicitHeight
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            interactive: contentHeight > height
            ScrollBar.vertical: ScrollRail {}

            Column {
                id: fields
                width: flick.width
                spacing: Tokens.s3

                // protocol pick: four plates, one lead
                Row {
                    width: parent.width
                    spacing: Tokens.s2
                    Repeater {
                        model: [ "serial", "telnet", "rdp", "vnc" ]
                        Btn {
                            required property string modelData
                            text: modelData.toUpperCase()
                            primary: sheet.kind === modelData
                            compact: true
                            armed: !sheet.editing
                            onAct: { sheet.kind = modelData; portF.text = sheet.defaultPort(); }
                        }
                    }
                }
                Text {
                    visible: sheet.editing
                    text: I18n.tr("the protocol stays with the berth")
                    color: Tokens.inkFaint
                    font.family: Tokens.mono; font.pixelSize: 9
                }

                Column {
                    width: parent.width
                    spacing: 4
                    LabelText { text: I18n.tr("NAME") }
                    Field { id: nameF; width: parent.width; placeholder: I18n.tr("core-switch") }
                }

                // serial shape
                Column {
                    width: parent.width
                    spacing: Tokens.s3
                    visible: sheet.serial
                    Column {
                        width: parent.width
                        spacing: 4
                        LabelText { text: I18n.tr("DEVICE") }
                        Field { id: devF; width: parent.width; tabular: true; placeholder: "/dev/ttyUSB0" }
                        Text {
                            text: I18n.tr("if access is denied, your user must join the operator group of the port")
                            color: Tokens.inkFaint
                            font.family: Tokens.ui; font.pixelSize: 9
                            wrapMode: Text.WordWrap
                            width: parent.width
                        }
                    }
                    Row {
                        width: parent.width
                        spacing: Tokens.s3
                        Column {
                            width: (fields.width - Tokens.s3) * 0.5
                            spacing: 4
                            LabelText { text: I18n.tr("BAUD") }
                            Field { id: baudF; width: parent.width; tabular: true; placeholder: "115200" }
                        }
                        Column {
                            width: (fields.width - Tokens.s3) * 0.5
                            spacing: 4
                            LabelText { text: I18n.tr("COMMON") }
                            Flow {
                                width: parent.width
                                spacing: Tokens.s1
                                Repeater {
                                    model: [ 9600, 19200, 38400, 57600, 115200 ]
                                    Btn {
                                        required property int modelData
                                        text: String(modelData)
                                        compact: true
                                        onAct: baudF.text = String(modelData)
                                    }
                                }
                            }
                        }
                    }
                }

                // network shape
                Column {
                    width: parent.width
                    spacing: Tokens.s3
                    visible: !sheet.serial
                    Row {
                        width: parent.width
                        spacing: Tokens.s3
                        Column {
                            width: (fields.width - Tokens.s3) * 0.72
                            spacing: 4
                            LabelText { text: I18n.tr("ADDRESS") }
                            Field { id: addrF; width: parent.width; tabular: true; placeholder: "router.home or 10.0.0.9" }
                        }
                        Column {
                            width: (fields.width - Tokens.s3) * 0.28
                            spacing: 4
                            LabelText { text: I18n.tr("PORT") }
                            Field { id: portF; width: parent.width; tabular: true }
                        }
                    }
                    Row {
                        width: parent.width
                        spacing: Tokens.s3
                        visible: sheet.kind === "rdp" || sheet.kind === "vnc"
                        Column {
                            width: (fields.width - Tokens.s3) * 0.5
                            spacing: 4
                            LabelText { text: I18n.tr("USER") }
                            Field { id: userF; width: parent.width; tabular: true; placeholder: "alice" }
                        }
                        Column {
                            width: (fields.width - Tokens.s3) * 0.5
                            spacing: 4
                            visible: sheet.kind === "rdp"
                            LabelText { text: I18n.tr("DOMAIN") }
                            Field { id: domainF; width: parent.width; tabular: true; placeholder: I18n.tr("WORKGROUP (optional)") }
                        }
                    }
                    Row {
                        width: parent.width
                        spacing: Tokens.s3
                        visible: sheet.kind === "rdp" || sheet.kind === "vnc"
                        LabelText { text: I18n.tr("FULLSCREEN"); anchors.verticalCenter: parent.verticalCenter }
                        Switch { id: fullToggle; anchors.verticalCenter: parent.verticalCenter }
                    }
                    Column {
                        width: parent.width
                        spacing: 4
                        visible: sheet.kind === "rdp" || sheet.kind === "vnc"
                        LabelText { text: I18n.tr("PASSWORD (optional)") }
                        Field {
                            id: pwF; width: parent.width; tabular: true; secret: true
                            placeholder: sheet.hadPassword ? I18n.tr("type to replace the saved password") : I18n.tr("stored in your keyring, fed to the client at launch")
                        }
                        Text {
                            visible: sheet.hadPassword
                            text: sheet.clearPassword ? I18n.tr("· clears on save") : I18n.tr("· saved - click to forget")
                            color: sheet.clearPassword ? Tokens.ink : Tokens.inkFaint
                            font.family: Tokens.mono; font.pixelSize: 9
                            MouseArea {
                                anchors.fill: parent; cursorShape: Qt.PointingHandCursor
                                onClicked: sheet.clearPassword = !sheet.clearPassword
                            }
                        }
                    }
                    Text {
                        visible: sheet.kind === "telnet"
                        width: parent.width
                        wrapMode: Text.WordWrap
                        text: I18n.tr("Telnet is plaintext. It is kept for gear that speaks nothing else; prefer SSH when the device can.")
                        color: Tokens.inkFaint
                        font.family: Tokens.ui; font.pixelSize: 9
                    }
                }
            }
        }

        Rectangle {
            id: footerBand
            anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
            height: footer.implicitHeight + Tokens.s6 * 2
            color: Tokens.paperLift
            Rectangle { anchors.top: parent.top; width: parent.width; height: 1; color: Tokens.lineSoft }
        }
        Column {
            id: footer
            anchors { bottom: parent.bottom; left: parent.left; right: parent.right }
            anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6; anchors.bottomMargin: Tokens.s6
            spacing: Tokens.s2
            Text {
                visible: !sheet.valid && (nameF.text.length > 0)
                width: parent.width
                wrapMode: Text.WordWrap
                text: sheet.serial
                    ? I18n.tr("A name, a /dev/ device, and a real baud rate are needed.")
                    : I18n.tr("A name and a bare address (no spaces) are needed.")
                color: Tokens.inkMuted
                font.family: Tokens.ui; font.pixelSize: 11
            }
            Row {
                anchors.right: parent.right
                spacing: Tokens.s2
                Btn { text: I18n.tr("CANCEL"); onAct: sheet.closed() }
                Btn { text: I18n.tr("SAVE"); primary: true; armed: sheet.valid; onAct: sheet.save() }
            }
        }
    }
}

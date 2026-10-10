pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

// The manifest: add or amend one remote. A plain sheet of the facts a connection
// needs -- alias, address, user, port, key, and the harbour's own grouping. It
// writes through ryossh into ryoport's ~/.ssh/config.d include, so the host also
// works from a bare `ssh <alias>`; nothing is locked in here.
Item {
    id: sheet

    property bool open: false
    property string editAlias: ""
    property bool hadPassword: false
    property bool clearPassword: false
    signal closed()

    anchors.fill: parent
    visible: opacity > 0.01
    opacity: open ? 1 : 0
    z: 100
    Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }

    readonly property bool editing: editAlias.length > 0
    // match the engine's rules: a host that never resolves is worse than a
    // blocked save, so alias/host must be a single whitespace-free token.
    readonly property bool spaced: /\s/.test(aliasF.text.trim()) || /\s/.test(hostF.text.trim())
    readonly property bool valid: aliasF.text.trim().length > 0 && hostF.text.trim().length > 0 && !spaced

    onOpenChanged: if (open) sheet.prefill()
    function prefill() {
        var h = null;
        if (sheet.editing)
            for (var i = 0; i < Remotes.hosts.length; i++)
                if (Remotes.hosts[i].alias === sheet.editAlias) { h = Remotes.hosts[i]; break; }
        aliasF.text = h ? h.alias : "";
        hostF.text = h ? (h.hostName || "") : "";
        userF.text = h ? (h.user || "") : "root";
        portF.text = h && h.port ? String(h.port) : "22";
        keyF.text = h ? (h.identityFile || "") : "";
        groupF.text = h ? (h.group || "") : "";
        tagsF.text = h && h.tags ? h.tags.join(", ") : "";
        jumpF.text = h ? (h.proxyJump || "") : "";
        watchF.text = h && h.watch ? h.watch.join(", ") : "";
        appsF.text = h && h.apps ? h.apps.map(a => a.name + "=" + a.url).join(", ") : "";
        notesF.text = h ? (h.notes || "") : "";
        pveUrlF.text = h && h.pve ? (h.pve.url || "") : "";
        pveTokenF.text = h && h.pve ? (h.pve.token || "") : "";
        pwF.text = "";
        sheet.hadPassword = !!(h && h.auth === "password");
        sheet.clearPassword = false;
        Qt.callLater(aliasF.grabFocus);
    }
    function save() {
        if (!sheet.valid) return;
        var tags = tagsF.text.split(",").map(t => t.trim()).filter(t => t.length > 0);
        var watch = watchF.text.split(",").map(t => t.trim()).filter(t => t.length > 0);
        var apps = appsF.text.split(",").map(p => {
            var i = p.indexOf("=");
            if (i < 0) return null;
            var name = p.slice(0, i).trim();
            var url = p.slice(i + 1).trim();
            return name.length > 0 && url.length > 0 ? { name: name, url: url } : null;
        }).filter(a => a !== null);
        var pveUrl = pveUrlF.text.trim();
        var pve = pveUrl.length > 0 ? { url: pveUrl, token: pveTokenF.text.trim(), insecure: true } : null;
        Remotes.addHost({
            alias: aliasF.text.trim(),
            hostName: hostF.text.trim(),
            user: userF.text.trim(),
            port: parseInt(portF.text) || 22,
            identityFile: keyF.text.trim(),
            proxyJump: jumpF.text.trim(),
            group: groupF.text.trim(),
            tags: tags,
            watch: watch,
            notes: notesF.text.trim(),
            apps: apps,
            pve: pve
        }, pwF.text, sheet.hadPassword && sheet.clearPassword);
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
        id: cardBg
        anchors.centerIn: parent
        width: Math.min(480, sheet.width - Tokens.s5 * 2)
        height: Math.min(Tokens.s6 + header.implicitHeight + Tokens.s3 + fields.implicitHeight + Tokens.s3 + footer.implicitHeight + Tokens.s6, sheet.height - Tokens.s5 * 2)
        radius: Tokens.radius
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        MouseArea { anchors.fill: parent }          // a click inside the sheet never dismisses it
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
                    text: (sheet.editing ? I18n.tr("EDIT") : I18n.tr("NEW")) + I18n.tr(" CONNECTION")
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

                Row {
                    width: parent.width
                    spacing: Tokens.s3
                    Column {
                        width: (fields.width - Tokens.s3) * 0.5
                        spacing: 4
                        LabelText { text: I18n.tr("ALIAS") }
                        Field { id: aliasF; width: parent.width; tabular: true; placeholder: "vps-fra" }
                    }
                    Column {
                        width: (fields.width - Tokens.s3) * 0.5
                        spacing: 4
                        LabelText { text: I18n.tr("GROUP") }
                        Field { id: groupF; width: parent.width; tabular: true; placeholder: I18n.tr("Production") }
                    }
                }
                Column {
                    width: parent.width
                    spacing: 4
                    LabelText { text: I18n.tr("HOST") }
                    Field { id: hostF; width: parent.width; tabular: true; placeholder: I18n.tr("203.0.113.9 or box.example.com") }
                }
                Row {
                    width: parent.width
                    spacing: Tokens.s3
                    Column {
                        width: (fields.width - Tokens.s3) * 0.66
                        spacing: 4
                        LabelText { text: I18n.tr("USER") }
                        Field { id: userF; width: parent.width; tabular: true; placeholder: "root" }
                    }
                    Column {
                        width: (fields.width - Tokens.s3) * 0.34
                        spacing: 4
                        LabelText { text: I18n.tr("PORT") }
                        Field { id: portF; width: parent.width; tabular: true; placeholder: "22" }
                    }
                }
                Column {
                    width: parent.width
                    spacing: 4
                    LabelText { text: I18n.tr("IDENTITY FILE") }
                    Field { id: keyF; width: parent.width; tabular: true; placeholder: I18n.tr("~/.ssh/id_ed25519 (optional)") }
                }
                Column {
                    width: parent.width
                    spacing: 4
                    Row {
                        spacing: Tokens.s2
                        LabelText { text: I18n.tr("PASSWORD"); anchors.verticalCenter: parent.verticalCenter }
                        Text {
                            visible: sheet.hadPassword
                            text: sheet.clearPassword ? I18n.tr("· clears on save") : I18n.tr("· saved - ✕ forget")
                            color: sheet.clearPassword ? Tokens.ink : Tokens.inkFaint
                            font.family: Tokens.mono; font.pixelSize: 9; font.letterSpacing: 1.0
                            anchors.verticalCenter: parent.verticalCenter
                            MouseArea {
                                anchors.fill: parent; cursorShape: Qt.PointingHandCursor
                                onClicked: sheet.clearPassword = !sheet.clearPassword
                            }
                        }
                    }
                    Field {
                        id: pwF; width: parent.width; tabular: true; secret: true
                        placeholder: sheet.hadPassword ? I18n.tr("type to replace the saved password") : I18n.tr("optional - stored in your keyring")
                    }
                }
                Column {
                    width: parent.width
                    spacing: 4
                    LabelText { text: I18n.tr("PROXY JUMP") }
                    Field { id: jumpF; width: parent.width; tabular: true; placeholder: I18n.tr("bastion.example.com (optional)") }
                }
                Column {
                    width: parent.width
                    spacing: 4
                    LabelText { text: I18n.tr("TAGS") }
                    Field { id: tagsF; width: parent.width; tabular: true; placeholder: I18n.tr("web, eu (comma separated)") }
                }
                Column {
                    width: parent.width
                    spacing: 4
                    LabelText { text: I18n.tr("WATCH SERVICES") }
                    Field { id: watchF; width: parent.width; tabular: true; placeholder: I18n.tr("nginx, postgresql (probe reports each)") }
                }
                Column {
                    width: parent.width
                    spacing: 4
                    LabelText { text: I18n.tr("APPS") }
                    Field { id: appsF; width: parent.width; tabular: true; placeholder: "grafana=http://host:3000, proxmox=https://host:8006" }
                }
                Column {
                    width: parent.width
                    spacing: 4
                    LabelText { text: I18n.tr("NOTES") }
                    Field { id: notesF; width: parent.width; placeholder: I18n.tr("what this box is for (optional)") }
                }
                Rectangle { width: parent.width; height: 1; color: Tokens.lineSoft }
                Row {
                    width: parent.width
                    spacing: Tokens.s3
                    Column {
                        width: (fields.width - Tokens.s3) * 0.5
                        spacing: 4
                        LabelText { text: I18n.tr("PROXMOX URL") }
                        Field { id: pveUrlF; width: parent.width; tabular: true; placeholder: "https://host:8006" }
                    }
                    Column {
                        width: (fields.width - Tokens.s3) * 0.5
                        spacing: 4
                        LabelText { text: I18n.tr("PROXMOX TOKEN") }
                        Field { id: pveTokenF; width: parent.width; tabular: true; placeholder: I18n.tr("user@pam!id=secret") }
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
                visible: sheet.spaced
                width: parent.width
                wrapMode: Text.WordWrap
                text: I18n.tr("Alias and host must be a single token, no spaces.")
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

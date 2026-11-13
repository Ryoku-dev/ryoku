pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"

// The settings of one saved WireGuard tunnel or VPN profile (by UUID), shown
// under its row in Connections > VPN, through the shell daemon's network.vpnGet
// and friends. Both have auto-connect and what they connect to; a tunnel also
// has its peer to edit, its address and DNS (the IP settings box), its
// traffic while up, and its .conf to open in an editor and load back.
Item {
    id: panel

    property string uuid: ""
    // the tunnel was loaded back from its .conf under a new uuid
    signal replaced(string uuid)

    property var info: null
    property bool busy: false
    property string error: ""
    property string notice: ""
    // the .conf written out by Open .conf, until it is loaded back
    property string exportPath: ""

    readonly property bool wireguard: panel.info !== null && panel.info.type === "wireguard"
    readonly property var peer: panel.wireguard && panel.info.peers.length > 0 ? panel.info.peers[0] : null
    readonly property int labelWidth: 112

    implicitHeight: col.implicitHeight + Tokens.s4 * 2

    function bytes(n) {
        var units = ["B", "kB", "MB", "GB", "TB"];
        var i = 0;
        var v = n || 0;
        while (v >= 1000 && i < units.length - 1) {
            v /= 1000;
            i++;
        }
        return (i === 0 ? v : v.toFixed(1)) + " " + units[i];
    }
    // reads the details; fill puts the peer into the form, which a poll for
    // traffic leaves alone so nothing typed is lost
    function load(fill) {
        if (!panel.uuid.length)
            return;
        NetworkTopic.vpnGet(panel.uuid, function(ok, error, result) {
            if (!ok || !result) {
                panel.error = error || I18n.tr("Could not read this connection.");
                return;
            }
            panel.info = result;
            if (fill && panel.peer) {
                nameField.text = result.name || "";
                endpointField.text = panel.peer.endpoint || "";
                allowedField.text = (panel.peer.allowedIps || []).join(", ");
                keepaliveField.text = panel.peer.keepalive ? String(panel.peer.keepalive) : "";
            }
        });
    }
    function done(ok, error) {
        panel.busy = false;
        if (!ok)
            panel.error = error || I18n.tr("The change failed.");
    }
    function setAutoconnect(on) {
        panel.error = "";
        NetworkTopic.vpnSetAutoconnect(panel.uuid, on, function(ok, error) {
            panel.done(ok, error);
            panel.load(false);
        });
    }
    function save() {
        var keepalive = parseInt(keepaliveField.text.trim() || "0", 10);
        if (isNaN(keepalive) || keepalive < 0 || keepalive > 65535) {
            panel.error = I18n.tr("Keepalive is a number of seconds, 0 to turn it off.");
            return;
        }
        panel.busy = true;
        panel.error = "";
        panel.notice = "";
        NetworkTopic.wgEdit(panel.uuid, {
            name: nameField.text,
            publicKey: panel.peer.publicKey,
            endpoint: endpointField.text.trim(),
            allowedIps: allowedField.text.split(/[\s,]+/).filter(function(s) { return s.length > 0; }),
            keepalive: keepalive
        }, function(ok, error, result) {
            panel.done(ok, error);
            if (!ok)
                return;
            panel.notice = result && result.applied === false
                ? I18n.tr("Saved. Reconnect the tunnel to use it.")
                : I18n.tr("Saved.");
            panel.load(true);
        });
    }
    function openConf() {
        panel.busy = true;
        panel.error = "";
        panel.notice = "";
        NetworkTopic.wgExport(panel.uuid, function(ok, error, result) {
            panel.done(ok, error);
            if (!ok || !result)
                return;
            panel.exportPath = result.path;
            Spawn.runApp(["xdg-open", result.path], { label: "editor" });
        });
    }
    function loadConf() {
        panel.busy = true;
        panel.error = "";
        panel.notice = "";
        NetworkTopic.wgReplace(panel.uuid, panel.exportPath, function(ok, error, result) {
            panel.done(ok, error);
            if (!ok || !result)
                return;
            panel.exportPath = "";
            panel.replaced(result.uuid);
        });
    }

    onUuidChanged: panel.load(true)
    Component.onCompleted: panel.load(true)

    // a tunnel that is up reports its traffic
    Timer {
        interval: 2000
        repeat: true
        running: panel.wireguard && panel.info.active === true
        onTriggered: panel.load(false)
    }

    component Label: Text {
        width: panel.labelWidth
        color: Tokens.inkDim
        font.family: Tokens.ui
        font.pixelSize: Tokens.fMicro
        font.weight: Font.Medium
        font.letterSpacing: Tokens.trackLabel
    }
    component Value: Text {
        color: Tokens.ink
        font.family: Tokens.mono
        font.pixelSize: Tokens.fSmall
        elide: Text.ElideRight
    }
    component Line: Row {
        property alias label: lineLabel.text
        property alias value: lineValue.text
        spacing: Tokens.s4
        Label {
            id: lineLabel
            anchors.verticalCenter: parent.verticalCenter
        }
        Value {
            id: lineValue
            anchors.verticalCenter: parent.verticalCenter
            width: col.width - panel.labelWidth - Tokens.s4
        }
    }

    Rectangle {
        width: panel.width
        height: panel.implicitHeight
        color: "transparent"
        radius: Tokens.radius
        border.width: Tokens.border
        border.color: Tokens.line
    }

    Column {
        id: col
        x: Tokens.s4
        y: Tokens.s4
        width: panel.width - Tokens.s4 * 2
        spacing: Tokens.s3
        enabled: panel.info !== null && !panel.busy

        Row {
            spacing: Tokens.s4
            Label {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("AUTO-CONNECT")
            }
            Sw {
                anchors.verticalCenter: parent.verticalCenter
                on: panel.info !== null && panel.info.autoconnect === true
                onToggled: function(next) { panel.setAutoconnect(next); }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Connect on its own whenever the network is up.")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }
        }

        // a VPN plugin profile: what it connects to; its own settings stay
        // with the plugin
        Line {
            visible: panel.info !== null && !panel.wireguard
            label: I18n.tr("PLUGIN")
            value: panel.info ? (panel.info.service || "") : ""
        }
        Line {
            visible: panel.info !== null && !panel.wireguard && !!(panel.info.data && (panel.info.data.remote || panel.info.data.gateway))
            label: I18n.tr("SERVER")
            value: panel.info && panel.info.data ? (panel.info.data.remote || panel.info.data.gateway || "") : ""
        }
        Line {
            visible: panel.info !== null && !panel.wireguard && !!(panel.info.data && (panel.info.data.username || panel.info.data.user))
            label: I18n.tr("USER")
            value: panel.info && panel.info.data ? (panel.info.data.username || panel.info.data.user || "") : ""
        }

        // a WireGuard tunnel
        Line {
            visible: panel.wireguard
            label: I18n.tr("INTERFACE")
            value: panel.wireguard ? panel.info.interface + (panel.info.listenPort ? "  ·  " + I18n.tr("port %1").arg(panel.info.listenPort) : "") : ""
        }
        Line {
            visible: panel.wireguard && panel.info.active === true
            label: I18n.tr("TRAFFIC")
            value: panel.wireguard ? I18n.tr("%1 received  ·  %2 sent").arg(panel.bytes(panel.info.rxBytes)).arg(panel.bytes(panel.info.txBytes)) : ""
        }

        Column {
            visible: panel.peer !== null
            width: col.width
            spacing: Tokens.s3

            Row {
                spacing: Tokens.s4
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("NAME")
                }
                Field {
                    id: nameField
                    width: Math.min(260, col.width - panel.labelWidth - Tokens.s4)
                }
            }
            Row {
                spacing: Tokens.s4
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("ENDPOINT")
                }
                Field {
                    id: endpointField
                    width: Math.min(260, col.width - panel.labelWidth - Tokens.s4)
                    tabular: true
                    placeholder: "vpn.example.com:51820"
                }
            }
            Row {
                spacing: Tokens.s4
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("ALLOWED IPS")
                }
                Field {
                    id: allowedField
                    width: Math.min(360, col.width - panel.labelWidth - Tokens.s4)
                    tabular: true
                    placeholder: "0.0.0.0/0, ::/0"
                }
            }
            Row {
                spacing: Tokens.s4
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("KEEPALIVE")
                }
                Field {
                    id: keepaliveField
                    width: 80
                    tabular: true
                    placeholder: I18n.tr("off")
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("seconds")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                }
                Btn {
                    anchors.verticalCenter: parent.verticalCenter
                    compact: true
                    primary: true
                    armed: !panel.busy && nameField.text.trim().length > 0
                    text: I18n.tr("Save")
                    onAct: panel.save()
                }
            }
            Text {
                visible: panel.wireguard && panel.info.peers.length > 1
                width: col.width
                text: I18n.tr("This tunnel has %1 peers; the first is shown. Open the .conf to change the others.").arg(panel.wireguard ? panel.info.peers.length : 0)
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }
        }

        // the tunnel's own address and DNS
        ConnectionsIpEditor {
            visible: panel.wireguard
            width: col.width
            uuid: panel.wireguard ? panel.uuid : ""
            tunnel: true
            framed: false
            labelWidth: panel.labelWidth
        }

        Row {
            visible: panel.wireguard
            spacing: Tokens.s2
            Label {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("CONFIG")
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                compact: true
                text: I18n.tr("Open .conf")
                onAct: panel.openConf()
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                visible: panel.exportPath !== ""
                compact: true
                primary: true
                text: I18n.tr("Load changes")
                onAct: panel.loadConf()
            }
        }
        Text {
            visible: panel.exportPath !== ""
            width: col.width
            text: I18n.tr("Edit the file, save it, then Load changes. It holds the tunnel's private key, so it stays in a folder only you can read and is removed once loaded, or when you log out.")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.WordWrap
        }
        Text {
            visible: panel.exportPath !== ""
            width: col.width
            text: panel.exportPath
            color: Tokens.inkMuted
            font.family: Tokens.mono
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.WrapAnywhere
        }

        Text {
            visible: panel.notice !== "" || panel.error !== ""
            width: col.width
            text: panel.error !== "" ? panel.error : panel.notice
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.WordWrap
        }
    }
}

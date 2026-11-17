import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// the settings of one saved WireGuard tunnel or VPN profile (by uuid), under its row on the VPN
// page. Both have auto-connect and what they connect to; a tunnel also has its peer to edit, its
// address and DNS, its traffic while up, and its .conf to open in an editor and load back
Column {
    id: panel

    required property var hub
    property string uuid: ""
    // the tunnel was loaded back from its .conf under a new uuid
    signal replaced(string uuid)

    property var info: null
    property bool busy: false
    property string error: ""
    property string notice: ""
    // the .conf written by Open .conf, until it is loaded back
    property string exportPath: ""

    readonly property bool wireguard: panel.info !== null && panel.info.type === "wireguard"
    readonly property var peer: panel.wireguard && panel.info.peers.length > 0 ? panel.info.peers[0] : null
    readonly property bool wide: panel.width >= 520
    readonly property bool usable: panel.info !== null && !panel.busy && NetworkLink.usable

    readonly property var facts: {
        var i = panel.info;
        if (!i)
            return [];
        if (!panel.wireguard) {
            var d = i.data || {};
            var out = [{ label: I18n.tr("Plugin"), value: i.service || "" }];
            if (d.remote || d.gateway)
                out.push({ label: I18n.tr("Server"), value: d.remote || d.gateway, literal: true });
            if (d.username || d.user)
                out.push({ label: I18n.tr("User"), value: d.username || d.user, literal: true });
            return out;
        }
        var rows = [{ label: I18n.tr("Interface"), literal: true,
                      value: i.interface + (i.listenPort ? "  \u00b7  " + I18n.tr("port %1").arg(i.listenPort) : "") }];
        if (i.active === true)
            rows.push({ label: I18n.tr("Traffic"), value: I18n.tr("%1 received  \u00b7  %2 sent").arg(panel.bytes(i.rxBytes)).arg(panel.bytes(i.txBytes)) });
        return rows;
    }

    width: parent ? parent.width : 0
    spacing: LithicTokens.space.s3
    bottomPadding: LithicTokens.space.s2

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

    // fill puts the peer into the form; a poll for traffic leaves the form alone so nothing typed is lost
    function load(fill) {
        if (panel.uuid === "")
            return;
        var uuid = panel.uuid;
        NetworkLink.vpnGet(uuid, function (ok, error, result) {
            if (uuid !== panel.uuid)
                return;
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
        NetworkLink.vpnSetAutoconnect(panel.uuid, on, function (ok, error) {
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
        NetworkLink.wgEdit(panel.uuid, {
            name: nameField.text,
            publicKey: panel.peer.publicKey,
            endpoint: endpointField.text.trim(),
            allowedIps: allowedField.text.split(/[\s,]+/).filter(s => s.length > 0),
            keepalive: keepalive
        }, function (ok, error, result) {
            panel.done(ok, error);
            if (!ok)
                return;
            panel.notice = result && result.applied === false ? I18n.tr("Saved. Reconnect the tunnel to use it.") : I18n.tr("Saved.");
            panel.load(true);
        });
    }

    function openConf() {
        panel.busy = true;
        panel.error = "";
        panel.notice = "";
        NetworkLink.wgExport(panel.uuid, function (ok, error, result) {
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
        NetworkLink.wgReplace(panel.uuid, panel.exportPath, function (ok, error, result) {
            panel.done(ok, error);
            if (!ok || !result)
                return;
            panel.exportPath = "";
            panel.replaced(result.uuid);
        });
    }

    function remove() {
        panel.hub.confirm({
            title: I18n.tr("Remove %1?").arg(panel.info ? panel.info.name : ""),
            text: I18n.tr("The tunnel and its keys are deleted from this computer. To use it again you need its .conf file."),
            actionText: I18n.tr("Remove")
        }, function () {
            panel.busy = true;
            panel.error = "";
            NetworkLink.wireguardDelete(panel.uuid, panel.done);
        });
    }

    property bool built: false
    onUuidChanged: if (panel.built) panel.load(true)
    Component.onCompleted: {
        panel.built = true;
        panel.load(true);
    }

    Connections {
        target: NetworkLink
        function onUsableChanged() {
            if (NetworkLink.usable && panel.info === null)
                panel.load(true);
        }
    }

    // a tunnel that is up reports its traffic
    Timer {
        interval: 2000
        repeat: true
        running: panel.wireguard && panel.info.active === true && panel.visible
        onTriggered: panel.load(false)
    }

    SettingRow {
        width: parent.width
        title: I18n.tr("Auto-connect")
        detail: I18n.tr("Connect on its own whenever the network is up")
        activatable: panel.usable
        onActivated: panel.setAutoconnect(!(panel.info !== null && panel.info.autoconnect === true))

        Toggle {
            id: autoToggle
            anchors.verticalCenter: parent.verticalCenter
            enabled: panel.usable
            checked: panel.info !== null && panel.info.autoconnect === true
            Accessible.name: I18n.tr("Auto-connect")
            onToggled: {
                var want = autoToggle.checked;
                autoToggle.checked = Qt.binding(() => panel.info !== null && panel.info.autoconnect === true);
                panel.setAutoconnect(want);
            }
        }
    }

    Facts {
        visible: panel.facts.length > 0
        width: parent.width
        facts: panel.facts
    }

    Column {
        visible: panel.peer !== null
        width: parent.width
        spacing: LithicTokens.space.s3

        Flow {
            width: parent.width
            spacing: LithicTokens.space.s3

            TextField {
                id: nameField
                width: panel.wide ? (parent.width - parent.spacing) / 2 : parent.width
                label: I18n.tr("Name")
                enabled: panel.usable
            }
            TextField {
                id: endpointField
                width: panel.wide ? (parent.width - parent.spacing) / 2 : parent.width
                label: I18n.tr("Endpoint")
                mono: true
                enabled: panel.usable
                placeholderText: "vpn.example.com:51820"
            }
            TextField {
                id: allowedField
                width: panel.wide ? (parent.width - parent.spacing) * 0.7 : parent.width
                label: I18n.tr("Allowed IPs")
                mono: true
                enabled: panel.usable
                placeholderText: "0.0.0.0/0, ::/0"
            }
            TextField {
                id: keepaliveField
                width: panel.wide ? (parent.width - parent.spacing) * 0.3 : parent.width
                label: I18n.tr("Keepalive, seconds")
                mono: true
                enabled: panel.usable
                placeholderText: I18n.tr("off")
            }
        }

        Label {
            visible: panel.wireguard && panel.info.peers.length > 1
            width: parent.width
            text: I18n.tr("This tunnel has %1 peers; the first is shown. Open the .conf to change the others.").arg(panel.wireguard ? panel.info.peers.length : 0)
            typeRole: "detail"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Row {
            anchors.right: parent.right
            spacing: LithicTokens.space.s2

            Plaque {
                kind: "gold"
                small: true
                enabled: panel.usable && nameField.text.trim().length > 0
                text: I18n.tr("Save")
                onClicked: panel.save()
            }
        }
    }

    // the tunnel's own address and DNS
    Loader {
        active: panel.wireguard
        visible: active
        width: parent.width
        sourceComponent: IpSettings {
            uuid: panel.uuid
            tunnel: true
        }
    }

    SettingRow {
        visible: panel.wireguard
        width: parent.width
        title: I18n.tr("Edit the .conf")
        detail: I18n.tr("Open the tunnel's whole .conf in a text editor, then load it back")

        Plaque {
            anchors.verticalCenter: parent.verticalCenter
            kind: "tonal"
            small: true
            glyph: "edit_document"
            enabled: panel.usable
            text: I18n.tr("Open .conf")
            onClicked: panel.openConf()
        }
        Plaque {
            anchors.verticalCenter: parent.verticalCenter
            visible: panel.exportPath !== ""
            kind: "gold"
            small: true
            enabled: panel.usable
            text: I18n.tr("Load changes")
            onClicked: panel.loadConf()
        }
    }

    Label {
        visible: panel.exportPath !== ""
        width: parent.width
        text: I18n.tr("The file holds the tunnel's private key, so it sits in a folder only you can read and goes once it is loaded, or when you log out.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Label {
        visible: panel.exportPath !== ""
        width: parent.width
        text: panel.exportPath
        typeRole: "mono"
        inkRole: "fg2"
        wrapMode: Text.WrapAnywhere
        elide: Text.ElideNone
    }

    SettingRow {
        visible: panel.wireguard
        width: parent.width
        title: I18n.tr("Remove")
        detail: I18n.tr("Deletes the tunnel from this computer")

        Plaque {
            anchors.verticalCenter: parent.verticalCenter
            kind: "dangerLine"
            small: true
            glyph: "delete"
            enabled: panel.usable
            text: I18n.tr("Remove tunnel")
            onClicked: panel.remove()
        }
    }

    Label {
        visible: panel.notice !== "" || panel.error !== "" || NetworkLink.unavailable !== ""
        width: parent.width
        text: NetworkLink.unavailable !== "" ? NetworkLink.unavailable : panel.error !== "" ? panel.error : panel.notice
        typeRole: "detail"
        ink: panel.error !== "" || NetworkLink.unavailable !== "" ? Lithic.jasper : Lithic.fg
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}

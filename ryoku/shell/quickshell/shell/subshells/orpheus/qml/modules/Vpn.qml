import QtQuick
import shell.services
import "../kit" as K

Item {
    id: root

    required property var scene
    required property string lane

    readonly property var live: Network.vpnTunnels.filter(tunnel => tunnel.active && !tunnel.connecting)
    readonly property bool connecting: Network.vpnTunnels.some(tunnel => tunnel.connecting)
    readonly property bool connected: root.live.length > 0 || Tailscale.up

    // A tunnel the user brought up names itself first; Tailscale, usually always
    // on, names its exit node, else this machine.
    readonly property string headline: {
        if (root.live.length > 0)
            return root.live[0].name;
        if (!Tailscale.up)
            return "";
        if (Tailscale.exitNodeActive && Tailscale.exitNodeName.length > 0)
            return Tailscale.exitNodeName;
        return Tailscale.hostName || "Tailscale";
    }

    readonly property string hintText: {
        const names = root.live.map(tunnel => tunnel.name);
        if (Tailscale.up)
            names.push("Tailscale" + (Tailscale.ip4.length > 0 ? " " + Tailscale.ip4 : ""));
        if (names.length > 0)
            return "VPN: " + names.join(" · ");
        return root.connecting ? "VPN connecting" : "VPN off";
    }

    // Shown once there is something to switch: Tailscale answering, or a saved
    // VPN or WireGuard profile.
    property bool present: Tailscale.available || Network.vpnTunnels.length > 0

    implicitWidth: present ? body.implicitWidth + 2 * K.Style.hitPad : 0
    implicitHeight: K.Style.barH
    height: K.Style.barH

    Component.onCompleted: Tailscale.setActive(root, true)
    Component.onDestruction: Tailscale.setActive(root, false)

    Row {
        id: body
        anchors.centerIn: parent
        spacing: K.Style.s1
        scale: press.pressScale

        K.Glyph {
            anchors.verticalCenter: parent.verticalCenter
            name: root.connected || root.connecting ? "vpn_lock" : "vpn_key_off"
            size: K.Style.barIcon
            filled: root.connected
            tone: root.connected ? "ink" : root.connecting ? "dim" : "faint"
        }

        K.Label {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.headline.length > 0
            width: Math.min(implicitWidth, 140 * K.Style.scale)
            elide: Text.ElideRight
            text: root.headline
            role: "bar"
        }
    }

    K.Press {
        id: press
        radius: K.Style.radius
        onClicked: mouse => root.scene.toggleCard("vpn", root)
    }

    K.Hint {
        target: root
        text: root.hintText
        shown: root.present && press.hovered
    }
}

pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import shell.services
import "../kit" as K

Item {
    id: root

    required property var scene
    property bool shown: false
    property var args: ({})

    // Turning Tailscale off while an exit node carries the traffic arms first;
    // the second switch-off within a few seconds goes through.
    property bool armed: false
    // Call id of the last profile switch, so only its own failure shows here.
    property int pendingCall: 0
    property string profileError: ""
    // ListRow's own press sits above its trailing slot, so a row tap is the
    // switch. A tap that also reaches the switch would fire twice; the second
    // within this window is the same tap and is dropped.
    property double lastFlipAt: 0

    // The address the internet sees, read from checkip.amazonaws.com only while
    // the card is open: on open, a moment after any tunnel or exit node change,
    // and every minute. "" while unknown.
    property string publicIp: ""
    property bool publicIpFailed: false
    property bool publicIpCopied: false
    readonly property string routeSignature: [Tailscale.up, Tailscale.exitNodeActive, Tailscale.exitNodeName]
        .concat(Network.vpnTunnels.filter(tunnel => tunnel.active).map(tunnel => tunnel.uuid)).join("|")

    function refreshPublicIp() {
        if (!root.shown || publicIpProbe.running)
            return;
        publicIpProbe.running = true;
    }

    function copyPublicIp() {
        if (root.publicIp.length === 0)
            return;
        Quickshell.execDetached(["wl-copy", root.publicIp]);
        root.publicIpCopied = true;
        copiedReset.restart();
    }

    onRouteSignatureChanged: if (root.shown) routeSettle.restart()

    readonly property bool anyLive: Tailscale.up || Network.vpnTunnels.some(tunnel => tunnel.active && !tunnel.connecting)
    readonly property bool empty: !Tailscale.available && Network.vpnTunnels.length === 0

    readonly property string tailscaleSubtitle: {
        if (Tailscale.signedOut)
            return "Signed out";
        if (!Tailscale.up)
            return "Off";
        const parts = [];
        if (Tailscale.ip4.length > 0)
            parts.push(Tailscale.ip4);
        if (Tailscale.peersTotal > 0)
            parts.push(Tailscale.peersOnline + "/" + Tailscale.peersTotal + " online");
        return parts.length > 0 ? parts.join(" · ") : "Connected";
    }

    function serviceLabel(service) {
        switch (service) {
        case "wireguard": return "WireGuard";
        case "openvpn": return "OpenVPN";
        case "openconnect": return "OpenConnect";
        case "vpnc": return "Cisco VPNC";
        case "l2tp": return "L2TP";
        case "pptp": return "PPTP";
        case "strongswan":
        case "libreswan": return "IPsec";
        case "fortisslvpn": return "Fortinet SSL";
        default: return service ? String(service) : "VPN";
        }
    }

    function tunnelSubtitle(tunnel) {
        if (tunnel.connecting)
            return "Connecting…";
        if (tunnel.active)
            return tunnel.ip4 && tunnel.ip4.length > 0 ? tunnel.ip4 : "Connected";
        return root.serviceLabel(tunnel.service);
    }

    function firstFlip() {
        const now = Date.now();
        if (now - root.lastFlipAt < 350)
            return false;
        root.lastFlipAt = now;
        return true;
    }

    function setTailscale(on) {
        if (on) {
            root.armed = false;
            Tailscale.turnOn();
            return;
        }
        if (Tailscale.exitNodeActive && !root.armed) {
            root.armed = true;
            disarm.restart();
            return;
        }
        root.armed = false;
        Tailscale.turnOff();
    }

    function setTunnel(tunnel, on) {
        root.profileError = "";
        root.pendingCall = Network.setVpnTunnel(tunnel, on);
    }

    implicitWidth: sheet.implicitWidth
    implicitHeight: sheet.implicitHeight

    onShownChanged: {
        Tailscale.setActive(root, shown);
        if (shown)
            root.refreshPublicIp();
        if (!shown)
            root.armed = false;
    }
    Component.onCompleted: {
        Tailscale.setActive(root, shown);
        root.refreshPublicIp();
    }
    Component.onDestruction: Tailscale.setActive(root, false)

    Connections {
        target: Network
        function onReplied(id, ok, error) {
            if (id === root.pendingCall && !ok)
                root.profileError = error.length > 0 ? error : "The VPN did not connect";
        }
    }

    Process {
        id: publicIpProbe
        command: ["curl", "-fsS", "--max-time", "5", "https://checkip.amazonaws.com/"]
        stdout: StdioCollector { id: publicIpOutput }
        onExited: code => {
            const address = publicIpOutput.text.trim();
            const valid = code === 0 && /^[0-9a-fA-F:.]{3,45}$/.test(address);
            root.publicIpFailed = !valid;
            if (valid)
                root.publicIp = address;
        }
    }

    // Routes take a moment to move after a switch; read once they have.
    Timer {
        id: routeSettle
        interval: 2500
        onTriggered: root.refreshPublicIp()
    }

    Timer {
        interval: 60000
        running: root.shown
        repeat: true
        onTriggered: root.refreshPublicIp()
    }

    Timer {
        id: copiedReset
        interval: 1500
        onTriggered: root.publicIpCopied = false
    }

    Timer {
        id: disarm
        interval: 4000
        onTriggered: root.armed = false
    }

    K.Label {
        id: stateLabel
        text: root.anyLive ? "Protected" : "Off"
        role: "caption"
        tone: root.anyLive ? "accent" : "muted"
    }

    K.Sheet {
        id: sheet
        pageWidth: 320
        title: "VPN"
        trailing: [stateLabel]

        // Many exit nodes or profiles scroll inside the card, as System does.
        K.Scroller {
            width: parent.width
            height: Math.min(contentHeight, Math.max(180,
                root.scene.cardMaxHeight - 96))

            Column {
                width: parent.width
                spacing: K.Style.s4

                K.ListRow {
                    width: parent.width
                    icon: "public"
                    title: root.publicIp.length > 0 ? root.publicIp
                        : root.publicIpFailed ? "Unavailable" : "Checking…"
                    subtitle: root.publicIpCopied ? "Copied"
                        : publicIpProbe.running && root.publicIp.length > 0 ? "Public IP · checking…"
                        : root.publicIpFailed && root.publicIp.length === 0 ? "Public IP · tap to retry"
                        : "Public IP · tap to copy"
                    onClicked: {
                        if (root.publicIp.length > 0)
                            root.copyPublicIp();
                        else
                            root.refreshPublicIp();
                    }
                }

                K.Section {
                    visible: Tailscale.available
                    width: parent.width
                    title: "Tailscale"

                    K.ListRow {
                        width: parent.width
                        icon: "vpn_lock"
                        title: Tailscale.hostName || "Tailscale"
                        subtitle: root.tailscaleSubtitle
                        onClicked: if (!Tailscale.busy && root.firstFlip()) root.setTailscale(!Tailscale.up)

                        K.Switch {
                            z: 2
                            checked: Tailscale.up
                            enabled: !Tailscale.busy
                            onToggled: value => { if (root.firstFlip()) root.setTailscale(value); }
                        }
                    }

                    K.Label {
                        visible: root.armed
                        width: parent.width
                        text: "Traffic leaves the exit node. Switch off again to confirm."
                        role: "caption"
                        tone: "alert"
                        wrapMode: Text.WordWrap
                    }

                    // Relay and route warnings only matter while the tunnel is up.
                    Repeater {
                        model: Tailscale.up ? Tailscale.health : []

                        delegate: K.Label {
                            required property var modelData
                            width: parent ? parent.width : 0
                            text: String(modelData)
                            role: "caption"
                            tone: "alert"
                            wrapMode: Text.WordWrap
                        }
                    }

                    Column {
                        visible: Tailscale.needsOperator
                        width: parent.width
                        spacing: K.Style.s2

                        K.Label {
                            width: parent.width
                            text: "Tailscale only lets its operator switch it. Authorise this user once; it asks for your password."
                            role: "caption"
                            tone: "dim"
                            wrapMode: Text.WordWrap
                        }

                        K.Button {
                            text: "Authorise"
                            icon: "admin_panel_settings"
                            variant: "filled"
                            enabled: !Tailscale.busy
                            onClicked: Tailscale.authorise()
                        }
                    }

                    Row {
                        spacing: K.Style.s2

                        K.Button {
                            visible: Tailscale.up && Tailscale.ip4.length > 0
                            text: "Copy IP"
                            icon: "content_copy"
                            variant: "outline"
                            onClicked: Tailscale.copyIp()
                        }

                        K.Button {
                            text: "Admin console"
                            icon: "open_in_new"
                            variant: "outline"
                            onClicked: {
                                Tailscale.openAdmin();
                                root.scene.closeCard();
                            }
                        }
                    }
                }

                // Where the traffic leaves: none, or one of the peers that offers.
                // Rows are taps, not buttons: ListRow's press covers its trailing slot.
                K.Section {
                    visible: Tailscale.up && (Tailscale.exitNodes.length > 0 || Tailscale.exitNodeActive || Tailscale.prefsKnown)
                    width: parent.width
                    title: "Exit node"

                    K.ListRow {
                        width: parent.width
                        icon: "block"
                        title: "None"
                        subtitle: "Traffic leaves from this machine"
                        selected: !Tailscale.exitNodeActive
                        enabled: !Tailscale.busy
                        onClicked: if (Tailscale.exitNodeActive && root.firstFlip()) Tailscale.stopExitNode()
                    }

                    Repeater {
                        model: Tailscale.exitNodes

                        delegate: K.ListRow {
                            id: exitRow
                            required property var modelData
                            width: parent ? parent.width : 0
                            icon: "alt_route"
                            title: modelData.name
                            subtitle: modelData.active
                                ? "In use" + (modelData.ip.length > 0 ? " · " + modelData.ip : "")
                                : modelData.online ? modelData.ip : "Offline"
                            selected: modelData.active
                            enabled: !Tailscale.busy && (modelData.online || modelData.active)
                            onClicked: {
                                if (!root.firstFlip())
                                    return;
                                if (exitRow.modelData.active)
                                    Tailscale.stopExitNode();
                                else
                                    Tailscale.useExitNode(exitRow.modelData);
                            }
                        }
                    }

                    K.ListRow {
                        visible: Tailscale.prefsKnown
                        width: parent.width
                        icon: "lan"
                        title: "Allow local network"
                        subtitle: "Keep reaching this LAN"
                        enabled: !Tailscale.busy
                        onClicked: if (root.firstFlip()) Tailscale.setAllowLan(!Tailscale.allowLan)

                        K.Switch {
                            z: 2
                            checked: Tailscale.allowLan
                            onToggled: value => { if (root.firstFlip()) Tailscale.setAllowLan(value); }
                        }
                    }

                    K.ListRow {
                        visible: Tailscale.prefsKnown
                        width: parent.width
                        icon: "router"
                        title: "Run as exit node"
                        subtitle: Tailscale.advertisingExit ? "Offered; approve in admin" : "Offer this machine"
                        enabled: !Tailscale.busy
                        onClicked: if (root.firstFlip()) Tailscale.setAdvertiseExit(!Tailscale.advertisingExit)

                        K.Switch {
                            z: 2
                            checked: Tailscale.advertisingExit
                            onToggled: value => { if (root.firstFlip()) Tailscale.setAdvertiseExit(value); }
                        }
                    }
                }

                K.Section {
                    visible: Network.vpnTunnels.length > 0
                    width: parent.width
                    title: "Profiles"

                    Repeater {
                        model: Network.vpnTunnels

                        delegate: K.ListRow {
                            id: tunnelRow
                            required property var modelData
                            width: parent ? parent.width : 0
                            icon: modelData.service === "wireguard" ? "key" : "vpn_key"
                            title: modelData.name
                            subtitle: root.tunnelSubtitle(modelData)
                            onClicked: if (root.firstFlip()) root.setTunnel(tunnelRow.modelData, !tunnelRow.modelData.active)

                            K.Switch {
                                z: 2
                                checked: tunnelRow.modelData.active
                                onToggled: value => { if (root.firstFlip()) root.setTunnel(tunnelRow.modelData, value); }
                            }
                        }
                    }
                }

                K.Label {
                    readonly property string message: root.profileError.length > 0 ? root.profileError : Tailscale.lastError
                    visible: message.length > 0
                    width: parent.width
                    text: message
                    role: "caption"
                    tone: "alert"
                    wrapMode: Text.WordWrap
                }

                K.Label {
                    visible: root.empty
                    width: parent.width
                    text: "No VPN found. Install Tailscale, or add a VPN or WireGuard profile to NetworkManager, and it shows up here."
                    role: "body"
                    tone: "variant"
                    wrapMode: Text.WordWrap
                }

                K.Button {
                    text: "Connection settings"
                    icon: "settings"
                    variant: "outline"
                    onClicked: root.scene.openHub("connections")
                }
            }
        }
    }
}

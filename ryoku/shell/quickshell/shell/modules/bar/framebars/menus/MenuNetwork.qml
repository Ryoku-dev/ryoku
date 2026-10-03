pragma ComponentBehavior: Bound

import QtQuick
import "../.." as Pill
import shell.services
import "../../../../components"
import Ryoku.Ui.Singletons

// Network entry (contract 06 sec 2.6): a RevealerRow whose inert action button
// carries the connection-status icon and whose label reports the current link;
// the reveal opens three sections in service order (no client sort): the Active
// Network with a Disconnect action, the WireGuard tunnels with connect /
// disconnect / delete / import, and the Available Networks with a per-AP
// password form and Connect. All state comes from the Go `network` topic via the
// Network singleton (NetworkManager over D-Bus in the daemon); QML never shells
// out. Reveal drives the wifi scan cadence (immediate, 30s loop, 15s "Scanning…"
// clear) and collapses on menu close.
//
// Divergence recorded (contract 00 divergence type 1): the reference opens a GTK
// FileDialog for WireGuard import; Quickshell has no file dialog, so import uses
// an inline path entry that feeds the same `nmcli connection import` the
// reference runs. The available list dedupes by SSID and band (one row per band,
// the strongest AP kept and the list sorted by signal) and excludes only the
// connected band row, so a dual-band SSID exposes both its 2.4 and 5 GHz rows.
Item {
    id: root

    property real s: 1
    property bool open: false

    property bool scanning: false

    // a hidden network has no scanned AP, so it joins through the daemon intent
    // with 802-11-wireless.hidden set; the form lives under the network list.
    property bool hiddenConnecting: false
    property int hiddenPendingId: -1
    property bool hiddenError: false

    implicitHeight: row.implicitHeight

    onOpenChanged: {
        Network.setVpnPolling(root, root.open);
        if (!root.open)
            row.revealed = false;
    }
    Component.onDestruction: Network.setVpnPolling(root, false)

    function submitHidden(ssid, password) {
        if (ssid === "" || root.hiddenConnecting)
            return;
        root.hiddenError = false;
        root.hiddenConnecting = true;
        root.hiddenPendingId = Network.connectWifi(ssid, password, "", true);
    }
    Connections {
        target: Network
        function onReplied(id, ok, error) {
            if (id !== root.hiddenPendingId)
                return;
            root.hiddenConnecting = false;
            root.hiddenPendingId = -1;
            if (!ok)
                root.hiddenError = true;
        }
    }

    // Available networks: one row per SSID+band, the strongest AP kept per key
    // and the list sorted by signal descending. Keying by SSID alone (the
    // reference behaviour) dropped a dual-band SSID's 5 GHz BSS; keying by band
    // keeps it. Only the connected band row is removed, so a link on 2.4 GHz
    // still leaves the same SSID's 5 GHz row pickable.
    readonly property var availableNets: {
        var aps = Network.accessPoints;
        var activeBand = "";
        for (var a = 0; a < aps.length; a++) {
            if (aps[a] && aps[a].active) {
                activeBand = aps[a].band || "";
                break;
            }
        }
        var best = ({});
        for (var i = 0; i < aps.length; i++) {
            var ap = aps[i];
            if (!ap || !ap.ssid)
                continue;
            if (ap.ssid === Network.activeSsid && (activeBand === "" || (ap.band || "") === activeBand))
                continue;
            var key = ap.ssid + "\u0000" + (ap.band || "");
            var prev = best[key];
            if (!prev || (ap.strength || 0) > (prev.strength || 0))
                best[key] = ap;
        }
        var out = [];
        for (var k in best)
            out.push(best[k]);
        out.sort(function (x, y) { return (y.strength || 0) - (x.strength || 0); });
        return out;
    }

    // SSIDs seen on more than one band in the scan: a row shows its band chip
    // only for these, so single-band networks read exactly as before. Counted
    // over every scanned AP, not the rendered list, so the 5 GHz row of the
    // SSID you are already on (whose 2.4 GHz row is filtered out) stays chipped.
    readonly property var multiBandSsids: {
        var bands = ({});
        var aps = Network.accessPoints;
        for (var i = 0; i < aps.length; i++) {
            var ap = aps[i];
            if (!ap || !ap.ssid || !ap.band)
                continue;
            if (!bands[ap.ssid])
                bands[ap.ssid] = ({});
            bands[ap.ssid][ap.band] = true;
        }
        var multi = ({});
        for (var s in bands)
            if (Object.keys(bands[s]).length > 1)
                multi[s] = true;
        return multi;
    }
    onAvailableNetsChanged: if (root.availableNets.length > 0) root.scanning = false

    function wifiIcon(strength) {
        var v = strength || 0;
        return v > 75 ? "signal_wifi_4_bar"
            : v > 50 ? "network_wifi_3_bar"
            : v > 25 ? "network_wifi_2_bar"
            : v > 0 ? "network_wifi_1_bar"
            : "signal_wifi_0_bar";
    }
    function secured(ap) {
        return ap && ap.security && ap.security !== "None";
    }

    readonly property string statusIcon: Network.kind === "ethernet" ? "lan"
        : Network.kind === "wifi" ? root.wifiIcon(Network.wifi.strength)
        : Network.wifiRadio ? "signal_wifi_off" : "wifi_off"
    readonly property string statusLabel: {
        var base;
        if (Network.kind === "ethernet")
            base = I18n.tr("Wired");
        else if (Network.kind === "wifi")
            base = Network.activeSsid.length > 0 ? Network.activeSsid : I18n.tr("Wi-Fi Connected");
        else if (Network.wifiConnectivity === "Connecting")
            base = I18n.tr("Connecting…");
        else
            base = I18n.tr("Not Connected");
        return Network.vpnActive ? I18n.tr("%1 (+WG)").arg(base) : base;
    }

    // A full-width primary-accent action button (the reference .ok-button-primary),
    // reused for Disconnect / Connect / Delete.
    component PrimaryButton: MenuButton {
        id: pb
        property alias text: pbLabel.text
        selected: true
        minH: pbLabel.implicitHeight + pb.pad * 2
        Text {
            id: pbLabel
            anchors.fill: parent
            horizontalAlignment: Text.AlignHCenter
            verticalAlignment: Text.AlignVCenter
            color: pb.contentColor
            font.family: Theme.fontPrimary
            font.pixelSize: Theme.fontMd
            font.weight: Font.Bold
        }
    }

    // A centered 18px variant-tone section header (.label-large-bold-variant).
    component SectionLabel: Text {
        width: parent ? parent.width : 0
        horizontalAlignment: Text.AlignHCenter
        color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
        font.family: Theme.fontPrimary
        font.pixelSize: Theme.fontLg
        font.weight: Font.Bold
    }

    // A centered 16px empty/status line (.label-medium).
    component EmptyLabel: Text {
        width: parent ? parent.width : 0
        horizontalAlignment: Text.AlignHCenter
        color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
        font.family: Theme.fontPrimary
        font.pixelSize: Theme.fontMd
    }

    // The wifi scan cadence is tied to the reveal (contract 06 sec 4): an
    // immediate scan on open, a 30s re-scan loop while open, and a 15s ceiling on
    // the "Scanning…" label (cleared earlier when results arrive, above).
    Timer {
        id: rescan
        interval: 30000
        repeat: true
        running: row.revealed
        onTriggered: Network.refresh()
    }
    Timer {
        id: scanClear
        interval: 15000
        onTriggered: root.scanning = false
    }

    RevealerRow {
        id: row
        width: root.width
        actionIconName: root.statusIcon
        actionSensitive: false

        middle: RevealerRowLabel {
            anchors.fill: parent
            label: root.statusLabel
        }

        onToggled: revealed => {
            if (revealed) {
                root.scanning = true;
                Network.refresh();
                scanClear.restart();
            } else {
                root.scanning = false;
                scanClear.stop();
            }
        }

        Column {
            width: parent.width
            spacing: 10

            // 1. Active Network -----------------------------------------------
            Column {
                width: parent.width
                spacing: 10
                visible: Network.wifiPresent && Network.activeSsid.length > 0

                SectionLabel { text: I18n.tr("Active Network") }

                RevealerButton {
                    width: parent.width
                    iconName: root.wifiIcon(Network.wifi.strength)
                    label: Network.activeSsid

                    PrimaryButton {
                        width: parent.width
                        text: I18n.tr("Disconnect")
                        onClicked: Network.disconnectWifi()
                    }
                }
            }

            // 2. Wireguard Connections ---------------------------------------
            Column {
                width: parent.width
                spacing: 10

                Item {
                    width: parent.width
                    height: Math.max(wgTitle.implicitHeight, wgAdd.implicitHeight)
                    SectionLabel {
                        id: wgTitle
                        anchors.centerIn: parent
                        width: parent.width
                        text: I18n.tr("Wireguard Connections")
                    }
                    MenuButton {
                        id: wgAdd
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        minW: Theme.iconSm + wgAdd.pad * 2
                        minH: Theme.iconSm + wgAdd.pad * 2
                        onClicked: wgImport.expanded = !wgImport.expanded
                        MaterialIcon {
                            anchors.centerIn: parent
                            font.pixelSize: Theme.iconSm
                            text: "add"
                            color: wgAdd.contentColor
                        }
                    }
                }

                // Inline import path entry (divergence: no Quickshell file dialog).
                Column {
                    id: wgImport
                    property bool expanded: false
                    width: parent.width
                    spacing: 8
                    visible: wgImport.expanded
                    height: visible ? implicitHeight : 0

                    Rectangle {
                        width: parent.width
                        height: wgPath.implicitHeight + Theme.paddingSm * 2
                        radius: Theme.radiusWidget
                        color: "transparent"
                        border.width: Theme.borderWidth
                        border.color: Theme.outline
                        TextInput {
                            id: wgPath
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.leftMargin: Theme.paddingMd
                            anchors.rightMargin: Theme.paddingMd
                            anchors.verticalCenter: parent.verticalCenter
                            color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                            font.family: Theme.fontPrimary
                            font.pixelSize: Theme.fontSm
                            clip: true
                            onAccepted: if (text.length > 0) { Network.wgImport(text); text = ""; wgImport.expanded = false; }
                            Text {
                                anchors.fill: parent
                                verticalAlignment: Text.AlignVCenter
                                text: I18n.tr("Path to .conf")
                                color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                                font: wgPath.font
                                visible: wgPath.text.length === 0 && !wgPath.activeFocus
                            }
                        }
                    }
                    PrimaryButton {
                        width: parent.width
                        text: I18n.tr("Import")
                        onClicked: if (wgPath.text.length > 0) { Network.wgImport(wgPath.text); wgPath.text = ""; wgImport.expanded = false; }
                    }
                }

                EmptyLabel {
                    text: I18n.tr("No Available WG Connections")
                    visible: Network.wgTunnels.length === 0
                }

                ListView {
                    width: parent.width
                    height: count > 0 ? Math.min(Math.max(contentHeight, 64 * root.s), 300 * root.s) : 0
                    clip: true
                    spacing: 10
                    model: row.revealed ? Network.wgTunnels : []
                    cacheBuffer: Math.max(0, height)
                    boundsBehavior: Flickable.StopAtBounds
                    delegate: wgRowComponent
                }
            }

            // 3. Available Networks ------------------------------------------
            Column {
                width: parent.width
                spacing: 10
                visible: Network.wifiPresent

                SectionLabel { text: I18n.tr("Available Networks") }

                EmptyLabel {
                    text: I18n.tr("No Available Networks")
                    visible: root.availableNets.length === 0 && !root.scanning
                }

                ListView {
                    width: parent.width
                    height: count > 0 ? Math.min(Math.max(contentHeight, 64 * root.s), 440 * root.s) : 0
                    clip: true
                    spacing: 10
                    model: row.revealed ? root.availableNets : []
                    cacheBuffer: Math.max(0, height)
                    boundsBehavior: Flickable.StopAtBounds
                    delegate: availNetComponent
                }

                EmptyLabel {
                    text: I18n.tr("Scanning…")
                    visible: root.scanning
                }

                // Hidden network: no scanned object exists, so the join rides
                // the daemon intent with the hidden flag (see root.submitHidden).
                Item {
                    width: parent.width
                    height: Math.max(hiddenTitle.implicitHeight, hiddenAdd.implicitHeight)
                    SectionLabel {
                        id: hiddenTitle
                        anchors.centerIn: parent
                        width: parent.width
                        text: I18n.tr("Hidden Network")
                    }
                    MenuButton {
                        id: hiddenAdd
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        minW: Theme.iconSm + hiddenAdd.pad * 2
                        minH: Theme.iconSm + hiddenAdd.pad * 2
                        onClicked: hiddenForm.expanded = !hiddenForm.expanded
                        MaterialIcon {
                            anchors.centerIn: parent
                            font.pixelSize: Theme.iconSm
                            text: "add"
                            color: hiddenAdd.contentColor
                        }
                    }
                }

                Column {
                    id: hiddenForm
                    property bool expanded: false
                    width: parent.width
                    spacing: 8
                    visible: hiddenForm.expanded
                    height: visible ? implicitHeight : 0

                    Rectangle {
                        width: parent.width
                        height: hiddenSsid.implicitHeight + Theme.paddingSm * 2
                        radius: Theme.radiusWidget
                        color: "transparent"
                        border.width: Theme.borderWidth
                        border.color: Theme.outline
                        TextInput {
                            id: hiddenSsid
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.leftMargin: Theme.paddingMd
                            anchors.rightMargin: Theme.paddingMd
                            anchors.verticalCenter: parent.verticalCenter
                            color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                            font.family: Theme.fontPrimary
                            font.pixelSize: Theme.fontSm
                            clip: true
                            onAccepted: hiddenPw.forceActiveFocus()
                            Text {
                                anchors.fill: parent
                                verticalAlignment: Text.AlignVCenter
                                text: I18n.tr("Network name (SSID)")
                                color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                                font: hiddenSsid.font
                                visible: hiddenSsid.text.length === 0 && !hiddenSsid.activeFocus
                            }
                        }
                    }
                    Rectangle {
                        width: parent.width
                        height: hiddenPw.implicitHeight + Theme.paddingSm * 2
                        radius: Theme.radiusWidget
                        color: "transparent"
                        border.width: Theme.borderWidth
                        border.color: Theme.outline
                        TextInput {
                            id: hiddenPw
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.leftMargin: Theme.paddingMd
                            anchors.rightMargin: Theme.paddingMd
                            anchors.verticalCenter: parent.verticalCenter
                            color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                            font.family: Theme.fontPrimary
                            font.pixelSize: Theme.fontSm
                            echoMode: TextInput.Password
                            clip: true
                            onAccepted: root.submitHidden(hiddenSsid.text, hiddenPw.text)
                            Text {
                                anchors.fill: parent
                                verticalAlignment: Text.AlignVCenter
                                text: I18n.tr("Password (leave empty if open)")
                                color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                                font: hiddenPw.font
                                visible: hiddenPw.text.length === 0 && !hiddenPw.activeFocus
                            }
                        }
                    }
                    Rectangle {
                        width: parent.width
                        visible: root.hiddenError
                        height: hiddenErrText.implicitHeight + Theme.paddingSm * 2
                        radius: Theme.radiusWidget
                        color: Theme.error
                        Text {
                            id: hiddenErrText
                            anchors.centerIn: parent
                            text: I18n.tr("Error Connecting")
                            color: Theme.inkOn(Theme.error, Theme.onError)
                            font.family: Theme.fontPrimary
                            font.pixelSize: Theme.fontMd
                            font.weight: Font.Bold
                        }
                    }
                    PrimaryButton {
                        width: parent.width
                        text: root.hiddenConnecting ? I18n.tr("Connecting…") : I18n.tr("Connect")
                        enabled: hiddenSsid.text.length > 0 && !root.hiddenConnecting
                        onClicked: root.submitHidden(hiddenSsid.text, hiddenPw.text)
                    }
                }
            }
        }
    }

    // A WireGuard tunnel row: collapsed to a key glyph (only while active) plus
    // its name, expanded to the actions valid for its state.
    Component {
        id: wgRowComponent

        RevealerButton {
            id: wgRow
            required property var modelData
            readonly property var tun: wgRow.modelData

            width: parent.width
            iconName: wgRow.tun.active ? "vpn_key" : ""
            label: wgRow.tun.name

            Column {
                width: parent.width
                spacing: 8

                PrimaryButton {
                    width: parent.width
                    visible: !wgRow.tun.active
                    text: I18n.tr("Connect")
                    onClicked: Network.wgActivate(wgRow.tun.uuid)
                }
                PrimaryButton {
                    width: parent.width
                    visible: wgRow.tun.active
                    text: I18n.tr("Disconnect")
                    onClicked: Network.wgDeactivate(wgRow.tun.uuid)
                }
                PrimaryButton {
                    width: parent.width
                    text: I18n.tr("Delete")
                    onClicked: Network.wgDelete(wgRow.tun.uuid)
                }
            }
        }
    }

    // An available-network row: collapsed to a strength glyph plus SSID, expanded
    // to a password form (only when secured and not already saved) and Connect.
    Component {
        id: availNetComponent

        RevealerButton {
            id: apRow
            required property var modelData
            readonly property var ap: apRow.modelData
            property bool showPassword: false
            property bool connecting: false
            property bool errorShown: false
            property int pendingId: -1

            width: parent.width
            iconName: root.wifiIcon(apRow.ap.strength)
            label: apRow.ap.ssid
            trailingText: root.multiBandSsids[apRow.ap.ssid] ? (apRow.ap.band || "") : ""

            function doConnect() {
                apRow.errorShown = false;
                apRow.connecting = true;
                var pw = (root.secured(apRow.ap) && !apRow.ap.saved) ? pwEntry.text : "";
                apRow.pendingId = Network.connectWifi(apRow.ap.ssid, pw, apRow.ap.bssid);
            }

            Connections {
                target: Network
                function onReplied(id, ok, error) {
                    if (id !== apRow.pendingId)
                        return;
                    apRow.connecting = false;
                    apRow.pendingId = -1;
                    if (!ok) {
                        apRow.errorShown = true;
                        pwEntry.text = "";
                    }
                }
            }

            Column {
                width: parent.width
                spacing: 8

                Rectangle {
                    width: parent.width
                    visible: root.secured(apRow.ap) && !apRow.ap.saved
                    height: pwEntry.implicitHeight + Theme.paddingSm * 2
                    radius: Theme.radiusWidget
                    color: "transparent"
                    border.width: Theme.borderWidth
                    border.color: Theme.outline

                    TextInput {
                        id: pwEntry
                        anchors.left: parent.left
                        anchors.right: eye.left
                        anchors.leftMargin: Theme.paddingMd
                        anchors.rightMargin: Theme.paddingSm
                        anchors.verticalCenter: parent.verticalCenter
                        color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                        font.family: Theme.fontPrimary
                        font.pixelSize: Theme.fontSm
                        echoMode: apRow.showPassword ? TextInput.Normal : TextInput.Password
                        clip: true
                        onAccepted: apRow.doConnect()
                        Text {
                            anchors.fill: parent
                            verticalAlignment: Text.AlignVCenter
                            text: I18n.tr("Password")
                            color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                            font: pwEntry.font
                            visible: pwEntry.text.length === 0 && !pwEntry.activeFocus
                        }
                    }
                    MaterialIcon {
                        id: eye
                        anchors.right: parent.right
                        anchors.rightMargin: Theme.paddingSm
                        anchors.verticalCenter: parent.verticalCenter
                        width: Theme.iconSm
                        height: Theme.iconSm
                        font.pixelSize: Theme.iconSm
                        text: apRow.showPassword ? "visibility_off" : "visibility"
                        color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                        TapHandler { onTapped: apRow.showPassword = !apRow.showPassword }
                    }
                }

                Rectangle {
                    width: parent.width
                    visible: apRow.errorShown
                    height: errText.implicitHeight + Theme.paddingSm * 2
                    radius: Theme.radiusWidget
                    color: Theme.error
                    Text {
                        id: errText
                        anchors.centerIn: parent
                        text: I18n.tr("Error Connecting")
                        color: Theme.inkOn(Theme.error, Theme.onError)
                        font.family: Theme.fontPrimary
                        font.pixelSize: Theme.fontMd
                        font.weight: Font.Bold
                    }
                }

                PrimaryButton {
                    width: parent.width
                    text: I18n.tr("Connect")
                    enabled: !apRow.connecting
                    onClicked: apRow.doConnect()
                }
            }
        }
    }
}

import QtQuick
import "../modules"
import "../components"
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import Ryoku.Ui.Singletons

PanelWindow {
    id: netPanel
    required property var root

    screen: root.activePopupScreen

    color: "transparent"
    anchors { top: true; bottom: true; left: true; right: true }
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "ryoku-network"

    readonly property int barBottom: root.v2BarHeight
    readonly property int gap: 6

    property string mode:  "none"   // wifi | ethernet | none
    property string ssid:  ""
    property int    signal: 0
    property string iface: ""
    property string ipAddr: ""
    property string freq:  ""

    property string wdev:   ""      // wifi device name (e.g. wlan0)
    property bool   hasWifi: false
    property bool   scanning: false
    property var    networks: []    // [{conn, ssid, sec, sig}]
    property var    known:   []     // known ssids
    property bool   savedOnly: false
    property string selectedNetworkKey: ""
    property string pendingForgetKey: ""
    property int    keyboardIndex: -1
    readonly property var shownNetworks: networks.filter(function(entry) {
        return savedOnly ? entry.known === true : entry.visible !== false
    })
    readonly property int savedCount: {
        var count = 0
        for (var i = 0; i < networks.length; i++)
            if (networks[i].known === true) count++
        return count
    }

    function networkKey(entry) {
        if (!entry)
            return ""
        return entry.entryKey || "ssid:" + (entry.ssid || "")
    }

    property string nmPasswordSsid: ""
    property string nmPasswordText: ""
    property var    nmPasswordNetwork: null
    property string nmConnectionError: ""
    property bool   nmConnecting: false
    property string networkActionError: ""
    property bool   nmProfilesLoaded: false

    property bool   hiddenNetworkOpen: false
    property string hiddenName: ""
    property string hiddenSsid: ""
    property string hiddenPassword: ""
    property string hiddenSecurity: "wpa-psk"
    property string hiddenConnectionError: ""
    property bool   hiddenConnecting: false

    // ── wifi radio ──
    property bool   wifiBlocked: false

    // ── link speed (negotiated connection rate) ──
    property string linkSpeed:   ""

    readonly property bool nmAdapterReady: root.useNM
        && nmAdapter.status === Loader.Ready
        && nmAdapter.item !== null
        && nmAdapter.item.available

    function flagForCountry(code) {
        var value = (code || "").toUpperCase()
        if (!/^[A-Z]{2}$/.test(value)) return ""
        return String.fromCharCode(
            0xD83C, 0xDDE6 + value.charCodeAt(0) - 65,
            0xD83C, 0xDDE6 + value.charCodeAt(1) - 65)
    }

    function formatMbps(value) {
        if (!(value > 0)) return "-"
        return (value >= 100 ? value.toFixed(0) : value.toFixed(1)) + " Mbps"
    }

    function formatPing(value) {
        if (!(value > 0)) return "-"
        return (value < 10 ? value.toFixed(1) : value.toFixed(0)) + " ms"
    }

    function edgeText() {
        if (speedTest.phase === "idle" || speedTest.phase === "cancelled") return I18n.tr("Not tested")
        if (speedTest.phase === "offline") return I18n.tr("Offline")
        if (speedTest.phase === "error" || speedTest.phase === "timeout") return I18n.tr("Unavailable")
        if (speedTest.phase === "latency") return I18n.tr("Locating…")
        var edge = speedTest.edgeCode !== "" ? "Cloudflare · " + speedTest.edgeCode : "Cloudflare Edge"
        var flag = flagForCountry(speedTest.countryCode)
        return edge + (flag !== "" ? " " + flag : "")
    }

    // timestamp captured when a run completes - shown in the green "done" footer
    property string lastTestStamp: ""
    property bool speedTestAttempted: false

    CloudflareSpeedTest {
        id: speedTest
        // live, 2 s-polled source (NetworkWidget → root.networkMode mirror) so a mid-test
        // disconnect flips online→false at once and onOnlineChanged shows "Offline",
        // instead of surfacing later as an XHR error/timeout. netPanel.mode (open-only) stays
        // the source for the panel's detail rows.
        online: root.networkMode !== "none"
    }

    Connections {
        target: speedTest
        function onPhaseChanged() {
            if (speedTest.phase === "success")
                netPanel.lastTestStamp = new Date().toLocaleString(Qt.locale("en_US"), "HH:mm · d MMM")
        }
    }

    // ✓ marks show only on a healthy run (in progress or finished ok) - never on error/cancel/offline
    readonly property bool speedRunOk: speedTest.running || speedTest.phase === "success"
    readonly property bool speedDetailsVisible: speedTestAttempted
        && speedTest.phase !== "idle"
        && speedTest.phase !== "cancelled"

    function toggleWifi() {
        if (nmAdapterReady) {
            nmAdapter.item.toggleWifi()
            return
        }
        if (root.useNM) {
            root.networkVisible = false
            openWifiSettings()
            return
        }

        var wasBlocked = netPanel.wifiBlocked
        rfkillToggle.command = ["bash", "-c", wasBlocked ? "rfkill unblock wifi" : "rfkill block wifi"]
        rfkillToggle.running = false; rfkillToggle.running = true
        netPanel.wifiBlocked = !wasBlocked      // optimistic; rfkillState corrects
        Qt.callLater(function() {
            rfkillState.running = false; rfkillState.running = true
            netData.running = false; netData.running = true
            if (wasBlocked) netPanel.scan()     // just turned ON → look for networks
        })
    }

    function scan() {
        if (nmAdapterReady) {
            nmAdapter.item.scan()
            return
        }
        if (scanning || wifiBlocked || root.useNM) return   // NM fallback: no iwctl scan
        scanning = true
        scanProc.running = false
        scanProc.running = true
        scanWatchdog.restart()        // never stay stuck in "scanning"
    }

    function connectTo(entryOrSsid, sec) {
        if (nmAdapterReady) {
            nmAdapter.item.connectTo(entryOrSsid)
            return
        }

        var ssid = typeof entryOrSsid === "object" && entryOrSsid !== null ? entryOrSsid.ssid : entryOrSsid
        var isKnown = known.indexOf(ssid) >= 0
        if (sec === "open" || isKnown) {
            if (!netPanel.wdev) return
            // argv form (no shell) → a crafted SSID cannot inject commands
            connectProc.command = ["iwctl", "station", netPanel.wdev, "connect", ssid]
            connectProc.running = false
            connectProc.running = true
            // re-scan shortly to reflect new connection
            rescanTimer.restart()
        } else {
            // unknown secured network - needs passphrase → open impala
            root.networkVisible = false
            wifiRunner.running = false
            wifiRunner.running = true
        }
    }

    function activateNetwork(entry) {
        if (!entry)
            return

        networkActionError = ""

        if (entry.conn) {
            if (nmAdapterReady && entry.network)
                entry.network.disconnect()
            else if (wdev !== "") {
                connectProc.command = ["iwctl", "station", wdev, "disconnect"]
                connectProc.running = false
                connectProc.running = true
            }
            rescanTimer.restart()
            return
        }

        connectTo(entry, entry.sec)
    }

    function forgetNetwork(entry) {
        if (!entry || !entry.known)
            return

        cancelForget()
        selectedNetworkKey = ""
        if (nmAdapterReady) {
            nmAdapter.item.forgetNetwork(entry)
            refreshNmNetworks()
        } else {
            forgetProc.command = ["iwctl", "known-networks", entry.ssid, "forget"]
            forgetProc.running = false
            forgetProc.running = true
        }
    }

    function selectNetwork(entry) {
        if (!entry)
            return
        cancelForget()
        var key = networkKey(entry)
        selectedNetworkKey = selectedNetworkKey === key ? "" : key
    }

    function isNeverConnected(entry) {
        return root.useNM && nmProfilesLoaded && entry && entry.known
            && !(Number(entry.lastSuccessful || 0) > 0)
    }

    function protectionLabel(entry) {
        if (!entry)
            return I18n.tr("Unknown")
        if (entry.securityLabel)
            return entry.securityLabel
        switch (entry.sec || "") {
        case "open": return I18n.tr("Open")
        case "psk": return "PSK"
        case "8021x": return "802.1X"
        case "wep": return "WEP"
        case "saved": return I18n.tr("Saved Wi-Fi profile")
        default: return I18n.tr("Unknown")
        }
    }

    function cancelForget() {
        pendingForgetKey = ""
        forgetConfirmTimer.stop()
    }

    function requestForget(entry) {
        if (!entry || !entry.known)
            return
        var key = networkKey(entry)
        if (pendingForgetKey === key) {
            cancelForget()
            forgetNetwork(entry)
            return
        }
        pendingForgetKey = key
        forgetConfirmTimer.restart()
    }

    function resetNetworkSelection() {
        cancelForget()
        selectedNetworkKey = ""
        keyboardIndex = -1
    }

    function ensureKeyboardNetworkVisible() {
        if (keyboardIndex < 0 || keyboardIndex >= networkRepeater.count)
            return
        var item = networkRepeater.itemAt(keyboardIndex)
        if (!item)
            return
        var top = item.y
        var bottom = top + item.height
        if (top < networkFlick.contentY)
            networkFlick.contentY = top
        else if (bottom > networkFlick.contentY + networkFlick.height)
            networkFlick.contentY = Math.min(networkFlick.contentHeight - networkFlick.height,
                                            bottom - networkFlick.height)
    }

    onSavedOnlyChanged: resetNetworkSelection()


    function openHiddenNetwork() {
        clearNmPassword()
        networkActionError = ""
        hiddenNetworkOpen = true
        hiddenName = ""
        hiddenSsid = ""
        hiddenPassword = ""
        hiddenSecurity = "wpa-psk"
        hiddenConnectionError = ""
        hiddenConnecting = false
        Qt.callLater(function() {
            if (hiddenNameInput.visible)
                hiddenNameInput.forceActiveFocus()
        })
    }

    function clearHiddenNetwork() {
        hiddenNetworkOpen = false
        hiddenName = ""
        hiddenSsid = ""
        hiddenPassword = ""
        hiddenSecurity = "wpa-psk"
        hiddenConnectionError = ""
        hiddenConnecting = false
    }

    function cycleHiddenSecurity() {
        if (hiddenSecurity === "open")
            hiddenSecurity = "wpa-psk"
        else if (hiddenSecurity === "wpa-psk")
            hiddenSecurity = "sae"
        else
            hiddenSecurity = "open"
        hiddenConnectionError = ""
    }

    function hiddenSecurityLabel() {
        switch (hiddenSecurity) {
        case "open": return I18n.tr("Open")
        case "sae": return I18n.tr("WPA3 Personal")
        default: return I18n.tr("WPA/WPA2 Personal")
        }
    }

    function submitHiddenNetwork() {
        if (!nmAdapterReady || hiddenConnecting)
            return

        var connectionName = hiddenName.trim()
        var ssid = hiddenSsid.trim()
        if (connectionName === "" || ssid === "")
            return

        if (hiddenSecurity !== "open" && hiddenPassword === "") {
            hiddenConnectionError = I18n.tr("Password is required")
            return
        }

        hiddenConnectionError = ""
        hiddenConnecting = true
        nmAdapter.item.connectHidden(connectionName, ssid, hiddenPassword, hiddenSecurity)
    }

    function openWifiSettings() {
        wifiRunner.running = false
        wifiRunner.running = true
    }

    function refreshNmNetworks() {
        if (nmAdapterReady)
            nmAdapter.item.syncNetworks()
    }

    function beginNmPassword(entry) {
        if (!entry || !entry.network)
            return

        nmPasswordSsid = entry.ssid || ""
        nmPasswordNetwork = entry.network
        nmPasswordText = ""
        nmConnectionError = ""
        nmConnecting = false
        nmConnectTimeout.stop()
        Qt.callLater(function() {
            if (nmPasswordInput.visible)
                nmPasswordInput.forceActiveFocus()
        })
    }

    function clearNmPassword() {
        nmPasswordSsid = ""
        nmPasswordText = ""
        nmPasswordNetwork = null
        nmConnectionError = ""
        nmConnecting = false
        nmConnectTimeout.stop()
    }

    function submitNmPassword() {
        if (!nmAdapterReady || !nmPasswordNetwork || nmPasswordText === "" || nmConnecting)
            return

        nmConnectionError = ""
        nmConnecting = true
        nmConnectTimeout.restart()
        nmAdapter.item.connectWithPsk(nmPasswordNetwork, nmPasswordText)
    }

    function handleNmConnected(network) {
        if (!network)
            return

        var name = network.name || ""
        if (name === nmPasswordSsid)
            clearNmPassword()
    }

    function handleNmConnectionFailed(network, reason) {
        if (!network)
            return

        var name = network.name || ""
        if (name !== nmPasswordSsid)
            return

        nmConnecting = false
        nmConnectTimeout.stop()
        nmConnectionError = I18n.tr("Connection failed")
        Qt.callLater(function() {
            if (nmPasswordInput.visible)
                nmPasswordInput.forceActiveFocus()
        })
    }

    property real reveal: root.networkVisible ? 1 : 0
    Behavior on reveal {
        NumberAnimation {
            duration: root.networkVisible ? 160 : 120
            easing.type: root.networkVisible ? Easing.OutCubic : Easing.InCubic
        }
    }
    visible: reveal > 0.001
    WlrLayershell.keyboardFocus: root.networkVisible ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    MouseArea { anchors.fill: parent; onClicked: root.networkVisible = false }

    Rectangle {
        id: card
        width: 300
        height: col.implicitHeight + 24
        radius: reveal > 0.001 ? root.panelRadius : 0
        color: "transparent"
        border.color: root.panelBorder
        border.width: 0
        PillShadow { theme: root }
        ConnectedPanelSurface {
            root: netPanel.root
            ownerActive: netPanel.root.networkVisible
            targetX: netPanel.root.networkBarX
            reveal: netPanel.reveal
        }

        x: Math.round(Math.max(6, Math.min(root.networkBarX - width / 2, parent.width - width - 6)))
        y: root.barPosition === "bottom"
            ? (parent.height - barBottom - gap - height) + 2 * (1 - netPanel.reveal)
            : (barBottom + gap) - 2 * (1 - netPanel.reveal)
        opacity: netPanel.reveal
        focus: root.networkVisible

        Keys.onPressed: function(event) {
            if (event.key === Qt.Key_Escape) {
                if (netPanel.pendingForgetKey !== "")
                    netPanel.cancelForget()
                else if (netPanel.selectedNetworkKey !== "")
                    netPanel.selectedNetworkKey = ""
                else
                    root.networkVisible = false
                event.accepted = true
                return
            }

            if (netPanel.nmPasswordSsid !== "" || netPanel.hiddenNetworkOpen)
                return

            var entries = netPanel.shownNetworks
            if (entries.length === 0)
                return

            if (event.key === Qt.Key_Down) {
                netPanel.keyboardIndex = (netPanel.keyboardIndex + 1) % entries.length
                Qt.callLater(netPanel.ensureKeyboardNetworkVisible)
                event.accepted = true
            } else if (event.key === Qt.Key_Up) {
                netPanel.keyboardIndex = netPanel.keyboardIndex <= 0
                    ? entries.length - 1 : netPanel.keyboardIndex - 1
                Qt.callLater(netPanel.ensureKeyboardNetworkVisible)
                event.accepted = true
            } else if (event.key === Qt.Key_Right) {
                if (netPanel.keyboardIndex >= 0)
                    netPanel.selectedNetworkKey = netPanel.networkKey(entries[netPanel.keyboardIndex])
                event.accepted = true
            } else if (event.key === Qt.Key_Left) {
                netPanel.cancelForget()
                netPanel.selectedNetworkKey = ""
                event.accepted = true
            } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                if (netPanel.keyboardIndex >= 0)
                    netPanel.activateNetwork(entries[netPanel.keyboardIndex])
                event.accepted = true
            }
        }

        MouseArea { anchors.fill: parent; onClicked: {} }

        Column {
            id: col
            anchors.fill: parent
            anchors.margins: 12
            spacing: 8

            // ── header ──
            Item {
                width: parent.width
                height: 24
                UiText {
                    anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Network")
                    color: root.ink; font.family: root.mono; font.pixelSize: 13
                    font.letterSpacing: 2; font.weight: Font.Medium
                }
                UiText {
                    anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                    text: "✕"; color: closeMa.containsMouse ? root.seal : root.sumi; font.pixelSize: 12
                    Behavior on color { ColorAnimation { duration: 120 } }
                    MouseArea { id: closeMa; anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor; onClicked: root.networkVisible = false }
                }
            }

            Rectangle { width: parent.width; height: 1; color: root.sep }

            // ── status ──
            Item {
                width: parent.width
                height: 30
                UiText {
                    anchors.horizontalCenter: parent.horizontalCenter
                    anchors.top: parent.top
                    text: {
                        if (netPanel.mode === "wifi")     return netPanel.signal + "%"
                        if (netPanel.mode === "ethernet") return I18n.tr("Connected")
                        return I18n.tr("Offline")
                    }
                    color: netPanel.mode === "none" ? root.sumi : root.seal
                    font.family: root.mono; font.pixelSize: 11; font.weight: Font.Medium
                }
                Rectangle {
                    anchors.bottom: parent.bottom
                    width: parent.width; height: 8; radius: 4
                    color: root.fillActive
                    Rectangle {
                        width: parent.width * (netPanel.mode === "wifi" ? netPanel.signal / 100 : (netPanel.mode === "ethernet" ? 1 : 0))
                        height: parent.height; radius: 4; color: root.seal
                        Behavior on width { NumberAnimation { duration: 300 } }
                    }
                }
            }

            // ── details ──
            Column {
                width: parent.width
                spacing: 4
                Row {
                    width: parent.width
                    visible: netPanel.mode === "wifi"
                    UiText { text: I18n.tr("SSID"); color: root.sumiHi; font.family: root.mono; font.pixelSize: 11; width: parent.width * 0.4 }
                    UiText { text: netPanel.ssid; color: root.ink; font.family: root.mono; font.pixelSize: 11; width: parent.width * 0.6; elide: Text.ElideRight }
                }
                Row {
                    width: parent.width
                    UiText { text: I18n.tr("Type"); color: root.sumiHi; font.family: root.mono; font.pixelSize: 11; width: parent.width * 0.4 }
                    UiText {
                        text: netPanel.mode === "wifi" ? I18n.tr("Wi-Fi") : (netPanel.mode === "ethernet" ? I18n.tr("Ethernet") : "-")
                        color: root.ink; font.family: root.mono; font.pixelSize: 11
                    }
                }
                Row {
                    width: parent.width
                    visible: netPanel.iface !== ""
                    UiText { text: I18n.tr("Interface"); color: root.sumiHi; font.family: root.mono; font.pixelSize: 11; width: parent.width * 0.4 }
                    UiText { text: netPanel.iface; color: root.ink; font.family: root.mono; font.pixelSize: 11 }
                }
                Row {
                    width: parent.width
                    visible: netPanel.ipAddr !== ""
                    UiText { text: "IP"; color: root.sumiHi; font.family: root.mono; font.pixelSize: 11; width: parent.width * 0.4 }
                    UiText { text: netPanel.ipAddr; color: root.ink; font.family: root.mono; font.pixelSize: 11 }
                }
                Row {
                    width: parent.width
                    visible: netPanel.mode === "wifi" && netPanel.freq !== ""
                    UiText { text: I18n.tr("Frequency"); color: root.sumiHi; font.family: root.mono; font.pixelSize: 11; width: parent.width * 0.4 }
                    UiText { text: netPanel.freq; color: root.ink; font.family: root.mono; font.pixelSize: 11 }
                }
                Row {
                    width: parent.width
                    visible: netPanel.linkSpeed !== ""
                    UiText { text: I18n.tr("Link speed"); color: root.sumiHi; font.family: root.mono; font.pixelSize: 11; width: parent.width * 0.4 }
                    UiText { text: netPanel.linkSpeed; color: root.ink; font.family: root.mono; font.pixelSize: 11 }
                }
            }

            Rectangle { width: parent.width; height: 1; color: root.sep }

            Column {
                width: parent.width
                spacing: 4

                Item {
                    width: parent.width
                    height: 24

                    UiText {
                        anchors.left: parent.left
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("SPEED TEST")
                        color: root.sumiHi
                        font.family: root.mono
                        font.pixelSize: 10
                        font.letterSpacing: 1
                    }

                    // action: a real button in the panel's hover idiom (network-row style)
                    Rectangle {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        width: 54
                        height: 22
                        radius: root.panelButtonRadius
                        color: speedTestMa.containsMouse ? root.fillHover : root.fillIdle
                        border.color: speedTestMa.containsMouse ? root.seal : root.sep
                        border.width: 1
                        Behavior on color { ColorAnimation { duration: 120 } }

                        UiText {
                            anchors.centerIn: parent
                            text: speedTest.running ? I18n.tr("stop") : I18n.tr("start")
                            color: speedTestMa.enabled ? root.seal : root.sumi
                            font.family: root.mono
                            font.pixelSize: 11
                        }

                        MouseArea {
                            id: speedTestMa
                            anchors.fill: parent
                            enabled: speedTest.running || netPanel.mode !== "none"
                            hoverEnabled: true
                            cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                            onClicked: {
                                if (speedTest.running) {
                                    speedTest.cancel()
                                    netPanel.speedTestAttempted = false
                                } else {
                                    netPanel.speedTestAttempted = true
                                    speedTest.start()
                                }
                            }
                        }
                    }
                }

                Row {
                    width: parent.width
                    visible: netPanel.speedDetailsVisible
                    UiText { text: I18n.tr("Edge"); color: root.sumiHi; font.family: root.mono; font.pixelSize: 11; width: parent.width * 0.4 }
                    UiText { text: netPanel.edgeText(); color: root.ink; font.family: root.mono; font.pixelSize: 11; width: parent.width * 0.6; elide: Text.ElideRight }
                }
                Item {
                    width: parent.width; height: 16
                    visible: netPanel.speedDetailsVisible
                    UiText {
                        id: pingLabel
                        anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Ping"); color: root.sumiHi; font.family: root.mono; font.pixelSize: 11
                        width: parent.width * 0.4
                    }
                    UiText {
                        anchors.left: pingLabel.right; anchors.right: pingCheck.left; anchors.rightMargin: 6
                        anchors.verticalCenter: parent.verticalCenter
                        text: speedTest.phase === "latency" ? I18n.tr("Testing…") : (netPanel.speedRunOk ? netPanel.formatPing(speedTest.pingMs) : "-")
                        color: root.ink; font.family: root.mono; font.pixelSize: 11; elide: Text.ElideRight
                    }
                    UiText {
                        id: pingCheck
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        text: "✓"; visible: speedTest.pingMs > 0 && netPanel.speedRunOk
                        color: root.green; font.family: root.mono; font.pixelSize: 11
                    }
                }
                Item {
                    width: parent.width; height: 16
                    visible: netPanel.speedDetailsVisible
                    UiText {
                        id: dlLabel
                        anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Download"); color: root.sumiHi; font.family: root.mono; font.pixelSize: 11
                        width: parent.width * 0.4
                    }
                    UiText {
                        anchors.left: dlLabel.right; anchors.right: dlCheck.left; anchors.rightMargin: 6
                        anchors.verticalCenter: parent.verticalCenter
                        text: speedTest.phase === "download" ? I18n.tr("Testing…") : (netPanel.speedRunOk ? netPanel.formatMbps(speedTest.downloadMbps) : "-")
                        color: (speedTest.downloadMbps > 0 && netPanel.speedRunOk) ? root.seal : root.ink
                        font.family: root.mono; font.pixelSize: 11; elide: Text.ElideRight
                    }
                    UiText {
                        id: dlCheck
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        text: "✓"; visible: speedTest.downloadMbps > 0 && netPanel.speedRunOk
                        color: root.green; font.family: root.mono; font.pixelSize: 11
                    }
                }
                Item {
                    width: parent.width; height: 16
                    visible: netPanel.speedDetailsVisible
                    UiText {
                        id: ulLabel
                        anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Upload"); color: root.sumiHi; font.family: root.mono; font.pixelSize: 11
                        width: parent.width * 0.4
                    }
                    UiText {
                        anchors.left: ulLabel.right; anchors.right: ulCheck.left; anchors.rightMargin: 6
                        anchors.verticalCenter: parent.verticalCenter
                        text: speedTest.phase === "upload" ? I18n.tr("Testing…") : (netPanel.speedRunOk ? netPanel.formatMbps(speedTest.uploadMbps) : "-")
                        color: (speedTest.uploadMbps > 0 && netPanel.speedRunOk) ? root.indigo : root.ink
                        font.family: root.mono; font.pixelSize: 11; elide: Text.ElideRight
                    }
                    UiText {
                        id: ulCheck
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        text: "✓"; visible: speedTest.uploadMbps > 0 && netPanel.speedRunOk
                        color: root.green; font.family: root.mono; font.pixelSize: 11
                    }
                }

                // animated height so the card grows/shrinks smoothly instead of snapping
                Item {
                    width: parent.width
                    height: speedFooter.visible ? speedFooter.implicitHeight : 0
                    clip: true
                    Behavior on height { NumberAnimation { duration: 140; easing.type: Easing.OutCubic } }

                    UiText {
                        id: speedFooter
                        width: parent.width
                        visible: speedTest.phase === "success" && netPanel.lastTestStamp !== ""
                        text: I18n.tr("done · %1").arg(netPanel.lastTestStamp)
                        color: root.green
                        font.family: root.mono
                        font.pixelSize: 10
                        font.letterSpacing: 1
                    }
                }
            }

            Rectangle { width: parent.width; height: 1; color: root.sep }

            DnsProviderSection {
                width: parent.width
                theme: root
                buttonRadius: root.panelButtonRadius
            }

            Rectangle { width: parent.width; height: 1; color: root.sep; visible: netPanel.hasWifi }

            // ── wifi radio toggle ──
            Item {
                width: parent.width
                height: 24
                visible: netPanel.hasWifi
                UiText {
                    anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Wi-Fi")
                    color: root.ink; font.family: root.mono; font.pixelSize: 11
                }
                Rectangle {
                    anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                    width: 50; height: 22; radius: 11
                    color: !netPanel.wifiBlocked ? root.fillActive
                                                 : wifiToggleMa.containsMouse ? root.fillHover
                                                 : root.fillIdle
                    border.color: (wifiToggleMa.containsMouse || !netPanel.wifiBlocked) ? root.seal : root.sep
                    border.width: 1
                    Behavior on color { ColorAnimation { duration: 120 } }
                    UiText {
                        anchors.centerIn: parent
                        text: netPanel.wifiBlocked ? I18n.tr("OFF") : I18n.tr("ON")
                        color: !netPanel.wifiBlocked ? root.seal : root.sumi
                        font.family: root.mono; font.pixelSize: 10; font.weight: Font.Medium
                    }
                    MouseArea {
                        id: wifiToggleMa
                        anchors.fill: parent; hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: netPanel.toggleWifi()
                    }
                }
            }

            // Available / saved profiles. Both views use the native
            // NetworkManager objects on Ryoku 4 and the iwctl snapshot on
            // legacy installations.
            Row {
                width: parent.width
                height: 28
                spacing: 6
                visible: netPanel.hasWifi && !netPanel.wifiBlocked && (!root.useNM || netPanel.nmAdapterReady)

                Repeater {
                    model: [
                        { label: I18n.tr("Available"), saved: false },
                        { label: I18n.tr("Saved") + (netPanel.savedCount > 0 ? " (" + netPanel.savedCount + ")" : ""), saved: true }
                    ]
                    delegate: Rectangle {
                        required property var modelData
                        width: (parent.width - 6) / 2
                        height: 28
                        radius: root.panelButtonRadius
                        readonly property bool active: netPanel.savedOnly === modelData.saved
                        color: active ? root.fillActive : tabMa.containsMouse ? root.fillHover : root.fillIdle
                        border.color: active || tabMa.containsMouse ? root.seal : root.sep
                        border.width: 1
                        Behavior on color { ColorAnimation { duration: 120 } }

                        UiText {
                            anchors.centerIn: parent
                            text: modelData.label
                            color: parent.active ? root.seal : root.ink
                            font.family: root.mono
                            font.pixelSize: 10
                        }
                        MouseArea {
                            id: tabMa
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                netPanel.savedOnly = modelData.saved
                                if (!modelData.saved) netPanel.scan()
                            }
                        }
                    }
                }
            }

            Item {
                width: parent.width
                height: 16
                visible: netPanel.hasWifi && !netPanel.wifiBlocked && (!root.useNM || netPanel.nmAdapterReady)
                UiText {
                    anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                    text: netPanel.savedOnly ? I18n.tr("SAVED NETWORKS") : I18n.tr("AVAILABLE NETWORKS")
                    color: root.sumiHi; font.family: root.mono; font.pixelSize: 10; font.letterSpacing: 1
                }
                UiText {
                    anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                    visible: !netPanel.savedOnly
                    text: netPanel.scanning ? I18n.tr("scanning…") : I18n.tr("rescan")
                    color: rescanMa.containsMouse ? root.fillPrimaryHover : root.seal
                    font.family: root.mono; font.pixelSize: 10
                    Behavior on color { ColorAnimation { duration: 120 } }
                    MouseArea { id: rescanMa; anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor; onClicked: netPanel.scan() }
                }
            }

            // scrollable network list
            Flickable {
                id: networkFlick
                width: parent.width
                height: Math.min(netList.implicitHeight, 180)
                contentHeight: netList.implicitHeight
                clip: true
                visible: netPanel.hasWifi && !netPanel.wifiBlocked && (!root.useNM || netPanel.nmAdapterReady)
                boundsBehavior: Flickable.StopAtBounds

                Column {
                    id: netList
                    width: parent.width
                    spacing: 4

                    Repeater {
                        id: networkRepeater
                        model: netPanel.shownNetworks
                        delegate: Column {
                            id: netTile
                            required property var modelData
                            required property int index
                            width: netList.width
                            spacing: 4
                            readonly property bool expanded: netPanel.selectedNetworkKey === netPanel.networkKey(modelData)
                            readonly property bool keyboardSelected: netPanel.keyboardIndex === index
                            readonly property bool confirmingForget: netPanel.pendingForgetKey === netPanel.networkKey(modelData)

                            Connections {
                                target: root.useNM && modelData.network ? modelData.network : null
                                function onConnectedChanged() {
                                    if (modelData.network && modelData.network.connected)
                                        netPanel.handleNmConnected(modelData.network)
                                    netPanel.refreshNmNetworks()
                                }
                                function onKnownChanged() { netPanel.refreshNmNetworks() }
                                function onStateChangingChanged() { netPanel.refreshNmNetworks() }
                                function onSignalStrengthChanged() { netPanel.refreshNmNetworks() }
                                function onConnectionFailed(reason) {
                                    netPanel.handleNmConnectionFailed(modelData.network, reason)
                                    netPanel.refreshNmNetworks()
                                }
                            }

                            Rectangle {
                                width: parent.width
                                height: 30
                                radius: root.panelButtonRadius
                                readonly property bool active: nma.containsMouse || netTile.expanded || netTile.keyboardSelected
                                color: modelData.conn ? root.fillActive : active ? root.fillHover : root.fillIdle
                                border.color: modelData.conn || active ? root.seal : root.sep
                                border.width: 1
                                Behavior on color { ColorAnimation { duration: 120 } }

                                Row {
                                    anchors.left: parent.left; anchors.leftMargin: 8
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: 6
                                    IconText {
                                        text: modelData.sec === "open" ? "\uE898" : "\uE897"
                                        font.pixelSize: 12
                                        color: root.sumiHi
                                        anchors.verticalCenter: parent.verticalCenter
                                    }
                                    UiText {
                                        text: modelData.ssid
                                        color: (nma.containsMouse || modelData.conn) ? root.seal : root.ink
                                        font.family: root.mono; font.pixelSize: 11
                                        font.weight: modelData.conn ? Font.Medium : Font.Normal
                                        width: modelData.conn ? 116 : 170; elide: Text.ElideRight
                                        anchors.verticalCenter: parent.verticalCenter
                                    }
                                    UiText {
                                        visible: modelData.conn
                                        text: I18n.tr("· Connected")
                                        color: root.seal
                                        font.family: root.mono; font.pixelSize: 9
                                        anchors.verticalCenter: parent.verticalCenter
                                    }
                                }

                                Row {
                                    anchors.right: detailButton.left; anchors.rightMargin: 4
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: 8
                                    UiText {
                                        visible: modelData.known && !modelData.conn
                                        text: netPanel.isNeverConnected(modelData) ? I18n.tr("profile") : I18n.tr("saved")
                                        color: root.sumiHi
                                        font.family: root.mono
                                        font.pixelSize: 9
                                    }
                                    Row {
                                        spacing: 2
                                        Repeater {
                                            model: 4
                                            delegate: Rectangle {
                                                required property int index
                                                width: 3; height: 4 + index * 2; radius: 1
                                                anchors.bottom: parent.bottom
                                                color: index < modelData.sig
                                                    ? (modelData.conn ? root.seal : root.ink)
                                                    : Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.18)
                                            }
                                        }
                                    }
                                }

                                MouseArea {
                                    id: nma
                                    anchors.left: parent.left; anchors.top: parent.top; anchors.bottom: parent.bottom
                                    anchors.right: detailButton.left
                                    hoverEnabled: true
                                    cursorShape: Qt.PointingHandCursor
                                    onEntered: netPanel.keyboardIndex = netTile.index
                                    onClicked: netPanel.activateNetwork(modelData)
                                }

                                UiText {
                                    id: detailButton
                                    anchors.right: parent.right; anchors.rightMargin: 7
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: 16
                                    horizontalAlignment: Text.AlignHCenter
                                    text: netTile.expanded ? "⌃" : "›"
                                    color: detailMa.containsMouse || netTile.expanded ? root.seal : root.sumiHi
                                    font.family: root.mono; font.pixelSize: 13
                                    MouseArea {
                                        id: detailMa
                                        anchors.fill: parent
                                        hoverEnabled: true
                                        cursorShape: Qt.PointingHandCursor
                                        onEntered: netPanel.keyboardIndex = netTile.index
                                        onClicked: netPanel.selectNetwork(modelData)
                                    }
                                }
                            }

                            Rectangle {
                                width: parent.width
                                height: netTile.expanded ? detailColumn.implicitHeight + 16 : 0
                                visible: height > 0
                                clip: true
                                radius: root.panelButtonRadius
                                color: root.fillIdle
                                border.color: root.sep
                                border.width: netTile.expanded ? 1 : 0
                                Behavior on height { NumberAnimation { duration: 160; easing.type: Easing.OutCubic } }

                                Column {
                                    id: detailColumn
                                    anchors.left: parent.left; anchors.right: parent.right
                                    anchors.top: parent.top
                                    anchors.margins: 8
                                    spacing: 6

                                    UiText {
                                        width: parent.width
                                        text: {
                                            var details = [
                                                netPanel.protectionLabel(modelData),
                                                modelData.visible === false
                                                    ? I18n.tr("Not currently visible")
                                                    : I18n.tr("Signal %1%").arg(modelData.sig * 25)
                                            ]
                                            if (modelData.known)
                                                details.push(netPanel.isNeverConnected(modelData)
                                                    ? I18n.tr("Never connected") : I18n.tr("Saved"))
                                            return details.join(" · ")
                                        }
                                        color: root.sumiHi
                                        font.family: root.mono; font.pixelSize: 10
                                        wrapMode: Text.Wrap
                                    }

                                    Row {
                                        width: parent.width
                                        height: 26
                                        spacing: 6

                                        Rectangle {
                                            width: modelData.known ? (parent.width - 6) / 2 : parent.width
                                            height: parent.height
                                            radius: root.panelButtonRadius
                                            color: networkActionMa.containsMouse ? root.fillHover : root.fillIdle
                                            border.color: networkActionMa.containsMouse ? root.seal : root.sep
                                            border.width: 1
                                            UiText {
                                                anchors.centerIn: parent
                                                text: netTile.confirmingForget
                                                    ? I18n.tr("Cancel")
                                                    : modelData.conn ? I18n.tr("Disconnect") : modelData.known ? I18n.tr("Reconnect") : I18n.tr("Connect")
                                                color: root.ink
                                                font.family: root.mono; font.pixelSize: 10
                                            }
                                            MouseArea {
                                                id: networkActionMa
                                                anchors.fill: parent
                                                hoverEnabled: true
                                                cursorShape: Qt.PointingHandCursor
                                                onClicked: {
                                                    if (netTile.confirmingForget)
                                                        netPanel.cancelForget()
                                                    else
                                                        netPanel.activateNetwork(modelData)
                                                }
                                            }
                                        }

                                        Rectangle {
                                            visible: modelData.known
                                            width: (parent.width - 6) / 2
                                            height: parent.height
                                            radius: root.panelButtonRadius
                                            color: forgetMa.containsMouse ? Qt.rgba(root.seal.r, root.seal.g, root.seal.b, 0.18) : root.fillIdle
                                            border.color: forgetMa.containsMouse ? root.seal : root.sep
                                            border.width: 1
                                            UiText {
                                                anchors.centerIn: parent
                                                text: netTile.confirmingForget ? I18n.tr("Confirm") : I18n.tr("Forget")
                                                color: forgetMa.containsMouse ? root.seal : root.ink
                                                font.family: root.mono; font.pixelSize: 10
                                            }
                                            MouseArea {
                                                id: forgetMa
                                                anchors.fill: parent
                                                hoverEnabled: true
                                                cursorShape: Qt.PointingHandCursor
                                                onClicked: netPanel.requestForget(modelData)
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }

                    UiText {
                        visible: !netPanel.scanning && netPanel.shownNetworks.length === 0
                        width: netList.width; horizontalAlignment: Text.AlignHCenter
                        text: netPanel.savedOnly ? I18n.tr("No saved networks") : I18n.tr("No networks found")
                        color: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.3)
                        font.family: root.mono; font.pixelSize: 11
                    }
                    UiText {
                        visible: netPanel.networkActionError !== ""
                        width: netList.width
                        text: netPanel.networkActionError
                        color: root.seal
                        wrapMode: Text.Wrap
                        font.family: root.mono; font.pixelSize: 10
                    }
                }
            }


            Rectangle {
                width: parent.width
                height: 28
                visible: root.useNM && netPanel.nmAdapterReady && !netPanel.wifiBlocked
                    && !netPanel.hiddenNetworkOpen && netPanel.nmPasswordSsid === ""
                radius: root.panelButtonRadius
                color: hiddenNetworkMa.containsMouse ? root.fillHover : root.fillIdle
                border.color: hiddenNetworkMa.containsMouse ? root.seal : root.sep
                border.width: 1
                Behavior on color { ColorAnimation { duration: 120 } }

                UiText {
                    anchors.centerIn: parent
                    text: I18n.tr("+ Connect to hidden network")
                    color: hiddenNetworkMa.containsMouse ? root.seal : root.ink
                    font.family: root.mono
                    font.pixelSize: 10
                }

                MouseArea {
                    id: hiddenNetworkMa
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: netPanel.openHiddenNetwork()
                }
            }

            Rectangle {
                width: parent.width
                height: visible ? hiddenForm.implicitHeight + 16 : 0
                visible: root.useNM && netPanel.nmAdapterReady && netPanel.hiddenNetworkOpen
                radius: root.panelButtonRadius
                color: root.fillIdle
                border.color: netPanel.hiddenConnectionError !== "" ? root.sealRaw : root.seal
                border.width: 1
                clip: true

                Column {
                    id: hiddenForm
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.margins: 8
                    spacing: 6

                    UiText {
                        width: parent.width
                        text: netPanel.hiddenConnectionError !== ""
                            ? netPanel.hiddenConnectionError
                            : I18n.tr("Connect to hidden network")
                        color: netPanel.hiddenConnectionError !== "" ? root.sealRaw : root.ink
                        font.family: root.mono
                        font.pixelSize: 10
                        elide: Text.ElideRight
                    }

                    Rectangle {
                        width: parent.width
                        height: 24
                        radius: root.panelButtonRadius
                        color: root.bg
                        border.color: hiddenNameInput.activeFocus ? root.seal : root.sep
                        border.width: 1

                        TextInput {
                            id: hiddenNameInput
                            anchors.fill: parent
                            anchors.leftMargin: 8
                            anchors.rightMargin: 8
                            verticalAlignment: TextInput.AlignVCenter
                            text: netPanel.hiddenName
                            color: root.ink
                            selectionColor: root.seal
                            selectedTextColor: root.paper
                            font.family: root.mono
                            font.pixelSize: 11
                            clip: true
                            enabled: !netPanel.hiddenConnecting
                            onTextChanged: netPanel.hiddenName = text
                            Keys.onPressed: function(event) {
                                if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                                    hiddenSsidInput.forceActiveFocus()
                                    event.accepted = true
                                } else if (event.key === Qt.Key_Escape) {
                                    netPanel.clearHiddenNetwork()
                                    event.accepted = true
                                }
                            }
                        }

                        UiText {
                            anchors.left: parent.left
                            anchors.leftMargin: 8
                            anchors.verticalCenter: parent.verticalCenter
                            visible: hiddenNameInput.text === "" && !hiddenNameInput.activeFocus
                            text: I18n.tr("Network name")
                            color: root.sumi
                            font.family: root.mono
                            font.pixelSize: 10
                        }
                    }

                    Rectangle {
                        width: parent.width
                        height: 24
                        radius: root.panelButtonRadius
                        color: root.bg
                        border.color: hiddenSsidInput.activeFocus ? root.seal : root.sep
                        border.width: 1

                        TextInput {
                            id: hiddenSsidInput
                            anchors.fill: parent
                            anchors.leftMargin: 8
                            anchors.rightMargin: 8
                            verticalAlignment: TextInput.AlignVCenter
                            text: netPanel.hiddenSsid
                            color: root.ink
                            selectionColor: root.seal
                            selectedTextColor: root.paper
                            font.family: root.mono
                            font.pixelSize: 11
                            clip: true
                            enabled: !netPanel.hiddenConnecting
                            onTextChanged: netPanel.hiddenSsid = text
                            Keys.onPressed: function(event) {
                                if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                                    if (netPanel.hiddenSecurity === "open")
                                        netPanel.submitHiddenNetwork()
                                    else
                                        hiddenPasswordInput.forceActiveFocus()
                                    event.accepted = true
                                } else if (event.key === Qt.Key_Escape) {
                                    netPanel.clearHiddenNetwork()
                                    event.accepted = true
                                }
                            }
                        }

                        UiText {
                            anchors.left: parent.left
                            anchors.leftMargin: 8
                            anchors.verticalCenter: parent.verticalCenter
                            visible: hiddenSsidInput.text === "" && !hiddenSsidInput.activeFocus
                            text: I18n.tr("SSID")
                            color: root.sumi
                            font.family: root.mono
                            font.pixelSize: 10
                        }
                    }

                    Rectangle {
                        width: parent.width
                        height: 26
                        radius: root.panelButtonRadius
                        color: hiddenSecurityMa.containsMouse ? root.fillHover : root.bg
                        border.color: hiddenSecurityMa.containsMouse ? root.seal : root.sep
                        border.width: 1
                        Behavior on color { ColorAnimation { duration: 120 } }

                        UiText {
                            anchors.left: parent.left
                            anchors.leftMargin: 8
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("Security")
                            color: root.sumiHi
                            font.family: root.mono
                            font.pixelSize: 10
                        }

                        UiText {
                            anchors.right: parent.right
                            anchors.rightMargin: 8
                            anchors.verticalCenter: parent.verticalCenter
                            text: netPanel.hiddenSecurityLabel() + "  ▾"
                            color: hiddenSecurityMa.containsMouse ? root.seal : root.ink
                            font.family: root.mono
                            font.pixelSize: 10
                        }

                        MouseArea {
                            id: hiddenSecurityMa
                            anchors.fill: parent
                            enabled: !netPanel.hiddenConnecting
                            hoverEnabled: true
                            cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                            onClicked: netPanel.cycleHiddenSecurity()
                        }
                    }

                    Rectangle {
                        width: parent.width
                        height: netPanel.hiddenSecurity === "open" ? 0 : 24
                        visible: netPanel.hiddenSecurity !== "open"
                        radius: root.panelButtonRadius
                        color: root.bg
                        border.color: hiddenPasswordInput.activeFocus ? root.seal : root.sep
                        border.width: 1

                        TextInput {
                            id: hiddenPasswordInput
                            anchors.fill: parent
                            anchors.leftMargin: 8
                            anchors.rightMargin: 8
                            verticalAlignment: TextInput.AlignVCenter
                            text: netPanel.hiddenPassword
                            echoMode: TextInput.Password
                            color: root.ink
                            selectionColor: root.seal
                            selectedTextColor: root.paper
                            font.family: root.mono
                            font.pixelSize: 11
                            clip: true
                            enabled: !netPanel.hiddenConnecting
                            onTextChanged: netPanel.hiddenPassword = text
                            Keys.onPressed: function(event) {
                                if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                                    netPanel.submitHiddenNetwork()
                                    event.accepted = true
                                } else if (event.key === Qt.Key_Escape) {
                                    netPanel.clearHiddenNetwork()
                                    event.accepted = true
                                }
                            }
                        }

                        UiText {
                            anchors.left: parent.left
                            anchors.leftMargin: 8
                            anchors.verticalCenter: parent.verticalCenter
                            visible: hiddenPasswordInput.text === "" && !hiddenPasswordInput.activeFocus
                            text: I18n.tr("Password")
                            color: root.sumi
                            font.family: root.mono
                            font.pixelSize: 10
                        }
                    }

                    Row {
                        width: parent.width
                        height: 22
                        spacing: 6

                        Rectangle {
                            width: (parent.width - 6) / 2
                            height: parent.height
                            radius: root.panelButtonRadius
                            color: hiddenSubmitMa.enabled
                                ? (hiddenSubmitMa.containsMouse ? root.fillPrimaryHover : root.seal)
                                : root.fillIdle
                            border.color: hiddenSubmitMa.enabled ? root.seal : root.sep
                            border.width: 1
                            Behavior on color { ColorAnimation { duration: 120 } }

                            UiText {
                                anchors.centerIn: parent
                                text: netPanel.hiddenConnecting ? I18n.tr("connecting…") : I18n.tr("connect")
                                color: hiddenSubmitMa.enabled ? root.paper : root.sumi
                                font.family: root.mono
                                font.pixelSize: 10
                            }

                            MouseArea {
                                id: hiddenSubmitMa
                                anchors.fill: parent
                                enabled: netPanel.hiddenName.trim() !== ""
                                    && netPanel.hiddenSsid.trim() !== ""
                                    && (netPanel.hiddenSecurity === "open" || netPanel.hiddenPassword !== "")
                                    && !netPanel.hiddenConnecting
                                hoverEnabled: true
                                cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                                onClicked: netPanel.submitHiddenNetwork()
                            }
                        }

                        Rectangle {
                            width: (parent.width - 6) / 2
                            height: parent.height
                            radius: root.panelButtonRadius
                            color: hiddenCancelMa.containsMouse ? root.fillHover : root.fillIdle
                            border.color: hiddenCancelMa.containsMouse ? root.seal : root.sep
                            border.width: 1
                            Behavior on color { ColorAnimation { duration: 120 } }

                            UiText {
                                anchors.centerIn: parent
                                text: I18n.tr("cancel")
                                color: hiddenCancelMa.containsMouse ? root.seal : root.sumi
                                font.family: root.mono
                                font.pixelSize: 10
                            }

                            MouseArea {
                                id: hiddenCancelMa
                                anchors.fill: parent
                                enabled: !netPanel.hiddenConnecting
                                hoverEnabled: true
                                cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                                onClicked: netPanel.clearHiddenNetwork()
                            }
                        }
                    }
                }
            }

            Rectangle {
                width: parent.width
                height: visible ? 92 : 0
                visible: root.useNM && netPanel.nmAdapterReady && netPanel.nmPasswordSsid !== ""
                radius: root.panelButtonRadius
                color: root.fillIdle
                border.color: netPanel.nmConnectionError !== "" ? root.sealRaw : root.seal
                border.width: 1
                clip: true

                Column {
                    anchors.fill: parent
                    anchors.margins: 8
                    spacing: 6

                    UiText {
                        width: parent.width
                        text: netPanel.nmConnectionError !== ""
                            ? netPanel.nmConnectionError
                            : I18n.tr("Password for %1").arg(netPanel.nmPasswordSsid)
                        color: netPanel.nmConnectionError !== "" ? root.sealRaw : root.ink
                        font.family: root.mono
                        font.pixelSize: 10
                        elide: Text.ElideRight
                    }

                    Rectangle {
                        width: parent.width
                        height: 24
                        radius: root.panelButtonRadius
                        color: root.bg
                        border.color: nmPasswordInput.activeFocus ? root.seal : root.sep
                        border.width: 1

                        TextInput {
                            id: nmPasswordInput
                            anchors.fill: parent
                            anchors.leftMargin: 8
                            anchors.rightMargin: 8
                            verticalAlignment: TextInput.AlignVCenter
                            text: netPanel.nmPasswordText
                            echoMode: TextInput.Password
                            color: root.ink
                            selectionColor: root.seal
                            selectedTextColor: root.paper
                            font.family: root.mono
                            font.pixelSize: 11
                            clip: true
                            enabled: !netPanel.nmConnecting
                            onTextChanged: netPanel.nmPasswordText = text
                            Keys.onPressed: function(event) {
                                if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                                    netPanel.submitNmPassword()
                                    event.accepted = true
                                } else if (event.key === Qt.Key_Escape) {
                                    netPanel.clearNmPassword()
                                    event.accepted = true
                                }
                            }
                        }
                    }

                    Row {
                        width: parent.width
                        height: 22
                        spacing: 6

                        Rectangle {
                            width: (parent.width - 6) / 2
                            height: parent.height
                            radius: root.panelButtonRadius
                            color: passwordSubmitMa.enabled
                                ? (passwordSubmitMa.containsMouse ? root.fillPrimaryHover : root.seal)
                                : root.fillIdle
                            border.color: passwordSubmitMa.enabled ? root.seal : root.sep
                            border.width: 1
                            Behavior on color { ColorAnimation { duration: 120 } }
                            UiText {
                                anchors.centerIn: parent
                                text: netPanel.nmConnecting ? I18n.tr("connecting…") : I18n.tr("connect")
                                color: passwordSubmitMa.enabled ? root.paper : root.sumi
                                font.family: root.mono
                                font.pixelSize: 10
                            }
                            MouseArea {
                                id: passwordSubmitMa
                                anchors.fill: parent
                                enabled: netPanel.nmPasswordText !== "" && !netPanel.nmConnecting
                                hoverEnabled: true
                                cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                                onClicked: netPanel.submitNmPassword()
                            }
                        }

                        Rectangle {
                            width: (parent.width - 6) / 2
                            height: parent.height
                            radius: root.panelButtonRadius
                            color: passwordCancelMa.containsMouse ? root.fillHover : root.fillIdle
                            border.color: passwordCancelMa.containsMouse ? root.seal : root.sep
                            border.width: 1
                            Behavior on color { ColorAnimation { duration: 120 } }
                            UiText {
                                anchors.centerIn: parent
                                text: I18n.tr("cancel")
                                color: passwordCancelMa.containsMouse ? root.seal : root.sumi
                                font.family: root.mono
                                font.pixelSize: 10
                            }
                            MouseArea {
                                id: passwordCancelMa
                                anchors.fill: parent
                                hoverEnabled: true
                                cursorShape: Qt.PointingHandCursor
                                onClicked: netPanel.clearNmPassword()
                            }
                        }
                    }
                }
            }

            // NetworkManager (Ryoku 4.0): the iwctl scan/connect don't apply here →
            // show an nmtui shortcut if the Quickshell.Networking adapter is unavailable.
            Rectangle {
                width: parent.width
                height: 52; radius: 6
                visible: root.useNM && netPanel.hasWifi && !netPanel.nmAdapterReady
                color: nmMa.containsMouse ? root.fillHover : root.fillIdle
                border.color: nmMa.containsMouse ? root.seal : root.sep; border.width: 1
                Behavior on color { ColorAnimation { duration: 120 } }
                Column {
                    anchors.centerIn: parent; spacing: 3; width: parent.width - 24
                    UiText {
                        width: parent.width; horizontalAlignment: Text.AlignHCenter
                        text: I18n.tr("Managed by NetworkManager")
                        color: root.ink; font.family: root.mono; font.pixelSize: 11
                    }
                    UiText {
                        width: parent.width; horizontalAlignment: Text.AlignHCenter
                        text: I18n.tr("click to open nmtui")
                        color: root.seal; font.family: root.mono; font.pixelSize: 10
                    }
                }
                MouseArea {
                    id: nmMa
                    anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor
                    onClicked: { root.networkVisible = false; netPanel.openWifiSettings() }
                }
            }

            Rectangle { width: parent.width; height: 1; color: root.sep }

            // ── button ──
            Rectangle {
                width: parent.width
                height: 28; radius: root.panelButtonRadius
                color: netSetMa.containsMouse ? root.fillPrimaryHover : root.seal
                Behavior on color { ColorAnimation { duration: 120 } }
                UiText { anchors.centerIn: parent; text: I18n.tr("Network settings"); color: root.paper; font.family: root.mono; font.pixelSize: 11 }
                MouseArea {
                    id: netSetMa
                    anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor
                    onClicked: { root.networkVisible = false; netPanel.openWifiSettings() }
                }
            }
        }
    }

    Loader {
        id: nmAdapter
        active: root.useNM
        source: "NetworkManagerAdapter.qml"
        onLoaded: {
            item.panel = netPanel
            item.panelOpen = Qt.binding(function() { return root.networkVisible })
            item.refresh()
        }
    }

    Process {
        id: netData
        command: ["bash", "-c",
            "IFACE=$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i==\"dev\"){print $(i+1); exit}}'); " +
            "if [ -z \"$IFACE\" ]; then echo NONE; exit; fi; " +
            "IPADDR=$(ip -o -4 addr show dev \"$IFACE\" 2>/dev/null | awk '{split($4,a,\"/\"); print a[1]; exit}'); " +
            "if [ -d \"/sys/class/net/$IFACE/wireless\" ]; then " +
            "  LINK=$(iw dev \"$IFACE\" link 2>/dev/null); " +
            "  SSID=$(printf '%s\\n' \"$LINK\" | sed -n 's/^\\s*SSID: //p' | head -1); " +
            "  if [[ \"$SSID\" =~ \\\\(x[0-9A-Fa-f]{2}|[0-7]{3}) ]]; then SSID=$(printf '%b' \"$SSID\"); fi; " +
            "  SIG=$(printf '%s\\n' \"$LINK\" | awk '/signal:/ {print int($2); exit}'); " +
            "  FRQ=$(printf '%s\\n' \"$LINK\" | awk '/freq:/ {print $2 \" MHz\"; exit}'); " +
            "  QUAL=$(awk -v s=\"$SIG\" 'BEGIN{q=int((s+110)*100/70);if(q<0)q=0;if(q>100)q=100;print q}'); " +
            "  printf 'WIFI\\t%s\\t%s\\t%s\\t%s\\t%s\\n' \"$SSID\" \"$QUAL\" \"$IFACE\" \"$IPADDR\" \"$FRQ\"; " +
            "else printf 'ETHERNET\\t%s\\t%s\\n' \"$IFACE\" \"$IPADDR\"; fi"
        ]
        running: false
        stdout: StdioCollector {
            onStreamFinished: {
                var parts = this.text.trim().split("\t")
                if (parts[0] === "WIFI") {
                    netPanel.mode = "wifi"; netPanel.ssid = parts[1] || ""
                    netPanel.signal = parseInt(parts[2]) || 0; netPanel.iface = parts[3] || ""
                    netPanel.ipAddr = parts[4] || ""; netPanel.freq = parts[5] || ""
                } else if (parts[0] === "ETHERNET") {
                    netPanel.mode = "ethernet"; netPanel.iface = parts[1] || ""; netPanel.ipAddr = parts[2] || ""
                    netPanel.ssid = ""; netPanel.freq = ""
                } else {
                    netPanel.mode = "none"; netPanel.iface = ""; netPanel.ipAddr = ""; netPanel.ssid = ""
                }
            }
        }
    }

    Process { id: wifiRunner; command: ["bash", "-c", root.launchWifiCmd] }

    // detect wifi device presence
    Process {
        id: devProbe
        command: ["bash", "-c", "for d in /sys/class/net/*/wireless; do [ -e \"$d\" ] || continue; basename \"$(dirname \"$d\")\"; break; done 2>/dev/null"]
        running: false
        stdout: StdioCollector {
            onStreamFinished: {
                var d = this.text.trim()
                netPanel.wdev = d
                netPanel.hasWifi = d !== ""
                if (netPanel.hasWifi) netPanel.scan()
            }
        }
    }

    // scan + list available networks (and known networks)
    Process {
        id: scanProc
        command: ["bash", "-c",
            "DEV=$(for d in /sys/class/net/*/wireless; do [ -e \"$d\" ] || continue; basename \"$(dirname \"$d\")\"; break; done); " +
            "[ -z \"$DEV\" ] && exit; " +
            "iwctl station \"$DEV\" scan >/dev/null 2>&1; sleep 1.5; " +
            "iwctl known-networks list 2>/dev/null | sed 's/\\x1b\\[[0-9;]*m//g; s/\\r//g' | " +
            "  awk '/^[[:space:]]*-+[[:space:]]*$/ {s++; next} s>=2 && NF>0 { sub(/^[[:space:]]+/,\"\"); sub(/[[:space:]][[:space:]]+.*$/,\"\"); if(length) print \"KNOWN\\t\" $0 }'; " +
            "iwctl station \"$DEV\" get-networks 2>/dev/null | sed 's/\\x1b\\[[0-9;]*m//g; s/\\r//g' | " +
            "  awk '" +
            "    /^[[:space:]]*-+[[:space:]]*$/ { seps++; next } " +
            "    seps>=2 && NF>0 { " +
            "      line=$0; conn=0; " +
            "      if (line ~ /^[[:space:]]*>/) conn=1; " +
            "      sub(/^[[:space:]]*>?[[:space:]]*/, \"\", line); " +
            "      if (match(line, /[[:space:]]+(open|psk|8021x|wep)[[:space:]]+\\*+[[:space:]]*$/)) { " +
            "        tail=substr(line, RSTART); ssid=substr(line, 1, RSTART-1); " +
            "        gsub(/[[:space:]]+$/, \"\", ssid); " +
            "        n=split(tail, a, /[[:space:]]+/); sec=\"\"; sig=0; " +
            "        for(i=1;i<=n;i++){ if(a[i] ~ /^(open|psk|8021x|wep)$/) sec=a[i]; if(a[i] ~ /^\\*+$/) sig=length(a[i]) } " +
            "        print \"NET\\t\" conn \"\\t\" ssid \"\\t\" sec \"\\t\" sig " +
            "      } " +
            "    }'"
        ]
        running: false
        stdout: StdioCollector {
            onStreamFinished: {
                var lines = this.text.trim().split("\n")
                var nets = [], kn = []
                for (var i = 0; i < lines.length; i++) {
                    var p = lines[i].split("\t")
                    if (p[0] === "KNOWN" && p[1]) {
                        kn.push(p[1].trim())
                    } else if (p[0] === "NET" && p.length >= 5) {
                        nets.push({ conn: p[1] === "1", ssid: p[2], sec: p[3], sig: parseInt(p[4]) || 0, known: false, visible: true })
                    }
                }
                for (var j = 0; j < nets.length; j++)
                    nets[j].known = kn.indexOf(nets[j].ssid) >= 0
                for (var k = 0; k < kn.length; k++) {
                    var found = false
                    for (var n = 0; n < nets.length; n++) {
                        if (nets[n].ssid === kn[k]) {
                            found = true
                            break
                        }
                    }
                    if (!found)
                        nets.push({ conn: false, ssid: kn[k], sec: "saved", sig: 0, known: true, visible: false })
                }
                // connected first, then by signal
                nets.sort(function(a, b) { return (b.conn - a.conn) || (b.sig - a.sig) })
                netPanel.networks = nets
                netPanel.known = kn
                netPanel.scanning = false
                scanWatchdog.stop()
            }
        }
    }

    Process { id: connectProc; command: ["bash", "-c", "true"] }
    Process {
        id: forgetProc
        command: ["true"]
        running: false
        onExited: rescanTimer.restart()
    }

    Timer { id: rescanTimer; interval: 1500; onTriggered: { netData.running = false; netData.running = true; netPanel.scan() } }
    Timer { id: forgetConfirmTimer; interval: 5000; onTriggered: netPanel.pendingForgetKey = "" }
    // safety: if a scan hangs, don't block future rescans forever
    Timer { id: scanWatchdog; interval: 8000; onTriggered: netPanel.scanning = false }
    Timer {
        id: nmConnectTimeout
        interval: 20000
        onTriggered: {
            if (!netPanel.nmConnecting)
                return

            netPanel.nmConnecting = false
            netPanel.nmConnectionError = I18n.tr("Connection timed out")
            Qt.callLater(function() {
                if (nmPasswordInput.visible)
                    nmPasswordInput.forceActiveFocus()
            })
        }
    }

    // ── wifi radio (rfkill) ──
    Process {
        id: rfkillState
        command: ["bash", "-c", "rfkill list wifi 2>/dev/null | grep -qi 'Soft blocked: yes' && echo BLOCKED || echo OK"]
        running: false
        stdout: StdioCollector {
            onStreamFinished: { netPanel.wifiBlocked = this.text.trim() === "BLOCKED" }
        }
    }
    Process { id: rfkillToggle; command: ["bash", "-c", "true"] }

    // negotiated link speed: ethernet from /sys, wifi from iw bitrate
    Process {
        id: speedProc
        command: ["bash", "-c",
            "IFACE=$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i==\"dev\"){print $(i+1); exit}}'); " +
            "[ -z \"$IFACE\" ] && exit; " +
            "if [ -d /sys/class/net/$IFACE/wireless ]; then " +
            "  R=$(iw dev \"$IFACE\" link 2>/dev/null | sed -n 's/.*tx bitrate: //p' | awk '{print $1\" \"$2; exit}'); " +
            "  [ -n \"$R\" ] && echo \"W:$R\"; " +
            "else " +
            "  S=$(cat /sys/class/net/$IFACE/speed 2>/dev/null); " +
            "  [ -n \"$S\" ] && echo \"E:$S\"; " +
            "fi"]
        running: false
        stdout: StdioCollector {
            onStreamFinished: {
                var t = this.text.trim()
                if (t.indexOf("E:") === 0) {
                    var mb = parseInt(t.slice(2)) || 0
                    netPanel.linkSpeed = mb >= 1000 ? (mb / 1000).toFixed(1).replace(/\.0$/, "") + " Gbit/s"
                                       : (mb > 0 ? mb + " Mbit/s" : "")
                } else if (t.indexOf("W:") === 0) {
                    netPanel.linkSpeed = t.slice(2)   // already e.g. "866.7 MBit/s"
                } else {
                    netPanel.linkSpeed = ""
                }
            }
        }
    }

    onVisibleChanged: {
        if (visible) {
            if (!root.useNM) {
                rfkillState.running = false; rfkillState.running = true
            } else if (nmAdapterReady) {
                nmAdapter.item.refresh()
            }
            netData.running = false; netData.running = true
            devProbe.running = false; devProbe.running = true
            speedProc.running = false; speedProc.running = true
        } else {
            if (speedTest.running)
                speedTest.cancel()
            speedTestAttempted = false
            clearNmPassword()
            resetNetworkSelection()
        }
    }
}

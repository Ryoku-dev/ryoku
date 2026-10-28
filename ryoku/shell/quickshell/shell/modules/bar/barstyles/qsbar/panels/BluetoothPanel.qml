import QtQuick
import "../modules"
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import Quickshell.Bluetooth
import "../IconMap.js" as IconMap
import Ryoku.Ui.Singletons
import shell.services

PanelWindow {
    id: btPanel
    required property var root

    screen: root.activePopupScreen

    color: "transparent"
    anchors { top: true; bottom: true; left: true; right: true }
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "ryoku-bluetooth"

    readonly property int barBottom: root.v2BarHeight
    readonly property int gap: 6

    readonly property var adapter: Bluetooth.defaultAdapter
    readonly property bool btOn: adapter !== null && adapter.enabled
    readonly property bool scanning: btOn && adapter.discovering

    readonly property var allDevices: (btOn && Bluetooth.devices) ? Bluetooth.devices.values : []
    readonly property var devices: {
        var out = []
        for (var i = 0; i < allDevices.length; i++) {
            var d = allDevices[i]
            if (!d) continue
            var known = d.connected || d.paired || d.bonded
            var named = (d.name && d.name.length > 0)
                || (d.deviceName && d.deviceName.length > 0)
            if (known || named)
                out.push(d)
        }
        out.sort(function(a, b) {
            var ra = a.connected ? 0 : (a.paired || a.bonded) ? 1 : 2
            var rb = b.connected ? 0 : (b.paired || b.bonded) ? 1 : 2
            return ra - rb
        })
        return out
    }
    readonly property var shownDevices: devices.slice(0, 8)
    readonly property int numConnected: {
        var n = 0
        for (var i = 0; i < devices.length; i++) if (devices[i].connected) n++
        return n
    }

    property string pairingAddr: ""
    property string pairError: ""
    readonly property bool busy: pairProc.running

    readonly property color deviceActionFill: Qt.rgba(
        root.paper.r * 0.88,
        root.paper.g * 0.88,
        root.paper.b * 0.88,
        1.0)

    function activateDevice(device) {
        if (!device || btPanel.busy) return
        if (device.connected) {
            device.disconnect()
            return
        }
        if (device.blocked) device.blocked = false
        // Paired and unpaired take the same path: Device1.Connect on an
        // existing bond fails at once when BlueZ holds keys the device has
        // forgotten, and nothing here cleared that. BtLink.linkCommand pairs
        // if needed, retries, and rebuilds a bond that will not connect.
        btPanel.link(device)
    }

    function link(device) {
        if (!device || pairProc.running) return
        var mac = String(device.address || "")
        if (!/^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$/.test(mac)) return
        btPanel.pairError = ""
        btPanel.pairingAddr = mac
        pairProc.command = BtLink.linkCommand(mac)
        pairProc.running = false
        pairProc.running = true
    }

    function forgetDevice(device) {
        if (!device || btPanel.busy) return
        device.forget()
    }

    function toggleScan() {
        if (!btOn) return
        BluetoothDiscovery.setDiscovering(btPanel, adapter, !scanning)
    }
    function stopScan() {
        BluetoothDiscovery.setDiscovering(btPanel, adapter, false)
        scanStop.stop()
    }

    // BlueZ phone charge is coarse/stale without provenance; suppress it (mirrors Shibumi).
    function batteryText(dev) {
        if (!dev || !dev.batteryAvailable) return ""
        var ic = String(dev.icon || "").toLowerCase()
        if (ic === "phone" || ic === "smartphone") return ""
        var b = Number(dev.battery)
        if (!isFinite(b) || b < 0 || b > 1) return ""
        return Math.round(b * 100) + "%"
    }
    function typeLabel(dev) {
        var ic = String(dev && dev.icon ? dev.icon : "").toLowerCase()
        if (ic === "input-gaming") return I18n.tr("Controller")
        if (ic === "audio-headphones") return I18n.tr("Headphones")
        if (ic === "audio-headset") return I18n.tr("Headset")
        if (ic === "audio-card") return I18n.tr("Speaker")
        if (ic === "input-mouse") return I18n.tr("Mouse")
        if (ic === "input-keyboard") return I18n.tr("Keyboard")
        if (ic === "phone") return I18n.tr("Phone")
        return ic ? ic : I18n.tr("Device")
    }

    property real reveal: root.bluetoothVisible ? 1 : 0
    Behavior on reveal {
        NumberAnimation {
            duration: root.bluetoothVisible ? 160 : 120
            easing.type: root.bluetoothVisible ? Easing.OutCubic : Easing.InCubic
        }
    }
    visible: reveal > 0.001
    WlrLayershell.keyboardFocus: root.bluetoothVisible ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    MouseArea { anchors.fill: parent; onClicked: root.bluetoothVisible = false }

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
            root: btPanel.root
            ownerActive: btPanel.root.bluetoothVisible
            targetX: btPanel.root.bluetoothBarX
            reveal: btPanel.reveal
        }

        x: Math.round(Math.max(6, Math.min(root.bluetoothBarX - width / 2, parent.width - width - 6)))
        y: root.barPosition === "bottom"
            ? (parent.height - barBottom - gap - height) + 2 * (1 - btPanel.reveal)
            : (barBottom + gap) - 2 * (1 - btPanel.reveal)
        opacity: btPanel.reveal
        focus: root.bluetoothVisible

        Keys.onPressed: function(event) {
            if (event.key === Qt.Key_Escape) { root.bluetoothVisible = false; event.accepted = true }
        }

        MouseArea { anchors.fill: parent; onClicked: {} }

        Column {
            id: col
            anchors.fill: parent
            anchors.margins: 12
            spacing: 8

            // ── header + power toggle ──
            Item {
                width: parent.width
                height: 24
                Row {
                    anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                    spacing: 8
                    UiText {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Bluetooth")
                        color: root.ink; font.family: root.mono; font.pixelSize: 13
                        font.letterSpacing: 2; font.weight: Font.Medium
                    }
                    Row {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: btPanel.btOn && btPanel.numConnected > 0
                        spacing: 3
                        IconText {
                            anchors.verticalCenter: parent.verticalCenter
                            text: IconMap.icon("bluetooth_connected")
                            color: root.seal
                            font.pixelSize: 13
                        }
                        UiText {
                            anchors.verticalCenter: parent.verticalCenter
                            text: String(btPanel.numConnected)
                            color: root.seal
                            font.family: root.mono; font.pixelSize: 11
                        }
                    }
                }
                Row {
                    anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                    spacing: 10
                    // power toggle pill
                    Rectangle {
                        anchors.verticalCenter: parent.verticalCenter
                        width: 46; height: 20; radius: 10
                        color: btPanel.btOn ? root.fillActive
                                            : root.fillIdle
                        border.color: btPanel.btOn ? root.seal : root.sep
                        border.width: 1
                        Behavior on color { ColorAnimation { duration: 150 } }
                        Rectangle {
                            width: 14; height: 14; radius: 7
                            anchors.verticalCenter: parent.verticalCenter
                            x: btPanel.btOn ? parent.width - width - 3 : 3
                            color: btPanel.btOn ? root.seal : root.sumi
                            Behavior on x { NumberAnimation { duration: 150; easing.type: Easing.OutCubic } }
                        }
                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            enabled: BluetoothPower.available
                            onClicked: BluetoothPower.toggle()
                        }
                    }
                    UiText {
                        anchors.verticalCenter: parent.verticalCenter
                        text: "✕"; color: closeMa.containsMouse ? root.seal : root.sumi; font.pixelSize: 12
                        Behavior on color { ColorAnimation { duration: 120 } }
                        MouseArea { id: closeMa; anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor; onClicked: root.bluetoothVisible = false }
                    }
                }
            }

            Rectangle { width: parent.width; height: 1; color: root.sep }

            // ── off state ──
            UiText {
                visible: !btPanel.btOn
                width: parent.width; horizontalAlignment: Text.AlignHCenter
                text: BluetoothPower.blocked ? I18n.tr("Bluetooth is blocked") : I18n.tr("Bluetooth is off")
                color: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.35)
                font.family: root.mono; font.pixelSize: 11
                topPadding: 4; bottomPadding: 4
            }

            // ── scan control (only when on) ──
            Rectangle {
                visible: btPanel.btOn
                width: parent.width
                height: 28; radius: root.panelButtonRadius
                readonly property bool hovered: scanMa.containsMouse
                color: btPanel.scanning ? root.fillActive
                       : hovered ? root.fillHover : root.fillIdle
                border.color: (btPanel.scanning || hovered) ? root.seal : root.sep
                border.width: 1
                Behavior on color { ColorAnimation { duration: 120 } }
                UiText {
                    anchors.centerIn: parent
                    text: btPanel.scanning ? I18n.tr("Scanning…") : I18n.tr("Scan for devices")
                    color: btPanel.scanning ? root.seal : root.ink
                    font.family: root.mono; font.pixelSize: 11
                }
                MouseArea {
                    id: scanMa
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: btPanel.toggleScan()
                }
            }

            UiText {
                visible: btPanel.pairError.length > 0
                width: parent.width; horizontalAlignment: Text.AlignHCenter
                text: btPanel.pairError
                color: root.danger
                wrapMode: Text.WordWrap
                font.family: root.mono; font.pixelSize: 10
                topPadding: 2; bottomPadding: 2
            }

            // ── device list ──
            Column {
                width: parent.width
                spacing: 4
                visible: btPanel.btOn
                Repeater {
                    model: btPanel.shownDevices
                    delegate: Rectangle {
                        id: devTile
                        required property var modelData
                        readonly property var nativeDev: modelData
                        readonly property string devMac: String(modelData.address || "")
                        readonly property bool devPaired: modelData.paired || modelData.bonded
                        property bool expanded: false
                        readonly property string batteryText: btPanel.batteryText(nativeDev)
                        readonly property bool canExpand: modelData.connected || devPaired
                        readonly property bool hovered: tileHover.containsMouse || actionMa.containsMouse || infoMa.containsMouse
                        readonly property int rowHeight: 42
                        width: col.width
                        height: rowHeight + (expanded && canExpand ? detailCol.implicitHeight + 10 : 0)
                        radius: root.panelButtonRadius
                        clip: true
                        color: modelData.connected ? root.fillActive
                               : hovered ? root.fillHover : root.fillIdle
                        border.color: modelData.connected ? root.seal
                                      : hovered ? root.seal : root.sep
                        border.width: 1
                        Behavior on color { ColorAnimation { duration: 120 } }
                        Behavior on height { NumberAnimation { duration: 150; easing.type: Easing.OutCubic } }

                        MouseArea {
                            id: tileHover
                            anchors.fill: parent
                            acceptedButtons: Qt.NoButton
                            hoverEnabled: true
                        }

                        Item {
                            id: rowItem
                            anchors.left: parent.left; anchors.right: parent.right; anchors.top: parent.top
                            height: devTile.rowHeight

                            Column {
                                anchors.left: parent.left; anchors.leftMargin: 8
                                anchors.right: devTile.canExpand ? infoPill.left : actionButton.left
                                anchors.rightMargin: 8
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: 1
                                UiText {
                                    width: parent.width
                                    text: BtLink.label(devTile.modelData)
                                    color: root.ink; font.family: root.mono; font.pixelSize: 11
                                    elide: Text.ElideRight
                                }
                                UiText {
                                    width: parent.width
                                    text: {
                                        if (btPanel.pairingAddr === devTile.devMac) return I18n.tr("Pairing…")
                                        if (devTile.modelData.state === BluetoothDeviceState.Connecting) return I18n.tr("Connecting…")
                                        if (devTile.modelData.connected)
                                            return devTile.batteryText !== "" ? I18n.tr("Connected · %1").arg(devTile.batteryText) : I18n.tr("Connected")
                                        return devTile.devPaired ? I18n.tr("Paired") : I18n.tr("Available")
                                    }
                                    color: root.ink
                                    font.family: root.mono; font.pixelSize: 10; font.weight: Font.Medium
                                    elide: Text.ElideRight
                                }
                            }

                            // info toggle: connected rows only, reveals the inline detail area
                            Rectangle {
                                id: infoPill
                                visible: devTile.canExpand
                                anchors.right: actionButton.left; anchors.rightMargin: 6
                                anchors.verticalCenter: parent.verticalCenter
                                width: 18; height: 18; radius: 9
                                color: devTile.expanded ? root.fillActive
                                       : infoMa.containsMouse ? root.fillHover : root.fillIdle
                                border.color: (devTile.expanded || infoMa.containsMouse) ? root.seal : root.sep
                                border.width: 1
                                Behavior on color { ColorAnimation { duration: 120 } }
                                UiText {
                                    anchors.centerIn: parent
                                    text: "!"
                                    color: (devTile.expanded || infoMa.containsMouse) ? root.seal : root.sumi
                                    font.family: root.mono; font.pixelSize: 11; font.weight: Font.Medium
                                }
                                MouseArea {
                                    id: infoMa
                                    anchors.fill: parent
                                    hoverEnabled: true
                                    cursorShape: Qt.PointingHandCursor
                                    onClicked: devTile.expanded = !devTile.expanded
                                }
                            }

                            Rectangle {
                                id: actionButton
                                anchors.right: parent.right; anchors.rightMargin: 8
                                anchors.verticalCenter: parent.verticalCenter
                                width: actionLabel.implicitWidth + 14
                                height: 24; radius: root.panelButtonRadius
                                color: btPanel.deviceActionFill
                                border.color: root.sep
                                border.width: 1
                                opacity: btPanel.busy ? 0.45 : 1
                                UiText {
                                    id: actionLabel
                                    anchors.centerIn: parent
                                    text: devTile.modelData.connected ? I18n.tr("Disconnect") : I18n.tr("Connect")
                                    color: actionMa.containsMouse ? root.seal : root.ink
                                    font.family: root.mono; font.pixelSize: 10
                                }
                                MouseArea {
                                    id: actionMa
                                    anchors.fill: parent
                                    enabled: !btPanel.busy
                                    hoverEnabled: true
                                    cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                                    onClicked: btPanel.activateDevice(devTile.modelData)
                                }
                            }
                        }

                        // inline detail: battery / type / address, gated by the info toggle
                        Column {
                            id: detailCol
                            anchors.top: rowItem.bottom; anchors.topMargin: 2
                            anchors.left: parent.left; anchors.leftMargin: 8
                            anchors.right: parent.right; anchors.rightMargin: 8
                            spacing: 2
                            visible: devTile.expanded && devTile.canExpand
                            opacity: visible ? 1 : 0
                            Behavior on opacity { NumberAnimation { duration: 120 } }

                            Item {
                                width: parent.width; height: 14
                                visible: devTile.batteryText !== ""
                                UiText { anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter; text: I18n.tr("Battery"); color: root.sumiHi; font.family: root.mono; font.pixelSize: 10; font.letterSpacing: 1 }
                                UiText { anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter; text: devTile.batteryText; color: root.ink; font.family: root.mono; font.pixelSize: 10 }
                            }
                            Item {
                                width: parent.width; height: 14
                                UiText { anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter; text: I18n.tr("Type"); color: root.sumiHi; font.family: root.mono; font.pixelSize: 10; font.letterSpacing: 1 }
                                UiText { anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter; text: btPanel.typeLabel(devTile.nativeDev); color: root.ink; font.family: root.mono; font.pixelSize: 10; elide: Text.ElideRight }
                            }
                            Item {
                                width: parent.width; height: 14
                                UiText { anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter; text: I18n.tr("Address"); color: root.sumiHi; font.family: root.mono; font.pixelSize: 10; font.letterSpacing: 1 }
                                UiText { anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter; text: devTile.devMac; color: root.ink; font.family: root.mono; font.pixelSize: 10 }
                            }
                            Item {
                                width: parent.width; height: 24
                                visible: devTile.devPaired
                                Rectangle {
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: forgetLabel.implicitWidth + 16
                                    height: 20; radius: root.panelButtonRadius
                                    color: forgetMa.containsMouse ? root.fillPrimaryHover : root.fillIdle
                                    border.color: forgetMa.containsMouse ? root.seal : root.sep
                                    border.width: 1
                                    opacity: btPanel.busy ? 0.45 : 1
                                    UiText { id: forgetLabel; anchors.centerIn: parent; text: I18n.tr("Forget"); color: forgetMa.containsMouse ? root.seal : root.sumiHi; font.family: root.mono; font.pixelSize: 10 }
                                    MouseArea { id: forgetMa; anchors.fill: parent; enabled: !btPanel.busy; hoverEnabled: true; cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor; onClicked: btPanel.forgetDevice(devTile.modelData) }
                                }
                            }
                        }
                    }
                }
                UiText {
                    visible: btPanel.btOn && btPanel.devices.length === 0
                    width: parent.width; horizontalAlignment: Text.AlignHCenter
                    text: btPanel.scanning ? I18n.tr("Searching…") : I18n.tr("No devices, tap Scan")
                    color: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.3)
                    font.family: root.mono; font.pixelSize: 11
                    topPadding: 2; bottomPadding: 2
                }
            }

            Rectangle { width: parent.width; height: 1; color: root.sep }

            Rectangle {
                width: parent.width
                height: 28; radius: root.panelButtonRadius
                color: btSetMa.containsMouse ? root.fillPrimaryHover : root.seal
                Behavior on color { ColorAnimation { duration: 120 } }
                UiText { anchors.centerIn: parent; text: I18n.tr("Bluetooth settings"); color: root.paper; font.family: root.mono; font.pixelSize: 11 }
                MouseArea {
                    id: btSetMa
                    anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor
                    onClicked: { root.bluetoothVisible = false; btRunner.running = false; btRunner.running = true }
                }
            }
        }
    }

    Process {
        id: pairProc
        running: false
        property string collected: ""
        // stderr as well as stdout: a failure in the script itself only lands
        // there, and losing it leaves the user the generic message with no way
        // to tell what went wrong.
        stdout: StdioCollector { onStreamFinished: pairProc.collected += this.text }
        stderr: StdioCollector { onStreamFinished: pairProc.collected += this.text }
        onExited: function(code, status) {
            if (code !== 0) {
                var lines = pairProc.collected.trim().split("\n")
                var msg = lines.length ? lines[lines.length - 1].trim() : ""
                btPanel.pairError = msg.length ? msg
                    : I18n.tr("Could not connect. Put the device in pairing mode and try again.")
            } else {
                btPanel.pairError = ""
            }
            pairProc.collected = ""
            btPanel.pairingAddr = ""
        }
    }

    Timer { id: scanStop; interval: 30000; onTriggered: btPanel.stopScan() }
    onScanningChanged: { if (scanning) scanStop.restart(); else scanStop.stop() }

    Process { id: btRunner; command: ["bash", "-c", root.launchBtCmd] }

    onVisibleChanged: { if (!visible) { btPanel.stopScan(); btPanel.pairError = "" } }
    Component.onDestruction: btPanel.stopScan()
}

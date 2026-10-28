pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Bluetooth
import Quickshell.Io
import shell.services
import Ryoku.Ui.Singletons
import ".."

Item {
    id: root

    required property real s
    required property bool active
    signal backRequested()

    readonly property var adapter: Bluetooth.defaultAdapter
    readonly property bool adapterAvailable: BluetoothPower.available
    readonly property bool adapterEnabled: root.adapter !== null && root.adapter.enabled
    readonly property bool scanning: root.adapterEnabled && root.adapter.discovering
    property var discoveryAdapter: null
    property string busyAddress: ""
    property string errorText: ""
    readonly property bool busy: linkProcess.running
    readonly property bool motionAllowed: root.active && !Tokens.reduceMotion && !Motion.reduce

    readonly property var devices: !root.adapterEnabled || !Bluetooth.devices
        ? [] : (Bluetooth.devices.values || []).filter(device => !!device)
    readonly property var connectedDevices: root.devices.filter(device => device && device.connected)
    readonly property var pairedDevices: root.devices.filter(device => device && !device.connected && (device.paired || device.bonded))
    readonly property var discoveredDevices: root.devices.filter(device => device && !device.connected && !device.paired && !device.bonded)

    implicitHeight: content.implicitHeight
    height: implicitHeight

    function syncDiscovery(): void {
        const wanted = root.active && root.adapterEnabled;
        if (root.discoveryAdapter && (!wanted || root.discoveryAdapter !== root.adapter)) {
            BluetoothDiscovery.setDiscovering(root, root.discoveryAdapter, false);
            root.discoveryAdapter = null;
        }
        if (wanted && root.adapter && root.discoveryAdapter !== root.adapter) {
            root.discoveryAdapter = root.adapter;
            BluetoothDiscovery.setDiscovering(root, root.adapter, true);
        } else if (wanted && root.adapter) {
            BluetoothDiscovery.setDiscovering(root, root.adapter, true);
        }
    }

    function stopDiscovery(): void {
        if (root.discoveryAdapter)
            BluetoothDiscovery.setDiscovering(root, root.discoveryAdapter, false);
        root.discoveryAdapter = null;
    }

    function link(device): void {
        if (!device || root.busy)
            return;
        const address = String(device.address || "");
        if (!/^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$/.test(address)) {
            root.errorText = I18n.tr("This device did not provide a usable Bluetooth address.");
            return;
        }
        root.errorText = "";
        root.busyAddress = address;
        linkProcess.command = BtLink.linkCommand(address);
        linkProcess.running = false;
        linkProcess.running = true;
    }

    function primaryAction(device): void {
        if (!device || root.busy)
            return;
        if (device.connected)
            device.disconnect();
        else
            root.link(device);
    }

    function deviceStatus(device): string {
        if (!device)
            return "";
        const battery = BtLink.batteryLevel(device);
        let status = device.connected ? I18n.tr("Connected")
            : (device.paired || device.bonded) ? I18n.tr("Paired")
            : BtLink.typeLabel(device);
        if (device.connected && battery >= 0)
            status += " · " + I18n.tr("%1% battery").arg(battery);
        if (device.trusted)
            status += " · " + I18n.tr("Trusted");
        return status;
    }

    onActiveChanged: root.syncDiscovery()
    onAdapterChanged: root.syncDiscovery()
    onAdapterEnabledChanged: root.syncDiscovery()
    Component.onCompleted: root.syncDiscovery()
    Component.onDestruction: root.stopDiscovery()

    Process {
        id: linkProcess
        command: []
        stdout: StdioCollector { id: linkOutput }
        stderr: StdioCollector { id: linkError }
        onExited: code => {
            root.busyAddress = "";
            if (code === 0) {
                root.errorText = "";
                return;
            }
            const combined = String(linkOutput.text || "") + "\n" + String(linkError.text || "");
            const lines = combined.trim().split("\n").filter(line => line.trim().length > 0);
            root.errorText = lines.length > 0 ? lines[lines.length - 1].trim()
                : I18n.tr("Could not connect to this device.");
        }
    }

    component SectionTitle: Item {
        id: sectionTitle
        required property string title
        property string status: ""
        width: parent ? parent.width : implicitWidth
        implicitHeight: Math.max(titleText.implicitHeight, statusText.implicitHeight)
        height: implicitHeight
        Text {
            id: titleText
            anchors.left: parent.left
            anchors.right: statusText.visible ? statusText.left : parent.right
            anchors.rightMargin: statusText.visible ? Tokens.s3 * root.s : 0
            text: sectionTitle.title
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fRow * root.s
            font.weight: Font.DemiBold
            wrapMode: Text.Wrap
        }
        Text {
            id: statusText
            visible: sectionTitle.status !== ""
            anchors.right: parent.right
            anchors.verticalCenter: titleText.verticalCenter
            text: sectionTitle.status
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall * root.s
            font.weight: Font.Medium
        }
    }

    component EmptyState: Text {
        width: parent ? parent.width : implicitWidth
        color: Tokens.inkMuted
        font.family: Tokens.ui
        font.pixelSize: Tokens.fSmall * root.s
        wrapMode: Text.WrapAtWordBoundaryOrAnywhere
        horizontalAlignment: Text.AlignHCenter
        topPadding: Tokens.s3 * root.s
        bottomPadding: Tokens.s3 * root.s
    }

    component DeviceRow: Rectangle {
        id: deviceRow
        required property var device
        width: parent ? parent.width : implicitWidth
        implicitHeight: Math.max(72 * root.s, deviceCopy.implicitHeight + Tokens.s3 * root.s * 2)
        height: implicitHeight
        radius: Tokens.radius * root.s * 1.5
        color: deviceHover.hovered ? Tokens.tint10
            : deviceRow.device && deviceRow.device.connected ? Tokens.tint10 : Tokens.tint5
        border.width: Tokens.border
        border.color: deviceRow.connecting ? Tokens.bone : deviceHover.hovered ? Tokens.lineStrong : Tokens.lineSoft
        Behavior on color {
            enabled: root.motionAllowed
            ColorAnimation { duration: Tokens.snap }
        }
        Behavior on border.color {
            enabled: root.motionAllowed
            ColorAnimation { duration: Tokens.snap }
        }
        readonly property bool connecting: root.busy
            && root.busyAddress === String(deviceRow.device ? deviceRow.device.address : "")

        Rectangle {
            id: deviceIcon
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s3 * root.s
            anchors.verticalCenter: parent.verticalCenter
            width: 42 * root.s
            height: width
            radius: width / 2
            color: deviceRow.device && deviceRow.device.connected ? Tokens.bone : Tokens.tint10
            Text {
                anchors.centerIn: parent
                text: BtLink.glyphFor(deviceRow.device)
                color: deviceRow.device && deviceRow.device.connected ? Tokens.inkOnBone : Tokens.inkDim
                font.family: "Material Symbols Rounded"
                font.pixelSize: 21 * root.s
            }
        }

        Column {
            id: deviceCopy
            anchors.left: deviceIcon.right
            anchors.right: deviceActions.left
            anchors.leftMargin: Tokens.s3 * root.s
            anchors.rightMargin: Tokens.s3 * root.s
            anchors.verticalCenter: parent.verticalCenter
            spacing: 2 * root.s
            Text {
                width: parent.width
                text: BtLink.label(deviceRow.device)
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fRow * root.s
                font.weight: Font.DemiBold
                elide: Text.ElideRight
            }
            Text {
                width: parent.width
                text: root.deviceStatus(deviceRow.device)
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall * root.s
                elide: Text.ElideRight
            }
        }

        Row {
            id: deviceActions
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s3 * root.s
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2 * root.s

            SidebarButton {
                s: root.s
                compact: true
                visible: !!deviceRow.device && (deviceRow.device.paired || deviceRow.device.bonded)
                glyph: deviceRow.device && deviceRow.device.trusted ? "verified" : "verified_off"
                text: ""
                Accessible.name: deviceRow.device && deviceRow.device.trusted ? I18n.tr("Untrust") : I18n.tr("Trust")
                armed: !root.busy
                onAct: if (deviceRow.device) deviceRow.device.trusted = !deviceRow.device.trusted
            }
            SidebarButton {
                s: root.s
                compact: true
                visible: !!deviceRow.device && (deviceRow.device.paired || deviceRow.device.bonded) && !deviceRow.device.connected
                glyph: "delete"
                text: ""
                Accessible.name: I18n.tr("Forget")
                armed: !root.busy
                onAct: if (deviceRow.device) deviceRow.device.forget()
            }
            SidebarButton {
                s: root.s
                compact: true
                text: deviceRow.connecting ? I18n.tr("Connecting…")
                    : deviceRow.device && deviceRow.device.connected ? I18n.tr("Disconnect")
                    : deviceRow.device && (deviceRow.device.paired || deviceRow.device.bonded) ? I18n.tr("Connect")
                    : I18n.tr("Pair")
                primary: !!deviceRow.device && deviceRow.device.connected
                armed: !root.busy
                onAct: root.primaryAction(deviceRow.device)
            }
        }

        HoverHandler { id: deviceHover }
    }

    Column {
        id: content
        width: root.width
        spacing: Tokens.s4 * root.s

        Item {
            width: parent.width
            implicitHeight: Math.max(48 * root.s, pageTitle.implicitHeight)
            height: implicitHeight

            SidebarButton {
                id: backButton
                s: root.s
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                glyph: "arrow_back"
                text: I18n.tr("Overview")
                onAct: root.backRequested()
            }
            Text {
                id: pageTitle
                anchors.left: backButton.right
                anchors.right: radioToggle.left
                anchors.leftMargin: Tokens.s4 * root.s
                anchors.rightMargin: Tokens.s3 * root.s
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Bluetooth devices")
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Tokens.fTitle * root.s
                font.weight: Font.DemiBold
                elide: Text.ElideRight
            }
            SidebarToggle {
                id: radioToggle
                s: root.s
                compact: true
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                width: implicitWidth
                text: ""
                on: root.adapterEnabled
                enabled: root.adapterAvailable
                Accessible.name: I18n.tr("Bluetooth radio")
                onToggleRequested: BluetoothPower.toggle()
            }
        }

        Rectangle {
            visible: root.scanning
            width: parent.width
            height: Tokens.s1 * root.s
            radius: height / 2
            color: Tokens.bone
            opacity: root.motionAllowed ? 0.24 : 1

            SequentialAnimation on opacity {
                running: root.active && root.scanning && root.motionAllowed
                loops: Animation.Infinite
                NumberAnimation { from: 0.24; to: 0.86; duration: Tokens.move; easing.type: Tokens.ease }
                NumberAnimation { from: 0.86; to: 0.24; duration: Tokens.move; easing.type: Tokens.ease }
            }
        }

        Rectangle {
            width: parent.width
            implicitHeight: statusBody.implicitHeight + Tokens.s4 * root.s * 2
            height: implicitHeight
            radius: Tokens.radius * root.s * 2
            color: Tokens.paperLift
            border.width: Tokens.border
            border.color: Tokens.lineSoft

            Row {
                id: statusBody
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.margins: Tokens.s4 * root.s
                spacing: Tokens.s4 * root.s

                Rectangle {
                    width: 52 * root.s
                    height: width
                    radius: width / 2
                    color: root.connectedDevices.length > 0 ? Tokens.bone : Tokens.tint10
                    Text {
                        anchors.centerIn: parent
                        text: root.adapterEnabled ? "bluetooth" : "bluetooth_disabled"
                        color: root.connectedDevices.length > 0 ? Tokens.inkOnBone : Tokens.inkDim
                        font.family: "Material Symbols Rounded"
                        font.pixelSize: 25 * root.s
                    }
                }
                Column {
                    width: parent.width - 52 * root.s - parent.spacing
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s1 * root.s
                    Text {
                        width: parent.width
                        text: BluetoothPower.blocked ? I18n.tr("Bluetooth is blocked")
                            : !root.adapterAvailable ? I18n.tr("No Bluetooth hardware")
                            : !root.adapterEnabled ? I18n.tr("Bluetooth is off")
                            : root.connectedDevices.length === 1 ? BtLink.label(root.connectedDevices[0])
                            : root.connectedDevices.length > 1 ? I18n.tr("%1 devices connected").arg(root.connectedDevices.length)
                            : I18n.tr("Ready to pair")
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fValue * root.s
                        font.weight: Font.DemiBold
                        elide: Text.ElideRight
                    }
                    Text {
                        width: parent.width
                        text: !root.adapterAvailable ? I18n.tr("This machine has no available Bluetooth adapter.")
                            : !root.adapterEnabled ? I18n.tr("Turn on the radio to discover nearby devices.")
                            : root.scanning ? I18n.tr("Discovering nearby devices. Put the device in pairing mode.")
                            : I18n.tr("Discovery pauses when you leave this page.")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                        wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                    }
                }
            }
        }

        Text {
            visible: root.errorText !== ""
            width: parent.width
            text: root.errorText
            color: Tokens.alert
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall * root.s
            font.weight: Font.Medium
            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
        }

        Column {
            visible: root.adapterEnabled
            width: parent.width
            spacing: Tokens.s3 * root.s

            SectionTitle {
                title: I18n.tr("Connected")
                status: root.connectedDevices.length > 0 ? String(root.connectedDevices.length) : ""
            }
            Repeater {
                model: root.connectedDevices
                delegate: DeviceRow { required property var modelData; device: modelData }
            }
            EmptyState {
                visible: root.connectedDevices.length === 0
                text: I18n.tr("No Bluetooth devices are connected.")
            }
        }

        Column {
            visible: root.adapterEnabled
            width: parent.width
            spacing: Tokens.s3 * root.s

            SectionTitle {
                title: I18n.tr("Paired devices")
                status: root.pairedDevices.length > 0 ? String(root.pairedDevices.length) : ""
            }
            Repeater {
                model: root.pairedDevices
                delegate: DeviceRow { required property var modelData; device: modelData }
            }
            EmptyState {
                visible: root.pairedDevices.length === 0
                text: I18n.tr("Paired devices will stay here when they are disconnected.")
            }
        }

        Column {
            visible: root.adapterEnabled
            width: parent.width
            spacing: Tokens.s3 * root.s

            SectionTitle {
                title: I18n.tr("Nearby devices")
                status: root.scanning ? I18n.tr("Scanning…") : ""
            }
            Repeater {
                model: root.discoveredDevices
                delegate: DeviceRow { required property var modelData; device: modelData }
            }
            EmptyState {
                visible: root.discoveredDevices.length === 0
                text: root.scanning ? I18n.tr("Looking for devices… Keep the device nearby and in pairing mode.")
                    : I18n.tr("No nearby devices found.")
            }
            SidebarButton {
                s: root.s
                visible: root.adapterEnabled && !root.scanning
                anchors.horizontalCenter: parent.horizontalCenter
                glyph: "refresh"
                text: I18n.tr("Scan again")
                onAct: {
                    root.stopDiscovery();
                    Qt.callLater(root.syncDiscovery);
                }
            }
        }
    }
}

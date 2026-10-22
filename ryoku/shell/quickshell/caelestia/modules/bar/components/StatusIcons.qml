pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Bluetooth
import Quickshell.Services.UPower
import Caelestia
import Caelestia.Config
import caelestia.components
import caelestia.services
import caelestia.utils

StyledRect {
    id: root

    property color colour: Colours.palette.m3secondary
    readonly property alias items: iconColumn

    property var bar: null
    readonly property var popouts: bar?.popouts ?? null

    property bool isDragging: false

    readonly property bool isHorizontal: Config.bar.position === "top" || Config.bar.position === "bottom"
    readonly property real rawScale: !isNaN(Config.bar.scale) ? Config.bar.scale : 1.0
    readonly property real scaleFactor: rawScale < 1.0 ? Math.sqrt(Math.max(0.1, rawScale)) : rawScale
    readonly property int barThickness: Math.round(Tokens.sizes.bar.innerWidth * scaleFactor)
    readonly property int baseThickness: Tokens.sizes.bar.innerWidth
    readonly property int effectiveThickness: rawScale < 1.0 ? baseThickness : barThickness
    readonly property int iconSize: Math.round(effectiveThickness * 0.42)

    property real hoverPos: -1
    property real hoverSpacing: Tokens.spacing.small

    readonly property bool isHovering: hoverPos !== -1
    property real currentHoverSpacing: isHovering ? hoverSpacing : 0

    readonly property var activeEntries: Config.bar.statusIcons.values.filter(entry => entry.enabled && root.entryActive(entry.id))

    readonly property var popoutNames: ({
            lockStatus: "lockstatus",
            kbLayout: "kblayout",
            microphone: "audio"
        })

    function popoutFor(id: string): string {
        return root.popoutNames[id] ?? id;
    }

    // Which icons are shown, and in what order, is the `bar.statusIcons` list, so
    // that the settings page can add, remove and reorder them. Everything below is
    // the other half of the question: whether an icon has anything to say now.
    // The Controls page of the host decides which of the matching readings stay on the bar.
    function allowedByControls(id: string): bool {
        switch (id) {
        case "network":
            return HostState.section("connections") && HostState.element("wifi");
        case "ethernet":
            return HostState.section("connections") && HostState.element("ethernet");
        case "bluetooth":
            return HostState.section("connections") && HostState.element("bluetooth");
        case "audio":
            return HostState.section("levels") && HostState.element("volume");
        case "microphone":
            return HostState.element("micMute");
        case "battery":
            return HostState.section("vitals") && HostState.element("battery");
        case "nightlight":
            return HostState.element("nightLight");
        default:
            return true;
        }
    }

    function entryActive(id: string): bool {
        if (!root.allowedByControls(id))
            return false;
        switch (id) {
        case "lockStatus":
            return Kwin.capsLock || Kwin.numLock;
        case "kbLayout":
            return (Kwin.kbLayout || "").length > 0;
        case "network":
            return !Nmcli.activeEthernet || Config.bar.status.showWifi;
        case "ethernet":
            return Nmcli.activeEthernet;
        case "nightlight":
            return HyprSunset.active;
        case "audio":
        case "microphone":
        case "bluetooth":
        case "battery":
        case "peripheralBattery":
        case "notifications":
            return true;
        default:
            return false;
        }
    }

    function moveEntry(fromId: string, toId: string): void {
        const entries = GlobalConfig.bar.statusIcons.values;
        const from = entries.findIndex(entry => entry.id === fromId);
        const to = entries.findIndex(entry => entry.id === toId);
        if (from < 0 || to < 0 || from === to)
            return;
        GlobalConfig.bar.statusIcons.move(from, to);
    }

    function openContextMenu(): void {
        const popouts = root.popouts;
        if (!popouts)
            return;

        if (popouts.hasCurrent && popouts.currentName === "statusiconscontext") {
            popouts.hasCurrent = false;
        } else {
            popouts.currentName = "statusiconscontext";
            popouts.currentCenter = root.isHorizontal ? root.mapToItem(null, root.implicitWidth / 2, 0).x : (root.mapToItem(null, 0, root.implicitHeight / 2).y ?? 0);
            popouts.hasCurrent = true;
        }
    }

    color: Colours.tPalette.m3surfaceContainer
    radius: Tokens.rounding.full
    clip: true
    implicitWidth: isHorizontal ? (iconColumn.implicitWidth + Tokens.padding.medium * 2) : barThickness
    implicitHeight: isHorizontal ? barThickness : (iconColumn.implicitHeight + Tokens.padding.medium * 2)

    Behavior on currentHoverSpacing { Anim { type: Anim.DefaultEffects } }

    MouseArea {
        anchors.fill: parent
        acceptedButtons: Qt.RightButton

        onClicked: mouse => {
            if (mouse.button === Qt.RightButton)
                root.openContextMenu();
        }
    }

    Grid {
        id: iconColumn

        readonly property real baseSpacing: Tokens.spacing.medium / 2
        readonly property real dynamicSpacing: baseSpacing + root.currentHoverSpacing

        anchors.centerIn: parent

        columns: isHorizontal ? -1 : 1
        rows: isHorizontal ? 1 : -1
        flow: isHorizontal ? Grid.LeftToRight : Grid.TopToBottom

        columnSpacing: isHorizontal ? dynamicSpacing : baseSpacing
        rowSpacing: !isHorizontal ? dynamicSpacing : baseSpacing

        move: Transition {
            Anim {
                duration: root.isDragging ? Tokens.anim.durations.expressiveFastSpatial : 0
                properties: "x,y"
                type: Anim.FastSpatial
            }
        }

        Repeater {
            model: ScriptModel {
                values: root.activeEntries
            }

            delegate: Item {
                id: delegateContainer

                required property var modelData
                required property int index

                property string name: modelData.id
                readonly property string popoutName: root.popoutFor(modelData.id)

                implicitWidth: loader.implicitWidth
                implicitHeight: loader.implicitHeight
                width: implicitWidth
                height: implicitHeight

                DropArea {
                    anchors.fill: parent

                    onEntered: drag => {
                        const fromId = drag.source?.entryId;
                        const toId = delegateContainer.name;
                        if (fromId && toId && fromId !== toId)
                            root.moveEntry(fromId, toId);
                    }
                    onDropped: drag => {
                        const fromId = drag.source?.entryId;
                        const toId = delegateContainer.name;
                        if (fromId && toId && fromId !== toId)
                            root.moveEntry(fromId, toId);
                    }
                }

                Item {
                    id: dragItem

                    property int delegateIndex: delegateContainer.index
                    readonly property string entryId: modelData.id

                    width: delegateContainer.width
                    height: delegateContainer.height

                    Drag.active: dragArea.held
                    Drag.source: dragItem
                    Drag.hotSpot.x: width / 2
                    Drag.hotSpot.y: height / 2

                    states: [
                        State {
                            when: dragArea.held

                            ParentChange {
                                target: dragItem
                                parent: root
                            }
                            PropertyChanges {
                                target: dragItem
                                opacity: 0.8
                                z: 999
                            }
                        }
                    ]

                    Loader {
                        id: loader

                        anchors.centerIn: parent

                        sourceComponent: {
                            switch(name) {
                                case "lockStatus": return lockstatusComp;
                                case "audio": return audioComp;
                                case "microphone": return microphoneComp;
                                case "kbLayout": return kblayoutComp;
                                case "network": return networkComp;
                                case "ethernet": return ethernetComp;
                                case "bluetooth": return bluetoothComp;
                                case "battery": return batteryComp;
                                case "peripheralBattery": return peripheralBatteryComp;
                                case "nightlight": return nightlightComp;
                                case "notifications": return notificationsComp;
                                default: return null;
                            }
                        }
                    }

                    MouseArea {
                        id: dragArea

                        property bool held: false

                        anchors.fill: parent
                        drag.target: held ? dragItem : null
                        drag.axis: root.isHorizontal ? Drag.XAxis : Drag.YAxis
                        cursorShape: Qt.PointingHandCursor
                        acceptedButtons: Qt.LeftButton | Qt.RightButton

                        onPressed: mouse => {
                            if (mouse.button === Qt.LeftButton) {
                                held = true;
                                root.isDragging = true;
                            }
                        }
                        onReleased: mouse => {
                            if (mouse.button === Qt.LeftButton) {
                                dragItem.Drag.drop();
                                held = false;
                                root.isDragging = false;
                                dragItem.x = 0;
                                dragItem.y = 0;
                            }
                        }
                        onCanceled: {
                            held = false;
                            root.isDragging = false;
                            dragItem.x = 0;
                            dragItem.y = 0;
                        }
                        onClicked: mouse => {
                            if (mouse.button === Qt.RightButton) {
                                root.openContextMenu();
                            } else if (name === "notifications") {
                                const vis = Visibilities.getForActive();
                                vis.sidebar = !vis.sidebar;
                            } else if (Paths.hosted && root.popouts) {
                                // a host bar (the Ryoku bar style) has no hover popouts: a click asks
                                root.popouts.currentName = delegateContainer.popoutName ?? name;
                                root.popouts.currentCenter = delegateContainer.mapToItem(null, delegateContainer.width / 2, 0).x;
                                root.popouts.hasCurrent = true;
                            }
                        }
                    }
                }
            }
        }
    }

    Component {
        id: lockstatusComp

        GridLayout {
            columns: root.isHorizontal ? -1 : 1
            rows: root.isHorizontal ? 1 : -1
            flow: root.isHorizontal ? GridLayout.LeftToRight : GridLayout.TopToBottom
            columnSpacing: 0
            rowSpacing: 0

            Item {
                implicitWidth: root.isHorizontal ? (Kwin.capsLock ? capslockIcon.implicitWidth : 0) : capslockIcon.implicitWidth
                implicitHeight: root.isHorizontal ? capslockIcon.implicitHeight : (Kwin.capsLock ? capslockIcon.implicitHeight : 0)

                MaterialIcon {
                    id: capslockIcon

                    anchors.centerIn: parent

                    scale: Kwin.capsLock ? 1 : 0.5
                    opacity: Kwin.capsLock ? 1 : 0

                    text: "keyboard_capslock_badge"
                    color: root.colour

                    Behavior on opacity {
                        Anim {
                            type: Anim.DefaultEffects
                        }
                    }

                    Behavior on scale {
                        Anim {}
                    }
                }

                Behavior on implicitHeight {
                    enabled: !root.isHorizontal

                    Anim {}
                }

                Behavior on implicitWidth {
                    enabled: root.isHorizontal

                    Anim {}
                }
            }

            Item {
                Layout.topMargin: !root.isHorizontal && Kwin.capsLock && Kwin.numLock ? Tokens.spacing.medium / 2 : 0
                Layout.leftMargin: root.isHorizontal && Kwin.capsLock && Kwin.numLock ? Tokens.spacing.medium / 2 : 0

                implicitWidth: root.isHorizontal ? (Kwin.numLock ? numlockIcon.implicitWidth : 0) : numlockIcon.implicitWidth
                implicitHeight: root.isHorizontal ? numlockIcon.implicitHeight : (Kwin.numLock ? numlockIcon.implicitHeight : 0)

                MaterialIcon {
                    id: numlockIcon

                    anchors.centerIn: parent

                    scale: Kwin.numLock ? 1 : 0.5
                    opacity: Kwin.numLock ? 1 : 0

                    text: "looks_one"
                    color: root.colour

                    Behavior on opacity {
                        Anim {
                            type: Anim.DefaultEffects
                        }
                    }

                    Behavior on scale {
                        Anim {}
                    }
                }

                Behavior on implicitHeight {
                    enabled: !root.isHorizontal

                    Anim {}
                }

                Behavior on implicitWidth {
                    enabled: root.isHorizontal

                    Anim {}
                }
            }
        }
    }

    Component {
        id: audioComp

        MaterialIcon {
            animate: true
            text: Icons.getVolumeIcon(Audio.volume, Audio.muted)
            color: root.colour
        }
    }

    Component {
        id: microphoneComp

        MaterialIcon {
            animate: true
            text: Icons.getMicVolumeIcon(Audio.sourceVolume, Audio.sourceMuted)
            color: root.colour
        }
    }

    Component {
        id: kblayoutComp

        StyledText {
            animate: true
            text: Kwin.kbLayout
            color: root.colour
            font: Tokens.font.mono.medium
        }
    }

    Component {
        id: networkComp

        MaterialIcon {
            animate: true
            text: Nmcli.active ? Icons.getNetworkIcon(Nmcli.active.strength ?? 0) : "wifi_off"
            color: root.colour
        }
    }

    Component {
        id: ethernetComp

        MaterialIcon {
            animate: true
            text: "cable"
            color: root.colour
        }
    }

    Component {
        id: bluetoothComp

        GridLayout {
            columns: root.isHorizontal ? -1 : 1
            rows: root.isHorizontal ? 1 : -1
            flow: root.isHorizontal ? GridLayout.LeftToRight : GridLayout.TopToBottom
            columnSpacing: Tokens.spacing.medium / 2
            rowSpacing: Tokens.spacing.medium / 2

            MaterialIcon {
                visible: !Bluetooth.defaultAdapter?.enabled || !Bluetooth.devices.values.some(d => d.state !== BluetoothDeviceState.Disconnected) // qmllint disable unresolved-type
                animate: true
                text: {
                    if (!Bluetooth.defaultAdapter?.enabled) // qmllint disable unresolved-type
                        return "bluetooth_disabled";
                    return "bluetooth";
                }
                color: root.colour
            }

            Repeater {
                model: ScriptModel {
                    values: Bluetooth.devices.values.filter(d => d.state !== BluetoothDeviceState.Disconnected) // qmllint disable unresolved-type
                }

                MaterialIcon {
                    id: device

                    required property BluetoothDevice modelData

                    animate: true
                    text: Icons.getBluetoothIcon(modelData?.icon)
                    color: root.colour
                    fill: 1

                    SequentialAnimation on opacity {
                        running: device.modelData?.state !== BluetoothDeviceState.Connected // qmllint disable unresolved-type
                        alwaysRunToEnd: true
                        loops: Animation.Infinite

                        Anim {
                            from: 1
                            to: 0
                            duration: Tokens.anim.durations.large
                            easing: Tokens.anim.standardAccel
                        }
                        Anim {
                            from: 0
                            to: 1
                            duration: Tokens.anim.durations.large
                            easing: Tokens.anim.standardDecel
                        }
                    }
                }
            }
        }
    }

    Component {
        id: batteryComp

        MaterialIcon {
            animate: true
            text: {
                if (!UPower.displayDevice.isLaptopBattery) {
                    if (PowerProfiles.profile === PowerProfile.PowerSaver)
                        return "energy_savings_leaf";
                    if (PowerProfiles.profile === PowerProfile.Performance)
                        return "rocket_launch";
                    return "balance";
                }
                return Icons.getBatteryIcon(UPower.displayDevice.percentage, [UPowerDeviceState.Charging, UPowerDeviceState.FullyCharged, UPowerDeviceState.PendingCharge].includes(UPower.displayDevice.state));
            }
            color: !UPower.onBattery || UPower.displayDevice.percentage > 0.2 ? root.colour : Colours.palette.m3error
            fill: 1
        }
    }

    Component {
        id: peripheralBatteryComp

        GridLayout {
            id: peripheralColumn

            readonly property var excluded: Config.bar.status.peripheralBatteryExcluded

            columns: root.isHorizontal ? -1 : 1
            rows: root.isHorizontal ? 1 : -1
            flow: root.isHorizontal ? GridLayout.LeftToRight : GridLayout.TopToBottom
            columnSpacing: Tokens.spacing.medium / 2
            rowSpacing: Tokens.spacing.medium / 2

            Repeater {
                model: ScriptModel {
                    values: UPower.devices.values.filter(d => !d.isLaptopBattery && d.type !== UPowerDeviceType.LinePower && d.isPresent && !peripheralColumn.excluded.some(e => e === d.model || e === d.nativePath)) // qmllint disable unresolved-type
                }

                MaterialIcon {
                    required property UPowerDevice modelData

                    animate: true
                    text: {
                        if (modelData.state === UPowerDeviceState.Charging || modelData.state === UPowerDeviceState.PendingCharge)
                            return "battery_charging_full";
                        if (modelData.state === UPowerDeviceState.FullyCharged)
                            return "battery_full";
                        return Icons.getBatteryIcon(modelData.percentage, false);
                    }
                    color: modelData.percentage > 0.2 ? root.colour : Colours.palette.m3error
                    fill: 1
                }
            }
        }
    }

    Component {
        id: nightlightComp

        MaterialIcon {
            animate: true
            text: "bedtime"
            color: root.colour
        }
    }

    Component {
        id: notificationsComp

        MaterialIcon {
            id: notifIcon

            text: {
                if (Notifs.dnd)
                    return "notifications_off";
                if (Notifs.openCount > 0)
                    return "notifications_unread";
                return "notifications";
            }
            color: root.colour
        }
    }
}

pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Bluetooth
import Quickshell.Io
import ".."
import shell.services
import "../../../components"
import Ryoku.Ui.Singletons

// Bluetooth popout: a frame-edge card (the shared PopoutCard skin, so it opens,
// melts and dismisses exactly like the music card) leading with the hero for the
// focused device, then a compact rail of hairline chips to switch, connect or
// pair the rest; power and scan ride the head. Everything is live off
// Quickshell.Bluetooth and the Audio graph; connecting a bonded device is
// instant, pairing a fresh one shells the reliable bluetoothctl pair/trust/
// connect chain. Material clarity, TUI density; never grabs the keyboard.
Item {
    id: root

    property real s: 1
    property bool open: false

    readonly property real pad: 11 * root.s
    readonly property color ink: Theme.ink(Theme.effectiveSurface)
    readonly property color inkDim: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
    readonly property color line: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.14)

    readonly property var adapter: Bluetooth.defaultAdapter
    readonly property bool hasAdapter: root.adapter !== null
    readonly property bool adapterOn: root.hasAdapter && root.adapter.enabled
    readonly property bool blocked: BluetoothPower.blocked
    readonly property bool discovering: root.adapterOn && root.adapter.discovering

    readonly property var devices: (root.adapterOn && Bluetooth.devices) ? Bluetooth.devices.values : []
    readonly property var known: root.devices.filter(d => d && (d.paired || d.bonded))
    readonly property var found: root.devices.filter(d => d && !(d.paired || d.bonded) && d.name && d.name.length > 0)

    property string focusAddr: ""
    readonly property var connectedDev: {
        for (let i = 0; i < root.devices.length; i++)
            if (root.devices[i] && root.devices[i].connected)
                return root.devices[i];
        return null;
    }
    function deviceByAddr(a) {
        if (!a)
            return null;
        for (let i = 0; i < root.devices.length; i++)
            if (root.devices[i] && ((root.devices[i].address || "") + "").toUpperCase() === a.toUpperCase())
                return root.devices[i];
        return null;
    }
    readonly property var focusDev: root.deviceByAddr(root.focusAddr) || root.connectedDev
        || (root.known.length > 0 ? root.known[0] : null)

    readonly property var railDevices: {
        const fa = root.focusDev ? ((root.focusDev.address || "") + "").toUpperCase() : "";
        const out = [];
        for (let i = 0; i < root.known.length; i++)
            if (((root.known[i].address || "") + "").toUpperCase() !== fa)
                out.push(root.known[i]);
        if (root.discovering)
            for (let j = 0; j < root.found.length; j++)
                if (((root.found[j].address || "") + "").toUpperCase() !== fa)
                    out.push(root.found[j]);
        return out;
    }

    property string busyAddr: ""
    property string errorText: ""
    readonly property bool focusBusy: root.focusDev
        && root.busyAddr === ((root.focusDev.address || "") + "")

    onOpenChanged: {
        BtLink.watch(root.open);
        if (!root.open) {
            root.stopScan();
            root.focusAddr = "";
            root.busyAddr = "";
            root.errorText = "";
        }
    }
    Component.onDestruction: { BtLink.watch(false); root.stopScan(); }
    onDiscoveringChanged: { if (root.discovering) scanStop.restart(); else scanStop.stop(); }

    // --- actions --------------------------------------------------------------
    function toggleAdapter() {
        BluetoothPower.toggle();
    }

    function toggleScan() {
        if (!root.adapterOn)
            return;
        BluetoothDiscovery.setDiscovering(root, root.adapter, !root.discovering);
    }
    function stopScan() { BluetoothDiscovery.setDiscovering(root, root.adapter, false); scanStop.stop(); }
    Timer { id: scanStop; interval: 30000; onTriggered: root.stopScan() }

    function primary(d) {
        if (!d)
            return;
        if (d.connected) {
            d.disconnect();
            return;
        }
        if (d.blocked)
            d.blocked = false;
        // Paired and unpaired both go through the same script. Device1.Connect
        // on an existing bond was the dead end in #144/#156: it fails at once
        // when BlueZ holds keys the device has forgotten, and nothing here ever
        // cleared that. BtLink.linkCommand pairs if needed, retries, and
        // rebuilds a bond that will not carry a connection.
        root.link(d);
    }
    function link(d) {
        if (!d || linkProc.running)
            return;
        root.busyAddr = d.address;
        root.errorText = "";
        linkProc.command = BtLink.linkCommand(d.address);
        linkProc.running = false;
        linkProc.running = true;
    }
    Process {
        id: linkProc
        property string collected: ""
        // stderr as well as stdout: the script redirects bluetoothctl's own
        // stderr, but a failure in the script itself (no bash, no
        // bluetoothctl) only ever lands here, and losing it is what left the
        // user with a generic message and no way to tell why.
        stdout: StdioCollector { onStreamFinished: linkProc.collected += this.text }
        stderr: StdioCollector { onStreamFinished: linkProc.collected += this.text }
        onExited: code => {
            if (code !== 0) {
                const lines = linkProc.collected.trim().split("\n");
                const msg = lines.length ? lines[lines.length - 1].trim() : "";
                root.errorText = msg.length ? msg
                    : I18n.tr("Could not connect. Put the device in pairing mode and try again.");
            } else {
                root.errorText = "";
            }
            linkProc.collected = "";
            root.busyAddr = "";
        }
    }
    function tapDevice(d) {
        if (!d)
            return;
        root.focusAddr = d.address;
        root.errorText = "";
        if (!d.connected && !d.pairing && root.busyAddr !== ((d.address || "") + ""))
            root.primary(d);
    }

    implicitWidth: 244 * root.s
    implicitHeight: content.implicitHeight + root.pad * 2

    // the shared card skin: same frame + click-swallow the music card uses.
    PopoutCard { anchors.fill: parent }

    // a hairline device chip in the rail: class glyph, name, connected/pending pip.
    component DeviceChip: Item {
        id: chip
        property var dev: null
        readonly property bool conn: chip.dev && chip.dev.connected
        readonly property bool pend: chip.dev && (chip.dev.pairing
            || root.busyAddr === ((chip.dev.address || "") + ""))
        implicitHeight: 24 * root.s
        implicitWidth: chipRow.implicitWidth + 14 * root.s
        Rectangle {
            anchors.fill: parent
            radius: 3 * root.s
            color: cTap.pressed ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.16)
                : (cHover.hovered ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.08) : "transparent")
            border.width: Theme.borderWidth
            border.color: chip.conn ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.4) : root.line
            Behavior on color { ColorAnimation { duration: Motion.fast } }
        }
        Row {
            id: chipRow
            anchors.centerIn: parent
            spacing: 5 * root.s
            GlyphIcon {
                anchors.verticalCenter: parent.verticalCenter
                width: 13 * root.s
                height: width
                name: BtLink.glyphFor(chip.dev)
                stroke: 1.6
                color: chip.conn ? root.ink : root.inkDim
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: BtLink.label(chip.dev)
                color: chip.conn ? root.ink : root.inkDim
                font.family: Theme.fontPrimary
                font.pixelSize: 11 * root.s
                font.weight: chip.conn ? Font.DemiBold : Font.Normal
                elide: Text.ElideRight
                maximumLineCount: 1
                width: Math.min(implicitWidth, 104 * root.s)
            }
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                visible: chip.conn || chip.pend
                width: 4 * root.s
                height: width
                radius: width / 2
                color: root.ink
                opacity: chip.pend ? 0.5 : 1
                SequentialAnimation on opacity {
                    running: chip.pend
                    loops: Animation.Infinite
                    NumberAnimation { from: 0.25; to: 0.9; duration: 600 }
                    NumberAnimation { from: 0.9; to: 0.25; duration: 600 }
                }
            }
        }
        HoverHandler { id: cHover; cursorShape: Qt.PointingHandCursor }
        MouseArea { id: cTap; anchors.fill: parent; onClicked: root.tapDevice(chip.dev) }
    }

    // --- layout ---------------------------------------------------------------
    Column {
        id: content
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: root.pad
        spacing: 8 * root.s

        // head: label, scan, power.
        Item {
            width: parent.width
            height: 20 * root.s
            Text {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("BLUETOOTH")
                color: root.inkDim
                font.family: Theme.mono
                font.pixelSize: 9 * root.s
                font.letterSpacing: 1.6
                font.weight: Font.Medium
            }
            Row {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 5 * root.s
                PopoutHeadButton {
                    visible: root.adapterOn
                    s: root.s
                    glyph: "reboot"
                    active: root.discovering
                    spin: root.discovering
                    onClicked: root.toggleScan()
                }
                PopoutHeadButton {
                    s: root.s
                    glyph: "shutdown"
                    active: root.adapterOn
                    onClicked: root.toggleAdapter()
                }
            }
        }

        // OFF / BLOCKED plate.
        Column {
            width: parent.width
            spacing: 7 * root.s
            visible: !root.adapterOn
            topPadding: 6 * root.s
            bottomPadding: 4 * root.s
            GlyphIcon {
                anchors.horizontalCenter: parent.horizontalCenter
                width: 27 * root.s
                height: width
                name: "bluetooth"
                stroke: 1.6
                color: root.inkDim
                opacity: 0.55
            }
            Text {
                width: parent.width
                horizontalAlignment: Text.AlignHCenter
                text: root.blocked ? I18n.tr("Bluetooth is blocked") : I18n.tr("Bluetooth is off")
                color: root.ink
                font.family: Theme.fontPrimary
                font.pixelSize: 12 * root.s
                font.weight: Font.DemiBold
            }
            PopoutAction {
                anchors.horizontalCenter: parent.horizontalCenter
                s: root.s
                label: root.blocked ? I18n.tr("Unblock") : I18n.tr("Turn on")
                onClicked: root.toggleAdapter()
            }
        }

        // HERO for the focused device.
        BtHero {
            width: parent.width
            visible: root.adapterOn && root.focusDev !== null
            s: root.s
            device: root.focusDev
            busy: root.focusBusy
            onPrimaryActed: root.primary(root.focusDev)
            onForgetActed: {
                if (root.focusDev) {
                    root.focusDev.forget();
                    root.focusAddr = "";
                }
            }
        }

        // pairing error.
        Text {
            width: parent.width
            visible: root.errorText.length > 0
            horizontalAlignment: Text.AlignHCenter
            text: root.errorText
            color: Theme.error
            wrapMode: Text.WordWrap
            font.family: Theme.fontPrimary
            font.pixelSize: 10 * root.s
        }

        // rail label, only when a hero already leads.
        Text {
            width: parent.width
            visible: root.adapterOn && root.focusDev !== null && root.railDevices.length > 0
            text: root.discovering ? I18n.tr("OTHER · SCANNING") : I18n.tr("OTHER DEVICES")
            color: root.inkDim
            font.family: Theme.mono
            font.pixelSize: 8.5 * root.s
            font.letterSpacing: 1.5
        }

        // DEVICE RAIL.
        Flow {
            width: parent.width
            spacing: 5 * root.s
            visible: root.adapterOn && root.railDevices.length > 0
            Repeater {
                model: root.railDevices
                delegate: DeviceChip {
                    required property var modelData
                    dev: modelData
                }
            }
        }

        // EMPTY plate.
        Column {
            width: parent.width
            spacing: 7 * root.s
            visible: root.adapterOn && root.focusDev === null && root.railDevices.length === 0
            topPadding: 6 * root.s
            bottomPadding: 4 * root.s
            GlyphIcon {
                anchors.horizontalCenter: parent.horizontalCenter
                width: 27 * root.s
                height: width
                name: "bluetooth"
                stroke: 1.6
                color: root.inkDim
                opacity: 0.55
            }
            Text {
                width: parent.width
                horizontalAlignment: Text.AlignHCenter
                text: root.discovering ? I18n.tr("Scanning…") : I18n.tr("No devices yet")
                color: root.ink
                font.family: Theme.fontPrimary
                font.pixelSize: 12 * root.s
                font.weight: Font.DemiBold
            }
            PopoutAction {
                anchors.horizontalCenter: parent.horizontalCenter
                s: root.s
                enabled: !root.discovering
                label: root.discovering ? I18n.tr("Scanning…") : I18n.tr("Scan")
                onClicked: root.toggleScan()
            }
        }
    }
}

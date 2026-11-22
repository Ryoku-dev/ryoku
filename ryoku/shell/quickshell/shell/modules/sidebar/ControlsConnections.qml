pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import "../ask/look"

Item {
    id: root

    required property real s
    required property var screen
    required property bool active
    signal openPage(string page)
    signal requestClose()

    readonly property bool showWifi: SidebarState.elementVisible("wifi")
    readonly property bool showBluetooth: SidebarState.elementVisible("bluetooth")

    readonly property var connectedDevices: root.showBluetooth
        ? BluetoothPower.devices.filter(device => device && device.connected) : []
    readonly property bool joined: Network.activeSsid !== ""
    readonly property string wifiGlyph: {
        if (!Network.wifiPresent || !Network.wifiRadio)
            return "wifi_off";
        if (!root.joined)
            return "wifi";
        const level = Network.level;
        return level >= 0.8 ? "signal_wifi_4_bar" : level >= 0.55 ? "network_wifi_3_bar"
            : level >= 0.3 ? "network_wifi_2_bar" : level > 0.05 ? "network_wifi_1_bar"
            : "signal_wifi_0_bar";
    }
    readonly property string wifiDetail: {
        if (!root.showWifi)
            return "";
        if (!Network.wifiPresent)
            return I18n.tr("No Wi-Fi adapter");
        if (!Network.wifiRadio)
            return I18n.tr("Radio off");
        if (!root.joined)
            return Network.wifiConnectivity === "Connecting" ? I18n.tr("Connecting…") : I18n.tr("Not connected");
        return Network.activeSsid;
    }
    readonly property string bluetoothDetail: BluetoothPower.blocked ? I18n.tr("Blocked")
        : !BluetoothPower.available ? I18n.tr("No adapter")
        : !BluetoothPower.enabled ? I18n.tr("Radio off")
        : root.connectedDevices.length === 1 ? BtLink.label(root.connectedDevices[0])
        : root.connectedDevices.length > 1 ? I18n.tr("%1 connected").arg(root.connectedDevices.length)
        : I18n.tr("Not connected")

    readonly property real gap: Math.round(QsLook.tileGap * root.s)

    // a hidden cell keeps the place it would land in so it fades in there instead of travelling
    readonly property var arrangement: {
        const places = [];
        let top = 0;
        let filled = 0;
        let columns = 0;
        let rowHeight = 0;
        const lay = (cells, count, height) => {
            const span = Math.max(0, root.width - root.gap * (count - 1));
            const base = Math.floor(span / count);
            for (const cell of cells) {
                let row = top;
                let slot = filled;
                if (filled > 0 && (filled === count || count !== columns || height !== rowHeight)) {
                    row = top + rowHeight + root.gap;
                    slot = 0;
                }
                places.push({
                    x: slot * (base + root.gap),
                    y: row,
                    width: slot === count - 1 ? span - base * (count - 1) : base
                });
                if (cell.present) {
                    top = row;
                    filled = slot + 1;
                    columns = count;
                    rowHeight = height;
                }
            }
        };
        lay([wifiCell, bluetoothCell, ethernetCell, vpnCell], QsLook.tileColumns("large"), wifiTile.implicitHeight);
        lay([nightCell, awakeCell, dndCell, micCell, gameCell], QsLook.tileColumns("small"), nightTile.implicitHeight);
        return { places: places, height: filled > 0 ? top + rowHeight : 0 };
    }

    function openSettings() {
        Bus.run("hub open wifi");
        root.requestClose();
    }

    implicitHeight: root.arrangement.height
    Accessible.role: Accessible.Grouping
    Accessible.name: I18n.tr("Connections and switches")

    component Cell: Item {
        id: cell

        property int order: 0
        property bool shown: false
        readonly property bool present: cell.opacity > 0
        readonly property var place: root.arrangement.places[cell.order]
        readonly property bool settled: root.active && cell.shown && cell.opacity >= 1
        readonly property real travel: QsLook.lithic ? 6 : QsLook.fluent ? -6 : 0
        readonly property real shrink: QsLook.material ? 0.1 : QsLook.adwaita ? 0.04 : 0

        x: cell.place.x
        y: cell.place.y
        width: cell.place.width
        height: cell.childrenRect.height
        visible: cell.present
        opacity: cell.shown ? 1 : 0
        scale: 1 - (1 - cell.opacity) * cell.shrink
        transform: Translate {
            y: Math.round((1 - cell.opacity) * cell.travel * root.s)
        }

        // an arriving spring may overshoot but a leaving fade must not dip below 0
        Behavior on opacity {
            id: fade
            NumberAnimation {
                duration: root.active ? QsLook.dur(fade.targetValue > 0 ? QsLook.unfoldMs : QsLook.closeMs) : 0
                easing.type: Easing.BezierSpline
                easing.bezierCurve: fade.targetValue > 0 ? QsLook.arrive : QsLook.leave
            }
        }
        Behavior on x {
            enabled: cell.settled
            NumberAnimation { duration: QsLook.dur(QsLook.unfoldMs); easing.type: Easing.BezierSpline; easing.bezierCurve: QsLook.unfold }
        }
        Behavior on y {
            enabled: cell.settled
            NumberAnimation { duration: QsLook.dur(QsLook.unfoldMs); easing.type: Easing.BezierSpline; easing.bezierCurve: QsLook.unfold }
        }
    }

    Cell {
        id: wifiCell
        order: 0
        shown: root.showWifi

        Tile {
            id: wifiTile
            width: parent.width
            s: root.s
            size: "large"
            glyph: root.wifiGlyph
            label: I18n.tr("Wi-Fi")
            detail: root.wifiDetail
            caption: root.joined ? Network.activeSsid : I18n.tr("Wi-Fi")
            on: Network.wifiRadio
            available: Network.wifiPresent
            split: true
            tone: root.joined ? "alive" : Network.wifiConnectivity === "Connecting" ? "becoming" : ""
            pageName: I18n.tr("Wi-Fi networks")
            onToggled: Toggles.toggleWifi()
            onOpened: root.openPage("wifi")
        }
    }

    Cell {
        id: bluetoothCell
        order: 1
        shown: root.showBluetooth

        Tile {
            width: parent.width
            s: root.s
            size: "large"
            glyph: root.connectedDevices.length > 0 ? "bluetooth_connected"
                : BluetoothPower.enabled ? "bluetooth" : "bluetooth_disabled"
            label: I18n.tr("Bluetooth")
            detail: root.bluetoothDetail
            caption: root.connectedDevices.length === 1 ? BtLink.label(root.connectedDevices[0]) : I18n.tr("Bluetooth")
            on: BluetoothPower.enabled
            available: BluetoothPower.available
            split: true
            tone: root.connectedDevices.length > 0 ? "alive" : ""
            pageName: I18n.tr("Bluetooth devices")
            onToggled: Toggles.toggleBt()
            onOpened: root.openPage("bluetooth")
        }
    }

    Cell {
        id: ethernetCell
        order: 2
        shown: SidebarState.elementVisible("ethernet") && Network.kind === "ethernet"

        Tile {
            width: parent.width
            s: root.s
            size: "large"
            glyph: "lan"
            label: I18n.tr("Ethernet")
            detail: I18n.tr("Connected")
            on: true
            toggles: false
            split: true
            pageName: I18n.tr("Wi-Fi and internet settings")
            tone: "alive"
            onOpened: root.openSettings()
        }
    }

    Cell {
        id: vpnCell
        order: 3
        shown: SidebarState.elementVisible("vpn") && Network.vpnActive

        Tile {
            width: parent.width
            s: root.s
            size: "large"
            glyph: "vpn_lock"
            label: I18n.tr("VPN")
            detail: Network.vpnName || I18n.tr("Connected")
            on: true
            toggles: false
            split: true
            pageName: I18n.tr("Wi-Fi and internet settings")
            tone: "alive"
            onOpened: root.openSettings()
        }
    }

    Cell {
        id: nightCell
        order: 4
        shown: SidebarState.elementVisible("nightLight") && Wm.caps.nightLight === true

        Tile {
            id: nightTile
            width: parent.width
            s: root.s
            size: "small"
            glyph: Toggles.nightOn ? "bedtime" : "bedtime_off"
            label: I18n.tr("Night light")
            on: Toggles.nightOn
            onToggled: Toggles.toggleNight()
        }
    }

    Cell {
        id: awakeCell
        order: 5
        shown: SidebarState.elementVisible("keepAwake")

        Tile {
            width: parent.width
            s: root.s
            size: "small"
            glyph: "coffee"
            label: I18n.tr("Keep awake")
            on: Toggles.keepAwake
            onToggled: Toggles.toggleCaffeine()
        }
    }

    Cell {
        id: dndCell
        order: 6
        shown: SidebarState.elementVisible("doNotDisturb")

        Tile {
            width: parent.width
            s: root.s
            size: "small"
            glyph: Toggles.dnd ? "do_not_disturb_on" : "notifications"
            label: I18n.tr("Do not disturb")
            on: Toggles.dnd
            onToggled: Toggles.toggleDnd()
        }
    }

    Cell {
        id: micCell
        order: 7
        shown: SidebarState.elementVisible("micMute")

        // on means live, as Android's microphone tile reads
        Tile {
            width: parent.width
            s: root.s
            size: "small"
            glyph: Toggles.micMuted ? "mic_off" : "mic"
            label: I18n.tr("Microphone")
            detail: Toggles.micMuted ? I18n.tr("Muted") : I18n.tr("On")
            on: !Toggles.micMuted
            available: !!(Audio.source && Audio.source.audio)
            onToggled: Toggles.toggleMic()
        }
    }

    Cell {
        id: gameCell
        order: 8
        shown: SidebarState.elementVisible("gamingMode") && (Wm.caps.liveConfigEval === true || Toggles.gameMode)

        Tile {
            width: parent.width
            s: root.s
            size: "small"
            glyph: "sports_esports"
            label: I18n.tr("Game mode")
            on: Toggles.gameMode
            onToggled: Toggles.toggleGame()
        }
    }
}

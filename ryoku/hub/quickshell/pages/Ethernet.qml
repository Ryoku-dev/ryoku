import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/network"
import "../components/wifi"
import "../schema/ethernet.js" as Schema

// the wired adapters: link, speed and address, Connect or Disconnect, the saved profile an adapter
// uses, auto-connect, and the profile's IP settings. Everything goes through the shell daemon
Item {
    id: root

    required property var hub
    property string pendingRow: ""

    // the adapter whose Connect or Disconnect is running, and a profile picked for one that is down
    property string busyDevice: ""
    property var errors: ({})
    property var chosen: ({})
    // adapters whose IP settings are open
    property var ipOpen: ({})

    // the cards by adapter name: only a change in which adapters exist rebuilds them, so an IP box
    // being typed in survives every frame NetworkManager sends
    property var deviceNames: []
    function syncDevices() {
        var names = NetworkLink.ethernet.map(d => d.device);
        if (names.join("\n") !== root.deviceNames.join("\n"))
            root.deviceNames = names;
    }
    Component.onCompleted: root.syncDevices()
    Connections {
        target: NetworkLink
        function onEthernetChanged() { root.syncDevices(); }
    }

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: []
    })

    readonly property string stateLine: {
        if (!NetworkLink.ready)
            return "";
        var list = NetworkLink.ethernet;
        if (list.length === 0)
            return I18n.tr("No wired adapter");
        var up = list.find(d => d.state === "connected");
        if (up)
            return I18n.tr("Connected by cable, %1").arg(root.speedText(up.speed) || up.device);
        return list.some(d => d.state === "connecting") ? I18n.tr("Connecting…") : I18n.tr("Not connected by cable");
    }

    function devOf(name) {
        return NetworkLink.ethernet.find(d => d.device === name) || null;
    }

    function setError(device, message) {
        var next = Object.assign({}, root.errors);
        if (message)
            next[device] = message;
        else
            delete next[device];
        root.errors = next;
    }

    // the saved profiles an adapter can use: those bound to it and the unbound ones
    function profilesFor(device) {
        return NetworkLink.ethernetProfiles.filter(p => !p.device || p.device === device);
    }

    // the profile the adapter's controls act on: the one in use, else the one picked, else the first
    function profileOf(dev) {
        if (!dev)
            return null;
        var list = root.profilesFor(dev.device);
        var want = dev.activeUuid || root.chosen[dev.device] || "";
        return list.find(p => p.uuid === want) || (list.length > 0 ? list[0] : null);
    }

    function linked(dev) {
        return !!dev && (dev.state === "connected" || dev.state === "connecting");
    }

    function speedText(mbps) {
        if (!mbps)
            return "";
        return mbps >= 1000 ? (mbps / 1000) + " Gb/s" : mbps + " Mb/s";
    }

    function statusText(dev) {
        if (!dev)
            return "";
        switch (dev.state) {
        case "connected":
            return [dev.activeName, root.speedText(dev.speed), dev.ip4].filter(s => !!s).join(" \u00b7 ") || I18n.tr("Connected");
        case "connecting":
            return I18n.tr("Connecting…");
        case "unplugged":
            return I18n.tr("No cable");
        case "unmanaged":
            return I18n.tr("Not managed by NetworkManager");
        }
        return I18n.tr("Cable in, not connected");
    }

    function done(device) {
        return function (ok, error) {
            root.busyDevice = "";
            if (!ok)
                root.setError(device, error || I18n.tr("The Ethernet change failed."));
        };
    }

    function connectDevice(device, uuid) {
        root.busyDevice = device;
        root.setError(device, "");
        NetworkLink.ethernetConnect(device, uuid, root.done(device));
    }

    function disconnectDevice(device) {
        root.busyDevice = device;
        root.setError(device, "");
        NetworkLink.ethernetDisconnect(device, root.done(device));
    }

    function setAutoconnect(device, uuid, on) {
        root.setError(device, "");
        NetworkLink.ethernetSetAutoconnect(uuid, on, function (ok, error) {
            if (!ok)
                root.setError(device, error || I18n.tr("Could not change auto-connect."));
        });
    }

    function pick(dev, uuid) {
        if (root.linked(dev)) {
            if (uuid !== dev.activeUuid)
                root.connectDevice(dev.device, uuid);
            return;
        }
        var next = Object.assign({}, root.chosen);
        next[dev.device] = uuid;
        root.chosen = next;
    }

    function toggleIp(device) {
        var next = Object.assign({}, root.ipOpen);
        next[device] = !next[device];
        root.ipOpen = next;
    }

    // a landing waits for its row, which needs the adapters read
    function landable(rowId) {
        return page.layout.length > 0 && (!!page.rowItems[rowId] || root.isSectionLink(rowId))
            && (NetworkLink.ready || NetworkLink.unavailable !== "" && focusRetry.tries >= 5);
    }

    function isSectionLink(id) {
        return !Schema.rows.some(r => r.id === id) && Schema.sections.some(s => s.id === id);
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (rowId !== root.pendingRow)
            focusRetry.tries = 0;
        if (!root.landable(rowId) && focusRetry.tries < 30) {
            root.pendingRow = rowId;
            focusRetry.tries++;
            focusRetry.restart();
            return;
        }
        root.pendingRow = "";
        if (root.isSectionLink(rowId)) {
            page.focusRow(rowId);
            return;
        }
        if (page.rowItems[rowId])
            page.land(rowId);
        else
            page.chooseSection(0);
    }

    Timer {
        id: focusRetry
        property int tries: 0
        interval: 100
        onTriggered: root.focusRow(root.pendingRow)
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "ethernet"
        schema: root.schema
        stateLine: root.stateLine
        bodies: ({ adapters: adaptersBody })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    Component {
        id: adaptersBody

        Column {
            width: parent ? parent.width : 0
            spacing: LithicTokens.space.s4

            LinkNotice {
                host: page
            }

            Slab {
                visible: NetworkLink.ready && root.deviceNames.length === 0
                width: parent.width

                Column {
                    width: parent.width

                    NetRow {
                        host: page
                        rowId: visible ? "wired-adapters" : ""
                        first: true
                        title: I18n.tr("No wired adapter")
                        detail: I18n.tr("Plug in a cable or a USB Ethernet adapter and it shows here")
                    }
                }
            }

            Repeater {
                model: root.deviceNames
                delegate: AdapterCard {}
            }
        }
    }

    component AdapterCard: Slab {
        id: card

        required property string modelData
        required property int index
        readonly property var dev: root.devOf(card.modelData) || card.last
        property var last: ({ device: card.modelData, state: "disconnected", speed: 0, ip4: "", activeUuid: "", activeName: "" })
        onDevChanged: card.last = card.dev
        readonly property var profiles: root.profilesFor(card.dev.device)
        readonly property var profile: root.profileOf(card.dev)
        readonly property bool linked: root.linked(card.dev)
        readonly property bool usable: card.dev.state !== "unplugged" && card.dev.state !== "unmanaged"
        readonly property bool mine: root.busyDevice === card.dev.device
        readonly property bool ipShown: root.ipOpen[card.dev.device] === true
        readonly property string error: root.errors[card.dev.device] || ""
        readonly property bool first: card.index === 0

        width: parent ? parent.width : 0

        Pace { id: cardPace; running: card.mine }

        Column {
            width: parent.width

            NetRow {
                host: page
                rowId: card.first ? "wired-adapters" : ""
                first: true
                title: card.dev.device
                detail: cardPace.slow ? I18n.tr("Working…") : root.statusText(card.dev)
                error: card.error
                onRetry: root.setError(card.dev.device, "")
                canRetry: false

                Glyph {
                    anchors.verticalCenter: parent.verticalCenter
                    name: card.dev.state === "unplugged" ? "link_off" : "settings_ethernet"
                    ink: card.linked ? Lithic.fg : Lithic.fg3
                    size: LithicTokens.size.iconPlaqueIcon
                }
                Working {
                    anchors.verticalCenter: parent.verticalCenter
                    visible: cardPace.slow
                    active: visible
                }
                Plaque {
                    anchors.verticalCenter: parent.verticalCenter
                    visible: card.usable
                    kind: "tonal"
                    small: true
                    enabled: NetworkLink.usable && root.busyDevice === ""
                    text: card.linked ? I18n.tr("Disconnect") : I18n.tr("Connect")
                    accessibleName: (card.linked ? I18n.tr("Disconnect %1") : I18n.tr("Connect %1")).arg(card.dev.device)
                    onClicked: card.linked ? root.disconnectDevice(card.dev.device)
                                           : root.connectDevice(card.dev.device, card.profile ? card.profile.uuid : "")
                }
            }

            LongWait {
                visible: cardPace.lagging
                cancellable: false
                text: I18n.tr("Still waiting for %1…").arg(card.dev.device)
            }

            NetRow {
                host: page
                rowId: card.first && card.profiles.length > 1 ? "ethernet-profile" : ""
                visible: card.profiles.length > 1
                title: I18n.tr("Profile")
                detail: card.linked ? I18n.tr("Picking another switches the adapter to it")
                                    : I18n.tr("The saved profile Connect uses")

                Segmented {
                    id: profileSeg
                    anchors.verticalCenter: parent.verticalCenter
                    enabled: NetworkLink.usable && root.busyDevice === ""
                    model: card.profiles.map(p => p.name)
                    accessibleName: I18n.tr("Profile for %1").arg(card.dev.device)
                    onActivated: index => root.pick(card.dev, card.profiles[index].uuid)
                    Binding {
                        target: profileSeg
                        property: "currentIndex"
                        value: Math.max(0, card.profiles.findIndex(p => card.profile && p.uuid === card.profile.uuid))
                    }
                }
            }

            NetRow {
                host: page
                rowId: card.first && card.profile ? "ethernet-auto" : ""
                visible: card.profile !== null
                title: I18n.tr("Auto-connect")
                detail: I18n.tr("Bring %1 up whenever a cable is plugged in").arg(card.profile ? card.profile.name : "")
                activatable: NetworkLink.usable
                onActivated: root.setAutoconnect(card.dev.device, card.profile.uuid, !(card.profile.autoconnect === true))

                Toggle {
                    id: autoToggle
                    anchors.verticalCenter: parent.verticalCenter
                    enabled: NetworkLink.usable
                    checked: card.profile !== null && card.profile.autoconnect === true
                    Accessible.name: I18n.tr("Auto-connect %1").arg(card.dev.device)
                    onToggled: {
                        var want = autoToggle.checked;
                        autoToggle.checked = Qt.binding(() => card.profile !== null && card.profile.autoconnect === true);
                        root.setAutoconnect(card.dev.device, card.profile.uuid, want);
                    }
                }
            }

            NetRow {
                host: page
                rowId: card.first && card.profile ? "ethernet-ip" : ""
                visible: card.profile !== null
                title: I18n.tr("IP settings")
                detail: I18n.tr("Automatic (DHCP) or a fixed address, gateway and DNS for %1").arg(card.profile ? card.profile.name : "")

                Plaque {
                    anchors.verticalCenter: parent.verticalCenter
                    kind: card.ipShown ? "line" : "tonal"
                    small: true
                    glyph: card.ipShown ? "expand_less" : "expand_more"
                    text: card.ipShown ? I18n.tr("Close") : I18n.tr("Change")
                    accessibleName: I18n.tr("IP settings of %1").arg(card.dev.device)
                    onClicked: root.toggleIp(card.dev.device)
                }
            }

            Loader {
                active: card.ipShown && card.profile !== null
                visible: active
                width: parent.width
                sourceComponent: IpSettings {
                    uuid: card.profile ? card.profile.uuid : ""
                }
            }
        }
    }
}

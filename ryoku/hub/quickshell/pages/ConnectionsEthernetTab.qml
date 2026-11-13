pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"

// Connections > Ethernet: one card per wired adapter, from the shell daemon's
// `network` topic. Each shows the link (profile, speed, address), connects or
// disconnects it, and for the profile it uses: auto-connect and the same IP
// settings box a saved Wi-Fi network has, always shown here. An adapter with
// no saved profile connects with the DHCP profile NetworkManager makes for it.
Item {
    id: tab

    property string busyDevice: ""
    property string error: ""
    // a profile picked for an adapter that is down, by adapter name
    property var chosen: ({})

    // profiles whose IP settings box has started on this visit; the first
    // start drops edits left from an earlier visit (a running revert is kept),
    // a rebuild of the card after that picks them up
    property var ipStarted: []

    function done(ok, error) {
        tab.busyDevice = "";
        if (!ok)
            tab.error = error || I18n.tr("The Ethernet change failed.");
    }
    function connectDevice(device, uuid) {
        tab.busyDevice = device;
        tab.error = "";
        NetworkTopic.ethernetConnect(device, uuid, tab.done);
    }
    function disconnectDevice(device) {
        tab.busyDevice = device;
        tab.error = "";
        NetworkTopic.ethernetDisconnect(device, tab.done);
    }
    function setAutoconnect(uuid, on) {
        tab.error = "";
        NetworkTopic.ethernetSetAutoconnect(uuid, on, function(ok, error) {
            if (!ok)
                tab.error = error || I18n.tr("Could not change auto-connect.");
        });
    }
    // the saved profiles an adapter can use: those bound to it and the unbound ones
    function profilesFor(device) {
        return NetworkTopic.ethernetProfiles.filter(function(p) {
            return !p.device || p.device === device;
        });
    }
    // the profile the adapter's controls act on: the one in use, else the one
    // picked, else the first it can use
    function profileOf(dev) {
        var list = tab.profilesFor(dev.device);
        var want = dev.activeUuid || tab.chosen[dev.device] || "";
        for (var i = 0; i < list.length; ++i) {
            if (list[i].uuid === want)
                return list[i];
        }
        return list.length > 0 ? list[0] : null;
    }
    function pick(dev, uuid) {
        if (dev.state === "connected" || dev.state === "connecting") {
            if (uuid !== dev.activeUuid)
                tab.connectDevice(dev.device, uuid);
            return;
        }
        var next = Object.assign({}, tab.chosen);
        next[dev.device] = uuid;
        tab.chosen = next;
    }
    function speedText(mbps) {
        if (!mbps)
            return "";
        return mbps >= 1000 ? (mbps / 1000) + " Gb/s" : mbps + " Mb/s";
    }
    function statusText(dev) {
        switch (dev.state) {
        case "connected":
            return [dev.activeName, tab.speedText(dev.speed), dev.ip4].filter(function(s) { return !!s; }).join(" · ");
        case "connecting":
            return I18n.tr("Connecting...");
        case "unplugged":
            return I18n.tr("No cable");
        case "unmanaged":
            return I18n.tr("Not managed by NetworkManager");
        default:
            return I18n.tr("Cable in, not connected");
        }
    }

    Flickable {
        id: flick
        anchors.fill: parent
        clip: true
        contentWidth: width
        contentHeight: column.implicitHeight + Tokens.s4
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        Column {
            id: column
            width: flick.width - Tokens.s4
            spacing: Tokens.s4

            Item {
                visible: tab.error !== ""
                width: parent.width
                implicitHeight: visible ? errorText.implicitHeight + Tokens.s2 : 0
                Text {
                    id: errorText
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.leftMargin: Tokens.s4
                    text: tab.error
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.WordWrap
                }
            }

            SettingCard {
                visible: NetworkTopic.ready && NetworkTopic.ethernet.length === 0
                width: parent.width
                title: I18n.tr("ETHERNET")
                collapsible: false
                SettingRow {
                    width: parent.width
                    label: I18n.tr("No wired adapter")
                    desc: I18n.tr("Plug in a cable or a USB Ethernet adapter and it shows here.")
                }
            }

            Repeater {
                model: NetworkTopic.ethernet
                delegate: SettingCard {
                    id: card
                    required property var modelData
                    readonly property var dev: card.modelData
                    readonly property var profiles: tab.profilesFor(card.dev.device)
                    readonly property var profile: tab.profileOf(card.dev)
                    readonly property bool linked: card.dev.state === "connected" || card.dev.state === "connecting"
                    readonly property bool usable: card.dev.state !== "unplugged" && card.dev.state !== "unmanaged"

                    width: column.width
                    title: card.dev.device
                    collapsible: false
                    enabled: tab.busyDevice === "" || tab.busyDevice === card.dev.device

                    SettingRow {
                        width: parent.width
                        label: I18n.tr("Connection")
                        desc: tab.statusText(card.dev)
                        controlWidth: linkBtn.implicitWidth
                        Btn {
                            id: linkBtn
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            visible: card.usable
                            primary: !card.linked
                            armed: NetworkTopic.ready && tab.busyDevice === ""
                            text: tab.busyDevice === card.dev.device
                                ? I18n.tr("Working...")
                                : (card.linked ? I18n.tr("Disconnect") : I18n.tr("Connect"))
                            onAct: card.linked
                                ? tab.disconnectDevice(card.dev.device)
                                : tab.connectDevice(card.dev.device, card.profile ? card.profile.uuid : "")
                        }
                    }

                    SettingRow {
                        width: parent.width
                        visible: card.profiles.length > 1
                        divider: true
                        label: I18n.tr("Profile")
                        desc: card.linked
                            ? I18n.tr("Picking another switches the adapter to it.")
                            : I18n.tr("The saved profile Connect uses.")
                        block: true
                        Seg {
                            options: card.profiles.map(function(p) { return p.uuid; })
                            labels: {
                                var m = {};
                                card.profiles.forEach(function(p) { m[p.uuid] = p.name; });
                                return m;
                            }
                            current: card.profile ? card.profile.uuid : ""
                            onChose: function(key) { tab.pick(card.dev, key); }
                        }
                    }

                    SettingRow {
                        width: parent.width
                        visible: card.profile !== null
                        divider: true
                        label: I18n.tr("Auto-connect")
                        desc: I18n.tr("Bring %1 up whenever a cable is plugged in.").arg(card.profile ? card.profile.name : "")
                        controlWidth: autoSw.implicitWidth
                        Sw {
                            id: autoSw
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            on: card.profile !== null && card.profile.autoconnect === true
                            onToggled: function(next) { tab.setAutoconnect(card.profile.uuid, next); }
                        }
                    }

                    SettingRow {
                        width: parent.width
                        visible: card.profile !== null
                        divider: true
                        label: I18n.tr("IP settings")
                        desc: I18n.tr("Automatic (DHCP) or a fixed address for %1.").arg(card.profile ? card.profile.name : "")
                        block: true
                        ConnectionsIpEditor {
                            width: parent.width
                            framed: false
                            uuid: card.profile ? card.profile.uuid : ""
                            fresh: tab.ipStarted.indexOf(uuid) < 0
                            onStarted: if (tab.ipStarted.indexOf(uuid) < 0) tab.ipStarted = tab.ipStarted.concat([uuid])
                        }
                    }
                }
            }
        }
    }
}

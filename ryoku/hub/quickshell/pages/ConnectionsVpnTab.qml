pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"

// Connections > VPN: the saved WireGuard tunnels and NetworkManager VPN
// profiles, switched through the shell daemon's `network.*` calls (the same
// ones the bar's VPN menus make). WireGuard tunnels import from a .conf and
// can be removed here; other VPN types are added with their own plugin.
Item {
    id: tab

    property string busyUuid: ""
    property string error: ""
    // the tunnel whose Remove was tapped once; a second tap removes it
    property string removeArmed: ""
    // the tunnel or profile whose settings are open under its row
    property string openUuid: ""

    function done(ok, error) {
        tab.busyUuid = "";
        if (!ok)
            tab.error = error || I18n.tr("The VPN change failed.");
    }
    function setWireguard(uuid, on) {
        tab.busyUuid = uuid;
        tab.error = "";
        NetworkTopic.wireguardSet(uuid, on, tab.done);
    }
    function setVpn(uuid, on) {
        tab.busyUuid = uuid;
        tab.error = "";
        NetworkTopic.vpnSet(uuid, on, tab.done);
    }
    function remove(uuid) {
        if (tab.removeArmed !== uuid) {
            tab.removeArmed = uuid;
            disarm.restart();
            return;
        }
        disarm.stop();
        tab.removeArmed = "";
        tab.busyUuid = uuid;
        tab.error = "";
        NetworkTopic.wireguardDelete(uuid, tab.done);
    }

    Timer {
        id: disarm
        interval: 3000
        onTriggered: tab.removeArmed = ""
    }

    FileDialogHost {
        id: picker
        title: I18n.tr("Import a WireGuard tunnel")
        nameFilters: [I18n.tr("WireGuard configuration") + " (*.conf)", I18n.tr("All files") + " (*)"]
        onAccepted: {
            var path = decodeURIComponent(("" + picker.selectedFile).replace(/^file:\/\//, ""));
            tab.busyUuid = "import";
            tab.error = "";
            NetworkTopic.wireguardImport(path, tab.done);
        }
    }

    Flickable {
        id: flick
        anchors.fill: parent
        clip: true
        contentWidth: width
        contentHeight: content.implicitHeight + Tokens.s4
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        Column {
            id: content
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

            CardColumns {
                id: cards
                width: parent.width
                minColumnWidth: 400

                SettingCard {
                    width: cards.colWidth
                    title: I18n.tr("WIREGUARD")
                    collapsible: false

                    SettingRow {
                        width: parent.width
                        label: I18n.tr("Add a tunnel")
                        desc: I18n.tr("Import the .conf file your VPN provider gives you.")
                        controlWidth: importBtn.implicitWidth
                        enabled: NetworkTopic.ready && tab.busyUuid === ""
                        Btn {
                            id: importBtn
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            text: tab.busyUuid === "import" ? I18n.tr("Importing...") : I18n.tr("Import .conf")
                            onAct: picker.open()
                        }
                    }

                    Repeater {
                        model: NetworkTopic.wireguard
                        delegate: Column {
                            id: wgItem
                            required property var modelData
                            readonly property bool open: tab.openUuid === wgItem.modelData.uuid
                            width: parent ? parent.width : 0

                            SettingRow {
                                id: wgRow
                                width: wgItem.width
                                label: wgItem.modelData.name || ""
                                desc: (wgItem.modelData.active ? I18n.tr("Connected") : I18n.tr("Off"))
                                    + (wgItem.modelData.autoconnect ? " · " + I18n.tr("auto-connect") : "")
                                controlWidth: wgControls.implicitWidth
                                enabled: tab.busyUuid === "" || tab.busyUuid === wgItem.modelData.uuid
                                Row {
                                    id: wgControls
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: Tokens.s2
                                    Btn {
                                        anchors.verticalCenter: parent.verticalCenter
                                        compact: true
                                        primary: wgItem.open
                                        text: I18n.tr("Settings") + (wgItem.open ? " \u25b4" : " \u25be")
                                        onAct: tab.openUuid = wgItem.open ? "" : wgItem.modelData.uuid
                                    }
                                    Btn {
                                        anchors.verticalCenter: parent.verticalCenter
                                        compact: true
                                        primary: tab.removeArmed === wgItem.modelData.uuid
                                        armed: tab.busyUuid === ""
                                        text: tab.removeArmed === wgItem.modelData.uuid ? I18n.tr("Confirm · Remove") : I18n.tr("Remove")
                                        onAct: tab.remove(wgItem.modelData.uuid)
                                    }
                                    Sw {
                                        anchors.verticalCenter: parent.verticalCenter
                                        on: wgItem.modelData.active === true
                                        onToggled: function(next) { tab.setWireguard(wgItem.modelData.uuid, next); }
                                    }
                                }
                            }

                            Loader {
                                x: Tokens.s4
                                width: wgItem.width - Tokens.s4 * 2
                                active: wgItem.open
                                visible: active
                                sourceComponent: ConnectionsVpnDetails {
                                    uuid: wgItem.modelData.uuid
                                    onReplaced: function(fresh) { tab.openUuid = fresh; }
                                }
                            }
                            Item {
                                visible: wgItem.open
                                width: 1
                                height: Tokens.s3
                            }
                        }
                    }

                    SettingRow {
                        width: parent.width
                        visible: NetworkTopic.ready && NetworkTopic.wireguard.length === 0
                        label: I18n.tr("No tunnels yet")
                        desc: I18n.tr("Imported tunnels show here and in the bar's VPN menu.")
                    }
                }

                SettingCard {
                    width: cards.colWidth
                    title: I18n.tr("VPN")
                    collapsible: false

                    Repeater {
                        model: NetworkTopic.vpn
                        delegate: Column {
                            id: vpnItem
                            required property var modelData
                            readonly property bool open: tab.openUuid === vpnItem.modelData.uuid
                            width: parent ? parent.width : 0

                            SettingRow {
                                id: vpnRow
                                readonly property var modelData: vpnItem.modelData
                                width: vpnItem.width
                                label: vpnRow.modelData.name || ""
                                desc: {
                                    var kind = vpnRow.modelData.service || I18n.tr("VPN");
                                    if (vpnRow.modelData.connecting)
                                        return kind + " · " + I18n.tr("Connecting...");
                                    if (vpnRow.modelData.active)
                                        return kind + " · " + (vpnRow.modelData.ip4 || I18n.tr("Connected"));
                                    return kind + " · " + I18n.tr("Off");
                                }
                                controlWidth: vpnControls.implicitWidth
                                enabled: tab.busyUuid === "" || tab.busyUuid === vpnRow.modelData.uuid
                                Row {
                                    id: vpnControls
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: Tokens.s2
                                    Btn {
                                        anchors.verticalCenter: parent.verticalCenter
                                        compact: true
                                        primary: vpnItem.open
                                        text: I18n.tr("Settings") + (vpnItem.open ? " \u25b4" : " \u25be")
                                        onAct: tab.openUuid = vpnItem.open ? "" : vpnItem.modelData.uuid
                                    }
                                    Sw {
                                        anchors.verticalCenter: parent.verticalCenter
                                        on: vpnRow.modelData.active === true
                                        onToggled: function(next) { tab.setVpn(vpnRow.modelData.uuid, next); }
                                    }
                                }
                            }

                            Loader {
                                x: Tokens.s4
                                width: vpnItem.width - Tokens.s4 * 2
                                active: vpnItem.open
                                visible: active
                                sourceComponent: ConnectionsVpnDetails {
                                    uuid: vpnItem.modelData.uuid
                                }
                            }
                            Item {
                                visible: vpnItem.open
                                width: 1
                                height: Tokens.s3
                            }
                        }
                    }

                    SettingRow {
                        width: parent.width
                        visible: NetworkTopic.ready && NetworkTopic.vpn.length === 0
                        label: I18n.tr("No VPN profiles")
                        desc: I18n.tr("OpenVPN and other VPN profiles added through NetworkManager show here.")
                    }
                }
            }
        }
    }
}

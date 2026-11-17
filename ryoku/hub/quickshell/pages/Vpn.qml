import QtQuick
import Lithic
import Ryoku.Ui as RU
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/network"
import "../components/wifi"
import "../schema/vpn.js" as Schema

// the saved WireGuard tunnels and the VPN profiles NetworkManager holds, switched through the shell
// daemon (the same calls the bar's VPN menus make). A tunnel imports from a .conf and opens its
// settings under its row; other VPN types are added with their own plugin
Item {
    id: root

    required property var hub
    property string pendingRow: ""

    // the connection whose switch is running, "import" while a .conf loads
    property string busyUuid: ""
    property var errors: ({})
    // the tunnel or profile whose settings are open under its row
    property string openUuid: ""

    // rows by uuid: only a change in which connections exist rebuilds them, so an open panel survives
    // every frame NetworkManager sends
    property var wgIds: []
    property var vpnIds: []
    function syncIds() {
        var wg = NetworkLink.wireguard.map(c => c.uuid);
        if (wg.join("\n") !== root.wgIds.join("\n"))
            root.wgIds = wg;
        var vpn = NetworkLink.vpn.map(c => c.uuid);
        if (vpn.join("\n") !== root.vpnIds.join("\n"))
            root.vpnIds = vpn;
    }
    Component.onCompleted: root.syncIds()
    Connections {
        target: NetworkLink
        function onWireguardChanged() { root.syncIds(); }
        function onVpnChanged() { root.syncIds(); }
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
        var all = NetworkLink.wireguard.concat(NetworkLink.vpn);
        if (all.length === 0)
            return I18n.tr("No tunnels or profiles yet");
        var live = all.filter(c => c.active === true);
        if (live.length === 0)
            return I18n.tr("No VPN is on");
        return live.length === 1 ? I18n.tr("%1 is on").arg(live[0].name) : I18n.tr("%1 connections are on").arg(live.length);
    }

    function wgOf(uuid) {
        return NetworkLink.wireguard.find(c => c.uuid === uuid) || null;
    }

    function vpnOf(uuid) {
        return NetworkLink.vpn.find(c => c.uuid === uuid) || null;
    }

    function setError(key, message) {
        var next = Object.assign({}, root.errors);
        if (message)
            next[key] = message;
        else
            delete next[key];
        root.errors = next;
    }

    function done(key, fallback) {
        return function (ok, error) {
            root.busyUuid = "";
            if (!ok)
                root.setError(key, error || fallback);
        };
    }

    function setWireguard(uuid, on) {
        root.busyUuid = uuid;
        root.setError(uuid, "");
        NetworkLink.wireguardSet(uuid, on, root.done(uuid, I18n.tr("The VPN change failed.")));
    }

    function setVpn(uuid, on) {
        root.busyUuid = uuid;
        root.setError(uuid, "");
        NetworkLink.vpnSet(uuid, on, root.done(uuid, I18n.tr("The VPN change failed.")));
    }

    function toggleOpen(uuid) {
        root.openUuid = root.openUuid === uuid ? "" : uuid;
    }

    function connectionDetail(c, kind) {
        var parts = [kind];
        if (c.connecting)
            parts.push(I18n.tr("Connecting…"));
        else if (c.active)
            parts.push(c.ip4 || I18n.tr("Connected"));
        else
            parts.push(I18n.tr("Off"));
        if (c.autoconnect)
            parts.push(I18n.tr("auto-connect"));
        return parts.filter(p => !!p).join(" \u00b7 ");
    }

    // a landing waits for its row, which needs the connections read
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

    RU.FileDialogHost {
        id: picker
        title: I18n.tr("Import a WireGuard tunnel")
        nameFilters: [I18n.tr("WireGuard configuration") + " (*.conf)", I18n.tr("All files") + " (*)"]
        onAccepted: {
            var path = decodeURIComponent(String(picker.selectedFile).replace(/^file:\/\//, ""));
            root.busyUuid = "import";
            root.setError("import", "");
            NetworkLink.wireguardImport(path, root.done("import", I18n.tr("The tunnel could not be imported.")));
        }
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "vpn"
        schema: root.schema
        stateLine: root.stateLine
        bodies: ({ wireguard: wireguardBody, profiles: profilesBody })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    Component {
        id: wireguardBody

        Column {
            width: parent ? parent.width : 0
            spacing: LithicTokens.space.s4

            LinkNotice {
                host: page
            }

            Slab {
                width: parent.width

                Column {
                    width: parent.width

                    NetRow {
                        id: importRow
                        host: page
                        rowId: "wireguard-import"
                        first: true
                        title: I18n.tr("Add a tunnel")
                        detail: root.busyUuid === "import" ? I18n.tr("Importing…")
                              : I18n.tr("Import the .conf file your VPN provider gives you. It starts switched off.")
                        error: root.errors["import"] || ""
                        onRetry: root.setError("import", "")
                        canRetry: false

                        Pace { id: importPace; running: root.busyUuid === "import" }

                        Working {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: importPace.slow
                            active: visible
                        }
                        Plaque {
                            anchors.verticalCenter: parent.verticalCenter
                            kind: "tonal"
                            small: true
                            glyph: "upload_file"
                            enabled: NetworkLink.usable && root.busyUuid === ""
                            text: I18n.tr("Import .conf")
                            onClicked: picker.open()
                        }
                    }

                    NetRow {
                        host: page
                        rowId: visible ? "wireguard-tunnels" : ""
                        visible: NetworkLink.ready && root.wgIds.length === 0
                        title: I18n.tr("No tunnels yet")
                        detail: I18n.tr("Imported tunnels show here and in the bar's VPN menu")
                    }

                    Repeater {
                        model: root.wgIds
                        delegate: ConnectionItem {
                            wireguard: true
                        }
                    }
                }
            }
        }
    }

    Component {
        id: profilesBody

        Slab {
            visible: NetworkLink.ready
            width: parent ? parent.width : 0

            Column {
                width: parent.width

                NetRow {
                    host: page
                    rowId: visible ? "vpn-profiles" : ""
                    first: true
                    visible: NetworkLink.ready && root.vpnIds.length === 0
                    title: I18n.tr("No VPN profiles")
                    detail: I18n.tr("OpenVPN and other profiles added through NetworkManager show here")
                }

                Repeater {
                    model: root.vpnIds
                    delegate: ConnectionItem {
                        wireguard: false
                    }
                }
            }
        }
    }

    // one tunnel or profile: its row with the settings switch and the on/off switch, and its settings under it
    component ConnectionItem: Column {
        id: entry

        required property string modelData
        required property int index
        property bool wireguard: true
        readonly property var conn: (entry.wireguard ? root.wgOf(entry.modelData) : root.vpnOf(entry.modelData)) || entry.last
        property var last: ({ uuid: entry.modelData, name: "", active: false, connecting: false, autoconnect: false, service: "", ip4: "" })
        onConnChanged: entry.last = entry.conn
        readonly property bool open: root.openUuid === entry.modelData
        readonly property bool mine: root.busyUuid === entry.modelData
        readonly property string error: root.errors[entry.modelData] || ""

        width: parent ? parent.width : 0

        Pace { id: entryPace; running: entry.mine }

        NetRow {
            host: page
            rowId: entry.index === 0 ? (entry.wireguard ? "wireguard-tunnels" : "vpn-profiles") : ""
            first: !entry.wireguard && entry.index === 0
            title: entry.conn.name
            detail: entryPace.slow ? I18n.tr("Working…") : root.connectionDetail(entry.conn, entry.wireguard ? "WireGuard" : (entry.conn.service || I18n.tr("VPN")))
            error: entry.error
            canRetry: false

            Working {
                anchors.verticalCenter: parent.verticalCenter
                visible: entryPace.slow
                active: visible
            }
            Plaque {
                anchors.verticalCenter: parent.verticalCenter
                kind: entry.open ? "line" : "tonal"
                small: true
                glyph: entry.open ? "expand_less" : "tune"
                text: I18n.tr("Settings")
                accessibleName: I18n.tr("Settings of %1").arg(entry.conn.name)
                onClicked: root.toggleOpen(entry.modelData)
            }
            Toggle {
                id: onToggle
                anchors.verticalCenter: parent.verticalCenter
                enabled: NetworkLink.usable && (root.busyUuid === "" || entry.mine)
                checked: entry.conn.active === true
                Accessible.name: I18n.tr("Switch %1 on or off").arg(entry.conn.name)
                onToggled: {
                    var want = onToggle.checked;
                    onToggle.checked = Qt.binding(() => entry.conn.active === true);
                    if (entry.wireguard)
                        root.setWireguard(entry.modelData, want);
                    else
                        root.setVpn(entry.modelData, want);
                }
            }
        }

        LongWait {
            visible: entryPace.lagging
            cancellable: false
            text: I18n.tr("Still waiting for %1. A VPN plugin can take a while.").arg(entry.conn.name)
        }

        Loader {
            active: entry.open
            visible: active
            width: parent.width
            sourceComponent: VpnDetails {
                hub: root.hub
                uuid: entry.modelData
                onReplaced: fresh => root.openUuid = fresh
            }
        }
    }
}

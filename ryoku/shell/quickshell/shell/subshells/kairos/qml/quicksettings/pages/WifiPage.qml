pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons
import Quickshell
import "../../kit"
import "../Segments.js" as Segments

Column {
    id: page

    property var host

    spacing: 8

    property string pendingSsid: ""
    property string pendingBssid: ""
    property int callId: -1
    property string callKey: ""
    property string errorKey: ""
    property string errorText: ""
    property bool scanning: false
    property bool hiddenOpen: false

    readonly property string hiddenKey: "\u0000hidden"
    readonly property bool joined: Network.wifiConnectivity === "Connected"
    readonly property bool joining: Network.wifiConnectivity === "Connecting"
    readonly property var current: {
        var list = Network.accessPoints || [];
        for (var i = 0; i < list.length; i++)
            if (list[i] && list[i].active) return list[i];
        return null;
    }
    // android lists networks, not radios
    readonly property var nets: {
        var list = Network.accessPoints || [];
        var best = ({});
        for (var i = 0; i < list.length; i++) {
            var ap = list[i];
            if (!ap || !ap.ssid || ap.active || ap.ssid === Network.activeSsid) continue;
            var had = best[ap.ssid];
            if (!had || (ap.strength || 0) > (had.strength || 0))
                best[ap.ssid] = Object.assign({}, ap, { saved: ap.saved || (had ? had.saved : false) });
            else if (ap.saved)
                had.saved = true;
        }
        var out = Object.keys(best).map(function (k) { return best[k]; });
        out.sort(function (a, b) { return (b.strength || 0) - (a.strength || 0); });
        return out;
    }
    readonly property var savedNets: page.nets.filter(function (ap) { return ap.saved; })
    readonly property var newNets: page.nets.filter(function (ap) { return !ap.saved; })
    // rows are kept per name across frames, so they read the latest frame from here
    readonly property var byName: {
        var out = ({});
        for (var i = 0; i < page.nets.length; i++)
            out[page.nets[i].ssid] = page.nets[i];
        return out;
    }
    function signalGlyph(strength) {
        var v = Number(strength || 0);
        if (v > 75) return "signal_wifi_4_bar";
        if (v > 50) return "network_wifi_3_bar";
        if (v > 25) return "network_wifi_2_bar";
        if (v > 0) return "network_wifi_1_bar";
        return "signal_wifi_0_bar";
    }
    function isOpen(ap) {
        var s = String(ap && ap.security || "").toLowerCase();
        return s === "" || s === "none";
    }
    function securityName(ap) {
        var s = String(ap && ap.security || "");
        if (s === "Enterprise") return I18n.tr("Enterprise");
        return I18n.tr("Secured with %1").replace("%1", s.toUpperCase());
    }
    function bandName(ap) {
        return ap && ap.band ? I18n.tr("%1 GHz").replace("%1", ap.band) : "";
    }
    function statusOf(ap) {
        if (page.callKey === ap.ssid) return I18n.tr("Connecting\u2026");
        if (page.errorKey === ap.ssid) return page.errorText;
        if (ap.saved) return I18n.tr("Saved");
        if (page.isOpen(ap)) return I18n.tr("Open network");
        return page.securityName(ap);
    }

    function scan() {
        if (!Network.wifiRadio) return;
        page.scanning = true;
        Network.refresh();
        scanDone.restart();
    }
    function begin(key, ssid, password, bssid, hidden) {
        page.errorKey = "";
        page.errorText = "";
        page.callKey = key;
        page.callId = Network.connectWifi(ssid, password, bssid, hidden);
    }
    // a hidden field keeps nothing focused, so Escape would stop reaching the island
    function settleFocus() {
        if (page.host) page.host.forceActiveFocus();
    }
    function choose(ap) {
        page.hiddenOpen = false;
        if (ap.saved || page.isOpen(ap)) {
            page.pendingSsid = "";
            page.settleFocus();
            page.begin(ap.ssid, ap.ssid, "", ap.bssid || "", false);
            return;
        }
        page.errorKey = page.errorKey === ap.ssid ? page.errorKey : "";
        page.pendingSsid = page.pendingSsid === ap.ssid ? "" : ap.ssid;
        page.pendingBssid = ap.bssid || "";
        if (page.pendingSsid === "") page.settleFocus();
    }
    function submitPassword(password) {
        if (page.pendingSsid === "" || password.length === 0) return;
        var ssid = page.pendingSsid;
        page.settleFocus();
        page.pendingSsid = "";
        page.begin(ssid, ssid, password, page.pendingBssid, false);
    }
    function cancelPassword() {
        page.settleFocus();
        if (page.errorKey === page.pendingSsid) page.errorKey = "";
        page.pendingSsid = "";
        page.pendingBssid = "";
    }
    function openHidden() {
        page.pendingSsid = "";
        page.errorKey = page.errorKey === page.hiddenKey ? "" : page.errorKey;
        page.hiddenOpen = true;
    }
    function cancelHidden() {
        page.settleFocus();
        page.hiddenOpen = false;
        if (page.errorKey === page.hiddenKey) page.errorKey = "";
    }
    function submitHidden(ssid, password) {
        var name = ssid.trim();
        if (name === "" || page.callKey === page.hiddenKey) return;
        page.begin(page.hiddenKey, name, password, "", true);
    }
    // forgetting drops the saved password, so the first press only arms it
    property string forgetArmed: ""
    function forget(ssid) {
        if (page.forgetArmed !== ssid) {
            page.forgetArmed = ssid;
            disarmForget.restart();
            return;
        }
        page.forgetArmed = "";
        if (page.pendingSsid === ssid) page.cancelPassword();
        Network.forgetWifi(ssid);
    }
    Timer {
        id: disarmForget
        interval: 3000
        onTriggered: page.forgetArmed = ""
    }

    Connections {
        target: Network
        function onReplied(id, ok, error) {
            if (id !== page.callId) return;
            var key = page.callKey;
            page.callId = -1;
            page.callKey = "";
            if (ok) {
                if (key === page.hiddenKey) page.cancelHidden();
                return;
            }
            page.errorKey = key;
            page.errorText = error && error.length > 0 ? error : I18n.tr("Could not connect");
            // a refused password is the usual cause, so ask for it again in place
            if (key !== page.hiddenKey) {
                for (var i = 0; i < page.nets.length; i++)
                    if (page.nets[i].ssid === key && !page.isOpen(page.nets[i])) {
                        page.pendingSsid = key;
                        page.pendingBssid = page.nets[i].bssid || "";
                    }
            }
        }
    }
    // the daemon replies once NetworkManager settles; the network going active is the same answer, sooner
    Connections {
        target: Network
        enabled: page.callKey !== "" && page.callKey !== page.hiddenKey
        function onActiveSsidChanged() {
            if (Network.activeSsid === page.callKey && page.joined) {
                page.callId = -1;
                page.callKey = "";
            }
        }
    }
    Timer {
        id: scanDone
        interval: 15000
        onTriggered: page.scanning = false
    }
    Component.onCompleted: if (page.nets.length === 0) page.scan()

    component NetRow: Column {
        id: net
        required property var modelData
        required property int index
        property int count: 1
        readonly property var ap: page.byName[net.modelData.ssid] || net.modelData
        readonly property bool asking: page.pendingSsid === net.ap.ssid
        readonly property string slot: Segments.position(net.index, net.count)
        width: parent ? parent.width : 0
        spacing: Md.seam

        ListItem {
            width: parent.width
            position: !net.asking ? net.slot
                : (net.slot === "only" ? "first" : net.slot === "last" ? "middle" : net.slot)
            icon: page.signalGlyph(net.ap.strength)
            headline: net.ap.ssid
            supporting: page.statusOf(net.ap)
            onClicked: page.choose(net.ap)

            Icon {
                anchors.verticalCenter: parent.verticalCenter
                visible: !page.isOpen(net.ap)
                name: "lock"
                size: 18
                color: Md.inkMuted
            }
            IconButton {
                anchors.verticalCenter: parent.verticalCenter
                visible: net.ap.saved === true
                icon: page.forgetArmed === net.ap.ssid ? "delete_forever" : "delete"
                iconSize: 20
                danger: true
                checked: page.forgetArmed === net.ap.ssid
                tip: checked ? I18n.tr("Press again to forget") : I18n.tr("Forget")
                onClicked: page.forget(net.ap.ssid)
            }
        }

        Segment {
            visible: net.asking
            position: net.slot === "only" || net.slot === "last" ? "last" : "middle"
            height: form.implicitHeight + 28

            onVisibleChanged: if (visible) {
                secret.text = "";
                secret.field.forceActiveFocus();
            }

            Column {
                id: form
                anchors { left: parent.left; right: parent.right; top: parent.top; margins: 14 }
                spacing: 10

                Row {
                    width: parent.width
                    spacing: 8

                    TextField {
                        id: secret
                        width: parent.width - reveal.width - parent.spacing
                        placeholder: I18n.tr("Password")
                        password: !reveal.checked
                        error: page.errorKey === net.ap.ssid
                        onAccepted: page.submitPassword(secret.text)
                        onEscaped: page.cancelPassword()
                    }
                    IconButton {
                        id: reveal
                        anchors.verticalCenter: parent.verticalCenter
                        icon: reveal.checked ? "visibility_off" : "visibility"
                        tip: reveal.checked ? I18n.tr("Hide password") : I18n.tr("Show password")
                        onClicked: reveal.checked = !reveal.checked
                    }
                }
                Label {
                    width: parent.width
                    visible: page.errorKey === net.ap.ssid
                    role: "caption"
                    color: Md.error
                    wrapMode: Text.Wrap
                    text: page.errorText
                }
                Row {
                    anchors.right: parent.right
                    spacing: 8

                    Button {
                        variant: "text"
                        text: I18n.tr("Cancel")
                        onClicked: page.cancelPassword()
                    }
                    Button {
                        variant: "filled"
                        text: I18n.tr("Connect")
                        enabled: secret.text.length > 0
                        onClicked: page.submitPassword(secret.text)
                    }
                }
            }
        }
    }

    TopBar {
        width: parent.width
        title: I18n.tr("Wi-Fi")
        onBack: page.host.back()

        IconButton {
            anchors.verticalCenter: parent.verticalCenter
            visible: Network.wifiPresent && Network.wifiRadio
            icon: "refresh"
            checked: page.scanning
            tip: page.scanning ? I18n.tr("Scanning") : I18n.tr("Scan for networks")
            onClicked: page.scan()
        }
        Switch {
            anchors.verticalCenter: parent.verticalCenter
            visible: Network.wifiPresent
            checked: Network.wifiRadio
            onToggled: v => Network.setWifiEnabled(v)
        }
    }

    EmptyState {
        visible: !Network.wifiPresent
        icon: "wifi_off"
        title: I18n.tr("No Wi-Fi adapter")
        body: I18n.tr("This computer has no Wi-Fi hardware the system can use. A cable or a USB Wi-Fi adapter gets it online.")
    }

    EmptyState {
        visible: Network.wifiPresent && !Network.wifiRadio
        icon: "wifi_off"
        title: I18n.tr("Wi-Fi is off")
        body: I18n.tr("Turn it on to see the networks around you and join one.")
        action: I18n.tr("Turn on Wi-Fi")
        onActed: Network.setWifiEnabled(true)
    }

    Column {
        width: parent.width
        visible: Network.wifiPresent && Network.wifiRadio
        spacing: 8

        ListItem {
            width: parent.width
            visible: Network.activeSsid.length > 0 && (page.joined || page.joining)
            selected: true
            clickable: false
            icon: page.signalGlyph(Network.wifi.strength)
            headline: Network.activeSsid
            supporting: page.joining ? I18n.tr("Connecting\u2026")
                : (page.bandName(page.current).length > 0
                    ? I18n.tr("Connected on %1").replace("%1", page.bandName(page.current))
                    : I18n.tr("Connected"))

            Button {
                anchors.verticalCenter: parent.verticalCenter
                variant: "tonal"
                text: I18n.tr("Disconnect")
                onClicked: Network.disconnectWifi()
            }
            IconButton {
                anchors.verticalCenter: parent.verticalCenter
                visible: !page.current || page.current.saved === true
                icon: page.forgetArmed === Network.activeSsid ? "delete_forever" : "delete"
                iconSize: 20
                danger: true
                checked: page.forgetArmed === Network.activeSsid
                tip: checked ? I18n.tr("Press again to forget") : I18n.tr("Forget")
                onClicked: page.forget(Network.activeSsid)
            }
        }

        SectionLabel {
            visible: page.savedNets.length > 0
            text: I18n.tr("Saved networks")
        }
        Column {
            width: parent.width
            visible: page.savedNets.length > 0
            spacing: Md.seam

            Repeater {
                model: ScriptModel { values: page.savedNets; objectProp: "ssid" }
                delegate: NetRow { count: page.savedNets.length }
            }
        }

        SectionLabel {
            visible: page.newNets.length > 0
            text: I18n.tr("Available networks")
        }
        Column {
            width: parent.width
            visible: page.newNets.length > 0
            spacing: Md.seam

            Repeater {
                model: ScriptModel { values: page.newNets; objectProp: "ssid" }
                delegate: NetRow { count: page.newNets.length }
            }
        }

        EmptyState {
            visible: page.nets.length === 0
            icon: page.scanning ? "wifi_find" : "wifi_off"
            title: page.scanning ? I18n.tr("Looking for networks\u2026") : I18n.tr("No networks found")
            body: page.scanning ? I18n.tr("Networks in range appear here in a moment.")
                : I18n.tr("Scan again, or move closer to the router.")
            action: page.scanning ? "" : I18n.tr("Scan again")
            onActed: page.scan()
        }

        Column {
            width: parent.width
            topPadding: 8
            spacing: Md.seam

            ListItem {
                width: parent.width
                position: page.hiddenOpen ? "first" : "only"
                icon: "add"
                headline: I18n.tr("Join a hidden network")
                supporting: page.callKey === page.hiddenKey ? I18n.tr("Connecting\u2026") : ""
                onClicked: page.hiddenOpen ? page.cancelHidden() : page.openHidden()
            }

            Segment {
                visible: page.hiddenOpen
                position: "last"
                height: hiddenForm.implicitHeight + 28

                onVisibleChanged: if (visible) {
                    hiddenName.text = "";
                    hiddenSecret.text = "";
                    hiddenName.field.forceActiveFocus();
                }

                Column {
                    id: hiddenForm
                    anchors { left: parent.left; right: parent.right; top: parent.top; margins: 14 }
                    spacing: 10

                    TextField {
                        id: hiddenName
                        width: parent.width
                        placeholder: I18n.tr("Network name")
                        onAccepted: hiddenSecret.field.forceActiveFocus()
                        onEscaped: page.cancelHidden()
                    }
                    TextField {
                        id: hiddenSecret
                        width: parent.width
                        placeholder: I18n.tr("Password, if it has one")
                        password: true
                        error: page.errorKey === page.hiddenKey
                        onAccepted: page.submitHidden(hiddenName.text, hiddenSecret.text)
                        onEscaped: page.cancelHidden()
                    }
                    Label {
                        width: parent.width
                        visible: page.errorKey === page.hiddenKey
                        role: "caption"
                        color: Md.error
                        wrapMode: Text.Wrap
                        text: page.errorText
                    }
                    Row {
                        anchors.right: parent.right
                        spacing: 8

                        Button {
                            variant: "text"
                            text: I18n.tr("Cancel")
                            onClicked: page.cancelHidden()
                        }
                        Button {
                            variant: "filled"
                            text: page.callKey === page.hiddenKey ? I18n.tr("Connecting\u2026") : I18n.tr("Connect")
                            enabled: hiddenName.text.trim().length > 0 && page.callKey !== page.hiddenKey
                            onClicked: page.submitHidden(hiddenName.text, hiddenSecret.text)
                        }
                    }
                }
            }
        }
    }

    Item { width: 1; height: 4 }
}

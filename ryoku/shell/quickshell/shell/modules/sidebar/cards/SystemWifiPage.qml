pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Lithic
import shell.services
import Ryoku.Ui.Singletons
import "../../ask/look"
import ".."

Item {
    id: root

    required property real s
    required property bool active
    property Item card: null
    signal backRequested()

    property bool scanning: false
    property var pendingAp: null
    property var retryAp: null
    property bool hiddenForm: false
    property string hiddenSsid: ""
    property string password: ""
    property int pendingCallId: -1
    property string pendingKey: ""
    property string errorText: ""

    readonly property bool busy: root.pendingCallId >= 0
    readonly property bool joined: Network.activeSsid !== ""
    readonly property bool radioOn: Network.wifiPresent && Network.wifiRadio
    readonly property var activeAp: {
        const aps = Network.accessPoints || [];
        for (let i = 0; i < aps.length; i++)
            if (aps[i] && aps[i].active)
                return aps[i];
        return null;
    }
    readonly property string joinedProof: I18n.tr("%1% signal").arg(Network.wifi.strength || 0)
        + (root.activeAp && root.activeAp.band ? " · " + I18n.tr("%1 GHz").arg(root.activeAp.band) : "")
    readonly property var multiBandSsids: {
        const bands = ({});
        const aps = Network.accessPoints || [];
        for (let i = 0; i < aps.length; i++) {
            const ap = aps[i];
            if (!ap || !ap.ssid || !ap.band)
                continue;
            if (!bands[ap.ssid])
                bands[ap.ssid] = ({});
            bands[ap.ssid][ap.band] = true;
        }
        const result = ({});
        for (const ssid in bands)
            if (Object.keys(bands[ssid]).length > 1)
                result[ssid] = true;
        return result;
    }
    readonly property var availableNets: {
        const aps = Network.accessPoints || [];
        const activeBand = root.activeAp ? root.activeAp.band || "" : "";
        const best = ({});
        for (let i = 0; i < aps.length; i++) {
            const ap = aps[i];
            if (!ap || !ap.ssid || ap.active)
                continue;
            if (ap.ssid === Network.activeSsid && (activeBand === "" || (ap.band || "") === activeBand))
                continue;
            const key = ap.ssid + "\u0000" + (ap.band || "");
            if (!best[key] || (ap.strength || 0) > (best[key].strength || 0))
                best[key] = ap;
        }
        const out = Object.keys(best).map(key => best[key]);
        out.sort((a, b) => {
            if (!!a.saved !== !!b.saved)
                return a.saved ? -1 : 1;
            return (b.strength || 0) - (a.strength || 0);
        });
        return out;
    }

    implicitHeight: content.implicitHeight
    height: implicitHeight

    function isOpen(ap): bool {
        const security = String(ap && ap.security ? ap.security : "").toLowerCase();
        return security === "" || security === "none";
    }

    function signalGlyph(strength): string {
        const value = Number(strength || 0);
        if (value > 75) return "signal_wifi_4_bar";
        if (value > 50) return "network_wifi_3_bar";
        if (value > 25) return "network_wifi_2_bar";
        if (value > 0) return "network_wifi_1_bar";
        return "signal_wifi_0_bar";
    }

    function networkKey(ap): string {
        return ap ? String(ap.ssid || "") + "\u0000" + String(ap.band || "") : "";
    }

    function networkMeta(ap): string {
        const parts = [root.isOpen(ap) ? I18n.tr("Open") : I18n.tr("Secure")];
        if (ap.saved)
            parts.push(I18n.tr("Saved"));
        if (root.multiBandSsids[ap.ssid] && ap.band)
            parts.push(I18n.tr("%1 GHz").arg(ap.band));
        parts.push(I18n.tr("%1%").arg(ap.strength || 0));
        return parts.join(" · ");
    }

    function beginConnect(ssid, password, bssid, hidden, key): void {
        if (root.busy || !ssid)
            return;
        root.errorText = "";
        root.pendingKey = key || ssid;
        root.pendingCallId = Network.connectWifi(ssid, password || "", bssid || "", hidden === true);
    }

    function choose(ap): void {
        if (!ap || root.busy)
            return;
        root.hiddenForm = false;
        root.hiddenSsid = "";
        root.password = "";
        root.errorText = "";
        if (ap.saved || root.isOpen(ap)) {
            root.pendingAp = null;
            root.retryAp = ap;
            root.beginConnect(ap.ssid, "", ap.bssid || "", false, root.networkKey(ap));
            return;
        }
        root.pendingAp = ap;
        Qt.callLater(() => passwordField.forceActiveFocus());
    }

    function openHidden(): void {
        root.pendingAp = null;
        root.hiddenForm = true;
        root.hiddenSsid = "";
        root.password = "";
        root.errorText = "";
        Qt.callLater(() => hiddenSsidField.forceActiveFocus());
    }

    function submitPassword(): void {
        if (!root.pendingAp || root.password.length === 0)
            return;
        root.retryAp = root.pendingAp;
        root.beginConnect(root.pendingAp.ssid, root.password, root.pendingAp.bssid || "", false,
            root.networkKey(root.pendingAp));
    }

    function submitHidden(): void {
        const ssid = root.hiddenSsid.trim();
        if (!ssid)
            return;
        root.retryAp = null;
        root.beginConnect(ssid, root.password, "", true, "hidden:" + ssid);
    }

    function clearForm(): void {
        root.pendingAp = null;
        root.retryAp = null;
        root.hiddenForm = false;
        root.hiddenSsid = "";
        root.password = "";
    }

    function refresh(): void {
        if (!root.active || !Network.wifiPresent || !Network.wifiRadio)
            return;
        root.scanning = true;
        Network.refresh();
        scanClear.restart();
    }

    onActiveChanged: {
        if (root.active)
            root.refresh();
        else {
            root.scanning = false;
            scanClear.stop();
        }
    }
    onAvailableNetsChanged: if (root.availableNets.length > 0) root.scanning = false
    Component.onCompleted: if (root.active) root.refresh()

    Timer {
        id: scanClear
        interval: 15000
        onTriggered: root.scanning = false
    }

    Connections {
        target: Network
        function onReplied(id, ok, error) {
            if (id !== root.pendingCallId)
                return;
            root.pendingCallId = -1;
            root.pendingKey = "";
            if (ok) {
                root.errorText = "";
                root.clearForm();
            } else {
                root.errorText = error && error.length > 0 ? error : I18n.tr("Could not connect to this network");
                root.password = "";
                if (root.retryAp && !root.isOpen(root.retryAp))
                    root.pendingAp = root.retryAp;
                Qt.callLater(() => {
                    if (root.pendingAp)
                        passwordField.forceActiveFocus();
                    else if (root.hiddenForm)
                        hiddenPasswordField.forceActiveFocus();
                });
            }
        }
    }

    Column {
        id: content
        width: root.width
        spacing: Math.round(QsLook.gap * root.s)

        RadioRow {
            width: parent.width
            s: root.s
            text: I18n.tr("Wi-Fi")
            detail: !Network.wifiPresent ? I18n.tr("No Wi-Fi adapter")
                : Network.wifiRadio ? I18n.tr("On") : I18n.tr("Off")
            glyph: Network.wifiRadio ? "wifi" : "wifi_off"
            on: Network.wifiRadio
            available: Network.wifiPresent
            searching: root.scanning && root.active
            canSearch: root.radioOn
            onToggleRequested: value => Network.setWifiEnabled(value)
            onSearchRequested: root.refresh()
        }

        Plate {
            visible: QsLook.lithic && root.joined
            width: parent.width
            height: joinedCopy.implicitHeight + Math.round(24 * root.s)
            s: root.s

            Symbol {
                id: joinedGlyph
                anchors.left: parent.left
                anchors.leftMargin: Math.round(12 * root.s)
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                name: root.signalGlyph(Network.wifi.strength)
                size: 22
                ink: Lithic.sel
            }
            Column {
                id: joinedCopy
                anchors.left: joinedGlyph.right
                anchors.leftMargin: Math.round(12 * root.s)
                anchors.right: lithicDisconnect.left
                anchors.rightMargin: Math.round(10 * root.s)
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(3 * root.s)

                Row {
                    spacing: Math.round(6 * root.s)
                    Flake {
                        anchors.verticalCenter: parent.verticalCenter
                        tone: "alive"
                        fill: Lithic.jade
                        size: Math.round(7 * root.s)
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Connected")
                        color: Lithic.fg2
                        font.family: QsLook.mono
                        font.pixelSize: Math.round(9 * root.s)
                        font.capitalization: Font.AllUppercase
                        font.letterSpacing: Look.tracking * root.s
                        renderType: Text.NativeRendering
                    }
                }
                Text {
                    width: parent.width
                    text: Network.activeSsid
                    textFormat: Text.PlainText
                    color: Lithic.fg
                    font.family: QsLook.sans
                    font.pixelSize: Math.round(16 * root.s)
                    font.weight: Font.Medium
                    elide: Text.ElideRight
                    renderType: Text.NativeRendering
                }
                Text {
                    width: parent.width
                    text: root.joinedProof
                    color: Lithic.fg2
                    font.family: QsLook.mono
                    font.pixelSize: Math.round(10 * root.s)
                    font.features: ({ "tnum": 1 })
                    elide: Text.ElideRight
                    renderType: Text.NativeRendering
                }
            }
            SidebarButton {
                id: lithicDisconnect
                anchors.right: parent.right
                anchors.rightMargin: Math.round(12 * root.s)
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                compact: true
                text: I18n.tr("Disconnect")
                onAct: Network.disconnectWifi()
            }
        }

        PageRow {
            visible: !QsLook.lithic && root.joined
            width: parent.width
            s: root.s
            selected: true
            checked: true
            glyph: root.signalGlyph(Network.wifi.strength)
            title: Network.activeSsid
            meta: I18n.tr("Connected") + " · " + root.joinedProof

            SidebarButton {
                s: root.s
                compact: true
                text: I18n.tr("Disconnect")
                onAct: Network.disconnectWifi()
            }
        }

        Text {
            visible: !root.joined
            width: parent.width
            text: !Network.wifiPresent ? I18n.tr("This machine has no Wi-Fi adapter")
                : !Network.wifiRadio ? I18n.tr("Wi-Fi is off")
                : Network.wifiConnectivity === "Connecting" ? I18n.tr("Connecting…")
                : I18n.tr("Not connected")
            color: Lithic.fg2
            font.family: QsLook.sans
            font.pixelSize: Math.round(13 * root.s)
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }

        Text {
            visible: root.errorText !== "" && root.pendingAp === null && !root.hiddenForm
            width: parent.width
            text: root.errorText
            textFormat: Text.PlainText
            color: Lithic.jasper
            font.family: QsLook.sans
            font.pixelSize: Math.round(12 * root.s)
            font.weight: Font.Medium
            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
            renderType: Text.NativeRendering
        }

        Column {
            visible: root.radioOn
            width: parent.width
            spacing: Math.round(8 * root.s)

            Micro {
                width: parent.width
                s: root.s
                text: I18n.tr("Available networks")
            }

            Column {
                id: networkList
                width: parent.width
                spacing: QsLook.material || QsLook.fluent ? Math.round(2 * root.s) : 0

                Repeater {
                    model: root.availableNets
                    delegate: PageRow {
                        id: apRow
                        required property var modelData
                        required property int index
                        readonly property bool connecting: root.busy && root.pendingKey === root.networkKey(apRow.modelData)
                        readonly property bool picked: root.pendingAp !== null
                            && root.networkKey(root.pendingAp) === root.networkKey(apRow.modelData)
                        width: networkList.width
                        s: root.s
                        first: apRow.index === 0
                        last: apRow.index === root.availableNets.length - 1
                        selected: apRow.picked
                        glyph: root.signalGlyph(apRow.modelData.strength)
                        title: apRow.modelData.ssid || ""
                        meta: root.networkMeta(apRow.modelData)

                        SidebarButton {
                            s: root.s
                            compact: true
                            text: apRow.connecting ? I18n.tr("Connecting…") : I18n.tr("Connect")
                            primary: apRow.picked
                            armed: !root.busy
                            Accessible.name: I18n.tr("Connect to %1").arg(apRow.modelData.ssid || "")
                            onAct: root.choose(apRow.modelData)
                        }
                    }
                }
            }

            PageSearching {
                visible: root.availableNets.length === 0
                width: parent.width
                s: root.s
                running: root.scanning && root.active
                text: root.scanning ? I18n.tr("Searching for nearby networks…")
                    : I18n.tr("No networks found. Move closer to an access point, then search again.")
            }
        }

        Flow {
            visible: root.radioOn && !root.hiddenForm && root.pendingAp === null
            width: parent.width
            spacing: Math.round(8 * root.s)

            SidebarButton {
                s: root.s
                glyph: "add"
                text: I18n.tr("Hidden network")
                onAct: root.openHidden()
            }
            SidebarButton {
                s: root.s
                visible: root.joined
                glyph: "delete"
                text: I18n.tr("Forget %1").arg(Network.activeSsid)
                onAct: Network.forgetWifi(Network.activeSsid)
            }
        }
    }

    QQC.Popup {
        id: joinPopup
        parent: root.card ?? QQC.Overlay.overlay
        visible: root.active && (root.pendingAp !== null || root.hiddenForm)
        width: Math.min(Math.round(400 * root.s), parent ? parent.width - Math.round(32 * root.s) : root.width)
        implicitHeight: joinForm.implicitHeight + padding * 2
        x: parent ? (parent.width - width) / 2 : 0
        y: parent ? Math.max(Math.round(16 * root.s), (parent.height - height) / 2) : 0
        padding: Math.round((QsLook.material ? 24 : 18) * root.s)
        modal: true
        // the overlay spans the panel's whole column, so its dim would shade past the card
        dim: false
        focus: true
        closePolicy: root.busy ? QQC.Popup.NoAutoClose : QQC.Popup.CloseOnEscape | QQC.Popup.CloseOnPressOutside
        onOpened: Qt.callLater(() => root.hiddenForm ? hiddenSsidField.forceActiveFocus() : passwordField.forceActiveFocus())
        onClosed: if (!root.busy) root.clearForm()

        enter: Transition {
            NumberAnimation {
                property: "opacity"
                from: 0
                to: 1
                duration: QsLook.dur(Look.viewMs)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: QsLook.arrive
            }
            NumberAnimation {
                property: "scale"
                from: QsLook.lithic ? 1 : QsLook.material ? 0.92 : 0.96
                to: 1
                duration: QsLook.dur(Look.viewMs)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: QsLook.arrive
            }
        }
        exit: Transition {
            NumberAnimation {
                property: "opacity"
                to: 0
                duration: QsLook.dur(QsLook.closeMs)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: QsLook.leave
            }
        }

        background: CutRect {
            cut: Look.cutFrame * root.s
            fill: Look.popover
            strokeColor: Look.popoverStroke
            strokeWidth: Qt.colorEqual(Look.popoverStroke, "transparent") ? 0 : Math.max(1, Math.round(root.s))
        }

        contentItem: Column {
            id: joinForm
            spacing: Math.round(12 * root.s)

            Text {
                width: parent.width
                text: root.hiddenForm ? I18n.tr("Join a hidden network")
                    : I18n.tr("Password for %1").arg(root.pendingAp ? root.pendingAp.ssid : "")
                textFormat: Text.PlainText
                color: Lithic.fg
                font.family: QsLook.sans
                font.pixelSize: Math.round(16 * root.s)
                font.weight: Font.DemiBold
                wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                renderType: Text.NativeRendering
            }
            Text {
                visible: root.errorText !== ""
                width: parent.width
                text: root.errorText
                textFormat: Text.PlainText
                color: Lithic.jasper
                font.family: QsLook.sans
                font.pixelSize: Math.round(12 * root.s)
                wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                renderType: Text.NativeRendering
            }
            TextField {
                id: hiddenSsidField
                visible: root.hiddenForm
                width: parent.width
                placeholderText: I18n.tr("Network name (SSID)")
                text: root.hiddenSsid
                onTextEdited: root.hiddenSsid = text
                onAccepted: hiddenPasswordField.forceActiveFocus()
            }
            TextField {
                id: passwordField
                visible: !root.hiddenForm
                width: parent.width
                secret: true
                placeholderText: I18n.tr("Password")
                text: root.password
                onTextEdited: root.password = text
                onAccepted: root.submitPassword()
            }
            TextField {
                id: hiddenPasswordField
                visible: root.hiddenForm
                width: parent.width
                secret: true
                placeholderText: I18n.tr("Password (leave empty if open)")
                text: root.password
                onTextEdited: root.password = text
                onAccepted: root.submitHidden()
            }
            Row {
                anchors.right: parent.right
                spacing: Math.round(8 * root.s)
                // Fluent's dialogs lead with the primary action, the others end on it
                layoutDirection: QsLook.fluent ? Qt.RightToLeft : Qt.LeftToRight

                SidebarButton {
                    s: root.s
                    text: I18n.tr("Cancel")
                    armed: !root.busy
                    onAct: root.clearForm()
                }
                SidebarButton {
                    s: root.s
                    text: root.busy ? I18n.tr("Connecting…") : I18n.tr("Connect")
                    primary: true
                    armed: !root.busy && (root.hiddenForm ? root.hiddenSsid.trim() !== ""
                        : root.pendingAp !== null && root.password.length > 0)
                    onAct: root.hiddenForm ? root.submitHidden() : root.submitPassword()
                }
            }
        }
    }
}

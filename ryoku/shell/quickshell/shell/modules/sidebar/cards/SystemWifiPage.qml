pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import shell.services
import Ryoku.Ui.Singletons
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
    readonly property bool motionAllowed: root.active && !Tokens.reduceMotion && !Motion.reduce
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
        let activeBand = "";
        for (let i = 0; i < aps.length; i++) {
            const ap = aps[i];
            if (ap && ap.active) {
                activeBand = ap.band || "";
                break;
            }
        }
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

    component SectionTitle: Text {
        color: Tokens.ink
        font.family: Tokens.ui
        font.pixelSize: Tokens.fRow * root.s
        font.weight: Font.DemiBold
        wrapMode: Text.Wrap
    }

    component Field: QQC.TextField {
        height: 46 * root.s
        font.family: Tokens.ui
        font.pixelSize: Tokens.fRow * root.s
        color: Tokens.ink
        placeholderTextColor: Tokens.inkMuted
        selectionColor: Tokens.bone
        selectedTextColor: Tokens.inkOnBone
        selectByMouse: true
        leftPadding: Tokens.s3 * root.s
        rightPadding: Tokens.s3 * root.s
        background: Rectangle {
            radius: Tokens.radius * root.s * 1.5
            color: Tokens.tint5
            border.width: Tokens.border
            border.color: parent.activeFocus ? Tokens.bone : Tokens.line
        }
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
                anchors.right: headerActions.left
                anchors.leftMargin: Tokens.s4 * root.s
                anchors.rightMargin: Tokens.s3 * root.s
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Wi-Fi networks")
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Tokens.fTitle * root.s
                font.weight: Font.DemiBold
                elide: Text.ElideRight
            }
            Row {
                id: headerActions
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: Tokens.s2 * root.s
                SidebarButton {
                    s: root.s
                    compact: true
                    glyph: "refresh"
                    text: root.scanning ? I18n.tr("Scanning…") : I18n.tr("Refresh")
                    armed: Network.wifiPresent && Network.wifiRadio && !root.scanning
                    onAct: root.refresh()
                }
                SidebarToggle {
                    s: root.s
                    compact: true
                    width: implicitWidth
                    text: ""
                    glyph: ""
                    on: Network.wifiRadio
                    enabled: Network.wifiPresent
                    Accessible.name: I18n.tr("Wi-Fi radio")
                    onToggleRequested: Network.setWifiEnabled(!Network.wifiRadio)
                }
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
            color: Network.activeSsid !== "" ? Tokens.bone : Tokens.paperLift
            border.width: Tokens.border
            border.color: Network.activeSsid !== "" ? Tokens.bone : Tokens.lineSoft

            Behavior on color {
                enabled: root.motionAllowed
                ColorAnimation { duration: Tokens.snap }
            }

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
                    color: Network.activeSsid !== "" ? Tokens.inkOnBone : Tokens.tint10
                    Text {
                        anchors.centerIn: parent
                        text: !Network.wifiPresent || !Network.wifiRadio ? "wifi_off" : root.signalGlyph(Network.wifi.strength)
                        color: Network.activeSsid !== "" ? Tokens.bone : Tokens.inkDim
                        font.family: "Material Symbols Rounded"
                        font.pixelSize: 25 * root.s
                    }
                }
                Column {
                    width: Math.max(0, parent.width - parent.spacing - 52 * root.s - (disconnectButton.visible ? disconnectButton.width + parent.spacing : 0))
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s1 * root.s
                    Text {
                        width: parent.width
                        text: !Network.wifiPresent ? I18n.tr("No Wi-Fi hardware")
                            : !Network.wifiRadio ? I18n.tr("Wi-Fi is off")
                            : Network.activeSsid ? Network.activeSsid
                            : Network.wifiConnectivity === "Connecting" ? I18n.tr("Connecting…")
                            : I18n.tr("Ready to connect")
                        color: Network.activeSsid !== "" ? Tokens.inkOnBone : Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fValue * root.s
                        font.weight: Font.DemiBold
                        elide: Text.ElideRight
                    }
                    Text {
                        width: parent.width
                        text: !Network.wifiPresent ? I18n.tr("This machine has no available wireless adapter.")
                            : !Network.wifiRadio ? I18n.tr("Turn on the radio to search for nearby networks.")
                            : Network.activeSsid ? I18n.tr("Connected · %1% signal").arg(Network.wifi.strength || 0)
                            : root.scanning ? I18n.tr("Searching for nearby networks…")
                            : I18n.tr("Choose a network below.")
                        color: Network.activeSsid !== "" ? Tokens.inkOnBone : Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                        wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                    }
                }
                QQC.AbstractButton {
                    id: disconnectButton
                    visible: Network.activeSsid !== ""
                    hoverEnabled: true
                    implicitWidth: disconnectLabel.implicitWidth + Tokens.s3 * root.s * 2
                    width: implicitWidth
                    height: (Tokens.rowH - Tokens.s2) * root.s
                    Accessible.name: I18n.tr("Disconnect")
                    onClicked: Network.disconnectWifi()
                    background: Rectangle {
                        radius: Tokens.radius * root.s * 1.5
                        color: disconnectButton.down || disconnectButton.hovered ? Tokens.inkOnBone : "transparent"
                        border.width: Tokens.border
                        border.color: Tokens.inkOnBone
                        Behavior on color {
                            enabled: root.motionAllowed
                            ColorAnimation { duration: Tokens.snap }
                        }
                    }
                    contentItem: Text {
                        id: disconnectLabel
                        text: I18n.tr("Disconnect")
                        color: disconnectButton.down || disconnectButton.hovered ? Tokens.bone : Tokens.inkOnBone
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fBody * root.s
                        font.weight: Font.Medium
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                    }
                    HoverHandler { cursorShape: Qt.PointingHandCursor }
                }
            }
        }

        Text {
            visible: root.errorText !== "" && root.pendingAp === null && !root.hiddenForm
            width: parent.width
            text: root.errorText
            color: Tokens.alert
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall * root.s
            font.weight: Font.Medium
            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
        }

        Column {
            visible: Network.wifiPresent && Network.wifiRadio
            width: parent.width
            spacing: Tokens.s2 * root.s

            SectionTitle {
                width: parent.width
                text: I18n.tr("Available networks")
            }

            Repeater {
                model: root.availableNets
                delegate: Rectangle {
                    id: apRow
                    required property var modelData
                    readonly property bool connecting: root.busy && root.pendingKey === root.networkKey(apRow.modelData)
                    readonly property bool selected: root.pendingAp
                        && root.networkKey(root.pendingAp) === root.networkKey(apRow.modelData)
                    width: parent.width
                    implicitHeight: Math.max(64 * root.s, apCopy.implicitHeight + Tokens.s3 * root.s * 2)
                    height: implicitHeight
                    radius: Tokens.radius * root.s * 1.5
                    color: apRow.selected ? Tokens.bone
                        : apHover.hovered ? Tokens.tint10
                        : apRow.modelData.saved ? Tokens.tint10 : Tokens.tint5
                    border.width: Tokens.border
                    border.color: apRow.selected ? Tokens.bone : apHover.hovered ? Tokens.lineStrong : Tokens.lineSoft

                    Behavior on color {
                        enabled: root.motionAllowed
                        ColorAnimation { duration: Tokens.snap }
                    }
                    Behavior on border.color {
                        enabled: root.motionAllowed
                        ColorAnimation { duration: Tokens.snap }
                    }

                    Text {
                        id: apIcon
                        anchors.left: parent.left
                        anchors.leftMargin: Tokens.s3 * root.s
                        anchors.verticalCenter: parent.verticalCenter
                        width: 30 * root.s
                        horizontalAlignment: Text.AlignHCenter
                        text: root.signalGlyph(apRow.modelData.strength)
                        color: apRow.selected ? Tokens.inkOnBone : Tokens.inkDim
                        font.family: "Material Symbols Rounded"
                        font.pixelSize: 23 * root.s
                    }
                    Column {
                        id: apCopy
                        anchors.left: apIcon.right
                        anchors.right: apAction.left
                        anchors.leftMargin: Tokens.s3 * root.s
                        anchors.rightMargin: Tokens.s3 * root.s
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 2 * root.s
                        Text {
                            width: parent.width
                            text: apRow.modelData.ssid || ""
                            color: apRow.selected ? Tokens.inkOnBone : Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fRow * root.s
                            font.weight: Font.DemiBold
                            elide: Text.ElideRight
                        }
                        Text {
                            width: parent.width
                            text: (root.isOpen(apRow.modelData) ? I18n.tr("Open") : String(apRow.modelData.security || I18n.tr("Secure")))
                                + (apRow.modelData.saved ? " · " + I18n.tr("Saved") : "")
                                + (root.multiBandSsids[apRow.modelData.ssid] && apRow.modelData.band ? " · " + I18n.tr("%1 GHz").arg(apRow.modelData.band) : "")
                                + " · " + I18n.tr("%1% signal").arg(apRow.modelData.strength || 0)
                            color: apRow.selected ? Tokens.inkOnBone : Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall * root.s
                            elide: Text.ElideRight
                        }
                    }
                    SidebarButton {
                        id: apAction
                        s: root.s
                        compact: true
                        anchors.right: parent.right
                        anchors.rightMargin: Tokens.s3 * root.s
                        anchors.verticalCenter: parent.verticalCenter
                        text: apRow.connecting ? I18n.tr("Connecting…") : I18n.tr("Connect")
                        primary: apRow.selected
                        armed: !root.busy
                        onAct: root.choose(apRow.modelData)
                    }
                    HoverHandler { id: apHover }
                }
            }

            Text {
                visible: root.availableNets.length === 0
                width: parent.width
                text: root.scanning ? I18n.tr("Searching for nearby networks…")
                    : I18n.tr("No networks found. Move closer to an access point, then refresh.")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall * root.s
                wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                horizontalAlignment: Text.AlignHCenter
                topPadding: Tokens.s4 * root.s
                bottomPadding: Tokens.s4 * root.s
            }
        }

        QQC.Popup {
            id: joinPopup
            parent: root.card ?? QQC.Overlay.overlay
            visible: root.active && (root.pendingAp !== null || root.hiddenForm)
            width: Math.min(520 * root.s, parent ? parent.width - Tokens.s5 * root.s * 2 : root.width)
            implicitHeight: joinForm.implicitHeight + padding * 2
            x: parent ? (parent.width - width) / 2 : 0
            y: parent ? Math.max(Tokens.s4 * root.s, (parent.height - height) / 2) : 0
            padding: Tokens.s4 * root.s
            modal: true
            // the overlay spans the panel's whole column, so its dim would shade past the card
            dim: false
            focus: true
            closePolicy: root.busy ? QQC.Popup.NoAutoClose : QQC.Popup.CloseOnEscape | QQC.Popup.CloseOnPressOutside
            onOpened: Qt.callLater(() => root.hiddenForm ? hiddenSsidField.forceActiveFocus() : passwordField.forceActiveFocus())
            onClosed: if (!root.busy) root.clearForm()
            background: Rectangle {
                radius: Tokens.radius * root.s * 2
                color: Tokens.paperLift
                border.width: Tokens.border
                border.color: Tokens.line
            }

            contentItem: Column {
                id: joinForm
                spacing: Tokens.s3 * root.s

                SectionTitle {
                    width: parent.width
                    text: root.hiddenForm ? I18n.tr("Join a hidden network")
                        : I18n.tr("Password for %1").arg(root.pendingAp ? root.pendingAp.ssid : "")
                }
                Text {
                    visible: root.errorText !== ""
                    width: parent.width
                    text: root.errorText
                    color: Tokens.alert
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall * root.s
                    wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                }
                Field {
                    id: hiddenSsidField
                    visible: root.hiddenForm
                    width: parent.width
                    placeholderText: I18n.tr("Network name (SSID)")
                    text: root.hiddenSsid
                    onTextEdited: root.hiddenSsid = text
                    onAccepted: hiddenPasswordField.forceActiveFocus()
                }
                Field {
                    id: passwordField
                    visible: !root.hiddenForm
                    width: parent.width
                    placeholderText: I18n.tr("Password")
                    echoMode: TextInput.Password
                    text: root.password
                    onTextEdited: root.password = text
                    onAccepted: root.submitPassword()
                }
                Field {
                    id: hiddenPasswordField
                    visible: root.hiddenForm
                    width: parent.width
                    placeholderText: I18n.tr("Password (leave empty if open)")
                    echoMode: TextInput.Password
                    text: root.password
                    onTextEdited: root.password = text
                    onAccepted: root.submitHidden()
                }
                Row {
                    spacing: Tokens.s2 * root.s
                    SidebarButton {
                        s: root.s
                        text: root.busy ? I18n.tr("Connecting…") : I18n.tr("Connect")
                        primary: true
                        armed: !root.busy && (root.hiddenForm ? root.hiddenSsid.trim() !== ""
                            : root.pendingAp !== null && root.password.length > 0)
                        onAct: root.hiddenForm ? root.submitHidden() : root.submitPassword()
                    }
                    SidebarButton {
                        s: root.s
                        text: I18n.tr("Cancel")
                        armed: !root.busy
                        onAct: root.clearForm()
                    }
                }
            }
        }

        Row {
            visible: Network.wifiPresent && Network.wifiRadio && !root.hiddenForm && root.pendingAp === null
            spacing: Tokens.s2 * root.s
            SidebarButton {
                s: root.s
                glyph: "add"
                text: I18n.tr("Hidden network")
                onAct: {
                    root.pendingAp = null;
                    root.hiddenForm = true;
                    root.hiddenSsid = "";
                    root.password = "";
                    root.errorText = "";
                    Qt.callLater(() => hiddenSsidField.forceActiveFocus());
                }
            }
            SidebarButton {
                s: root.s
                visible: Network.activeSsid !== ""
                glyph: "delete"
                text: I18n.tr("Forget %1").arg(Network.activeSsid)
                onAct: Network.forgetWifi(Network.activeSsid)
            }
        }
    }
}

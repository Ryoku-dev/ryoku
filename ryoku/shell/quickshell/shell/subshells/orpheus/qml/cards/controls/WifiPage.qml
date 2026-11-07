pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../../kit" as K

Item {
    id: page

    required property var scene
    property var card: null
    property bool shown: false
    property string passwordSsid: ""
    property string passwordBssid: ""
    property string password: ""
    property int pendingId: -1
    property string connectingSsid: ""
    property string errorText: ""

    implicitWidth: 320
    implicitHeight: content.implicitHeight

    readonly property bool connected: Network.wifiConnectivity === "Connected"
    readonly property bool connecting: pendingId >= 0
    readonly property var accessPoints: {
        const source = (Network.accessPoints || []).slice();
        source.sort(function(left, right) {
            if (!!left.active !== !!right.active)
                return left.active ? -1 : 1;
            return Number(right.strength || 0) - Number(left.strength || 0);
        });
        return source.filter(function(ap) { return ap && !ap.active; });
    }

    function openNetwork(ap) {
        const security = String(ap && ap.security || "").toLowerCase();
        return security === "" || security === "none" || security === "open";
    }

    function strengthIcon(strength) {
        const value = Number(strength || 0);
        if (value >= 70) return "wifi";
        if (value >= 40) return "network_wifi_2_bar";
        return "network_wifi_1_bar";
    }

    function beginConnect(ssid, passwordValue, bssid) {
        if (connecting || !ssid)
            return;
        errorText = "";
        connectingSsid = ssid;
        pendingId = Network.connectWifi(ssid, passwordValue || "", bssid || "");
    }

    function choose(ap) {
        if (!ap || connecting)
            return;
        if (ap.saved || openNetwork(ap)) {
            passwordSsid = "";
            password = "";
            beginConnect(ap.ssid, "", ap.bssid);
            return;
        }
        passwordSsid = String(ap.ssid || "");
        passwordBssid = String(ap.bssid || "");
        password = "";
        errorText = "";
    }

    function submitPassword() {
        if (passwordSsid && password.length > 0)
            beginConnect(passwordSsid, password, passwordBssid);
    }

    function cancelPassword() {
        if (connecting)
            return;
        passwordSsid = "";
        passwordBssid = "";
        password = "";
        errorText = "";
    }

    Connections {
        target: Network
        function onReplied(id, ok, message) {
            if (id !== page.pendingId)
                return;
            page.pendingId = -1;
            page.connectingSsid = "";
            if (ok) {
                page.passwordSsid = "";
                page.passwordBssid = "";
                page.password = "";
                page.errorText = "";
            } else {
                page.password = "";
                page.errorText = message || "Could not connect";
            }
        }
    }

    component PasswordField: FocusScope {
        id: field

        property alias text: passwordInput.text
        property string placeholder: "Password"
        signal accepted()
        signal cancelled()

        implicitHeight: 48

        Rectangle {
            anchors.fill: parent
            radius: K.Style.radius
            color: "transparent"
            border.width: passwordInput.activeFocus ? 2 : K.Style.border
            border.color: passwordInput.activeFocus ? K.Style.bone : K.Style.line
        }

        K.Label {
            anchors.left: parent.left
            anchors.leftMargin: K.Style.s4
            anchors.right: parent.right
            anchors.rightMargin: K.Style.s4
            anchors.verticalCenter: parent.verticalCenter
            visible: passwordInput.text.length === 0
            text: field.placeholder
            role: "body"
            tone: "faint"
        }

        TextInput {
            id: passwordInput
            anchors.left: parent.left
            anchors.leftMargin: K.Style.s4
            anchors.right: parent.right
            anchors.rightMargin: K.Style.s4
            anchors.verticalCenter: parent.verticalCenter
            activeFocusOnTab: true
            color: K.Style.ink
            selectionColor: K.Style.bone
            selectedTextColor: K.Style.inkOnBone
            echoMode: TextInput.Password
            passwordCharacter: "•"
            font.family: K.Style.font
            font.pixelSize: K.Style.body
            font.features: K.Style.tabular
            font.capitalization: Font.MixedCase
            font.letterSpacing: 0
            clip: true

            Keys.onReturnPressed: event => {
                field.accepted();
                event.accepted = true;
            }
            Keys.onEnterPressed: event => {
                field.accepted();
                event.accepted = true;
            }
            Keys.onEscapePressed: event => {
                field.cancelled();
                event.accepted = true;
            }
        }

        TapHandler {
            acceptedButtons: Qt.LeftButton
            onTapped: passwordInput.forceActiveFocus()
        }
    }

    Column {
        id: content

        width: parent.width
        spacing: K.Style.s3

        K.ListRow {
            visible: page.connected
            width: parent.width
            icon: page.strengthIcon(Network.wifi && Network.wifi.strength)
            title: Network.activeSsid || "Wi-Fi"
            subtitle: "Connected"
            selected: true

            K.Button {
                z: 2
                text: "Disconnect"
                icon: "link_off"
                variant: "filled"
                onClicked: Network.disconnectWifi()
            }
        }

        K.Label {
            visible: !Network.wifiRadio
            width: parent.width
            text: Network.wifiPresent
                ? "Turn Wi-Fi on to see nearby networks."
                : "No Wi-Fi adapter found."
            role: "body"
            tone: "variant"
            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
        }

        Column {
            id: networkList

            visible: Network.wifiRadio
            width: parent.width
            spacing: K.Style.s2

            Repeater {
                model: page.accessPoints

                delegate: Item {
                    id: accessPoint

                    required property var modelData
                    required property int index
                    width: networkList.width
                    implicitHeight: networkRow.implicitHeight
                        + (page.passwordSsid === String(modelData.ssid || "")
                            ? passwordPanel.implicitHeight + K.Style.s2 : 0)
                    height: implicitHeight

                    K.ListRow {
                        id: networkRow

                        width: parent.width
                        icon: page.strengthIcon(accessPoint.modelData.strength)
                        title: accessPoint.modelData.ssid || "Hidden network"
                        subtitle: page.connectingSsid === String(accessPoint.modelData.ssid || "")
                            ? "Connecting…"
                            : (accessPoint.modelData.saved ? "Saved"
                                : (page.openNetwork(accessPoint.modelData) ? "Open"
                                    : String(accessPoint.modelData.security || "Secured")))
                        divider: accessPoint.index > 0 || page.connected
                        enabled: !page.connecting
                        onClicked: page.choose(accessPoint.modelData)
                        onRightClicked: if (accessPoint.modelData.saved)
                            Network.forgetWifi(accessPoint.modelData.ssid)

                        K.Glyph {
                            visible: !page.openNetwork(accessPoint.modelData)
                            name: "lock"
                            size: K.Style.icon
                            tone: "variant"
                        }
                    }

                    Column {
                        id: passwordPanel

                        anchors.top: networkRow.bottom
                        anchors.topMargin: K.Style.s2
                        width: parent.width
                        visible: page.passwordSsid === String(accessPoint.modelData.ssid || "")
                        spacing: K.Style.s2

                        Row {
                            width: parent.width
                            spacing: K.Style.s2

                            PasswordField {
                                width: Math.max(100, parent.width - connectButton.width
                                    - cancelButton.width - 2 * parent.spacing)
                                text: page.passwordSsid === String(accessPoint.modelData.ssid || "")
                                    ? page.password : ""
                                enabled: !page.connecting
                                onTextChanged: if (page.passwordSsid === String(accessPoint.modelData.ssid || ""))
                                    page.password = text
                                onAccepted: page.submitPassword()
                                onCancelled: page.cancelPassword()
                            }

                            K.Button {
                                id: connectButton
                                anchors.verticalCenter: parent.verticalCenter
                                width: implicitWidth
                                text: page.connecting ? "Connecting…" : "Connect"
                                variant: "filled"
                                enabled: page.password.length > 0 && !page.connecting
                                onClicked: page.submitPassword()
                            }

                            K.IconButton {
                                id: cancelButton
                                anchors.verticalCenter: parent.verticalCenter
                                icon: "close"
                                enabled: !page.connecting
                                onClicked: page.cancelPassword()
                            }
                        }

                        K.Label {
                            visible: page.errorText.length > 0
                            width: parent.width
                            text: page.errorText
                            role: "caption"
                            tone: "alert"
                            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                        }
                    }
                }
            }

            K.Label {
                visible: page.accessPoints.length === 0
                width: parent.width
                text: Network.wifiConnectivity === "Connecting"
                    ? "Looking for networks…" : "No networks found"
                role: "body"
                tone: "variant"
            }
        }

        K.Label {
            visible: page.errorText.length > 0 && page.passwordSsid.length === 0
            width: parent.width
            text: page.errorText
            role: "caption"
            tone: "alert"
            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
        }
    }
}

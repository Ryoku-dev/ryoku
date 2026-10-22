pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.utils

ColumnLayout {
    id: root

    required property PopoutState popouts
    property var network: NetworkConnection.passwordNetwork
    property string connectingSsid: ""
    property bool isClosing: false

    readonly property bool shouldBeVisible: root.popouts.currentName === "wirelesspassword"

    property bool _isSidebarOpen: false

    property real scaleOffset: 1.0
    property real fontScale: 1.0

    function checkConnectionStatus(): void {
        if (!root.shouldBeVisible || !connectButton.connecting) {
            return;
        }

        const target = (root.connectingSsid || (root.network ? root.network.ssid : "")).toLowerCase().trim();
        if (!target) {
            return;
        }

        const isConnected = Nmcli.active && Nmcli.active.ssid && Nmcli.active.ssid.toLowerCase().trim() === target;

        if (isConnected) {
            if (!connectionSuccessTimer.running) {
                connectionSuccessTimer.start();
            }
            return;
        }

        if (Nmcli.pendingConnection === null && connectButton.connecting) {
            if (connectionMonitor.repeatCount > 10) {
                connectionMonitor.stop();
                connectButton.connecting = false;
                connectButton.hasError = true;
                connectButton.enabled = true;
                connectButton.text = qsTr("Connect");
                passwordContainer.passwordBuffer = "";
                if (target) {
                    Nmcli.forgetNetwork(target);
                }
            }
        }
    }

    function closeDialog(): void {
        if (isClosing) {
            return;
        }

        isClosing = true;
        root.connectingSsid = "";
        connectionSuccessTimer.stop();
        resetPopoutTimer.stop();
        passwordContainer.passwordBuffer = "";
        connectButton.connecting = false;
        connectButton.hasError = false;
        connectButton.text = qsTr("Connect");
        connectionMonitor.stop();
        NetworkConnection.passwordNetwork = null;

        if (root.popouts.currentName === "wirelesspassword") {
            root.popouts.currentName = "network";
        }
    }

    spacing: Tokens.spacing.medium * scaleOffset
    implicitWidth: Math.max(400 * scaleOffset, _isSidebarOpen ? (Visibilities.sidebarWidthFor(Tokens.sizes.sidebar.width) * scaleOffset) - Tokens.padding.extraLargeIncreased : 0)
    implicitHeight: content.implicitHeight + Tokens.padding.extraLargeIncreased * scaleOffset
    visible: shouldBeVisible || isClosing
    enabled: shouldBeVisible && !isClosing
    focus: enabled

    Component.onCompleted: {
        if (shouldBeVisible) {
            focusTimer.start();
        }
    }

    onShouldBeVisibleChanged: {
        if (shouldBeVisible) {
            if (NetworkConnection.passwordNetwork) {
                root.network = {
                    ssid: NetworkConnection.passwordNetwork.ssid,
                    bssid: NetworkConnection.passwordNetwork.bssid || "",
                    isSecure: NetworkConnection.passwordNetwork.isSecure ?? true,
                    strength: NetworkConnection.passwordNetwork.strength ?? 0
                };
            }
            focusTimer.start();
        } else {
            root.connectingSsid = "";
        }
    }

    Keys.onEscapePressed: closeDialog()

    Connections {
        function onHasCurrentChanged() {
            if (!root.popouts.hasCurrent && root.popouts.currentName === "wirelesspassword") {
                root.connectingSsid = "";
                connectionMonitor.stop();
                connectionSuccessTimer.stop();
                resetPopoutTimer.stop();
                connectButton.connecting = false;
                connectButton.hasError = false;
                passwordContainer.passwordBuffer = "";
                NetworkConnection.passwordNetwork = null;
                root.popouts.currentName = "network";
            }
        }

        function onCurrentNameChanged() {
            if (root.popouts.currentName === "wirelesspassword") {
                focusTimer.start();
            } else {
                root.connectingSsid = "";
                connectionSuccessTimer.stop();
                resetPopoutTimer.stop();
            }
        }

        target: root.popouts
    }

    Timer {
        id: focusTimer

        interval: 150
        onTriggered: {
            root.forceActiveFocus();
            passwordContainer.forceActiveFocus();
        }
    }

    StyledRect {
        id: card

        Layout.fillWidth: true
        Layout.preferredWidth: 400 * root.scaleOffset
        implicitHeight: content.implicitHeight + Tokens.padding.extraLargeIncreased * root.scaleOffset
        radius: Tokens.rounding.large * root.scaleOffset
        color: Colours.tPalette.m3surfaceContainer
        visible: root.shouldBeVisible || root.isClosing
        opacity: root.shouldBeVisible && !root.isClosing ? 1 : 0
        scale: root.shouldBeVisible && !root.isClosing ? 1 : 0.7
        Keys.onEscapePressed: root.closeDialog()

        Behavior on opacity {
            Anim {
                type: Anim.DefaultEffects
            }
        }

        Behavior on scale {
            Anim {}
        }

        ParallelAnimation {
            running: root.isClosing
            onFinished: {
                if (root.isClosing) {
                    root.isClosing = false;
                }
            }

            Anim {
                type: Anim.DefaultEffects
                target: card
                property: "opacity"
                to: 0
            }
            Anim {
                target: card
                property: "scale"
                to: 0.7
            }
        }

        ColumnLayout {
            id: content

            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.margins: Tokens.padding.large * root.scaleOffset

            spacing: Tokens.spacing.medium * root.scaleOffset

            MaterialIcon {
                Layout.alignment: Qt.AlignHCenter
                text: "lock"
                fontStyle: Tokens.font.icon.builders.extraLarge.scale(2 * root.scaleOffset).build()
            }

            StyledText {
                Layout.alignment: Qt.AlignHCenter
                text: qsTr("Enter password")
                font.pointSize: Tokens.font.body.large.pointSize * root.fontScale
                font.weight: Font.Medium
            }

            StyledText {
                id: networkNameText

                Layout.alignment: Qt.AlignHCenter
                text: {
                    if (root.network) {
                        const ssid = root.network.ssid;
                        if (ssid && ssid.length > 0) {
                            return qsTr("Network: %1").arg(ssid);
                        }
                    }
                    return qsTr("Unknown network");
                }
                color: Colours.palette.m3outline
                font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
            }

            Timer {
                property int attempts: 0

                interval: 50
                running: root.shouldBeVisible && !connectButton.connecting && (!root.network || !root.network.ssid)
                repeat: true
                onTriggered: {
                    attempts++;
                    // Fallback in case property binding was delayed
                    if (NetworkConnection.passwordNetwork) {
                        root.network = {
                            ssid: NetworkConnection.passwordNetwork.ssid,
                            bssid: NetworkConnection.passwordNetwork.bssid || "",
                            isSecure: NetworkConnection.passwordNetwork.isSecure ?? true,
                            strength: NetworkConnection.passwordNetwork.strength ?? 0
                        };
                    }
                    if ((root.network && root.network.ssid) || attempts >= 20) {
                        stop();
                        attempts = 0;
                    }
                }
                onRunningChanged: {
                    if (!running) {
                        attempts = 0;
                    }
                }
            }

            StyledText {
                id: statusText

                Layout.alignment: Qt.AlignHCenter
                Layout.topMargin: Tokens.spacing.small * root.scaleOffset
                visible: connectButton.connecting || connectButton.hasError
                text: {
                    if (connectButton.hasError) {
                        return qsTr("Connection failed. Please check your password and try again.");
                    }
                    if (connectButton.connecting) {
                        return qsTr("Connecting...");
                    }
                    return "";
                }
                color: connectButton.hasError ? Colours.palette.m3error : Colours.palette.m3onSurfaceVariant
                font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
                font.weight: Font.Normal
                wrapMode: Text.WordWrap
                Layout.maximumWidth: parent.width - Tokens.padding.extraLargeIncreased * root.scaleOffset
            }

            FocusScope {
                id: passwordContainer

                property string passwordBuffer: ""

                objectName: "passwordContainer"
                Layout.topMargin: Tokens.spacing.largeIncreased * root.scaleOffset
                Layout.fillWidth: true
                implicitHeight: Math.max(48 * root.scaleOffset, charList.implicitHeight + Tokens.padding.medium * 2 * root.scaleOffset)
                focus: true
                activeFocusOnTab: true

                Component.onCompleted: {
                    if (root.shouldBeVisible) {
                        passwordFocusTimer.start();
                    }
                }

                Keys.onPressed: event => {
                    if (!activeFocus) {
                        forceActiveFocus();
                    }

                    if (event.key === Qt.Key_Escape) {
                        event.accepted = false;
                        closeDialog();
                    }

                    if (connectButton.hasError && event.text && event.text.length > 0) {
                        connectButton.hasError = false;
                    }

                    if (event.key === Qt.Key_Enter || event.key === Qt.Key_Return) {
                        if (connectButton.enabled) {
                            connectButton.clicked();
                        }
                        event.accepted = true;
                    } else if (event.key === Qt.Key_Backspace) {
                        if (event.modifiers & Qt.ControlModifier) {
                            passwordBuffer = "";
                        } else {
                            passwordBuffer = passwordBuffer.slice(0, -1);
                        }
                        event.accepted = true;
                    } else if (event.text && event.text.length > 0) {
                        if (event.key === Qt.Key_Tab) {
                            event.accepted = false;
                            return;
                        }
                        passwordBuffer += event.text;
                        event.accepted = true;
                    }
                }

                Connections {
                    function onShouldBeVisibleChanged(): void {
                        if (root.shouldBeVisible) {
                            passwordFocusTimer.start();
                            passwordContainer.passwordBuffer = "";
                            connectButton.hasError = false;
                        }
                    }

                    target: root
                }

                Timer {
                    id: passwordFocusTimer

                    interval: 50
                    onTriggered: {
                        passwordContainer.forceActiveFocus();
                    }
                }

                StyledRect {
                    anchors.fill: parent
                    radius: Tokens.rounding.large * root.scaleOffset
                    color: passwordContainer.activeFocus ? Qt.lighter(Colours.tPalette.m3surfaceContainer, 1.05) : Colours.tPalette.m3surfaceContainer
                    border.width: passwordContainer.activeFocus || connectButton.hasError ? 4 : (root.shouldBeVisible ? 1 : 0)
                    border.color: {
                        if (connectButton.hasError) {
                            return Colours.palette.m3error;
                        }
                        if (passwordContainer.activeFocus) {
                            return Colours.palette.m3primary;
                        }
                        return root.shouldBeVisible ? Colours.palette.m3outline : Qt.alpha(Colours.palette.m3outline, 0);
                    }

                    Behavior on border.color {
                        CAnim {}
                    }

                    Behavior on border.width {
                        Anim {
                            type: Anim.FastSpatial
                        }
                    }

                    Behavior on color {
                        CAnim {}
                    }
                }

                StateLayer {
                    hoverEnabled: false
                    cursorShape: Qt.IBeamCursor
                    radius: Tokens.rounding.large * root.scaleOffset
                    onClicked: passwordContainer.forceActiveFocus()
                }

                StyledText {
                    id: placeholder

                    anchors.centerIn: parent
                    text: qsTr("Password")
                    color: Colours.palette.m3outline
                    font.pointSize: Tokens.font.mono.medium.pointSize * root.fontScale
                    opacity: passwordContainer.passwordBuffer ? 0 : 1

                    Behavior on opacity {
                        Anim {
                            type: Anim.DefaultEffects
                        }
                    }
                }

                ListView {
                    id: charList

                    readonly property int fullWidth: count * (implicitHeight + spacing) - spacing

                    anchors.centerIn: parent
                    implicitWidth: fullWidth
                    implicitHeight: Tokens.font.body.medium.pointSize * root.fontScale

                    orientation: Qt.Horizontal
                    spacing: Tokens.spacing.extraSmall * root.scaleOffset
                    interactive: false

                    model: ScriptModel {
                        values: passwordContainer.passwordBuffer.split("")
                    }

                    delegate: StyledRect {
                        id: ch

                        implicitWidth: implicitHeight
                        implicitHeight: charList.implicitHeight

                        color: Colours.palette.m3onSurface
                        radius: Tokens.rounding.medium / 2 * root.scaleOffset

                        opacity: 0
                        scale: 0
                        Component.onCompleted: {
                            opacity = 1;
                            scale = 1;
                        }
                        ListView.onRemove: removeAnim.start()

                        SequentialAnimation {
                            id: removeAnim

                            PropertyAction {
                                target: ch
                                property: "ListView.delayRemove"
                                value: true
                            }
                            ParallelAnimation {
                                Anim {
                                    type: Anim.DefaultEffects
                                    target: ch
                                    property: "opacity"
                                    to: 0
                                }
                                Anim {
                                    target: ch
                                    property: "scale"
                                    to: 0.5
                                }
                            }
                            PropertyAction {
                                target: ch
                                property: "ListView.delayRemove"
                                value: false
                            }
                        }

                        Behavior on opacity {
                            Anim {
                                type: Anim.DefaultEffects
                            }
                        }

                        Behavior on scale {
                            Anim {
                                type: Anim.FastSpatial
                            }
                        }
                    }

                    Behavior on implicitWidth {
                        Anim {}
                    }
                }
            }

            RowLayout {
                Layout.topMargin: Tokens.spacing.medium * root.scaleOffset
                Layout.fillWidth: true
                spacing: Tokens.spacing.medium * root.scaleOffset

                TextButton {
                    id: cancelButton

                    Layout.fillWidth: true
                    Layout.minimumHeight: Tokens.font.body.medium.pointSize * root.fontScale + Tokens.padding.medium * 2 * root.fontScale
                    inactiveColour: Colours.palette.m3secondaryContainer
                    inactiveOnColour: Colours.palette.m3onSecondaryContainer
                    text: qsTr("Cancel")

                    onClicked: root.closeDialog()
                }

                TextButton {
                    id: connectButton

                    property bool connecting: false
                    property bool hasError: false

                    Layout.fillWidth: true
                    Layout.minimumHeight: Tokens.font.body.medium.pointSize * root.fontScale + Tokens.padding.medium * 2 * root.fontScale
                    inactiveColour: Colours.palette.m3primary
                    inactiveOnColour: Colours.palette.m3onPrimary
                    text: qsTr("Connect")
                    enabled: passwordContainer.passwordBuffer.length > 0 && !connecting

                    onClicked: {
                        if (!root.network || connecting) {
                            return;
                        }

                        const password = passwordContainer.passwordBuffer;
                        if (!password || password.length === 0) {
                            return;
                        }

                        hasError = false;

                        connecting = true;
                        root.connectingSsid = root.network ? root.network.ssid : "";
                        enabled = false;
                        text = qsTr("Connecting...");

                        NetworkConnection.connectWithPassword(root.network, password, result => {
                            if (result && result.success) {
                                root.checkConnectionStatus();
                            } else if (result && result.needsPassword) {
                                connectionMonitor.stop();
                                connecting = false;
                                hasError = true;
                                enabled = true;
                                text = qsTr("Connect");
                                passwordContainer.passwordBuffer = "";
                                if (root.network && root.network.ssid) {
                                    Nmcli.forgetNetwork(root.network.ssid);
                                }
                            } else {
                                connectionMonitor.stop();
                                connecting = false;
                                hasError = true;
                                enabled = true;
                                text = qsTr("Connect");
                                passwordContainer.passwordBuffer = "";
                                if (root.network && root.network.ssid) {
                                    Nmcli.forgetNetwork(root.network.ssid);
                                }
                            }
                        });

                        connectionMonitor.start();
                    }
                }
            }
        }
    }

    Timer {
        id: connectionMonitor

        property int repeatCount: 0

        interval: 1000
        repeat: true
        triggeredOnStart: false

        onTriggered: {
            repeatCount++;
            root.checkConnectionStatus();
        }

        onRunningChanged: {
            if (!running) {
                repeatCount = 0;
            }
        }
    }

    Timer {
        id: connectionSuccessTimer

        interval: 300
        onTriggered: {
            const target = (root.connectingSsid || (root.network ? root.network.ssid : "")).toLowerCase().trim();
            if (root.shouldBeVisible && Nmcli.active && Nmcli.active.ssid) {
                const stillConnected = Nmcli.active.ssid.toLowerCase().trim() === target;
                if (stillConnected) {
                    connectionMonitor.stop();
                    connectButton.connecting = false;
                    connectButton.hasError = false;
                    connectButton.text = qsTr("Connect");
                    passwordContainer.passwordBuffer = "";
                    root.connectingSsid = "";
                    NetworkConnection.passwordNetwork = null;
                    root.popouts.hasCurrent = false;
                    resetPopoutTimer.start();
                }
            }
        }
    }

    Timer {
        id: resetPopoutTimer

        interval: 350
        onTriggered: {
            if (root.popouts.currentName === "wirelesspassword") {
                root.popouts.currentName = "network";
            }
        }
    }

    Connections {
        function onActiveChanged() {
            if (root.shouldBeVisible) {
                root.checkConnectionStatus();
            }
        }

        function onConnectionSuccessful(ssid: string) {
            const target = (root.connectingSsid || (root.network ? root.network.ssid : "")).toLowerCase().trim();
            if (root.shouldBeVisible && target && target === ssid.toLowerCase().trim()) {
                root.checkConnectionStatus();
            }
        }

        function onConnectionFailed(ssid: string) {
            const target = (root.connectingSsid || (root.network ? root.network.ssid : "")).toLowerCase().trim();
            if (root.shouldBeVisible && target && target === ssid.toLowerCase().trim() && connectButton.connecting) {
                connectionMonitor.stop();
                connectionSuccessTimer.stop();
                connectButton.connecting = false;
                connectButton.hasError = true;
                connectButton.enabled = true;
                connectButton.text = qsTr("Connect");
                passwordContainer.passwordBuffer = "";
                root.connectingSsid = "";
                Nmcli.forgetNetwork(ssid);
            }
        }

        target: Nmcli
    }
}

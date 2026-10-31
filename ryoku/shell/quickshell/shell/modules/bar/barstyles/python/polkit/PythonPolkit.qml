pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Quickshell
import Quickshell.Wayland
import shell.services as RyokuServices
import Ryoku.Ui.Singletons
import ".."

Scope {
    id: root

    property var targetScreen: null

    function focusedScreen() {
        const screens = Quickshell.screens || [];
        for (let i = 0; i < screens.length; i++) {
            if (screens[i].name === Wm.focusedOutput) return screens[i];
        }
        return screens.length > 0 ? screens[0] : null;
    }

    Connections {
        target: RyokuServices.Polkit
        function onActiveChanged() {
            if (RyokuServices.Polkit.active) root.targetScreen = root.focusedScreen();
        }
    }

    Loader {
        active: RyokuServices.Polkit.active
        sourceComponent: PanelWindow {
            id: window

            screen: root.targetScreen || root.focusedScreen()
            color: "transparent"
            exclusionMode: ExclusionMode.Ignore
            WlrLayershell.namespace: "quickshell:python-polkit"
            WlrLayershell.layer: WlrLayer.Overlay
            WlrLayershell.keyboardFocus: WlrKeyboardFocus.Exclusive

            anchors {
                top: true
                bottom: true
                left: true
                right: true
            }

            Shortcut {
                sequence: "Escape"
                onActivated: RyokuServices.Polkit.cancel()
            }

            Rectangle {
                anchors.fill: parent
                color: Qt.rgba(0, 0, 0, 0.55)

                MouseArea {
                    anchors.fill: parent
                    onClicked: RyokuServices.Polkit.cancel()
                }
            }

            Rectangle {
                id: card
                anchors.centerIn: parent
                width: Math.min(520, window.width - 40)
                implicitHeight: body.implicitHeight + 36
                height: implicitHeight
                radius: ThemeBackend.borderRadius
                color: ThemeBackend.base

                MouseArea { anchors.fill: parent }

                ColumnLayout {
                    id: body
                    anchors.fill: parent
                    anchors.margins: 18
                    spacing: 14

                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 12

                        Rectangle {
                            Layout.preferredWidth: 44
                            Layout.preferredHeight: 44
                            radius: Math.max(0, ThemeBackend.borderRadius - 2)
                            color: ThemeBackend.surface0

                            Text {
                                anchors.centerIn: parent
                                text: "󰌾"
                                font.family: ThemeBackend.iconFont
                                font.pixelSize: 22
                                color: ThemeBackend.mauve
                            }
                        }

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 3

                            Text {
                                Layout.fillWidth: true
                                text: RyokuServices.Polkit.message || I18n.t("polkit.default_message", "Authentication required")
                                color: ThemeBackend.text
                                font.family: ThemeBackend.fontFamily
                                font.pixelSize: 14
                                font.weight: Font.Bold
                                wrapMode: Text.Wrap
                            }

                            Text {
                                Layout.fillWidth: true
                                visible: text.length > 0
                                text: RyokuServices.Polkit.info
                                color: ThemeBackend.subtext0
                                font.family: ThemeBackend.fontFamily
                                font.pixelSize: 12
                                wrapMode: Text.Wrap
                            }
                        }
                    }

                    Text {
                        Layout.fillWidth: true
                        visible: text.length > 0
                        text: RyokuServices.Polkit.error
                        color: ThemeBackend.red
                        font.family: ThemeBackend.fontFamily
                        font.pixelSize: 12
                        wrapMode: Text.Wrap
                    }

                    TextField {
                        id: input
                        Layout.fillWidth: true
                        enabled: RyokuServices.Polkit.active && !RyokuServices.Polkit.busy
                        echoMode: RyokuServices.Polkit.echo ? TextInput.Normal : TextInput.Password
                        placeholderText: {
                            const prompt = RyokuServices.Polkit.prompt.trim();
                            return prompt.length > 0 ? prompt.replace(/:\s*$/, "") : I18n.t("polkit.password_placeholder", "Password");
                        }
                        font.family: ThemeBackend.fontFamily
                        color: ThemeBackend.text
                        onAccepted: if (enabled) RyokuServices.Polkit.submit(text)
                        background: Rectangle {
                            radius: Math.max(0, ThemeBackend.borderRadius - 4)
                            color: ThemeBackend.surface0
                            border.width: input.activeFocus ? 1 : 0
                            border.color: ThemeBackend.mauve
                        }
                    }

                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 8

                        Button {
                            Layout.fillWidth: true
                            text: I18n.t("common.cancel", "Cancel")
                            onClicked: RyokuServices.Polkit.cancel()
                        }

                        Button {
                            Layout.fillWidth: true
                            enabled: RyokuServices.Polkit.active && !RyokuServices.Polkit.busy
                            text: RyokuServices.Polkit.busy ? I18n.t("polkit.checking", "Checking...") : I18n.t("polkit.authenticate", "Authenticate")
                            onClicked: RyokuServices.Polkit.submit(input.text)
                        }
                    }
                }

                Connections {
                    target: RyokuServices.Polkit
                    function onRefreshed() {
                        input.text = "";
                        input.forceActiveFocus();
                    }
                }

                Component.onCompleted: input.forceActiveFocus()
            }
        }
    }
}

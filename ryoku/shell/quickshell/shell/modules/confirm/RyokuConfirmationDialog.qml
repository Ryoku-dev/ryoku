pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import shell.services
import Ryoku.Ui.Singletons
import "../desktop/Singletons" as Desktop

// Shared confirmation for desktop and bar session actions.
PanelWindow {
    id: root

    required property var modelData
    property string action: ""
    property string message: ""
    property string positiveLabel: ""
    property string negativeLabel: I18n.tr("Cancel")
    property string focusedChoice: "confirm"
    signal confirmed(string action)
    signal cancelled(string action)

    function activateFocused() {
        if (root.focusedChoice === "confirm")
            root.confirmed(root.action);
        else
            root.cancelled(root.action);
    }

    screen: modelData
    visible: root.action !== ""
    color: "transparent"
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Top
    WlrLayershell.keyboardFocus: root.action !== "" ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
    WlrLayershell.namespace: "ryoku-dialog"
    anchors { top: true; bottom: true; left: true; right: true }

    mask: Region {
        x: card.x
        y: card.y
        width: card.width
        height: card.height
    }

    onActionChanged: if (root.action !== "") {
        root.focusedChoice = "confirm";
        keyboard.forceActiveFocus();
    }

    Item {
        id: keyboard
        anchors.fill: parent
        focus: root.action !== ""
        Keys.onEscapePressed: root.cancelled(root.action)
        Keys.onReturnPressed: root.activateFocused()
        Keys.onEnterPressed: root.activateFocused()
        Keys.onPressed: event => {
            if (event.key === Qt.Key_Tab || event.key === Qt.Key_Backtab) {
                root.focusedChoice = root.focusedChoice === "confirm" ? "cancel" : "confirm";
                event.accepted = true;
            }
        }

        Rectangle {
            id: card
            anchors.centerIn: parent
            width: Math.min(Tokens.s7 * 8, root.width - Tokens.s4 * 2)
            height: content.implicitHeight + Tokens.s5 * 2
            radius: Tokens.radius
            color: Tokens.paper
            border.width: Tokens.border
            border.color: Tokens.line
            opacity: root.action !== "" ? 1 : 0
            Behavior on opacity { NumberAnimation { duration: Tokens.dur(140) } }
            transform: Scale {
                origin.x: card.width / 2
                origin.y: card.height / 2
                xScale: root.action !== "" ? 1 : 0.96
                yScale: root.action !== "" ? 1 : 0.96
                Behavior on xScale { NumberAnimation { duration: Tokens.dur(140); easing.type: Easing.OutCubic } }
                Behavior on yScale { NumberAnimation { duration: Tokens.dur(140); easing.type: Easing.OutCubic } }
            }

            Column {
                id: content
                anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s5 }
                spacing: Tokens.s5

                Row {
                    width: parent.width
                    spacing: Tokens.s2
                    Text {
                        visible: UiPreferences.showJapaneseGloss
                        text: "卓上"
                        color: Tokens.inkFaint
                        font.family: Tokens.jp
                        font.pixelSize: Tokens.fBody
                    }
                    Text {
                        text: I18n.tr("Desktop").toUpperCase()
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                        font.weight: Font.DemiBold
                        font.letterSpacing: Tokens.trackMark
                        anchors.verticalCenter: parent.verticalCenter
                    }
                    Rectangle {
                        width: Math.max(0, content.width - parent.children[0].width
                            - parent.children[1].width - Tokens.s2 * 3)
                        height: Tokens.border
                        color: Tokens.line
                        anchors.verticalCenter: parent.verticalCenter
                    }
                }

                Row {
                    width: parent.width
                    spacing: Tokens.s4
                    Rectangle {
                        width: Tokens.rowH
                        height: Tokens.rowH
                        radius: Tokens.radius
                        color: Tokens.tint5
                        border.width: Tokens.border
                        border.color: Tokens.lineSoft
                        Text {
                            anchors.centerIn: parent
                            text: root.action === "shutdown" ? "power_settings_new"
                                : root.action === "reboot" ? "restart_alt" : "logout"
                            color: Tokens.ink
                            font.family: Desktop.Theme.iconFont
                            font.pixelSize: Tokens.fValue
                        }
                    }
                    Column {
                        width: parent.width - Tokens.rowH - Tokens.s4
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: Tokens.s1
                        Text {
                            width: parent.width
                            text: root.positiveLabel
                            color: Tokens.ink
                            font.family: Tokens.display
                            font.pixelSize: Tokens.fValue
                            elide: Text.ElideRight
                        }
                        Text {
                            width: parent.width
                            text: root.message
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }
                    }
                }

                Rectangle {
                    width: parent.width
                    height: Tokens.border
                    color: Tokens.lineSoft
                }

                Row {
                    anchors.right: parent.right
                    spacing: Tokens.s2

                    Rectangle {
                        id: cancelButton
                        width: Math.max(Tokens.s7 * 2, cancelText.implicitWidth + Tokens.s4 * 2)
                        height: Tokens.rowH
                        radius: Tokens.radius
                        color: cancelMouse.containsPress ? Tokens.tint16
                            : cancelMouse.containsMouse ? Tokens.tint10 : Tokens.paper
                        border.width: Tokens.border
                        border.color: keyboard.activeFocus && root.focusedChoice === "cancel"
                            ? Tokens.ink : Tokens.line
                        Text {
                            id: cancelText
                            anchors.centerIn: parent
                            text: root.negativeLabel.toUpperCase()
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fMicro
                            font.weight: Font.DemiBold
                            font.letterSpacing: Tokens.trackLabel
                        }
                        MouseArea {
                            id: cancelMouse
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: root.cancelled(root.action)
                        }
                    }

                    Rectangle {
                        id: confirmButton
                        width: Math.max(Tokens.s7 * 2, confirmText.implicitWidth + Tokens.s4 * 2)
                        height: Tokens.rowH
                        radius: Tokens.radius
                        color: confirmMouse.containsPress ? Tokens.tint16 : Tokens.bone
                        border.width: Tokens.border
                        border.color: keyboard.activeFocus && root.focusedChoice === "confirm"
                            ? Tokens.ink : Tokens.lineOnBone
                        Text {
                            id: confirmText
                            anchors.centerIn: parent
                            text: root.positiveLabel.toUpperCase()
                            color: Tokens.inkOnBone
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fMicro
                            font.weight: Font.DemiBold
                            font.letterSpacing: Tokens.trackLabel
                        }
                        MouseArea {
                            id: confirmMouse
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: root.confirmed(root.action)
                        }
                    }
                }
            }
        }
    }
}

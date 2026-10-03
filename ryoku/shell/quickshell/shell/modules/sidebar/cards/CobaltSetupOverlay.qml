pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import shell.services

Item {
    id: root

    property real s: 1
    property bool open: false
    signal closed()

    readonly property bool busy: Stash.setupState === "running"

    visible: opacity > 0.01
    enabled: root.open
    opacity: root.open ? 1 : 0
    z: 50

    Behavior on opacity {
        NumberAnimation {
            duration: Tokens.dur(140)
            easing.type: Tokens.ease
        }
    }

    Rectangle {
        anchors.fill: parent
        color: Qt.rgba(0, 0, 0, 0.66)
        TapHandler { onTapped: if (!root.busy) root.closed() }
    }

    Rectangle {
        id: panel
        anchors.centerIn: parent
        width: Math.min(parent.width - Tokens.s5 * root.s * 2, 330 * root.s)
        implicitHeight: content.implicitHeight + Tokens.s5 * root.s * 2
        height: implicitHeight
        radius: Tokens.radius * root.s
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        scale: root.open ? 1 : 0.96

        Behavior on scale {
            NumberAnimation {
                duration: Tokens.dur(180)
                easing.type: Tokens.ease
            }
        }

        TapHandler { }

        Column {
            id: content
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Tokens.s5 * root.s
            spacing: Tokens.s4 * root.s

            Row {
                width: parent.width
                spacing: Tokens.s3 * root.s

                Rectangle {
                    width: 40 * root.s
                    height: width
                    radius: Tokens.radius * root.s
                    color: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.14)
                    border.width: Tokens.border
                    border.color: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.40)
                    Text {
                        font.family: "Material Symbols Rounded"
                        anchors.centerIn: parent
                        text: "deployed_code_update"
                        color: Tokens.sun
                        font.pixelSize: 22 * root.s
                    }
                }

                Column {
                    width: parent.width - 40 * root.s - parent.spacing
                    spacing: Tokens.s1 * root.s
                    Text {
                        width: parent.width
                        text: I18n.tr("Set up the cobalt engine")
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: Tokens.fValue * root.s
                        wrapMode: Text.WordWrap
                    }
                    Text {
                        width: parent.width
                        text: {
                            switch (Stash.setupState) {
                            case "done": return I18n.tr("Ready. Downloads now run through your local cobalt.");
                            case "failed": return I18n.tr("Setup stopped. Nothing was left running.");
                            default: return I18n.tr("Cobalt runs locally in a container. This starts the runtime and downloads the image once — no reboot needed.");
                            }
                        }
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                        wrapMode: Text.WordWrap
                    }
                }
            }

            Column {
                width: parent.width
                spacing: Tokens.s2 * root.s

                Repeater {
                    model: Stash.setupSteps
                    delegate: Rectangle {
                        id: stepRow
                        required property string label
                        required property string stepState
                        required property string msg
                        width: parent.width
                        implicitHeight: Math.max(44 * root.s, stepText.implicitHeight + Tokens.s3 * root.s * 2)
                        radius: Tokens.radius * root.s
                        color: stepRow.stepState === "running" ? Tokens.tint10 : Tokens.tint5
                        border.width: Tokens.border
                        border.color: stepRow.stepState === "failed" ? Tokens.alert
                            : stepRow.stepState === "done" ? Tokens.sun
                            : Tokens.lineSoft

                        readonly property color tone: stepRow.stepState === "failed" ? Tokens.alert
                            : stepRow.stepState === "done" || stepRow.stepState === "running" ? Tokens.sun
                            : Tokens.inkFaint

                        Text {
                            font.family: "Material Symbols Rounded"
                            id: stepIcon
                            anchors.left: parent.left
                            anchors.leftMargin: Tokens.s3 * root.s
                            anchors.verticalCenter: parent.verticalCenter
                            text: stepRow.stepState === "done" ? "check_circle"
                                : stepRow.stepState === "failed" ? "error"
                                : stepRow.stepState === "running" ? "progress_activity"
                                : "radio_button_unchecked"
                            color: stepRow.tone
                            font.pixelSize: Tokens.fBody * root.s
                            RotationAnimation on rotation {
                                running: stepRow.stepState === "running" && !Tokens.reduceMotion
                                loops: Animation.Infinite
                                from: 0
                                to: 360
                                duration: 900
                            }
                        }

                        Column {
                            id: stepText
                            anchors.left: stepIcon.right
                            anchors.right: parent.right
                            anchors.leftMargin: Tokens.s3 * root.s
                            anchors.rightMargin: Tokens.s3 * root.s
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s1 * root.s
                            Text {
                                width: parent.width
                                text: I18n.tr(stepRow.label)
                                color: stepRow.tone
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall * root.s
                                font.weight: Font.Medium
                                wrapMode: Text.WordWrap
                            }
                            Text {
                                width: parent.width
                                visible: stepRow.msg.length > 0
                                text: stepRow.msg
                                color: stepRow.stepState === "failed" ? Tokens.alert : Tokens.inkMuted
                                font.family: Tokens.mono
                                font.pixelSize: Tokens.fTiny * root.s
                                wrapMode: Text.WrapAnywhere
                            }
                        }
                    }
                }
            }

            Row {
                anchors.right: parent.right
                spacing: Tokens.s2 * root.s

                Btn {
                    text: Stash.setupState === "done" ? I18n.tr("Done") : I18n.tr("Close")
                    armed: !root.busy
                    onAct: root.closed()
                }
                Btn {
                    visible: Stash.setupState === "failed"
                    text: I18n.tr("Retry")
                    primary: true
                    onAct: Stash.startSetup()
                }
                Btn {
                    visible: Stash.setupState === "idle"
                    text: I18n.tr("Start setup")
                    primary: true
                    onAct: Stash.startSetup()
                }
            }
        }
    }
}

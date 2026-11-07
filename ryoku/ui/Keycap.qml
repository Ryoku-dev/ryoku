pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Ryoku.Ui.Singletons

Item {
    id: keycap

    property string text: ""
    property bool dark: true
    property bool pressed: false
    property bool held: false
    property bool motionEnabled: true
    property int pulse: 0
    property bool ready: false
    // Per-monitor UI scale, threaded from KeyChord (default 1 = no-op).
    property real us: 1

    readonly property real capHeight: 42 * keycap.us
    readonly property bool depressed: held || pressed
    readonly property real faceTravel: depressed ? 5 * keycap.us : 0
    readonly property color faceColor: dark ? Tokens.keycapDark : Tokens.keycapLight
    readonly property color labelColor: dark ? Tokens.keycapOnDark : Tokens.keycapOnLight
    readonly property color lipColor: Qt.darker(faceColor, dark ? 1.9 : 1.22)

    implicitWidth: Math.max(capHeight, label.implicitWidth + 28 * keycap.us)
    implicitHeight: capHeight + 8 * keycap.us

    layer.enabled: true
    layer.smooth: true
    layer.effect: MultiEffect {
        shadowEnabled: true
        shadowColor: Qt.rgba(Tokens.keycapOnLight.r, Tokens.keycapOnLight.g, Tokens.keycapOnLight.b,
                             keycap.depressed ? 0.24 : 0.42)
        shadowBlur: keycap.depressed ? 0.32 : 0.58
        shadowVerticalOffset: keycap.depressed ? 2 : 6
        blurMax: 24
    }

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        y: 7 * keycap.us
        height: keycap.capHeight
        radius: (Tokens.radius + 2) * keycap.us
        border.width: Tokens.border * keycap.us
        border.color: Qt.rgba(keycap.labelColor.r, keycap.labelColor.g, keycap.labelColor.b, 0.16)
        gradient: Gradient {
            GradientStop {
                position: 0
                color: keycap.dark ? Qt.lighter(keycap.lipColor, 1.22) : Qt.lighter(keycap.lipColor, 1.04)
            }
            GradientStop { position: 0.46; color: keycap.lipColor }
            GradientStop {
                position: 1
                color: keycap.dark ? Qt.darker(keycap.lipColor, 1.28) : Qt.darker(keycap.lipColor, 1.08)
            }
        }
    }

    Rectangle {
        id: face
        anchors.left: parent.left
        anchors.right: parent.right
        y: keycap.faceTravel
        height: keycap.capHeight
        radius: (Tokens.radius + 2) * keycap.us
        border.width: Tokens.border * keycap.us
        border.color: Qt.rgba(keycap.labelColor.r, keycap.labelColor.g, keycap.labelColor.b, 0.18)
        clip: true
        gradient: Gradient {
            GradientStop {
                position: 0
                color: keycap.dark ? Qt.lighter(keycap.faceColor, 1.28) : Qt.lighter(keycap.faceColor, 1.04)
            }
            GradientStop { position: 0.62; color: keycap.faceColor }
            GradientStop {
                position: 1
                color: keycap.dark ? Qt.darker(keycap.faceColor, 1.16) : Qt.darker(keycap.faceColor, 1.04)
            }
        }

        Item {
            anchors.fill: parent
            visible: keycap.held && keycap.motionEnabled
            clip: true

            Rectangle {
                id: holdSheen

                width: Math.max(24 * keycap.us, face.width * 0.42)
                height: face.height * 1.8
                y: -face.height * 0.4
                rotation: 12
                opacity: keycap.dark ? 0.2 : 0.12
                gradient: Gradient {
                    orientation: Gradient.Horizontal
                    GradientStop { position: 0; color: "transparent" }
                    GradientStop {
                        position: 0.5
                        color: Qt.rgba(keycap.labelColor.r, keycap.labelColor.g, keycap.labelColor.b, 0.72)
                    }
                    GradientStop { position: 1; color: "transparent" }
                }

                NumberAnimation on x {
                    from: -holdSheen.width
                    to: face.width + holdSheen.width
                    duration: 1150
                    loops: Animation.Infinite
                    running: keycap.held && keycap.motionEnabled
                    easing.type: Easing.InOutSine
                }
            }
        }

        Rectangle {
            anchors.fill: parent
            anchors.margins: 1 * keycap.us
            radius: parent.radius - 1 * keycap.us
            color: "transparent"
            border.width: 2 * keycap.us
            border.color: Qt.rgba(keycap.labelColor.r, keycap.labelColor.g, keycap.labelColor.b, 0.34)
            opacity: keycap.held ? 1 : 0

            Behavior on opacity {
                NumberAnimation {
                    duration: keycap.motionEnabled ? Tokens.snap : 0
                    easing.type: Tokens.ease
                }
            }
        }

        Rectangle {
            anchors.fill: parent
            anchors.margins: 3 * keycap.us
            radius: parent.radius - 2 * keycap.us
            color: "transparent"
            border.width: Tokens.border * keycap.us
            border.color: Qt.rgba(keycap.labelColor.r, keycap.labelColor.g, keycap.labelColor.b, 0.08)
        }

        Text {
            id: label
            anchors.centerIn: parent
            text: keycap.text
            color: keycap.labelColor
            font.family: Tokens.ui
            font.pixelSize: 14 * keycap.us
            font.weight: Font.DemiBold
            font.letterSpacing: text.length <= 2 ? 0.25 * keycap.us : 0
        }

        Behavior on y {
            NumberAnimation {
                duration: keycap.motionEnabled ? Tokens.snap : 0
                easing.type: Tokens.ease
            }
        }
    }

    SequentialAnimation {
        id: pressFeedback

        PropertyAction { target: keycap; property: "pressed"; value: true }
        NumberAnimation {
            target: keycap
            property: "scale"
            to: 0.985
            duration: keycap.motionEnabled ? Tokens.snap / 2 : 0
            easing.type: Tokens.easeSnap
        }
        PauseAnimation { duration: keycap.motionEnabled ? Tokens.snap / 2 : 0 }
        PropertyAction { target: keycap; property: "pressed"; value: false }
        NumberAnimation {
            target: keycap
            property: "scale"
            to: 1
            duration: keycap.motionEnabled ? Tokens.flap : 0
            easing.type: Tokens.ease
        }
    }

    onPulseChanged: if (ready && motionEnabled) pressFeedback.restart()
    onMotionEnabledChanged: if (!motionEnabled) {
        pressFeedback.stop();
        pressed = false;
        scale = 1;
    }
    Component.onCompleted: ready = true
}

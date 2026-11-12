pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../../components"
import "look"

// One media transport button: a Material symbol in a borderless circle, or the
// filled primary disc. Passing symbolOn cross-fades and spins between the two.
Item {
    id: button

    property real s: 1
    property string symbol: ""
    property string symbolOn: ""
    property bool on: false
    property bool primary: false
    property bool available: true

    width: (button.primary ? 36 : 32) * button.s
    height: 32 * button.s
    opacity: button.available ? 1 : 0.32

    signal tapped()

    readonly property real disc: 28 * button.s
    readonly property real iconSize: 18 * button.s
    readonly property color ink: button.primary ? GameBarLook.accentInk : GameBarLook.fg

    Behavior on opacity {
        NumberAnimation { duration: Motion.dur(150) }
    }

    Rectangle {
        anchors.fill: parent
        radius: height / 2
        visible: !button.primary
        color: pointer.pressed ? GameBarLook.alpha(GameBarLook.fg, 0.12)
            : (pointer.containsMouse ? GameBarLook.hover : "transparent")

        Behavior on color {
            ColorAnimation { duration: Motion.dur(120) }
        }
    }

    Rectangle {
        id: disc
        anchors.centerIn: parent
        width: button.disc
        height: button.disc
        radius: height / 2
        visible: button.primary
        color: GameBarLook.accent
        scale: pointer.pressed ? 0.92 : 1

        Behavior on scale {
            NumberAnimation { duration: Motion.dur(110); easing.type: Easing.OutCubic }
        }

        Rectangle {
            anchors.fill: parent
            radius: height / 2
            color: GameBarLook.alpha(GameBarLook.accentInk, 0.10)
            opacity: pointer.containsMouse && !pointer.pressed ? 1 : 0

            Behavior on opacity {
                NumberAnimation { duration: Motion.dur(120) }
            }
        }
    }

    Item {
        anchors.centerIn: parent
        width: button.iconSize
        height: button.iconSize

        MaterialIcon {
            anchors.fill: parent
            text: button.symbol
            color: button.ink
            fill: button.primary ? 1 : 0
            font.pixelSize: button.iconSize
            opacity: button.symbolOn.length === 0 || !button.on ? 1 : 0
            rotation: button.symbolOn.length > 0 && button.on ? 90 : 0

            Behavior on opacity {
                NumberAnimation { duration: Motion.dur(160) }
            }
            Behavior on rotation {
                NumberAnimation { duration: Motion.dur(230); easing.type: Easing.OutCubic }
            }
        }

        MaterialIcon {
            anchors.fill: parent
            visible: button.symbolOn.length > 0
            text: button.symbolOn
            color: button.ink
            fill: button.primary ? 1 : 0
            font.pixelSize: button.iconSize
            opacity: button.on ? 1 : 0
            rotation: button.on ? 0 : -90

            Behavior on opacity {
                NumberAnimation { duration: Motion.dur(160) }
            }
            Behavior on rotation {
                NumberAnimation { duration: Motion.dur(230); easing.type: Easing.OutCubic }
            }
        }
    }

    MouseArea {
        id: pointer
        anchors.fill: parent
        hoverEnabled: true
        enabled: button.available
        cursorShape: Qt.PointingHandCursor
        onClicked: button.tapped()
    }
}

pragma ComponentBehavior: Bound

import QtQuick
import Qt5Compat.GraphicalEffects
import shell.services
import "../../../components"
import "../look"

// One voice participant: a round avatar with a speaking ring, a mute or deaf
// badge, and the display name under it. Everyone who is not speaking fades back
// so the active speaker reads at a glance.
Item {
    id: root

    required property var participant

    property real s: 1
    property real avatarSize: 46 * root.s
    property bool showName: true
    property bool dimOthers: true
    property real dimOpacity: 0.5

    readonly property bool active: root.participant && root.participant.speaking === true
    readonly property bool muted: root.participant && root.participant.mute === true
    readonly property bool deafened: root.participant && root.participant.deaf === true
    readonly property string displayName: {
        if (!root.participant)
            return "";
        return root.participant.nick || root.participant.username || "?";
    }
    readonly property string initial: root.displayName.length > 0
        ? root.displayName.charAt(0).toUpperCase() : "?"

    implicitWidth: root.showName ? Math.max(root.avatarSize, 58 * root.s) : root.avatarSize
    implicitHeight: root.avatarSize + (root.showName ? 15 * root.s : 0)
    opacity: root.dimOthers && !root.active && DiscordVoice.participantCount > 1
        ? root.dimOpacity : 1

    Behavior on opacity {
        NumberAnimation { duration: 140 }
    }

    Rectangle {
        id: ring
        anchors.horizontalCenter: parent.horizontalCenter
        y: 0
        width: root.avatarSize + 4 * root.s
        height: width
        radius: width / 2
        color: "transparent"
        border.width: root.active ? 2 * root.s : 0
        border.color: GameBarLook.accent

        Behavior on border.width {
            NumberAnimation { duration: 120 }
        }
    }

    Item {
        id: circle
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.top: parent.top
        width: root.avatarSize
        height: root.avatarSize

        Rectangle {
            anchors.fill: parent
            radius: width / 2
            color: GameBarLook.raised
        }

        Image {
            id: photo
            anchors.fill: parent
            source: DiscordVoice.avatarUrl(root.participant, 128)
            sourceSize.width: Math.ceil(root.avatarSize)
            sourceSize.height: Math.ceil(root.avatarSize)
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
            cache: true
            visible: false
            layer.enabled: true
        }

        Rectangle {
            id: photoMask
            anchors.fill: photo
            radius: height / 2
            visible: false
            layer.enabled: true
        }

        OpacityMask {
            anchors.fill: photo
            source: photo
            maskSource: photoMask
            visible: photo.status === Image.Ready
        }

        Text {
            anchors.centerIn: parent
            visible: photo.status !== Image.Ready
            text: root.initial
            color: GameBarLook.onLayer
            font.family: GameBarLook.face
            font.pixelSize: 17 * root.s
            font.weight: Font.DemiBold
        }

        Rectangle {
            visible: root.muted || root.deafened
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            width: 16 * root.s
            height: 16 * root.s
            radius: width / 2
            color: GameBarLook.danger

            GlyphIcon {
                anchors.centerIn: parent
                width: 10 * root.s
                height: 10 * root.s
                name: root.deafened ? "headphones-off" : "mic-off"
                color: GameBarLook.onDanger
                stroke: 2
            }
        }
    }

    Text {
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.top: circle.bottom
        anchors.topMargin: 2 * root.s
        width: root.width
        visible: root.showName
        text: root.displayName
        color: GameBarLook.onLayer
        font.family: GameBarLook.face
        font.pixelSize: GameBarLook.smallest * root.s
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideRight
    }
}

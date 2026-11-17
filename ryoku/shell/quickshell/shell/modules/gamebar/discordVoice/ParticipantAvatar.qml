pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import ".."
import "../../../components"
import Lithic
import "../look"

Item {
    id: root

    required property var participant

    property real avatarSize: 46
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
    readonly property real _scale: Lithic.scaleOf(root)
    readonly property string initial: root.displayName.length > 0
        ? root.displayName.charAt(0).toUpperCase() : "?"

    implicitWidth: root.showName ? Math.max(root.avatarSize, 58 * root._scale) : root.avatarSize
    implicitHeight: root.avatarSize + (root.showName
        ? Math.round(Lithic.type.caption.lineHeight * root._scale) : 0)
    opacity: root.dimOthers && !root.active && DiscordVoice.participantCount > 1
        ? root.dimOpacity : 1

    Behavior on opacity {
        NumberAnimation { duration: Lithic.duration(140) }
    }

    Item {
        id: plate
        anchors.horizontalCenter: parent.horizontalCenter
        width: root.avatarSize
        height: root.avatarSize

        CutRect {
            anchors.fill: parent
            anchors.margins: -3 * root._scale
            visible: root.active
            fill: "transparent"
            strokeColor: GameBarTone.select
            strokeWidth: 2 * root._scale
            cut: LithicTokens.cut.small + 3
        }

        GameBarPlate {
            anchors.fill: parent
            cut: LithicTokens.cut.small
            source: DiscordVoice.avatarUrl(root.participant, 128)
            placeholder: root.initial
            decodeSize: 128
        }

        CutRect {
            visible: root.muted || root.deafened
            width: 16 * root._scale
            height: 16 * root._scale
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            fill: GameBarTone.danger
            cut: LithicTokens.cut.token

            Glyph {
                anchors.centerIn: parent
                name: root.deafened ? "headphones" : "mic_off"
                size: 10
                ink: GameBarTone.dangerInk
            }
        }
    }

    Label {
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.top: plate.bottom
        anchors.topMargin: 1 * root._scale
        width: root.width
        visible: root.showName
        text: root.displayName
        typeRole: "caption"
        ink: root.active ? GameBarTone.ink : GameBarTone.ink2
        horizontalAlignment: Text.AlignHCenter
    }
}

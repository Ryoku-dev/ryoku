pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import ".."
import "../../../components"
import Lithic
import "../look"
import "layout.js" as VoiceLayout

Item {
    id: root

    required property var participant

    property real avatarSize: 46
    property bool showName: true
    property string namePosition: "below"
    property real nameGap: 1
    property int nameChars: 12
    property real nameShift: 0
    property bool nameShiftMirrored: false
    property bool dimOthers: true
    property real dimOpacity: 0.5

    readonly property bool active: root.participant && root.participant.speaking === true
    readonly property bool muted: root.participant && root.participant.mute === true
    readonly property bool deafened: root.participant && root.participant.deaf === true
    readonly property string displayName: {
        if (!root.participant)
            return "";
        const full = root.participant.nick || root.participant.username || "?";
        if (full.length <= root.nameChars)
            return full;
        return full.slice(0, root.nameChars) + "...";
    }
    readonly property real _scale: Lithic.scaleOf(root)
    readonly property string initial: root.displayName.length > 0
        ? root.displayName.charAt(0).toUpperCase() : "?"
    readonly property bool beside: root.showName && VoiceLayout.beside(root.namePosition)
    readonly property real gap: root.nameGap * root._scale
    readonly property bool nameOnLeft: root.beside && root.namePosition === "left"
    // A left-side name gives up width to the gap, so the avatar never shifts.
    readonly property real nameWidth: (VoiceLayout.nameWidth(root.nameChars)
        - (root.nameOnLeft ? root.nameGap : 0)) * root._scale

    implicitWidth: root.beside ? root.avatarSize + root.nameWidth + (root.nameOnLeft ? 0 : root.gap)
        : root.showName ? Math.max(root.avatarSize, 58 * root._scale) : root.avatarSize
    implicitHeight: root.beside ? root.avatarSize
        : root.avatarSize + (root.showName
            ? Math.round(Lithic.type.caption.lineHeight * root._scale) + root.gap : 0)
    opacity: root.dimOthers && !root.active && DiscordVoice.participantCount > 1
        ? root.dimOpacity : 1

    Behavior on opacity {
        NumberAnimation { duration: Lithic.duration(140) }
    }

    Item {
        id: plate
        x: root.beside ? (root.nameOnLeft ? root.nameWidth + root.gap : 0)
            : (root.width - root.avatarSize) / 2
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
        id: label
        x: root.beside ? (root.namePosition === "left" ? 0 : root.avatarSize + root.gap)
            : (root.nameShiftMirrored ? 0 : root.nameShift)
        y: root.beside ? (root.avatarSize - label.implicitHeight) / 2
            : plate.y + plate.height + root.gap
        width: root.beside ? root.nameWidth : root.width - root.nameShift
        visible: root.showName
        text: root.displayName
        typeRole: "caption"
        ink: root.active ? GameBarTone.ink : GameBarTone.ink2
        horizontalAlignment: root.beside
            ? (root.namePosition === "left" ? Text.AlignRight : Text.AlignLeft)
            : Text.AlignHCenter
    }
}

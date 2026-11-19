pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import ".."
import "../../../components"
import Ryoku.Ui.Singletons
import Lithic
import "../look"
import "../discordVoice"
import "../discordVoice/layout.js" as VoiceLayout

GameBarCard {
    id: card

    title: I18n.tr("Voice")
    resizable: true
    // Set by the host to the number of avatars that fit below the strip.
    property int visibleAvatars: 8

    readonly property var voice: DiscordVoice
    readonly property int avatarLimit: Math.max(1, Math.min(8, Config.gamebar.voiceAvatars || 8))
    readonly property bool dimOthers: Config.gamebar.voiceDim !== false
    readonly property real dimOpacity: Math.max(0.2, Math.min(1, Config.gamebar.voiceDimOpacity || 0.5))
    readonly property bool vertical: Config.gamebar.voiceVertical !== false
    // Off-centre alignment only fits a vertical stack; a horizontal row is
    // always centered so the avatars stay inside the card.
    readonly property string align: !card.vertical ? "center"
        : ["left", "right"].indexOf(Config.gamebar.voiceAlign) >= 0
            ? Config.gamebar.voiceAlign : "left"
    readonly property string configuredNamePosition: ["below", "left", "right"].indexOf(Config.gamebar.voiceNamePosition) >= 0
        ? Config.gamebar.voiceNamePosition : "below"
    // A name on the same side as the alignment would point out of the widget:
    // mirror it. Below is always allowed; horizontal always uses Below.
    readonly property string namePosition: !card.vertical ? "below"
        : (card.align === "left" && card.configuredNamePosition === "left") ? "right"
        : (card.align === "right" && card.configuredNamePosition === "right") ? "left"
        : card.configuredNamePosition
    readonly property int nameGap: typeof Config.gamebar.voiceNameGap === "number"
        ? Math.max(0, Math.min(16, Config.gamebar.voiceNameGap)) : 1
    readonly property int nameChars: typeof Config.gamebar.voiceNameChars === "number"
        ? Math.max(4, Math.min(32, Config.gamebar.voiceNameChars)) : 12
    readonly property real avatarSpacing: 4 * card.s
    readonly property real avatarCell: (VoiceLayout.beside(card.namePosition)
        ? 46 + VoiceLayout.nameWidth(card.nameChars) + (card.namePosition === "right" ? card.nameGap : 0)
        : Math.max(58, VoiceLayout.nameWidth(card.nameChars))) * card.s
    readonly property real avatarSize: 46 * card.s
    // Where the avatar sits inside its cell, for each name position.
    readonly property real avatarOffset: card.namePosition === "right" ? 0
        : card.namePosition === "left" ? VoiceLayout.nameWidth(card.nameChars) * card.s
        : (card.avatarCell - card.avatarSize) / 2
    readonly property int avatarColumns: card.vertical ? 1 : Math.max(1, Math.min(8,
        Math.floor((stage.width + card.avatarSpacing) / (card.avatarCell + card.avatarSpacing))))

    readonly property real miniAvatarLogical: VoiceLayout.avatar(Config.gamebar.voiceAvatarSize)
    readonly property bool miniNames: Config.gamebar.voiceNames !== false
    readonly property bool miniControls: Config.gamebar.voiceControls !== false
    readonly property real miniBgOpacity: {
        const value = Config.gamebar.voiceBgOpacity;
        return typeof value === "number" ? Math.max(0, Math.min(1, value)) : 0.72;
    }
    readonly property real miniPad: VoiceLayout.PAD * card.s
    readonly property real miniGap: VoiceLayout.SPACE * card.s
    readonly property real miniHeaderHeight: VoiceLayout.HEADER * card.s
    readonly property real miniCell: VoiceLayout.cell(card.miniAvatarLogical, card.miniNames, card.namePosition, card.nameGap, card.nameChars) * card.s
    // The pinned header's Discord tile starts at miniPad; line the first
    // avatar up with it when the grid is on the left.
    readonly property real miniPlateOffset: card.namePosition === "right" ? 0
        : card.namePosition === "left" ? VoiceLayout.nameWidth(card.nameChars) * card.s
        : (card.miniCell - card.miniAvatarLogical * card.s) / 2
    // The avatar grid follows Avatar alignment, never the header mirror.
    readonly property real miniLogoCenter: card.miniPad + 20 * card.s
    readonly property real miniLeftX: Math.max(card.miniPad - card.miniPlateOffset,
        card.miniLogoCenter - card.miniAvatarLogical * card.s / 2 - card.miniPlateOffset)
    readonly property real miniNameShift: Math.max(0, -card.miniLeftX) + card.miniPad
    // Right mirrors left around the avatar, not around the (wider) name cell.
    readonly property real miniRightX: card.width - card.miniPad
        - card.miniAvatarLogical * card.s - card.miniPlateOffset
    readonly property int miniColumns: VoiceLayout.columns(card.width / card.s,
        Math.min(card.avatarLimit, card.visibleAvatars, card.voice.participantCount),
        card.miniAvatarLogical, card.miniNames, card.vertical, card.namePosition, card.nameGap, card.nameChars)
    readonly property string miniSubtitle: {
        if (card.voice.inVoice) {
            const n = card.voice.participantCount;
            return n === 1 ? I18n.tr("1 participant")
                : I18n.tr("%1 participants").arg(n);
        }
        if (card.voice.status === "authenticated")
            return I18n.tr("Not in a call");
        return card.subtitle;
    }

    readonly property string headline: {
        if (card.voice.inVoice)
            return card.voice.channel.name || I18n.tr("Voice channel");
        switch (card.voice.status) {
        case "auth_required": return I18n.tr("Discord voice");
        case "authorizing": return I18n.tr("Waiting for Discord");
        case "unavailable": return I18n.tr("Discord unavailable");
        case "disconnected": return I18n.tr("Reconnecting");
        case "stopped": return I18n.tr("Bridge stopped");
        case "authenticated": return I18n.tr("No voice channel");
        default: return I18n.tr("Connecting");
        }
    }

    readonly property string subtitle: {
        if (card.voice.inVoice) {
            const n = card.voice.participantCount;
            const line = n === 1 ? I18n.tr("1 person in voice") : I18n.tr("%1 people in voice").arg(n);
            return card.voice.backendLabel.length > 0
                ? line + "  " + card.voice.backendLabel : line;
        }
        if (card.voice.errorMessage.length > 0)
            return card.voice.errorMessage;
        if (card.voice.status === "authenticated")
            return I18n.tr("Join a channel to see who is talking");
        return card.voice.backendLabel.length > 0
            ? I18n.tr("via %1").arg(card.voice.backendLabel)
            : I18n.tr("Discord voice overlay");
    }

    readonly property string emptyMessage: {
        switch (card.voice.status) {
        case "auth_required":
            return card.voice.companionNeeded
                ? I18n.tr("Vesktop needs its voice companion before Ryoku can read the call.")
                : I18n.tr("Allow Ryoku to read and control your Discord voice state.");
        case "authorizing": return I18n.tr("Confirm the request in Discord.");
        case "unavailable": return I18n.tr("Discord's voice socket is not reachable. Retry in a moment.");
        case "disconnected": return I18n.tr("Discord went away. Reconnecting in a moment.");
        case "stopped": return I18n.tr("The bridge stopped. Retry to start it again.");
        case "authenticated": return I18n.tr("Join a voice channel to see who is talking.");
        default: return I18n.tr("Connecting to Discord.");
        }
    }

    Item {
        anchors.fill: parent
        visible: !card.minimal

        Item {
            id: header
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            height: 56 * card.s

            CutRect {
                id: glyphTile
                anchors.left: parent.left
                anchors.leftMargin: card.space3
                anchors.verticalCenter: parent.verticalCenter
                width: 40 * card.s
                height: 40 * card.s
                fill: card.voice.inVoice ? GameBarTone.select : GameBarTone.card
                strokeColor: GameBarTone.cardStroke
                strokeWidth: LithicTokens.size.hairline
                cut: GameBarTone.cardCut

                GameBarGlyph {
                    anchors.centerIn: parent
                    brand: "discord"
                    size: 20
                    ink: card.voice.inVoice ? GameBarTone.selectInk : GameBarTone.ink2
                }
            }

            Column {
                anchors.left: glyphTile.right
                anchors.leftMargin: card.space3
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 1 * card.s

                Label {
                    width: parent.width
                    text: card.headline
                    typeRole: "title"
                }
                Label {
                    width: parent.width
                    text: card.subtitle
                    typeRole: "detail"
                    inkRole: "fg2"
                }
            }
        }

        Item {
            id: stage
            anchors.top: header.bottom
            anchors.topMargin: card.space2
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: footer.top
            anchors.bottomMargin: card.space1

            Grid {
                anchors.verticalCenter: parent.verticalCenter
                x: card.align === "left" ? card.pad + card.avatarSpacing
                    : card.align === "right" ? Math.max(card.pad, parent.width - card.pad - card.avatarSpacing
                        - card.avatarSize - card.avatarOffset)
                    : (parent.width - implicitWidth) / 2
                columns: card.avatarColumns
                spacing: card.avatarSpacing
                visible: card.voice.participantCount > 0

                Repeater {
                    model: Math.min(card.avatarLimit, card.visibleAvatars, card.voice.participantCount)

                    delegate: ParticipantAvatar {
                        required property int index
                        participant: (card.voice.participantRevision, card.voice.participantAt(index))
                        width: card.avatarCell
                        avatarSize: card.avatarSize
                        namePosition: card.namePosition
                        nameGap: card.nameGap
                        nameChars: card.nameChars
                        dimOthers: card.dimOthers
                        dimOpacity: card.dimOpacity
                    }
                }
            }

            Column {
                anchors.centerIn: parent
                width: parent.width
                spacing: card.space1 + 2 * card.s
                visible: card.voice.participantCount === 0

                Item {
                    width: parent.width
                    height: 56 * card.s

                    CutRect {
                        anchors.horizontalCenter: parent.horizontalCenter
                        width: 56 * card.s
                        height: 56 * card.s
                        fill: "transparent"
                        strokeColor: GameBarTone.line
                        strokeWidth: LithicTokens.size.hairline
                        cut: GameBarTone.cardCut

                        GameBarGlyph {
                            anchors.centerIn: parent
                            brand: "discord"
                            size: 22
                            ink: GameBarTone.ink3
                        }
                    }
                }

                Label {
                    width: parent.width
                    text: card.emptyMessage
                    typeRole: "detail"
                    inkRole: "fg2"
                    wrapMode: Text.WordWrap
                    horizontalAlignment: Text.AlignHCenter
                }

                Item {
                    width: parent.width
                    height: visible ? authPlaque.implicitHeight : 0
                    visible: card.voice.status !== "authenticated"

                    Plaque {
                        id: authPlaque
                        anchors.horizontalCenter: parent.horizontalCenter
                        kind: card.voice.status === "auth_required" || card.voice.status === "authorizing" ? "gold" : "line"
                        text: card.voice.status === "authorizing"
                            ? I18n.tr("Waiting for Discord")
                            : (card.voice.status === "auth_required"
                                ? I18n.tr("Authorize Discord")
                                : I18n.tr("Retry"))
                        enabled: card.voice.status !== "authorizing"
                        onClicked: {
                            if (card.voice.status === "auth_required")
                                card.voice.authorize();
                            else
                                card.voice.connect();
                        }
                    }
                }

                Item {
                    width: parent.width
                    height: visible ? companionPlaque.implicitHeight : 0
                    visible: card.voice.companionNeeded && !card.voice.installerBusy

                    Plaque {
                        id: companionPlaque
                        anchors.horizontalCenter: parent.horizontalCenter
                        small: true
                        kind: "quiet"
                        text: I18n.tr("Install voice companion")
                        onClicked: card.voice.installCompanion()
                    }
                }

                Label {
                    width: parent.width
                    visible: card.voice.installerBusy || card.voice.installerState === "failed"
                    text: card.voice.installerBusy ? card.voice.installerLine
                        : I18n.tr("The companion install did not finish")
                    typeRole: "caption"
                    inkRole: "fg3"
                    horizontalAlignment: Text.AlignHCenter
                }
            }
        }

        Item {
            id: footer
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: visible ? 56 * card.s : 0
            visible: card.voice.authenticated && card.voice.status !== "authorizing"

            Row {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                spacing: 6 * card.s

                IconPlaque {
                    small: true
                    kind: card.voice.muted ? "tonal" : "quiet"
                    glyph: card.voice.muted ? "mic_off" : "mic"
                    Accessible.name: I18n.tr("Microphone")
                    onClicked: card.voice.setMuted(!card.voice.muted)
                }

                IconPlaque {
                    small: true
                    kind: card.voice.deafened ? "tonal" : "quiet"
                    glyph: card.voice.deafened ? "headphones" : "headphones"
                    Accessible.name: I18n.tr("Headphones")
                    onClicked: card.voice.setDeafened(!card.voice.deafened)
                }
            }

            Label {
                anchors.right: parent.right
                anchors.rightMargin: card.resizable ? 22 * card.s : 0
                anchors.verticalCenter: parent.verticalCenter
                text: card.voice.deafened ? I18n.tr("Deafened")
                    : (card.voice.muted ? I18n.tr("Muted") : I18n.tr("Mic on"))
                typeRole: "detail"
                inkRole: "fg2"
            }
        }
    }

    Item {
        id: mini
        anchors.fill: parent
        visible: card.minimal

        CutRect {
            anchors.fill: parent
            fill: Lithic.alpha(GameBarTone.pane, card.miniBgOpacity)
            strokeColor: GameBarTone.paneStroke
            strokeWidth: LithicTokens.size.hairline
            cut: GameBarTone.paneCut
        }

        Item {
            id: miniHeader
            x: card.miniPad
            y: card.miniPad
            width: parent.width - card.miniPad * 2
            height: card.miniHeaderHeight

            CutRect {
                id: miniTile
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                width: 40 * card.s
                height: 40 * card.s
                fill: card.voice.inVoice ? GameBarTone.select : GameBarTone.card
                strokeColor: GameBarTone.cardStroke
                strokeWidth: LithicTokens.size.hairline
                cut: GameBarTone.cardCut

                GameBarGlyph {
                    anchors.centerIn: parent
                    brand: "discord"
                    size: 20
                    ink: card.voice.inVoice ? GameBarTone.selectInk : GameBarTone.ink2
                }
            }

            Row {
                id: miniButtons
                readonly property bool active: card.miniControls && card.voice.authenticated
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 4 * card.s
                visible: active
                width: active ? implicitWidth : 0

                IconPlaque {
                    small: true
                    kind: card.voice.muted ? "tonal" : "quiet"
                    glyph: card.voice.muted ? "mic_off" : "mic"
                    Accessible.name: I18n.tr("Microphone")
                    onClicked: card.voice.setMuted(!card.voice.muted)
                }

                IconPlaque {
                    small: true
                    kind: card.voice.deafened ? "tonal" : "quiet"
                    glyph: "headphones"
                    Accessible.name: I18n.tr("Headphones")
                    onClicked: card.voice.setDeafened(!card.voice.deafened)
                }
            }

            Column {
                anchors.left: miniTile.right
                anchors.leftMargin: card.space3
                anchors.right: miniButtons.left
                anchors.rightMargin: 8 * card.s
                anchors.verticalCenter: parent.verticalCenter
                spacing: 0

                Label {
                    width: parent.width
                    text: card.headline
                    typeRole: "title"
                }
                Label {
                    width: parent.width
                    text: card.miniSubtitle
                    typeRole: "detail"
                    inkRole: "fg2"
                }
            }
        }

        Grid {
            id: miniGrid
            x: card.align === "left"
                ? (card.namePosition === "left" ? Math.max(0, card.miniLeftX) : card.miniLeftX)
                : card.align === "right" ? Math.max(card.miniPad, card.miniRightX)
                : (parent.width - implicitWidth) / 2
            y: card.miniPad + card.miniHeaderHeight + card.miniGap
            columns: card.miniColumns
            rowSpacing: card.miniGap
            columnSpacing: card.miniGap
            visible: card.voice.participantCount > 0

            Repeater {
                model: Math.min(card.avatarLimit, card.visibleAvatars, card.voice.participantCount)

                delegate: ParticipantAvatar {
                    required property int index
                    participant: (card.voice.participantRevision, card.voice.participantAt(index))
                    width: card.miniCell
                    // layout.js sizes the panel in logical units that GameBar
                    // scales by s, so the delegate must use that scale too or
                    // stacked rows overrun the panel.
                    avatarSize: card.miniAvatarLogical * card.s
                    showName: card.miniNames
                    namePosition: card.namePosition
                    nameGap: card.nameGap
                    nameChars: card.nameChars
                    nameShift: (card.align === "left" || card.align === "right")
                        && card.namePosition === "below" ? card.miniNameShift : 0
                    nameShiftMirrored: card.align === "right" && card.namePosition === "below"
                    dimOthers: card.dimOthers
                    dimOpacity: card.dimOpacity
                }
            }
        }
    }
}

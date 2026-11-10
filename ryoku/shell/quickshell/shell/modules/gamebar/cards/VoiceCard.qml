pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import ".."
import "../../../components"
import Ryoku.Ui.Singletons
import "../look"
import "../discordVoice"
import "../discordVoice/layout.js" as VoiceLayout

// Voice: the Discord channel the user is in, who is there, and who is talking.
// The bridge reads Discord's own RPC or the Ryoku companion inside Vesktop.
GameBarCard {
    id: card

    title: I18n.tr("Voice")
    resizable: true

    readonly property var voice: DiscordVoice
    readonly property int avatarLimit: Math.max(1, Math.min(8, Config.gamebar.voiceAvatars || 8))
    readonly property bool dimOthers: Config.gamebar.voiceDim !== false
    readonly property real dimOpacity: Math.max(0.2, Math.min(1, Config.gamebar.voiceDimOpacity || 0.5))
    readonly property real avatarSpacing: 4 * card.s
    readonly property real avatarCell: 58 * card.s
    readonly property int avatarColumns: Math.max(1, Math.min(8,
        Math.floor((stage.width + card.avatarSpacing) / (card.avatarCell + card.avatarSpacing))))

    readonly property real miniAvatarLogical: VoiceLayout.avatar(Config.gamebar.voiceAvatarSize)
    readonly property real miniAvatarSize: card.miniAvatarLogical * card.s
    readonly property bool miniNames: Config.gamebar.voiceNames !== false
    readonly property bool miniControls: Config.gamebar.voiceControls !== false
    readonly property real miniBgOpacity: {
        const value = Config.gamebar.voiceBgOpacity;
        return typeof value === "number" ? Math.max(0, Math.min(1, value)) : 0.72;
    }
    readonly property real miniPad: VoiceLayout.PAD * card.s
    readonly property real miniGap: VoiceLayout.SPACE * card.s
    readonly property real miniHeaderHeight: VoiceLayout.HEADER * card.s
    readonly property real miniCell: VoiceLayout.cell(card.miniAvatarLogical, card.miniNames) * card.s
    readonly property int miniColumns: VoiceLayout.columns(card.width / card.s,
        Math.min(card.avatarLimit, card.voice.participantCount),
        card.miniAvatarLogical, card.miniNames)
    readonly property string miniSubtitle: {
        if (card.voice.inVoice) {
            const n = card.voice.participantCount;
            return n === 1 ? I18n.tr("1 participant")
                : I18n.tr("%1 participants").arg(n);
        }
        if (card.voice.status === "authenticated")
            return I18n.tr("No voice channel");
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
            height: 40 * card.s

            Rectangle {
                id: glyphTile
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                width: 36 * card.s
                height: 36 * card.s
                radius: GameBarLook.radiusSmall * card.s
                color: card.voice.inVoice ? GameBarLook.toggled : GameBarLook.raised

                GlyphIcon {
                    anchors.centerIn: parent
                    width: 19 * card.s
                    height: 19 * card.s
                    name: "discord"
                    color: card.voice.inVoice ? GameBarLook.toggledInk : GameBarLook.onLayer
                    stroke: 1.7
                }
            }

            Column {
                anchors.left: glyphTile.right
                anchors.leftMargin: 10 * card.s
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 1 * card.s

                Text {
                    width: parent.width
                    textFormat: Text.PlainText
                    text: card.headline
                    color: GameBarLook.fg
                    font.family: GameBarLook.face
                    font.pixelSize: GameBarLook.small * card.s
                    font.weight: Font.DemiBold
                    elide: Text.ElideRight
                }

                Text {
                    width: parent.width
                    text: card.subtitle
                    color: GameBarLook.subtext
                    font.family: GameBarLook.face
                    font.pixelSize: GameBarLook.smaller * card.s
                    elide: Text.ElideRight
                }
            }
        }

        Item {
            id: stage
            anchors.top: header.bottom
            anchors.topMargin: 6 * card.s
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: footer.top
            anchors.bottomMargin: 4 * card.s

            Grid {
                anchors.centerIn: parent
                columns: card.avatarColumns
                spacing: card.avatarSpacing
                visible: card.voice.participantCount > 0

                Repeater {
                    model: Math.min(card.avatarLimit, card.voice.participantCount)

                    delegate: ParticipantAvatar {
                        required property int index
                        participant: (card.voice.participantRevision, card.voice.participantAt(index))
                        s: card.s
                        dimOthers: card.dimOthers
                        dimOpacity: card.dimOpacity
                    }
                }
            }

            Column {
                anchors.centerIn: parent
                width: parent.width
                spacing: 8 * card.s
                visible: card.voice.participantCount === 0

                GlyphIcon {
                    anchors.horizontalCenter: parent.horizontalCenter
                    width: 26 * card.s
                    height: 26 * card.s
                    name: "discord"
                    color: GameBarLook.subtext
                    stroke: 1.6
                }

                Text {
                    width: parent.width
                    text: card.emptyMessage
                    color: GameBarLook.onLayer
                    font.family: GameBarLook.face
                    font.pixelSize: GameBarLook.smaller * card.s
                    horizontalAlignment: Text.AlignHCenter
                    wrapMode: Text.WordWrap
                }

                GameBarTextButton {
                    anchors.horizontalCenter: parent.horizontalCenter
                    // A healthy authenticated bridge is not a reconnect case:
                    // the only next step is to join a channel in Discord.
                    visible: card.voice.status !== "authenticated"
                    s: card.s
                    compact: true
                    primary: card.voice.status === "auth_required" || card.voice.status === "authorizing"
                    label: card.voice.status === "authorizing"
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

                GameBarTextButton {
                    anchors.horizontalCenter: parent.horizontalCenter
                    visible: card.voice.companionNeeded && !card.voice.installerBusy
                    s: card.s
                    compact: true
                    quiet: true
                    label: I18n.tr("Install voice companion")
                    onClicked: card.voice.installCompanion()
                }

                Text {
                    width: parent.width
                    visible: card.voice.installerBusy || card.voice.installerState === "failed"
                    textFormat: Text.PlainText
                    text: card.voice.installerBusy ? card.voice.installerLine
                        : I18n.tr("The companion install did not finish")
                    color: GameBarLook.subtext
                    font.family: GameBarLook.face
                    font.pixelSize: GameBarLook.smallest * card.s
                    horizontalAlignment: Text.AlignHCenter
                    elide: Text.ElideRight
                }
            }
        }

        Item {
            id: footer
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: visible ? 34 * card.s : 0
            visible: card.voice.authenticated && card.voice.status !== "authorizing"

            Row {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                spacing: 6 * card.s

                GameBarVoiceToggle {
                    s: card.s
                    glyph: card.voice.muted ? "mic-off" : "mic"
                    on: card.voice.muted
                    onClicked: card.voice.setMuted(!card.voice.muted)
                }

                GameBarVoiceToggle {
                    s: card.s
                    glyph: card.voice.deafened ? "headphones-off" : "headphones"
                    on: card.voice.deafened
                    onClicked: card.voice.setDeafened(!card.voice.deafened)
                }
            }

            Text {
                anchors.right: parent.right
                anchors.rightMargin: card.resizable ? 16 * card.s : 0
                anchors.verticalCenter: parent.verticalCenter
                text: card.voice.deafened ? I18n.tr("Deafened")
                    : (card.voice.muted ? I18n.tr("Muted") : I18n.tr("Mic on"))
                color: GameBarLook.subtext
                font.family: GameBarLook.face
                font.pixelSize: GameBarLook.smaller * card.s
            }
        }
    }

    // Pinned with the bar closed, the card becomes the overlay: a rounded
    // translucent panel sized to its content, with the channel, mute and deafen
    // state, and the people on an avatar grid.
    Rectangle {
        id: mini
        anchors.fill: parent
        visible: card.minimal
        radius: GameBarLook.radiusVeryLarge * card.s
        color: Qt.rgba(GameBarLook.panel.r, GameBarLook.panel.g, GameBarLook.panel.b,
            card.miniBgOpacity)

        Item {
            id: miniHeader
            x: card.miniPad
            y: card.miniPad
            width: parent.width - card.miniPad * 2
            height: card.miniHeaderHeight

            Rectangle {
                id: miniTile
                anchors.verticalCenter: parent.verticalCenter
                width: 36 * card.s
                height: 36 * card.s
                radius: GameBarLook.radiusSmall * card.s
                color: card.voice.inVoice ? GameBarLook.toggled : GameBarLook.raised

                GlyphIcon {
                    anchors.centerIn: parent
                    width: 20 * card.s
                    height: 20 * card.s
                    name: "discord"
                    color: card.voice.inVoice ? GameBarLook.toggledInk : GameBarLook.onLayer
                    stroke: 1.7
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

                GameBarVoiceToggle {
                    diameter: 40 * card.s
                    glyph: card.voice.muted ? "mic-off" : "mic"
                    on: card.voice.muted
                    onClicked: card.voice.setMuted(!card.voice.muted)
                }

                GameBarVoiceToggle {
                    diameter: 40 * card.s
                    glyph: card.voice.deafened ? "headphones-off" : "headphones"
                    on: card.voice.deafened
                    onClicked: card.voice.setDeafened(!card.voice.deafened)
                }
            }

            Column {
                anchors.left: miniTile.right
                anchors.leftMargin: 9 * card.s
                anchors.right: miniButtons.left
                anchors.rightMargin: 8 * card.s
                anchors.verticalCenter: parent.verticalCenter
                spacing: 0

                Text {
                    width: parent.width
                    textFormat: Text.PlainText
                    text: card.headline
                    color: GameBarLook.fg
                    font.family: GameBarLook.face
                    font.pixelSize: GameBarLook.small * card.s
                    font.weight: Font.DemiBold
                    elide: Text.ElideRight
                }

                Text {
                    width: parent.width
                    text: card.miniSubtitle
                    color: GameBarLook.subtext
                    font.family: GameBarLook.face
                    font.pixelSize: GameBarLook.smaller * card.s
                    elide: Text.ElideRight
                }
            }
        }

        Grid {
            id: miniGrid
            anchors.horizontalCenter: parent.horizontalCenter
            y: card.miniPad + card.miniHeaderHeight + VoiceLayout.GAP * card.s
            columns: card.miniColumns
            rowSpacing: card.miniGap
            columnSpacing: card.miniGap
            visible: card.voice.participantCount > 0

            Repeater {
                model: Math.min(card.avatarLimit, card.voice.participantCount)

                delegate: ParticipantAvatar {
                    required property int index
                    participant: (card.voice.participantRevision, card.voice.participantAt(index))
                    width: card.miniCell
                    s: card.s
                    avatarSize: card.miniAvatarSize
                    showName: card.miniNames
                    dimOthers: card.dimOthers
                    dimOpacity: card.dimOpacity
                }
            }
        }
    }
}

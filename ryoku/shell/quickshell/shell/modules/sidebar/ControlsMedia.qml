pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Widgets
import Ryoku.Ui.Singletons
import shell.services

Item {
    id: root

    required property real s
    required property bool active

    readonly property var player: active ? Media.player : null
    readonly property bool showArtwork: SidebarState.elementVisible("mediaArtwork")
    readonly property bool showTrack: SidebarState.elementVisible("mediaTrack")
    readonly property bool showTransport: SidebarState.elementVisible("mediaTransport")
    readonly property string artists: root.player
        ? Theme.joinArtists(root.player.trackArtists, root.player.trackArtist) : ""

    implicitHeight: (Tokens.rowH + Tokens.s5) * s

    Rectangle {
        anchors.fill: parent
        radius: Tokens.radius * root.s * 1.5
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineSoft

        ClippingRectangle {
            id: art
            visible: root.showArtwork
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s3 * root.s
            anchors.verticalCenter: parent.verticalCenter
            width: Tokens.s7 * root.s
            height: width
            radius: Tokens.radius * root.s
            color: Tokens.tint10

            Image {
                id: cover
                anchors.fill: parent
                source: root.player ? (root.player.trackArtUrl || "") : ""
                sourceSize.width: Math.round(parent.width * 2)
                sourceSize.height: Math.round(parent.height * 2)
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
                cache: true
            }
            Text {
                anchors.centerIn: parent
                visible: cover.status !== Image.Ready
                text: "music_note"
                color: Tokens.inkMuted
                font.family: "Material Symbols Rounded"
                font.pixelSize: Tokens.fValue * root.s
            }
        }

        Column {
            visible: root.showTrack
            anchors.left: art.visible ? art.right : parent.left
            anchors.leftMargin: Tokens.s3 * root.s
            anchors.right: transport.visible ? transport.left : parent.right
            anchors.rightMargin: Tokens.s3 * root.s
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s1 * root.s

            Text {
                width: parent.width
                text: root.player && root.player.trackTitle ? root.player.trackTitle : I18n.tr("Nothing playing")
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fRow * root.s
                font.weight: Font.DemiBold
                elide: Text.ElideRight
            }
            Text {
                width: parent.width
                text: root.artists !== "" ? root.artists : I18n.tr("Media controls appear when a player is available")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall * root.s
                elide: Text.ElideRight
            }
        }

        Row {
            id: transport
            visible: root.showTransport
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s3 * root.s
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s1 * root.s

            CornerButton {
                s: root.s
                glyph: "skip_previous"
                enabled: root.player !== null && root.player.canGoPrevious
                subtle: true
                Accessible.name: I18n.tr("Previous")
                onClicked: if (root.player) root.player.previous()
            }
            CornerButton {
                s: root.s
                glyph: Media.playing ? "pause" : "play_arrow"
                enabled: root.player !== null
                subtle: false
                Accessible.name: Media.playing ? I18n.tr("Pause") : I18n.tr("Play")
                onClicked: Media.toggle()
            }
            CornerButton {
                s: root.s
                glyph: "skip_next"
                enabled: root.player !== null && root.player.canGoNext
                subtle: true
                Accessible.name: I18n.tr("Next")
                onClicked: if (root.player) root.player.next()
            }
        }
    }
}

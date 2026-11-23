pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons
import "../kit" as K

// An app in detail: its description, its desktop actions as tracks, the windows
// it has open, and when it is the player that is playing, a now-playing strip.
LeadGeneric {
    id: root

    readonly property var entry: root.engine && root.row ? root.engine.appEntry(root.row) : null
    readonly property bool playingHere: root.engine !== null && root.row !== null
        && root.engine.isPlayingApp(root.row) === true
    readonly property var player: root.engine && root.engine.player ? root.engine.player : Media.player

    description: root.entry && root.entry.comment ? String(root.entry.comment) : ""

    Column {
        visible: root.lead !== null && root.lead.windowItems.length > 0
        width: root.width
        spacing: K.Style.s1

        K.Label {
            width: parent.width
            text: I18n.tr("Open windows")
            role: "micro"
        }

        ActionList {
            width: parent.width
            numbered: false
            items: root.lead ? root.lead.windowItems : []
            focusIndex: root.lead ? root.lead.windowFocus : -1
            scroller: root.lead ? root.lead.scroller : null
            onActivated: index => root.lead.runAt(root.lead.actions.length + index)
        }
    }

    K.Plate {
        id: strip
        visible: root.playingHere && root.player !== null
        width: root.width
        height: 64

        K.CoverArt {
            id: cover
            anchors.left: parent.left
            anchors.leftMargin: K.Style.s2
            anchors.verticalCenter: parent.verticalCenter
            size: 48
            source: strip.visible ? Music.coverFor(root.player) : ""
        }

        Column {
            anchors.left: cover.right
            anchors.leftMargin: K.Style.s3
            anchors.right: transport.left
            anchors.rightMargin: K.Style.s2
            anchors.verticalCenter: parent.verticalCenter
            spacing: 2

            K.Label {
                width: parent.width
                text: root.player ? String(root.player.trackTitle || "") : ""
                role: "bodyStrong"
            }

            K.Label {
                width: parent.width
                visible: text.length > 0
                text: root.player ? Theme.joinArtists(root.player.trackArtists, root.player.trackArtist) : ""
                role: "caption"
                tone: "muted"
            }
        }

        Row {
            id: transport
            anchors.right: parent.right
            anchors.rightMargin: K.Style.s2
            anchors.verticalCenter: parent.verticalCenter
            spacing: K.Style.s1

            K.IconButton {
                icon: "skip_previous"
                size: 32
                variant: "plain"
                enabled: root.player !== null && root.player.canGoPrevious === true
                onClicked: root.player.previous()
            }

            K.IconButton {
                icon: root.player && root.player.isPlaying ? "pause" : "play_arrow"
                size: 32
                variant: "filled"
                enabled: root.player !== null && root.player.canTogglePlaying === true
                onClicked: root.player.togglePlaying()
            }

            K.IconButton {
                icon: "skip_next"
                size: 32
                variant: "plain"
                enabled: root.player !== null && root.player.canGoNext === true
                onClicked: root.player.next()
            }
        }
    }
}

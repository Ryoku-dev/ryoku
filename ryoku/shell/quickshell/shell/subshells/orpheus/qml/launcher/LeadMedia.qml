import QtQuick
import shell.services
import Ryoku.Ui.Singletons
import "../kit" as K

// A player row: the cover, the track and artist, and the transport. The row's
// actions stay on the keyboard as tracks under it.
Column {
    id: root

    property var lead: null
    readonly property var row: root.lead ? root.lead.row : null

    // the row id carries the player's bus name
    readonly property var player: {
        const id = root.row ? String(root.row.id || "") : "";
        const set = Media.playerSet || [];
        for (let i = 0; i < set.length; i++) {
            const candidate = set[i];
            if (candidate && id === "mpris:" + String(candidate.dbusName || ("player-" + String(candidate.uniqueId))))
                return candidate;
        }
        return Media.player;
    }
    readonly property bool playing: root.player !== null && root.player.isPlaying === true

    spacing: K.Style.s3

    Item {
        width: root.width
        height: cover.height

        K.CoverArt {
            id: cover
            y: root.lead ? root.lead.rise : 0
            size: 112
            source: Music.coverFor(root.player)
        }

        Column {
            anchors.left: cover.right
            anchors.leftMargin: K.Style.s4
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: K.Style.s1

            K.Label {
                width: parent.width
                text: root.player && String(root.player.trackTitle || "").length > 0
                    ? String(root.player.trackTitle) : root.row ? String(root.row.title || "") : ""
                role: "title"
                wrapMode: Text.Wrap
                maximumLineCount: 2
                elide: Text.ElideRight
            }

            K.Label {
                width: parent.width
                visible: text.length > 0
                text: root.player ? Theme.joinArtists(root.player.trackArtists, root.player.trackArtist)
                    : root.row ? String(root.row.subtitle || "") : ""
                role: "body"
                tone: "dim"
            }

            K.Label {
                width: parent.width
                visible: text.length > 0
                text: root.player ? String(root.player.identity || "") : ""
                role: "caption"
                tone: "faint"
            }
        }
    }

    Row {
        anchors.horizontalCenter: parent.horizontalCenter
        spacing: K.Style.s3

        K.IconButton {
            anchors.verticalCenter: parent.verticalCenter
            icon: "skip_previous"
            size: 40
            variant: "plain"
            enabled: root.player !== null && root.player.canGoPrevious === true
            onClicked: root.player.previous()
        }

        K.PlayButton {
            anchors.verticalCenter: parent.verticalCenter
            playing: root.playing
            enabled: root.player !== null && root.player.canTogglePlaying === true
            onClicked: root.player.togglePlaying()
        }

        K.IconButton {
            anchors.verticalCenter: parent.verticalCenter
            icon: "skip_next"
            size: 40
            variant: "plain"
            enabled: root.player !== null && root.player.canGoNext === true
            onClicked: root.player.next()
        }
    }

    ActionList {
        width: root.width
        items: root.lead ? root.lead.actionItems : []
        focusIndex: root.lead ? root.lead.actionFocus : -1
        scroller: root.lead ? root.lead.scroller : null
        onActivated: index => root.lead.runAt(index)
    }
}

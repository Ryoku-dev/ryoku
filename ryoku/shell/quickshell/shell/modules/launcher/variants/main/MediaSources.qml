import QtQuick
import QtQuick.Shapes
import Quickshell.Widgets
import Quickshell.Services.Mpris
import "../../shared/Singletons"
import shell.services as Svc
import "metrics.js" as MainMetrics

// The other-players strip: extends under the now-playing card so every media
// source is visible and switchable at a glance without a second panel. The active
// player owns the full card above; every OTHER controllable player with a track
// (a paused browser tab, Spotify, a video) shows here as one slim
// row: tiny cover, title dot artist, a play button. Tapping a row switches source,
// pausing the rest so two streams never overlap. Slim by design so two players
// read as one compact stack, not a second card.
Column {
    id: root

    property real s: 1
    property var activePlayer: null

    spacing: 4 * s

    // Controllable players with a track, minus the one in the card, capped so the
    // strip never balloons. Players.realPlayers drops the playerctld proxy and
    // dedupes by dbusName, so a source never shows twice; the Mpris.players read
    // keeps it live as playback state changes.
    readonly property var sources: {
        void Mpris.players.values;
        var list = Players.realPlayers();
        var out = [];
        for (var i = 0; i < list.length && out.length < 3; i++) {
            var p = list[i];
            if (p === root.activePlayer)
                continue;
            if (p.canControl && p.trackTitle && p.trackTitle.length > 0)
                out.push(p);
        }
        return out;
    }

    // Switch the airwaves to `target`: resume it, pause every other real player,
    // so exactly one source plays. Works for any player.
    function switchTo(target) {
        var list = Players.realPlayers();
        for (var i = 0; i < list.length; i++) {
            var p = list[i];
            if (p === target) {
                if (!p.isPlaying) {
                    if (p.canPlay)
                        p.play();
                    else if (p.canTogglePlaying)
                        p.togglePlaying();
                }
            } else if (p.isPlaying && p.canPause) {
                p.pause();
            }
        }
    }

    Repeater {
        model: root.sources

        // one slim source row.
        delegate: Rectangle {
            id: strip
            required property var modelData
            width: root.width
            height: 34 * root.s
            radius: MainMetrics.radiusRow * root.s
            color: hover.containsMouse ? Theme.threadBg : Qt.rgba(0.94, 0.88, 0.84, 0.03)
            border.width: hover.containsMouse ? 1 : 0
            border.color: Theme.lineStrong

            readonly property string cover: Svc.Music.coverFor(modelData)
            readonly property string trackName: modelData.trackTitle || ""
            readonly property string who: Theme.joinArtists(modelData.trackArtists, modelData.trackArtist)

            ClippingRectangle {
                id: thumb
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                anchors.leftMargin: 6 * root.s
                width: 24 * root.s
                height: 24 * root.s
                radius: Theme.radius
                color: Theme.tileBg

                Image {
                    anchors.fill: parent
                    source: strip.cover
                    sourceSize: Qt.size(Math.ceil(width * 2), Math.ceil(height * 2))
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    cache: true
                    visible: status === Image.Ready
                }
                Text {
                    anchors.centerIn: parent
                    visible: strip.cover === ""
                    text: "\u266a"
                    color: Theme.subtle
                    font.family: Theme.font
                    font.pixelSize: 12 * root.s
                }
            }

            // title dot artist on one line: title reads first, artist faint after
            // a dot, both elided so the strip never wraps.
            Text {
                id: label
                anchors.left: thumb.right
                anchors.right: playBtn.left
                anchors.verticalCenter: parent.verticalCenter
                anchors.leftMargin: 10 * root.s
                anchors.rightMargin: 8 * root.s
                textFormat: Text.StyledText
                text: {
                    var t = strip.escapeHtml(strip.trackName);
                    if (strip.who.length > 0)
                        return t + "  <font color=\"" + Theme.faint + "\">\u00b7  " + strip.escapeHtml(strip.who) + "</font>";
                    return t;
                }
                color: Theme.cream
                font.family: Theme.font
                font.pixelSize: Metrics.fontSubtitle * root.s
                elide: Text.ElideRight
            }

            function escapeHtml(x) {
                return String(x).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
            }

            // mini play/resume button. A paused strip is always resumable; the
            // whole row is clickable too, this just makes the affordance obvious.
            Item {
                id: playBtn
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                anchors.rightMargin: 8 * root.s
                width: 20 * root.s
                height: 20 * root.s

                Shape {
                    anchors.centerIn: parent
                    width: 12 * root.s
                    height: 12 * root.s
                    preferredRendererType: Shape.CurveRenderer
                    ShapePath {
                        strokeColor: "transparent"
                        fillColor: Theme.vermLit
                        // scale a 24-unit play glyph into the 12*s box.
                        scale: Qt.size((12 * root.s) / 24, (12 * root.s) / 24)
                        PathSvg { path: "M8 5l11 7-11 7z" }
                    }
                }
            }

            MouseArea {
                id: hover
                anchors.fill: parent
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onClicked: root.switchTo(strip.modelData)
            }
        }
    }
}

import QtQuick
import QtQuick.Shapes
import QtQuick.Effects
import Quickshell.Widgets
import "../../shared/Singletons"
import shell.services as Svc
import "metrics.js" as MainMetrics
import "../../shared/lib/radio.js" as RadioLib
import "." as MainVariant
import Ryoku.Ui.Singletons

// Now-playing detail: cover art over a blurred bleed backdrop, title/artist, a
// compact prev/play-pause/next transport row, and the signature wavy seekbar (a
// sine while playing, flat when paused). Reads the active MPRIS player supplied
// by Launcher.qml. The FrameAnimation that drives the wave repaints only while
// playing and visible, so an idle launcher costs nothing. Transport buttons dim
// from the player's can* flags so the card degrades gracefully (streams/ads).
// Cover art comes from the active player or the live-radio engine; the music-note
// fallback keeps missing metadata explicit without a background network lookup.
Item {
    id: root

    property real s: 1
    property var player: null

    readonly property bool hasPlayer: player !== null && player !== undefined
    readonly property bool playing: hasPlayer && player.isPlaying
    // The ryoku radio wears a different coat: ON AIR eyebrow, station name, a
    // LIVE pulse where a track would show elapsed/total, and no seekbar - a
    // broadcast has no position to scrub.
    readonly property bool radio: hasPlayer && RadioLib.isRadioPlayer(player.dbusName, player.trackTitle)
    // The player's own title, but a raw URL-ish stream title (a browser tab, or an
    // mpv still resolving) is suppressed to a neutral label, not shown as a URL.
    readonly property string rawTitle: hasPlayer && player.trackTitle ? player.trackTitle : ""
    readonly property bool rawIsUrl: rawTitle.indexOf("watch?v=") !== -1
        || /^(https?:\/\/|www\.)/i.test(rawTitle)
    readonly property string title: radio ? rawTitle.slice(RadioLib.TITLE_PREFIX.length)
        : (rawTitle.length > 0 && !rawIsUrl ? rawTitle : I18n.tr("Nothing playing"))
    readonly property string artist: radio
        ? (Radio.fellBack ? I18n.tr("internet radio · fallback station") : I18n.tr("internet radio · 24/7 live"))
        : (hasPlayer ? Theme.joinArtists(player.trackArtists, player.trackArtist) : "")
    readonly property string artUrl: Svc.Music.coverFor(player)
    readonly property string effectiveArt:
        radio ? Radio.artUrl : artUrl
    readonly property real positionSec: hasPlayer ? player.position : 0
    readonly property real lengthSec: hasPlayer && player.length > 0 ? player.length : 0
    readonly property real frac: lengthSec > 0 ? Math.max(0, Math.min(1, positionSec / lengthSec)) : 0
    readonly property bool canPlay: hasPlayer && player.canTogglePlaying
    readonly property bool canNext: hasPlayer && player.canGoNext
    readonly property bool canPrev: hasPlayer && player.canGoPrevious
    // Scrub-to-seek: canSeek gates it (the player allows a seek and has a known
    // length); while dragging, the fill follows the cursor and the elapsed label
    // previews the target, committed to player.position on release. Never on a
    // radio: a live stream's "length" is whatever the buffer claims.
    readonly property bool canSeek: hasPlayer && !radio && player.canSeek && lengthSec > 0
    property bool scrubbing: false
    property real scrubFrac: 0

    // "m:ss" for the elapsed and total labels.
    function fmt(sec) {
        if (!(sec > 0))
            return "0:00";
        var s = Math.floor(sec);
        var m = Math.floor(s / 60);
        var r = s % 60;
        return m + ":" + (r < 10 ? "0" + r : r);
    }

    implicitHeight: 148 * s


    ClippingRectangle {
        anchors.fill: parent
        radius: MainMetrics.radiusRow * root.s
        color: Theme.drawer

        Image {
            id: bleed
            anchors.fill: parent
            source: root.effectiveArt
            sourceSize: Qt.size(128, 128)
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
            cache: true
            visible: false
        }
        MultiEffect {
            anchors.fill: parent
            source: bleed
            scale: 1.12
            visible: !Performance.blurDisabled && root.effectiveArt !== "" && bleed.status === Image.Ready
            blurEnabled: !Performance.blurDisabled
            blur: 0.95
            blurMax: 48
            saturation: 0.1
        }
        // Veil over the bleed so title/artist stay legible on any album art.
        Rectangle {
            anchors.fill: parent
            color: Qt.rgba(Theme.drawer.r, Theme.drawer.g, Theme.drawer.b, 0.78)
        }


        ClippingRectangle {
            id: cover
            anchors.left: parent.left
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            anchors.margins: 12 * root.s
            width: height
            radius: Theme.radius
            color: Theme.tileBg

            Image {
                anchors.fill: parent
                source: root.effectiveArt
                sourceSize: Qt.size(Math.ceil(width * 2), Math.ceil(height * 2))
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
                cache: true
                visible: status === Image.Ready
            }
            // Music-note fallback when the player has no art URL. Kept as an
            // inline Text so NowPlaying has no external glyph dependency.
            Text {
                anchors.centerIn: parent
                visible: root.effectiveArt === "" && !root.radio
                text: "\u266a"
                color: Theme.subtle
                font.family: Theme.font
                font.pixelSize: 28 * root.s
            }
            // The radio's station plate: a broadcast mark instead of a cover -
            // no album exists, and an iTunes guess would be someone else's.
            Column {
                anchors.centerIn: parent
                visible: root.radio && root.effectiveArt === ""
                spacing: 2 * root.s
                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: "\u25ce"
                    color: Theme.vermLit
                    font.family: Theme.font
                    font.pixelSize: 30 * root.s
                }
                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: I18n.tr("ON AIR")
                    color: Theme.vermLit
                    font.family: Theme.mono
                    font.pixelSize: 9 * root.s
                    font.letterSpacing: 2
                }
            }
        }

        Column {
            id: meta
            anchors.left: cover.right
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.leftMargin: 14 * root.s
            anchors.rightMargin: 14 * root.s
            anchors.topMargin: 16 * root.s
            spacing: 4 * root.s

            Row {
                width: parent.width
                spacing: 6 * root.s
                MainVariant.BrandMark {
                    anchors.verticalCenter: parent.verticalCenter
                    size: Metrics.fontEyebrow * root.s
                    color: Theme.vermLit
                }
                // the radio's tally lamp: a slow pulse that says "broadcast",
                // where a track just states NOW PLAYING.
                Rectangle {
                    id: tallyDot
                    visible: root.radio
                    anchors.verticalCenter: parent.verticalCenter
                    width: 6 * root.s
                    height: width
                    radius: width / 2
                    color: Theme.vermLit
                    SequentialAnimation on opacity {
                        running: root.radio && root.visible && root.playing
                        loops: Animation.Infinite
                        NumberAnimation { from: 1; to: 0.25; duration: 900; easing.type: Easing.InOutSine }
                        NumberAnimation { from: 0.25; to: 1; duration: 900; easing.type: Easing.InOutSine }
                        onStopped: tallyDot.opacity = 1
                    }
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: root.radio ? I18n.tr("ON AIR · LIVE RADIO") : I18n.tr(" NOW PLAYING")
                    color: Theme.vermLit
                    font.family: Theme.font
                    font.pixelSize: Metrics.fontEyebrow * root.s
                    font.letterSpacing: 1.5
                }
            }
            Text {
                width: parent.width
                text: root.title
                color: Theme.bright
                font.family: Theme.font
                font.pixelSize: Metrics.fontTitle * root.s
                font.weight: Font.DemiBold
                elide: Text.ElideRight
            }
            Text {
                width: parent.width
                text: root.artist
                color: Theme.subtle
                font.family: Theme.font
                font.pixelSize: Metrics.fontSubtitle * root.s
                elide: Text.ElideRight
                visible: root.artist.length > 0
            }
        }

        // MPRIS never pushes position, so poll it while playing to advance the
        // seek fill and the elapsed label; re-reading `position` is the standard
        // Quickshell pattern (see pill Media.qml).
        Timer {
            interval: 500
            running: root.visible && root.playing
            repeat: true
            onTriggered: if (root.player) root.player.positionChanged();
        }

        // Bottom media strip: elapsed time, a centered transport cluster, and
        // total time, with the wavy seekbar above. Buttons dim when the player
        // disallows the step so the affordance never lies about what will happen.
        Row {
            id: transport
            anchors.horizontalCenter: seek.horizontalCenter
            anchors.bottom: parent.bottom
            anchors.bottomMargin: 12 * root.s
            spacing: 2 * root.s

            MainVariant.TransportButton { s: root.s; glyph: "prev"; active: root.canPrev; onTapped: root.player.previous() }
            MainVariant.TransportButton { s: root.s; glyph: root.playing ? "pause" : "play"; active: root.canPlay; onTapped: root.player.togglePlaying() }
            MainVariant.TransportButton { s: root.s; glyph: "next"; active: root.canNext; onTapped: root.player.next() }
        }

        Text {
            id: elapsed
            anchors.left: cover.right
            anchors.leftMargin: 14 * root.s
            anchors.verticalCenter: transport.verticalCenter
            // a broadcast has no elapsed: the corner carries the LIVE tally
            // instead, pulsing while on air, steady when paused.
            text: root.radio ? I18n.tr("● LIVE") : root.fmt(root.scrubbing ? root.scrubFrac * root.lengthSec : root.positionSec)
            color: root.radio ? Theme.vermLit : Theme.faint
            font.family: Theme.mono
            font.pixelSize: Metrics.fontEyebrow * root.s
            font.features: { "tnum": 1 }
            SequentialAnimation on opacity {
                running: root.radio && root.visible && root.playing
                loops: Animation.Infinite
                NumberAnimation { from: 1; to: 0.35; duration: 900; easing.type: Easing.InOutSine }
                NumberAnimation { from: 0.35; to: 1; duration: 900; easing.type: Easing.InOutSine }
                // the pulse can stop mid-fade (pause, or a track takes over);
                // land back at rest instead of wherever the wave was.
                onStopped: elapsed.opacity = 1
            }
        }
        Text {
            id: total
            anchors.right: parent.right
            anchors.rightMargin: 14 * root.s
            anchors.verticalCenter: transport.verticalCenter
            text: root.radio ? "24/7" : root.fmt(root.lengthSec)
            color: Theme.faint
            font.family: Theme.mono
            font.pixelSize: Metrics.fontEyebrow * root.s
            font.features: { "tnum": 1 }
        }

        // Wavy seekbar: a dry base line with a sine over the filled portion
        // while playing. drawFrac glides between the 500ms polls so the fill
        // advances smoothly instead of stepping.
        Canvas {
            id: seek
            anchors.left: cover.right
            anchors.right: parent.right
            anchors.bottom: transport.top
            anchors.leftMargin: 14 * root.s
            anchors.rightMargin: 14 * root.s
            anchors.bottomMargin: 6 * root.s
            height: 12 * root.s
            // a broadcast has no position: the bar would be a lie at 0:00
            // forever. The cava wave behind the card carries the motion.
            visible: !root.radio

            property real phase: 0
            // While scrubbing the fill snaps to the cursor (no glide); otherwise it
            // eases between the 500ms position polls so playback advances smoothly.
            property real drawFrac: root.scrubbing ? root.scrubFrac : root.frac
            Behavior on drawFrac { enabled: !root.scrubbing; NumberAnimation { duration: 480; easing.type: Easing.Linear } }
            onDrawFracChanged: requestPaint()

            // Scrub-to-seek: press or drag anywhere on the bar to preview a target,
            // commit it to the player's position on release. A generous vertical
            // hit area makes the thin bar easy to grab. Only when the player can
            // seek; otherwise the bar stays a pure indicator.
            MouseArea {
                anchors.fill: parent
                anchors.topMargin: -8 * root.s
                anchors.bottomMargin: -8 * root.s
                enabled: root.canSeek
                cursorShape: root.canSeek ? Qt.PointingHandCursor : Qt.ArrowCursor
                preventStealing: true
                function fracAt(x) { return Math.max(0, Math.min(1, x / seek.width)); }
                onPressed: (m) => { root.scrubbing = true; root.scrubFrac = fracAt(m.x); }
                onPositionChanged: (m) => { if (root.scrubbing) root.scrubFrac = fracAt(m.x); }
                onReleased: (m) => {
                    if (!root.scrubbing) return;
                    var target = fracAt(m.x) * root.lengthSec;
                    if (root.player) root.player.position = target;
                    root.scrubbing = false;
                }
                onCanceled: root.scrubbing = false
            }

            onPaint: {
                var ctx = getContext("2d");
                ctx.reset();
                var w = width;
                var cy = height / 2;
                var fillW = w * seek.drawFrac;

                ctx.strokeStyle = Theme.faint;
                ctx.lineWidth = 2 * root.s;
                ctx.beginPath();
                ctx.moveTo(fillW, cy);
                ctx.lineTo(w, cy);
                ctx.stroke();

                var amp = root.playing ? 3 * root.s : 0;
                var waves = Math.max(1, Math.round(fillW / (24 * root.s)));
                var steps = waves * 16;
                ctx.strokeStyle = Theme.vermLit;
                ctx.lineWidth = 2.5 * root.s;
                ctx.beginPath();
                for (var i = 0; i <= steps; i++) {
                    var x = (fillW * i) / steps;
                    var theta = waves * 2 * Math.PI * (i / steps) + seek.phase;
                    var y = cy + amp * Math.sin(theta);
                    if (i === 0)
                        ctx.moveTo(x, y);
                    else
                        ctx.lineTo(x, y);
                }
                ctx.stroke();
            }
        }

        FrameAnimation {
            running: root.playing && root.visible
            onTriggered: {
                seek.phase = Date.now() / 400;
                seek.requestPaint();
            }
        }

        // Position/state changes still repaint the seekbar when paused so a
        // scrub or a play->pause snaps the wave immediately.
        Connections {
            target: root
            function onFracChanged() { seek.requestPaint(); }
            function onPlayingChanged() { seek.requestPaint(); }
        }
    }

}

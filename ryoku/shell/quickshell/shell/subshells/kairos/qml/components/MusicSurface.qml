// The Kairos music island in three sizes, all driven by the caller: the cover
// bubble beside the clock, the peek a hover opens (cover, track, transport) and
// the full now-playing panel a click opens (the cover blurred into an ambient
// wash behind a theme tint, the track, its progress and its transport).
//
// The peek and the full panel share a width, so only the height and the corner
// radius move between them; the cover is one item whose size, position and corner
// all interpolate, and the transport is one row that glides from its compact
// right-aligned place into the full row.

import QtQuick
import QtQuick.Effects
import shell.services
import shell.barkit as Pill
import "../IslandMetrics.js" as Island

Rectangle {
    id: surface

    required property var player
    // 0..1 hover extension, 0..1 full panel. Both 0 is the bubble.
    required property real peek
    required property real full

    property color ink: "#ffffff"
    property color accent: "#7fd4d4"
    property real peekHeight: Island.musicPeekHeight

    signal activated()

    // Scrub state while the user drags (or clicks) the progress bar to seek.
    property bool scrubbing: false
    property real scrubFrac: 0
    readonly property bool clockHeld: surface.full > 0.5
    readonly property bool seekable: surface.player !== null
        && surface.player.canSeek && surface.length > 0

    readonly property real extend: Math.max(surface.peek, surface.full)
    readonly property real peekVis: surface.peek * (1 - surface.full)
    readonly property string artUrl: surface.player ? (surface.player.trackArtUrl || "") : ""
    readonly property bool hasArt: surface.artUrl.length > 0
    readonly property bool playing: surface.player !== null && surface.player.isPlaying
    readonly property real length: Music.duration
    readonly property real position: Music.position
    readonly property real fraction: surface.length > 0
        ? Math.max(0, Math.min(1, surface.position / surface.length)) : 0
    readonly property bool backdropActive: surface.full > 0.5
        && surface.hasArt && !Perf.blurDisabled
    readonly property int coverDecodeSize: Math.max(1,
        Math.ceil(Island.coverOpen * Screen.devicePixelRatio))
    readonly property size backdropDecodeSize: Qt.size(
        Math.max(1, Math.ceil((Island.musicWidth - 2 * Island.framePad)
            * Screen.devicePixelRatio)),
        Math.max(1, Math.ceil((Island.musicHeight - 2 * Island.framePad)
            * Screen.devicePixelRatio)))
    readonly property string source: {
        if (!surface.player)
            return "";
        return String(surface.player.identity
            || surface.player.dbusName || "").replace(/^org\.mpris\.MediaPlayer2\./, "");
    }

    function clockText(seconds) {
        var s = Math.max(0, Math.floor(seconds || 0));
        var m = Math.floor(s / 60);
        var r = s % 60;
        return m + ":" + (r < 10 ? "0" + r : String(r));
    }

    onClockHeldChanged: Music.holdClock(surface, surface.clockHeld)
    Component.onCompleted: Music.holdClock(surface, surface.clockHeld)
    Component.onDestruction: Music.holdClock(surface, false)

    width: Island.bubbleSize + (Island.musicWidth - Island.bubbleSize) * surface.extend
    height: Island.bubbleSize
        + (surface.peekHeight - Island.bubbleSize) * surface.peek
        + (Island.musicHeight - Island.bubbleSize) * surface.full
    radius: Island.bubbleSize / 2
        + (Island.musicPeekRadius - Island.bubbleSize / 2) * surface.peek
        + (Island.musicRadius - Island.bubbleSize / 2) * surface.full
    color: Theme.shadow
    // A hairline rim so the panel's rounded shape reads on any wallpaper -- its
    // pure-black frame would otherwise vanish into a dark desktop.
    border.width: 1
    border.color: Qt.rgba(surface.ink.r, surface.ink.g, surface.ink.b, 0.16)
    clip: true

    // The whole island is the click target; the transport below takes its own
    // clicks first, being later in the draw order.
    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: surface.activated()
    }

    // ── the ambient backdrop ─────────────────────────────────────────────────
    // The full panel's cover, blurred into an even wash and inset so the pill's
    // near-black reads as a bezel around it. The whole group renders to one layer
    // that blurs it and masks it to a rounded rect (glassMask), so its corners
    // follow the frame instead of poking square into the bezel. A low accent tint
    // recolours the wash with the wallpaper palette; a soft dark scrim, deeper
    // toward the transport, keeps the text legible.
    Item {
        id: glass
        anchors.fill: parent
        anchors.margins: Island.framePad
        visible: surface.full > 0.001
        opacity: surface.full
        readonly property real radius: Math.max(0, surface.radius - Island.framePad)
        layer.enabled: surface.backdropActive
        layer.smooth: surface.backdropActive
        layer.effect: MultiEffect {
            maskEnabled: surface.backdropActive
            maskSource: glassMask
            autoPaddingEnabled: false
            blurEnabled: surface.backdropActive
            blur: 1.0
            blurMax: 64
            saturation: 0.4
            brightness: -0.05
        }

        // A palette wash under the art, and the whole backdrop when the blur is
        // off or there is no cover.
        Rectangle {
            anchors.fill: parent
            radius: glass.radius
            color: Qt.darker(surface.accent, 1.4)
        }
        Image {
            anchors.fill: parent
            source: surface.artUrl
            sourceSize: surface.backdropDecodeSize
            visible: surface.backdropActive
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
            cache: true
        }
        Rectangle {
            anchors.fill: parent
            radius: glass.radius
            color: surface.accent
            opacity: surface.hasArt ? 0.1 : 0.18
        }
        Rectangle {
            anchors.fill: parent
            radius: glass.radius
            gradient: Gradient {
                GradientStop { position: 0.0; color: Qt.rgba(0, 0, 0, 0.26) }
                GradientStop { position: 1.0; color: Qt.rgba(0, 0, 0, 0.5) }
            }
        }
    }

    // The rounded mask for the ambient group; a sibling so the group's own layer
    // can sample it without masking the mask.
    Rectangle {
        id: glassMask
        x: glass.x
        y: glass.y
        width: glass.width
        height: glass.height
        radius: glass.radius
        visible: false
        layer.enabled: surface.backdropActive
    }

    // The peek's own tinted fill, so the compact bar reads as part of the theme.
    Rectangle {
        anchors.fill: parent
        radius: surface.radius
        color: surface.accent
        opacity: 0.14 * surface.peekVis
    }

    // A soft shadow lifts the cover off the ambient wash in the full panel.
    RectangularShadow {
        anchors.fill: cover
        radius: cover.radius
        blur: 16
        spread: 0
        offset: Qt.vector2d(0, 4)
        color: Qt.rgba(0, 0, 0, 0.5)
        opacity: surface.full
    }

    // ── the cover ────────────────────────────────────────────────────────────
    // A circle in the bubble, a round square in the peek and the panel, ringed in
    // black. One item, interpolated through all three sizes.
    Item {
        id: cover
        x: Island.coverPeekX * surface.peek + Island.coverX * surface.full
        y: Island.coverPeekY * surface.peek + Island.coverY * surface.full
        width: Island.bubbleSize
            + (Island.coverPeek - Island.bubbleSize) * surface.peek
            + (Island.coverOpen - Island.bubbleSize) * surface.full
        height: width
        readonly property real radius: Island.bubbleSize / 2
            + (Island.coverPeekRadius - Island.bubbleSize / 2) * surface.peek
            + (Island.coverRadius - Island.bubbleSize / 2) * surface.full


        Rectangle {
            anchors.fill: parent
            radius: cover.radius
            color: Qt.darker(surface.accent, 1.5)
            visible: !surface.hasArt
        }

        // The art is clipped through a mask layer, not a Shape fill: an
        // invisible Image feeding ShapePath.fillItem never repainted once the
        // art arrived, and the bubble stayed blank. The cover is static, so the
        // two small layers cost nothing per frame.
        Rectangle {
            id: coverMask
            anchors.fill: parent
            radius: cover.radius
            visible: false
            layer.enabled: surface.hasArt
        }

        Image {
            anchors.fill: parent
            source: surface.artUrl
            sourceSize: Qt.size(surface.coverDecodeSize, surface.coverDecodeSize)
            visible: surface.hasArt
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
            cache: true
            layer.enabled: surface.hasArt
            layer.smooth: true
            layer.effect: MultiEffect { maskEnabled: true; maskSource: coverMask }
        }

        Pill.MaterialIcon {
            anchors.centerIn: parent
            visible: !surface.hasArt
            text: "music_note"
            color: surface.ink
            font.pixelSize: Math.round(cover.height * 0.4)
        }

        Rectangle {
            anchors.fill: parent
            radius: cover.radius
            color: "transparent"
            border.width: 2
            border.color: Theme.shadow
        }
    }

    // ── the peek's track line ────────────────────────────────────────────────
    Column {
        id: peekMeta
        x: Island.coverPeekX + Island.coverPeek + 14
        y: 25
        width: 332 - x
        spacing: 2
        opacity: surface.peekVis

        Text {
            width: parent.width
            text: surface.player ? (surface.player.trackTitle || "") : ""
            color: surface.ink
            font.family: Theme.fontPrimary
            font.pixelSize: 15
            font.weight: Font.DemiBold
            elide: Text.ElideRight
        }
        Text {
            width: parent.width
            text: surface.player
                ? Theme.joinArtists(surface.player.trackArtists, surface.player.trackArtist) : ""
            color: Qt.rgba(surface.ink.r, surface.ink.g, surface.ink.b, 0.7)
            font.family: Theme.fontPrimary
            font.pixelSize: 11
            elide: Text.ElideRight
        }
    }

    // ── the full panel's track ───────────────────────────────────────────────
    Column {
        id: meta
        x: Island.coverX + Island.coverOpen + 20
        y: 32
        width: Island.musicWidth - x - 20
        spacing: 2
        opacity: surface.full

        Text {
            width: parent.width
            text: surface.player ? (surface.player.trackTitle || "") : ""
            color: surface.ink
            font.family: Theme.fontPrimary
            font.pixelSize: 22
            font.weight: Font.DemiBold
            elide: Text.ElideRight
        }
        Text {
            width: parent.width
            height: 16
            text: surface.player
                ? Theme.joinArtists(surface.player.trackArtists, surface.player.trackArtist) : ""
            color: Qt.rgba(surface.ink.r, surface.ink.g, surface.ink.b, 0.78)
            font.family: Theme.fontPrimary
            font.pixelSize: 14
            elide: Text.ElideRight
        }
        Text {
            width: parent.width
            height: 15
            visible: surface.player && (surface.player.trackAlbum || "").length > 0
            text: surface.player ? (surface.player.trackAlbum || "") : ""
            color: Qt.rgba(surface.ink.r, surface.ink.g, surface.ink.b, 0.5)
            font.family: Theme.fontPrimary
            font.pixelSize: 12
            elide: Text.ElideRight
        }
        Text {
            width: parent.width
            height: 15
            visible: surface.source.length > 0
            text: surface.source
            color: Qt.rgba(surface.accent.r, surface.accent.g, surface.accent.b, 0.85)
            font.family: Theme.fontPrimary
            font.pixelSize: 12
            elide: Text.ElideRight
        }
    }

    // ── progress, full panel only: click or drag the bar to seek ─────────────
    Item {
        id: seek
        x: meta.x
        y: 140
        width: meta.width
        height: 24
        opacity: surface.full
        visible: surface.full > 0.01

        // What the fill shows: the live position, or the scrub target while dragging.
        readonly property real shownFrac: surface.scrubbing ? surface.scrubFrac : surface.fraction
        function fracAt(mx) { return Math.max(0, Math.min(1, mx / width)); }

        Rectangle {
            id: track
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.top
            anchors.verticalCenterOffset: 5
            height: surface.scrubbing ? 7 : 5
            radius: height / 2
            color: Qt.rgba(surface.ink.r, surface.ink.g, surface.ink.b, 0.22)
            Behavior on height {
                enabled: !Motion.reduce
                NumberAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
            }

            Rectangle {
                id: fill
                width: parent.width * seek.shownFrac
                height: parent.height
                radius: parent.radius
                color: surface.accent
            }

            // The draggable knob, shown only when the player can seek.
            Rectangle {
                width: 13
                height: 13
                radius: height / 2
                color: surface.accent
                border.width: 2
                border.color: Theme.shadow
                anchors.verticalCenter: parent.verticalCenter
                x: Math.max(-height / 2, Math.min(parent.width - height / 2, fill.width - height / 2))
                opacity: surface.seekable ? surface.full : 0
                scale: surface.scrubbing ? 1.3 : 1
                Behavior on scale {
                    enabled: !Motion.reduce
                    NumberAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
                }
            }
        }

        // Hit target, taller than the bar so it is easy to grab; commits the seek
        // on release and previews it live while dragging.
        MouseArea {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: track.verticalCenter
            height: 22
            enabled: surface.seekable
            cursorShape: Qt.PointingHandCursor
            preventStealing: true
            onPressed: mouse => { surface.scrubbing = true; surface.scrubFrac = seek.fracAt(mouse.x); }
            onPositionChanged: mouse => { if (surface.scrubbing) surface.scrubFrac = seek.fracAt(mouse.x); }
            onReleased: {
                if (surface.player && surface.scrubbing)
                    Music.seekPosition(surface.scrubFrac * surface.length);
                surface.scrubbing = false;
            }
            onCanceled: surface.scrubbing = false
        }

        Text {
            anchors.left: parent.left
            anchors.top: track.bottom
            anchors.topMargin: 3
            text: surface.clockText(surface.scrubbing
                ? surface.scrubFrac * surface.length : surface.position)
            color: Qt.rgba(surface.ink.r, surface.ink.g, surface.ink.b, 0.6)
            font.family: Theme.mono
            font.pixelSize: 10
        }
        Text {
            anchors.right: parent.right
            anchors.top: track.bottom
            anchors.topMargin: 3
            text: surface.length > 0 ? surface.clockText(surface.length) : ""
            color: Qt.rgba(surface.ink.r, surface.ink.g, surface.ink.b, 0.6)
            font.family: Theme.mono
            font.pixelSize: 10
        }
    }

    // ── transport ────────────────────────────────────────────────────────────
    // One row, compact and right-aligned in the peek, centred under the track in
    // the full panel; it glides between the two as the panel grows.
    Row {
        id: transport
        x: 340 * surface.peek + 255 * surface.full
        y: 30 * surface.peek + 170 * surface.full
        spacing: 16
        opacity: surface.extend

        Pill.MaterialIcon {
            anchors.verticalCenter: parent.verticalCenter
            text: "skip_previous"
            color: surface.ink
            opacity: surface.player && surface.player.canGoPrevious ? 0.9 : 0.3
            font.pixelSize: 20
            MouseArea {
                anchors.fill: parent
                anchors.margins: -6
                cursorShape: Qt.PointingHandCursor
                onClicked: if (surface.player && surface.player.canGoPrevious) surface.player.previous()
            }
        }

        Rectangle {
            anchors.verticalCenter: parent.verticalCenter
            width: 40
            height: 40
            radius: height / 2
            color: Qt.rgba(surface.accent.r, surface.accent.g, surface.accent.b, 0.28)

            Pill.MaterialIcon {
                anchors.centerIn: parent
                text: surface.playing ? "pause" : "play_arrow"
                fill: 1
                color: surface.ink
                font.pixelSize: 22
            }
            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: if (surface.player && surface.player.canTogglePlaying) surface.player.togglePlaying()
            }
        }

        Pill.MaterialIcon {
            anchors.verticalCenter: parent.verticalCenter
            text: "skip_next"
            color: surface.ink
            opacity: surface.player && surface.player.canGoNext ? 0.9 : 0.3
            font.pixelSize: 20
            MouseArea {
                anchors.fill: parent
                anchors.margins: -6
                cursorShape: Qt.PointingHandCursor
                onClicked: if (surface.player && surface.player.canGoNext) surface.player.next()
            }
        }
    }
}

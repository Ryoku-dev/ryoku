import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Io
import Quickshell.Services.Mpris
import shell.services

Item {
    id: rootMod
    required property var root

    // shared player selection (ghost-filtering) - see MprisSelect.qml
    MprisSelect { id: sel }
    readonly property var  player:  sel.player
    readonly property bool active:  sel.active
    readonly property bool playing: sel.playing
    readonly property bool fullMode: root.mprisBarStyle === "full"
    readonly property int titleWidth: root.mprisTitleWidth
    readonly property bool motionAllowed: !Perf.reduceMotion && !Perf.pillFrozen
    readonly property color contentColor: root.widgetContentColor("G9", root.ink)
    readonly property color accentColor: root.widgetHasFill("G9")
        ? contentColor
        : root.seal

    readonly property string trackLabel: {
        if (!player) return ""
        var t = player.trackTitle  || ""
        var a = player.trackArtist || ""
        return a ? t + "  ·  " + a : t
    }

    // ── FULL / muse: compact real-audio waveform ───────────────────
    readonly property int museBands: 24
    property var museLevels: []

    function resetMuseLevels() {
        var rest = []
        for (var i = 0; i < museBands; i++) rest.push(0.04)
        museLevels = rest
    }

    Component.onCompleted: resetMuseLevels()

    Process {
        id: museCava
        running: rootMod.visible && rootMod.active && rootMod.fullMode && rootMod.playing && rootMod.motionAllowed
        command: ["bash", "-c",
            "command -v cava >/dev/null 2>&1 || exit 0; " +
            "exec cava -p <(printf '%s\\n' " +
            "'[general]' 'bars = 24' 'framerate = 60' 'autosens = 1' 'sleep_timer = 0' " +
            "'[input]' 'method = pipewire' 'source = auto' " +
            "'[output]' 'method = raw' 'raw_target = /dev/stdout' " +
            "'data_format = ascii' 'ascii_max_range = 100' " +
            "'[smoothing]' 'monstercat = 0' 'waves = 0' 'noise_reduction = 20')"
        ]
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: function(line) {
                if (!rootMod.playing || !rootMod.fullMode || !rootMod.motionAllowed) return
                var parts = line.split(";")
                var out = []
                for (var i = 0; i < rootMod.museBands; i++) {
                    var value = parseInt(parts[i])
                    out.push(isNaN(value) ? 0 : Math.min(1, value / 100))
                }
                rootMod.museLevels = out
            }
        }
    }

    // ── equalizer bar heights (0.0 - 1.0) ──
    property real barH1: 0.08
    property real barH2: 0.08
    property real barH3: 0.08

    // bounce sequences - regular animations with explicit target, no PVS conflict
    SequentialAnimation {
        id: anim1
        running: rootMod.visible && rootMod.playing && !rootMod.fullMode && rootMod.motionAllowed; loops: Animation.Infinite
        NumberAnimation { target: rootMod; property: "barH1"; to: 0.85; duration: 220; easing.type: Easing.InOutSine }
        NumberAnimation { target: rootMod; property: "barH1"; to: 0.18; duration: 300; easing.type: Easing.InOutSine }
        NumberAnimation { target: rootMod; property: "barH1"; to: 0.70; duration: 260; easing.type: Easing.InOutSine }
        NumberAnimation { target: rootMod; property: "barH1"; to: 0.10; duration: 280; easing.type: Easing.InOutSine }
    }
    SequentialAnimation {
        id: anim2
        running: rootMod.visible && rootMod.playing && !rootMod.fullMode && rootMod.motionAllowed; loops: Animation.Infinite
        NumberAnimation { target: rootMod; property: "barH2"; to: 0.45; duration: 310; easing.type: Easing.InOutSine }
        NumberAnimation { target: rootMod; property: "barH2"; to: 0.92; duration: 280; easing.type: Easing.InOutSine }
        NumberAnimation { target: rootMod; property: "barH2"; to: 0.28; duration: 340; easing.type: Easing.InOutSine }
        NumberAnimation { target: rootMod; property: "barH2"; to: 0.65; duration: 290; easing.type: Easing.InOutSine }
    }
    SequentialAnimation {
        id: anim3
        running: rootMod.visible && rootMod.playing && !rootMod.fullMode && rootMod.motionAllowed; loops: Animation.Infinite
        NumberAnimation { target: rootMod; property: "barH3"; to: 0.60; duration: 380; easing.type: Easing.InOutSine }
        NumberAnimation { target: rootMod; property: "barH3"; to: 0.12; duration: 320; easing.type: Easing.InOutSine }
        NumberAnimation { target: rootMod; property: "barH3"; to: 0.95; duration: 350; easing.type: Easing.InOutSine }
        NumberAnimation { target: rootMod; property: "barH3"; to: 0.32; duration: 400; easing.type: Easing.InOutSine }
    }

    // drop bars to rest when paused
    ParallelAnimation {
        id: dropAnim
        NumberAnimation { target: rootMod; property: "barH1"; to: 0.08; duration: 380; easing.type: Easing.OutCubic }
        NumberAnimation { target: rootMod; property: "barH2"; to: 0.08; duration: 430; easing.type: Easing.OutCubic }
        NumberAnimation { target: rootMod; property: "barH3"; to: 0.08; duration: 480; easing.type: Easing.OutCubic }
    }
    onPlayingChanged: {
        if (!playing) {
            dropAnim.restart()
            resetMuseLevels()
        }
    }
    onMotionAllowedChanged: {
        if (!motionAllowed) {
            dropAnim.stop()
            barH1 = 0.08
            barH2 = 0.08
            barH3 = 0.08
            resetMuseLevels()
        }
        if (marqueeClip) marqueeClip.resetMarquee()
    }
    onFullModeChanged: {
        resetMuseLevels()
        if (!fullMode) marqueeClip.resetMarquee()
    }

    visible: implicitWidth > 0.5
    implicitWidth: root.modMpris
        ? (active
            ? ((fullMode ? fullRow.implicitWidth : defaultRow.implicitWidth) + 18)
            : (idleNote.implicitWidth + 16))
        : 0
    implicitHeight: 28
    opacity: root.modMpris ? 1 : 0

    Behavior on opacity { NumberAnimation { duration: 140; easing.type: Easing.OutCubic } }

    Behavior on implicitWidth {
        NumberAnimation { duration: 220; easing.type: Easing.OutCubic }
    }

    // ── idle: a single music-note, clickable to open the no-song panel ──
    IconText {
        id: idleNote
        anchors.centerIn: parent
        visible: !rootMod.active
        text: ""   // music_note
        font.pixelSize: 15
        color: Qt.rgba(rootMod.contentColor.r, rootMod.contentColor.g, rootMod.contentColor.b, 0.45)
        MouseArea {
            anchors.fill: parent
            acceptedButtons: Qt.LeftButton
            cursorShape: Qt.PointingHandCursor
            onClicked: root.mprisVisible = !root.mprisVisible
        }
    }

    Row {
        id: defaultRow
        visible: rootMod.active && !rootMod.fullMode
        anchors.centerIn: parent
        spacing: 4

        // ── prev ──
        IconText {
            anchors.verticalCenter: parent.verticalCenter
            text: ""
            font.pixelSize: 13
            color: (rootMod.player && rootMod.player.canGoPrevious)
                ? Qt.rgba(rootMod.contentColor.r, rootMod.contentColor.g, rootMod.contentColor.b, 0.7)
                : Qt.rgba(rootMod.contentColor.r, rootMod.contentColor.g, rootMod.contentColor.b, 0.22)
            Behavior on color { ColorAnimation { duration: 150 } }
            MouseArea {
                anchors.fill: parent; cursorShape: Qt.PointingHandCursor
                onClicked: if (rootMod.player) rootMod.player.previous()
            }
        }

        // ── play / pause ──
        IconText {
            anchors.verticalCenter: parent.verticalCenter
            text: rootMod.playing ? "" : ""
            font.pixelSize: 13
            color: rootMod.accentColor
            MouseArea {
                anchors.fill: parent; cursorShape: Qt.PointingHandCursor
                onClicked: if (rootMod.player) rootMod.player.togglePlaying()
            }
        }

        // ── next ──
        IconText {
            anchors.verticalCenter: parent.verticalCenter
            text: ""
            font.pixelSize: 13
            color: (rootMod.player && rootMod.player.canGoNext)
                ? Qt.rgba(rootMod.contentColor.r, rootMod.contentColor.g, rootMod.contentColor.b, 0.7)
                : Qt.rgba(rootMod.contentColor.r, rootMod.contentColor.g, rootMod.contentColor.b, 0.22)
            Behavior on color { ColorAnimation { duration: 150 } }
            MouseArea {
                anchors.fill: parent; cursorShape: Qt.PointingHandCursor
                onClicked: if (rootMod.player) rootMod.player.next()
            }
        }

        // hidden alpha-mask source for the marquee fade - defined BEFORE the masked
        // item so the layer.effect can resolve the id; visible:false → no Row layout.
        Item {
            id: marqueeFadeMask
            width: rootMod.titleWidth; height: 28
            visible: false
            layer.enabled: true
            Rectangle {
                anchors.fill: parent
                gradient: Gradient {
                    orientation: Gradient.Horizontal
                    GradientStop { position: 0.0;  color: "white" }
                    GradientStop { position: 0.92; color: "white" }
                    GradientStop { position: 1.0;  color: "transparent" }
                }
            }
        }

        // ── marquee title ──
        Item {
            id: marqueeClip
            implicitWidth: rootMod.titleWidth
            width: rootMod.titleWidth
            height: 28
            anchors.verticalCenter: parent.verticalCenter
            // alpha-mask fade of the right edge: the scrolling title dissolves into
            // the real pixels behind it (no fixed colour → no seam on the translucent
            // pill). layer.enabled also clips to bounds like the old clip:true.
            layer.enabled: true
            layer.effect: MultiEffect {
                maskEnabled: true
                maskSource: marqueeFadeMask
                maskThresholdMin: 0.5
                maskSpreadAtMin: 0.5
            }

            // The title is the widget's own surface -- the transport glyphs beside
            // it keep their own clicks -- so this is where a click means "show me
            // the track", i.e. open the now-playing card.
            MouseArea {
                anchors.fill: parent
                acceptedButtons: Qt.LeftButton
                cursorShape: Qt.PointingHandCursor
                onClicked: { tip.hide(); root.mprisVisible = !root.mprisVisible }
            }

            Text {
                id: marqueeText
                anchors.verticalCenter: parent.verticalCenter
                text: rootMod.trackLabel
                color: rootMod.contentColor
                font.family: root.mono
                font.pixelSize: 12
                x: 0
                Behavior on color { ColorAnimation { duration: 200 } }
                onTextChanged: marqueeClip.resetMarquee()
            }
            onWidthChanged: if (marqueeText) resetMarquee()

            function resetMarquee() {
                marqueeAnim.stop()
                marqueeText.x = 0
                if (rootMod.visible && rootMod.playing && !rootMod.fullMode && rootMod.motionAllowed && marqueeText.implicitWidth > marqueeClip.width)
                    marqueeAnim.start()
            }

            Connections {
                target: rootMod
                function onPlayingChanged() { marqueeClip.resetMarquee() }
                function onVisibleChanged() { marqueeClip.resetMarquee() }   // stop/restart the scroll when the widget is hidden (toggle off)
            }

            SequentialAnimation {
                id: marqueeAnim
                loops: Animation.Infinite
                PauseAnimation  { duration: 2000 }
                NumberAnimation {
                    target: marqueeText; property: "x"
                    to: -(marqueeText.implicitWidth - marqueeClip.width + 4)
                    duration: Math.max(100, marqueeText.implicitWidth - marqueeClip.width + 4) * 20
                    easing.type: Easing.Linear
                }
                PauseAnimation  { duration: 900 }
                NumberAnimation { target: marqueeText; property: "x"; to: 0; duration: 0 }
            }
        }

        // ── equalizer canvas ──
        Canvas {
            id: eqCanvas
            implicitWidth: 16
            width: 16
            height: 14
            anchors.verticalCenter: parent.verticalCenter

            property color tint: rootMod.accentColor
            onTintChanged: requestPaint()

            onPaint: {
                var ctx = getContext("2d")
                ctx.clearRect(0, 0, width, height)

                var bars   = [rootMod.barH1, rootMod.barH2, rootMod.barH3]
                var bw     = 3
                var gap    = 2
                var totalW = bars.length * bw + (bars.length - 1) * gap
                var startX = (width - totalW) / 2
                var maxH   = height - 1
                var r      = bw / 2

                ctx.fillStyle = eqCanvas.tint

                for (var i = 0; i < bars.length; i++) {
                    var bh = Math.max(r * 2, bars[i] * maxH)
                    var x  = startX + i * (bw + gap)
                    var y  = height - bh

                    ctx.beginPath()
                    ctx.moveTo(x + r, y)
                    ctx.lineTo(x + bw - r, y)
                    ctx.arcTo(x + bw, y,      x + bw, y + r,      r)
                    ctx.lineTo(x + bw, y + bh - r)
                    ctx.arcTo(x + bw, y + bh, x + bw - r, y + bh, r)
                    ctx.lineTo(x + r,  y + bh)
                    ctx.arcTo(x,       y + bh, x, y + bh - r,      r)
                    ctx.lineTo(x, y + r)
                    ctx.arcTo(x, y,    x + r,  y,                   r)
                    ctx.closePath()
                    ctx.fill()
                }
            }

            Connections {
                target: rootMod
                function onBarH1Changed() { eqCanvas.requestPaint() }
                function onBarH2Changed() { eqCanvas.requestPaint() }
                function onBarH3Changed() { eqCanvas.requestPaint() }
            }
            MouseArea {
                anchors.fill: parent
                acceptedButtons: Qt.LeftButton
                cursorShape: Qt.PointingHandCursor
                onClicked: { tip.hide(); root.mprisVisible = !root.mprisVisible }
            }
            Component.onCompleted: requestPaint()
        }
    }

    // ── FULL: muse-style vinyl · centered waveform · transport state ──
    Item {
        id: fullRow
        visible: rootMod.active && rootMod.fullMode
        anchors.centerIn: parent
        implicitWidth: fullMuseCore.width
        implicitHeight: 28

        Item {
            id: fullMuseCore
            anchors.centerIn: parent
            width: 144
            height: 28

            Row {
                id: museRow
                anchors.centerIn: parent
                spacing: 7

                Item {
                    id: vinylMark
                    width: 18
                    height: 18
                    anchors.verticalCenter: parent.verticalCenter
                    transformOrigin: Item.Center

                    Canvas {
                        id: vinylCanvas
                        anchors.fill: parent
                        antialiasing: true
                        property color tint: rootMod.accentColor
                        onTintChanged: requestPaint()
                        onPaint: {
                            var ctx = getContext("2d")
                            ctx.clearRect(0, 0, width, height)
                            ctx.strokeStyle = tint
                            ctx.fillStyle = tint
                            ctx.lineWidth = 1.4

                            ctx.beginPath()
                            ctx.arc(width / 2, height / 2, 7.2, 0, Math.PI * 2)
                            ctx.stroke()
                            ctx.globalAlpha = 0.55
                            ctx.beginPath()
                            ctx.arc(width / 2, height / 2, 4.6, -0.35, 2.35)
                            ctx.stroke()
                            ctx.beginPath()
                            ctx.arc(width / 2, height / 2, 4.6, 2.8, 5.5)
                            ctx.stroke()
                            ctx.globalAlpha = 1
                            ctx.beginPath()
                            ctx.arc(width / 2, height / 2, 1.45, 0, Math.PI * 2)
                            ctx.fill()
                        }
                        Component.onCompleted: requestPaint()
                    }

                    NumberAnimation on rotation {
                        from: 0
                        to: 360
                        duration: 3200
                        loops: Animation.Infinite
                        running: rootMod.visible && rootMod.fullMode && rootMod.playing && rootMod.motionAllowed
                    }
                }

                Item {
                    id: museWaveform
                    width: 96
                    height: 22
                    anchors.verticalCenter: parent.verticalCenter

                    Canvas {
                        id: museCanvas
                        anchors.fill: parent
                        antialiasing: true
                        property color tint: rootMod.accentColor
                        onTintChanged: requestPaint()
                        onPaint: {
                            var ctx = getContext("2d")
                            ctx.clearRect(0, 0, width, height)
                            var levels = rootMod.museLevels
                            var count = rootMod.museBands
                            var barWidth = 2
                            var gap = (width - count * barWidth) / (count - 1)
                            var centerY = height / 2
                            var maxHalf = centerY - 1
                            ctx.fillStyle = tint

                            for (var i = 0; i < count; i++) {
                                var level = levels && levels[i] !== undefined ? levels[i] : 0.04
                                var half = 1 + level * (maxHalf - 1)
                                var x = i * (barWidth + gap)
                                var y = centerY - half
                                var barHeight = half * 2
                                var radius = barWidth / 2

                                ctx.beginPath()
                                ctx.moveTo(x + radius, y)
                                ctx.arcTo(x + barWidth, y, x + barWidth, y + radius, radius)
                                ctx.lineTo(x + barWidth, y + barHeight - radius)
                                ctx.arcTo(x + barWidth, y + barHeight, x + radius, y + barHeight, radius)
                                ctx.arcTo(x, y + barHeight, x, y + barHeight - radius, radius)
                                ctx.lineTo(x, y + radius)
                                ctx.arcTo(x, y, x + radius, y, radius)
                                ctx.closePath()
                                ctx.fill()
                            }
                        }

                        Connections {
                            target: rootMod
                            function onMuseLevelsChanged() { museCanvas.requestPaint() }
                        }
                        Component.onCompleted: requestPaint()
                    }
                }

                Item {
                    id: transportMark
                    anchors.verticalCenter: parent.verticalCenter
                    width: 16
                    height: 18

                    Row {
                        anchors.centerIn: parent
                        spacing: 3
                        visible: rootMod.playing
                        Repeater {
                            model: 2
                            Rectangle {
                                width: 3
                                height: 10
                                radius: 1
                                color: rootMod.accentColor
                            }
                        }
                    }

                    Canvas {
                        id: playCanvas
                        anchors.centerIn: parent
                        width: 11
                        height: 12
                        visible: !rootMod.playing
                        antialiasing: true
                        property color tint: rootMod.accentColor
                        onTintChanged: requestPaint()
                        onPaint: {
                            var ctx = getContext("2d")
                            ctx.clearRect(0, 0, width, height)
                            ctx.fillStyle = tint
                            ctx.beginPath()
                            ctx.moveTo(2, 1)
                            ctx.lineTo(width - 1, height / 2)
                            ctx.lineTo(2, height - 1)
                            ctx.closePath()
                            ctx.fill()
                        }
                        Component.onCompleted: requestPaint()
                    }
                }
            }

            MouseArea {
                anchors.fill: parent
                z: 2
                hoverEnabled: true
                acceptedButtons: Qt.LeftButton
                cursorShape: Qt.PointingHandCursor
                // Full mode shows no transport glyphs, so a click on it means the
                // same as a click on the compact title: open the now-playing card,
                // where play/pause (and Space) live. The wheel still skips tracks.
                onClicked: { tip.hide(); root.mprisVisible = !root.mprisVisible }
                onWheel: function(wheel) {
                    if (!rootMod.player || wheel.angleDelta.y === 0) return
                    if (wheel.angleDelta.y > 0) {
                        if (rootMod.player.canGoNext) rootMod.player.next()
                    } else if (rootMod.player.canGoPrevious) {
                        rootMod.player.previous()
                    }
                    wheel.accepted = true
                }
            }
        }
    }

    readonly property string tooltipText: player
        ? (player.trackArtist ? player.trackArtist + " - " + player.trackTitle : player.trackTitle)
        : ""

    TooltipMixin { id: tip; root: rootMod.root; owner: rootMod; text: rootMod.tooltipText }

    MouseArea {
        anchors.fill: parent
        hoverEnabled: true
        acceptedButtons: Qt.RightButton
        onEntered: { if (rootMod.tooltipText) tip.show() }
        onExited:  { tip.hide() }
        onClicked: { tip.hide(); root.mprisVisible = !root.mprisVisible }
    }
}

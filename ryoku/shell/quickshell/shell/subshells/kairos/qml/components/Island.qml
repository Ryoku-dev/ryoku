// Kairos: the island bar style. The clock rests as a small pill on the screen's
// centre line; hovering grows it there into android's first pull (the glance
// and the compact tiles), and the expand button grows the same pill on into the
// full quick-settings shade. The music island is a bubble to its left that opens
// straight into the now-playing panel while the pointer is over it.

import QtQuick
import QtQuick.Effects
import Ryoku.Blobs
import shell.services
import "../kit"
import "../settings" as S
import "../quicksettings" as Q
import "../IslandMetrics.js" as Island
import "." as C

Item {
    id: island

    // the launcher's own pill starts at the clock's exact size and covers it, so both rest
    property bool suspended: false
    property bool quickOpen: false
    property real screenHeight: 1080

    signal settingsToggled()
    signal quickSettingsToggled()
    signal quickCloseRequested()

    S.IslandSettings { id: cfg }

    readonly property var player: Media.player
    readonly property bool hasTrack: Media.present
    readonly property real topGap: cfg.topGap
    readonly property real restHeight: cfg.restHeight
    readonly property real restFontSize: cfg.restFontSize
    readonly property real band: topGap + restHeight

    // ── the quick-settings shade ─────────────────────────────────────────────
    readonly property real quickWidth: 472
    readonly property real quickPad: 16
    // taller pages scroll inside the shade
    readonly property real quickMax: Math.max(420, Math.min(860, island.screenHeight - island.topGap - 24))
    property real quickHeight: Math.min(island.quickMax, pane.contentHeight + 2 * island.quickPad)
    Behavior on quickHeight {
        enabled: !Md.still && island.quickProgress > 0.99
        Spring { speed: "fast" }
    }
    SpringValue { id: quickSpring; target: island.quickOpen ? 1 : 0 }
    readonly property real quickProgress: quickSpring.value
    // the pointer drifting off never closes the shade, only a click outside or escape
    onQuickOpenChanged: {
        if (!island.quickOpen)
            return;
        Notifs.markSeen();
        Qt.callLater(function () { clockPill.forceActiveFocus(); });
    }

    // a compact tile's detail page opens the shade on that page; its other actions fold the glance
    property string pendingPage: ""
    function openShadeAt(page) {
        island.pendingPage = page;
        if (!island.quickOpen)
            island.quickSettingsToggled();
        else
            pane.go(page);
    }
    Connections {
        target: pane
        function onVisibleChanged() {
            if (pane.visible && island.pendingPage !== "") {
                pane.go(island.pendingPage);
                island.pendingPage = "";
            }
        }
    }
    QtObject {
        id: glanceHost
        function go(page) { island.openShadeAt(page); }
        function close() { clockGrace.stop(); island.clockHovered = false; }
    }

    // ── the music island ─────────────────────────────────────────────────────
    // a scrub may stray off the panel, so it holds the panel open until release
    readonly property bool full: (island.musicHovered || music.scrubbing)
        && island.hasTrack && !island.suspended && !island.quickOpen
    SpringValue { id: fullSpring; target: island.full ? 1 : 0 }
    readonly property real fullProgress: fullSpring.value
    SpringValue { id: trackSpring; target: (island.hasTrack && cfg.music) ? 1 : 0 }
    readonly property real trackPresence: trackSpring.value

    // ── the clock island ─────────────────────────────────────────────────────
    // hovering a heads-up holds the card, it shouldn't open the clock under it
    readonly property bool clockWanted: (island.clockHovered && !headsUp.showing) || island.quickOpen
    // the launcher's pill takes over at the clock's resting size, so the island drops there at once
    SpringValue { id: clockSpring; target: (island.clockWanted && !island.suspended) ? 1 : 0 }
    onSuspendedChanged: if (island.suspended) clockSpring.snap()
    readonly property real clockProgress: clockSpring.value
    readonly property real glanceReveal: Math.max(0, Math.min(1, (island.clockProgress - 0.5) / 0.4))
        * Math.max(0, 1 - 2 * island.quickProgress)

    // one grace timer each, so crossing the gap between the islands collapses neither mid-reach;
    // the music opens only after a short dwell so a pointer passing along the top edge leaves it shut
    property bool musicHovered: false
    property bool clockHovered: false
    Timer {
        id: clockGrace
        interval: 320
        onTriggered: island.clockHovered = false
    }
    Timer {
        id: musicIntent
        interval: 140
        onTriggered: island.musicHovered = true
    }
    Timer {
        id: musicGrace
        interval: 320
        onTriggered: island.musicHovered = false
    }

    // ── geometry ─────────────────────────────────────────────────────────────
    // measured at a fixed size so the growing clock can't feed back into its own width
    readonly property real restWidth: Math.round(metrics.implicitWidth + 2 * cfg.restPadX)
    readonly property real restingWidth: island.restWidth + Math.max(
        (osdLayer.wantedWidth - island.restWidth) * osdLayer.progress,
        (headsUp.wantedWidth - island.restWidth) * headsUp.progress)
    readonly property real restingHeight: island.restHeight
        + (headsUp.wantedHeight - island.restHeight) * headsUp.progress
    readonly property real hoverWidth: Math.round(island.restingWidth
        + (cfg.hoverWidth - island.restingWidth) * island.clockProgress)
    readonly property bool trayHostActive: island.clockWanted && !island.suspended && !island.quickOpen
    property real trayExtra: trayFlyout.extraHeight
    Behavior on trayExtra {
        enabled: !Md.still
        Spring { speed: "fast" }
    }
    readonly property real hoverHeight: Math.round(island.restingHeight
        + (cfg.hoverHeight + island.trayExtra - island.restingHeight) * island.clockProgress)
    readonly property real restingRadius: cfg.restHeight / 2 + (28 - cfg.restHeight / 2) * headsUp.progress
    readonly property real baseRadius: island.restingRadius
        + (cfg.hoverRadius - island.restingRadius) * island.clockProgress
    readonly property real clockWidth: Math.round(island.hoverWidth
        + (island.quickWidth - island.hoverWidth) * island.quickProgress)
    readonly property real clockHeight: Math.round(island.hoverHeight
        + (island.quickHeight - island.hoverHeight) * island.quickProgress)
    readonly property real clockRadius: island.baseRadius + (32 - island.baseRadius) * island.quickProgress

    // the clock rests centred and grows symmetric, so it needs this much room left of its resting
    // edge (or the music's reach) without leaving the item
    readonly property real musicExtent: (music.width + Island.bubbleGap) * island.trackPresence
    readonly property real clockGrowMax: (Math.max(cfg.hoverWidth, island.quickWidth,
        osdLayer.wantedWidth, headsUp.wantedWidth) - island.restWidth) / 2
    readonly property real clockRestLeft: Math.max(island.clockGrowMax, island.musicExtent)
    // fixed in item coordinates and landed on the screen's centre line by the scene
    readonly property real clockRestCentre: island.clockRestLeft + island.restWidth / 2

    width: Math.round(island.clockRestCentre + island.clockWidth / 2)
    height: Math.round(Math.max(island.hasTrack ? music.height : 0, island.clockHeight))

    readonly property color fill: Theme.shadow
    readonly property color ink: Theme.ink(fill, 7)
    // the bubble buds out of the clock when a track starts and is absorbed back as the shade opens
    readonly property real musicPresence: Math.max(0, Math.min(1, island.trackPresence))
        * (1 - Math.max(0, Math.min(1, island.quickProgress)))

    // the clock and the music are one liquid body: within the smoothing reach they melt together
    // and each shape jiggles with its own speed as it moves
    BlobGroup {
        id: liquid
        color: island.fill
        smoothing: 12
        borderWidth: 1
        borderColor: Theme.blend(Qt.rgba(island.ink.r, island.ink.g, island.ink.b, 0.16), island.fill)
        shadowStrength: 0
        shadowSize: 0
    }
    BlobRect {
        id: musicBody
        group: liquid
        z: -1
        // every shape in a group draws even when hidden, so an absent bubble shrinks to nothing
        // tucked inside the clock
        readonly property bool present: island.musicPresence > 0.01
        width: musicBody.present ? music.width * island.musicPresence : 0
        height: musicBody.present ? music.height * (0.4 + 0.6 * island.musicPresence) : 0
        x: musicBody.present ? music.x + music.width - musicBody.width : clockPill.x + clockPill.width / 2
        y: musicBody.present ? (music.height - musicBody.height) / 2 * (1 - island.musicPresence) : clockPill.height / 2
        radius: Math.min(music.radius, musicBody.height / 2, musicBody.width / 2)
        // only the bare bubble jiggles, a stretched panel body would slide out from under its content
        deformScale: island.fullProgress > 0.05 ? 0 : 0.0004
    }
    BlobRect {
        id: clockBody
        group: liquid
        z: -1
        x: clockPill.x
        y: clockPill.y
        width: clockPill.width
        height: clockPill.height
        radius: Math.min(island.clockRadius, clockPill.height / 2)
        // its centre only moves because it grows, and a velocity stretch would squeeze the body
        // narrower than the content it holds
        deformScale: 0
    }

    C.MusicSurface {
        id: music
        // pinned to the clock's live left edge so the growing clock pushes it aside
        x: island.clockRestCentre - island.clockWidth / 2 - Island.bubbleGap - width
        z: 0
        player: island.player
        full: island.fullProgress
        ink: island.ink
        accent: Theme.primary
        opacity: island.musicPresence
        enabled: !island.quickOpen

        HoverHandler {
            onHoveredChanged: {
                if (hovered) {
                    musicGrace.stop();
                    if (!island.musicHovered)
                        musicIntent.restart();
                } else {
                    musicIntent.stop();
                    musicGrace.restart();
                }
            }
        }
    }

    Rectangle {
        id: clockPill
        x: island.clockRestCentre - island.clockWidth / 2
        y: 0
        z: 2
        width: island.clockWidth
        height: island.clockHeight
        radius: island.clockRadius
        // the liquid body underneath draws the pill; this only holds and clips the content
        color: "transparent"
        clip: true

        // presses on the shade's own gaps stay on the shade instead of reaching the click-away area
        MouseArea {
            anchors.fill: parent
            enabled: island.quickOpen
            acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
            onPressed: function (event) { event.accepted = true; }
        }

        HoverHandler {
            onHoveredChanged: {
                if (hovered) {
                    clockGrace.stop();
                    island.clockHovered = true;
                } else {
                    clockGrace.restart();
                }
            }
        }

        focus: island.quickOpen
        Keys.onPressed: function (e) {
            if (!island.quickOpen) return;
            if (e.key !== Qt.Key_Escape) return;
            if (pane.page !== "home") pane.back();
            else island.quickCloseRequested();
            e.accepted = true;
        }

        C.Clock { id: clock }

        C.IslandOsd {
            id: osdLayer
            anchors { left: parent.left; right: parent.right; top: parent.top }
            height: cfg.restHeight
            active: cfg.islandOsd
            blocked: island.clockWanted || island.suspended
        }

        C.HeadsUp {
            id: headsUp
            width: headsUp.wantedWidth
            height: parent.height
            x: (parent.width - width) / 2
            active: cfg.islandNotifications
            blocked: island.quickOpen || island.suspended
            yielding: osdLayer.showing
            onOpenShade: island.quickSettingsToggled()
        }

        // red while recording, accent for unread notifications
        Rectangle {
            readonly property real rest: (1 - island.clockProgress) * (1 - osdLayer.progress) * (1 - headsUp.progress)
            x: Math.round(cfg.restPadX / 2 - width / 2)
            y: Math.round((cfg.restHeight - height) / 2)
            width: 6
            height: 6
            radius: 3
            color: Recorder.active ? Theme.error : Theme.primary
            visible: (Recorder.active || Notifs.hasUnread) && rest > 0.01
            opacity: Math.max(0, Math.min(1, rest))
        }

        Text {
            id: metrics
            visible: false
            text: clock.time
            font.family: Theme.mono
            font.pixelSize: cfg.restFontSize
            font.weight: Font.Medium
        }

        // its groups are pinned to the pill's edges, so growing slides them instead of re-laying out
        C.Glance {
            id: glance
            anchors.fill: parent
            reveal: island.glanceReveal
            uiScale: cfg.scale
            tiles: cfg.glanceTiles
            showLocation: cfg.showLocation
            showWeather: cfg.glanceWeather
            showBattery: cfg.glanceBattery
            host: glanceHost
            enabled: island.glanceReveal > 0.95
            onExpandRequested: island.quickSettingsToggled()
            onSettingsRequested: island.settingsToggled()
        }

        // the shared element: one Text at the glance's size, scaled down into the resting pill,
        // so the move is a transform and the glyphs are never rebuilt mid-motion
        Text {
            readonly property real p: island.clockProgress
            readonly property real restScale: cfg.restFontSize / cfg.hoverFontSize
            transformOrigin: Item.TopLeft
            scale: restScale + (1 - restScale) * p
            x: (parent.width - width * scale) / 2 * (1 - p) + (glance.x + Island.glancePad * cfg.scale) * p
            y: (cfg.restHeight - height * scale) / 2 * (1 - p) + Island.glanceClockY * cfg.scale * p
            text: clock.time
            color: island.ink
            font.family: Theme.mono
            font.pixelSize: cfg.hoverFontSize
            font.weight: Font.Medium
            opacity: Math.max(0, (1 - 2 * island.quickProgress) * (1 - osdLayer.progress) * (1 - headsUp.progress))
        }

        Q.QuickSettingsPane {
            id: pane
            width: island.quickWidth - 2 * island.quickPad
            height: Math.max(0, parent.height - 2 * island.quickPad)
            x: (parent.width - width) / 2
            y: island.quickPad
            visible: island.quickProgress > 0.01
            opacity: Math.max(0, Math.min(1, (island.quickProgress - 0.25) / 0.45))
            enabled: island.quickProgress > 0.95
            host: island
        }

        C.TrayFlyout {
            id: trayFlyout
            anchors { bottom: parent.bottom; horizontalCenter: parent.horizontalCenter }
            width: cfg.hoverWidth
            ink: island.ink
            accent: Theme.primary
            hostActive: island.trayHostActive
            visible: cfg.tray && island.glanceReveal > 0.01
            opacity: island.glanceReveal
            enabled: island.glanceReveal > 0.95
        }
    }

    // a battery at 20% or lower rings the island in red, with a brief pulse every few seconds
    Rectangle {
        id: lowBatteryRing
        anchors.fill: clockPill
        anchors.margins: -5
        radius: clockPill.radius + 5
        color: "transparent"
        border.width: 2
        border.color: Theme.error
        visible: Battery.present && Battery.low
        opacity: 1
        z: 1

        SequentialAnimation on opacity {
            running: lowBatteryRing.visible && !Md.still
            loops: Animation.Infinite
            PauseAnimation { duration: 4200 }
            NumberAnimation { from: 1; to: 0.35; duration: 500; easing.type: Easing.InOutSine }
            NumberAnimation { from: 0.35; to: 1; duration: 500; easing.type: Easing.InOutSine }
        }
    }

    // shadows sit under the liquid body, one per shape so the gap between them stays clear
    RectangularShadow {
        anchors.fill: musicBody
        radius: musicBody.radius
        blur: 30
        spread: 2
        offset: Qt.vector2d(0, 6)
        color: Qt.rgba(0, 0, 0, 0.6)
        opacity: island.musicPresence
        visible: musicBody.present
        z: -2
    }
    RectangularShadow {
        anchors.fill: clockPill
        radius: clockBody.radius
        blur: 24
        spread: 2
        offset: Qt.vector2d(0, 4)
        color: Qt.rgba(0, 0, 0, 0.6)
        z: -2
    }
}

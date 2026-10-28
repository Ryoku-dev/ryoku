pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Widgets
import Quickshell.Services.Mpris
import ".."
import shell.services
import "../../../components"
import Ryoku.Ui.Singletons

// Dock now-playing card, opened on chip hover via MusicPreview (mirrors
// DockPreviewPopout's hover/mask model): a hairline paper plate beside a bone
// spectrum column, all bone-on-black with the sleeve the only colour.
Popout {
    id: root

    readonly property real pad: 12 * root.s
    readonly property real recW: 196 * root.s
    readonly property real specW: 74 * root.s
    readonly property real artSize: 140 * root.s
    readonly property color line: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.14)
    readonly property color faint: Qt.rgba(Theme.onSurfaceVariant.r, Theme.onSurfaceVariant.g, Theme.onSurfaceVariant.b, 0.5)

    readonly property var player: Media.player
    readonly property bool present: Media.present
    readonly property bool playing: Media.playing
    readonly property string artUrl: root.player ? (root.player.trackArtUrl || "") : ""
    readonly property bool seekable: root.present && !Media.radio && root.player !== null && root.player.length > 0
    readonly property real posn: root.player ? root.player.position : 0
    readonly property real len: root.seekable ? root.player.length : 0
    property real dragFrac: -1
    readonly property real frac: root.dragFrac >= 0 ? root.dragFrac : (root.len > 0 ? Math.max(0, Math.min(1, root.posn / root.len)) : 0)

    function fmtTime(sec) {
        sec = Math.max(0, Math.floor(sec));
        return Math.floor(sec / 60) + ":" + (sec % 60 < 10 ? "0" : "") + (sec % 60);
    }

    edge: MusicPreview.edge
    hoverOpen: false
    closeDelay: 300
    arrivalDelay: 1200
    edgeGap: MusicPreview.margin > 0 ? MusicPreview.margin : 10 * root.s
    radius: Theme.radiusWindow
    triggerHovered: MusicPreview.hovered && root.present
    alongCenter: {
        if (MusicPreview.gx < 0)
            return -1;
        const p = root.mapFromGlobal(MusicPreview.gx, MusicPreview.gy);
        return (root.edge === "left" || root.edge === "right") ? p.y : p.x;
    }
    Behavior on alongCenter {
        enabled: root.heldOpen
        NumberAnimation { duration: Motion.menuSlide; easing.type: Motion.menuSlideCurve }
    }
    openW: root.recW + root.specW
    // derived from content so a two-line title can never collide with the foot.
    openH: recCol.implicitHeight + root.pad * 2

    readonly property bool showing: root.prog > 0.004
    onShowingChanged: AudioBars.setActive(root, root.showing)
    Component.onDestruction: AudioBars.setActive(root, false)

    Timer {
        interval: 500
        repeat: true
        running: root.showing && root.player !== null && root.player.isPlaying
        onTriggered: root.player.positionChanged()
    }

    function cycleLoop() {
        if (!root.player)
            return;
        if (root.player.loopState === MprisLoopState.None)
            root.player.loopState = MprisLoopState.Playlist;
        else if (root.player.loopState === MprisLoopState.Playlist)
            root.player.loopState = MprisLoopState.Track;
        else
            root.player.loopState = MprisLoopState.None;
    }

    // play/pause is the one emphasis: an inverted bone plate, not a tint.
    component Ctl: Item {
        id: cb
        property string glyph: ""
        property bool plate: false
        property bool active: false
        property bool on: true
        signal act()
        readonly property real d: cb.plate ? 32 * root.s : 25 * root.s
        width: cb.d
        height: cb.d
        opacity: cb.on ? 1 : 0.3
        Rectangle {
            anchors.fill: parent
            radius: Theme.radiusWidget
            visible: cb.plate
            color: Theme.inverseSurface
        }
        GlyphIcon {
            anchors.centerIn: parent
            width: (cb.plate ? 15 : 14) * root.s
            height: width
            name: cb.glyph
            stroke: 1.8
            color: cb.plate ? Theme.inverseOnSurface : (hh.hovered || cb.active ? Theme.onSurface : root.faint)
        }
        HoverHandler { id: hh; enabled: cb.on; cursorShape: Qt.PointingHandCursor }
        TapHandler { enabled: cb.on; onSingleTapped: cb.act() }
    }

    PopoutCard { anchors.fill: parent }

    Rectangle {
        width: root.recW
        height: parent.height
        radius: Theme.radiusWindow
        color: Theme.surface
        border.width: Theme.borderWidth
        border.color: Theme.outline

        Column {
            id: recCol
            anchors { top: parent.top; left: parent.left; right: parent.right; margins: root.pad }
            spacing: 10 * root.s

            Eyebrow { label: I18n.tr("Now Playing"); s: root.s }

            Item {
                width: root.artSize
                height: root.artSize
                anchors.horizontalCenter: parent.horizontalCenter
                ClippingRectangle {
                    anchors.fill: parent
                    radius: Theme.radiusWidget
                    color: Theme.surfaceContainer
                    GlyphIcon {
                        visible: art.status !== Image.Ready
                        anchors.centerIn: parent
                        width: 36 * root.s
                        height: width
                        name: "music"
                        stroke: 1.4
                        color: root.faint
                    }
                    Image {
                        id: art
                        anchors.fill: parent
                        source: Music.artUrl.length > 0 ? Music.artUrl : root.artUrl
                        fillMode: Image.PreserveAspectCrop
                        asynchronous: true
                        cache: true
                        smooth: true
                        mipmap: true
                        sourceSize.width: Math.round(root.artSize * 2)
                        sourceSize.height: Math.round(root.artSize * 2)
                    }
                }
                Rectangle {
                    anchors.fill: parent
                    radius: Theme.radiusWidget
                    color: "transparent"
                    border.width: 1
                    border.color: root.line
                }
                Repeater {
                    model: 4
                    delegate: Item {
                        required property int index
                        readonly property bool rightCol: index === 1 || index === 3
                        readonly property bool botRow: index >= 2
                        readonly property real arm: 5 * root.s
                        x: rightCol ? parent.width - arm : 0
                        y: botRow ? parent.height - arm : 0
                        width: arm
                        height: arm
                        Rectangle { width: parent.arm; height: 1; color: root.faint; y: parent.botRow ? parent.arm - 1 : 0 }
                        Rectangle { width: 1; height: parent.arm; color: root.faint; x: parent.rightCol ? parent.arm - 1 : 0 }
                    }
                }
            }

            Column {
                width: parent.width
                spacing: 2 * root.s
                Text {
                    width: parent.width
                    text: root.player ? (root.player.trackTitle || "") : ""
                    color: Theme.onSurface
                    font.family: Theme.display
                    font.pixelSize: 18 * root.s
                    font.weight: Font.Medium
                    lineHeight: 0.98
                    wrapMode: Text.WordWrap
                    maximumLineCount: 2
                    elide: Text.ElideRight
                }
                Text {
                    width: parent.width
                    visible: text.length > 0
                    text: root.player ? Theme.joinArtists(root.player.trackArtists, root.player.trackArtist) : ""
                    color: Theme.onSurfaceVariant
                    font.family: Theme.fontPrimary
                    font.pixelSize: 12.5 * root.s
                    elide: Text.ElideRight
                }
                Text {
                    width: parent.width
                    visible: text.length > 0
                    text: root.player ? (root.player.trackAlbum || "") : ""
                    color: root.faint
                    font.family: Theme.fontPrimary
                    font.pixelSize: 11 * root.s
                    elide: Text.ElideRight
                }
            }

            Column {
                width: parent.width
                spacing: 5 * root.s
                visible: root.seekable
                Item {
                    width: parent.width
                    height: 10 * root.s
                    Rectangle {
                        id: seekRule
                        anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter }
                        height: 1
                        color: root.line
                        Rectangle {
                            anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                            width: parent.width * root.frac
                            height: 1
                            color: Theme.onSurface
                        }
                    }
                    Rectangle {
                        width: 2 * root.s
                        height: parent.height
                        color: Theme.onSurface
                        x: Math.max(0, Math.min(seekRule.width - width, seekRule.width * root.frac - width / 2))
                    }
                    MouseArea {
                        anchors.fill: parent
                        anchors.topMargin: -4 * root.s
                        anchors.bottomMargin: -4 * root.s
                        enabled: root.player !== null && root.player.canSeek
                        cursorShape: Qt.PointingHandCursor
                        onPressed: mouse => root.dragFrac = Math.max(0, Math.min(1, mouse.x / width))
                        onPositionChanged: mouse => { if (pressed) root.dragFrac = Math.max(0, Math.min(1, mouse.x / width)); }
                        onReleased: {
                            if (root.player && root.len > 0)
                                root.player.position = root.dragFrac * root.len;
                            root.dragFrac = -1;
                        }
                    }
                }
                Item {
                    width: parent.width
                    height: elapsed.implicitHeight
                    Text {
                        id: elapsed
                        anchors.left: parent.left
                        text: root.fmtTime(root.posn)
                        color: root.faint
                        font.family: Theme.mono
                        font.pixelSize: 9 * root.s
                    }
                    Text {
                        anchors.right: parent.right
                        text: root.fmtTime(root.len)
                        color: root.faint
                        font.family: Theme.mono
                        font.pixelSize: 9 * root.s
                    }
                }
            }

            Row {
                anchors.horizontalCenter: parent.horizontalCenter
                spacing: 6 * root.s
                Ctl { glyph: "repeat"; active: root.player !== null && root.player.loopState !== MprisLoopState.None; on: root.player !== null && root.player.loopSupported; onAct: root.cycleLoop() }
                Ctl { glyph: "prev"; on: root.player !== null && root.player.canGoPrevious; onAct: if (root.player) root.player.previous() }
                Ctl { plate: true; glyph: root.playing ? "pause" : "play"; on: root.player !== null && root.player.canTogglePlaying; onAct: Media.toggle() }
                Ctl { glyph: "next"; on: root.player !== null && root.player.canGoNext; onAct: if (root.player) root.player.next() }
                Ctl { glyph: "shuffle"; active: root.player !== null && root.player.shuffle; on: root.player !== null && root.player.shuffleSupported; onAct: if (root.player) root.player.shuffle = !root.player.shuffle }
            }
        }
    }

    Rectangle {
        x: root.recW
        anchors { top: parent.top; bottom: parent.bottom; topMargin: root.pad; bottomMargin: root.pad }
        width: 1
        color: root.line
    }

    Item {
        x: root.recW
        width: root.specW
        anchors { top: parent.top; bottom: parent.bottom }
        Text {
            id: seal
            anchors { top: parent.top; topMargin: root.pad; horizontalCenter: parent.horizontalCenter }
            text: "\u97f3"
            color: root.faint
            font.family: Theme.fontJp
            font.pixelSize: 13 * root.s
        }
        VerticalCava {
            anchors { top: seal.bottom; bottom: parent.bottom; horizontalCenter: parent.horizontalCenter; topMargin: 8 * root.s; bottomMargin: root.pad }
            width: parent.width - 18 * root.s
            bands: 38
            s: root.s
            running: root.showing && root.playing
            lowColor: root.faint
            highColor: Theme.onSurface
            axisColor: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.16)
        }
    }
}

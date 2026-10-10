import QtQuick
import QtMultimedia
import Ryoku.Ui.Singletons

Item {
    id: media

    property url source: ""
    property string mode: "cover"       // cover | hero | plate | view
    property bool active: true
    property bool actualPixels: false
    property color surface: Tokens.paper
    property string fallbackText: ""
    property color fallbackSurface: Tokens.paperLift
    property color fallbackInk: Tokens.inkMuted
    property bool timedOut: false

    readonly property string cleanSource: String(source).split(/[?#]/)[0].toLowerCase()
    readonly property bool video: /\.(mp4|webm|mkv|mov|m4v)$/.test(cleanSource)
    readonly property bool animated: !video && /\.(gif|webp)$/.test(cleanSource)
    readonly property bool framed: mode === "plate" || mode === "view"
    readonly property bool fitted: framed || mode === "hero"
    readonly property bool nativeDecode: actualPixels || fitted || animated
    readonly property int imageFit: fitted ? Image.PreserveAspectFit : Image.PreserveAspectCrop
    readonly property int videoFit: fitted ? VideoOutput.PreserveAspectFit : VideoOutput.PreserveAspectCrop
    readonly property real dpr: Window.window && Window.window.screen
            ? Math.max(1, Window.window.screen.devicePixelRatio) : 1
    readonly property real inset: mode === "plate" ? Tokens.s3 : 0
    readonly property real innerW: Math.max(1, width - inset * 2)
    readonly property real innerH: Math.max(1, height - inset * 2)
    readonly property bool windowFocused: !Window.window || Window.window.active
    readonly property bool shouldPlay: active && visible && windowFocused
    readonly property bool ready: video ? videoPlayer.hasVideo
            : (animated ? moving.status === AnimatedImage.Ready : still.status === Image.Ready)
    readonly property bool failed: cleanSource !== "" && (video
            ? videoPlayer.error !== MediaPlayer.NoError
            : (animated ? moving.status === AnimatedImage.Error : still.status === Image.Error))
    readonly property bool fallbackNeeded: !ready && (cleanSource === "" || failed || timedOut)
    readonly property var front: animated ? moving : still
    readonly property real nativeLogicalWidth: video || !nativeDecode ? innerW
            : Math.max(1, front.implicitWidth / dpr)
    readonly property real nativeLogicalHeight: video || !nativeDecode ? innerH
            : Math.max(1, front.implicitHeight / dpr)
    onSourceChanged: timedOut = false
    readonly property bool playing: video
            ? videoPlayer.playbackState === MediaPlayer.PlayingState
            : (animated && moving.playing)
    readonly property real paintedWidth: video ? videoOutput.contentRect.width : front.paintedWidth
    readonly property real paintedHeight: video ? videoOutput.contentRect.height : front.paintedHeight

    clip: true

    Rectangle {
        anchors.fill: parent
        color: media.surface
    }

    Rectangle {
        objectName: "ryostore-media-fallback"
        anchors.fill: parent
        anchors.margins: media.inset
        visible: media.fallbackNeeded && media.fallbackText !== ""
        color: media.fallbackSurface

        Text {
            objectName: "ryostore-media-fallback-text"
            anchors.centerIn: parent
            text: media.fallbackText
            color: media.fallbackInk
            font.family: Tokens.display
            font.pixelSize: Math.max(Tokens.fHero, Math.min(parent.width, parent.height) * 0.24)
            font.weight: Font.Medium
        }
    }

    Rectangle {
        id: skeleton
        objectName: "ryostore-media-skeleton"
        anchors.fill: parent
        anchors.margins: media.inset
        visible: media.cleanSource !== "" && !media.ready && !media.fallbackNeeded
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineSoft
        clip: true

        Rectangle {
            width: parent.width * 0.28
            height: parent.height
            color: Tokens.tint5
            x: media.shouldPlay ? -width : (parent.width - width) / 2
            XAnimator on x {
                from: -skeleton.width * 0.28
                to: skeleton.width
                duration: 1100
                loops: Animation.Infinite
                running: skeleton.visible && media.shouldPlay
            }
        }
    }

    Timer {
        interval: 4000
        running: media.cleanSource !== "" && !media.ready && !media.failed && !media.timedOut && media.visible
        onTriggered: media.timedOut = true
    }

    Image {
        id: still
        anchors.centerIn: parent
        source: !media.video && !media.animated ? media.source : ""
        sourceSize: media.nativeDecode
                ? Qt.size(0, 0)
                : Qt.size(Math.max(1, Math.ceil(media.innerW * media.dpr)),
                          Math.max(1, Math.ceil(media.innerH * media.dpr)))
        width: media.actualPixels
                ? media.nativeLogicalWidth
                : Math.min(media.innerW, media.nativeLogicalWidth)
        height: media.actualPixels
                ? media.nativeLogicalHeight
                : Math.min(media.innerH, media.nativeLogicalHeight)
        fillMode: media.imageFit
        asynchronous: true
        cache: true
        retainWhileLoading: true
        smooth: true
        mipmap: true
        visible: source !== "" && status === Image.Ready
    }

    AnimatedImage {
        id: moving
        anchors.centerIn: parent
        source: media.animated ? media.source : ""
        width: media.actualPixels
                ? media.nativeLogicalWidth
                : Math.min(media.innerW, media.nativeLogicalWidth)
        height: media.actualPixels
                ? media.nativeLogicalHeight
                : Math.min(media.innerH, media.nativeLogicalHeight)
        fillMode: media.imageFit
        asynchronous: true
        cache: true
        retainWhileLoading: true
        smooth: true
        mipmap: true
        playing: media.shouldPlay && visible
        visible: source !== "" && status === AnimatedImage.Ready
    }

    VideoOutput {
        id: videoOutput
        anchors.fill: parent
        anchors.margins: media.inset
        visible: media.video && videoPlayer.hasVideo
        fillMode: media.videoFit
    }

    MediaPlayer {
        id: videoPlayer
        source: media.video ? media.source : ""
        videoOutput: videoOutput
        loops: MediaPlayer.Infinite
        audioOutput: AudioOutput { muted: true }

        function syncPlayback() {
            if (media.shouldPlay && source.toString() !== "" && media.video)
                play();
            else
                pause();
        }

        onSourceChanged: syncPlayback()
        onMediaStatusChanged: syncPlayback()
    }

    Connections {
        target: media
        function onShouldPlayChanged() { videoPlayer.syncPlayback(); }
    }

    Rectangle {
        visible: media.framed && media.ready && media.paintedWidth > 0 && media.paintedHeight > 0
        width: media.paintedWidth
        height: media.paintedHeight
        anchors.centerIn: parent
        color: "transparent"
        border.width: Tokens.border
        border.color: Tokens.line
    }
}

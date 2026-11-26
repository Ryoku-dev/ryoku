import QtQuick
import QtMultimedia

// a local card picture, moving: an animated GIF or WebP, or a badge holder's short video looping
// without sound. CardFigure loads it by URL only while the card is pointed at, so at rest
// nothing here exists and nothing decodes; it is framed and filled like the still under it
Item {
    id: motion

    property url url
    property bool fit: false
    readonly property bool video: /\.(mp4|webm|mkv)$/.test(String(motion.url))
    // a frame is up, so the still beneath can step aside
    readonly property bool ready: motion.video
        ? player.hasVideo && player.playbackState === MediaPlayer.PlayingState
        : animated.status === Image.Ready

    AnimatedImage {
        id: animated
        anchors.fill: parent
        visible: !motion.video
        source: motion.video ? "" : motion.url
        fillMode: motion.fit ? Image.PreserveAspectFit : Image.PreserveAspectCrop
        cache: false
        smooth: true
        mipmap: true
        asynchronous: true
    }

    // no audio output: the picture is silent, and its sound is never decoded
    MediaPlayer {
        id: player
        source: motion.video ? motion.url : ""
        loops: MediaPlayer.Infinite
        videoOutput: output
        onSourceChanged: if (motion.video) player.play()
    }
    VideoOutput {
        id: output
        anchors.fill: parent
        visible: motion.video
        fillMode: motion.fit ? VideoOutput.PreserveAspectFit : VideoOutput.PreserveAspectCrop
    }
}

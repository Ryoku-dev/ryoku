import QtQuick
import QtMultimedia
import Quickshell.Widgets

// A looping visual backdrop for the now-playing surfaces: the track's Spotify
// Canvas, a chosen video/gif, or a still, whichever the widget hands it. Video
// runs muted through QtMultimedia (the same pipeline the wallpaper picker uses),
// a gif through AnimatedImage, anything else as an Image; an empty source draws
// nothing. Everything fills by aspect-crop and clips to the given radius, and
// playback is gated on `active` so a hidden or idle surface spins no decoder.
Item {
    id: root

    property string source: ""     // path/url: .mp4/.webm/.mkv/.mov/.m4v, .gif, or an image
    property bool active: true      // play gate (surface visible)
    property real radius: 0

    readonly property string lc: root.source.toLowerCase()
    readonly property bool hasSource: root.source.length > 0
    readonly property bool isVideo: /\.(mp4|webm|mkv|mov|m4v|avi)(\?|$)/.test(root.lc)
    readonly property bool isGif: /\.gif(\?|$)/.test(root.lc)

    readonly property bool displayed: root.visible && root.hasSource && width > 0 && height > 0
    readonly property size decodeSize: Qt.size(
        Math.max(1, Math.ceil(width * Screen.devicePixelRatio)),
        Math.max(1, Math.ceil(height * Screen.devicePixelRatio)))

    // a bare filesystem path becomes a file URL; http(s)/file/qrc pass through.
    readonly property url srcUrl: !root.hasSource ? ""
        : (/^(https?|file|qrc):/.test(root.source) ? root.source : "file://" + root.source)

    ClippingRectangle {
        anchors.fill: parent
        radius: root.radius
        color: "transparent"

        Loader {
            anchors.fill: parent
            active: root.isVideo && root.displayed
            sourceComponent: videoComp
        }
        Component {
            id: videoComp
            Item {
                anchors.fill: parent
                MediaPlayer {
                    id: mp
                    source: root.srcUrl
                    loops: MediaPlayer.Infinite
                    videoOutput: vout
                    audioOutput: AudioOutput { muted: true }
                    // play only once the clip is actually loaded, and re-assert
                    // it whenever the surface is shown again or the source is
                    // swapped back in: play() before LoadedMedia is dropped,
                    // which is why toggling away and back used to freeze it.
                    readonly property bool go: root.active && root.hasSource
                    function sync() {
                        if (mp.go && mp.mediaStatus >= MediaPlayer.LoadedMedia)
                            mp.play();
                        else if (!mp.go)
                            mp.pause();
                    }
                    onGoChanged: sync()
                    onMediaStatusChanged: sync()
                    onSourceChanged: sync()
                    Component.onCompleted: sync()
                }
                VideoOutput {
                    id: vout
                    anchors.fill: parent
                    fillMode: VideoOutput.PreserveAspectCrop
                }
            }
        }

        AnimatedImage {
            anchors.fill: parent
            visible: root.isGif && root.hasSource
            source: root.displayed && root.isGif ? root.srcUrl : ""
            sourceSize: root.decodeSize
            playing: root.active && visible
            // Sequential network sources need cached frames to loop.
            cache: /^https?:/i.test(root.source)
            smooth: true
            asynchronous: true
            fillMode: Image.PreserveAspectCrop
        }

        Image {
            anchors.fill: parent
            visible: root.hasSource && !root.isVideo && !root.isGif
            source: root.displayed && !root.isVideo && !root.isGif ? root.srcUrl : ""
            sourceSize: root.decodeSize
            asynchronous: true
            cache: false
            smooth: true
            mipmap: true
            fillMode: Image.PreserveAspectCrop
        }
    }
}

import QtQuick
import QtQuick.Effects
import Ryoku.Ui.Singletons

// Product artwork with four deliberate presentations:
//   cover  a crop-filled tile (the browse grid, thumbnails)
//   hero   a cinematic full-bleed frame that fades into the surface
//   plate  the whole preview shown as a framed gallery plate, its own colour
//          bled through a dark blur behind it so the art never floats as a
//          hard rectangle
//   view   the expanded lightbox: the artwork alone, on nothing, never
//          enlarged past its own pixels -- a preview is a fixed raster, so
//          stretching it to fill a big window only blurs it
Item {
    id: media

    property url source: ""
    property string mode: "cover"       // cover | hero | plate | view
    property bool active: true
    property color surface: Tokens.paper

    readonly property string cleanSource: String(source).split(/[?#]/)[0].toLowerCase()
    readonly property bool animated: cleanSource.endsWith(".gif")
    readonly property bool plate: mode === "plate"
    readonly property bool view: mode === "view"
    readonly property bool framed: plate || view
    readonly property bool bled: mode === "hero" || mode === "plate"
    readonly property int fit: framed ? Image.PreserveAspectFit : Image.PreserveAspectCrop
    readonly property var front: media.animated ? moving : still
    readonly property real pw: front.paintedWidth
    readonly property real ph: front.paintedHeight

    // the box framed art fits into: a plate keeps its inset margin, and the
    // artwork is capped to its own pixels rather than stretched across a big
    // window, which is what made an enlarged preview read as blurred.
    readonly property real frontMargin: plate ? Math.round(Math.min(width, height) * 0.045) : 0
    readonly property real innerW: Math.max(1, width - frontMargin * 2)
    readonly property real innerH: Math.max(1, height - frontMargin * 2)

    clip: true

    // ── colour bleed: the artwork itself, blown up and blurred, so the plate
    // sits in a wash of its own palette instead of on flat black ────────────
    Image {
        id: bleed
        anchors.fill: parent
        source: media.bled && media.source !== "" ? media.source : ""
        sourceSize: Qt.size(Math.max(1, Math.ceil(width)), Math.max(1, Math.ceil(height)))
        fillMode: Image.PreserveAspectCrop
        asynchronous: true
        cache: true
        retainWhileLoading: true
        smooth: true
        visible: false
    }

    MultiEffect {
        anchors.fill: parent
        source: bleed
        scale: 1.18
        visible: media.bled && bleed.status === Image.Ready
        blurEnabled: visible
        blur: 1
        blurMax: 64
        saturation: media.plate ? 0.35 : 0.18
        brightness: -0.32
    }

    // vignette: darken the edges so the bleed reads as depth, not a photo
    Rectangle {
        anchors.fill: parent
        visible: media.bled
        gradient: Gradient {
            GradientStop { position: 0; color: "#7a000000" }
            GradientStop { position: 0.42; color: "#1a000000" }
            GradientStop { position: 0.72; color: "#33000000" }
            GradientStop { position: 1; color: "#a6000000" }
        }
    }

    // ── foreground plate shadow (plate only): a soft drop shadow shaped by the
    // art's own alpha, so the framed preview lifts off the bleed ─────────────
    MultiEffect {
        anchors.fill: parent
        source: media.front
        visible: media.plate && media.front.status === Image.Ready
        autoPaddingEnabled: false
        blurEnabled: false
        shadowEnabled: visible
        shadowColor: "#d0000000"
        shadowBlur: 1
        shadowVerticalOffset: 14
        shadowScale: 1.005
        opacity: 0.9
    }

    Image {
        id: still
        anchors.centerIn: parent
        // A framed plate decodes at its own pixels and is then capped to the
        // frame, so a small preview is never stretched across a big window --
        // forcing sourceSize from the frame size would be circular (an Image's
        // implicit size follows its sourceSize), so framed art simply leaves it
        // native. A crop tile keeps its oversampled hidpi decode.
        sourceSize: media.framed ? Qt.size(0, 0)
                : Qt.size(Math.max(1, Math.ceil(media.width * 2)),
                          Math.max(1, Math.ceil(media.height * 2)))
        width: media.framed ? Math.min(media.innerW, Math.max(1, implicitWidth)) : media.width
        height: media.framed ? Math.min(media.innerH, Math.max(1, implicitHeight)) : media.height
        source: !media.animated ? media.source : ""
        fillMode: media.fit
        asynchronous: true
        cache: true
        retainWhileLoading: true
        smooth: true
        mipmap: media.plate
        visible: source !== "" && status === Image.Ready
        opacity: visible ? 1 : 0

        Behavior on opacity { NumberAnimation { duration: media.view ? 0 : 180 } }
    }

    AnimatedImage {
        id: moving
        anchors.centerIn: parent
        width: media.framed ? Math.min(media.innerW, Math.max(1, implicitWidth)) : media.width
        height: media.framed ? Math.min(media.innerH, Math.max(1, implicitHeight)) : media.height
        source: media.animated ? media.source : ""
        // No forced sourceSize: an AnimatedImage scales its movie to sourceSize
        // exactly (QMovie), which stretches gifs whose aspect differs from the
        // frame; native size plus fillMode keeps the aspect right.
        fillMode: media.fit
        asynchronous: true
        cache: true
        retainWhileLoading: true
        smooth: true
        playing: media.active && visible
        visible: source !== "" && status === AnimatedImage.Ready
        opacity: visible ? 1 : 0

        Behavior on opacity { NumberAnimation { duration: media.view ? 0 : 180 } }
    }

    // hairline frame drawn exactly around the fitted plate, so the preview
    // reads as a mounted print rather than a raw image
    Rectangle {
        visible: media.plate && media.pw > 0 && media.ph > 0
        width: media.pw
        height: media.ph
        anchors.centerIn: parent
        color: "transparent"
        radius: Tokens.radius
        border.width: Tokens.border
        border.color: "#33ffffff"
    }

    // reading scrim: keep overlaid text legible, and seat the frame in the
    // surface colour at the bottom edge
    Rectangle {
        anchors.fill: parent
        visible: media.bled
        gradient: Gradient {
            GradientStop { position: 0; color: "#12000000" }
            GradientStop { position: 0.6; color: "#00000000" }
            GradientStop {
                position: 1
                color: Qt.rgba(media.surface.r, media.surface.g, media.surface.b, media.plate ? 0.35 : 0.7)
            }
        }
    }
}

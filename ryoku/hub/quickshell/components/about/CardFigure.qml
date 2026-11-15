import QtQuick
import QtQuick.Effects
import Quickshell
import Lithic
import Ryoku.Ui.Singletons

// the profile's figure (profile.json hero). The stock one is a photograph, the Caracalla
// Farnese, drawn as it is; a gallery figure is inked in one colour so it reads on either
// ground. The reader's own picture is framed (zoom, position, fill or fit), adjusted
// (brightness, contrast, colour) and shown as a photo, in black and white, or as a 1-bit
// dither in the ink. The system card, Home and the card studio draw the same one
Item {
    id: hero

    property color ink: Lithic.fg
    // a figure drawn at another size than its dots asks for soft sampling, or the dots beat
    property bool soft: false
    readonly property string configDir: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku"
    readonly property var config: ProfileStore.get("hero", ({}))
    readonly property string kind: hero.config.kind || "default"
    readonly property bool stock: hero.kind === "default"
    // the stock photograph is cut to its marble: whoever sizes the figure by it reads this
    readonly property real stockAspect: 665 / 874
    readonly property string src: hero.config.source || ""
    readonly property real zoom: hero.config.zoom !== undefined ? hero.config.zoom : 1
    readonly property real focalX: hero.config.focalX !== undefined ? hero.config.focalX : 0.5
    readonly property real focalY: hero.config.focalY !== undefined ? hero.config.focalY : 0.4
    // "photo", "mono" or "dither"; a picture chosen before there was a choice was dithered
    readonly property string style: hero.config.style || "dither"
    readonly property bool fit: hero.config.fit === true
    readonly property real brightness: hero.config.brightness || 0
    readonly property real contrast: hero.config.contrast || 0
    readonly property real saturation: hero.config.saturation || 0

    clip: true

    // "Caracalla MAN Napoli Inv6033 n01" by Marie-Lan Nguyen, CC BY 2.5: see the root NOTICE
    Image {
        anchors.fill: parent
        visible: hero.stock
        source: hero.stock ? Qt.resolvedUrl("../../art/caracalla.png") : ""
        fillMode: Image.PreserveAspectFit
        verticalAlignment: Image.AlignBottom
        smooth: true
        mipmap: true
        asynchronous: true
    }

    // your own picture, framed: zoomed about the focal point, filling the figure or fitting in it
    Item {
        id: framed
        anchors.fill: parent
        visible: false
        clip: true
        Image {
            width: hero.width * hero.zoom
            height: hero.height * hero.zoom
            x: (hero.width - width) * hero.focalX
            y: (hero.height - height) * hero.focalY
            source: hero.kind === "custom" && hero.src.length > 0 ? "file://" + hero.configDir + "/profile/" + hero.src : ""
            fillMode: hero.fit ? Image.PreserveAspectFit : Image.PreserveAspectCrop
            sourceSize.width: 1600
            smooth: true
            mipmap: true
            asynchronous: true
        }
    }
    // and adjusted; shown as it is for a photo, or handed to the dither
    MultiEffect {
        id: adjusted
        anchors.fill: parent
        visible: hero.kind === "custom" && hero.style !== "dither"
        source: framed
        brightness: hero.brightness
        contrast: hero.contrast
        saturation: hero.style === "mono" ? -1 : hero.saturation
    }
    ShaderEffectSource {
        id: adjustedTexture
        sourceItem: hero.kind === "custom" && hero.style === "dither" ? adjusted : null
        hideSource: true
        live: true
    }
    ShaderEffect {
        anchors.fill: parent
        visible: hero.kind === "custom" && hero.style === "dither"
        property variant src: adjustedTexture
        property color ink: hero.ink
        property color accent: hero.ink
        property vector2d srcSize: Qt.vector2d(Math.max(1, width), Math.max(1, height))
        property real dotScale: hero.config.dither !== undefined ? hero.config.dither : 1
        property real develop: 1
        property real scan: 0
        property real band: 0
        property real invert: hero.config.invert === true ? 1 : 0
        fragmentShader: Qt.resolvedUrl("../../shaders/plate.frag.qsb")
    }

    // the gallery figures are already one-bit and only take the ink
    Image {
        id: inked
        anchors.fill: parent
        visible: false
        source: hero.kind === "gallery" && hero.src.length > 0 ? Ryodecors.dir + hero.src : ""
        fillMode: Image.PreserveAspectFit
        smooth: hero.soft
        mipmap: hero.soft
        asynchronous: true
    }
    MultiEffect {
        anchors.fill: parent
        visible: hero.kind === "gallery"
        source: inked
        colorization: 1
        colorizationColor: hero.ink
    }
}

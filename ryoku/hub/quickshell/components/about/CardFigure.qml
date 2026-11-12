import QtQuick
import QtQuick.Effects
import Quickshell
import Lithic
import Ryoku.Ui as RyokuUi
import Ryoku.Ui.Singletons

// the profile's figure (profile.json hero). The stock one is a photograph, the Caracalla
// Farnese, drawn as it is; a gallery figure or the reader's own picture is inked in one
// colour so it reads on either ground. The system card and Home draw the same one
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

    // your own photo is dithered live; the gallery figures are already one-bit and only take the ink
    RyokuUi.DitherImage {
        visible: hero.kind === "custom"
        width: hero.width * hero.zoom
        height: hero.height * hero.zoom
        x: (hero.width - width) * hero.focalX
        y: (hero.height - height) * hero.focalY
        bone: hero.ink
        source: hero.kind === "custom" && hero.src.length > 0 ? "file://" + hero.configDir + "/profile/" + hero.src : ""
        dotScale: hero.config.dither !== undefined ? hero.config.dither : 1
        invert: hero.config.invert === true
        fillMode: Image.PreserveAspectCrop
    }
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

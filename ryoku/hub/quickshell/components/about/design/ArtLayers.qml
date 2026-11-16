import QtQuick
import Lithic

Item {
    id: root
    property var layers: []
    property bool live: false
    property vector2d tilt: Qt.vector2d(0, 0)
    property real time: 0
    property real glitch: 0
    property string epithet: ""
    property color ink: Lithic.fg
    readonly property bool ready: image0.ready && image1.ready && image2.ready && image3.ready
    function file(index) { return root.layers[index] ? "file://" + root.layers[index].image : ""; }
    function depth(index) { return root.layers[index] ? root.layers[index].depth : 0; }
    function blend(index) { var b = root.layers[index] ? root.layers[index].blend : "normal"; return b === "screen" ? 1 : b === "add" ? 2 : 0; }
    function aspect(image) { return image.sourceSize.height > 0 ? image.sourceSize.width / image.sourceSize.height : 1; }
    DesignImage { id: image0; width: root.width; height: root.height; opacity: 0; source: root.file(0); live: root.live && root.visible }
    DesignImage { id: image1; width: root.width; height: root.height; opacity: 0; source: root.file(1); live: root.live && root.visible }
    DesignImage { id: image2; width: root.width; height: root.height; opacity: 0; source: root.file(2); live: root.live && root.visible }
    DesignImage { id: image3; width: root.width; height: root.height; opacity: 0; source: root.file(3); live: root.live && root.visible }
    Item {
        id: inscription
        anchors.fill: parent
        Text {
            x: 14; y: 16; width: parent.width - 28
            text: root.epithet
            textFormat: Text.PlainText
            renderType: Text.QtRendering
            color: Lithic.alpha(root.ink, 0.12)
            font.family: Lithic.font.sans
            font.weight: Font.Black
            font.capitalization: Font.AllUppercase
            font.pixelSize: 110
            fontSizeMode: Text.HorizontalFit
            minimumPixelSize: 30
            horizontalAlignment: Text.AlignHCenter
        }
    }
    ShaderEffectSource {
        id: inscriptionTexture
        sourceItem: inscription
        hideSource: true
        live: true
    }
    ShaderEffect {
        anchors.fill: parent
        visible: root.layers.length > 0
        property var source0: image0
        property var source1: root.layers.length > 1 ? image1 : image0
        property var source2: root.layers.length > 2 ? image2 : image0
        property var source3: root.layers.length > 3 ? image3 : image0
        property var lettering: inscriptionTexture
        property vector4d aspects: Qt.vector4d(root.aspect(image0), root.aspect(image1), root.aspect(image2), root.aspect(image3))
        property vector4d depths: Qt.vector4d(root.depth(0), root.depth(1), root.depth(2), root.depth(3))
        property vector4d blends: Qt.vector4d(root.blend(0), root.blend(1), root.blend(2), root.blend(3))
        property vector2d shift: root.tilt
        property real aspect: width / Math.max(1, height)
        property real count: root.layers.length
        property real time: root.time
        property real glitch: root.live ? root.glitch : 0
        fragmentShader: Qt.resolvedUrl("../../../shaders/design-art.frag.qsb")
    }
}

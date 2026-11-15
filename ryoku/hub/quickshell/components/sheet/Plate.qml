import QtQuick
import Lithic
import "../../Singletons"

// a dithered plate: a classical sculpture in the ink's dots, registered by corner ticks, its
// credit under it. It develops in from the top when it arrives, and the
// ambient scan crosses it now and then (shaders/plate.frag)
Item {
    id: root

    implicitHeight: art.height + (credit.visible ? credit.anchors.topMargin + credit.implicitHeight : 0)

    // { file, credit } from registry/plates.js, or null
    property var plate: null
    property real dotScale: 1.5
    readonly property url source: root.plate ? Qt.resolvedUrl("../../art/plates/" + root.plate.file) : ""

    property real develop: 1
    onSourceChanged: {
        if (Lithic.reduceMotion) {
            root.develop = 1;
            return;
        }
        arrive.restart();
    }
    NumberAnimation {
        id: arrive
        target: root
        property: "develop"
        from: 0
        to: 1
        duration: 1100
        easing.type: Easing.OutCubic
    }

    Item {
        id: art
        width: parent.width
        height: Math.round(parent.width / 1.6)
    }

    // a hidden Image is still a texture provider for the shader
    Image {
        id: photo
        anchors.fill: art
        source: root.source
        fillMode: Image.PreserveAspectCrop
        asynchronous: true
        visible: false
        sourceSize.width: 640
    }

    ShaderEffect {
        anchors.fill: art
        visible: photo.status === Image.Ready
        property variant src: photo
        property color ink: Lithic.alpha(Lithic.fg2, 0.92)
        property color accent: Lithic.sel
        property vector2d srcSize: Qt.vector2d(Math.max(1, width), Math.max(1, height))
        property real dotScale: root.dotScale
        property real develop: root.develop
        property real scan: Math.max(0, Ambient.scan)
        property real band: Ambient.scan < 0 ? 0 : 1
        fragmentShader: Qt.resolvedUrl("../../shaders/plate.frag.qsb")
    }

    // the corner ticks that register the plate on the sheet
    Repeater {
        model: 4
        delegate: Item {
            required property int index
            readonly property bool atRight: index % 2 === 1
            readonly property bool atBottom: index > 1
            x: atRight ? art.width - 9 : 0
            y: atBottom ? art.height - 9 : 0
            width: 10
            height: 10
            Rectangle {
                y: parent.atBottom ? 9 : 0
                width: 10
                height: 1
                color: Lithic.edge
            }
            Rectangle {
                x: parent.atRight ? 9 : 0
                width: 1
                height: 10
                color: Lithic.edge
            }
        }
    }

    // the credit its licence asks for, under the plate in the smallest mono
    Label {
        id: credit
        visible: !!root.plate
        anchors.top: art.bottom
        anchors.topMargin: 6
        anchors.left: art.left
        anchors.right: art.right
        text: root.plate ? root.plate.credit : ""
        typeRole: "monoSm"
        inkRole: "fg3"
        font.pixelSize: 9
        font.letterSpacing: 0.4
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}

import QtQuick
import QtQuick.Effects
import Lithic
import "../../../../ask/look"

// A thumbnail wearing the design's corner (a cut in Lithic), or the kind's glyph
// on a quiet plane while there is no picture.
Item {
    id: root

    property real s: 1
    // a local file path
    property string source: ""
    property string glyph: "draft"
    property real cutSize: Look.cutRow
    property real glyphSize: 22
    readonly property bool shown: root.source.length > 0 && picture.status === Image.Ready

    Image {
        id: picture
        anchors.fill: parent
        visible: false
        source: root.source.length > 0 ? "file://" + root.source : ""
        sourceSize: Qt.size(Math.ceil(root.width * 2), Math.ceil(root.height * 2))
        fillMode: Image.PreserveAspectCrop
        asynchronous: true
    }
    MultiEffect {
        anchors.fill: parent
        visible: root.shown
        source: picture
        maskEnabled: true
        maskSource: mask
        maskThresholdMin: 0.5
        maskSpreadAtMin: 1
    }
    // the outline stays in the scene (hidden by its source) so the mask follows a
    // change of design; an invisible layer would keep the corner it was born with
    Shape {
        id: outline
        anchors.fill: parent
        s: root.s
        cutSize: root.cutSize
        fill: Lithic.fg
    }
    ShaderEffectSource {
        id: mask
        anchors.fill: parent
        visible: false
        sourceItem: outline
        hideSource: true
    }

    Shape {
        anchors.fill: parent
        visible: !root.shown
        s: root.s
        cutSize: root.cutSize
        fill: Lithic.alpha(Lithic.fg, Look.dark ? 0.08 : 0.07)

        Symbol {
            anchors.centerIn: parent
            s: root.s
            name: root.glyph
            size: root.glyphSize
            ink: Lithic.fg2
        }
    }
}

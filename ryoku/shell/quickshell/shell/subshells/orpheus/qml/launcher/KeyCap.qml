import QtQuick
import "../kit" as K

// A key name set as a keycap: a hairline cut cap with mono text in Lithic,
// tonal in Material and Adwaita, outlined in Fluent. `inverse` draws it on a
// filled bead (Lithic's bone plate), in that plate's ink.
Item {
    id: root

    property string text: ""
    property bool inverse: false

    readonly property color ink: root.inverse && K.Look.lithic ? K.Style.inkOnBone : K.Style.inkDim

    implicitWidth: Math.max(implicitHeight, label.implicitWidth + K.Style.s2 + 2)
    implicitHeight: 18

    K.Shape {
        anchors.fill: parent
        cutSize: K.Look.cutToken
        fill: K.Look.material || K.Look.adwaita
            ? (root.inverse ? K.Style.tint16 : K.Style.tint10) : "transparent"
        strokeWidth: K.Look.lithic || K.Look.fluent ? K.Style.border : 0
        strokeColor: K.Look.lithic
            ? (root.inverse ? K.Style.alpha(K.Style.inkOnBone, 0.45) : K.Style.line)
            : K.Style.lineSoft
    }

    Text {
        id: label
        anchors.centerIn: parent
        text: root.text
        color: root.ink
        font.family: K.Look.lithic ? K.Style.mono : K.Style.font
        font.pixelSize: K.Style.tiny + 1
        font.weight: Font.Medium
        font.letterSpacing: K.Look.lithic ? 0.6 : 0
        font.capitalization: K.Look.lithic ? Font.AllUppercase : Font.MixedCase
        renderType: Text.NativeRendering
    }
}

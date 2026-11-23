import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// the foot's line while a removal waits for its undo
Row {
    id: root

    required property real s
    property string text: ""
    // the Ctrl+Z cap shows only while the chord reaches the undo, not the field's own undo
    property bool keyed: true

    signal undo()

    spacing: Math.round(8 * root.s)

    Text {
        anchors.verticalCenter: parent.verticalCenter
        text: root.text
        color: Lithic.fg
        font.family: Look.sans
        font.pixelSize: Math.round(12 * root.s)
        font.weight: Font.Medium
        renderType: Text.NativeRendering
    }
    Chip {
        anchors.verticalCenter: parent.verticalCenter
        s: root.s
        tier: "line"
        glyph: "undo"
        text: I18n.tr("Undo")
        onClicked: root.undo()
    }
    KeyHint {
        anchors.verticalCenter: parent.verticalCenter
        visible: root.keyed
        s: root.s
        hint: "Ctrl+Z"
    }
}

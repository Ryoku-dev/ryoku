pragma ComponentBehavior: Bound
import QtQuick
import Lithic
import "../../../ask/look"

Item {
    id: root
    required property real s
    required property string label
    property string glyph: ""
    property string detail: ""
    property bool selected: false
    property bool compact: false
    signal chosen()
    signal triggered()
    implicitHeight: (compact ? 48 : 94) * s
    Shape {
        anchors.fill: parent
        s: root.s
        cutSize: root.compact ? Look.cutRow : Look.cutCard
        fill: root.selected ? Look.plate : pointer.pressed ? Look.press : pointer.containsMouse ? Look.hover : Look.card
        strokeColor: root.selected ? Look.plateStroke : Look.cardStroke
        strokeWidth: 1
        Behavior on fill { ColorAnimation { duration: Look.dur(Look.plateMs) } }
    }
    Symbol { x: 14 * root.s; y: root.compact ? (parent.height - height) / 2 : 15 * root.s; s: root.s; name: root.glyph; size: 21; color: root.selected ? Look.plateMark : Lithic.fg2 }
    Text {
        x: (root.compact ? 46 : 14) * root.s
        y: root.compact ? 0 : 58 * root.s
        width: parent.width - x - 14 * root.s
        height: root.compact ? parent.height : 24 * root.s
        text: root.label
        font.family: Look.sans
        font.pixelSize: 13 * root.s
        color: Lithic.fg
        verticalAlignment: Text.AlignVCenter
        elide: Text.ElideRight
    }
    MouseArea {
        id: pointer
        anchors.fill: parent
        hoverEnabled: true
        onClicked: { root.chosen(); root.triggered(); }
    }
    Accessible.role: Accessible.Button
    Accessible.name: label
    Accessible.onPressAction: { root.chosen(); root.triggered(); }
}

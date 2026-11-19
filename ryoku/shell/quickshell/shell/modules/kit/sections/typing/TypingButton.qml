pragma ComponentBehavior: Bound
import QtQuick
import Lithic
import "../../../ask/look"
FocusScope {
    id: root
    required property real s
    property string text: ""
    property bool chosen: false
    signal clicked()
    implicitWidth: label.implicitWidth + 22 * s
    implicitHeight: 28 * s
    activeFocusOnTab: true
    Shape { anchors.fill: parent; cut: Look.cutToken * root.s; fill: root.chosen ? Lithic.sel : hover.hovered || root.activeFocus ? Look.hover : "transparent"; strokeColor: root.activeFocus ? Lithic.fg : Look.cardStroke; strokeWidth: 1; Behavior on fill { ColorAnimation { duration: Look.dur(Look.stateMs) } } }
    Text { id: label; anchors.centerIn: parent; text: root.text; color: root.chosen ? Lithic.onSel : Lithic.fg; font.family: Look.sans; font.pixelSize: 12 * root.s }
    HoverHandler { id: hover; cursorShape: Qt.PointingHandCursor }
    TapHandler { onTapped: root.clicked() }
    Keys.onReturnPressed: root.clicked()
    Keys.onSpacePressed: root.clicked()
}

pragma ComponentBehavior: Bound
import QtQuick
import Lithic
import "../../../ask/look"

Item {
    id: root
    required property real s
    property string label: ""
    property alias text: input.text
    signal accepted()
    signal navigate(int delta)
    implicitHeight: 58 * s
    Text { text: root.label; color: Lithic.fg2; font.family: Look.sans; font.pixelSize: Math.round(12 * root.s) }
    Shape {
        y: 20 * root.s; width: parent.width; height: 34 * root.s; s: root.s
        fill: Look.well; strokeColor: input.activeFocus ? Lithic.sel : Lithic.vein; strokeWidth: 1
        TextInput {
            id: input
            anchors.fill: parent; anchors.margins: 8 * root.s
            color: Lithic.fg; selectionColor: Lithic.sel; selectedTextColor: Lithic.onSel
            font.family: Look.mono; font.pixelSize: Math.round(11 * root.s)
            clip: true; selectByMouse: true
            onAccepted: root.accepted()
            Keys.onTabPressed: event => { root.navigate(1); event.accepted = true; }
            Keys.onBacktabPressed: event => { root.navigate(-1); event.accepted = true; }
        }
    }
    function takeFocus() { input.forceActiveFocus(); }
}

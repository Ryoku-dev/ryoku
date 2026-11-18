import QtQuick
import QtQuick.Templates as T
import Lithic

T.AbstractButton {
    id: control
    property string glyph: ""
    property string accessibleName: ""
    property real s: 1
    property bool turnOnHover: false
    property bool selected: false
    property bool copied: false
    implicitWidth: 30 * s
    implicitHeight: 30 * s
    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    Accessible.role: Accessible.Button
    Accessible.name: accessibleName
    Keys.onReturnPressed: if (enabled) clicked()
    Keys.onEnterPressed: if (enabled) clicked()
    opacity: enabled ? 1 : Lithic.disabledOpacity
    scale: down ? 0.93 : 1
    Behavior on scale { NumberAnimation { duration: Lithic.duration(90); easing.type: Easing.OutCubic } }
    background: Item {
        CutRect {
            anchors.fill: parent; cut: 6
            fill: Lithic.alpha(Lithic.fg, control.down ? 0.12 : 0.08)
            opacity: control.hovered || control.down || control.selected ? 1 : 0
            Behavior on opacity { NumberAnimation { duration: Lithic.duration(control.hovered ? 160 : 100) } }
        }
        FocusRing { anchors.fill: parent; cut: 6; keyboardFocusVisible: control.visualFocus }
    }
    contentItem: Glyph {
        name: control.copied ? "check" : control.glyph
        size: 18
        ink: control.hovered ? Lithic.fg : Lithic.fg2
        rotation: control.turnOnHover && control.hovered ? 90 : 0
        Behavior on rotation { NumberAnimation { duration: Lithic.duration(240); easing.type: Easing.OutCubic } }
        scale: control.copied ? 1.05 : 1
        Behavior on scale { NumberAnimation { duration: Lithic.duration(120); easing.type: Easing.OutCubic } }
    }
}

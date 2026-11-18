import QtQuick
import QtQuick.Templates as T
import Lithic

T.AbstractButton {
    id: action
    property string glyph: ""
    property real s: 1
    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    Accessible.role: Accessible.Button
    Accessible.name: text
    Keys.onReturnPressed: if (enabled) clicked()
    Keys.onEnterPressed: if (enabled) clicked()
    implicitWidth: content.implicitWidth + 26 * s
    implicitHeight: 32 * s
    leftPadding: 13 * s
    rightPadding: 13 * s
    opacity: enabled ? 1 : Lithic.disabledOpacity
    scale: down ? 0.98 : 1
    Behavior on scale { NumberAnimation { duration: Lithic.duration(90) } }
    background: Item {
        CutRect { anchors.fill: parent; cut: 6; fill: "transparent"; strokeColor: Lithic.vein; strokeWidth: 1 }
        CutRect {
            anchors.fill: parent; cut: 6; fill: Lithic.alpha(Lithic.fg, 0.08)
            opacity: action.hovered || action.down ? 1 : 0
            Behavior on opacity { NumberAnimation { duration: Lithic.duration(action.hovered ? 160 : 100) } }
        }
        FocusRing { anchors.fill: parent; cut: 6; keyboardFocusVisible: action.visualFocus }
    }
    contentItem: Row {
        id: content
        spacing: 6 * action.s
        Glyph { visible: action.glyph.length > 0; anchors.verticalCenter: parent.verticalCenter; name: action.glyph; size: 17; ink: Lithic.fg2 }
        Label { anchors.verticalCenter: parent.verticalCenter; text: action.text; typeRole: "label"; inkRole: "fg2" }
    }
}

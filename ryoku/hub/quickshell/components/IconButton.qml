import QtQuick
import QtQuick.Templates as T
import Lithic
import "../Singletons"

// a round glyph button: a soft disc under the pointer, a small press
T.AbstractButton {
    id: root

    property string glyph: ""
    property string accessibleName: ""

    implicitWidth: 34
    implicitHeight: 34
    hoverEnabled: true
    focusPolicy: Qt.TabFocus
    Accessible.role: Accessible.Button
    Accessible.name: root.accessibleName

    background: Rectangle {
        radius: width / 2
        color: root.down ? Tone.press : Tone.hover
        opacity: root.hovered || root.down || root.visualFocus ? 1 : 0
        border.width: root.visualFocus ? 1 : 0
        border.color: Lithic.fg2
        Behavior on opacity { NumberAnimation { duration: Tone.quick } }
    }

    contentItem: Glyph {
        name: root.glyph
        size: 18
        ink: root.hovered ? Lithic.fg : Lithic.fg2
        scale: root.down ? 0.85 : 1
        rotation: root.hovered && root.glyph === "close" && !Lithic.reduceMotion ? 90 : 0
        Behavior on scale { NumberAnimation { duration: Tone.quick; easing.type: Easing.OutCubic } }
        Behavior on rotation { NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic } }
    }
}

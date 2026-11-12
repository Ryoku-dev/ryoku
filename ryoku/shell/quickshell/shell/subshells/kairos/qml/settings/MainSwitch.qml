import QtQuick
import "../kit"

// android's main switch bar: the one control a page hangs on, as a large pill that fills in when on
Item {
    id: bar

    property string text: ""
    property bool checked: false
    signal toggled(bool value)

    implicitWidth: 320
    implicitHeight: 76

    Accessible.role: Accessible.CheckBox
    Accessible.name: bar.text
    Accessible.checked: bar.checked

    Rectangle {
        id: shape
        anchors.fill: parent
        radius: Md.rXl + 4
        color: bar.checked ? Md.accentContainer : Md.containerHigh
        Behavior on color {
            enabled: !Md.still
            ColorAnimation { duration: Md.fast }
        }
    }

    StateLayer {
        radius: shape.radius
        tint: bar.checked ? Md.onAccentContainer : Md.ink
        onActivated: bar.toggled(!bar.checked)
    }

    Label {
        anchors { left: parent.left; leftMargin: 24; right: sw.left; rightMargin: 16; verticalCenter: parent.verticalCenter }
        role: "title"
        font.pixelSize: 18
        font.weight: Font.Medium
        color: bar.checked ? Md.onAccentContainer : Md.ink
        text: bar.text
    }

    Switch {
        id: sw
        anchors { right: parent.right; rightMargin: 20; verticalCenter: parent.verticalCenter }
        checked: bar.checked
        onToggled: function (v) { bar.toggled(v); }
    }
}

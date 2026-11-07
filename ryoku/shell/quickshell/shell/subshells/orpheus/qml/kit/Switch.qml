import QtQuick
import Ryoku.Ui as Ui

Item {
    id: root

    property bool checked: false
    signal toggled(bool checked)

    implicitWidth: control.implicitWidth
    implicitHeight: control.implicitHeight

    Ui.Sw {
        id: control
        anchors.fill: parent
        on: root.checked
        onToggled: value => root.toggled(value)
    }
}

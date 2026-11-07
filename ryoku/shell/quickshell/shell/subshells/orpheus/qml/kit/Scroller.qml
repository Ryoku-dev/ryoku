import QtQuick
import QtQuick.Controls
import Ryoku.Ui as Ui

Flickable {
    id: root

    default property alias content: contentColumn.data

    implicitWidth: contentColumn.implicitWidth
    implicitHeight: contentColumn.implicitHeight
    contentWidth: width
    contentHeight: contentColumn.implicitHeight
    clip: true
    interactive: contentHeight > height + 0.5
    boundsBehavior: Flickable.StopAtBounds
    flickDeceleration: 3000

    Column {
        id: contentColumn
        width: root.width
    }

    Ui.WheelScroll {}

    ScrollBar.vertical: Ui.ScrollRail {
        policy: root.contentHeight > root.height + 0.5
            ? ScrollBar.AsNeeded : ScrollBar.AlwaysOff
    }
}

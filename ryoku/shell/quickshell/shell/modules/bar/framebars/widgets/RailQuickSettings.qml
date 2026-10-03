pragma ComponentBehavior: Bound

import QtQuick
import "../../../../components"
import shell.services

// Sidebar launch button: opens the left sidebar on click. The Ryoku brand mark
// distinguishes the shell at a glance; geometry stays the reference 16px icon
// box.
Item {
    id: root

    required property string edge
    required property real scale

    implicitWidth: btn.implicitWidth
    implicitHeight: btn.implicitHeight

    RailButton {
        id: btn
        anchors.centerIn: parent
        edge: root.edge
        scale: root.scale
        onClicked: ShellState.requestSurfaceActive("sidebar-left")

        // Sized off the button's glyph box, so the brand mark tracks every other
        // rail icon instead of shrinking with a thin bar on its own.
        Item {
            width: btn.glyphPx
            height: btn.glyphPx
            BrandMark {
                anchors.centerIn: parent
                size: btn.glyphPx * (15 / Theme.iconSm)
                color: Theme.onSurface
            }
        }
    }
}

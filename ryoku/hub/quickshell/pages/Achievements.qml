import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/about"

Item {
    id: root
    required property var hub

    Component.onCompleted: Progression.award("ryoku-first-steps")

    Flickable {
        id: scroll
        anchors.fill: parent
        contentHeight: content.implicitHeight + LithicTokens.space.s5 * 2
        clip: true
        boundsBehavior: Flickable.StopAtBounds

        Column {
            id: content
            x: LithicTokens.space.s5
            y: LithicTokens.space.s5
            width: Math.max(0, scroll.width - 2 * LithicTokens.space.s5 - 16)
            spacing: LithicTokens.space.s5

            ProgressionTab {
                width: parent.width
            }
        }
    }
    ScrollRail {
        flickable: scroll
        anchors.top: scroll.top
        anchors.bottom: scroll.bottom
        anchors.right: parent.right
    }
}

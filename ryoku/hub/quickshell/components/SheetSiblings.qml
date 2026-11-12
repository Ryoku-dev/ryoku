import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// the open category's pages as a strip above the page: the one showing reads as
// chosen, the rest are one click away. Every page layout draws it, so a page
// with its own body (Keyboard shortcuts, Displays) is never left without it.
Item {
    id: root

    required property var hub

    readonly property bool shown: hub.wide && hub.pageId !== "" && hub.categoryPages.length >= 2
    height: shown ? strip.height + LithicTokens.space.s4 : 0
    visible: shown

    Flickable {
        id: strip
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: siblings.height
        contentWidth: siblings.width
        contentHeight: siblings.height
        boundsBehavior: Flickable.StopAtBounds
        clip: true
        interactive: siblings.width > width

        Row {
            id: siblings
            spacing: LithicTokens.space.s2

            Repeater {
                model: root.hub.categoryPages

                Plaque {
                    required property var modelData

                    small: true
                    kind: modelData.id === root.hub.pageId ? "on" : "quiet"
                    text: I18n.tr(modelData.title)
                    onClicked: root.hub.navigate(modelData.id, "")
                }
            }
        }
    }
}

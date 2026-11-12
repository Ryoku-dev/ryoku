import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../registry/registry.js" as Registry

// the work pane for a page that has not moved into the new Settings yet
Item {
    id: root

    required property var hub
    property string pageId: ""
    readonly property var page: Registry.page(root.pageId)

    function focusRow(rowId) {}

    Column {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.leftMargin: LithicTokens.size.settingsInsetSide - LithicTokens.size.workContentInset
        spacing: LithicTokens.space.s2

        Label {
            width: parent.width
            text: root.page ? I18n.tr(root.page.title) : ""
            typeRole: "name"
        }
        Label {
            width: Math.min(parent.width, LithicTokens.size.emptyBodyMax)
            text: I18n.tr("This page moves here next. Until then it is in the current Settings.")
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }
}

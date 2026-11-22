import QtQuick
import ".."

Item {
    id: root

    property real s: 1
    property bool on: false
    property bool available: true
    property string text: ""
    property string detail: ""
    property string glyph: ""
    property bool searching: false
    property bool canSearch: true
    signal toggleRequested(bool value)
    signal searchRequested()

    implicitHeight: Math.max(toggle.implicitHeight + Math.round(8 * root.s), search.implicitHeight)

    Item {
        id: switchRow
        anchors.left: parent.left
        anchors.right: search.left
        anchors.rightMargin: Math.round(QsLook.tileGap * root.s)
        height: parent.height

        RowPlane {
            id: plane
            anchors.fill: parent
            s: root.s
            hovered: toggle.hovered
        }
        SidebarToggle {
            id: toggle
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.leftMargin: plane.inset
            anchors.rightMargin: plane.inset
            anchors.verticalCenter: parent.verticalCenter
            s: root.s
            text: root.text
            detail: root.detail
            glyph: root.glyph
            on: root.on
            enabled: root.available
            onToggleRequested: value => root.toggleRequested(value)
        }
    }

    SearchButton {
        id: search
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        s: root.s
        subtle: QsLook.fluent
        spinning: root.searching && !QsLook.still
        enabled: root.canSearch
        onClicked: root.searchRequested()
    }
}

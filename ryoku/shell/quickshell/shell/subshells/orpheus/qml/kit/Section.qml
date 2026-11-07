import QtQuick

Item {
    id: root

    property string icon: ""
    property string title: ""
    default property alias rows: rowsColumn.data

    implicitWidth: Math.max(sectionHeader.implicitWidth, rowsColumn.implicitWidth)
    implicitHeight: (sectionHeader.visible ? sectionHeader.implicitHeight + Style.s2 : 0)
        + rowsColumn.implicitHeight

    Label {
        id: sectionHeader
        visible: root.title.length > 0
        text: root.title
        role: "micro"
        tone: "muted"
    }

    Column {
        id: rowsColumn
        anchors.top: sectionHeader.visible ? sectionHeader.bottom : parent.top
        anchors.topMargin: sectionHeader.visible ? Style.s2 : 0
        width: root.width
        spacing: Style.s1
    }
}

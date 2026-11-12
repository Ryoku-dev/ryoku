import QtQuick

Item {
    id: root

    property string title: ""
    property int count: 0

    visible: count > 0
    implicitHeight: LithicTokens.size.groupHeaderTop
        + Math.max(titleLabel.implicitHeight, countLabel.implicitHeight)
        + LithicTokens.size.groupHeaderBottom
    implicitWidth: titleLabel.implicitWidth + countLabel.implicitWidth
        + LithicTokens.size.groupHeaderLeft
        + LithicTokens.size.groupHeaderRight
        + LithicTokens.space.s2

    Label {
        id: titleLabel
        anchors.left: parent.left
        anchors.leftMargin: LithicTokens.size.groupHeaderLeft
        anchors.top: parent.top
        anchors.topMargin: LithicTokens.size.groupHeaderTop
        text: root.title
        typeRole: "group"
        inkRole: "fg3"
    }

    Label {
        id: countLabel
        anchors.right: parent.right
        anchors.rightMargin: LithicTokens.size.groupHeaderRight
        anchors.top: titleLabel.top
        text: root.count
        typeRole: "caption"
        numeric: true
        inkRole: "fg3"
    }
}

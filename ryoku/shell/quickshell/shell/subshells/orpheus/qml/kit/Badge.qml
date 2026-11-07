import QtQuick

Item {
    id: root

    property int count: 0
    property bool dot: false

    readonly property bool countVisible: count > 0

    implicitWidth: countVisible ? Math.max(16, badgeLabel.implicitWidth + Style.s2)
        : dot ? 6 : 0
    implicitHeight: countVisible ? 16 : dot ? 6 : 0
    visible: countVisible || dot

    Rectangle {
        anchors.fill: parent
        radius: height / 2
        color: root.countVisible ? Style.bone : Style.alert
    }

    Label {
        id: badgeLabel
        anchors.centerIn: parent
        visible: root.countVisible
        text: root.count > 99 ? "99+" : String(root.count)
        role: "tag"
        tone: "onBone"
        numeric: true
        strong: true
    }
}

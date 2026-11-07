import QtQuick

Item {
    id: root

    property real value: 0
    property color color: Style.ink
    readonly property real clampedValue: Math.max(0, Math.min(1, value))

    implicitWidth: 80
    implicitHeight: 4

    Rectangle {
        anchors.fill: parent
        color: Style.tint10
        antialiasing: false

        Rectangle {
            anchors.left: parent.left
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            width: parent.width * root.clampedValue
            color: root.color
            antialiasing: false

            Behavior on width {
                enabled: !Style.still
                NumberAnimation { duration: Style.move; easing.type: Easing.OutCubic }
            }
        }
    }
}

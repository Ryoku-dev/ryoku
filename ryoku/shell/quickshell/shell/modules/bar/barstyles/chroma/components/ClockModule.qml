pragma ComponentBehavior: Bound

import QtQuick
import shell.services

Rectangle {
    id: root

    required property var colors
    required property real s

    property date now: new Date()

    implicitWidth: (Theme.iconLg * 4 + Theme.paddingLg) * root.s
    implicitHeight: (Theme.iconLg + Theme.paddingLg) * root.s

    radius: Theme.radiusWidget * root.s
    color: colors.accent(0)

    readonly property color contentColor: colors.inkOn(color)

    Behavior on color {
        enabled: !Motion.reduce
        ColorAnimation { duration: Motion.standard; easing.type: Motion.easeStandard }
    }

    Timer {
        interval: 1000
        running: true
        repeat: true
        onTriggered: root.now = new Date()
    }

    Column {
        anchors {
            left: parent.left
            leftMargin: Theme.paddingLg * root.s
            verticalCenter: parent.verticalCenter
        }

        Text {
            text: Qt.formatDateTime(root.now, "HH:mm")
            color: root.contentColor
            font.family: Theme.mono
            font.pixelSize: 21 * root.s
            font.weight: Font.Black
            font.letterSpacing: root.s
        }

        Text {
            text: Qt.formatDateTime(root.now, "ddd  dd.MM.yy").toUpperCase()
            color: root.contentColor
            opacity: 0.68
            font.family: Theme.mono
            font.pixelSize: 8 * root.s
            font.weight: Font.Bold
            font.letterSpacing: 1.1 * root.s
        }
    }

    Rectangle {
        anchors {
            right: parent.right
            rightMargin: Theme.paddingLg * root.s
            verticalCenter: parent.verticalCenter
        }
        width: Theme.paddingMd * root.s
        height: Theme.iconMd * root.s
        radius: width / 2
        color: root.contentColor
        opacity: 0.9
    }
}

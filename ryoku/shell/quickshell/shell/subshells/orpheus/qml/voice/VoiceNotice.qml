pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../kit" as K

Item {
    id: root

    required property var scene
    required property var notification

    readonly property string appIcon: notification
        ? String(notification.appIcon || "") : ""
    readonly property url iconSource: {
        if (!appIcon)
            return "";
        if (appIcon.indexOf("/") === 0 || appIcon.indexOf("file:") === 0
                || appIcon.indexOf("image:") === 0)
            return appIcon;
        return Icons.path(appIcon, true);
    }
    readonly property string summary: notification
        ? String(notification.summary || notification.appName || "Notification") : "Notification"
    readonly property string bodyLine: {
        const body = notification ? String(notification.body || "") : "";
        return body.split(/\r?\n/)[0].replace(/<[^>]*>/g, " ").replace(/\s+/g, " ").trim();
    }
    readonly property real textRoom: Math.max(1, Math.min(
        220 * K.Style.scale, scene.textRoom,
        Math.max(summaryMeasure.implicitWidth, bodyMeasure.implicitWidth)))

    implicitWidth: content.implicitWidth
    implicitHeight: K.Style.barH

    K.Label {
        id: summaryMeasure
        visible: false
        text: root.summary
        role: "bar"
    }

    K.Label {
        id: bodyMeasure
        visible: false
        text: root.bodyLine
        role: "barSub"
    }

    Row {
        id: content
        anchors.centerIn: parent
        spacing: K.Style.s2

        Item {
            anchors.verticalCenter: parent.verticalCenter
            width: 18 * K.Style.scale
            height: width

            Image {
                id: noticeIcon
                anchors.fill: parent
                visible: source.toString().length > 0 && status !== Image.Error
                source: root.iconSource
                sourceSize: Qt.size(Math.round(width * 2), Math.round(height * 2))
                fillMode: Image.PreserveAspectFit
                asynchronous: true
                smooth: true
                mipmap: true
            }

            K.Glyph {
                anchors.centerIn: parent
                visible: !noticeIcon.visible
                name: "notifications"
                size: parent.width
                tone: "dim"
            }
        }

        Column {
            anchors.verticalCenter: parent.verticalCenter
            width: root.textRoom
            spacing: 0

            K.Label {
                width: parent.width
                text: root.summary
                role: "bar"
                maximumLineCount: 1
            }

            K.Label {
                visible: root.bodyLine.length > 0
                width: parent.width
                text: root.bodyLine
                role: "barSub"
                tone: "dim"
                maximumLineCount: 1
            }
        }
    }
}

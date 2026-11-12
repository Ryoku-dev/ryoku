pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons
import "../kit"
import "../components/Notification.js" as Note

Item {
    id: item

    required property var notif
    property string position: "only"
    signal opened()

    function n() { return item.notif || ({}) }

    readonly property var actions: Note.actions(item.notif)
    readonly property var defaultAction: Note.defaultAction(item.notif)
    readonly property string imageSource: Note.source(item.n().image, Icons)
    readonly property string appIconSource: Note.source(item.n().appIcon, Icons)

    readonly property bool topOuter: item.position === "only" || item.position === "first"
    readonly property bool bottomOuter: item.position === "only" || item.position === "last"

    implicitHeight: content.implicitHeight + 28
    clip: true

    Item {
        id: body
        width: parent.width
        height: parent.height
        opacity: 1 - Math.min(1, Math.abs(body.x) / Math.max(1, item.width * 0.6))

        Behavior on x {
            enabled: !Md.still && !swipe.active
            NumberAnimation { duration: Md.standard; easing.type: Easing.OutCubic }
        }

        Rectangle {
            id: shape
            anchors.fill: parent
            topLeftRadius: item.topOuter ? Md.rXl : Md.rXs
            topRightRadius: item.topOuter ? Md.rXl : Md.rXs
            bottomLeftRadius: item.bottomOuter ? Md.rXl : Md.rXs
            bottomRightRadius: item.bottomOuter ? Md.rXl : Md.rXs
            color: Md.container
        }

        StateLayer {
            id: hit
            topLeftRadius: shape.topLeftRadius
            topRightRadius: shape.topRightRadius
            bottomLeftRadius: shape.bottomLeftRadius
            bottomRightRadius: shape.bottomRightRadius
            onActivated: {
                if (item.defaultAction) {
                    item.defaultAction.invoke();
                    item.opened();
                }
            }
        }

        Rectangle {
            id: disc
            anchors { left: parent.left; leftMargin: 14; top: parent.top; topMargin: 14 }
            width: 40
            height: 40
            radius: 20
            color: Md.containerHighest

            Icon {
                anchors.centerIn: parent
                visible: !icon.visible
                name: "notifications"
                size: 20
                filled: true
            }
            Image {
                id: icon
                anchors.fill: parent
                anchors.margins: 4
                property bool imageFailed: false
                source: (icon.imageFailed || item.imageSource === "") ? item.appIconSource : item.imageSource
                onStatusChanged: if (status === Image.Error && String(source) === item.imageSource) icon.imageFailed = true
                sourceSize: Qt.size(64, 64)
                fillMode: Image.PreserveAspectFit
                asynchronous: true
                visible: status === Image.Ready
            }
        }

        Column {
            id: content
            anchors { left: disc.right; leftMargin: 14; right: parent.right; rightMargin: 14; top: parent.top; topMargin: 14 }
            spacing: 2

            Item {
                width: parent.width
                height: 18

                Label {
                    anchors { left: parent.left; right: when.left; rightMargin: 8; verticalCenter: parent.verticalCenter }
                    role: "caption"
                    color: Md.inkMuted
                    text: item.n().appName || ""
                }
                Label {
                    id: when
                    anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                    visible: !hit.containsMouse && !close.hovered
                    role: "caption"
                    color: Md.inkMuted
                    text: Notifs.timeLabel(item.notif)
                }
            }
            Label {
                width: parent.width
                role: "title"
                font.pixelSize: 14
                text: item.n().summary || ""
            }
            Label {
                width: parent.width
                visible: text.length > 0
                role: "body"
                font.pixelSize: 13
                color: Md.inkMuted
                textFormat: Text.PlainText
                wrapMode: Text.Wrap
                maximumLineCount: 2
                text: Note.plainBody(item.notif)
            }
            Row {
                visible: item.actions.length > 0
                topPadding: 6
                spacing: 8
                Repeater {
                    model: item.actions
                    delegate: Button {
                        required property var modelData
                        variant: "tonal"
                        height: 34
                        text: modelData.text
                        onClicked: {
                            modelData.invoke();
                            item.opened();
                        }
                    }
                }
            }
        }

        IconButton {
            id: close
            readonly property bool hovered: closeHover.hovered
            anchors { right: parent.right; rightMargin: 8; top: parent.top; topMargin: 6 }
            visible: hit.containsMouse || closeHover.hovered
            size: 32
            iconSize: 18
            icon: "close"
            tip: I18n.tr("Dismiss")
            onClicked: Notifs.dismiss(item.notif)
            HoverHandler { id: closeHover }
        }
    }

    DragHandler {
        id: swipe
        target: body
        yAxis.enabled: false
        xAxis.minimum: -item.width
        xAxis.maximum: item.width
        onActiveChanged: {
            if (swipe.active)
                return;
            if (Math.abs(body.x) > item.width * 0.35) {
                body.x = body.x > 0 ? item.width : -item.width;
                dismissLater.restart();
            } else {
                body.x = 0;
            }
        }
    }
    Timer {
        id: dismissLater
        interval: Md.still ? 0 : Md.standard
        onTriggered: Notifs.dismiss(item.notif)
    }
}

pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons
import "../kit"
import "Notification.js" as Note

Item {
    id: hu

    property bool active: true
    property bool blocked: false
    property bool yielding: false
    signal openShade()

    readonly property var notif: (Notifs.popups && Notifs.popups.length > 0) ? Notifs.popups[0] : null
    readonly property int queued: Notifs.popups ? Math.max(0, Notifs.popups.length - 1) : 0
    readonly property bool showing: hu.active && hu.notif !== null && !hu.blocked && !Flags.dnd
    readonly property bool hovered: hover.hovered
    readonly property real wantedWidth: 420
    readonly property real wantedHeight: Math.ceil(content.implicitHeight + 28)

    property real progress: (hu.showing && !hu.yielding) ? 1 : 0
    Behavior on progress {
        enabled: !Md.still
        Spring {}
    }

    function n() { return hu.notif || ({}) }
    readonly property var defaultAction: Note.defaultAction(hu.notif)
    readonly property var actions: Note.actions(hu.notif)
    readonly property string imageSource: Note.source(hu.n().image, Icons)
    readonly property string appIconSource: Note.source(hu.n().appIcon, Icons)
    function putAway() {
        if (hu.notif)
            Notifs.removePopup(hu.notif);
    }

    // hover holds the card, leaving gives it a few more seconds
    onHoveredChanged: {
        if (!hu.notif)
            return;
        if (hu.hovered) {
            release.stop();
            Notifs.cancelPopupTimeout(hu.notif.id);
        } else if (hu.showing) {
            release.restart();
        }
    }
    onNotifChanged: {
        release.stop();
        art.imageFailed = false;
    }
    Timer {
        id: release
        interval: 3000
        onTriggered: hu.putAway()
    }

    visible: hu.progress > 0.01
    opacity: Math.max(0, Math.min(1, (hu.progress - 0.45) / 0.55))

    HoverHandler { id: hover }

    StateLayer {
        radius: 28
        ring: false
        onActivated: {
            if (hu.defaultAction) {
                hu.defaultAction.invoke();
                hu.putAway();
            } else {
                hu.putAway();
                hu.openShade();
            }
        }
    }

    DragHandler {
        id: swipe
        target: null
        xAxis.enabled: false
        onActiveChanged: if (!swipe.active && swipe.translation.y < -24) hu.putAway()
    }

    Rectangle {
        id: disc
        anchors { left: parent.left; leftMargin: 16; top: parent.top; topMargin: 16 }
        width: 36
        height: 36
        radius: 18
        color: Md.containerHighest

        Icon {
            anchors.centerIn: parent
            visible: art.status !== Image.Ready
            name: "notifications"
            size: 18
            filled: true
        }
        Image {
            id: art
            anchors.fill: parent
            anchors.margins: 3
            property bool imageFailed: false
            source: (art.imageFailed || hu.imageSource === "") ? hu.appIconSource : hu.imageSource
            onStatusChanged: if (status === Image.Error && String(source) === hu.imageSource) art.imageFailed = true
            sourceSize: Qt.size(64, 64)
            fillMode: Image.PreserveAspectFit
            asynchronous: true
            visible: status === Image.Ready
        }
    }

    Column {
        id: content
        anchors { left: disc.right; leftMargin: 12; right: parent.right; rightMargin: 16; top: parent.top; topMargin: 14 }
        spacing: 2

        Item {
            width: parent.width
            height: 20

            Label {
                anchors { left: parent.left; right: meta.left; rightMargin: 8; verticalCenter: parent.verticalCenter }
                role: "caption"
                color: Md.inkMuted
                text: hu.n().appName || ""
            }
            Row {
                id: meta
                anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                spacing: 6

                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    visible: hu.queued > 0
                    role: "caption"
                    color: Md.accent
                    text: I18n.tr("%1 more").replace("%1", hu.queued)
                }
                IconButton {
                    anchors.verticalCenter: parent.verticalCenter
                    size: 28
                    iconSize: 16
                    icon: "close"
                    tip: I18n.tr("Put away")
                    onClicked: hu.putAway()
                }
            }
        }
        Label {
            width: parent.width
            role: "title"
            font.pixelSize: 15
            text: hu.n().summary || ""
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
            text: Note.plainBody(hu.notif)
        }
        Row {
            visible: hu.actions.length > 0
            topPadding: 8
            spacing: 8

            Repeater {
                model: hu.actions
                delegate: Button {
                    required property var modelData
                    height: 34
                    variant: "tonal"
                    text: modelData.text
                    onClicked: {
                        modelData.invoke();
                        hu.putAway();
                    }
                }
            }
        }
    }
}

pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services
import Ryoku.Ui.Singletons
import "../kit" as K

K.Sheet {
    id: root

    required property var scene
    property bool shown: false
    property var args: ({})
    property var collapsedGroups: ({})

    onShownChanged: {
        if (shown)
            Notifs.markSeen();
    }

    Connections {
        target: Notifs

        function onNewestArrivalMsChanged() {
            if (root.shown)
                Notifs.markSeen();
        }
    }

    readonly property var groups: {
        const result = [];
        const byName = ({});
        const history = Notifs.history || [];
        for (let index = 0; index < history.length; ++index) {
            const notification = history[index];
            const name = String(notification.appName || "System").trim() || "System";
            const key = name.toLocaleLowerCase();
            let group = byName[key];
            if (!group) {
                group = {
                    key: key,
                    name: name,
                    icon: String(notification.appIcon || ""),
                    notifications: []
                };
                byName[key] = group;
                result.push(group);
            }
            group.notifications.push(notification);
        }
        return result;
    }

    pageWidth: 420
    title: "Notifications"
    trailing: [dndAction, clearAction]

    function iconSource(name) {
        const source = String(name || "");
        if (!source)
            return "";
        if (source.indexOf("/") === 0 || source.indexOf("file:") === 0
                || source.indexOf("image:") === 0)
            return source;
        return Icons.path(source, true);
    }

    function actionsFor(notification) {
        const source = notification && notification.actions ? notification.actions : [];
        const result = [];
        for (let index = 0; index < source.length; ++index) {
            const action = source[index];
            if (!action || typeof action.invoke !== "function")
                continue;
            const label = String(action.text || "").trim();
            if (label || action.identifier === "default")
                result.push(action);
        }
        return result;
    }

    function actionLabel(action) {
        const label = String(action && action.text || "").trim();
        return label || "Open";
    }

    function groupCollapsed(key) {
        return collapsedGroups[key] === true;
    }

    function toggleGroup(key) {
        const next = Object.assign({}, collapsedGroups);
        next[key] = next[key] !== true;
        collapsedGroups = next;
    }

    Item {
        id: dndAction

        implicitWidth: dndRow.implicitWidth
        implicitHeight: 32

        Row {
            id: dndRow
            anchors.centerIn: parent
            spacing: K.Style.s2

            K.Label {
                anchors.verticalCenter: parent.verticalCenter
                text: "Do not disturb"
                role: "caption"
                tone: "dim"
            }

            K.Switch {
                anchors.verticalCenter: parent.verticalCenter
                checked: Notifs.dnd
                onToggled: checked => {
                    if (checked !== Notifs.dnd)
                        Toggles.toggleDnd();
                }
            }
        }
    }

    K.Button {
        id: clearAction
        text: "Clear all"
        variant: "outline"
        enabled: Notifs.history.length > 0
        onClicked: Notifs.clearAll()
    }

    K.Scroller {
        id: notificationScroller

        width: parent.width
        height: Math.min(contentHeight, Math.max(120,
            root.scene.cardMaxHeight - 96))

        Column {
            id: groupsColumn

            width: parent.width
            spacing: K.Style.s4
            visible: root.groups.length > 0

            Repeater {
                model: root.groups

                delegate: Column {
                    id: groupBlock

                    required property var modelData
                    property bool collapsed: root.groupCollapsed(modelData.key)

                    width: groupsColumn.width
                    spacing: K.Style.s2

                    Item {
                        id: groupHeader

                        width: parent.width
                        height: 40
                        scale: groupPress.pressScale

                        Item {
                            id: appIconSlot
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            width: 24
                            height: 24

                            Image {
                                id: appIcon
                                anchors.fill: parent
                                source: root.iconSource(groupBlock.modelData.icon)
                                sourceSize: Qt.size(48, 48)
                                fillMode: Image.PreserveAspectFit
                                asynchronous: true
                                smooth: true
                                mipmap: true
                                visible: source.toString().length > 0 && status === Image.Ready
                            }

                            K.Glyph {
                                anchors.centerIn: parent
                                visible: !appIcon.visible
                                name: "notifications"
                                size: 20
                                tone: "variant"
                            }
                        }

                        K.Label {
                            id: appName
                            anchors.left: appIconSlot.right
                            anchors.leftMargin: K.Style.s2
                            anchors.right: appCount.left
                            anchors.rightMargin: K.Style.s2
                            anchors.verticalCenter: parent.verticalCenter
                            text: groupBlock.modelData.name
                            role: "label"
                            tone: "dim"
                        }

                        K.Label {
                            id: appCount
                            anchors.right: collapseGlyph.left
                            anchors.rightMargin: K.Style.s2
                            anchors.verticalCenter: parent.verticalCenter
                            text: String(groupBlock.modelData.notifications.length)
                            role: "tag"
                            tone: "muted"
                            numeric: true
                        }

                        K.Glyph {
                            id: collapseGlyph
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            name: "expand_less"
                            size: 20
                            tone: "variant"
                            rotation: groupBlock.collapsed ? 180 : 0

                            Behavior on rotation {
                                enabled: !K.Style.still
                                NumberAnimation {
                                    duration: K.Style.effects
                                    easing.type: Easing.OutCubic
                                }
                            }
                        }

                        K.Press {
                            id: groupPress
                            radius: K.Style.radius
                            onClicked: mouse => root.toggleGroup(groupBlock.modelData.key)
                        }
                    }

                    Column {
                        width: parent.width
                        spacing: 0
                        visible: !groupBlock.collapsed

                        Repeater {
                            model: groupBlock.modelData.notifications

                            delegate: Item {
                                id: notificationRow

                                required property var modelData
                                readonly property var notification: modelData
                                readonly property var rowActions: root.actionsFor(notification)
                                property real collapseProgress: 1
                                property bool dismissing: false

                                width: parent.width
                                height: rowSurface.implicitHeight * collapseProgress
                                opacity: collapseProgress
                                clip: true

                                function dismiss() {
                                    if (dismissing)
                                        return;
                                    dismissing = true;
                                    collapseAnimation.restart();
                                }

                                NumberAnimation {
                                    id: collapseAnimation
                                    target: notificationRow
                                    property: "collapseProgress"
                                    from: 1
                                    to: 0
                                    duration: K.Style.effects
                                    easing.type: Easing.OutCubic
                                    onFinished: Notifs.dismiss(notificationRow.notification)
                                }

                                NumberAnimation {
                                    id: returnAnimation
                                    target: rowSurface
                                    property: "x"
                                    to: 0
                                    duration: K.Style.effects
                                    easing.type: Easing.OutCubic
                                }

                                Item {
                                    id: rowSurface

                                    width: parent.width
                                    implicitHeight: notificationContent.implicitHeight + K.Style.s3 * 2
                                    height: implicitHeight
                                    opacity: 1 - Math.min(0.55, Math.abs(x) / Math.max(1, width))

                                    Rectangle {
                                        anchors.left: parent.left
                                        anchors.right: parent.right
                                        anchors.top: parent.top
                                        height: K.Style.border
                                        color: K.Style.lineSoft
                                    }

                                    Column {
                                        id: notificationContent

                                        anchors.left: parent.left
                                        anchors.right: parent.right
                                        anchors.top: parent.top
                                        anchors.margins: K.Style.s3
                                        spacing: K.Style.s1

                                        Item {
                                            width: parent.width
                                            height: Math.max(summaryLabel.implicitHeight,
                                                timeLabel.implicitHeight, dismissButton.implicitHeight)

                                            K.Label {
                                                id: summaryLabel
                                                anchors.left: parent.left
                                                anchors.right: timeLabel.left
                                                anchors.rightMargin: K.Style.s2
                                                anchors.verticalCenter: parent.verticalCenter
                                                text: String(notificationRow.notification.summary
                                                    || "Notification")
                                                role: "bodyStrong"
                                            }

                                            K.Label {
                                                id: timeLabel
                                                anchors.right: dismissButton.left
                                                anchors.rightMargin: K.Style.s1
                                                anchors.verticalCenter: parent.verticalCenter
                                                text: Notifs.timeLabel(notificationRow.notification)
                                                role: "caption"
                                                tone: "muted"
                                                numeric: true
                                            }

                                            K.IconButton {
                                                id: dismissButton
                                                anchors.right: parent.right
                                                anchors.verticalCenter: parent.verticalCenter
                                                size: 28
                                                icon: "close"
                                                opacity: rowHover.hovered && !notificationRow.dismissing ? 1 : 0
                                                enabled: opacity > 0
                                                onClicked: notificationRow.dismiss()

                                                Behavior on opacity {
                                                    enabled: !K.Style.still
                                                    NumberAnimation { duration: K.Style.quick }
                                                }
                                            }
                                        }

                                        K.Label {
                                            width: parent.width
                                            visible: text.length > 0
                                            text: String(notificationRow.notification.body || "")
                                            role: "body"
                                            tone: "dim"
                                            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                                            maximumLineCount: 3
                                            elide: Text.ElideRight
                                        }

                                        Flow {
                                            width: parent.width
                                            visible: notificationRow.rowActions.length > 0
                                            spacing: K.Style.s2

                                            Repeater {
                                                model: notificationRow.rowActions

                                                delegate: K.Button {
                                                    required property var modelData
                                                    text: root.actionLabel(modelData)
                                                    variant: "outline"
                                                    onClicked: modelData.invoke()
                                                }
                                            }
                                        }
                                    }

                                    HoverHandler { id: rowHover }

                                    DragHandler {
                                        id: swipeHandler
                                        target: rowSurface
                                        xAxis.minimum: -notificationRow.width
                                        xAxis.maximum: notificationRow.width
                                        yAxis.enabled: false
                                        onActiveChanged: {
                                            if (active)
                                                returnAnimation.stop();
                                            else if (Math.abs(rowSurface.x) > notificationRow.width * 0.28)
                                                notificationRow.dismiss();
                                            else
                                                returnAnimation.restart();
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }

        Item {
            width: parent.width
            height: 120
            visible: root.groups.length === 0

            Column {
                anchors.centerIn: parent
                spacing: K.Style.s2

                K.Glyph {
                    anchors.horizontalCenter: parent.horizontalCenter
                    name: "notifications"
                    size: 48
                    tone: "faint"
                }

                K.Label {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: "No notifications"
                    role: "body"
                    tone: "variant"
                }
            }
        }
    }
}

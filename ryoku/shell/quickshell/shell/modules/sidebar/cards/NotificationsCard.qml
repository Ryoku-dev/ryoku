pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui
import Ryoku.Ui.Singletons
import ".."

Item {
    id: root

    required property real s
    required property bool open
    required property real reveal
    required property bool tabActive
    property int index: 0
    signal requestClose()

    readonly property var notices: root.open && root.tabActive ? Notifs.history : []
    readonly property real gap: Tokens.s3 * root.s
    readonly property real pad: Tokens.s4 * root.s

    implicitHeight: shell.implicitHeight

    function visibleActions(notification): var {
        const actions = notification && notification.actions ? notification.actions : [];
        const out = [];
        for (let i = 0; i < actions.length; i++) {
            if (actions[i] && actions[i].identifier !== "default" && (actions[i].text || "").length > 0)
                out.push(actions[i]);
        }
        return out;
    }

    function defaultAction(notification): var {
        const actions = notification && notification.actions ? notification.actions : [];
        for (let i = 0; i < actions.length; i++) {
            if (actions[i] && actions[i].identifier === "default")
                return actions[i];
        }
        return null;
    }

    function invoke(action): void {
        if (action && typeof action.invoke === "function")
            action.invoke();
        root.requestClose();
    }

    component Notice: Rectangle {
        id: notice
        required property var notification

        readonly property var actions: root.visibleActions(notice.notification)
        readonly property var openAction: root.defaultAction(notice.notification)

        implicitHeight: noticeColumn.implicitHeight + root.pad * 2
        radius: Tokens.radius * root.s
        color: noticeHover.hovered ? Tokens.tint10 : Tokens.tint5
        border.width: Tokens.border
        border.color: noticeHover.hovered ? Tokens.lineStrong : Tokens.line
        scale: closeTap.pressed ? 0.985 : noticeHover.hovered && !Tokens.reduceMotion ? 1.006 : 1
        Behavior on color { ColorAnimation { duration: Tokens.snap } }
        Behavior on scale { NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap } }

        HoverHandler { id: noticeHover }

        Column {
            id: noticeColumn
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: root.pad
            spacing: Tokens.s2 * root.s

            Item {
                width: parent.width
                implicitHeight: Math.max(appLabel.implicitHeight, actionCluster.implicitHeight)

                Text {
                    id: appLabel
                    anchors.left: parent.left
                    anchors.right: timeLabel.left
                    anchors.rightMargin: Tokens.s2 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    text: notice.notification ? notice.notification.appName || I18n.tr("Notification") : I18n.tr("Notification")
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro * root.s
                    font.weight: Font.DemiBold
                    font.letterSpacing: Tokens.trackLabel
                    elide: Text.ElideRight
                }
                Text {
                    id: timeLabel
                    anchors.right: actionCluster.left
                    anchors.rightMargin: Tokens.s2 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    text: Notifs.timeLabel(notice.notification)
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny * root.s
                }
                Row {
                    id: actionCluster
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s1 * root.s

                    Rectangle {
                        visible: notice.openAction !== null
                        width: 28 * root.s
                        height: width
                        radius: Tokens.radius * root.s
                        color: openTap.pressed ? Tokens.tint16 : openHover.hovered ? Tokens.tint10 : "transparent"
                        border.width: Tokens.border
                        border.color: openHover.hovered ? Tokens.lineStrong : Tokens.lineSoft
                        Text {
                            anchors.centerIn: parent
                            text: "open_in_new"
                            color: Tokens.inkDim
                            font.family: "Material Symbols Rounded"
                            font.pixelSize: 16 * root.s
                        }
                        HoverHandler { id: openHover; cursorShape: Qt.PointingHandCursor }
                        TapHandler { id: openTap; onTapped: root.invoke(notice.openAction) }
                    }
                    Rectangle {
                        width: 28 * root.s
                        height: width
                        radius: Tokens.radius * root.s
                        color: closeTap.pressed ? Tokens.tint16 : closeHover.hovered ? Tokens.tint10 : "transparent"
                        border.width: Tokens.border
                        border.color: closeHover.hovered ? Tokens.lineStrong : Tokens.lineSoft
                        Text {
                            anchors.centerIn: parent
                            text: "close"
                            color: closeHover.hovered ? Tokens.alert : Tokens.inkDim
                            font.family: "Material Symbols Rounded"
                            font.pixelSize: 16 * root.s
                        }
                        HoverHandler { id: closeHover; cursorShape: Qt.PointingHandCursor }
                        TapHandler { id: closeTap; onTapped: Notifs.dismiss(notice.notification) }
                    }
                }
            }

            Text {
                width: parent.width
                text: notice.notification ? notice.notification.summary || "" : ""
                visible: text.length > 0
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fRow * root.s
                font.weight: Font.DemiBold
                wrapMode: Text.WrapAtWordBoundaryOrAnywhere
            }
            Text {
                width: parent.width
                text: notice.notification ? notice.notification.body || "" : ""
                visible: text.length > 0
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall * root.s
                wrapMode: Text.WrapAtWordBoundaryOrAnywhere
            }
            Flow {
                width: parent.width
                spacing: Tokens.s2 * root.s
                visible: notice.actions.length > 0
                Repeater {
                    model: notice.actions
                    delegate: Btn {
                        required property var modelData
                        text: modelData.text
                        compact: true
                        primary: true
                        onAct: root.invoke(modelData)
                    }
                }
            }
        }
    }

    SidebarCardShell {
        id: shell
        width: root.width
        index: root.index
        open: root.open
        reveal: root.reveal
        tabActive: root.tabActive
        title: I18n.tr("Notifications")
        glyph: "notifications"
        eyebrow: root.notices.length === 0 ? I18n.tr("ALL CLEAR") : I18n.tr("%1 RECENT").arg(root.notices.length)

        Column {
            width: parent.width
            spacing: root.gap

            Rectangle {
                width: parent.width
                implicitHeight: 64 * root.s
                radius: Tokens.radius * root.s
                color: Flags.dnd ? Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.14) : Tokens.tint5
                border.width: Tokens.border
                border.color: Flags.dnd ? Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.5) : Tokens.line

                Row {
                    anchors.left: parent.left
                    anchors.right: dndSwitch.left
                    anchors.leftMargin: root.pad
                    anchors.rightMargin: Tokens.s3 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s3 * root.s
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: Flags.dnd ? "notifications_off" : "notifications_active"
                        color: Flags.dnd ? Tokens.sun : Tokens.inkDim
                        font.family: "Material Symbols Rounded"
                        font.pixelSize: 25 * root.s
                    }
                    Column {
                        anchors.verticalCenter: parent.verticalCenter
                        width: parent.width - 36 * root.s
                        spacing: Tokens.s1 * root.s
                        Text {
                            width: parent.width
                            text: I18n.tr("Do not disturb")
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fRow * root.s
                            font.weight: Font.DemiBold
                        }
                        Text {
                            width: parent.width
                            text: Flags.dnd ? I18n.tr("Banners are paused") : I18n.tr("Banners are live")
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fMicro * root.s
                        }
                    }
                }
                Sw {
                    id: dndSwitch
                    anchors.right: parent.right
                    anchors.rightMargin: root.pad
                    anchors.verticalCenter: parent.verticalCenter
                    on: Flags.dnd
                    onToggled: Toggles.toggleDnd()
                }
            }

            Item {
                width: parent.width
                implicitHeight: Math.max(historyLabel.implicitHeight, clearButton.implicitHeight)
                Text {
                    id: historyLabel
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("History")
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow * root.s
                    font.weight: Font.DemiBold
                }
                Btn {
                    id: clearButton
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Clear all")
                    compact: true
                    armed: root.notices.length > 0
                    onAct: {
                        Notifs.clearAll();
                        root.requestClose();
                    }
                }
            }

            Rectangle {
                visible: root.notices.length === 0
                width: parent.width
                implicitHeight: emptyColumn.implicitHeight + Tokens.s6 * root.s
                radius: Tokens.radius * root.s
                color: Tokens.tint5
                border.width: Tokens.border
                border.color: Tokens.line
                Column {
                    id: emptyColumn
                    anchors.centerIn: parent
                    spacing: Tokens.s2 * root.s
                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: "done_all"
                        color: Tokens.sun
                        font.family: "Material Symbols Rounded"
                        font.pixelSize: 36 * root.s
                    }
                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: I18n.tr("Nothing needs your attention")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                    }
                }
            }

            Repeater {
                model: root.notices
                delegate: Notice {
                    required property var modelData
                    width: parent.width
                    notification: modelData
                }
            }
        }
    }
}

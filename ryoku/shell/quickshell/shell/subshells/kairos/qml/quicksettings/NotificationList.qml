pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons
import "../kit"

Column {
    id: list

    property var pane
    property var expanded: ({})

    readonly property var groups: {
        var hist = Notifs.history || [], order = [], byApp = {};
        for (var i = 0; i < hist.length; i++) {
            var n = hist[i];
            if (!n) continue;
            var key = String(n.appName || "");
            if (!byApp[key]) {
                byApp[key] = [];
                order.push(key);
            }
            byApp[key].push(n);
        }
        return order.map(function (k) { return { app: k, items: byApp[k] }; });
    }

    spacing: 12

    Item {
        width: parent.width
        height: 40

        Label {
            anchors { left: parent.left; leftMargin: 4; verticalCenter: parent.verticalCenter }
            role: "title"
            text: I18n.tr("Notifications")
        }
        Button {
            anchors { right: parent.right; verticalCenter: parent.verticalCenter }
            visible: Notifs.history.length > 0
            variant: "tonal"
            height: 34
            text: I18n.tr("Clear all")
            onClicked: Notifs.clearAll()
        }
    }

    Item {
        width: parent.width
        height: 96
        visible: list.groups.length === 0

        Column {
            anchors.centerIn: parent
            spacing: 8
            Icon {
                anchors.horizontalCenter: parent.horizontalCenter
                name: Flags.dnd ? "do_not_disturb_on" : "done_all"
                size: 28
                color: Md.inkMuted
            }
            Label {
                anchors.horizontalCenter: parent.horizontalCenter
                role: "body"
                color: Md.inkMuted
                text: Flags.dnd ? I18n.tr("Do not disturb is on") : I18n.tr("You're all caught up")
            }
        }
    }

    Repeater {
        model: list.groups
        delegate: Column {
            id: group
            required property var modelData
            readonly property bool open: list.expanded[group.modelData.app] === true
            readonly property var shown: group.open ? group.modelData.items : group.modelData.items.slice(0, 3)
            readonly property int hidden: group.modelData.items.length - group.shown.length
            width: list.width
            spacing: Md.seam

            Repeater {
                model: group.shown
                delegate: NotificationItem {
                    required property var modelData
                    required property int index
                    width: group.width
                    notif: modelData
                    position: {
                        var last = group.shown.length - 1 + (group.hidden > 0 ? 1 : 0);
                        if (last === 0) return "only";
                        if (index === 0) return "first";
                        return index === last ? "last" : "middle";
                    }
                    onOpened: list.pane.close()
                }
            }

            ListItem {
                visible: group.hidden > 0
                width: group.width
                implicitHeight: 48
                position: "last"
                headline: I18n.tr("Show %1 more").replace("%1", group.hidden)
                onClicked: {
                    var e = Object.assign({}, list.expanded);
                    e[group.modelData.app] = true;
                    list.expanded = e;
                }
            }
        }
    }
}

pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons
import "../../kit"

// a second press for the session enders, a stray click shouldn't end everything
Column {
    id: page

    property var host
    property string armed: ""

    spacing: 10

    function press(action) {
        if (action === "lock" || action === "suspend" || page.armed === action) {
            page.armed = "";
            page.host.close();
            SessionActions.run(action);
            return;
        }
        page.armed = action;
        standDown.restart();
    }
    Timer {
        id: standDown
        interval: 4000
        onTriggered: page.armed = ""
    }
    Component.onCompleted: Session.watchers += 1
    Component.onDestruction: Session.watchers = Math.max(0, Session.watchers - 1)

    TopBar {
        width: parent.width
        title: I18n.tr("Power")
        onBack: page.host.back()
    }

    component PowerButton: Item {
        id: button
        required property string action
        required property string icon
        required property string label
        required property string confirm
        property bool primary: false
        readonly property bool hot: page.armed === button.action
        readonly property color body: button.hot ? Md.error : button.primary ? Md.accent : Md.containerHigh
        readonly property color ink: button.hot ? Md.onError : button.primary ? Md.onAccent : Md.ink

        Accessible.role: Accessible.Button
        Accessible.name: button.hot ? button.confirm : button.label

        Rectangle {
            id: shape
            anchors.fill: parent
            radius: hit.pressed ? Md.rLg : (button.hot ? height / 2 : Md.rXxl)
            color: button.body
            Behavior on radius {
                enabled: !Md.still
                Spring { speed: "fast" }
            }
            Behavior on color {
                enabled: !Md.still
                ColorAnimation { duration: Md.fast }
            }
        }
        Column {
            anchors.centerIn: parent
            width: parent.width - 24
            spacing: 8

            Icon {
                anchors.horizontalCenter: parent.horizontalCenter
                name: button.icon
                size: 28
                filled: true
                color: button.ink
            }
            Label {
                width: parent.width
                horizontalAlignment: Text.AlignHCenter
                role: "label"
                color: button.ink
                wrapMode: Text.Wrap
                maximumLineCount: 2
                text: button.hot ? button.confirm : button.label
            }
        }
        StateLayer {
            id: hit
            radius: shape.radius
            tint: button.ink
            onActivated: page.press(button.action)
        }
    }

    Row {
        width: parent.width
        spacing: 10

        PowerButton {
            width: (parent.width - parent.spacing) / 2
            height: 120
            primary: true
            action: "shutdown"
            icon: "power_settings_new"
            label: I18n.tr("Shut down")
            confirm: I18n.tr("Press again to shut down")
        }
        PowerButton {
            width: (parent.width - parent.spacing) / 2
            height: 120
            action: "reboot"
            icon: "restart_alt"
            label: I18n.tr("Restart")
            confirm: I18n.tr("Press again to restart")
        }
    }

    Row {
        width: parent.width
        spacing: 10

        PowerButton {
            width: (parent.width - 2 * parent.spacing) / 3
            height: 96
            action: "suspend"
            icon: "bedtime"
            label: I18n.tr("Sleep")
            confirm: ""
        }
        PowerButton {
            width: (parent.width - 2 * parent.spacing) / 3
            height: 96
            action: "lock"
            icon: "lock"
            label: I18n.tr("Lock")
            confirm: ""
        }
        PowerButton {
            width: (parent.width - 2 * parent.spacing) / 3
            height: 96
            action: "logout"
            icon: "logout"
            label: I18n.tr("Log out")
            confirm: I18n.tr("Press again to log out")
        }
    }

    Label {
        width: parent.width
        visible: Session.uptimeText.length > 0
        topPadding: 4
        horizontalAlignment: Text.AlignHCenter
        role: "caption"
        color: Md.inkMuted
        text: I18n.tr("Up for %1").replace("%1", Session.uptimeText)
    }

    Item { width: 1; height: 4 }
}

pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../kit" as K

Item {
    id: root

    required property var scene
    property bool shown: false
    property var args: ({})
    property bool watchingSession: false

    readonly property var actions: [
        { id: "lock", label: "Lock", icon: "lock", danger: false },
        { id: "logout", label: "Log out", icon: "logout", danger: false },
        { id: "suspend", label: "Suspend", icon: "bedtime", danger: false },
        { id: "reboot", label: "Restart", icon: "restart_alt", danger: true },
        { id: "shutdown", label: "Shut down", icon: "power_settings_new", danger: true }
    ]

    implicitWidth: sheet.implicitWidth
    implicitHeight: sheet.implicitHeight

    function watchSession(enabled) {
        if (watchingSession === enabled)
            return;
        watchingSession = enabled;
        Session.watchers = Math.max(0, Session.watchers + (enabled ? 1 : -1));
    }

    onShownChanged: watchSession(shown)
    Component.onCompleted: watchSession(shown)
    Component.onDestruction: watchSession(false)

    K.Sheet {
        id: sheet
        pageWidth: 280
        title: Session.user || "Session"

        K.Label {
            width: parent.width
            text: "Up " + Session.uptimeText
            role: "caption"
            tone: "muted"
            numeric: true
        }

        Column {
            width: parent.width
            spacing: K.Style.s1

            Repeater {
                model: root.actions

                delegate: K.HoldButton {
                    required property var modelData
                    width: parent.width
                    leading: true
                    icon: modelData.icon
                    text: modelData.label
                    danger: modelData.danger
                    holdMs: 800
                    onActivated: SessionActions.run(modelData.id)
                }
            }
        }

        K.Label {
            width: parent.width
            text: "Hold to run"
            role: "tag"
            tone: "faint"
        }
    }
}

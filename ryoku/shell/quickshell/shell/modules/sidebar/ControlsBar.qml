pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import shell.services

Item {
    id: root

    required property real s
    required property var screen
    required property bool active
    signal session(string action)
    signal openExtensions()
    signal requestClose()
    function elementVisible(id) {
        return SidebarState.elementVisible(id);
    }

    implicitHeight: (Tokens.rowH + Tokens.s3) * s

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: Tokens.border
        color: Tokens.lineSoft
    }

    Row {
        id: bar
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: Tokens.rowH * root.s
        spacing: Tokens.s2 * root.s

        Row {
            id: sessionActions
            readonly property var allActions: [
                { id: "lock", action: "lock", glyph: "lock", label: I18n.tr("Lock") },
                { id: "sleep", action: "suspend", glyph: "dark_mode", label: I18n.tr("Sleep") },
                { id: "logout", action: "logout", glyph: "logout", label: I18n.tr("Log out") },
                { id: "restart", action: "reboot", glyph: "restart_alt", label: I18n.tr("Restart") },
                { id: "powerOff", action: "shutdown", glyph: "power_settings_new", label: I18n.tr("Power off") }
            ]
            readonly property var actions: allActions.filter(entry => root.elementVisible(entry.id))
            readonly property int actionCount: actions.length
            width: actionCount > 0 ? Math.max(
                (Tokens.ctlH + Tokens.s3) * root.s * actionCount + spacing * Math.max(0, actionCount - 1),
                bar.width - toggles.implicitWidth - gear.implicitWidth - divider.width - bar.spacing * 3) : 0
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s1 * root.s
            readonly property real buttonWidth: actionCount > 0
                ? (width - spacing * Math.max(0, actionCount - 1)) / actionCount : 0
            // Labels are all or nothing: a bar mixing labelled and icon-only
            // actions reads as five different controls.
            readonly property bool labelled: widest.width + (Tokens.s6 + Tokens.s5) * root.s <= buttonWidth

            TextMetrics {
                id: widest
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall * root.s
                font.weight: Font.Medium
                text: sessionActions.actions.map(entry => entry.label).reduce((a, b) => a.length >= b.length ? a : b, "")
            }

            Repeater {
                model: sessionActions.actions
                delegate: HoldButton {
                    required property var modelData
                    width: sessionActions.buttonWidth
                    s: root.s
                    glyph: modelData.glyph
                    text: modelData.label
                    subtle: true
                    labelled: sessionActions.labelled
                    onHeld: root.session(modelData.action)
                }
            }
        }

        Rectangle {
            id: divider
            visible: sessionActions.actionCount > 0 && (toggles.hasControls || gear.visible)
            anchors.verticalCenter: parent.verticalCenter
            width: Tokens.border
            height: Tokens.ctlH * root.s
            color: Tokens.lineSoft
        }

        Row {
            id: toggles
            readonly property bool hasControls: implicitWidth > 0
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s1 * root.s

            CornerButton {
                visible: root.elementVisible("nightLight") && Wm.caps.nightLight === true
                s: root.s
                glyph: Toggles.nightOn ? "bedtime" : "light_mode"
                checked: Toggles.nightOn
                subtle: true
                Accessible.name: I18n.tr("Night light")
                onClicked: Toggles.toggleNight()
            }
            CornerButton {
                s: root.s
                visible: root.elementVisible("keepAwake")
                glyph: "coffee"
                checked: Toggles.keepAwake
                subtle: true
                Accessible.name: I18n.tr("Keep awake")
                onClicked: Toggles.toggleCaffeine()
            }
            CornerButton {
                s: root.s
                visible: root.elementVisible("doNotDisturb")
                glyph: "notifications_off"
                checked: Toggles.dnd
                subtle: true
                Accessible.name: I18n.tr("Do not disturb")
                onClicked: Toggles.toggleDnd()
            }
            CornerButton {
                s: root.s
                visible: root.elementVisible("micMute")
                glyph: Toggles.micMuted ? "mic_off" : "mic"
                enabled: !!(Audio.source && Audio.source.audio)
                checked: Toggles.micMuted
                subtle: true
                Accessible.name: I18n.tr("Microphone mute")
                onClicked: Toggles.toggleMic()
            }
            CornerButton {
                visible: root.elementVisible("gamingMode") && (Wm.caps.liveConfigEval === true || Toggles.gameMode)
                s: root.s
                glyph: "sports_esports"
                checked: Toggles.gameMode
                subtle: true
                Accessible.name: I18n.tr("Gaming mode")
                onClicked: Toggles.toggleGame()
            }
        }

        CornerButton {
            id: gear
            visible: root.elementVisible("panelSettings")
            anchors.verticalCenter: parent.verticalCenter
            s: root.s
            glyph: "settings"
            checked: settings.opened
            subtle: true
            Accessible.name: I18n.tr("Panel settings")
            onClicked: settings.opened ? settings.close() : settings.open()
        }
    }

    ControlsSettingsPopup {
        id: settings
        parent: root
        x: Math.max(0, root.width - width)
        y: -height - Tokens.s2 * root.s
        s: root.s
        active: root.active
        onOpenExtensions: root.openExtensions()
        onRequestClose: root.requestClose()
    }
}

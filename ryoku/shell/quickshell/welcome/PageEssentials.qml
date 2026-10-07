pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: root

    signal runCommand(var argv)

    readonly property var essentials: [
        { chord: I18n.tr("Super + Space"), title: I18n.tr("Launcher"), description: I18n.tr("Find apps and files."), command: ["ryoku-shell", "launcher"] },
        { chord: I18n.tr("Alt + Space"), title: I18n.tr("AI bar"), description: I18n.tr("Ask a quick question."), command: ["ryoku-shell", "ask"] },
        { chord: I18n.tr("Super + comma"), title: I18n.tr("Ryoku Hub"), description: I18n.tr("Change system and desktop settings."), command: ["ryoku-shell", "hub", "open"] },
        { chord: I18n.tr("Super + Escape"), title: I18n.tr("Quick settings"), description: I18n.tr("Open controls and session actions."), command: ["ryoku-shell", "quicksettings"] },
        { chord: I18n.tr("Super + K"), title: I18n.tr("Cheatsheet"), description: I18n.tr("See every shortcut."), command: ["sh", "-c", "pkill -x -f 'qs -c keys' 2>/dev/null || flock -n -o /tmp/ryoku-keys.lock qs -c keys"] },
        { chord: I18n.tr("Super + Shift + S"), title: I18n.tr("Screenshot"), description: I18n.tr("Capture, edit, scan or record."), command: [] },
        { chord: I18n.tr("Super + Return"), title: I18n.tr("Terminal"), description: I18n.tr("Open the configured terminal."), command: ["ryoku-app", "terminal"] },
        { chord: I18n.tr("Super + L"), title: I18n.tr("Lock"), description: I18n.tr("Lock the current session."), command: ["ryoku-shell", "lock"] }
    ]

    Column {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        spacing: Tokens.s4

        PageTitle {
            width: parent.width
            title: I18n.tr("The keys that matter")
            description: I18n.tr("These eight chords cover most days. Click a row when the action can open from here.")
        }

        Flow {
            id: list
            width: parent.width
            spacing: Tokens.s2

            Repeater {
                model: root.essentials

                ActionKey {
                    required property var modelData
                    width: Math.floor((list.width - list.spacing) / 2)
                    chord: modelData.chord
                    title: modelData.title
                    description: modelData.description
                    command: modelData.command
                    onRunCommand: argv => root.runCommand(argv)
                }
            }
        }
    }
}

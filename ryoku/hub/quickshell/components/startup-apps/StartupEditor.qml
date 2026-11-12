import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../default-apps"

// one startup app: find it by name or type its command
Column {
    id: editor

    property var item: ({})
    readonly property bool valid: command.text.trim().length > 0

    function value() {
        return Object.assign({}, editor.item, { command: command.text.trim() });
    }

    spacing: LithicTokens.space.s3

    AppFinder {
        width: parent.width
        plane: Lithic.field
        current: command.text.trim()
        onPicked: (name, cmd) => command.text = cmd
    }

    TextField {
        id: command
        width: parent.width
        label: I18n.tr("Command")
        helpText: I18n.tr("What runs at sign-in, such as nm-applet or a script's path")
        mono: true
        text: editor.item.command || ""
    }
}

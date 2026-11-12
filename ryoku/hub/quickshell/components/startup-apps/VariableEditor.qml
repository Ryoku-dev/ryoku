import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// one session variable: a name and its value
Column {
    id: editor

    property var item: ({})
    // names already set by the other variables
    property var taken: []
    readonly property string name: nameField.text.trim()
    readonly property bool nameOk: /^[A-Za-z_][A-Za-z0-9_]*$/.test(editor.name)
    readonly property bool valid: editor.nameOk && editor.taken.indexOf(editor.name) < 0
    property bool nameLeft: false

    function value() {
        return Object.assign({}, editor.item, { key: editor.name, value: valueField.text });
    }

    spacing: LithicTokens.space.s3

    TextField {
        id: nameField
        width: parent.width
        label: I18n.tr("Name")
        placeholderText: "MOZ_ENABLE_WAYLAND"
        mono: true
        text: editor.item.key || ""
        errorText: !editor.nameLeft || editor.name.length === 0 ? ""
            : !editor.nameOk ? I18n.tr("Use letters, digits and underscores, starting with a letter")
            : editor.taken.indexOf(editor.name) >= 0 ? I18n.tr("%1 is already set in this list").arg(editor.name)
            : ""
        onActiveFocusChanged: {
            if (!activeFocus)
                editor.nameLeft = true;
        }
    }

    TextField {
        id: valueField
        width: parent.width
        label: I18n.tr("Value")
        placeholderText: "1"
        mono: true
        text: editor.item.value || ""
    }
}

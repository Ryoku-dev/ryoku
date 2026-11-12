import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property string path: ""
    property string label: ""
    property string kind: "folder"
    property string placeholder: kind === "file" ? Lithic.tr("Choose a file") : Lithic.tr("Choose a folder")
    property string errorText: ""
    property string accessibleName: label.length > 0 ? label : placeholder
    property string previewState: ""

    // the host opens the portal dialog and sets path; the library never opens one
    signal chooseRequested()
    signal edited(string path)

    implicitWidth: LithicTokens.size.settingsControlMax
    implicitHeight: field.implicitHeight
    padding: 0

    Accessible.role: Accessible.Grouping
    Accessible.name: accessibleName
    // the field holds focus itself, so Ctrl+Z reaches whoever holds it; a typed
    // character opens the path with that character, and a click into the path edits it
    focusPolicy: Qt.StrongFocus
    Keys.onPressed: event => {
        if (field.activeFocus || event.text.length !== 1 || event.text.charCodeAt(0) <= 32
                || (event.modifiers & (Qt.ControlModifier | Qt.AltModifier | Qt.MetaModifier)))
            return
        field.forceActiveFocus(Qt.OtherFocusReason)
        field.text = event.text
        event.accepted = true
    }

    onPathChanged: {
        if (!field.activeFocus)
            field.text = path;
    }

    // hands focus back to the field from the path, with the keyboard mark
    function _leavePath() {
        root.forceActiveFocus(Qt.TabFocusReason);
    }

    contentItem: Item {
        TextField {
            id: field
            width: parent.width
            label: root.label
            mono: true
            text: root.path
            placeholderText: root.placeholder
            errorText: root.errorText
            enabled: root.enabled
            rightPadding: LithicTokens.size.textFieldSecretAction + LithicTokens.size.textFieldSecretInset * 2
            // the field's own focus shows on the path
            previewState: root.previewState === "chooseFocus" ? ""
                        : root.visualFocus ? "focus" : root.previewState
            focusPolicy: Qt.ClickFocus
            activeFocusOnTab: false
            Accessible.name: root.accessibleName
            // leaving the path, by Return, Enter, Tab or a click elsewhere, reports a changed one
            onActiveFocusChanged: {
                if (!activeFocus && text !== root.path)
                    root.edited(text);
            }
            Keys.onReturnPressed: root._leavePath()
            Keys.onEnterPressed: root._leavePath()
            // Escape drops an edit; with nothing typed it goes on, so a dialog can close
            Keys.onEscapePressed: event => {
                const edited = field.text !== root.path
                field.text = root.path
                root._leavePath()
                event.accepted = edited
            }
        }

        // beside the path rather than inside it, so keys on it never reach the text input;
        // it still fades with the path when the field is off, as it did inside it
        Item {
            x: field.x
            y: field.y
            width: field.width
            height: field.height
            opacity: field.opacity

            IconPlaque {
                id: choose
                anchors.right: parent.right
                anchors.rightMargin: LithicTokens.size.textFieldSecretInset
                y: field._labelHeight + LithicTokens.size.textFieldSecretInset
                small: true
                kind: "quiet"
                glyph: "folder_open"
                // a name, never the placeholder, which a host may set to a whole path
                accessibleName: root.label.length > 0 ? Lithic.tr("Choose %1").arg(root.label)
                              : root.kind === "file" ? Lithic.tr("Choose a file") : Lithic.tr("Choose a folder")
                previewState: root.previewState === "chooseFocus" ? "focus" : ""
                onClicked: root.chooseRequested()
            }
        }
    }
}

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

    onPathChanged: {
        if (!field.activeFocus)
            field.text = path;
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
            previewState: root.previewState === "chooseFocus" ? "" : root.previewState
            Accessible.name: root.accessibleName
            onEditingFinished: {
                if (text !== root.path)
                    root.edited(text);
            }

            IconPlaque {
                anchors.right: parent.right
                anchors.rightMargin: LithicTokens.size.textFieldSecretInset
                y: field._labelHeight + LithicTokens.size.textFieldSecretInset
                small: true
                kind: "quiet"
                glyph: "folder_open"
                accessibleName: root.label.length > 0 ? Lithic.tr("Choose %1").arg(root.label) : root.placeholder
                previewState: root.previewState === "chooseFocus" ? "focus" : ""
                onClicked: root.chooseRequested()
            }
        }
    }
}

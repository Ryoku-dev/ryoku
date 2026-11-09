import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property string title: ""
    property string text: ""
    property string actionText: "Confirm"
    property string requiredName: ""
    property bool destructive: true
    property bool busy: false
    property Item overlayParent: null
    property Item opener: null
    property string previewState: ""
    property bool previewEmbedded: false
    property bool previewOpen: false
    property alias inputText: confirmationField.text
    readonly property bool ready: requiredName.length === 0 || inputText === requiredName
    readonly property bool opened: previewOpen || dialog.opened

    signal confirmed()
    signal cancelled()

    implicitWidth: LithicTokens.size.dialogWidth
    implicitHeight: 0
    visible: false

    function open() {
        dialog.open()
    }

    function close() {
        dialog.close()
    }

    function reset() {
        confirmationField.text = ""
    }

    Dialog {
        id: dialog
        parent: root.overlayParent ? root.overlayParent : root.parent
        opener: root.opener
        title: root.title
        previewState: root.previewState
        previewEmbedded: root.previewEmbedded || root.previewOpen
        actions: [
            {
                text: "Cancel",
                kind: "quiet",
                closeOnTrigger: true
            },
            {
                text: root.actionText,
                kind: root.destructive ? "danger" : "gold",
                enabled: root.ready && !root.busy,
                busy: root.busy,
                closeOnTrigger: true
            }
        ]

        onActionTriggered: index => {
            if (index === 0) {
                root.cancelled()
            } else if (root.ready) {
                root.confirmed()
            }
        }
        onClosed: root.reset()

        Column {
            width: parent.width
            spacing: root.requiredName.length > 0
                     ? LithicTokens.size.dialogConfirmFieldGap : 0

            Label {
                width: parent.width
                text: root.text
                typeRole: "body"
                inkRole: "fg2"
                wrapMode: Text.Wrap
            }

            TextField {
                id: confirmationField
                visible: root.requiredName.length > 0
                width: parent.width
                label: root.requiredName.length > 0
                       ? "Type " + root.requiredName + " to confirm" : ""
                placeholderText: root.requiredName
            }
        }
    }

}

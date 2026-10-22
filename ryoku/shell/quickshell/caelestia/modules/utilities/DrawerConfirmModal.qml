pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Caelestia.Config
import caelestia.components
import caelestia.components.controls

/// The card a utilities-drawer modal shows when it asks before doing something:
/// a title, what confirming affects, and Cancel / confirm buttons. Cancelling and
/// clicking outside both report `dismissed`, so an instance only has to say what
/// confirming does.
DrawerModal {
    id: root

    /// The question, e.g. qsTr("Delete recording?").
    required property string title
    /// What confirming does to what, e.g. qsTr("Recording '%1' will be permanently deleted.").
    required property string message
    /// The label of the button that confirms.
    required property string confirmText

    /// The user confirmed.
    signal confirmed()

    StyledText {
        text: root.title
        font: Tokens.font.body.large
    }

    StyledText {
        Layout.fillWidth: true
        text: root.message
        color: Colours.palette.m3onSurfaceVariant
        font: Tokens.font.body.small
        wrapMode: Text.WrapAtWordBoundaryOrAnywhere
    }

    RowLayout {
        Layout.topMargin: Tokens.spacing.medium
        Layout.alignment: Qt.AlignRight
        spacing: Tokens.spacing.medium

        TextButton {
            text: qsTr("Cancel")
            type: TextButton.Text
            onClicked: root.dismissed()
        }

        TextButton {
            text: root.confirmText
            type: TextButton.Text
            onClicked: {
                root.confirmed();
                root.dismissed();
            }
        }
    }
}

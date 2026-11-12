import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// an action past 10 s: a sweeping blade, what is happening, and Cancel
Column {
    id: wait

    property string text: ""
    property bool cancellable: true
    signal cancelled()

    width: parent ? parent.width : 0
    spacing: LithicTokens.space.s2

    BladeProgress {
        width: parent.width
        active: wait.visible
        indeterminate: true
    }

    Item {
        width: parent.width
        height: Math.max(note.implicitHeight, cancel.implicitHeight)

        Label {
            id: note
            anchors.left: parent.left
            anchors.right: cancel.visible ? cancel.left : parent.right
            anchors.rightMargin: LithicTokens.space.s3
            anchors.verticalCenter: parent.verticalCenter
            text: wait.text
            typeRole: "detail"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: cancel
            visible: wait.cancellable
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            kind: "quiet"
            small: true
            text: I18n.tr("Cancel")
            onClicked: wait.cancelled()
        }
    }
}

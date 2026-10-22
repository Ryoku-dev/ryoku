pragma ComponentBehavior: Bound

import QtQuick
import Caelestia
import caelestia.components
import caelestia.components.effects

Loader {
    id: root

    required property var props
    required property matrix4x4 deformMatrix

    asynchronous: true
    anchors.fill: parent

    opacity: root.props.recordingConfirmDelete ? 1 : 0
    active: opacity > 0

    sourceComponent: DrawerConfirmModal {
        // The prop is cleared as soon as the modal is dismissed, but the card keeps
        // naming the recording while it fades out.
        property string path

        deformMatrix: root.deformMatrix
        open: root.props.recordingConfirmDelete !== ""
        title: qsTr("Delete recording?")
        message: qsTr("Recording '%1' will be permanently deleted.").arg(path)
        confirmText: qsTr("Delete")
        onDismissed: root.props.recordingConfirmDelete = ""
        onConfirmed: CUtils.deleteFile(Qt.resolvedUrl(path))

        Component.onCompleted: path = root.props.recordingConfirmDelete
    }

    Behavior on opacity {
        Anim {
            type: Anim.DefaultEffects
        }
    }
}

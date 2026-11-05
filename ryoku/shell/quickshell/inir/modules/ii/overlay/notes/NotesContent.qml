pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell.Io
import inir.modules.common
import inir.modules.ii.overlay
import inir.services

OverlayBackground {
    id: root

    property bool ready: false
    property bool adopting: false
    implicitWidth: 420
    implicitHeight: 280
    radius: OverlayLook.roundingNormal

    Component.onCompleted: prepare.running = true

    TextArea {
        id: editor
        anchors.fill: parent
        anchors.margins: 10
        wrapMode: TextEdit.Wrap
        selectByMouse: true
        placeholderText: Translation.tr("Write something here...")
        color: OverlayLook.colOnSurface
        background: null
        onTextChanged: if (root.ready && !root.adopting) saveDelay.restart()
    }

    Timer {
        id: saveDelay
        interval: 400
        onTriggered: file.setText(editor.text)
    }

    Process {
        id: prepare
        command: ["bash", "-lc", "mkdir -p \"$(dirname \"$1\")\" && touch \"$1\"", "bash", Directories.notesPath]
        onExited: (code, status) => {
            if (code === 0) file.reload()
        }
    }

    FileView {
        id: file
        path: Qt.resolvedUrl(Directories.notesPath)
        onLoaded: {
            root.adopting = true
            editor.text = file.text()
            root.adopting = false
            root.ready = true
        }
    }
}

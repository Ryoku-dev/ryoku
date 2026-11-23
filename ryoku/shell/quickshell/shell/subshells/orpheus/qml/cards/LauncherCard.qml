pragma ComponentBehavior: Bound

import QtQuick
import "../launcher" as L

// The launcher as an Orpheus card: the Sheet pours out of the voice drop
// through the card weld. The paper is the bar's own blob, drawn by Body.
FocusScope {
    id: root

    required property var scene
    property bool shown: false
    property var args: ({})
    readonly property alias sheet: sheet

    implicitWidth: sheet.implicitWidth
    implicitHeight: sheet.level
    focus: true

    onShownChanged: if (shown) Qt.callLater(sheet.focusSearch)

    // swallows clicks on the paper so the scene's click-away only fires off the card
    MouseArea {
        anchors.fill: parent
        acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
        onPressed: mouse => mouse.accepted = true
    }

    L.Sheet {
        id: sheet
        anchors.fill: parent
        shown: root.shown
        welded: true
        monitor: root.scene ? root.scene.screenName : ""
        maxHeight: root.scene ? root.scene.launcherMaxHeight : 600
        // the music edge ripples outward below the paper, so no room is reserved
        waveRoom: 0
        onCloseRequested: if (root.scene) root.scene.closeCard()
    }
}

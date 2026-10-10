pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import inir
import inir.modules.iris.frame
import inir.modules.iris.palette
import "../../../../services/lib/screens.js" as Screens

Variants {
    id: root

    property bool open: false
    property string screenName: ""

    model: Screens.uniqueByName(Quickshell.screens)

    PanelWindow {
        id: win

        required property var modelData
        readonly property bool selected: root.screenName.length === 0
            ? String(GlobalStates.focusedScreen?.name ?? "") === String(modelData?.name ?? "")
            : root.screenName === String(modelData?.name ?? "")

        screen: modelData
        visible: win.selected && (root.open || palette.present)
        color: "transparent"
        surfaceFormat.opaque: false
        exclusionMode: ExclusionMode.Ignore
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.namespace: "launcher"
        WlrLayershell.keyboardFocus: root.open && win.selected
            ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
        anchors {
            top: true
            bottom: true
            left: true
            right: true
        }
        // A fresh transparent layer has no first damage on its own. Wake three
        // frames so the palette's warm-up can observe frameSwapped and paint.
        property int wakeFrames: 0
        onVisibleChanged: if (visible) {
            win.wakeFrames = 0
            frameWake.start()
        }
        Timer {
            id: frameWake
            interval: 16
            repeat: true
            onTriggered: {
                const quick = win.contentItem?.Window.window
                if (quick) quick.update()
                if (++win.wakeFrames >= 3) stop()
            }
        }

        IrisPalette {
            id: palette
            anchors.fill: parent
            anchors.margins: IrisFrame.band
            screen: win.modelData
            standalone: true
        }
    }
}

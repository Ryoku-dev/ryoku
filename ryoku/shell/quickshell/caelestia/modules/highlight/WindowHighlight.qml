pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import caelestia.services

// An outline around Kwin.highlightedAddress, one click-through layer per screen.
// Window geometry is global; each layer subtracts its screen's origin.
Variants {
    model: Quickshell.screens

    PanelWindow {
        id: win

        required property var modelData
        readonly property var target: Kwin.highlightedAddress === "" ? null : (Kwin.windowList || []).find(w => w.address === Kwin.highlightedAddress && w.output === win.modelData.name) ?? null

        screen: modelData
        visible: win.target !== null
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
        WlrLayershell.namespace: "caelestia-window-highlight"
        anchors {
            top: true
            bottom: true
            left: true
            right: true
        }
        mask: Region {}

        Rectangle {
            x: win.target ? win.target.x - win.modelData.x : 0
            y: win.target ? win.target.y - win.modelData.y : 0
            width: win.target ? win.target.width : 0
            height: win.target ? win.target.height : 0
            radius: 12
            color: Qt.rgba(Colours.palette.m3primary.r, Colours.palette.m3primary.g, Colours.palette.m3primary.b, 0.12)
            border.width: 3
            border.color: Colours.palette.m3primary
        }
    }
}

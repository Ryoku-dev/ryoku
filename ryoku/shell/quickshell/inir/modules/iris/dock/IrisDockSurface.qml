pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import inir.modules.common

Scope {
    id: root

    required property var screen
    property bool surfaceVisible: true
    // The Stage Editor frames this monitor: the dock rises above the lifted
    // desktop and holds revealed (IrisDock publishes the edge it occupies).
    property bool stageEditing: false
    PanelWindow {
        id: surface

        screen: root.screen
        visible: root.surfaceVisible
        color: "transparent"
        anchors { left: true; right: true; top: true; bottom: true }
        exclusionMode: ExclusionMode.Ignore
        WlrLayershell.namespace: "ryoku-dock-shima"
        WlrLayershell.layer: root.stageEditing ? WlrLayer.Overlay : WlrLayer.Top
        WlrLayershell.keyboardFocus: dock.menuOpen ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
        mask: dock.menuOpen ? null : dockRegion

        Region {
            id: dockRegion
            item: dock.inputOff ? null : dock.hitItem
        }

        IrisDock {
            id: dock
            anchors.fill: parent
            screen: root.screen
            stageEditing: root.stageEditing
        }
    }

    PanelWindow {
        id: reservation

        screen: root.screen
        visible: root.surfaceVisible && dock.reserveSpace && !dock.autoHide
        color: "transparent"
        exclusionMode: ExclusionMode.Normal
        WlrLayershell.namespace: "ryoku-dock-shima-reserve"
        WlrLayershell.layer: WlrLayer.Bottom
        exclusiveZone: visible ? dock.thickness : 0
        mask: emptyRegion
        implicitHeight: dock.vertical ? 0 : dock.thickness
        implicitWidth: dock.vertical ? dock.thickness : 0
        anchors {
            left: dock.atLeft || !dock.vertical
            right: dock.atRight || !dock.vertical
            top: dock.atTop || dock.vertical
            bottom: dock.atBottom || dock.vertical
        }

        Region { id: emptyRegion }
    }
}

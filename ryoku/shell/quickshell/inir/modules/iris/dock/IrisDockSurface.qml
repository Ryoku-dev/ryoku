pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import inir.modules.common
import shell.modules.stage.frame
import shell.services as Ryoku

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
        // stand above a bar that holds the dock's edge; the Stage card frames the whole screen
        margins.bottom: root.stageEditing ? 0 : Ryoku.Dock.barInset(dock.edge)
        exclusionMode: ExclusionMode.Ignore
        WlrLayershell.namespace: "ryoku-dock-shima"
        WlrLayershell.layer: root.stageEditing ? WlrLayer.Overlay : WlrLayer.Top
        WlrLayershell.keyboardFocus: dock.menuOpen ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
        readonly property point dockInputOrigin: dock.hitItem
            ? dock.hitItem.mapToItem(frameLayer.rest, 0, 0) : Qt.point(0, 0)
        readonly property rect dockInputRect: frameLayer.maskRect(surface.dockInputOrigin.x,
            surface.dockInputOrigin.y, dock.hitItem?.width ?? 0, dock.hitItem?.height ?? 0)
        mask: dock.menuOpen ? null : dockRegion

        Region {
            id: dockRegion
            x: Math.round(surface.dockInputRect.x)
            y: Math.round(surface.dockInputRect.y)
            width: dock.inputOff ? 0 : Math.round(surface.dockInputRect.width)
            height: dock.inputOff ? 0 : Math.round(surface.dockInputRect.height)
        }

        // This window already spans the output; the layer moves its dock into
        // the Stage card without changing the reservation surface below.
        StageFramedLayer {
            id: frameLayer
            screenName: surface.screen?.name ?? ""
            edge: dock.edge
            restThickness: dock.vertical ? surface.width : surface.height
            screenWidth: surface.screen?.width ?? 1920
            screenHeight: surface.screen?.height ?? 1080

        IrisDock {
            id: dock
            anchors.fill: parent
            screen: root.screen
            stageEditing: root.stageEditing
        }
        }
    }

    PanelWindow {
        id: reservation

        screen: root.screen
        visible: root.surfaceVisible && dock.reserveSpace && !dock.autoHide
        color: "transparent"
        exclusionMode: ExclusionMode.Normal
        WlrLayershell.namespace: "ryoku-dock-shima-reserve"
        WlrLayershell.layer: Ryoku.Dock.yieldsToBar ? WlrLayer.Top : WlrLayer.Bottom
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

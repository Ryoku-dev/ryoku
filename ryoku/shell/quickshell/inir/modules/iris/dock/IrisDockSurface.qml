pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import inir.modules.common
import shell.modules.stage.frame
import stage as StageEditor

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
        // The Shima chassis paints this dock's fused body in its Top-layer
        // field. Keep the icon surface above that field regardless of which
        // one finishes loading first or whether the shell was just restarted.
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.keyboardFocus: dock.menuOpen ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
        // The dock body is visually reparented through the Stage layer. Mapping
        // it to `rest` collapses to (0, 0), so build the compositor mask from
        // its actual scene corners and keep it moving with the Stage card.
        readonly property rect dockInputRect: {
            const item = dock.hitItem
            if (!item) return Qt.rect(0, 0, 0, 0)
            void (item.x + item.y + item.width + item.height
                + (item.parent?.x ?? 0) + (item.parent?.y ?? 0)
                + frameLayer.x + frameLayer.y + frameLayer.width + frameLayer.height
                + StageEditor.GlobalStates.editProgress + StageEditor.GlobalStates.editDrawerProgress)
            const start = item.mapToItem(null, 0, 0)
            const end = item.mapToItem(null, item.width, item.height)
            return Qt.rect(Math.min(start.x, end.x), Math.min(start.y, end.y),
                Math.abs(end.x - start.x), Math.abs(end.y - start.y))
        }
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
            restThickness: dock.vertical ? screenWidth : screenHeight
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
        WlrLayershell.layer: WlrLayer.Bottom
        exclusiveZone: visible ? Math.max(0, dock.thickness + dock.extraEdgeGap) : 0
        mask: emptyRegion
        implicitHeight: dock.vertical ? 0 : Math.max(0, dock.thickness + dock.extraEdgeGap)
        implicitWidth: dock.vertical ? Math.max(0, dock.thickness + dock.extraEdgeGap) : 0
        anchors {
            left: dock.atLeft || !dock.vertical
            right: dock.atRight || !dock.vertical
            top: dock.atTop || dock.vertical
            bottom: dock.atBottom || dock.vertical
        }

        Region { id: emptyRegion }
    }
}

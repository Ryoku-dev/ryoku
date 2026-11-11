pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import inir
import inir.modules.common
import inir.services

Scope {
    id: root

    property bool everOpened: GlobalStates.overlayOpen
    property var targetScreen: GlobalStates.focusedScreen

    readonly property bool hasPersistentPins: {
        const state = Persistent.states?.overlay
        return Array.from(state?.open ?? []).some(identifier => state?.[identifier]?.pinned === true)
    }

    Connections {
        target: GlobalStates
        function onOverlayOpenChanged(): void {
            if (!GlobalStates.overlayOpen) return
            root.everOpened = true
            root.targetScreen = GlobalStates.focusedScreen
        }
    }

    Loader {
        active: root.everOpened || root.hasPersistentPins
        sourceComponent: PanelWindow {
            id: window
            visible: GlobalStates.overlayOpen || OverlayContext.hasPinnedWidgets || OverlayContext.nativeDialogOpen
            screen: root.targetScreen ?? GlobalStates.primaryScreen
            exclusionMode: ExclusionMode.Ignore
            WlrLayershell.namespace: "quickshell:iris-tools"
            WlrLayershell.layer: OverlayContext.nativeDialogOpen ? WlrLayer.Bottom : WlrLayer.Overlay
            WlrLayershell.keyboardFocus: OverlayContext.nativeDialogOpen ? WlrKeyboardFocus.None
                : GlobalStates.overlayOpen ? WlrKeyboardFocus.Exclusive
                : OverlayContext.clickableWidgets.length > 0 ? WlrKeyboardFocus.OnDemand : WlrKeyboardFocus.None
            color: "transparent"
            anchors { top: true; bottom: true; left: true; right: true }

            Item { id: emptyMask; width: 0; height: 0 }
            property var activeRegions: []
            function rebuildRegions(): void {
                for (const region of window.activeRegions) region.destroy()
                window.activeRegions = OverlayContext.clickableWidgets.map(widget => regionComponent.createObject(window, { item: widget }))
                clickableMask.regions = window.activeRegions
            }
            Component.onCompleted: window.rebuildRegions()
            Connections {
                target: OverlayContext
                function onClickableWidgetsChanged(): void { window.rebuildRegions() }
            }
            mask: Region {
                id: clickableMask
                item: GlobalStates.overlayOpen && !OverlayContext.nativeDialogOpen ? content : emptyMask
                regions: []
            }

            Shortcut {
                sequence: StandardKey.Cancel
                enabled: GlobalStates.overlayOpen
                onActivated: GlobalStates.overlayOpen = false
            }

            OverlayContent { id: content; anchors.fill: parent }
        }
    }

    property Component regionComponent: Component { Region {} }
}

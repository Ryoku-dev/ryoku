pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.FrameBars
import "framebars/widgets"
import shell.services

// Maps a catalogue id to its Rail* component and forwards the widget's intents
// (menu open, bar action) up to the frame. A self-hiding widget collapses the
// host to nothing so its zone leaves no gap.
Item {
    id: root

    property string widgetId: ""
    required property string edge
    required property real scale
    // space the zone leaves this widget's group (RailZone pushes the binding);
    // only the dock consumes it, to shrink on rails too short for every zone.
    property real zoneAvail: -1
    signal menuRequested(string id, rect ownerRect)
    signal actionRequested(string id)

    readonly property bool loaded: widgetLoader.item !== null
    // The item's own implicitWidth is already 0 when it self-hides (a widget
    // gates on its own condition, never on `visible`, which ancestors pull to
    // false while the bar is collapsed). Hiding a zero-width host lets the zone
    // skip it entirely, so a self-hidden widget leaves no gap.
    implicitWidth: widgetLoader.item ? widgetLoader.item.implicitWidth : 0
    implicitHeight: widgetLoader.item ? widgetLoader.item.implicitHeight : 0
    visible: widgetLoader.item ? widgetLoader.item.implicitWidth > 0 : false

    function componentFor(id) {
        switch (id) {
        case "clock": return clockComponent;
        case "workspaces": return workspacesComponent;
        case "tray": return trayComponent;
        case "quick-settings": return quickSettingsComponent;
        case "dock": return dockComponent;
        case "music": return musicComponent;
        case "sysmon": return sysmonComponent;
        case "vpn": return vpnComponent;
        case "audio-input":
        case "audio-output":
        case "battery":
        case "bluetooth":
        case "network":
        case "notifications": return statusComponent;
        case "lock":
        case "logout":
        case "shutdown": return actionComponent;
        default:
            if (BarCatalog.entry(id)) console.error("frame bars: no host component for " + id);
            return null;
        }
    }


    Loader {
        id: widgetLoader
        anchors.centerIn: parent
        sourceComponent: root.componentFor(root.widgetId)
    }
    Connections {
        target: widgetLoader.item
        ignoreUnknownSignals: true
        function onMenuRequested(id, ownerRect) {
            const topLeft = widgetLoader.item.mapToGlobal(ownerRect.x, ownerRect.y);
            root.menuRequested(id, { x: topLeft.x, y: topLeft.y, width: ownerRect.width, height: ownerRect.height });
        }
        function onActionRequested(id) { root.actionRequested(id); }
    }

    Component { id: clockComponent; RailClock { edge: root.edge; scale: root.scale } }
    Component { id: workspacesComponent; RailWorkspaces { edge: root.edge; scale: root.scale } }
    Component { id: trayComponent; RailTray { edge: root.edge; scale: root.scale } }
    Component { id: quickSettingsComponent; RailQuickSettings { edge: root.edge; scale: root.scale } }
    Component {
        id: dockComponent
        RailDock {
            edge: root.edge
            scale: root.scale
            clients: Dock.clients
            activeClass: Dock.activeClass
            maxExtent: root.zoneAvail
            onActivate: className => Dock.activate(className)
        }
    }
    Component { id: statusComponent; RailStatus { edge: root.edge; scale: root.scale; statusId: root.widgetId } }
    Component { id: musicComponent; RailMusic { edge: root.edge; scale: root.scale } }
    Component { id: sysmonComponent; RailSysmon { edge: root.edge; scale: root.scale } }
    Component { id: vpnComponent; RailVpn { edge: root.edge; scale: root.scale; active: true } }
    Component { id: actionComponent; RailAction { edge: root.edge; scale: root.scale; actionId: root.widgetId } }
}

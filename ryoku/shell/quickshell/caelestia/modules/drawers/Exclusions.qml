pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.components.containers
import caelestia.services
import caelestia.modules.bar as Bar

Scope {
    id: root

    required property ShellScreen screen
    required property Bar.BarWrapper bar
    required property DrawerVisibilities visibilities
    // A pinned sidebar reserves its width so maximised windows sit beside it
    // rather than under it. It stays reserved while hidden for a fullscreen window
    // so the other windows are not resized back and forth.
    readonly property bool reserveSidebar: Visibilities.sidebarPinned && Config.sidebar.enabled
        && (visibilities.sidebar || visibilities.sidebarSuspended)
    // Follows the sidebar width, but only once a resize drag ends: reserving every
    // step of the drag would resize maximised windows on each frame. The default
    // comes from the bar, which has the screen's tokens.
    property real sidebarWidth

    Binding on sidebarWidth {
        value: Visibilities.sidebarWidthFor(root.bar.Tokens.sizes.sidebar.width)
        when: !Visibilities.sidebarResizing
        restoreMode: Binding.RestoreNone
    }

    ExclusionZone {
        anchors.left: true
        exclusiveZone: (root.bar.position === "left" ? root.bar.exclusiveZone : Config.border.thickness)
            + (root.reserveSidebar && root.bar.position === "right" ? root.sidebarWidth : 0)
        Config.screen: root.screen.name
    }

    ExclusionZone {
        anchors.top: true
        exclusiveZone: root.bar.position === "top" ? root.bar.exclusiveZone : Config.border.thickness
        Config.screen: root.screen.name
    }

    ExclusionZone {
        anchors.right: true
        exclusiveZone: (root.bar.position === "right" ? root.bar.exclusiveZone : Config.border.thickness)
            + (root.reserveSidebar && root.bar.position !== "right" ? root.sidebarWidth : 0)
        Config.screen: root.screen.name
    }

    ExclusionZone {
        anchors.bottom: true
        exclusiveZone: root.bar.position === "bottom" ? root.bar.exclusiveZone : Config.border.thickness
        Config.screen: root.screen.name
    }

    component ExclusionZone: StyledWindow {
        screen: root.screen
        name: "border-exclusion"
        exclusiveZone: Config.border.thickness
        mask: Region {}
        implicitWidth: 1
        implicitHeight: 1
        Config.screen: root.screen.name
    }
}

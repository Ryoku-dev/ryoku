pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.modules.bar as Bar

Region {
    id: root

    required property Bar.BarWrapper bar
    required property Panels panels
    required property var win
    readonly property real borderThickness: Config.border.thickness
    readonly property real clampedThickness: Config.border.clampedThickness
    readonly property real barLeftWidth: bar.position === "left" ? bar.clampedThickness : clampedThickness
    readonly property real barRightWidth: bar.position === "right" ? bar.clampedThickness : clampedThickness
    readonly property real barTopHeight: bar.position === "top" ? bar.clampedThickness : clampedThickness
    readonly property real barBottomHeight: bar.position === "bottom" ? bar.clampedThickness : clampedThickness
    // A closed panel's input region must be at least as deep as its hover trigger,
    // otherwise the pointer never reaches the area Interactions tests against.

    function edgeExtent(hoverThickness: real): real {
        return Math.max(borderThickness, hoverThickness);
    }

    x: barLeftWidth + win.dragMaskPadding
    y: barTopHeight + win.dragMaskPadding
    width: win.width - barLeftWidth - barRightWidth - win.dragMaskPadding * 2
    height: win.height - barTopHeight - barBottomHeight - win.dragMaskPadding * 2
    intersection: Intersection.Xor

    R {
        panel: root.panels.dashboard
        y: 0
        height: panel.height * (1 - root.panels.dashboard.offsetScale) + root.edgeExtent(root.Config.dashboard.hoverThickness)
    }
    R {
        panel: root.panels.launcher
        y: root.win.height - height
        height: panel.height * (1 - root.panels.launcher.offsetScale) + root.edgeExtent(root.Config.launcher.hoverThickness)
    }
    R {
        id: sessionRegion

        panel: root.panels.sessionWrapper
        x: root.bar.position === "right" ? 0 : root.win.width - sessionRegion.width
        width: panel.width * (1 - root.panels.session.offsetScale) + root.borderThickness + sidebarRegion.width
    }
    R {
        id: sidebarRegion

        panel: root.panels.sidebar
        x: root.bar.position === "right" ? 0 : root.win.width - sidebarRegion.width
        width: panel.width * (1 - root.panels.sidebar.offsetScale) + root.borderThickness
    }
    R {
        id: osdRegion

        panel: root.panels.osdWrapper
        x: root.bar.position === "right" ? 0 : root.win.width - osdRegion.width
        width: panel.width * (1 - root.panels.osd.offsetScale) + root.edgeExtent(root.Config.osd.hoverThickness) + sessionRegion.width
    }
    R {
        panel: root.panels.notifications
    }
    R {
        panel: root.panels.utilities
        y: root.bar.position === "bottom" ? 0 : root.win.height - height
        height: panel.height * (1 - root.panels.utilities.offsetScale) + root.edgeExtent(root.Config.utilities.hoverThickness)
    }
    R {
        panel: root.panels.popoutsWrapper
        width: panel.width * (1 - root.panels.popoutsWrapper.offsetScale)
    }
    Region {
        x: 0; y: 0
        width: (root.Config.overview.enabled && root.Config.overview.hoverTopLeft) ? root.Config.overview.hoverThickness : 0
        height: root.Config.overview.hoverThickness
        intersection: Intersection.Subtract
    }
    Region {
        x: root.win.width - root.Config.overview.hoverThickness; y: 0
        width: (root.Config.overview.enabled && root.Config.overview.hoverTopRight) ? root.Config.overview.hoverThickness : 0
        height: root.Config.overview.hoverThickness
        intersection: Intersection.Subtract
    }
    Region {
        x: 0; y: root.win.height - root.Config.overview.hoverThickness
        width: (root.Config.overview.enabled && root.Config.overview.hoverBottomLeft) ? root.Config.overview.hoverThickness : 0
        height: root.Config.overview.hoverThickness
        intersection: Intersection.Subtract
    }
    Region {
        x: root.win.width - root.Config.overview.hoverThickness; y: root.win.height - root.Config.overview.hoverThickness
        width: (root.Config.overview.enabled && root.Config.overview.hoverBottomRight) ? root.Config.overview.hoverThickness : 0
        height: root.Config.overview.hoverThickness
        intersection: Intersection.Subtract
    }

    component R: Region {
        required property Item panel

        x: panel.x + root.panels.leftMargin
        y: panel.y + root.panels.topMargin
        width: panel.width
        height: panel.height
        intersection: Intersection.Subtract
    }
    Config.screen: win.screen.name
}

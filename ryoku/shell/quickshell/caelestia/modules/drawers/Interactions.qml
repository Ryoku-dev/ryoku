import QtQuick
import QtQuick.Controls
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.modules.bar as Bar
import caelestia.modules.bar.popouts as BarPopouts

CustomMouseArea {
    id: root

    required property ShellScreen screen
    required property BarPopouts.Wrapper popouts
    required property DrawerVisibilities visibilities
    required property Panels panels
    required property Bar.BarWrapper bar
    required property real borderThickness
    required property bool fullscreen
    property var focusGrab: null
    property point dragStart
    property bool dashboardShortcutActive
    property bool osdShortcutActive
    property bool utilitiesShortcutActive
    readonly property bool isBarHorizontal: bar.isHorizontal
    // Uses clampedThickness rather than the raw implicit size so the bar stays
    // reachable when the border thickness is set to 0 (the collapsed bar is 0px wide).

    function inBarArea(x: real, y: real): bool {
        if (bar.position === "left")
            return x < bar.x + bar.clampedThickness;
        if (bar.position === "right")
            return x > bar.x + bar.width - bar.clampedThickness;
        if (bar.position === "top")
            return y < bar.y + bar.clampedThickness;
        if (bar.position === "bottom")
            return y > bar.y + bar.height - bar.clampedThickness;
        return false;
    }
    function spanBounds(near: real, far: real, span: real): point {
        if (span >= 100)
            return Qt.point(near, far);
        const half = (far - near) * Math.max(0, span) / 200;
        const centre = (near + far) / 2;
        return Qt.point(centre - half, centre + half);
    }
    function withinPanelHeight(panel: Item, x: real, y: real, span = 100): bool {
        const panelY = panels.topMargin + panel.y;
        const panelHeight = panel.content ? panel.content.nonAnimHeight : panel.height;
        const b = spanBounds(panelY - Config.border.rounding - panels.topMargin, panelY + panelHeight + Config.border.rounding + panels.bottomMargin, span);
        return y >= b.x && y <= b.y;
    }
    function withinPanelWidth(panel: Item, x: real, y: real, span = 100): bool {
        const panelX = panels.leftMargin + panel.x;
        const panelWidth = panel.content ? panel.content.nonAnimWidth : panel.width;
        const b = spanBounds(panelX - Config.border.rounding - panels.leftMargin, panelX + panelWidth + Config.border.rounding + panels.rightMargin, span);
        return x >= b.x && x <= b.y;
    }
    function inLeftPanel(panel: Item, x: real, y: real): bool {
        const panelWidth = panel.content ? panel.content.nonAnimWidth : panel.width;
        const panelHeight = panel.content ? panel.content.nonAnimHeight : panel.height;

        if (bar.position === "left")
            return x < panels.leftMargin + panel.x + panelWidth && withinPanelHeight(panel, x, y);
        if (bar.position === "right")
            return x > screen.width - panels.rightMargin - panelWidth && withinPanelHeight(panel, x, y);
        if (bar.position === "top")
            return y < panels.topMargin + panel.y + panelHeight && (withinPanelWidth(panel, x, y) || abovePopoutItem(panel, x));
        if (bar.position === "bottom")
            return y > screen.height - panels.bottomMargin - panelHeight && (withinPanelWidth(panel, x, y) || abovePopoutItem(panel, x));
        return false;
    }
    // Next to the open sidebar a popout doesn't always cover the bar item that
    // opened it; keep the strip straight above the item so it can be reached.
    function abovePopoutItem(panel: Item, x: real): bool {
        return panel === panels.popoutsWrapper && !popouts.isDetached && Math.abs(x - popouts.currentCenter) <= Config.border.rounding;
    }
    function inRightPanel(panel: Item, x: real, y: real, edge = 0, span = 100): bool {
        const onLeft = bar.position === "right";
        const strip = Math.max(Config.border.minThickness, edge || (onLeft ? panels.leftMargin : panels.rightMargin));
        const closed = (panel.offsetScale ?? 0) >= 1 || panel.width <= 0; // qmllint disable missing-property

        if (onLeft) {
            const shown = panels.leftMargin + panel.x + panel.width;
            return x < (closed ? strip : Math.max(strip, shown)) && withinPanelHeight(panel, x, y, closed ? span : 100);
        }

        const shown = screen.width - (panels.leftMargin + panel.x);
        return x > screen.width - (closed ? strip : Math.max(strip, shown)) && withinPanelHeight(panel, x, y, closed ? span : 100);
    }
    function inTopPanel(panel: Item, x: real, y: real, edge = Config.border.thickness, span = 100): bool {
        const panelHeight = panel.height * (1 - (panel.offsetScale ?? 0)); // qmllint disable missing-property
        return y < Math.max(Config.border.minThickness, edge + panelHeight) && withinPanelWidth(panel, x, y, panelHeight > 0 ? 100 : span);
    }
    function inBottomPanel(panel: Item, x: real, y: real, isCorner = false, edge = Config.border.thickness, span = 100): bool {
        const panelHeight = panel.height * (1 - (panel.offsetScale ?? 0)); // qmllint disable missing-property
        return y > screen.height - Math.max(Config.border.minThickness, edge + panelHeight) - (isCorner ? Config.border.rounding : 0) && withinPanelWidth(panel, x, y, panelHeight > 0 ? 100 : span);
    }
    function inOverviewCorner(x: real, y: real): string {
        const thickness = Config.overview.hoverThickness;
        if (Config.overview.hoverTopLeft && x <= thickness && y <= thickness) return "TopLeft";
        if (Config.overview.hoverTopRight && x >= screen.width - thickness && y <= thickness) return "TopRight";
        if (Config.overview.hoverBottomLeft && x <= thickness && y >= screen.height - thickness) return "BottomLeft";
        if (Config.overview.hoverBottomRight && x >= screen.width - thickness && y >= screen.height - thickness) return "BottomRight";
        return "";
    }
    function onWheel(event: WheelEvent): void {
        if (fullscreen)
            return;
        if (inBarArea(event.x, event.y)) {
            bar.handleWheel(isBarHorizontal ? event.x : event.y, event.angleDelta);
        }
    }

    anchors.fill: parent
    acceptedButtons: (fullscreen && !(root.focusGrab && (root.focusGrab.active || popouts.isDetached))) ? Qt.NoButton : Qt.AllButtons
    hoverEnabled: true
    onPressed: event => {
        dragStart = Qt.point(event.x, event.y);

        if (root.focusGrab && (root.focusGrab.active || popouts.isDetached)) {
            let inside = false;
            
            if (inBarArea(event.x, event.y)) inside = true;
            else if (visibilities.launcher && inBottomPanel(panels.launcher, event.x, event.y, false, Config.launcher.hoverThickness, Config.launcher.hoverWidth) && withinPanelWidth(panels.launcher, event.x, event.y)) inside = true;
            else if (visibilities.session && inRightPanel(panels.sessionWrapper, event.x, event.y)) inside = true;
            else if (visibilities.sidebar && inRightPanel(panels.sidebar, event.x, event.y)) inside = true;
            else if (visibilities.dashboard && inTopPanel(panels.dashboard, event.x, event.y, Config.dashboard.hoverThickness, Config.dashboard.hoverWidth) && withinPanelWidth(panels.dashboard, event.x, event.y)) inside = true;
            else if (visibilities.utilities && (bar.position === "bottom" ? inTopPanel(panels.utilities, event.x, event.y, Config.utilities.hoverThickness, Config.utilities.hoverWidth) : inBottomPanel(panels.utilities, event.x, event.y, true, Config.utilities.hoverThickness, Config.utilities.hoverWidth)) && withinPanelWidth(panels.utilities, event.x, event.y)) inside = true;
            else if (popouts.hasCurrent && inLeftPanel(panels.popoutsWrapper, event.x, event.y)) inside = true;

            if (!inside) {
                root.focusGrab.clear();
            }
        }
    }
    onContainsMouseChanged: {
        if (!containsMouse) {
            if (!osdShortcutActive) {
                visibilities.osd = false;
                root.panels.osd.hovered = false;
            }

            if (Config.dashboard.showOnHover && !dashboardShortcutActive)
                visibilities.dashboard = false;

            if (Config.utilities.showOnHover && !utilitiesShortcutActive)
                visibilities.utilities = false;

            if (!popoutHideTimer.running)
                popoutHideTimer.start();

            if (Config.bar.showOnHover)
                bar.isHovered = false;
        } else {
            popoutHideTimer.stop();
        }
    }
    onPositionChanged: event => {
        if (popouts.isDetached)
            return;

        const x = event.x;
        const y = event.y;
        const dragX = x - dragStart.x;
        const dragY = y - dragStart.y;

        if (fullscreen) {
            root.panels.osd.hovered = inRightPanel(panels.osdWrapper, x, y, Config.osd.hoverThickness, Config.osd.hoverWidth);
            return;
        }

        if (!visibilities.bar && Config.bar.showOnHover && inBarArea(x, y))
            bar.isHovered = true;

        if (pressed && inBarArea(dragStart.x, dragStart.y)) {
            if (bar.position === "left") {
                if (dragX > Config.bar.dragThreshold)
                    visibilities.bar = true;
                else if (dragX < -Config.bar.dragThreshold)
                    visibilities.bar = false;
            } else if (bar.position === "right") {
                if (dragX < -Config.bar.dragThreshold)
                    visibilities.bar = true;
                else if (dragX > Config.bar.dragThreshold)
                    visibilities.bar = false;
            } else if (bar.position === "top") {
                if (dragY > Config.bar.dragThreshold)
                    visibilities.bar = true;
                else if (dragY < -Config.bar.dragThreshold)
                    visibilities.bar = false;
            } else if (bar.position === "bottom") {
                if (dragY < -Config.bar.dragThreshold)
                    visibilities.bar = true;
                else if (dragY > Config.bar.dragThreshold)
                    visibilities.bar = false;
            }
        }

        if (panels.sidebar.offsetScale === 1) {
            const showOsd = inRightPanel(panels.osdWrapper, x, y, Config.osd.hoverThickness, Config.osd.hoverWidth);

            if (!osdShortcutActive) {
                visibilities.osd = showOsd;
                root.panels.osd.hovered = showOsd;
            } else if (showOsd) {
                osdShortcutActive = false;
                root.panels.osd.hovered = true;
            }

            const showSidebar = bar.position === "right" ? pressed && dragStart.x < Math.max(Config.border.minThickness, panels.leftMargin + panels.sidebar.x + panels.sidebar.width) + Config.sidebar.grabWidth : pressed && dragStart.x > Math.min(screen.width - Config.border.minThickness, panels.leftMargin + panels.sidebar.x) - Config.sidebar.grabWidth;

            if (pressed && inRightPanel(panels.sessionWrapper, dragStart.x, dragStart.y) && withinPanelHeight(panels.sessionWrapper, x, y)) {
                const showThreshold = bar.position === "right" ? Config.session.dragThreshold : -Config.session.dragThreshold;
                const hideThreshold = bar.position === "right" ? -Config.session.dragThreshold : Config.session.dragThreshold;

                if (bar.position === "right" ? dragX > showThreshold : dragX < showThreshold)
                    visibilities.session = true;
                else if (bar.position === "right" ? dragX < hideThreshold : dragX > hideThreshold)
                    visibilities.session = false;

                const showSidebarThreshold = bar.position === "right" ? Config.sidebar.dragThreshold : -Config.sidebar.dragThreshold;
                if (showSidebar && panels.session.offsetScale <= 0 && (bar.position === "right" ? dragX > showSidebarThreshold : dragX < showSidebarThreshold))
                    visibilities.sidebar = true;
            } else if (showSidebar && (bar.position === "right" ? dragX > Config.sidebar.dragThreshold : dragX < -Config.sidebar.dragThreshold)) {
                visibilities.sidebar = true;
            }
        } else {
            const outOfSidebar = bar.position === "right" ? x > panels.leftMargin + panels.sidebar.width * (1 - panels.sidebar.offsetScale) : x < screen.width - panels.sidebar.width * (1 - panels.sidebar.offsetScale);
            const showOsd = outOfSidebar && inRightPanel(panels.osdWrapper, x, y, Config.osd.hoverThickness, Config.osd.hoverWidth);

            if (!osdShortcutActive) {
                visibilities.osd = showOsd;
                root.panels.osd.hovered = showOsd;
            } else if (showOsd) {
                osdShortcutActive = false;
                root.panels.osd.hovered = true;
            }

            if (pressed && outOfSidebar && inRightPanel(panels.sessionWrapper, dragStart.x, dragStart.y) && withinPanelHeight(panels.sessionWrapper, x, y)) {
                const showThreshold = bar.position === "right" ? Config.session.dragThreshold : -Config.session.dragThreshold;
                const hideThreshold = bar.position === "right" ? -Config.session.dragThreshold : Config.session.dragThreshold;

                if (bar.position === "right" ? dragX > showThreshold : dragX < showThreshold)
                    visibilities.session = true;
                else if (bar.position === "right" ? dragX < hideThreshold : dragX > hideThreshold)
                    visibilities.session = false;
            }

            if (pressed && inRightPanel(panels.sidebar, dragStart.x, 0) && (bar.position === "right" ? dragX < -Config.sidebar.dragThreshold : dragX > Config.sidebar.dragThreshold))
                visibilities.sidebar = false;
        }

        if (Config.launcher.showOnHover) {
            if (!visibilities.launcher && inBottomPanel(panels.launcher, x, y, false, Config.launcher.hoverThickness, Config.launcher.hoverWidth))
                visibilities.launcher = true;
        } else if (pressed && inBottomPanel(panels.launcher, dragStart.x, dragStart.y, false, Config.launcher.hoverThickness, Config.launcher.hoverWidth) && withinPanelWidth(panels.launcher, x, y)) {
            if (dragY < -Config.launcher.dragThreshold)
                visibilities.launcher = true;
            else if (dragY > Config.launcher.dragThreshold)
                visibilities.launcher = false;
        }

        const showDashboard = Config.dashboard.showOnHover && inTopPanel(panels.dashboard, x, y, Config.dashboard.hoverThickness, Config.dashboard.hoverWidth);

        if (Config.dashboard.showOnHover) {
            if (!dashboardShortcutActive) {
                visibilities.dashboard = showDashboard;
            } else if (showDashboard) {
                dashboardShortcutActive = false;
            }
        }

        if (pressed && inTopPanel(panels.dashboard, dragStart.x, dragStart.y, Config.dashboard.hoverThickness, Config.dashboard.hoverWidth) && withinPanelWidth(panels.dashboard, x, y)) {
            if (dragY > Config.dashboard.dragThreshold)
                visibilities.dashboard = true;
            else if (dragY < -Config.dashboard.dragThreshold)
                visibilities.dashboard = false;
        }

        if (inBarArea(x, y)) {
            bar.checkPopout(isBarHorizontal ? x : y);
            popoutHideTimer.stop();
        } else {
            bar.resetHover();
            if ((!popouts.currentName.startsWith("traymenu") || (Config.bar.popouts.tray && ((popouts.current as StackView)?.depth ?? 0) <= 1)) && !inLeftPanel(panels.popoutsWrapper, x, y)) {
                if (!popoutHideTimer.running) popoutHideTimer.start();
            } else {
                popoutHideTimer.stop();
            }
        }

        const isUtilitiesOnLeft = bar.position === "right";
        const inUtilitiesAreaClosed = isUtilitiesOnLeft ? x <= (screen.width / 2) : (x >= (screen.width / 2) && (bar.position === "bottom" ? x <= (screen.width - 200) : true));
        const inUtilitiesAreaOpen = x >= 0 && x <= screen.width;
        
        const inUtilitiesArea = bar.position === "bottom"
            ? inTopPanel(panels.utilities, x, y, Config.utilities.hoverThickness, Config.utilities.hoverWidth) && (root.visibilities.utilities ? inUtilitiesAreaOpen : inUtilitiesAreaClosed)
            : inBottomPanel(panels.utilities, x, y, true, Config.utilities.hoverThickness, Config.utilities.hoverWidth) && (root.visibilities.utilities ? inUtilitiesAreaOpen : inUtilitiesAreaClosed);
        const showUtilities = Config.utilities.showOnHover && !popouts.hasCurrent && panels.popoutsWrapper.offsetScale > 0.99 && inUtilitiesArea;

        if (Config.utilities.showOnHover) {
            if (!utilitiesShortcutActive) {
                visibilities.utilities = showUtilities;
            } else if (showUtilities) {
                utilitiesShortcutActive = false;
            }
        }

        if (pressed) {
            const inUtilitiesDragStart = bar.position === "bottom"
                ? inTopPanel(panels.utilities, dragStart.x, dragStart.y, Config.utilities.hoverThickness, Config.utilities.hoverWidth)
                : inBottomPanel(panels.utilities, dragStart.x, dragStart.y, true, Config.utilities.hoverThickness, Config.utilities.hoverWidth);

            if (inUtilitiesDragStart && (bar.position === "bottom" ? withinPanelWidth(panels.utilities, x, y) : withinPanelWidth(panels.utilities, x, y))) {
                if (bar.position === "bottom") {
                    if (dragY > Config.utilities.dragThreshold)
                        visibilities.utilities = true;
                    else if (dragY < -Config.utilities.dragThreshold)
                        visibilities.utilities = false;
                } else {
                    if (dragY < -Config.utilities.dragThreshold)
                        visibilities.utilities = true;
                    else if (dragY > Config.utilities.dragThreshold)
                        visibilities.utilities = false;
                }
            }
        }

        if (utilitiesShortcutActive) {
            const inUtilitiesAreaOpen = x >= 0 && x <= screen.width;
            const stillInUtilitiesArea = bar.position === "bottom" ? inTopPanel(panels.utilities, x, y, Config.utilities.hoverThickness, Config.utilities.hoverWidth) && inUtilitiesAreaOpen : inBottomPanel(panels.utilities, x, y, true, Config.utilities.hoverThickness, Config.utilities.hoverWidth) && inUtilitiesAreaOpen;
            if (!stillInUtilitiesArea) {
                utilitiesShortcutActive = false;
            }
        }

        if (Config.overview.enabled && !visibilities.overview) {
            if (Config.overview.showOnHover) {
                if (inOverviewCorner(x, y) !== "") {
                    Visibilities.setOverview(true);
                }
            } else if (pressed) {
                if (inOverviewCorner(dragStart.x, dragStart.y) !== "") {
                    if (Math.hypot(dragX, dragY) > Config.overview.dragThreshold) {
                        Visibilities.setOverview(true);
                    }
                }
            }
        }
    }
    Timer {
        id: popoutHideTimer

        interval: 150
        onTriggered: {
            if (!popouts.currentName.startsWith("traymenu") || ((popouts.current as StackView)?.depth ?? 0) <= 1) {
                popouts.hasCurrent = false;
                bar.closeTray();
            }
        }
    }
    Connections {
        function onLauncherChanged() {
            if (!root.visibilities.launcher) {
                root.dashboardShortcutActive = false;
                root.osdShortcutActive = false;
                root.utilitiesShortcutActive = false;

                const inDashboardArea = root.inTopPanel(root.panels.dashboard, root.mouseX, root.mouseY, Config.dashboard.hoverThickness, Config.dashboard.hoverWidth);
                const inOsdArea = root.inRightPanel(root.panels.osdWrapper, root.mouseX, root.mouseY, Config.osd.hoverThickness, Config.osd.hoverWidth);

                if (!inDashboardArea) {
                    root.visibilities.dashboard = false;
                }
                if (!inOsdArea) {
                    root.visibilities.osd = false;
                    root.panels.osd.hovered = false;
                }
            }
        }
        function onDashboardChanged() {
            if (root.visibilities.dashboard) {
                const inDashboardArea = root.inTopPanel(root.panels.dashboard, root.mouseX, root.mouseY, Config.dashboard.hoverThickness, Config.dashboard.hoverWidth);
                if (!inDashboardArea) {
                    root.dashboardShortcutActive = true;
                }
            } else {
                root.dashboardShortcutActive = false;
            }
        }
        function onOsdChanged() {
            if (root.visibilities.osd) {
                const inOsdArea = root.inRightPanel(root.panels.osdWrapper, root.mouseX, root.mouseY, Config.osd.hoverThickness, Config.osd.hoverWidth);
                if (!inOsdArea) {
                    root.osdShortcutActive = true;
                }
            } else {
                root.osdShortcutActive = false;
            }
        }
        function onUtilitiesChanged() {
            if (root.visibilities.utilities) {
                const margin = (root.visibilities.utilities || root.bar.position !== "bottom") ? 0 : 200;
                const inUtilitiesArea = root.bar.position === "bottom" ? root.inTopPanel(root.panels.utilities, root.mouseX, root.mouseY, Config.utilities.hoverThickness, Config.utilities.hoverWidth) && root.mouseX >= margin && root.mouseX <= screen.width - margin : root.inBottomPanel(root.panels.utilities, root.mouseX, root.mouseY, true, Config.utilities.hoverThickness, Config.utilities.hoverWidth) && root.mouseX >= margin && root.mouseX <= screen.width - margin;
                if (!inUtilitiesArea) {
                    root.utilitiesShortcutActive = true;
                }
            } else {
                root.utilitiesShortcutActive = false;
            }
        }

        target: root.visibilities
    }
    Config.screen: screen.name
}

// Kairos: the system tray, folded into the island's own hover face instead of
// a separate window. A caret rides in the sliver already left under the date
// wheel; a click grows the pill by exactly the row (or menu) beneath it, so
// the tray reads as one more thing the island opens into, not a bolt-on panel.
// Right-click on an icon swaps the row for that app's own dbusmenu tree, read
// live off the tray daemon -- still inside the same pill.

pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import shell.barkit as Pill
import Ryoku.Ui.Singletons

Item {
    id: flyout

    property color ink: "white"
    property color accent: "white"
    // Whether the island is currently in a state that can show the tray at
    // all (hovered, not suspended, not mid-quick-settings). Losing this closes
    // whatever the tray had open, so it never lingers behind a collapsed pill.
    property bool hostActive: false
    onHostActiveChanged: if (!flyout.hostActive) { flyout.open = false; flyout.menuService = ""; }

    function dim(a) { return Qt.rgba(flyout.ink.r, flyout.ink.g, flyout.ink.b, a); }

    readonly property int itemCount: Tray.items.length
    property bool open: false
    property string menuService: ""
    property var expanded: ({})

    readonly property var menuItem: {
        if (flyout.menuService === "") return null;
        var its = Tray.items;
        for (var i = 0; i < its.length; i++)
            if (its[i] && its[i].service === flyout.menuService) return its[i];
        return null;
    }
    readonly property var menuTree: flyout.menuItem ? flyout.menuItem.menu : null
    // The app quit or dropped its menu while the row was open in its place.
    onMenuTreeChanged: if (flyout.menuService !== "" && !flyout.menuTree) flyout.menuService = "";
    // No items left at all (last one quit) -- fold back down on its own.
    onItemCountChanged: if (flyout.itemCount === 0) { flyout.open = false; flyout.menuService = ""; }

    function displayName(item) {
        if (!item) return I18n.tr("Tray App");
        var title = String(item.title || "").trim();
        if (title !== "") return title;
        var tip = String((item.tooltip && item.tooltip.title) || "").trim();
        if (tip !== "") return tip;
        var fallback = String(item.id || "").trim();
        var slash = fallback.lastIndexOf("/");
        if (slash >= 0 && slash < fallback.length - 1) fallback = fallback.substring(slash + 1);
        fallback = fallback.replace(/^org\.(kde|ayatana|freedesktop)\./i, "").replace(/[_-]+/g, " ");
        return fallback !== "" ? fallback : I18n.tr("Tray App");
    }
    function cleanLabel(s) {
        if (!s) return "";
        return String(s).replace(/_([^_])/g, "$1").replace(/__/g, "_");
    }
    function iconSource(item) {
        if (!item) return "";
        if (item.iconPath) return "file://" + item.iconPath;
        if (item.iconName) return Icons.path(item.iconName, true);
        return "";
    }

    function openMenu(service) {
        flyout.expanded = ({});
        flyout.menuService = service;
        Tray.aboutToShow(service);
    }
    function closeMenu() { flyout.menuService = ""; }
    function toggleExpand(id) {
        var e = {};
        for (var k in flyout.expanded) e[k] = flyout.expanded[k];
        if (e[id]) delete e[id]; else e[id] = true;
        flyout.expanded = e;
    }
    function activate(id) {
        Tray.menuEvent(flyout.menuService, id);
        flyout.closeMenu();
    }

    function rowsFor(node, depth, out) {
        var kids = (node && node.children) ? node.children : [];
        for (var i = 0; i < kids.length; i++) {
            var c = kids[i];
            if (!c || c.visible === false) continue;
            var sep = c.type === "separator";
            var hasKids = !sep && Array.isArray(c.children) && c.children.length > 0;
            out.push({ node: c, depth: depth, sep: sep, hasKids: hasKids });
            if (hasKids && flyout.expanded[c.id]) flyout.rowsFor(c, depth + 1, out);
        }
        return out;
    }
    readonly property var menuRows: flyout.menuTree ? flyout.rowsFor(flyout.menuTree, 0, []) : []

    // ── sizing ───────────────────────────────────────────────────────────────
    // caretBand is the sliver already reserved beneath the date wheel at rest
    // (see Island's hoverHeight/reach): the caret alone must fit inside it so
    // the pill never grows just to show that a tray exists.
    readonly property real caretBand: 14
    readonly property real rowH: 26
    readonly property real topPad: 6
    readonly property real bottomPad: 8
    readonly property real headerH: 22
    readonly property real headerGap: 4
    readonly property int maxVisibleRows: 6

    function menuRowsHeight(rows) {
        var h = 0;
        for (var i = 0; i < rows.length; i++)
            h += rows[i].sep ? 7 : flyout.rowH;
        return h;
    }

    readonly property real menuContentHeight: flyout.menuRowsHeight(flyout.menuRows)
    readonly property real menuViewportHeight: Math.min(
        flyout.menuContentHeight, flyout.maxVisibleRows * flyout.rowH
    )
    readonly property real listExtra: flyout.itemCount > 0
        ? flyout.topPad + flyout.rowH + flyout.bottomPad : 0
    readonly property real menuExtra: flyout.menuService !== ""
        ? flyout.topPad + flyout.headerH + flyout.headerGap
            + flyout.menuViewportHeight + flyout.bottomPad
        : 0
    // What Island must grow the pill by, under the caret band, for the current
    // state. Capped so callers can reserve a fixed maximum once (see
    // Island.trayMaxExtra) instead of resizing the surface live.
    readonly property real extraHeight: flyout.menuService !== "" ? flyout.menuExtra
        : (flyout.open ? flyout.listExtra : 0)
    // The tallest this flyout can ever add, derived from the same constants so
    // the island's fixed reserve cannot drift from it.
    readonly property real maxExtra: flyout.topPad + flyout.headerH + flyout.headerGap
        + flyout.maxVisibleRows * flyout.rowH + flyout.bottomPad

    implicitHeight: flyout.caretBand + flyout.extraHeight

    // ── the caret ────────────────────────────────────────────────────────────
    Item {
        id: caret
        anchors.horizontalCenter: parent.horizontalCenter
        y: 0
        width: 32
        height: flyout.caretBand
        // Island owns the instance's `visible` (hover gating), so an empty tray
        // is folded from the inside: no caret until something is in the tray.
        visible: flyout.itemCount > 0

        Pill.MaterialIcon {
            anchors.centerIn: parent
            text: "expand_more"
            color: "white"
            opacity: caretHover.hovered ? 1 : 0.8
            font.pixelSize: 14
            rotation: (flyout.open || flyout.menuService !== "") ? 180 : 0
            Behavior on rotation {
                enabled: !Motion.reduce
                NumberAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
            }
        }

        HoverHandler { id: caretHover }
        MouseArea {
            anchors.fill: parent
            anchors.margins: -4
            cursorShape: Qt.PointingHandCursor
            onClicked: {
                if (flyout.menuService !== "") flyout.closeMenu();
                else flyout.open = !flyout.open;
            }
        }
    }
    // ── the icon row ─────────────────────────────────────────────────────────
    // Bounded to the pill and scrollable: a dozen tray apps would otherwise
    // overflow the clipping island and the edges would be unreachable. The row
    // centres itself while it fits and drags when it does not.
    Flickable {
        id: iconStrip
        anchors.horizontalCenter: parent.horizontalCenter
        y: flyout.caretBand + flyout.topPad
        width: Math.max(0, flyout.width - 24)
        height: flyout.rowH
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        visible: flyout.open && flyout.menuService === ""
        opacity: visible ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Motion.fast } }

        contentWidth: Math.max(width, iconRow.implicitWidth)
        contentHeight: height

        Row {
            id: iconRow
            x: Math.max(0, (iconStrip.width - implicitWidth) / 2)
            height: flyout.rowH
            spacing: 12
            Repeater {
                model: Tray.items

                delegate: Item {
                    id: trayIcon
                    required property var modelData
                    width: 22
                    height: 22
                    anchors.verticalCenter: parent.verticalCenter

                    Image {
                        anchors.centerIn: parent
                        source: flyout.iconSource(trayIcon.modelData)
                        sourceSize.width: 16
                        sourceSize.height: 16
                        width: 16
                        height: 16
                        fillMode: Image.PreserveAspectFit
                        smooth: true
                    }

                    Rectangle {
                        visible: trayIcon.modelData.status === "NeedsAttention"
                        width: 6; height: 6; radius: 3
                        color: flyout.accent
                        anchors { right: parent.right; top: parent.top; margins: -1 }
                    }

                    HoverHandler { id: iconHover }
                    Rectangle {
                        anchors.centerIn: parent
                        width: 28; height: 28; radius: 8
                        visible: iconHover.hovered
                        color: flyout.dim(0.10)
                    }

                    MouseArea {
                        anchors.fill: parent
                        anchors.margins: -4
                        cursorShape: Qt.PointingHandCursor
                        acceptedButtons: Qt.LeftButton | Qt.RightButton
                        onClicked: (e) => {
                            if (e.button === Qt.RightButton && trayIcon.modelData.menu)
                                flyout.openMenu(trayIcon.modelData.service);
                            else
                                Tray.activate(trayIcon.modelData.service);
                        }
                    }
                }
            }
        }
    }

    // ── the app menu, opened in place of the row ────────────────────────────
    Column {
        id: menuCol
        anchors { left: parent.left; right: parent.right; leftMargin: 14; rightMargin: 14 }
        y: flyout.caretBand + 6
        spacing: 0
        visible: flyout.menuService !== ""
        opacity: visible ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Motion.fast } }

        Item {
            width: parent.width
            height: flyout.headerH

            Pill.MaterialIcon {
                id: backGlyph
                anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                text: "arrow_back"
                color: flyout.dim(backHover.hovered ? 0.9 : 0.55)
                font.pixelSize: 15
            }
            Text {
                anchors {
                    left: backGlyph.right; leftMargin: 6
                    right: parent.right; verticalCenter: parent.verticalCenter
                }
                text: flyout.displayName(flyout.menuItem)
                color: flyout.ink
                font.family: Theme.fontPrimary
                font.pixelSize: 11
                font.weight: Font.DemiBold
                elide: Text.ElideRight
            }
            HoverHandler { id: backHover }
            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: flyout.closeMenu()
            }
        }

        Item { width: 1; height: flyout.headerGap }

        Flickable {
            width: parent.width
            height: Math.max(0, flyout.menuViewportHeight)
            contentHeight: rowsCol.implicitHeight
            clip: true
            interactive: contentHeight > height
            boundsBehavior: Flickable.StopAtBounds

            Column {
                id: rowsCol
                width: parent.width

                Repeater {
                    model: flyout.menuRows

                    delegate: Item {
                        id: entryRow
                        required property var modelData
                        readonly property var node: modelData.node
                        readonly property bool sep: modelData.sep
                        readonly property bool hasKids: modelData.hasKids
                        readonly property bool rowEnabled: node.enabled !== false
                        width: rowsCol.width
                        height: entryRow.sep ? 7 : flyout.rowH

                        Rectangle {
                            visible: entryRow.sep
                            anchors.verticalCenter: parent.verticalCenter
                            anchors.left: parent.left
                            anchors.leftMargin: 6 + entryRow.modelData.depth * 12
                            anchors.right: parent.right
                            height: 1
                            color: flyout.dim(0.14)
                        }

                        Rectangle {
                            visible: !entryRow.sep
                            anchors.fill: parent
                            radius: 8
                            color: (entryMa.containsMouse && entryRow.rowEnabled) ? flyout.dim(0.12) : "transparent"
                        }

                        Text {
                            visible: !entryRow.sep
                            anchors {
                                left: parent.left; leftMargin: 6 + entryRow.modelData.depth * 12
                                right: chevron.left; rightMargin: 4
                                verticalCenter: parent.verticalCenter
                            }
                            text: flyout.cleanLabel(entryRow.node.label)
                            color: entryRow.rowEnabled ? flyout.ink : flyout.dim(0.35)
                            font.family: Theme.fontPrimary
                            font.pixelSize: 11
                            elide: Text.ElideRight
                        }

                        Pill.MaterialIcon {
                            id: chevron
                            visible: !entryRow.sep && entryRow.hasKids
                            anchors { right: parent.right; rightMargin: 4; verticalCenter: parent.verticalCenter }
                            text: "chevron_right"
                            rotation: flyout.expanded[entryRow.node.id] ? 90 : 0
                            color: flyout.dim(0.5)
                            font.pixelSize: 14
                        }

                        MouseArea {
                            id: entryMa
                            anchors.fill: parent
                            visible: !entryRow.sep
                            enabled: entryRow.rowEnabled
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: entryRow.hasKids
                                ? flyout.toggleExpand(entryRow.node.id)
                                : flyout.activate(entryRow.node.id)
                        }
                    }
                }
            }
        }

        // Keep the last entry visibly inside the pill instead of letting it sit
        // directly on the rounded outer edge. This also makes menus with and
        // without separators look consistent.
        Item {
            width: 1
            height: flyout.bottomPad
        }
    }
}

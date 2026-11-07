pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Bus
import Ryoku.Ui.Singletons
import "../../../../components"
import shell.services

// Pill workspaces: each non-special workspace across all monitors is a rounded
// pill showing up to 3 tiny app icons resolved via DesktopEntries (same method
// as RailDock). Active workspace pill is expanded and primary-tinted; inactive
// occupied pills are compact and dimmed; empty workspaces are a small dot.
// Click switches workspace, vertical wheel cycles. Monitor dividers are dropped
// in favour of letting the pill sizes carry the visual weight.
// Uses the same restrained spacing as the rest of the rail kit.
// Contract 03 sec 2.3/2.4/4.3/4.4.
Item {
    id: root

    required property string edge
    required property real scale

    readonly property bool horizontal: edge === "top" || edge === "bottom"
    readonly property real cross: 48 * scale

    // Tiny icon size for workspace previews -- non-interactive, informational.
    readonly property real iconPx: 10 * scale
    readonly property int maxIcons: 3

    // Non-special workspaces sorted by (output, workspace number).
    readonly property var entries: {
        const list = Wm.workspaces;
        const out = [];
        const seen = {};
        for (let i = 0; i < list.length; ++i) {
            const w = list[i];
            if (!w || w.special)
                continue;
            const name = w.name || "";
            if (name === "" || seen[name])
                continue;
            seen[name] = true;
            out.push({ name: name, output: w.output || "" });
        }
        if (out.length === 0 && Wm.focusedWorkspace)
            out.push({ name: Wm.focusedWorkspace.name, output: Wm.focusedWorkspace.output || "" });
        out.sort((a, b) => a.output < b.output ? -1 :
            (a.output > b.output ? 1 : Number(a.name) - Number(b.name)));
        return out;
    }
    // The pills follow a keyed copy of entries, so a workspace frame keeps every
    // pill (and its icon row) that is still there.
    BusListModel {
        id: entryModel
        key: "name"
        roles: ["name", "output"]
        rows: root.entries
    }

    // Active set is per-output (the active workspace shown on each output).
    readonly property var activeIds: {
        const s = {};
        const outs = Wm.outputs;
        for (let i = 0; i < outs.length; ++i) {
            const o = outs[i];
            if (o && o.activeWorkspace)
                s[o.activeWorkspace] = true;
        }
        if (Wm.focusedWorkspace)
            s[Wm.focusedWorkspace.name] = true;
        return s;
    }

    // Icon path resolution: same two-step lookup as RailDock.
    function iconFor(className) {
        const desktop = DesktopEntries.heuristicLookup(className);
        const byEntry = (desktop && desktop.icon) ?
            Icons.path(desktop.icon, true) : "";
        return byEntry !== "" ? byEntry :
            Icons.path(className.toLowerCase(), true);
    }

    function focusWorkspace(name) {
        Wm.focusWorkspace(name);
    }

    implicitWidth: horizontal ? strip.implicitWidth : Math.max(cross, strip.implicitWidth)
    implicitHeight: horizontal ? Math.max(cross, strip.implicitHeight) : strip.implicitHeight

    Loader {
        id: strip
        anchors.centerIn: parent
        sourceComponent: root.horizontal ? rowComp : colComp
    }

    Component {
        id: rowComp
        Row {
            spacing: 4 * root.scale
            Repeater {
                model: entryModel
                delegate: pillComp
            }
        }
    }
    Component {
        id: colComp
        Column {
            spacing: 4 * root.scale
            Repeater {
                model: entryModel
                delegate: pillComp
            }
        }
    }

    // Each workspace pill. With ComponentBehavior: Bound, outer-scope IDs
    // (root, Theme, Motion) are accessible; modelData must be required.
    Component {
        id: pillComp
        Item {
            id: pill

            required property var modelData

            readonly property string wsName: modelData.name
            readonly property bool isActive: root.activeIds[wsName] === true

            // Up to maxIcons unique app ids for windows on this workspace.
            readonly property var _liveClasses: {
                const wins = Wm.windows;
                const out = [];
                const seen = {};
                for (let i = 0; i < wins.length; ++i) {
                    if (out.length >= root.maxIcons)
                        break;
                    const t = wins[i];
                    if (!t || t.workspace !== pill.wsName)
                        continue;
                    const cls = t.appId || "";
                    if (!cls || seen[cls])
                        continue;
                    seen[cls] = true;
                    out.push(cls);
                }
                return out;
            }

            // Wm.windows rebuilds on any title change anywhere; swapping in an
            // equal array would still tear down and rebuild the icon Repeaters.
            property var classes: []
            function _adoptClasses() {
                const next = pill._liveClasses;
                if (next.join("\u0000") !== pill.classes.join("\u0000"))
                    pill.classes = next;
            }
            on_LiveClassesChanged: pill._adoptClasses()
            Component.onCompleted: pill._adoptClasses()

            readonly property bool hasIcons: classes.length > 0

            // Pill geometry. For vertical bars, the pill sits across the rail
            // (full 36px width); its height expands when active. For horizontal
            // bars, the pill sits along the rail; its width expands.
            readonly property real gap: 2 * root.scale
            readonly property real padV: 6 * root.scale   // vertical padding (top+bottom)
            readonly property real padH: 5 * root.scale   // horizontal padding (left+right)
            readonly property real dotSize: 6 * root.scale
            readonly property real iconRowWidth: classes.length * root.iconPx
                + Math.max(0, classes.length - 1) * gap
            readonly property real iconColHeight: classes.length * root.iconPx
                + Math.max(0, classes.length - 1) * gap

            // Extent = the along-bar dimension. Every occupied pill sizes to its
            // own app icons, so the active pill matches an inactive one with the
            // same apps and is set apart by its highlight, not by a fixed
            // reserved width that left it a wide near-empty oval beside the
            // compact inactive pills.
            readonly property real occupiedExtent: root.horizontal
                ? (iconRowWidth + 2 * padH)
                : (iconColHeight + 2 * padV)
            readonly property real emptyExtent: dotSize + 2 * (root.horizontal ? padH : padV)
            readonly property real extent: !hasIcons ? emptyExtent : occupiedExtent

            // Cross = the dimension fixed to the rail width.
            readonly property real pillCross: 32 * root.scale

            width: root.horizontal ? extent : pillCross
            height: root.horizontal ? pillCross : extent

            Behavior on width {
                enabled: !Motion.reduce
                NumberAnimation {
                    duration: Motion.fast
                    easing.type: Motion.easeStandard
                }
            }
            Behavior on height {
                enabled: !Motion.reduce
                NumberAnimation {
                    duration: Motion.fast
                    easing.type: Motion.easeStandard
                }
            }

            // Background: primary-tinted for active, subtle for inactive occupied,
            // transparent for empty (the dot carries the visual weight).
            Rectangle {
                anchors.fill: parent
                radius: Math.min(parent.width, parent.height) / 2
                color: pill.hasIcons
                    ? (pill.isActive
                        ? Qt.rgba(Theme.primary.r, Theme.primary.g, Theme.primary.b, 0.16)
                        : Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.07))
                    : "transparent"

                Behavior on color {
                    enabled: !Motion.reduce
                    ColorAnimation { duration: Motion.fast }
                }
            }

            // Empty workspace: a small dot. Active empty gets the primary colour.
            Rectangle {
                visible: !pill.hasIcons
                anchors.centerIn: parent
                width: pill.dotSize
                height: pill.dotSize
                radius: width / 2
                color: pill.isActive ? Theme.primary
                    : Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.28)

                Behavior on color {
                    enabled: !Motion.reduce
                    ColorAnimation { duration: Motion.fast }
                }
            }

            // Active indicator: a small primary line along the bar edge.
            Rectangle {
                visible: pill.isActive && pill.hasIcons
                width: root.horizontal
                    ? Math.min(pill.width * 0.5, 16 * root.scale)
                    : 2 * root.scale
                height: root.horizontal
                    ? 2 * root.scale
                    : Math.min(pill.height * 0.5, 16 * root.scale)
                radius: Math.min(width, height) / 2
                color: Theme.primary
                // Placed on the outer edge of the bar.
                anchors.horizontalCenter: root.horizontal ? parent.horizontalCenter : undefined
                anchors.verticalCenter: !root.horizontal ? parent.verticalCenter : undefined
                anchors.bottom: root.edge === "bottom" ? parent.bottom : undefined
                anchors.bottomMargin: root.edge === "bottom" ? 3 * root.scale : 0
                anchors.right: root.edge === "right" ? parent.right : undefined
                anchors.rightMargin: root.edge === "right" ? 3 * root.scale : 0
                anchors.left: root.edge === "left" ? parent.left : undefined
                anchors.leftMargin: root.edge === "left" ? 3 * root.scale : 0
                anchors.top: root.edge === "top" ? parent.top : undefined
                anchors.topMargin: root.edge === "top" ? 3 * root.scale : 0
            }

            // App icon row/column. Icons re-resolve only when classes changes.
            Loader {
                anchors.centerIn: parent
                active: pill.hasIcons
                sourceComponent: root.horizontal ? iconsRowComp : iconsColComp
            }

            // For horizontal bars: icons in a horizontal row inside the pill.
            Component {
                id: iconsRowComp
                Row {
                    spacing: pill.gap
                    opacity: pill.isActive ? 1.0 : 0.5
                    Behavior on opacity {
                        enabled: !Motion.reduce
                        NumberAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
                    }
                    Repeater {
                        model: pill.classes
                        delegate: wsIconComp
                    }
                }
            }

            // For vertical bars: icons in a vertical column inside the pill.
            Component {
                id: iconsColComp
                Column {
                    spacing: pill.gap
                    opacity: pill.isActive ? 1.0 : 0.5
                    Behavior on opacity {
                        enabled: !Motion.reduce
                        NumberAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
                    }
                    Repeater {
                        model: pill.classes
                        delegate: wsIconComp
                    }
                }
            }

            // Each app icon. Falls back to the app-grid glyph if the icon
            // path cannot be resolved (same pattern as RailDock).
            Component {
                id: wsIconComp
                Item {
                    id: wsIconItem
                    required property string modelData
                    width: root.iconPx
                    height: root.iconPx

                    Image {
                        id: wsIcon
                        anchors.fill: parent
                        source: root.iconFor(wsIconItem.modelData)
                        sourceSize.width: root.iconPx
                        sourceSize.height: root.iconPx
                        smooth: true
                        asynchronous: true
                        visible: source !== "" && status === Image.Ready
                    }
                    SymbolIcon {
                        anchors.fill: parent
                        visible: !wsIcon.visible
                        name: "view-app-grid"
                        size: root.iconPx
                        color: Theme.onSurface
                    }
                }
            }

            // Click to focus this workspace.
            HoverHandler { id: hover }
            MouseArea {
                anchors.fill: parent
                acceptedButtons: Qt.LeftButton
                cursorShape: Qt.PointingHandCursor
                onClicked: root.focusWorkspace(pill.wsName)
            }
        }
    }

    // Vertical wheel: up -> previous workspace, down -> next.
    WheelHandler {
        onWheel: event => Wm.cycleWorkspace(event.angleDelta.y > 0 ? -1 : 1)
    }
}

pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Widgets
import Caelestia.Config
import caelestia.components
import caelestia.components.images
import caelestia.services
import caelestia.utils

StyledRect {
    id: root

    required property int index
    required property int activeWsId
    required property string screenName
    required property var occupied
    required property int groupOffset
    readonly property bool isWorkspace: true
    property real scaleFactor: 1.0
    property real swipeOffset: 0.0
    property bool isSwiping: false
    property var closingWindows: []
    readonly property int baseIndicatorSize: 120
    readonly property int baseWidth: 200
    readonly property int indicatorSize: Math.floor(baseIndicatorSize * scaleFactor)
    readonly property int size: implicitWidth
    readonly property int ws: groupOffset + index + 1
    readonly property int maxIcons: 8
    readonly property bool isOccupied: occupied[ws] ?? false
    readonly property bool hasWindows: isOccupied
    property var kwinWindowList: Kwin.windowList
    readonly property bool active: activeWsId === ws
    readonly property real swipeWeight: {
        if (!isSwiping || swipeOffset === 0.0) return active ? 1.0 : 0.0;
        const activeIdx = activeWsId - 1;
        const targetIdx = swipeOffset > 0 ? activeIdx + 1 : activeIdx - 1;
        const t = Math.abs(swipeOffset);
        const myIdx = ws - 1;
        if (myIdx === activeIdx) return 1.0 - t;
        if (myIdx === targetIdx) return t;
        return 0.0;
    }
    property real smoothSwipeWeight: swipeWeight

    signal selected()
    signal reselected()

    implicitWidth: Math.floor(baseWidth * scaleFactor)
    implicitHeight: indicatorSize
    radius: Tokens.rounding.large
    color: active ? Colours.layer(Colours.palette.m3surfaceContainerHighest, 1) : (isOccupied ? Colours.tPalette.m3surfaceContainer : "transparent")
    border.color: isSwiping ? Qt.rgba(
        Colours.palette.m3primary.r * smoothSwipeWeight + Colours.tPalette.m3outlineVariant.r * (1.0 - smoothSwipeWeight),
        Colours.palette.m3primary.g * smoothSwipeWeight + Colours.tPalette.m3outlineVariant.g * (1.0 - smoothSwipeWeight),
        Colours.palette.m3primary.b * smoothSwipeWeight + Colours.tPalette.m3outlineVariant.b * (1.0 - smoothSwipeWeight),
        Colours.palette.m3primary.a * smoothSwipeWeight + Colours.tPalette.m3outlineVariant.a * (1.0 - smoothSwipeWeight))
        : (active ? Colours.palette.m3primary : Colours.tPalette.m3outlineVariant)
    border.width: active ? 2 : (isOccupied ? 0 : 2)
    Layout.alignment: Qt.AlignVCenter
    Layout.preferredWidth: Math.floor(baseWidth * scaleFactor)
    Layout.preferredHeight: indicatorSize
    Drag.active: workspaceDragHandler.active
    Drag.source: root
    Drag.hotSpot.x: width / 2
    Drag.hotSpot.y: height / 2
    transform: Translate {
        x: workspaceDragHandler.active ? workspaceDragHandler.translation.x : 0
        y: workspaceDragHandler.active ? workspaceDragHandler.translation.y : 0
    }
    states: [
        State {
            when: workspaceDragHandler.active

            PropertyChanges {
                target: root
                opacity: 0.8
                z: 999
            }
        }
    ]

    Behavior on color { CAnim {} }
    Behavior on smoothSwipeWeight {
        enabled: root.isSwiping

        SmoothedAnimation {
            velocity: -1
            duration: 60
            easing.type: Easing.Linear
        }
    }
    DragHandler {
        id: workspaceDragHandler

        target: null
        onActiveChanged: {
            if (!active) {
                root.Drag.drop();
            }
        }
    }
    StateLayer {
        id: workspaceMouseArea

        anchors.fill: parent
        radius: parent.radius
        onClicked: {
            if (active) {
                reselected();
                let p = parent;
                while (p) {
                    if (p.requestClose) {
                        p.requestClose();
                        break;
                    }
                    p = p.parent;
                }
            } else {
                const wId = Kwin.workspaces[root.ws - 1]?.id || root.ws.toString();
                Kwin.switchToWorkspace(wId, root.screenName);
                selected();
            }
        }
    }
    Item {
        anchors.top: parent.top
        anchors.right: parent.right
        anchors.margins: Tokens.padding.small
        width: Math.floor(28 * root.scaleFactor)
        height: Math.floor(28 * root.scaleFactor)
        z: 99
        opacity: 1.0
        enabled: true

        Behavior on opacity {
            Anim {
                type: Anim.FastEffects
            }
        }
        StateLayer {
            id: closeBtn

            anchors.fill: parent
            radius: parent.width / 2
            onClicked: {
                const wId = Kwin.workspaces[root.ws - 1].id;
                if (wId)
                    Kwin.removeWorkspace(wId);
            }
        }
        MaterialIcon {
            anchors.centerIn: parent
            text: "close"
            fontStyle.pixelSize: Math.max(10, Math.floor(18 * root.scaleFactor))
            color: Colours.palette.m3onSurfaceVariant
        }
    }
    StyledText {
        anchors.bottom: parent.bottom
        anchors.right: parent.right
        anchors.margins: Tokens.padding.small
        text: root.ws.toString()
        font.pixelSize: 24
        font.weight: Font.Bold
        color: Colours.tPalette.m3onSurfaceVariant
        opacity: 0.3
    }
    DropArea {
        id: windowDropArea

        // Remembered because onExited carries no drag argument.
        property var hovering: null

        function clearHover(): void {
            if (windowDropArea.hovering) {
                windowDropArea.hovering.dropTargetScale = 0;
                windowDropArea.hovering = null;
            }
        }

        anchors.fill: parent
        onEntered: drag => {
            //console.log("DROP AREA ENTERED:", drag.source ? drag.source.clientAddress : "null");
            if (!drag.source || drag.source.clientAddress === undefined)
                return;
            if (!("dropTargetScale" in drag.source))
                return;
            windowDropArea.hovering = drag.source;
            drag.source.dropTargetScale = Math.min(width / Math.max(1, drag.source.width), height / Math.max(1, drag.source.height)) * 0.85;
        }
        onExited: windowDropArea.clearHover()
        onDropped: drop => {
            windowDropArea.clearHover();
            const sourceItem = drop.source;
            if (sourceItem && sourceItem.clientAddress) {
                if (sourceItem.wsId !== root.ws) {
                    sourceItem.visible = false;
                    Kwin.setWindowDesktop(sourceItem.clientAddress, root.ws);
                }
                drop.accept();
            }
        }
    }
    GridLayout {
        id: gridLayout

        readonly property int count: repeater.count
        readonly property int effectiveCount: {
            if (windowDropArea.hovering === null) return count;
            return windowDropArea.hovering.wsId !== root.ws ? count + 1 : count;
        }

        anchors.fill: parent
        anchors.margins: Tokens.padding.medium
        rowSpacing: Tokens.padding.small
        columnSpacing: Tokens.padding.small
        columns: effectiveCount <= 2 ? Math.max(1, effectiveCount) : Math.ceil(effectiveCount / 2)

        Repeater {
            id: repeater

            model: ScriptModel {
                values: {
                    const wsId = root.ws;
                    const _ = root.kwinWindowList;
                    let windows = [];
                    const wins = Kwin.windowsForWorkspace(wsId, false);
                    for (let i = 0; i < wins.length; ++i) {
                        const w = wins[i];
                        if (w.output !== root.screenName)
                            continue;
                        if (!Kwin.isIgnoredWindow(w)) {
                            windows.push(w);
                        }
                    }

                    const maxIcons = root.maxIcons;
                    return maxIcons > 0 ? windows.slice(0, maxIcons) : windows;
                }
            }
            delegate: StyledRect {
                id: iconDelegate

                required property var modelData
                required property int index
                readonly property string clientAddress: modelData.address || ""
                readonly property int wsId: root.ws
                property bool expanded: false
                property real dropTargetScale: 0
                readonly property real liftedBy: dragHandler.active ? iconDelegate.dragStartY - iconDelegate.y : 0
                readonly property real windowAspect: {
                    const w = modelData.width, h = modelData.height;
                    return (w > 0 && h > 0) ? w / h : 16 / 9;
                }
                property real dragStartX: 0
                property real dragStartY: 0
                property real dragStartWidth: 0
                property real dragStartHeight: 0
                property Item topLevel: null
                property bool closing: {
                    if (!root.closingWindows) return false;
                    for (let i = 0; i < root.closingWindows.length; i++) {
                        if (root.closingWindows[i] === modelData.address) return true;
                    }
                    return false;
                }

                Component.onCompleted: {
                    if (modelData && !DesktopEntries.heuristicLookup(modelData.iconName || modelData.class || ""))
                        WinIcons.request(modelData.class, modelData.title, modelData.pid ?? 0, modelData.address ? String(modelData.address) : "");
                }

                radius: Tokens.rounding.small
                color: Colours.tPalette.m3surfaceContainerHigh
                Layout.fillWidth: true
                Layout.fillHeight: true
                Layout.rowSpan: (gridLayout.effectiveCount >= 3 && gridLayout.effectiveCount % 2 !== 0 && index === 0) ? 2 : 1
                scale: closing ? 0.0 : (dragHandler.active && dropTargetScale > 0 ? dropTargetScale : 1.0)
                opacity: closing ? 0.0 : 1.0
                visible: !(windowDropArea.hovering !== null && !dragHandler.active && windowDropArea.hovering.clientAddress === iconDelegate.clientAddress)

                Drag.active: dragHandler.active
                Drag.source: iconDelegate
                Drag.hotSpot.x: width / 2
                Drag.hotSpot.y: height / 2
                states: [
                    State {
                        when: dragHandler.active

                        ParentChange {
                            target: iconDelegate
                            parent: topLevel
                            x: iconDelegate.dragStartX
                            y: iconDelegate.dragStartY
                        }
                        PropertyChanges {
                            target: iconDelegate
                            height: iconDelegate.expanded ? Math.round(360 / Math.max(0.2, iconDelegate.windowAspect)) : iconDelegate.dragStartHeight
                            opacity: 0.8
                            width: iconDelegate.expanded ? 360 : iconDelegate.dragStartWidth
                            z: 999
                        }
                    }
                ]

                onLiftedByChanged: {
                    if (!dragHandler.active)
                        iconDelegate.expanded = false;
                    else if (!iconDelegate.expanded && iconDelegate.liftedBy > 170)
                        iconDelegate.expanded = true;
                    else if (iconDelegate.expanded && iconDelegate.liftedBy < 100)
                        iconDelegate.expanded = false;
                }
                onExpandedChanged: Visibilities.streamClaim = iconDelegate.expanded ? iconDelegate.clientAddress : ""
                Component.onDestruction: {
                    if (iconDelegate.expanded)
                        Visibilities.streamClaim = "";
                }

                Behavior on scale { NumberAnimation { duration: 250; easing.type: Easing.OutCubic } }
                Behavior on opacity { NumberAnimation { duration: 250; easing.type: Easing.OutCubic } }
                Behavior on width { NumberAnimation { duration: 220; easing.type: Easing.OutCubic } }
                Behavior on height { NumberAnimation { duration: 220; easing.type: Easing.OutCubic } }
                Behavior on x { enabled: !dragHandler.active; NumberAnimation { duration: 220; easing.type: Easing.OutCubic } }
                Behavior on y { enabled: !dragHandler.active; NumberAnimation { duration: 220; easing.type: Easing.OutCubic } }

                DragHandler {
                    id: dragHandler

                    onActiveChanged: {
                        if (active) {
                            let tl = iconDelegate;
                            while (tl.parent) tl = tl.parent;
                            iconDelegate.topLevel = tl;
                            
                            if (tl) {
                                const p = iconDelegate.mapToItem(tl, 0, 0);
                                iconDelegate.dragStartX = p.x;
                                iconDelegate.dragStartY = p.y;
                            }
                            iconDelegate.dragStartWidth = iconDelegate.width;
                            iconDelegate.dragStartHeight = iconDelegate.height;
                        } else {
                            iconDelegate.Drag.drop();
                        }
                    }
                }
                WindowPreview {
                    active: iconDelegate.expanded
                    address: iconDelegate.clientAddress
                    anchors.fill: parent
                    fallbackIcon: WinIcons.sourceForClient(modelData)
                    fallbackScale: 0.6
                    sourceAspect: iconDelegate.windowAspect
                }
                StateLayer {
                    anchors.fill: parent
                    radius: parent.radius
                    onClicked: {
                        if (root.active) {
                            if (modelData.address) {
                                Kwin.focusWindow(modelData.address);
                                
                                let p = parent;
                                while (p) {
                                    if (p.requestClose) {
                                        p.requestClose();
                                        break;
                                    }
                                    p = p.parent;
                                }
                            }
                        } else {
                            root.selected();
                        }
                    }
                }
            }
        }
        StyledRect {
            id: dropPlaceholder

            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.rowSpan: (gridLayout.effectiveCount >= 3 && gridLayout.effectiveCount % 2 !== 0 && repeater.count === 0) ? 2 : 1
            color: "transparent"
            border.color: Colours.palette.m3outlineVariant
            border.width: 2
            radius: Tokens.rounding.small
            visible: windowDropArea.hovering !== null

            Behavior on width { NumberAnimation { duration: 220; easing.type: Easing.OutCubic } }
            Behavior on height { NumberAnimation { duration: 220; easing.type: Easing.OutCubic } }
        }
    }
}

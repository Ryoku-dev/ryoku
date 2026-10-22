pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import M3Shapes
import Caelestia.Config
import caelestia.components
import caelestia.services
import caelestia.utils

GridLayout {
    id: root

    required property int ws
    required property int activeWsId
    required property string screenName
    required property var occupied

    readonly property bool isWorkspace: true
    readonly property bool isHorizontal: Config.bar.position === "top" || Config.bar.position === "bottom"
    readonly property real rawScale: !isNaN(Config.bar.scale) ? Config.bar.scale : 1.0
    readonly property real scaleFactor: rawScale < 1.0 ? Math.sqrt(Math.max(0.1, rawScale)) : rawScale
    readonly property int barThickness: Math.round(Tokens.sizes.bar.innerWidth * scaleFactor)

    readonly property bool isActive: activeWsId === ws
    // An inactive, empty workspace keeps its pill only when the user wants the
    // full strip; otherwise it collapses to nothing.
    readonly property bool shouldShow: Config.bar.workspaces.showUnoccupied || isOccupied || isActive
    property real reveal: shouldShow ? 1 : 0
    readonly property real revealProgress: Math.max(0, Math.min(1, reveal))

    readonly property real size: ((isHorizontal ? implicitWidth : implicitHeight) + (hasWindows ? Tokens.padding.extraSmall : 0)) * revealProgress

    readonly property int maxIcons: Config.bar.workspaces.maxWindowIcons
    readonly property bool isOccupied: occupied[ws] ?? false
    readonly property bool hasWindows: isOccupied && Config.bar.workspaces.showWindows && (Config.bar.workspaces.maxWindowIcons > 0)
    property var kwinWindowList: Kwin.windowList

    property var _cache: ({ colKeys: "", colIcons: [], rowKeys: "", rowIcons: [] })

    columns: isHorizontal ? -1 : 1
    rows: isHorizontal ? 1 : -1
    flow: isHorizontal ? GridLayout.LeftToRight : GridLayout.TopToBottom

    Layout.alignment: isHorizontal ? Qt.AlignVCenter : Qt.AlignHCenter
    Layout.preferredWidth: isHorizontal ? size : -1
    Layout.preferredHeight: isHorizontal ? -1 : size

    columnSpacing: 0
    rowSpacing: 0

    visible: shouldShow || revealProgress > 0
    opacity: revealProgress

    Behavior on reveal {
        Anim {
            type: Anim.DefaultEffects
        }
    }

    Loader {
        id: indicator

        Layout.alignment: isHorizontal ? (Qt.AlignVCenter | Qt.AlignLeft) : (Qt.AlignHCenter | Qt.AlignTop)
        Layout.preferredWidth: isHorizontal ? (barThickness - Tokens.padding.small) : -1
        Layout.preferredHeight: isHorizontal ? -1 : (barThickness - Tokens.padding.small)

        asynchronous: true
        sourceComponent: Config.bar.workspaces.displayType === BarWorkspaceDisplay.Text ? textComponent : iconComponent
    }

    Component {
        id: textComponent

        StyledText {
            anchors.fill: parent
            animate: true
            text: {
                const wsName = root.ws;
                let displayName = wsName.toString();
                if (Config.bar.workspaces.capitalisation.toLowerCase() === "upper") {
                    displayName = displayName.toUpperCase();
                } else if (Config.bar.workspaces.capitalisation.toLowerCase() === "lower") {
                    displayName = displayName.toLowerCase();
                }
                const label = Config.bar.workspaces.label || displayName;
                const occupiedLabel = Config.bar.workspaces.occupiedLabel || label;
                const activeLabel = Config.bar.workspaces.activeLabel || (root.isOccupied ? occupiedLabel : label);
                return root.activeWsId === root.ws ? activeLabel : root.isOccupied ? occupiedLabel : label;
            }
            color: Config.bar.workspaces.occupiedBg || root.isOccupied || root.activeWsId === root.ws ? Colours.palette.m3onSurface : Colours.layer(Colours.palette.m3outlineVariant, 2)
            horizontalAlignment: Qt.AlignHCenter
            verticalAlignment: Qt.AlignVCenter
            font.family: Tokens.font.workspaces
        }
    }

    Component {
        id: iconComponent

        Item {
            id: iconRoot

            readonly property bool active: root.activeWsId === root.ws
            readonly property list<int> focusShapes: [MaterialShape.Slanted, MaterialShape.Arch, MaterialShape.Oval, MaterialShape.Pill, MaterialShape.Triangle, MaterialShape.Arrow, MaterialShape.Diamond, MaterialShape.Pentagon, MaterialShape.Gem, MaterialShape.VerySunny, MaterialShape.Sunny]

            property int randShape: MaterialShape.Slanted
            property bool wasPositionActive: false
            property int lastKnownWs: -1
            property int prevActiveWsId: -1
            property bool hasRandomShape: false

            property int prevWs: -1

            property int watchedWs: root.ws

            property int lastWatchedWs: -1

            property int swipeStartWsId: -1
            property bool generatedShapeThisSwipe: false

            property real rawSwipeOffset: Kwin.swipeOffsetByOutput?.[root.screenName] ?? Kwin.swipeOffset
            property real lastRawSwipeOffset: 0.0
            property bool isSwiping: false

            readonly property real swipeWeight: {
                if (!isSwiping || rawSwipeOffset === 0.0) return active ? 1.0 : 0.0;

                // Use swipeStartWsId to prevent KWin desyncs when activeWsId changes before rawSwipeOffset resets
                const startId = swipeStartWsId !== -1 ? swipeStartWsId : root.activeWsId;
                const activeIdx = startId - 1;
                const targetIdx = rawSwipeOffset > 0 ? activeIdx + 1 : activeIdx - 1;
                const t = Math.abs(rawSwipeOffset);
                const myIdx = root.ws - 1;
                if (myIdx === activeIdx) return 1.0 - t;
                if (myIdx === targetIdx) return t;
                return 0.0;
            }

            property real smoothSwipeWeight: swipeWeight

            function handleActivation() {
                const wsChanged = lastKnownWs !== root.ws;
                if (active && (!wasPositionActive || wsChanged)) {
                    if (!hasRandomShape) {
                        randShape = focusShapes[Math.floor(Math.random() * focusShapes.length)];
                        wsShape.shape = randShape;
                        hasRandomShape = true;
                    }
                } else if (!active && (wasPositionActive || wsChanged)) {
                    if (!isSwiping) {
                        const targetShape = root.isOccupied ? MaterialShape.Square : MaterialShape.Circle;
                        wsShape.shape = targetShape;
                        hasRandomShape = false;
                    }
                }
                wasPositionActive = active;
                prevWs = lastKnownWs;
                lastKnownWs = root.ws;
                prevActiveWsId = root.activeWsId;
            }

            implicitWidth: barThickness - Tokens.padding.small
            implicitHeight: barThickness - Tokens.padding.small

            onRawSwipeOffsetChanged: {
                if (rawSwipeOffset !== 0.0) {
                    if (lastRawSwipeOffset === 0.0) {
                        swipeStartWsId = root.activeWsId;
                    }
                    isSwiping = true;
                    swipeSettleTimer.stop();
                } else {
                    swipeSettleTimer.restart();
                }
                lastRawSwipeOffset = rawSwipeOffset;
            }

            onIsSwipingChanged: {
                if (!isSwiping) {
                    generatedShapeThisSwipe = false;
                }
            }

            onSmoothSwipeWeightChanged: {
                if (isSwiping) {
                    if (smoothSwipeWeight >= 0.05 && !hasRandomShape) {
                        if (!generatedShapeThisSwipe && !active) {
                            randShape = focusShapes[Math.floor(Math.random() * focusShapes.length)];
                            generatedShapeThisSwipe = true;
                        }
                        wsShape.shape = randShape;
                        hasRandomShape = true;
                    } else if (smoothSwipeWeight < 0.05 && hasRandomShape) {
                        wsShape.shape = root.isOccupied ? MaterialShape.Square : MaterialShape.Circle;
                        hasRandomShape = false;
                    }
                }
            }

            onWatchedWsChanged: {
                if (lastWatchedWs !== -1 && watchedWs !== lastWatchedWs && !active) {
                    if (!isSwiping) {
                        wsShape.shape = root.isOccupied ? MaterialShape.Square : MaterialShape.Circle;
                        hasRandomShape = false;
                    }
                }
                lastWatchedWs = watchedWs;
            }

            onPrevActiveWsIdChanged: {
                if (prevActiveWsId !== -1 && prevActiveWsId !== root.activeWsId && active) {
                    handleActivation();
                }
            }

            onActiveChanged: handleActivation()

            Component.onCompleted: {
                if (active) {
                    handleActivation();
                } else {
                    wsShape.shape = root.isOccupied ? MaterialShape.Square : MaterialShape.Circle;
                    hasRandomShape = false;
                }
                wasPositionActive = active;
                prevWs = -1;
                lastKnownWs = root.ws;
                prevActiveWsId = root.activeWsId;
                lastWatchedWs = root.ws;
            }

            Timer {
                id: swipeSettleTimer

                interval: 120
                repeat: false
                onTriggered: {
                    iconRoot.isSwiping = false;
                    if (!iconRoot.active) {
                        wsShape.shape = root.isOccupied ? MaterialShape.Square : MaterialShape.Circle;
                        hasRandomShape = false;
                    }
                }
            }

            Behavior on smoothSwipeWeight {
                enabled: iconRoot.isSwiping

                SmoothedAnimation { velocity: -1; duration: 60; easing.type: Easing.Linear }
            }

            MaterialShape {
                id: wsShape

                anchors.centerIn: parent
                implicitSize: iconRoot.width
                width: implicitWidth
                height: implicitHeight
                scale: {
                    if (iconRoot.isSwiping) {
                        return (1 / 3) + (iconRoot.smoothSwipeWeight * (1 / 3));
                    }
                    return iconRoot.active ? 2 / 3 : 1 / 3;
                }
                color: {
                    const isActiveVisual = iconRoot.isSwiping ? (iconRoot.smoothSwipeWeight >= 0.8) : iconRoot.active;
                    return Config.bar.workspaces.occupiedBg || root.isOccupied || isActiveVisual ? Colours.palette.m3onSurface : Colours.layer(Colours.palette.m3outlineVariant, 2);
                }

                Behavior on color {
                    CAnim {}
                }

                Behavior on scale {
                    enabled: !iconRoot.isSwiping

                    Anim {
                        type: Anim.FastSpatial
                    }
                }
            }
        }
    }

    Loader {
        id: windows

        asynchronous: true

        Layout.alignment: isHorizontal ? Qt.AlignVCenter : Qt.AlignHCenter
        Layout.fillWidth: isHorizontal && enabled
        Layout.fillHeight: !isHorizontal && enabled
        Layout.topMargin: isHorizontal ? 0 : -barThickness / 10
        Layout.leftMargin: isHorizontal ? -barThickness / 10 : 0

        visible: active
        active: root.hasWindows

        sourceComponent: isHorizontal ? rowComponent : columnComponent
    }

    Component {
        id: columnComponent

        Column {
            spacing: 0

            add: Transition {
                Anim {
                    properties: "scale"
                    from: 0
                    to: 1
                    easing: Tokens.anim.standardDecel
                }
            }

            move: Transition {
                Anim {
                    properties: "scale"
                    to: 1
                    easing: Tokens.anim.standardDecel
                }
                Anim {
                    properties: "x,y"
                }
            }

            Repeater {
                model: ScriptModel {
                    values: {
                        const wins = Kwin.filterWindows(Kwin.windowList, root.ws, root.screenName, false);
                        let windows = wins.filter(w => !Kwin.isIgnoredWindow(w));

                        const maxIcons = root.Config.bar.workspaces.maxWindowIcons;
                        windows = maxIcons > 0 ? windows.slice(0, maxIcons) : windows;
                        const keys = windows.map(w => w.address || w["class"]).sort().join(",");
                        if (keys === root._cache.colKeys) return root._cache.colIcons;
                        root._cache.colKeys = keys;
                        root._cache.colIcons = windows;
                        return windows;
                    }
                }

                MaterialIcon {
                    required property var modelData

                    grade: 0
                    text: Icons.getAppCategoryIcon(modelData.lastIpcObject ? modelData.lastIpcObject["class"] : modelData["class"], "terminal")
                    color: Colours.palette.m3onSurfaceVariant
                }
            }
        }
    }

    Component {
        id: rowComponent

        Row {
            spacing: 0

            add: Transition {
                Anim {
                    properties: "scale"
                    from: 0
                    to: 1
                    easing: Tokens.anim.standardDecel
                }
            }

            move: Transition {
                Anim {
                    properties: "scale"
                    to: 1
                    easing: Tokens.anim.standardDecel
                }
                Anim {
                    properties: "x,y"
                }
            }

            Repeater {
                model: ScriptModel {
                    values: {
                        const wins = Kwin.filterWindows(Kwin.windowList, root.ws, root.screenName, false);
                        let windows = wins.filter(w => !Kwin.isIgnoredWindow(w));

                        const maxIcons = root.Config.bar.workspaces.maxWindowIcons;
                        windows = maxIcons > 0 ? windows.slice(0, maxIcons) : windows;
                        const keys = windows.map(w => w.address || w["class"]).sort().join(",");
                        if (keys === root._cache.rowKeys) return root._cache.rowIcons;
                        root._cache.rowKeys = keys;
                        root._cache.rowIcons = windows;
                        return windows;
                    }
                }

                MaterialIcon {
                    required property var modelData

                    grade: 0
                    text: Icons.getAppCategoryIcon(modelData.lastIpcObject ? modelData.lastIpcObject["class"] : modelData["class"], "terminal")
                    color: Colours.palette.m3onSurfaceVariant
                }
            }
        }
    }

    Behavior on Layout.preferredHeight {
        enabled: !isHorizontal

        Anim {}
    }

    Behavior on Layout.preferredWidth {
        enabled: isHorizontal

        Anim {}
    }
}

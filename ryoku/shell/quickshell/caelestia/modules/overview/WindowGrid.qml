pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Widgets
import Caelestia.Config
import Caelestia.Layouts
import caelestia.components
import caelestia.components.controls
import caelestia.components.images
import caelestia.services
import caelestia.utils

Item {
    id: root

    property var cardItems: []
    property var activeInfoClient: null
    property var panels: null
    required property ShellScreen screen
    property var closingWindows: []
    property alias indicatorContainer: indicatorContainer
    readonly property real overviewBorderThickness: Math.min(width, height) * 0.15
    readonly property real indicatorScale: Math.max(0.5, Math.min(3, height / 768))
    readonly property real indicatorSpace: indicatorContainer.height + (Tokens.padding.large * 2) * indicatorScale
    readonly property real verticalOffset: indicatorSpace - overviewBorderThickness
    readonly property int activeWsId: {
        const perOutput = Kwin.activeByOutput[root.screen.name];
        if (perOutput > 0)
            return perOutput;
        return Kwin.activeWsId > 0 ? Kwin.activeWsId : 1;
    }
    property bool ignoreNextSwitch: false
    property bool _initialized: false
    property bool isDragging: false
    readonly property real edgeBand: 140
    readonly property int edgeDirection: {
        if (!root.isDragging || Visibilities.dragAddress === "" || Visibilities.dragOriginScreen !== root.screen.name)
            return 0;
        const local = Visibilities.dragX - root.screen.x;
        const span = root.screen.width;
        if (local < 0 || local >= span)
            return 0;
        if (local < root.edgeBand)
            return -1;
        if (local > span - root.edgeBand)
            return 1;
        return 0;
    }
    readonly property real hoverScale: 1.02
    property int selectedIndex: -1
    readonly property var currentWindows: {
        if (listView.currentIndex < 0)
            return [];
        const wsList = Kwin.workspaces;
        if (listView.currentIndex >= wsList.length)
            return [];
        const wsId = wsList[listView.currentIndex].index;
        const _ = Kwin.windowList;
        return Kwin.filterWindows(Kwin.windowsForWorkspace(wsId, false), null, root.screen.name);
    }

    signal requestWindowInfo(var client)
    signal requestClose()

    /// The screen containing a point in global coordinates, or null.
    function screenAtGlobal(gx: real, gy: real): var {
        const all = Quickshell.screens;
        for (let i = 0; i < all.length; ++i) {
            const s = all[i];
            if (gx >= s.x && gx < s.x + s.width && gy >= s.y && gy < s.y + s.height)
                return s;
        }
        return null;
    }

    function cycleSelection(backwards: bool): void {
        const n = root.currentWindows.length;
        if (n === 0)
            return;
        if (backwards)
            root.selectedIndex = root.selectedIndex <= 0 ? n - 1 : root.selectedIndex - 1;
        else
            root.selectedIndex = root.selectedIndex >= n - 1 ? 0 : root.selectedIndex + 1;
    }
    function activateSelected(): void {
        const wins = root.currentWindows;
        if (root.selectedIndex < 0 || root.selectedIndex >= wins.length)
            return;
        const addr = wins[root.selectedIndex].address;
        if (!addr)
            return;
                    Kwin.focusWindow(addr);
        if (listView.currentIndex >= 0)
            Kwin.switchToWorkspace(Kwin.workspaces[listView.currentIndex].index, root.screen.name);
        root.requestClose();
    }
    function syncPage() {
        if (root.isDragging) return;
        for (let i = 0; i < Kwin.workspaces.length; ++i) {
            const wId = Kwin.workspaces[i].index;
            if (wId === activeWsId) {
                if (listView.currentIndex !== i) {
                    listView.currentIndex = i;
                }
                if (!root._initialized || root.opacity <= 0.01)
                    listView.positionViewAtIndex(i, ListView.SnapPosition);
                break;
            }
        }
        root.ignoreNextSwitch = false;
        ignoreTimer.stop();
        root._initialized = true;
    }

    onOpacityChanged: {
        if (opacity <= 0) {
            selectedIndex = -1;
            root.isDragging = false;
        } else {
            root.syncPage();
            if (Visibilities.preOverviewActiveWindowAddress !== "") {
                const targetAddress = Visibilities.preOverviewActiveWindowAddress;
                let foundIndex = -1;
                const wins = root.currentWindows;
                if (wins) {
                    for (let i = 0; i < wins.length; ++i) {
                        if (wins[i].address === targetAddress) {
                            foundIndex = i;
                            break;
                        }
                    }
                }
                root.selectedIndex = foundIndex;
            } else {
                root.selectedIndex = -1;
            }
        }
    }
    onActiveWsIdChanged: root.syncPage()
    Component.onCompleted: {
        const count = Kwin.workspaces.length;
        for (let i = 0; i < count; ++i) {
            workspaceModel.append({});
        }

        Qt.callLater(syncPage);
    }

    Connections {
        function onCycleOverview(backwards) {
            if (root.opacity > 0)
                root.cycleSelection(backwards);
        }

        target: Visibilities
    }
    Shortcut {
        sequences: ["Return", "Enter"]
        enabled: root.opacity > 0
        onActivated: root.activateSelected()
    }
    Shortcut {
        sequences: {
            const s = ["Tab", "Right", "Down"];
            if (GlobalConfig.launcher.vimKeybinds) {
                s.push("Ctrl+J", "Ctrl+N");
            }
            return s;
        }
        enabled: root.opacity > 0
        onActivated: root.cycleSelection(false)
    }
    Shortcut {
        sequences: {
            const s = ["Shift+Tab", "Backtab", "Left", "Up"];
            if (GlobalConfig.launcher.vimKeybinds) {
                s.push("Ctrl+K", "Ctrl+P");
            }
            return s;
        }
        enabled: root.opacity > 0
        onActivated: root.cycleSelection(true)
    }
    ListModel {
        id: workspaceModel
    }
    Connections {
        function onWorkspacesChanged() {
            const newCount = Kwin.workspaces.length;
            while (workspaceModel.count < newCount) {
                workspaceModel.append({});
            }
            while (workspaceModel.count > newCount) {
                workspaceModel.remove(workspaceModel.count - 1);
            }
        }

        target: Kwin
    }
    ListView {
        id: listView

        property real rawSwipeOffset: Kwin.swipeOffsetByOutput?.[root.screen.name] ?? Kwin.swipeOffset

        property real targetContentX: (currentIndex + rawSwipeOffset) * width

        anchors.fill: parent
        anchors.topMargin: -verticalOffset
        anchors.bottomMargin: verticalOffset
        orientation: ListView.Horizontal
        highlightRangeMode: ListView.NoHighlightRange
        cacheBuffer: 100000
        boundsBehavior: Flickable.StopAtBounds
        interactive: false
        contentX: root._initialized ? targetContentX : currentIndex * width
        model: workspaceModel

        onCountChanged: Qt.callLater(root.syncPage)
        onCurrentIndexChanged: {
            if (root.ignoreNextSwitch) return;
            if (root.isDragging) return;
            switchTimer.restart();
        }

        delegate: Item {
            id: page

            required property int index
            readonly property int wsId: (Kwin.workspaces && index < Kwin.workspaces.length) ? Kwin.workspaces[index].index : index + 1
            readonly property string wsName: (Kwin.workspaces && index < Kwin.workspaces.length) ? Kwin.workspaces[index].name : wsId.toString()
            property var wsWindows: []
            readonly property var _winTrigger: Kwin.windowList

            function _updateWsWindows(): void {
                const arr = Kwin.filterWindows(Kwin.windowsForWorkspace(wsId, false), null, root.screen.name);

                let changed = arr.length !== wsWindows.length;
                if (!changed) {
                    for (let i = 0; i < arr.length; ++i) {
                        const a = arr[i];
                        const b = wsWindows[i];
                        if (a.address !== b.address || a.width !== b.width || a.height !== b.height || a.x !== b.x || a.y !== b.y || a.title !== b.title) {
                            changed = true;
                            break;
                        }
                    }
                }

                if (changed) {
                    wsWindows = arr;
                }
            }

            on_WinTriggerChanged: _updateWsWindows()
            onWsIdChanged: _updateWsWindows()

            width: listView.width
            height: listView.height
            Component.onCompleted: {
                _updateWsWindows();
                //console.log("WindowGrid Page initialized. wsId:", wsId, "windows found:", wsWindows.length, "Total windows globally:", Kwin.windowList.length);
            }
            onWsWindowsChanged: {
                //console.log("WindowGrid Page updated. wsId:", wsId, "windows found:", wsWindows.length);
            }

            TapHandler {
                onTapped: root.requestClose()
            }
            DropArea {
                anchors.fill: parent
                onDropped: drop => {
                    const sourceItem = drop.source;
                    if (sourceItem && sourceItem.clientAddress) {
                        if (sourceItem.wsId !== undefined && sourceItem.wsId !== page.wsId) {
                            sourceItem.visible = false;
                            const addr = sourceItem.clientAddress;
                            const targetId = page.wsId;
                            Qt.callLater(() => {
                                Kwin.setWindowDesktop(addr, targetId);
                            });
                            drop.accept();
                        }
                    }
                }
            }
            Item {
                id: gridItem

                readonly property real hoverHeadroom: Math.ceil(Math.max(parent.width, parent.height) * (root.hoverScale - 1) / 2)
                property var windowLayout: Config.overview.layoutType === 0 ? LayoutKde.calculateLayout(page.wsWindows, width, height, Tokens.spacing.large, Tokens.spacing.large) : LayoutGnome.calculateLayout(page.wsWindows, width, height, Tokens.spacing.large, Tokens.spacing.large)

                anchors.fill: parent
                anchors.margins: (root.panels ? root.panels.overviewBorderThickness : Tokens.padding.extraLarge) + hoverHeadroom

                Repeater {
                    model: page.wsWindows
                    delegate: StyledRect {
                        id: activeWin

                        required property var modelData
                        required property int index
                        readonly property string clientAddress: modelData.address
                        readonly property int wsId: page.wsId
                            readonly property var layoutProps: gridItem.windowLayout && gridItem.windowLayout[modelData.address] ? gridItem.windowLayout[modelData.address] : { x: 0, y: 0, width: 200, height: 150 }
                            readonly property real windowAspect: {
                                const w = modelData.width;
                                const h = modelData.height;
                                return (w > 0 && h > 0) ? (w / h) : (16.0 / 10.0);
                            }
                            readonly property bool isSelected: page.index === listView.currentIndex && activeWin.index === root.selectedIndex && !root.activeInfoClient
                            readonly property bool showCaption: height > 96 && (hover.hovered || isSelected)

                            property bool closing: false
                            property url infoScreenshot: ""
                            property real dropTargetScale: 0
                            readonly property bool morphed: dragHandler.active && activeWin.dropTargetScale > 0

                            function publishDrag(): void {
                                if (!dragHandler.active)
                                    return;
                                const p = dragHandler.centroid.scenePosition;
                                Visibilities.setDrag(activeWin.clientAddress, root.screen.x + p.x, root.screen.y + p.y, activeWin.width, activeWin.height, root.screen.name);
                            }

                            x: dragHandler.active ? x : layoutProps.x
                            y: dragHandler.active ? y : layoutProps.y
                            width: layoutProps.width
                            height: layoutProps.height
                            color: activeWin.morphed ? "transparent" : Colours.palette.m3surfaceContainer
                            radius: Tokens.rounding.large
                            scale: {
                                if (closing)
                                    return 0;
                                if (dragHandler.active && activeWin.dropTargetScale > 0)
                                    return activeWin.dropTargetScale;
                                return activeWin.isSelected && !dragHandler.active ? root.hoverScale : 1;
                            }
                            // Once the drag is over another screen that screen
                            // draws it, and the half still poking out here would
                            // otherwise sit there as a stream-less icon.
                            opacity: closing || Visibilities.streamClaim === activeWin.clientAddress ? 0 : 1
                            border.width: activeWin.isSelected && !activeWin.morphed ? 2 : 0
                            border.color: Colours.palette.m3primary

                            onXChanged: activeWin.publishDrag()
                            onYChanged: activeWin.publishDrag()

                            Component.onCompleted: {
                                root.cardItems = [...root.cardItems, activeWin];
                                if (modelData && !DesktopEntries.heuristicLookup(modelData.iconName || modelData.class || "")) {
                                    WinIcons.request(modelData.class, modelData.title, modelData.pid ?? 0, modelData.address ? String(modelData.address) : "");
                                }
                            }
                            Component.onDestruction: {
                                root.cardItems = root.cardItems.filter(x => x !== activeWin);
                            }
                            states: [
                                State {
                                    when: dragHandler.active

                                    ParentChange {
                                        target: activeWin
                                        parent: root
                                    }
                                    PropertyChanges {
                                        target: activeWin
                                        opacity: 0.8
                                    }
                                }
                            ]

                            DragHandler {
                                id: dragHandler

                                onActiveChanged: {
                                    root.isDragging = active;
                                    if (!active) {
                                        activeWin.dropTargetScale = 0;
                                        Visibilities.clearDrag();
                                        switchTimer.restart();


                                        const target = root.screenAtGlobal(Visibilities.dragX, Visibilities.dragY);
                                        if (target && target.name !== root.screen.name) {
                                            const addr = clientAddress;
                                            activeWin.Drag.cancel();
                                            activeWin.visible = false;
                                            Qt.callLater(() => {
                                                Kwin.sendToOutput(addr, target.name);
                                            });
                                            return;
                                        }

                                        let dropAction = activeWin.Drag.drop();
                                        if (dropAction !== Qt.IgnoreAction) {
                                            return;
                                        }

                                        const targetWsId = Kwin.workspaces[listView.currentIndex].index;
                                        if (targetWsId !== page.wsId) {
                                            activeWin.visible = false;
                                            const addr = clientAddress;
                                            Qt.callLater(() => {
                                                Kwin.setWindowDesktop(addr, targetWsId);
                                            });
                                        }
                                    }
                                }
                            }
                            Behavior on scale {
                                Anim {
                                    type: dragHandler.active ? Anim.SlowSpatial : Anim.DefaultSpatial
                                }
                            }
                            Behavior on opacity {
                                NumberAnimation {
                                    id: opacityAnim

                                    duration: 250
                                    easing.type: Easing.OutCubic
                                }
                            }
                            Connections {
                                function onRunningChanged() {
                                    if (!opacityAnim.running && activeWin.closing) {
                                        Kwin.closeWindow(modelData.address);
                                    }
                                }

                                target: opacityAnim
                            }
                            Behavior on x { enabled: !dragHandler.active && root.opacity > 0.5; NumberAnimation { duration: 250; easing.type: Easing.OutQuad } }
                            Behavior on y { enabled: !dragHandler.active && root.opacity > 0.5; NumberAnimation { duration: 250; easing.type: Easing.OutQuad } }
                            HoverHandler {
                                id: hover

                                onHoveredChanged: {
                                    if (page.index === listView.currentIndex) {
                                        if (hovered) {
                                            root.selectedIndex = activeWin.index;
                                        } else if (root.selectedIndex === activeWin.index) {
                                            root.selectedIndex = -1;
                                        }
                                    }
                                }
                            }

                            CachingIconImage {
                                anchors.centerIn: parent
                                implicitSize: Math.round(Math.min(activeWin.width, activeWin.height) * 0.62)
                                opacity: activeWin.morphed ? 1 : 0
                                source: WinIcons.sourceForClient(modelData)
                                visible: opacity > 0.01
                                z: 10

                                Behavior on opacity {
                                    Anim {
                                        type: Anim.SlowEffects
                                    }
                                }
                            }

                            Item {
                                id: cardLayout

                                anchors.fill: parent
                                anchors.margins: Tokens.padding.small
                                opacity: activeWin.morphed ? 0 : 1
                                visible: opacity > 0.01

                                Behavior on opacity {
                                    Anim {
                                        type: Anim.SlowEffects
                                    }
                                }

                                StyledClippingRect {
                                    id: thumb

                                    anchors.top: parent.top
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    height: parent.height - (caption.opacity * (caption.implicitHeight + Tokens.padding.extraSmall * 2))
                                    color: Colours.tPalette.m3surfaceContainerHighest
                                    radius: Tokens.rounding.medium

                                    WindowPreview {
                                        active: root.opacity > 0
                                            && (dragHandler.active || Math.abs(page.index - listView.currentIndex) <= 1)
                                            && !(root.activeInfoClient && root.activeInfoClient.address === modelData.address)
                                            && Visibilities.streamClaim !== modelData.address
                                        address: modelData.address ?? ""
                                        width: {
                                            const wAspect = activeWin.windowAspect;
                                            const containerAspect = thumb.width / Math.max(1, thumb.height);
                                            return (wAspect > containerAspect) ? thumb.height * wAspect : thumb.width;
                                        }
                                        height: {
                                            const wAspect = activeWin.windowAspect;
                                            const containerAspect = thumb.width / Math.max(1, thumb.height);
                                            return (wAspect > containerAspect) ? thumb.height : thumb.width / wAspect;
                                        }
                                        anchors.centerIn: parent
                                        fallbackIcon: WinIcons.sourceForClient(modelData)
                                        sourceAspect: activeWin.windowAspect
                                    }

                                    Image {
                                        anchors.fill: parent
                                        source: activeWin.infoScreenshot
                                        visible: root.activeInfoClient && root.activeInfoClient.address === modelData.address && activeWin.infoScreenshot !== ""
                                        fillMode: Image.PreserveAspectCrop
                                    }
                                }
                                RowLayout {
                                    id: caption

                                    spacing: Tokens.spacing.small
                                    opacity: activeWin.showCaption ? 1 : 0
                                    visible: opacity > 0.01
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.bottom: parent.bottom
                                    anchors.leftMargin: Tokens.padding.extraSmall
                                    anchors.rightMargin: Tokens.padding.extraSmall
                                    anchors.bottomMargin: Tokens.padding.extraSmall

                                    Behavior on opacity { Anim {} }

                                    IconImage {
                                        implicitSize: Math.round(titleText.implicitHeight * 1.1)
                                        asynchronous: true
                                        source: WinIcons.sourceForClient(modelData)
                                    }
                                    StyledText {
                                        id: titleText

                                        text: modelData.title || modelData.class || ""
                                        color: Colours.palette.m3primary
                                        font: Tokens.font.body.small
                                        elide: Text.ElideRight
                                        Layout.fillWidth: true
                                    }
                                }
                            }

                            StateLayer {
                                anchors.fill: parent
                                radius: Tokens.rounding.large
                                stateOpacity: containsMouse || manualHoverOverride ? 0.02 : 0
                                onClicked: {
                                    if (modelData.address) {
                                        Kwin.focusWindow(modelData.address);
                                        Kwin.switchToWorkspace(page.wsId, root.screen.name);
                                    }
                                    if (typeof Visibilities !== "undefined")
                                        Visibilities.setOverview(false);
                                }
                            }

                            RowLayout {
                                anchors.top: cardLayout.top
                                anchors.right: cardLayout.right
                                anchors.margins: Tokens.padding.small
                                spacing: Tokens.spacing.small
                                opacity: hover.hovered && !dragHandler.active ? 1 : 0
                                visible: opacity > 0.01

                                Behavior on opacity { Anim {} }
                                StyledRect {
                                    implicitWidth: infoIcon.implicitHeight + Tokens.padding.small * 2
                                    implicitHeight: infoIcon.implicitHeight + Tokens.padding.small * 2
                                    radius: Tokens.rounding.small
                                    color: Colours.palette.m3secondaryContainer

                                    StateLayer {
                                        anchors.fill: parent
                                        radius: Tokens.rounding.small

                                        onClicked: {
                                            thumb.grabToImage(function(result) {
                                                activeWin.infoScreenshot = result.url;
                                                root.requestWindowInfo(modelData);
                                            });
                                        }
                                    }
                                    MaterialIcon {
                                        id: infoIcon

                                        anchors.centerIn: parent
                                        text: "chevron_right"
                                        color: Colours.palette.m3onSecondaryContainer
                                        fontStyle.pointSize: Tokens.font.body.medium.pointSize
                                    }
                                }
                                StyledRect {
                                    implicitWidth: closeIcon.implicitHeight + Tokens.padding.small * 2
                                    implicitHeight: closeIcon.implicitHeight + Tokens.padding.small * 2
                                    radius: Tokens.rounding.small
                                    color: Colours.palette.m3errorContainer

                                    StateLayer {
                                        anchors.fill: parent
                                        radius: Tokens.rounding.small
                                        onClicked: {
                                            if (modelData.address) {
                                                activeWin.closing = true;
                                                root.closingWindows = root.closingWindows.concat([modelData.address]);
                                            }
                                        }
                                    }
                                    MaterialIcon {
                                        id: closeIcon

                                        anchors.centerIn: parent
                                        text: "close"
                                        color: Colours.palette.m3onErrorContainer
                                        fontStyle.pointSize: Tokens.font.body.medium.pointSize
                                    }
                                }
                            }

                            Drag.active: dragHandler.active
                            Drag.source: activeWin
                            Drag.hotSpot: dragHandler.centroid.position
                        }
                    }
                }
            }

        Timer {
            id: switchTimer

            interval: 50
            onTriggered: {
                if (Kwin.workspaces.length > listView.currentIndex) {
                    const wId = Kwin.workspaces[listView.currentIndex].index;
                    if (root.activeWsId !== wId) {
                        Kwin.switchToWorkspace(wId, root.screen.name);
                    }
                }
            }
        }
        MouseArea {
            anchors.fill: parent
            acceptedButtons: Qt.NoButton
            onWheel: event => {
                if (!Config.bar.scrollActions.workspaces) return;

                if (event.angleDelta.y > 0 || event.angleDelta.x > 0) {
                    if (listView.currentIndex > 0) {
                        listView.currentIndex -= 1;
                    }
                } else if (event.angleDelta.y < 0 || event.angleDelta.x < 0) {
                    if (listView.currentIndex < listView.count - 1) {
                        listView.currentIndex += 1;
                    }
                }
            }
        }

        Behavior on contentX {
            enabled: root._initialized

            NumberAnimation {
                duration: listView.rawSwipeOffset === 0.0 ? 300 : 0
                easing.type: listView.rawSwipeOffset === 0.0 ? Easing.OutCubic : Easing.Linear
            }
        }
    }
    StyledRect {
        id: indicatorContainer

        anchors.bottom: parent.bottom
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottomMargin: Tokens.padding.large * root.indicatorScale
        implicitWidth: workspaceIndicator.implicitWidth + (Tokens.padding.large * 2) * root.indicatorScale
        implicitHeight: workspaceIndicator.implicitHeight + (Tokens.padding.medium * 2) * root.indicatorScale
        radius: Tokens.rounding.large * root.indicatorScale
        color: Colours.layer(Colours.palette.m3surfaceContainerHigh, 2)

        WorkspaceIndicator {
            id: workspaceIndicator

            anchors.centerIn: parent
            baseScale: root.indicatorScale
            maxWidth: Math.max(200, root.width - 100)
            screenName: root.screen.name
            count: listView.count
            currentIndex: listView.currentIndex
            closingWindows: root.closingWindows
            onWorkspaceSelected: index => {
                root.ignoreNextSwitch = false;
                listView.currentIndex = index;
            }
            onWorkspaceReselected: root.requestClose()
            onCreateWorkspaceRequest: {
                root.ignoreNextSwitch = true;
                Kwin.createWorkspace();
                ignoreTimer.restart();
            }
        }
    }
    Timer {
        id: ignoreTimer

        interval: 500
        onTriggered: root.ignoreNextSwitch = false
    }
    Item {
        id: incoming

        readonly property var window: {
            const addr = Visibilities.dragAddress;
            if (!addr)
                return null;
            const all = Kwin.windowList || [];
            for (let i = 0; i < all.length; ++i)
                if (all[i].address === addr)
                    return all[i];
            return null;
        }
        readonly property bool arriving: Visibilities.dragAddress !== ""
            && Visibilities.dragOriginScreen !== root.screen.name
            && Visibilities.dragX >= root.screen.x
            && Visibilities.dragX < root.screen.x + root.screen.width
            && Visibilities.dragY >= root.screen.y
            && Visibilities.dragY < root.screen.y + root.screen.height
        readonly property real aspect: {
            const w = incoming.window;
            if (w && w.width > 0 && w.height > 0)
                return w.width / w.height;
            return 16 / 9;
        }

        height: Visibilities.dragHeight > 0 ? Visibilities.dragHeight : Math.round(width / Math.max(0.2, incoming.aspect))
        opacity: arriving ? 1 : 0
        visible: opacity > 0.01
        width: Visibilities.dragWidth > 0 ? Visibilities.dragWidth : Math.round(Math.min(root.width, root.height) * 0.32)
        x: Visibilities.dragX - root.screen.x - width / 2
        y: Visibilities.dragY - root.screen.y - height / 2
        z: 1000

        onArrivingChanged: Visibilities.streamClaim = incoming.arriving ? Visibilities.dragAddress : ""

        Behavior on opacity {
            Anim {
                type: Anim.DefaultEffects
            }
        }

        StyledClippingRect {
            anchors.fill: parent
            color: Colours.palette.m3surfaceContainer
            radius: Tokens.rounding.large

            WindowPreview {
                active: incoming.arriving
                address: Visibilities.dragAddress
                anchors.fill: parent
                fallbackIcon: incoming.window ? WinIcons.sourceForClient(incoming.window) : ""
                sourceAspect: incoming.aspect
            }
        }
    }

    Timer {
        id: edgeDwell

        interval: 900
        repeat: true
        running: root.isDragging && root.edgeDirection !== 0
        onTriggered: {
            if (root.edgeDirection < 0 && listView.currentIndex > 0)
                listView.currentIndex -= 1;
            else if (root.edgeDirection > 0 && listView.currentIndex < listView.count - 1)
                listView.currentIndex += 1;
        }
    }
    StyledRect {
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        anchors.leftMargin: Tokens.padding.large
        implicitWidth: prevIcon.implicitWidth + Tokens.padding.large * 2
        implicitHeight: prevIcon.implicitHeight + Tokens.padding.large * 2
        radius: height / 2
        color: Colours.tPalette.m3surfaceContainerHigh
        opacity: hoverPrev.hovered ? 1 : 0.6
        visible: listView.currentIndex > 0

        HoverHandler { id: hoverPrev }
        StateLayer {
            anchors.fill: parent
            radius: parent.radius
            onClicked: listView.currentIndex -= 1
        }
        MaterialIcon {
            id: prevIcon

            anchors.centerIn: parent
            text: "chevron_left"
            color: Colours.palette.m3onSurface
            fontStyle.pointSize: Tokens.font.body.large.pointSize
        }
    }
    StyledRect {
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.rightMargin: Tokens.padding.large
        implicitWidth: nextIcon.implicitWidth + Tokens.padding.large * 2
        implicitHeight: nextIcon.implicitHeight + Tokens.padding.large * 2
        radius: height / 2
        color: Colours.tPalette.m3surfaceContainerHigh
        opacity: hoverNext.hovered ? 1 : 0.6
        visible: listView.currentIndex < listView.count - 1

        HoverHandler { id: hoverNext }
        StateLayer {
            anchors.fill: parent
            radius: parent.radius
            onClicked: listView.currentIndex += 1
        }
        MaterialIcon {
            id: nextIcon

            anchors.centerIn: parent
            text: "chevron_right"
            color: Colours.palette.m3onSurface
            fontStyle.pointSize: Tokens.font.body.large.pointSize
        }
    }
}

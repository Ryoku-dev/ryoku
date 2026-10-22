pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import Caelestia
import Caelestia.Config
import Caelestia.Services
import caelestia.components
import caelestia.components.controls
import caelestia.components.effects
import caelestia.components.images
import caelestia.services
import caelestia.utils

Item {
    id: root

    required property var bar

    property int modelUpdateTrigger: 0

    property var launchingApps: ({})

    property bool isDragging: false

    property real spacing: Tokens.spacing.medium

    property real padding: Tokens.padding.medium

    readonly property real rawScale: !isNaN(Config.bar.scale) ? Config.bar.scale : 1.0
    readonly property real scaleFactor: rawScale < 1.0 ? Math.sqrt(Math.max(0.1, rawScale)) : rawScale
    readonly property int barThickness: Math.round(Tokens.sizes.bar.innerWidth * scaleFactor)
    readonly property int baseThickness: Tokens.sizes.bar.innerWidth
    readonly property int effectiveThickness: rawScale < 1.0 ? baseThickness : barThickness
    readonly property real configuredItemSize: Math.max(16, Math.min(effectiveThickness, Config.bar.dock.iconSize || 32))
    // Icon size of a dock item. Derived from the configured item size rather than the
    // delegate's laid-out width, which is still 0 while the delegate is being created.
    readonly property real iconSize: Math.round(configuredItemSize * 0.7 / 2) * 2

    implicitWidth: bar.isHorizontal ? container.width : container.implicitWidth
    implicitHeight: bar.isHorizontal ? container.implicitHeight : container.height

    HoverHandler { id: dockHover }

    ListModel { id: dockModel }

    function activateEntry(entry: var): void {
        if (!entry)
            return;

        if (entry.toplevels.length > 0) {
            let activeIdx = -1;
            let activeAddr = "";

            if (Kwin.activeWindow) {
                activeAddr = Kwin.activeWindow.address ? String(Kwin.activeWindow.address) : "";
            } else if (root.activeTop && root.activeTop.address) {
                activeAddr = String(root.activeTop.address);
            }

            for (let i = 0; i < entry.toplevels.length; i++) {
                let top = entry.toplevels[i];
                let topAddr = String(top.address);
                let isMinimized = top.minimized || false;
                if (!isMinimized && (top.focused || (activeAddr !== "" && activeAddr === topAddr))) {
                    activeIdx = i;
                    break;
                }
            }

            const isKWin = (Kwin.windowList.length > 0);

            if (entry.toplevels.length === 1) {
                let addr = String(entry.toplevels[0].address);
                if (activeIdx !== 0) {
                    if (isKWin) {
                        Kwin.focusWindow(addr);
                    } else {
                        Kwin.dispatch(Kwin.usingLua ? `hl.dsp.focus({ window = "address:0x${addr}" })` : `focuswindow address:0x${addr}`);
                    }
                }
            } else {
                let nextIdx = activeIdx !== -1 ? (activeIdx + 1) % entry.toplevels.length : 0;
                let addr = String(entry.toplevels[nextIdx].address);
                if (isKWin) {
                    Kwin.focusWindow(addr);
                } else {
                    Kwin.dispatch(Kwin.usingLua ? `hl.dsp.focus({ window = "address:0x${addr}" })` : `focuswindow address:0x${addr}`);
                }
            }
        } else if (entry.entry) {
            let newLaunching = Object.assign({}, root.launchingApps);
            newLaunching[entry.appClass || entry.id] = true;
            root.launchingApps = newLaunching;

            Launch.launchEntry(entry.entry);
        }
    }

    function activateIndex(idx: int): void {
        if (idx < 0 || idx >= root.modelDataArray.length)
            return;
        root.activateEntry(root.modelDataArray[idx]);
    }

    function activateNewIndex(idx: int): void {
        if (idx < 0 || idx >= root.modelDataArray.length)
            return;
        const entry = root.modelDataArray[idx];
        if (!entry)
            return;
        if (entry.entry) {
            let newLaunching = Object.assign({}, root.launchingApps);
            newLaunching[entry.appClass || entry.id] = true;
            root.launchingApps = newLaunching;

            Launch.launchEntry(entry.entry);
        } else if (entry.toplevels.length > 0) {
            root.activateEntry(entry);
        }
    }

    function saveNewOrder(): void {
        const newArr = [];
        const newFavs = [];

        for (let i = 0; i < root.currentOrder.length; ++i) {
            const mData = root.currentOrder[i];
            if (!mData) continue;

            if (mData.isPinned) {
                newFavs.push(mData.id);
            }
            newArr.push(mData);
        }

        const currentFavs = GlobalConfig.bar.dock.pinnedApps || [];
        let changed = currentFavs.length !== newFavs.length;
        if (!changed) {
            for (let i = 0; i < newFavs.length; i++) {
                if (currentFavs[i] !== newFavs[i]) {
                    changed = true;
                    break;
                }
            }
        }

        if (changed) {
            GlobalConfig.bar.dock.pinnedApps = newFavs;
        }

        root.modelDataArray = newArr;
        const map = {};
        for (const app of newArr) {
            map[app.id] = app;
        }
        root.modelDataMap = map;
    }

    function handleWheel(angleDelta: point): void {
        scrollByWheel(angleDelta, Qt.point(0, 0));
    }

    function scrollByWheel(angleDelta: point, pixelDelta: point): void {
        if (root.isDragging) return;

        const isH = bar.isHorizontal;
        const fullSpan = Math.max(isH ? listView.contentWidth : listView.contentHeight, container.__computedContentWidth);
        const viewSpan = isH ? listView.width : listView.height;
        const maxScroll = Math.max(0, fullSpan - viewSpan);
        if (maxScroll <= 0) return;

        const rawDelta = isH
            ? (Math.abs(angleDelta.x) > Math.abs(angleDelta.y) ? angleDelta.x : angleDelta.y)
            : (Math.abs(angleDelta.y) > Math.abs(angleDelta.x) ? angleDelta.y : angleDelta.x);

        const rawPixel = isH
            ? (Math.abs(pixelDelta.x) > Math.abs(pixelDelta.y) ? pixelDelta.x : pixelDelta.y)
            : (Math.abs(pixelDelta.y) > Math.abs(pixelDelta.x) ? pixelDelta.y : pixelDelta.x);

        if (rawPixel !== 0) {
            scrollAnim.stop();
            const current = isH ? listView.contentX : listView.contentY;
            const target = Math.max(0, Math.min(maxScroll, current - rawPixel));
            if (isH) listView.contentX = target;
            else listView.contentY = target;
            return;
        }

        if (rawDelta === 0) return;

        const step = container.itemSize + root.spacing;
        const scrollAmount = -(rawDelta / 120) * step;

        const current = scrollAnim.running ? scrollAnim.to : (isH ? listView.contentX : listView.contentY);
        const target = Math.max(0, Math.min(maxScroll, current + scrollAmount));

        scrollAnim.stop();
        scrollAnim.from = isH ? listView.contentX : listView.contentY;
        scrollAnim.to = target;
        scrollAnim.start();
    }

    function clampScroll(): void {
        const isH = bar.isHorizontal;
        const fullSpan = Math.max(isH ? listView.contentWidth : listView.contentHeight, container.__computedContentWidth);
        const viewSpan = isH ? listView.width : listView.height;
        const maxScroll = Math.max(0, fullSpan - viewSpan);
        if (isH) {
            if (listView.contentX > maxScroll)
                listView.contentX = maxScroll;
        } else {
            if (listView.contentY > maxScroll)
                listView.contentY = maxScroll;
        }
    }

    StyledRect {
        id: container

        color: dockModel.count > 0 ? Colours.tPalette.m3surfaceContainer : Qt.alpha(Colours.tPalette.m3surfaceContainer, 0)
        radius: Tokens.rounding.full

        property int __itemCount: dockModel.count

        property real __computedContentWidth: __itemCount > 0 ? __itemCount * itemSize + (__itemCount - 1) * root.spacing : 0

        implicitWidth: bar.isHorizontal ? (__computedContentWidth + padding * 2) : bar.thickness
        implicitHeight: bar.isHorizontal ? bar.thickness : (__computedContentWidth + padding * 2)

        width: bar.isHorizontal ? Math.min(implicitWidth, maxHorizontalSize) : implicitWidth
        height: !bar.isHorizontal ? Math.min(implicitHeight, maxVerticalSize) : implicitHeight

        property string currentZone: {
            if (!bar) return "middle";
            if (bar.leftEntries.some(e => e.id === "dock")) return "left";
            if (bar.rightEntries.some(e => e.id === "dock")) return "right";
            return "middle";
        }

        property real availableSize: {
            if (!bar) return 9999;

            const W = bar.isHorizontal ? bar.width : bar.height;
            const spacing = Tokens.spacing.medium;
            const pad = bar.vPadding;

            let otherSize = 0;
            if (root.parent && root.parent.parent) {
                const layout = root.parent.parent;
                for (let i = 0; i < layout.children.length; i++) {
                    const child = layout.children[i];
                    if (child !== root.parent && child.visible) {
                        otherSize += (bar.isHorizontal ? child.implicitWidth : child.implicitHeight) + spacing;
                    }
                }
            }

            let result = 0;
            if (currentZone === "left") {
                const M = bar.middleZoneSize;
                const R = bar.rightZoneSize;
                let maxZone = W - 2*pad;
                if (M > 0) maxZone = W / 2 - M / 2 - spacing - pad;
                else if (R > 0) maxZone = W - R - spacing - 2*pad;

                result = Math.max(0, maxZone - otherSize);
            } else if (currentZone === "right") {
                const L = bar.leftZoneSize;
                const M = bar.middleZoneSize;
                let maxZone = W - 2*pad;
                if (M > 0) maxZone = W / 2 - M / 2 - spacing - pad;
                else if (L > 0) maxZone = W - L - spacing - 2*pad;

                result = Math.max(0, maxZone - otherSize);
            } else {
                const L = bar.leftZoneSize;
                const R = bar.rightZoneSize;
                let maxZone = W - 2*pad;
                if (L > 0) maxZone -= (L + spacing);
                if (R > 0) maxZone -= (R + spacing);

                result = Math.max(0, maxZone - otherSize);
            }

            return result;
        }

        property real itemSize: root.configuredItemSize

        property int maxHorizontalItems: Math.max(0, Math.floor((availableSize - padding * 2 - itemSize * 0.5) / (itemSize + spacing)))

        property real maxHorizontalSize: maxHorizontalItems >= 1 ? ((maxHorizontalItems + 0.5) * itemSize + maxHorizontalItems * spacing + padding * 2) : availableSize

        property int maxVerticalItems: Math.max(0, Math.floor((availableSize - padding * 2 - itemSize * 0.5) / (itemSize + spacing)))

        property real maxVerticalSize: maxVerticalItems >= 1 ? ((maxVerticalItems + 0.5) * itemSize + maxVerticalItems * spacing + padding * 2) : availableSize

        property var _appsValues: DesktopEntries.applications.values
        on_AppsValuesChanged: root.rebuildModel()

        MouseArea {
            anchors.fill: parent
            acceptedButtons: Qt.RightButton
            enabled: dockModel.count > 0

            onClicked: mouse => {
                const popouts = root.bar?.popouts;
                if (!popouts)
                    return;
                popouts.currentName = "dockbgcontext";
                popouts.currentCenter = bar.isHorizontal ? container.mapToItem(null, container.width / 2, 0).x : (container.mapToItem(null, 0, container.height / 2).y ?? 0);
                popouts.hasCurrent = true;
                mouse.accepted = true;
            }
        }

        Item {
            id: layout

            anchors.fill: parent

            WheelHandler {
                id: wheelHandler

                target: null
                orientation: Qt.Vertical | Qt.Horizontal

                onWheel: event => {
                    root.scrollByWheel(event.angleDelta, event.pixelDelta);
                }
            }

            NumberAnimation {
                id: scrollAnim

                target: listView
                property: bar.isHorizontal ? "contentX" : "contentY"
                duration: 180
                easing.type: Easing.OutCubic
            }

            ListView {
                id: listView

                anchors.centerIn: parent
                width: bar.isHorizontal ? (container.width - padding * 2) : container.itemSize
                height: bar.isHorizontal ? container.itemSize : (container.height - padding * 2)
                orientation: bar.isHorizontal ? ListView.Horizontal : ListView.Vertical
                spacing: root.spacing
                interactive: bar.isHorizontal ? contentWidth > width + 1 : contentHeight > height + 1
                clip: true

                onContentWidthChanged: root.clampScroll()
                onWidthChanged: root.clampScroll()
                onContentHeightChanged: root.clampScroll()
                onHeightChanged: root.clampScroll()

                add: Transition {
                    NumberAnimation { property: "scale"; from: 0; to: 1; duration: 250; easing.type: Easing.OutBack }
                }
                remove: Transition {
                    NumberAnimation { property: "scale"; from: 1; to: 0; duration: 250; easing.type: Easing.InBack }
                }

                move: Transition {
                    NumberAnimation { properties: "x,y"; duration: 250; easing.type: Easing.OutCubic }
                }
                moveDisplaced: Transition {
                    NumberAnimation { properties: "x,y"; duration: 250; easing.type: Easing.OutCubic }
                }

                model: DelegateModel {
                    id: visualModel

                    model: dockModel
                    delegate: dockDelegate
                }
            }

            StyledScrollBar {
                flickable: listView
                orientation: Qt.Horizontal
                size: listView.visibleArea.widthRatio
                position: listView.visibleArea.xPosition
                shouldBeActive: dockHover.hovered || listView.moving || scrollAnim.running
                anchors.left: listView.left
                anchors.right: listView.right
                anchors.bottom: listView.bottom
                anchors.bottomMargin: -root.padding + 2
                visible: bar.isHorizontal && listView.contentWidth > listView.width + 1
            }

            StyledScrollBar {
                flickable: listView
                orientation: Qt.Vertical
                size: listView.visibleArea.heightRatio
                position: listView.visibleArea.yPosition
                shouldBeActive: dockHover.hovered || listView.moving || scrollAnim.running
                anchors.top: listView.top
                anchors.bottom: listView.bottom
                anchors.right: listView.right
                anchors.rightMargin: -root.padding + 2
                visible: !bar.isHorizontal && listView.contentHeight > listView.height + 1
            }
        }

        Component {
            id: dockDelegate

            Item {
                id: delegateContainer

                required property int index
                required property string appId

                width: container.itemSize
                height: container.itemSize
                implicitWidth: width
                implicitHeight: height

                property var modelData: root.modelDataMap[appId] || root.modelDataArray[index]

                DropArea {
                    anchors.fill: parent
                    anchors.margins: Tokens.padding.small
                    onEntered: drag => {
                        const from = drag.source.delegateIndex;
                        const to = delegateContainer.index;
                        if (from !== undefined && to !== undefined && from !== to) {
                            dockModel.move(from, to, 1);
                            const movedItem = root.currentOrder.splice(from, 1)[0];
                            root.currentOrder.splice(to, 0, movedItem);
                        }
                    }
                    onDropped: drag => {
                        root.saveNewOrder();
                    }
                }

                Item {
                    id: delegateItem

                    width: delegateContainer.width
                    height: delegateContainer.height

                    property int delegateIndex: delegateContainer.index

                    Drag.active: dragArea.held
                    Drag.source: delegateItem
                    Drag.hotSpot.x: width / 2
                    Drag.hotSpot.y: height / 2
                    StyledRect {
                        anchors.fill: parent
                        radius: Tokens.rounding.medium
                        color: Colours.palette.m3onSurface
                        opacity: delegateItem.isActive ? 0.1 : 0

                        Behavior on opacity {
                            Anim {
                                type: Anim.DefaultEffects
                            }
                        }
                    }

                    StyledRect {
                        anchors.fill: parent
                        radius: Tokens.rounding.medium
                        color: Colours.palette.m3error
                        opacity: (delegateItem.badge?.urgent ?? false) ? 0.35 : 0

                        Behavior on opacity {
                            Anim {
                                type: Anim.DefaultEffects
                            }
                        }
                    }

                    StateLayer {
                        id: stateLayer

                        anchors.fill: parent
                        radius: Tokens.rounding.medium
                        acceptedButtons: Qt.NoButton

                        onEntered: {
                            if (bar.popouts.hasCurrent && bar.popouts.currentName === "dockcontext") return;
                            bar.popouts.currentName = "dockhover";
                            bar.popouts.currentCenter = bar.isHorizontal ? delegateItem.mapToItem(null, delegateItem.width / 2, 0).x : (delegateItem.mapToItem(null, 0, delegateItem.height / 2).y ?? 0);
                            bar.popouts.dockModel = modelData;
                            bar.popouts.hasCurrent = true;
                        }
                    }

                    MouseArea {
                        id: dragArea

                        property bool held: false

                        anchors.fill: parent
                        drag.target: held ? delegateItem : null
                        drag.axis: bar.isHorizontal ? Drag.XAxis : Drag.YAxis
                        acceptedButtons: Qt.LeftButton | Qt.RightButton
                        cursorShape: Qt.PointingHandCursor

                        onPressed: mouse => {
                            held = true;
                            root.isDragging = true;
                            stateLayer.press(mouse.x, mouse.y);
                        }

                        onClicked: mouse => {
                            if (mouse.button === Qt.LeftButton) {
                                if (modelData.isPinned) {
                                    bounceAnim.start();
                                }

                                root.activateEntry(modelData);
                            } else if (mouse.button === Qt.RightButton) {
                                bar.popouts.currentName = "dockcontext";
                                bar.popouts.currentCenter = bar.isHorizontal ? delegateItem.mapToItem(null, delegateItem.width / 2, 0).x : (delegateItem.mapToItem(null, 0, delegateItem.height / 2).y ?? 0);
                                bar.popouts.dockModel = modelData;
                                bar.popouts.hasCurrent = true;
                            }
                        }

                        onReleased: {
                            held = false;
                            root.isDragging = false;
                            delegateItem.x = 0;
                            delegateItem.y = 0;
                            root.saveNewOrder();
                        }

                        onCanceled: {
                            held = false;
                            root.isDragging = false;
                            delegateItem.x = 0;
                            delegateItem.y = 0;
                        }
                    }

                    states: [
                        State {
                            when: dragArea.held

                            ParentChange {
                                target: delegateItem
                                parent: listView
                            }
                            PropertyChanges {
                                target: delegateItem
                                opacity: 0.8
                                z: 999
                            }
                        }
                    ]

                    property bool isActive: {
                        const dummy = root.modelUpdateTrigger;
                        if (!modelData) return false;
                        for (const top of modelData.toplevels) {
                            if (top.focused || (root.activeTop && root.activeTop.address === top.address)) return true;
                        }
                        return false;
                    }

                    property bool hasWindows: {
                        const dummy = root.modelUpdateTrigger;
                        if (!modelData) return false;
                        return modelData.toplevels.length > 0;
                    }

                    readonly property var badge: {
                        const dummy = LauncherEntry.revision;
                        if (!modelData || !(Config.bar.dock.showBadges ?? true))
                            return null;
                        return LauncherEntry.forApp(modelData.id);
                    }



                    CachingIconImage {
                        id: icon

                        anchors.centerIn: parent
                        implicitSize: root.iconSize
                        source: modelData ? WinIcons.sourceFor(modelData.entry, modelData.appClass, modelData.iconName, modelData.pid ?? 0) : ""
                        visible: !(Config.bar.dock.recolourIcons ?? false)

                        SequentialAnimation {
                            id: bounceAnim

                            NumberAnimation { target: delegateItem; property: "scale"; to: 0.7; duration: 100; easing.type: Easing.OutQuad }
                            NumberAnimation { target: delegateItem; property: "scale"; to: 1.0; duration: 400; easing.type: Easing.OutElastic }
                        }
                    }

                    ColouredIcon {
                        anchors.centerIn: parent
                        implicitSize: root.iconSize
                        source: icon.source
                        colour: Colours.palette.m3secondary
                        layer.enabled: true
                        visible: Config.bar.dock.recolourIcons ?? false
                    }

                    Loader {
                        anchors.fill: icon
                        active: modelData ? (root.launchingApps[modelData.appClass || modelData.id] || false) : false
                        sourceComponent: CircularIndicator {
                            running: true
                            strokeWidth: 2
                        }
                    }

                    StyledRect {
                        id: countBadge

                        readonly property int badgeHeight: Math.max(12, Math.round(root.iconSize * 0.55))
                        readonly property int dotSize: Math.max(6, Math.round(badgeHeight * 0.75))
                        readonly property bool asDot: (delegateItem.badge?.count ?? 0) <= 0

                        visible: delegateItem.badge?.countVisible ?? false
                        color: Colours.palette.m3error
                        radius: Tokens.rounding.full
                        width: asDot ? dotSize : Math.max(badgeHeight, badgeLabel.implicitWidth + Tokens.padding.extraSmall)
                        height: asDot ? dotSize : badgeHeight
                        anchors.right: icon.right
                        anchors.top: icon.top
                        anchors.rightMargin: -Math.round(height * 0.25)
                        anchors.topMargin: -Math.round(height * 0.25)

                        Text {
                            id: badgeLabel

                            anchors.centerIn: parent
                            visible: !countBadge.asDot
                            text: {
                                const count = delegateItem.badge?.count ?? 0;
                                if (count > 9999)
                                    return "9k+";
                                if (count > 999)
                                    return `${Math.floor(count / 1000)}k`;
                                return `${count}`;
                            }
                            color: Colours.palette.m3onError
                            font: Tokens.font.label.builders.small.size(Math.max(6, Math.round(countBadge.badgeHeight * 0.62))).weight(Font.DemiBold).build()
                        }
                    }

                    StyledRect {
                        id: progressBar

                        visible: delegateItem.badge?.progressVisible ?? false
                        anchors.top: icon.bottom
                        anchors.topMargin: 1
                        anchors.horizontalCenter: parent.horizontalCenter
                        width: root.iconSize
                        height: 3
                        radius: Tokens.rounding.full
                        color: Qt.alpha(Colours.palette.m3onSurface, 0.15)

                        StyledRect {
                            width: Math.round(parent.width * (delegateItem.badge?.progress ?? 0))
                            height: parent.height
                            radius: parent.radius
                            color: Colours.palette.m3primary
                        }
                    }

                    ListView {
                        anchors.bottom: parent.bottom
                        anchors.horizontalCenter: parent.horizontalCenter
                        anchors.bottomMargin: 0
                        spacing: 2
                        orientation: ListView.Horizontal
                        interactive: false

                        height: 2
                        width: contentWidth

                        remove: Transition {
                            NumberAnimation { property: "scale"; from: 1; to: 0; duration: 250; easing.type: Easing.InBack }
                            NumberAnimation { property: "y"; from: 0; to: -15; duration: 250; easing.type: Easing.InBack }
                        }
                        addDisplaced: Transition {
                            NumberAnimation { properties: "x,y"; duration: 250; easing.type: Easing.OutCubic }
                        }
                        removeDisplaced: Transition {
                            NumberAnimation { properties: "x,y"; duration: 250; easing.type: Easing.OutCubic }
                        }

                        model: {
                            const dummy = root.modelUpdateTrigger;
                            if (!modelData) return 0;
                            return Math.min(2, modelData.toplevels.length);
                        }

                        delegate: Rectangle {
                            required property int index

                            width: (index === 0 && delegateItem.isActive) ? 16 : 2

                                height: 2

                                radius: 1

                                color: delegateItem.isActive ? Colours.palette.m3primary : Colours.palette.m3onSurface

                                scale: 0
                                y: -15
                                Component.onCompleted: {
                                    scale = 1;
                                    y = 0;
                                }

                                Behavior on width { NumberAnimation { duration: 250; easing.type: Easing.OutCubic } }
                                Behavior on height { NumberAnimation { duration: 250; easing.type: Easing.OutCubic } }
                                Behavior on color { ColorAnimation { duration: 250 } }
                                Behavior on scale { NumberAnimation { duration: 250; easing.type: Easing.OutBack } }
                                Behavior on y { NumberAnimation { duration: 250; easing.type: Easing.OutBack } }
                            }
                        }
                }
            }
        }
    }

    function handleHover(relPos: real, isHorizontal: bool): void {
        if (bar.popouts.hasCurrent && bar.popouts.currentName === "dockcontext") return;

        const itemSize = container.itemSize;
        const itemWidthWithSpacing = itemSize + spacing;
        const adjustedPos = isHorizontal ? relPos - container.x - padding : relPos - container.y - padding;
        const scrolled = isHorizontal ? listView.contentX : listView.contentY;
        const visibleSpan = isHorizontal ? listView.width : listView.height;

        if (adjustedPos < 0 || adjustedPos > visibleSpan) {
            bar.popouts.hasCurrent = false;
            return;
        }

        const index = Math.floor((adjustedPos + scrolled) / itemWidthWithSpacing);

        if (index >= 0 && index < modelDataArray.length) {
            bar.popouts.currentName = "dockhover";
            const centerOffset = index * itemWidthWithSpacing + itemSize / 2 - scrolled;
            if (centerOffset < 0 || centerOffset > visibleSpan) {
                bar.popouts.hasCurrent = false;
                return;
            }
            const absoluteCenter = isHorizontal
                ? container.mapToItem(null, padding + centerOffset, 0).x
                : container.mapToItem(null, 0, padding + centerOffset).y;

            bar.popouts.currentCenter = absoluteCenter;
            bar.popouts.dockModel = modelDataArray[index];
            bar.popouts.hasCurrent = true;
        }
    }

    property var modelDataArray: []

    property var modelDataMap: ({})

    property var currentOrder: []

    onModelDataArrayChanged: currentOrder = [...modelDataArray]

    function rebuildModel(): void {
        if (root.isDragging) return;
        let apps = [];

        const pinnedIds = GlobalConfig.bar.dock.pinnedApps || [];

        for (const pid of pinnedIds) {
            for (const entry of DesktopEntries.applications.values) {
                if (Strings.testRegexList([pid], entry.id)) {
                    if (!apps.some(a => a.id === entry.id)) {
                        apps.push({
                            id: entry.id,
                            isPinned: true,
                            entry: entry,
                            toplevels: [],
                            appClass: entry.id.replace(".desktop", ""),
                            iconName: entry.id,
                            pid: 0
                        });
                    }
                }
            }
        }

        for (const toplevel of root._toplevels) {
            const ipc = toplevel;
            if (!ipc) continue;
            const appClass = ipc.class || ipc.initialClass;
            if (!appClass) continue;

            if (appClass.toLowerCase().includes("xwaylandvideobridge")) continue;

            // Ungrouped keeps every window on its own icon (pinned entries stay
            // launchers); PinnedFirst caps a pinned entry at its first window and
            // lets the rest stand alone. Per-window ids carry the address (or pid)
            // so they stay stable for the window's lifetime.
            const grouping = GlobalConfig.bar.dock.windowGrouping;
            const ungrouped = grouping === DockWindowGrouping.Ungrouped;
            const pinnedFirst = grouping === DockWindowGrouping.PinnedFirst;
            let found = false;
            let pinnedCapped = false;
            if (!ungrouped) {
                for (const app of apps) {
                    const isToplevelSteamGame = appClass.toLowerCase().startsWith("steam_app_");

                    if (isToplevelSteamGame) {
                        if (app.appClass.toLowerCase() === appClass.toLowerCase()) {
                            if (pinnedFirst && app.isPinned && app.toplevels.length > 0)
                                pinnedCapped = true;
                            else {
                                app.toplevels.push(toplevel);
                                found = true;
                            }
                            break;
                        }
                    } else {
                        const isAppSteamGame = app.id.toLowerCase().startsWith("steam_app_") || app.appClass.toLowerCase().startsWith("steam_app_");
                        if (isAppSteamGame) continue;

                        const baseId = app.id.toLowerCase().replace(".desktop", "");
                        if (app.appClass.toLowerCase() === appClass.toLowerCase() ||
                            app.id.toLowerCase().includes(appClass.toLowerCase()) ||
                            appClass.toLowerCase().includes(baseId)) {
                            if (pinnedFirst && app.isPinned && app.toplevels.length > 0)
                                pinnedCapped = true;
                            else {
                                app.toplevels.push(toplevel);
                                found = true;
                            }
                            break;
                        }
                    }
                }
            }

            if (!found) {
                const isToplevelSteamGame = appClass.toLowerCase().startsWith("steam_app_");
                let entry = null;
                let iconName = appClass;

                if (isToplevelSteamGame) {
                    const appId = appClass.substring(10);
                    iconName = `steam_icon_${appId}`;
                    entry = DesktopEntries.applications.values.find(e => e.id.toLowerCase() === `steam_app_${appId}.desktop` || e.id.toLowerCase() === `steam-${appId}.desktop`) || null;
                } else {
                    entry = DesktopEntries.heuristicLookup(appClass) || null;
                    if (!entry) {
                        entry = DesktopEntries.applications.values.find(e => {
                            const eBase = e.id.toLowerCase().replace(".desktop", "");
                            return e.id.toLowerCase().includes(appClass.toLowerCase()) || appClass.toLowerCase().includes(eBase);
                        }) || null;
                    }
                    if (entry)
                        iconName = entry.id;
                    else
                        iconName = appClass.toLowerCase().split(/[^a-z0-9]/)[0] || appClass;
                }

                const pid = ipc.pid || 0;
                if (!entry)
                    WinIcons.request(appClass, ipc.title || "", pid, ipc.address ? String(ipc.address) : "");

                apps.push({
                    id: ungrouped || pinnedCapped ? `${appClass}#${ipc.address || pid}` : appClass,
                    isPinned: false,
                    entry: entry,
                    toplevels: [toplevel],
                    appClass: appClass,
                    iconName: iconName,
                    pid: pid
                });
            }
        }

        let newLaunching = Object.assign({}, root.launchingApps);
        let launchingChanged = false;

        for (const app of apps) {
            if (app.toplevels.length > 0) {
                if (newLaunching[app.appClass]) {
                    delete newLaunching[app.appClass];
                    launchingChanged = true;
                }
                if (newLaunching[app.id]) {
                    delete newLaunching[app.id];
                    launchingChanged = true;
                }
            }
        }

        if (launchingChanged) {
            root.launchingApps = newLaunching;
        }

        const existingOrder = (root.currentOrder && root.currentOrder.length > 0) ? root.currentOrder : root.modelDataArray;
        const existingPinnedOrder = existingOrder.filter(a => a && a.isPinned).map(a => a.id);
        const pinnedOrderMatches = existingPinnedOrder.length === pinnedIds.length &&
            existingPinnedOrder.every((id, idx) => id === pinnedIds[idx]);

        if (existingOrder.length > 0 && pinnedOrderMatches) {
            const orderedApps = [];
            const remainingApps = [...apps];

            for (let i = 0; i < existingOrder.length; i++) {
                const prevItem = existingOrder[i];
                if (!prevItem) continue;
                const idx = remainingApps.findIndex(a => a.id === prevItem.id);
                if (idx !== -1) {
                    orderedApps.push(remainingApps.splice(idx, 1)[0]);
                }
            }

            for (let i = 0; i < remainingApps.length; i++) {
                orderedApps.push(remainingApps[i]);
            }

            apps = orderedApps;
        }

        let changed = false;
        if (apps.length !== dockModel.count) {
            changed = true;
        } else {
            for (let i = 0; i < apps.length; i++) {
                if (apps[i].id !== dockModel.get(i).appId) {
                    changed = true;
                    break;
                }
            }
        }

        if (changed) {
            for (let i = dockModel.count - 1; i >= 0; i--) {
                let found = false;
                for (let j = 0; j < apps.length; j++) {
                    if (apps[j].id === dockModel.get(i).appId) { found = true; break; }
                }
                if (!found) {
                    dockModel.remove(i);
                }
            }

            for (let i = 0; i < apps.length; i++) {
                let found = false;
                for (let j = 0; j < dockModel.count; j++) {
                    if (dockModel.get(j).appId === apps[i].id) { found = true; break; }
                }
                if (!found) {
                    dockModel.append({ appId: apps[i].id });
                }
            }

            for (let i = 0; i < apps.length; i++) {
                let currentId = apps[i].id;
                if (dockModel.get(i).appId !== currentId) {
                    let foundIdx = -1;
                    for (let j = i + 1; j < dockModel.count; j++) {
                        if (dockModel.get(j).appId === currentId) { foundIdx = j; break; }
                    }
                    if (foundIdx !== -1) {
                        dockModel.move(foundIdx, i, 1);
                    }
                }
            }
        }

        const map = {};
        for (const app of apps) {
            map[app.id] = app;
        }
        root.modelDataMap = map;
        root.modelDataArray = apps;
        root.modelUpdateTrigger += 1;
    }

    property var _toplevels: (Config.bar.dock.currentDesktopOnly
        ? Kwin.filterWindows(Kwin.windowList, Kwin.activeWorkspaceFor(bar?.screen?.name), bar?.screen?.name, true)
        : (Kwin.windowList || [])).filter(w => !w.skipTaskbar)

    on_ToplevelsChanged: {
        root.rebuildModel()
        delayedRebuildTimer.restart()
    }

    Timer {
        id: delayedRebuildTimer

        interval: 100
        repeat: false
        onTriggered: root.rebuildModel()
    }

    property var activeTop: (Kwin.activeWindow && Kwin.activeWindow.address) ? Kwin.activeWindow : null

    onActiveTopChanged: {
        root.rebuildModel()
        delayedRebuildTimer.restart()
    }

    Connections {
        target: GlobalConfig.bar.dock

        function onPinnedAppsChanged(): void {
            root.rebuildModel();
        }
    }

    Connections {
        target: bar

        function onIsHorizontalChanged(): void {
            scrollAnim.stop();
        }
    }

    Connections {
        target: Kwin

        function onActiveWsIdChanged(): void {
            if (Config.bar.dock.currentDesktopOnly)
                root.rebuildModel();
        }

        function onActiveByOutputChanged(): void {
            if (Config.bar.dock.currentDesktopOnly)
                root.rebuildModel();
        }
    }

    Connections {
        target: Config.bar.dock

        function onCurrentDesktopOnlyChanged(): void {
            root.rebuildModel();
        }

        function onWindowGroupingChanged(): void {
            root.rebuildModel();
        }
    }

    Component.onCompleted: {
        root.rebuildModel();
        if (root.bar && root.bar.screen)
            Visibilities.registerDock(root.bar.screen, root);
    }

    Component.onDestruction: {
        if (root.bar && root.bar.screen)
            Visibilities.unregisterDock(root.bar.screen);
    }
}

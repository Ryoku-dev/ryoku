pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import "../../kit" as K
import "../../modules/Registry.js" as Registry
import ".." as S
import "." as P

Item {
    id: root

    property var scene: null
    property bool shown: false
    property var args: ({})
    property Item dragHost: null
    property var workingLayout: ({ left: [], center: [], right: [] })
    property bool dragActive: false
    property bool dragReturning: false
    property string dragId: ""
    property string sourceLane: ""
    property int sourceIndex: -1
    property string targetLane: ""
    property int targetIndex: -1
    property real dragVisualX: 0
    property real dragVisualY: 0
    property real dragHomeX: 0
    property real dragHomeY: 0
    property real dragGrabX: 0
    property real dragGrabY: 0

    readonly property var laneNames: ["left", "center", "right", "hidden"]
    readonly property var laneTitles: ({
        left: "Left",
        center: "Centre",
        right: "Right",
        hidden: "Not shown"
    })
    readonly property var draggedEntry: Registry.find(dragId)

    implicitHeight: content.implicitHeight
    enabled: shown

    function copyLayout(layout) {
        const source = layout || {};
        return {
            left: Array.isArray(source.left) ? source.left.slice() : [],
            center: Array.isArray(source.center) ? source.center.slice() : [],
            right: Array.isArray(source.right) ? source.right.slice() : []
        };
    }

    function refreshLayout() {
        root.workingLayout = {
            left: S.Prefs.lane("left"),
            center: S.Prefs.lane("center"),
            right: S.Prefs.lane("right")
        };
    }

    function commit(layout) {
        const next = root.copyLayout(layout);
        root.workingLayout = next;
        S.Prefs.setLayout(root.copyLayout(next));
    }

    function contains(id) {
        return root.workingLayout.left.indexOf(id) >= 0
            || root.workingLayout.center.indexOf(id) >= 0
            || root.workingLayout.right.indexOf(id) >= 0;
    }

    function itemsForLane(lane) {
        if (lane !== "hidden")
            return root.workingLayout[lane] || [];
        return Registry.entries.filter(entry => !root.contains(entry.id))
            .map(entry => entry.id);
    }

    function locked(id) {
        return id === "logo" || id === "voice";
    }

    function addToHome(id) {
        if (root.contains(id) || root.locked(id))
            return;
        const entry = Registry.find(id);
        const lane = entry && entry.home === "left" ? "left" : "right";
        const next = root.copyLayout(root.workingLayout);
        next[lane].push(id);
        root.commit(next);
    }

    function removeFromLane(lane, id) {
        if (lane === "hidden" || root.locked(id))
            return;
        const next = root.copyLayout(root.workingLayout);
        const index = next[lane].indexOf(id);
        if (index < 0)
            return;
        next[lane].splice(index, 1);
        root.commit(next);
    }

    function moveWithin(lane, id, offset) {
        if (lane === "hidden" || root.locked(id))
            return;
        const next = root.copyLayout(root.workingLayout);
        const index = next[lane].indexOf(id);
        const floor = lane === "left" ? 1 : 0;
        const destination = Math.max(floor,
            Math.min(next[lane].length - 1, index + offset));
        if (index < 0 || index === destination)
            return;
        next[lane].splice(index, 1);
        next[lane].splice(destination, 0, id);
        root.commit(next);
    }

    function moveBetween(lane, id, offset) {
        if (root.locked(id))
            return;
        const laneIndex = root.laneNames.indexOf(lane);
        const destinationIndex = laneIndex + offset;
        if (destinationIndex < 0 || destinationIndex >= root.laneNames.length)
            return;
        const destination = root.laneNames[destinationIndex];
        if (lane === "hidden") {
            if (destination !== "hidden") {
                const next = root.copyLayout(root.workingLayout);
                next[destination].push(id);
                root.commit(next);
            }
            return;
        }
        const next = root.copyLayout(root.workingLayout);
        const index = next[lane].indexOf(id);
        if (index < 0)
            return;
        next[lane].splice(index, 1);
        if (destination !== "hidden")
            next[destination].push(id);
        root.commit(next);
    }

    function handleKey(lane, id, event) {
        if (event.key === Qt.Key_Delete || event.key === Qt.Key_Backspace) {
            root.removeFromLane(lane, id);
            event.accepted = true;
        } else if (event.key === Qt.Key_Left) {
            root.moveWithin(lane, id, -1);
            event.accepted = true;
        } else if (event.key === Qt.Key_Right) {
            if (lane === "hidden")
                root.addToHome(id);
            else
                root.moveWithin(lane, id, 1);
            event.accepted = true;
        } else if (event.key === Qt.Key_Up) {
            root.moveBetween(lane, id, -1);
            event.accepted = true;
        } else if (event.key === Qt.Key_Down) {
            root.moveBetween(lane, id, 1);
            event.accepted = true;
        } else if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter
                    || event.key === Qt.Key_Space) && lane === "hidden") {
            root.addToHome(id);
            event.accepted = true;
        }
    }

    function insertionIndex(panel, pointerX, pointerY) {
        let position = 0;
        for (let index = 0; index < panel.chipRepeater.count; ++index) {
            const chip = panel.chipRepeater.itemAt(index);
            if (!chip || chip.moduleId === root.dragId)
                continue;
            const topLeft = chip.mapToItem(root, 0, 0);
            const bottom = topLeft.y + chip.height;
            if (pointerY < topLeft.y - K.Style.s1)
                return position;
            if (pointerY <= bottom + K.Style.s1
                    && pointerX < topLeft.x + chip.naturalWidth / 2)
                return position;
            ++position;
        }
        return position;
    }

    function updateTarget(pointerX, pointerY) {
        root.targetLane = "";
        root.targetIndex = -1;
        for (let index = 0; index < laneRepeater.count; ++index) {
            const panel = laneRepeater.itemAt(index);
            if (!panel)
                continue;
            const local = panel.dropZone.mapFromItem(root, pointerX, pointerY);
            if (local.x < 0 || local.x > panel.dropZone.width
                    || local.y < 0 || local.y > panel.dropZone.height)
                continue;
            root.targetLane = panel.lane;
            const insertion = root.insertionIndex(panel, pointerX, pointerY);
            root.targetIndex = panel.lane === "left" || panel.lane === "center"
                ? Math.max(1, insertion) : insertion;
            return;
        }
    }

    function beginDrag(chip, lane, id, index, pointerX, pointerY) {
        if (root.dragActive || root.locked(id))
            return;
        const host = root.dragHost || root;
        const home = chip.mapToItem(host, 0, 0);
        const pointer = root.mapToItem(host, pointerX, pointerY);
        root.dragId = id;
        root.sourceLane = lane;
        root.sourceIndex = index;
        root.dragHomeX = home.x;
        root.dragHomeY = home.y;
        root.dragVisualX = home.x;
        root.dragVisualY = home.y;
        root.dragGrabX = pointer.x - home.x;
        root.dragGrabY = pointer.y - home.y;
        root.dragReturning = false;
        root.dragActive = true;
        root.updateTarget(pointerX, pointerY);
    }

    function updateDrag(pointerX, pointerY) {
        if (!root.dragActive || root.dragReturning)
            return;
        const host = root.dragHost || root;
        const pointer = root.mapToItem(host, pointerX, pointerY);
        root.dragVisualX = pointer.x - root.dragGrabX;
        root.dragVisualY = pointer.y - root.dragGrabY;
        root.updateTarget(pointerX, pointerY);
    }

    function applyDrop() {
        const next = root.copyLayout(root.workingLayout);
        for (let index = 0; index < 3; ++index) {
            const lane = root.laneNames[index];
            const at = next[lane].indexOf(root.dragId);
            if (at >= 0)
                next[lane].splice(at, 1);
        }
        if (root.targetLane !== "hidden") {
            const destination = next[root.targetLane];
            const index = Math.max(0, Math.min(destination.length, root.targetIndex));
            destination.splice(index, 0, root.dragId);
        }
        if (!(root.sourceLane === "hidden" && root.targetLane === "hidden"))
            root.commit(next);
    }

    function clearDrag() {
        root.dragActive = false;
        root.dragReturning = false;
        root.dragId = "";
        root.sourceLane = "";
        root.sourceIndex = -1;
        root.targetLane = "";
        root.targetIndex = -1;
    }

    function finishDrag() {
        if (!root.dragActive || root.dragReturning)
            return;
        if (root.targetLane.length > 0) {
            root.applyDrop();
            root.clearDrag();
            return;
        }
        root.dragReturning = true;
        flyBackX.from = root.dragVisualX;
        flyBackX.to = root.dragHomeX;
        flyBackY.from = root.dragVisualY;
        flyBackY.to = root.dragHomeY;
        flyBack.restart();
    }

    Component.onCompleted: refreshLayout()

    Connections {
        target: S.Prefs
        function onLayoutChanged() { root.refreshLayout(); }
    }

    Column {
        id: content

        width: root.width
        spacing: K.Style.s5

        P.PageHead {
            width: parent.width
            title: "Layout"
            description: "Arrange modules across the bar or return them to the shelf."
        }

        Item {
            width: parent.width
            height: resetButton.implicitHeight

            K.Button {
                id: resetButton
                anchors.right: parent.right
                text: "RESET LAYOUT"
                enabled: !root.dragActive
                onClicked: root.commit(Registry.defaultLayout)
            }
        }

        SettingCard {
            width: parent.width
            title: "MODULE LANES"
            collapsible: false

            Item {
                width: parent.width
                height: laneColumn.implicitHeight + K.Style.s4 * 2

                Column {
                    id: laneColumn

                    x: K.Style.s4
                    y: K.Style.s4
                    width: parent.width - K.Style.s4 * 2
                    spacing: K.Style.s3

                    Repeater {
                        id: laneRepeater
                        model: root.laneNames

                        delegate: Item {
                            id: lanePanel

                            required property string modelData
                            required property int index
                            property alias dropZone: laneBox
                            property alias chipRepeater: chips
                            readonly property string lane: modelData
                            readonly property var modules: root.itemsForLane(lane)
                            readonly property int eligibleCount: modules.length
                                - (modules.indexOf(root.dragId) >= 0 ? 1 : 0)

                            function positionWithoutDrag(rawIndex) {
                                let position = 0;
                                for (let index = 0; index < rawIndex; ++index) {
                                    if (modules[index] !== root.dragId)
                                        ++position;
                                }
                                return position;
                            }

                            width: laneColumn.width
                            height: laneLabel.implicitHeight + K.Style.s2 + laneBox.height

                            K.Label {
                                id: laneLabel
                                anchors.left: parent.left
                                anchors.top: parent.top
                                text: root.laneTitles[lanePanel.lane]
                                role: "micro"
                                tone: "faint"
                            }

                            Rectangle {
                                id: laneBox

                                anchors.left: parent.left
                                anchors.right: parent.right
                                anchors.top: laneLabel.bottom
                                anchors.topMargin: K.Style.s2
                                height: Math.max(56, moduleFlow.implicitHeight + K.Style.s3 * 2)
                                radius: K.Style.radius
                                color: root.dragActive && root.targetLane === lanePanel.lane
                                    ? K.Style.tint5 : "transparent"
                                border.width: 1
                                border.color: root.dragActive
                                    && root.targetLane === lanePanel.lane
                                    ? K.Style.lineStrong : K.Style.lineSoft

                                Behavior on color {
                                    enabled: !K.Style.still
                                    ColorAnimation { duration: K.Style.snap }
                                }
                                Behavior on border.color {
                                    enabled: !K.Style.still
                                    ColorAnimation { duration: K.Style.snap }
                                }

                                Flow {
                                    id: moduleFlow

                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.top: parent.top
                                    anchors.margins: K.Style.s3
                                    spacing: K.Style.s2

                                    Repeater {
                                        id: chips
                                        model: lanePanel.modules

                                        delegate: Item {
                                            id: moduleChip

                                            required property string modelData
                                            required property int index
                                            property bool movedDuringPress: false
                                            property real pressX: 0
                                            property real pressY: 0
                                            readonly property string moduleId: modelData
                                            readonly property var entry: Registry.find(moduleId)
                                            readonly property bool isLocked: root.locked(moduleId)
                                            readonly property bool isHidden: lanePanel.lane === "hidden"
                                            readonly property bool draggedSelf: root.dragActive
                                                && root.dragId === moduleId
                                            readonly property int filteredIndex:
                                                lanePanel.positionWithoutDrag(index)
                                            readonly property bool gapBefore: root.dragActive
                                                && root.targetLane === lanePanel.lane
                                                && root.targetIndex === filteredIndex
                                                && !draggedSelf
                                            readonly property real naturalWidth:
                                                chipContent.implicitWidth + K.Style.s3 * 2
                                            readonly property real gapWidth: gapBefore ? 40 : 0

                                            width: draggedSelf ? 1 : naturalWidth + gapWidth
                                            height: 32
                                            opacity: draggedSelf ? 0 : 1
                                            activeFocusOnTab: true
                                            Keys.onPressed: event => root.handleKey(
                                                lanePanel.lane, moduleId, event)

                                            Behavior on width {
                                                enabled: !K.Style.still
                                                NumberAnimation {
                                                    duration: K.Style.ms(160)
                                                    easing.type: Easing.BezierSpline
                                                    easing.bezierCurve: K.Style.spatialFastCurve
                                                }
                                            }

                                            Rectangle {
                                                id: chipBody

                                                x: moduleChip.gapWidth
                                                width: moduleChip.naturalWidth
                                                height: 32
                                                radius: K.Style.radius
                                                color: chipHover.hovered
                                                    ? K.Style.tint5 : "transparent"
                                                border.width: 1
                                                border.color: moduleChip.activeFocus
                                                    ? K.Style.lineStrong : K.Style.line

                                                Row {
                                                    id: chipContent

                                                    anchors.centerIn: parent
                                                    spacing: K.Style.s2

                                                    K.Glyph {
                                                        name: moduleChip.entry
                                                            ? moduleChip.entry.icon : "widgets"
                                                        size: 16
                                                        tone: "dim"
                                                    }

                                                    K.Label {
                                                        text: moduleChip.entry
                                                            ? moduleChip.entry.label : moduleChip.moduleId
                                                        role: "body"
                                                        tone: "ink"
                                                    }

                                                    K.Glyph {
                                                        visible: moduleChip.isLocked || moduleChip.isHidden
                                                            || chipHover.hovered
                                                        opacity: visible ? 1 : 0
                                                        name: moduleChip.isLocked ? "lock"
                                                            : moduleChip.isHidden ? "add" : "close"
                                                        size: 14
                                                        tone: "faint"
                                                    }
                                                }

                                                HoverHandler { id: chipHover }

                                                MouseArea {
                                                    id: chipPointer

                                                    anchors.fill: parent
                                                    enabled: !root.dragActive
                                                        || root.dragId === moduleChip.moduleId
                                                    hoverEnabled: true
                                                    preventStealing: true
                                                    acceptedButtons: Qt.LeftButton | Qt.RightButton
                                                    cursorShape: moduleChip.isLocked
                                                        ? Qt.ArrowCursor : Qt.OpenHandCursor

                                                    onPressed: mouse => {
                                                        moduleChip.forceActiveFocus();
                                                        moduleChip.movedDuringPress = false;
                                                        const point = moduleChip.mapToItem(root,
                                                            mouse.x + chipBody.x, mouse.y);
                                                        moduleChip.pressX = point.x;
                                                        moduleChip.pressY = point.y;
                                                    }
                                                    onPositionChanged: mouse => {
                                                        if (moduleChip.isLocked
                                                                || !(mouse.buttons & Qt.LeftButton))
                                                            return;
                                                        const point = moduleChip.mapToItem(root,
                                                            mouse.x + chipBody.x, mouse.y);
                                                        if (!root.dragActive) {
                                                            const dx = point.x - moduleChip.pressX;
                                                            const dy = point.y - moduleChip.pressY;
                                                            if (Math.sqrt(dx * dx + dy * dy) < 6)
                                                                return;
                                                            moduleChip.movedDuringPress = true;
                                                            root.beginDrag(moduleChip, lanePanel.lane,
                                                                moduleChip.moduleId, moduleChip.index,
                                                                point.x, point.y);
                                                        }
                                                        if (root.dragId === moduleChip.moduleId)
                                                            root.updateDrag(point.x, point.y);
                                                    }
                                                    onReleased: {
                                                        if (root.dragActive
                                                                && root.dragId === moduleChip.moduleId)
                                                            root.finishDrag();
                                                    }
                                                    onCanceled: {
                                                        if (root.dragActive
                                                                && root.dragId === moduleChip.moduleId) {
                                                            root.targetLane = "";
                                                            root.finishDrag();
                                                        }
                                                    }
                                                    onClicked: mouse => {
                                                        if (moduleChip.movedDuringPress)
                                                            return;
                                                        if (mouse.button === Qt.RightButton)
                                                            root.removeFromLane(lanePanel.lane,
                                                                moduleChip.moduleId);
                                                        else if (moduleChip.isHidden)
                                                            root.addToHome(moduleChip.moduleId);
                                                    }
                                                }

                                                MouseArea {
                                                    anchors.right: parent.right
                                                    anchors.verticalCenter: parent.verticalCenter
                                                    width: 30
                                                    height: parent.height
                                                    visible: chipHover.hovered && !moduleChip.isLocked
                                                        && !moduleChip.isHidden && !root.dragActive
                                                    acceptedButtons: Qt.LeftButton
                                                    cursorShape: Qt.PointingHandCursor
                                                    onClicked: root.removeFromLane(lanePanel.lane,
                                                        moduleChip.moduleId)
                                                }
                                            }
                                        }
                                    }

                                    Item {
                                        readonly property bool open: root.dragActive
                                            && root.targetLane === lanePanel.lane
                                            && root.targetIndex >= lanePanel.eligibleCount
                                        visible: open || width > 0.5
                                        width: open ? 32 : 0
                                        height: 32

                                        Rectangle {
                                            anchors.fill: parent
                                            radius: K.Style.radius
                                            color: K.Style.tint5
                                            border.width: 1
                                            border.color: K.Style.lineStrong
                                        }

                                        Behavior on width {
                                            enabled: !K.Style.still
                                            NumberAnimation {
                                                duration: K.Style.ms(160)
                                                easing.type: Easing.BezierSpline
                                                easing.bezierCurve: K.Style.spatialFastCurve
                                            }
                                        }
                                    }

                                    K.Label {
                                        visible: lanePanel.modules.length === 0 && !root.dragActive
                                        text: lanePanel.lane === "hidden"
                                            ? "Every module is shown" : "Drop a module here"
                                        role: "caption"
                                        tone: "faint"
                                        height: 32
                                        verticalAlignment: Text.AlignVCenter
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }

        K.Label {
            width: parent.width
            text: "Drag modules between lanes. Right-click one to remove it."
            color: K.Style.inkFaint
            font.family: K.Style.font
            font.pixelSize: 12
        }
    }

    Item {
        id: dragVisual

        parent: root.dragHost || root
        z: 1000
        visible: root.dragActive
        x: root.dragVisualX
        y: root.dragVisualY
        width: dragVisualContent.implicitWidth + K.Style.s3 * 2
        height: 32
        scale: root.dragReturning ? 1 : 1.05

        Rectangle {
            anchors.fill: parent
            radius: K.Style.radius
            color: K.Style.bone
            border.width: 1
            border.color: K.Style.bone
        }

        Row {
            id: dragVisualContent

            anchors.centerIn: parent
            spacing: K.Style.s2

            K.Glyph {
                name: root.draggedEntry ? root.draggedEntry.icon : "widgets"
                size: 16
                tone: "onBone"
            }

            K.Label {
                text: root.draggedEntry ? root.draggedEntry.label : root.dragId
                role: "bodyStrong"
                tone: "onBone"
            }
        }
    }

    ParallelAnimation {
        id: flyBack

        onFinished: root.clearDrag()

        NumberAnimation {
            id: flyBackX
            target: root
            property: "dragVisualX"
            duration: K.Style.ms(250)
            easing.type: Easing.OutCubic
        }

        NumberAnimation {
            id: flyBackY
            target: root
            property: "dragVisualY"
            duration: K.Style.ms(250)
            easing.type: Easing.OutCubic
        }
    }
}

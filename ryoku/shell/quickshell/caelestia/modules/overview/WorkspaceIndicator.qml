pragma ComponentBehavior: Bound

import "components/workspaces" as WsComponents
import QtQuick
import QtQuick.Layouts
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.services

Item {
    id: root

    required property string screenName
    property int count: 0
    property int currentIndex: 0
    readonly property int activeWsId: currentIndex + 1
    property int maxWidth: 1000
    property real baseScale: 1.0
    readonly property real unscaledWidth: (count + 1) * 200 + count * Tokens.spacing.small
    readonly property real scaleFactor: (unscaledWidth * baseScale) > maxWidth ? maxWidth / unscaledWidth : baseScale
    readonly property real requiredWidth: unscaledWidth * scaleFactor
    property real swipeOffset: Kwin.swipeOffsetByOutput?.[screenName] ?? Kwin.swipeOffset
    property bool isSwiping: false
    property var closingWindows: []
    readonly property var occupied: {
        let occ = {};
        for (let i = 1; i <= root.count; ++i) {
            occ[i] = false;
        }
        const kwinList = root.kwinWindowList;
        if (kwinList) {
            for (let i = 0; i < kwinList.length; ++i) {
                const w = kwinList[i];
                if (w.output !== root.screenName)
                    continue;
                if (w.workspace) {
                    const wid = typeof w.workspace.id === "number" ? w.workspace.id : (typeof w.workspace.index === "number" ? w.workspace.index : null);
                    if (wid !== null)
                        occ[wid] = true;
                }
            }
        }
        return occ;
    }
    property var kwinWindowList: Kwin.windowList

    signal workspaceSelected(int index)
    signal workspaceReselected(int index)
    signal createWorkspaceRequest()

    implicitWidth: layout.implicitWidth + Tokens.padding.small
    implicitHeight: layout.implicitHeight + Tokens.padding.small

    onSwipeOffsetChanged: {
        if (swipeOffset !== 0.0) {
            isSwiping = true;
            wsSwipeSettleTimer.stop();
        } else {
            wsSwipeSettleTimer.restart();
        }
    }

    Connections {
        function onWorkspacesChanged() {
            Kwin.refreshWindows();
        }

        target: Kwin
    }
    Timer {
        id: wsSwipeSettleTimer

        interval: 120
        repeat: false
        onTriggered: root.isSwiping = false
    }
    Item {
        anchors.fill: parent

        Loader {
            asynchronous: true
            active: Config.bar.workspaces.occupiedBg
            anchors.fill: parent
            anchors.margins: Tokens.padding.extraSmall
            sourceComponent: WsComponents.OccupiedBg {
                workspaces: workspaces
                occupied: root.occupied
                groupOffset: 0
            }
        }
        Loader {
            asynchronous: true
            anchors.verticalCenter: parent.verticalCenter
            active: true
            sourceComponent: WsComponents.ActiveIndicator {
                activeWsId: root.activeWsId
                workspaces: workspaces
                mask: layout
                screenName: root.screenName
            }
        }
        GridLayout {
            id: layout

            anchors.centerIn: parent
            columns: -1
            rows: 1
            flow: GridLayout.LeftToRight
            columnSpacing: Math.floor(Tokens.spacing.small)
            rowSpacing: Math.floor(Tokens.spacing.small)

            Repeater {
                id: workspaces

                model: root.count

                WsComponents.Workspace {
                    scaleFactor: root.scaleFactor
                    activeWsId: root.activeWsId
                    screenName: root.screenName
                    occupied: root.occupied
                    groupOffset: 0
                    swipeOffset: root.swipeOffset
                    isSwiping: root.isSwiping
                    closingWindows: root.closingWindows
                    onSelected: root.workspaceSelected(ws - 1)
                    onReselected: root.workspaceReselected(ws - 1)
                }
            }
            StyledRect {
                radius: Tokens.rounding.large
                color: "transparent"
                border.color: Colours.tPalette.m3outlineVariant
                border.width: 2

                StyledText {
                    anchors.centerIn: parent
                    text: "+"
                    font.pixelSize: Math.max(12, Math.floor(24 * root.scaleFactor))
                    font.weight: Font.Bold
                    color: Colours.tPalette.m3onSurfaceVariant
                    opacity: 0.5
                }
                StateLayer {
                    anchors.fill: parent
                    radius: parent.radius
                    onClicked: {
                        Kwin.createWorkspace();
                    }
                }
                Layout.alignment: Qt.AlignVCenter
                Layout.preferredWidth: Math.floor(200 * root.scaleFactor)
                Layout.preferredHeight: Math.floor(120 * root.scaleFactor)
            }
        }
        MouseArea {
            anchors.fill: layout
            acceptedButtons: Qt.NoButton
            onWheel: event => {
                if (!Config.bar.scrollActions.workspaces) return;
                
                if (event.angleDelta.y > 0 || event.angleDelta.x > 0) {
                    if (root.currentIndex > 0) {
                        root.workspaceSelected(root.currentIndex - 1);
                    }
                } else if (event.angleDelta.y < 0 || event.angleDelta.x < 0) {
                    if (root.currentIndex < root.count - 1) {
                        root.workspaceSelected(root.currentIndex + 1);
                    }
                }
            }
        }
        DropArea {
            id: workspaceDropArea

            anchors.fill: layout
            anchors.margins: -2000
            onEntered: drag => {
                if (!(drag.source && drag.source.isWorkspace)) {
                    drag.accepted = false;
                }
            }
            onDropped: drop => {
                const sourceItem = drop.source;
                if (sourceItem && sourceItem.isWorkspace) {
                    const fromWs = sourceItem.ws;
                    const itemWidth = 200 * root.scaleFactor;
                    const spacing = typeof Tokens !== "undefined" ? Tokens.spacing.small : 8;
                    
                    const localPoint = layout.mapFromItem(workspaceDropArea, drop.x, drop.y);
                    let insertIndex = Math.floor((localPoint.x + spacing / 2) / (itemWidth + spacing));
                    
                    if (insertIndex < 0) insertIndex = 0;
                    if (insertIndex > root.count - 1) insertIndex = root.count - 1;
                    
                    const toWs = insertIndex + 1;
                    
                    if (fromWs !== toWs) {
                        if (Kwin.windowList.length > 0) {
                            const kwinList = Kwin.windowList;
                            let windowsByWs = {};
                            for (let i = 0; i < kwinList.length; ++i) {
                                let w = kwinList[i];
                                if (w.workspace) {
                                    let wid = null;
                                    if (typeof w.workspace.index === "number") wid = w.workspace.index;
                                    else if (typeof w.workspace.id === "number") wid = w.workspace.id;
                                    else if (typeof w.workspace.id === "string") {
                                        for (let k = 0; k < Kwin.workspaces.length; ++k) {
                                            if (Kwin.workspaces[k].id === w.workspace.id) {
                                                wid = Kwin.workspaces[k].index;
                                                break;
                                            }
                                        }
                                    }
                                    if (wid !== null) {
                                        if (!windowsByWs[wid]) windowsByWs[wid] = [];
                                        windowsByWs[wid].push(w.address);
                                    }
                                }
                            }

                            if (fromWs < toWs) {
                                let temp = windowsByWs[fromWs] || [];
                                for (let i = fromWs; i < toWs; ++i) {
                                    let wins = windowsByWs[i + 1] || [];
                                    for (let j = 0; j < wins.length; ++j) Kwin.setWindowDesktop(wins[j], i);
                                }
                                for (let j = 0; j < temp.length; ++j) Kwin.setWindowDesktop(temp[j], toWs);
                            } else {
                                let temp = windowsByWs[fromWs] || [];
                                for (let i = fromWs; i > toWs; --i) {
                                    let wins = windowsByWs[i - 1] || [];
                                    for (let j = 0; j < wins.length; ++j) Kwin.setWindowDesktop(wins[j], i);
                                }
                                for (let j = 0; j < temp.length; ++j) Kwin.setWindowDesktop(temp[j], toWs);
                            }

                            const actId = root.activeWsId;
                            let newActId = actId;
                            if (actId === fromWs) {
                                newActId = toWs;
                            } else if (fromWs < toWs && actId > fromWs && actId <= toWs) {
                                newActId = actId - 1;
                            } else if (fromWs > toWs && actId >= toWs && actId < fromWs) {
                                newActId = actId + 1;
                            }
                            if (newActId !== actId) {
                                const targetUuid = Kwin.workspaces[newActId - 1]?.id;
                                if (targetUuid)
                                    Kwin.switchToWorkspace(targetUuid, root.screenName);
                            }
                        
                        }
                    }
                    drop.accept();
                }
            }
        }
    }
}

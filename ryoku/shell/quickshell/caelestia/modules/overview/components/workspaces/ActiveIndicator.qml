pragma ComponentBehavior: Bound

import QtQuick
import Caelestia.Config
import caelestia.components
import caelestia.components.effects
import caelestia.services

StyledRect {
    id: root

    required property int activeWsId
    required property Repeater workspaces
    required property Item mask
    property string screenName: ""

    readonly property int currentWsIdx: {
        let i = activeWsId - 1;
        const count = workspaces.count > 0 ? workspaces.count : Config.bar.workspaces.shown;
        while (i < 0)
            i += count;
        return i % count;
    }

    property var currentItem: workspaces.count > 0 ? workspaces.itemAt(currentWsIdx) : null
    readonly property int indicatorSize: currentItem ? (currentItem as Workspace).indicatorSize : 40
    property real rawSwipeOffset: Kwin.swipeOffsetByOutput?.[screenName] ?? Kwin.swipeOffset ?? 0.0
    property bool isSwiping: false
    property real basePos: currentItem ? currentItem.x : 0
    property real baseSize: currentItem ? (currentItem as Workspace).size : 0
    property real targetPos: {
        if (!isSwiping) return basePos;
        let startIdx = currentWsIdx;
        let endIdx = rawSwipeOffset > 0 ? startIdx + 1 : startIdx - 1;
        if (endIdx < 0 || endIdx >= workspaces.count) endIdx = startIdx;
        let startItem = workspaces.itemAt(startIdx);
        let endItem = workspaces.itemAt(endIdx);
        if (!startItem || !endItem) return basePos;
        let startPos = startItem.x;
        let endPos = endItem.x;
        return startPos + Math.abs(rawSwipeOffset) * (endPos - startPos);
    }
    property real targetSize: {
        if (!isSwiping) return baseSize;
        let startIdx = currentWsIdx;
        let endIdx = rawSwipeOffset > 0 ? startIdx + 1 : startIdx - 1;
        if (endIdx < 0 || endIdx >= workspaces.count) endIdx = startIdx;
        let startItem = workspaces.itemAt(startIdx);
        let endItem = workspaces.itemAt(endIdx);
        if (!startItem || !endItem) return baseSize;
        let startSize = (startItem as Workspace).size;
        let endSize = (endItem as Workspace).size;
        return startSize + Math.abs(rawSwipeOffset) * (endSize - startSize);
    }
    property real smoothPos: targetPos
    property real smoothSize: targetSize
    property real leading: smoothPos
    property real trailing: smoothPos
    property real currentSize: smoothSize
    property real offset: Math.min(leading, trailing)
    property real size: {
        const s = Math.abs(leading - trailing) + currentSize;
        if (Config.bar.workspaces.activeTrail && lastWs > currentWsIdx) {
            const ws = workspaces.itemAt(lastWs) as Workspace;
            return ws ? Math.min(ws.x + ws.size - offset, s) : 0;
        }
        return s;
    }
    property int cWs: currentWsIdx
    property int lastWs: currentWsIdx

    onCurrentWsIdxChanged: {
        lastWs = cWs;
        cWs = currentWsIdx;
    }
    onRawSwipeOffsetChanged: {
        if (rawSwipeOffset !== 0.0) {
            isSwiping = true;
            swipeSettleTimer.stop();
        } else {
            swipeSettleTimer.restart();
        }
    }

    clip: true
    anchors.verticalCenter: parent.verticalCenter
    x: offset + mask.x
    y: 0
    implicitWidth: size
    implicitHeight: indicatorSize
    radius: Tokens.rounding.large
    color: Colours.palette.m3primary

    Timer {
        id: swipeSettleTimer

        interval: 120
        repeat: false
        onTriggered: root.isSwiping = false
    }
    Behavior on smoothPos {
        enabled: root.isSwiping

        SmoothedAnimation {
            velocity: -1
            duration: 60
            easing.type: Easing.Linear
        }
    }
    Behavior on smoothSize {
        enabled: root.isSwiping

        SmoothedAnimation {
            velocity: -1
            duration: 60
            easing.type: Easing.Linear
        }
    }
    Behavior on leading {
        enabled: root.Config.bar.workspaces.activeTrail && !root.isSwiping

        EAnim {}
    }
    Behavior on trailing {
        enabled: root.Config.bar.workspaces.activeTrail && !root.isSwiping

        EAnim {
            duration: Tokens.anim.durations.normal * 2
        }
    }
    Behavior on currentSize {
        enabled: root.Config.bar.workspaces.activeTrail && !root.isSwiping

        EAnim {}
    }
    Behavior on offset {
        enabled: !root.Config.bar.workspaces.activeTrail && !root.isSwiping

        EAnim {}
    }
    Behavior on size {
        enabled: !root.Config.bar.workspaces.activeTrail && !root.isSwiping

        EAnim {}
    }

    component EAnim: Anim {
        type: Anim.Emphasized
    }
}

pragma ComponentBehavior: Bound

import QtQuick
import Caelestia.Config
import caelestia.components
import caelestia.components.effects
import caelestia.services

StyledRect {
    id: root

    required property int activeWsId
    required property var workspaces
    required property Item mask
    required property bool fullscreen
    property string screenName: ""

    readonly property int currentWsIdx: workspaces.findIndex(p => p?.ws === activeWsId)

    readonly property bool isHorizontal: Config.bar.position === "top" || Config.bar.position === "bottom"
    readonly property real rawScale: !isNaN(Config.bar.scale) ? Config.bar.scale : 1.0
    readonly property real scaleFactor: rawScale < 1.0 ? Math.sqrt(Math.max(0.1, rawScale)) : rawScale
    readonly property int barThickness: Math.round(Tokens.sizes.bar.innerWidth * scaleFactor)
    readonly property int expandAmt: rawScale < 0.8 ? 2 : 0
    readonly property int offsetAmt: rawScale < 0.8 ? 1 : 0

    property var currentItem: workspaces[currentWsIdx] ?? null
    property real rawSwipeOffset: Kwin.swipeOffsetByOutput?.[screenName] ?? Kwin.swipeOffset ?? 0.0
    property bool isSwiping: false
    property real basePos: currentItem ? (isHorizontal ? currentItem.x : currentItem.y) : 0
    property real baseSize: currentItem ? (currentItem as Workspace).size : 0
    property real targetPos: {
        if (!isSwiping) return basePos;
        let startIdx = currentWsIdx;
        let endIdx = rawSwipeOffset > 0 ? startIdx + 1 : startIdx - 1;
        if (endIdx < 0 || endIdx >= workspaces.length) endIdx = startIdx;
        let startItem = workspaces[startIdx];
        let endItem = workspaces[endIdx];
        if (!startItem || !endItem) return basePos;
        let startPos = isHorizontal ? startItem.x : startItem.y;
        let endPos = isHorizontal ? endItem.x : endItem.y;
        return startPos + Math.abs(rawSwipeOffset) * (endPos - startPos);
    }
    property real targetSize: {
        if (!isSwiping) return baseSize;
        let startIdx = currentWsIdx;
        let endIdx = rawSwipeOffset > 0 ? startIdx + 1 : startIdx - 1;
        if (endIdx < 0 || endIdx >= workspaces.length) endIdx = startIdx;
        let startItem = workspaces[startIdx];
        let endItem = workspaces[endIdx];
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
            const ws = workspaces[lastWs];
            return ws ? Math.min((isHorizontal ? ws.x : ws.y) + ws.size - offset, s) : 0;
        }
        return s;
    }

    property int cWs
    property int lastWs

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
    anchors.horizontalCenter: isHorizontal ? undefined : parent.horizontalCenter
    anchors.verticalCenter: isHorizontal ? parent.verticalCenter : undefined

    x: isHorizontal ? offset + mask.x - offsetAmt : 0
    y: isHorizontal ? 0 : offset + mask.y - offsetAmt
    implicitWidth: isHorizontal ? size + expandAmt : barThickness - Tokens.padding.small + expandAmt
    implicitHeight: isHorizontal ? barThickness - Tokens.padding.small + expandAmt : size + expandAmt
    radius: Tokens.rounding.full
    color: Colours.palette.m3primary

    Timer {
        id: swipeSettleTimer

        interval: 120
        repeat: false
        onTriggered: root.isSwiping = false
    }
    Colouriser {
        source: root.mask
        sourceColor: Colours.palette.m3onSurface
        colorizationColor: Colours.palette.m3onPrimary

        x: isHorizontal ? -parent.offset + offsetAmt : 0
        y: isHorizontal ? 0 : -parent.offset + offsetAmt
        implicitWidth: root.mask.implicitWidth
        implicitHeight: root.mask.implicitHeight
        anchors.horizontalCenter: isHorizontal ? undefined : parent.horizontalCenter
        anchors.verticalCenter: isHorizontal ? parent.verticalCenter : undefined
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

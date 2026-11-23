pragma ComponentBehavior: Bound

import QtQuick
import "../kit" as K
import "lib/motion.js" as Motion

// The track list: the engine's rows over one bead. Rows swap instantly on a
// keystroke and only the bead travels. On each open the rows ripple in once.
ListView {
    id: list

    property var engine: null
    property bool live: false
    property string matchText: ""

    property real rippleClock: 0
    property bool rippling: false
    readonly property int rippleStep: 14
    readonly property int rippleDuration: K.Style.ms(Math.round(0.6 * K.Look.enterMs))
    property bool rippleArmed: false
    property point lastPointer: Qt.point(-1, -1)

    signal hovered(int index)
    signal picked(int index)

    model: list.engine ? list.engine.rows : []
    clip: true
    interactive: list.contentHeight > list.height
    boundsBehavior: Flickable.StopAtBounds
    cacheBuffer: K.Style.rowH * 4

    // Called on each open: the first rows to land ripple in, and the pointer
    // must move before hover may take the selection.
    function armOpen() {
        list.lastPointer = Qt.point(-1, -1);
        list.rippleArmed = K.Look.microMotion && !K.Style.still;
        list.rippling = false;
        rippleAnimation.stop();
        list.startRippleIfReady();
    }

    function startRippleIfReady() {
        if (!list.rippleArmed || !list.engine || list.engine.rows.length === 0)
            return;
        list.rippleArmed = false;
        list.rippleClock = 0;
        list.rippling = true;
        const span = Math.max(1, Motion.rippleSpan(
            Math.min(list.engine.rows.length, 9), list.rippleStep, list.rippleDuration));
        rippleAnimation.to = span;
        rippleAnimation.duration = span;
        rippleAnimation.restart();
    }

    function pointerAt(index, scenePosition) {
        const first = list.lastPointer.x < 0;
        const moved = !first && (scenePosition.x !== list.lastPointer.x
            || scenePosition.y !== list.lastPointer.y);
        list.lastPointer = scenePosition;
        if (moved)
            list.hovered(index);
    }

    function reveal() {
        if (list.engine && list.engine.selectedIndex >= 0)
            list.positionViewAtIndex(list.engine.selectedIndex, ListView.Contain);
        else
            list.positionViewAtBeginning();
    }

    NumberAnimation {
        id: rippleAnimation
        target: list
        property: "rippleClock"
        from: 0
        easing.type: Easing.Linear
        onFinished: list.rippling = false
    }

    Connections {
        target: list.engine
        function onRowsChanged() {
            list.startRippleIfReady();
            Qt.callLater(list.reveal);
        }
        function onSelectedIndexChanged() { Qt.callLater(list.reveal); }
        function onRan(resultKey) {
            const rows = list.engine.rows;
            for (let i = 0; i < rows.length; i++) {
                if (rows[i].resultKey !== resultKey)
                    continue;
                const item = list.itemAtIndex(i);
                if (item)
                    item.pinchNow();
                return;
            }
        }
    }

    Bead {
        // the view stacks delegates by creation order, so the bead stays explicitly under them
        z: -1
        width: list.width
        rowHeight: K.Style.rowH
        targetY: Math.max(0, list.engine ? list.engine.selectedIndex : 0) * K.Style.rowH
        visible: list.engine !== null && list.engine.selectedIndex >= 0 && list.count > 0
    }

    delegate: TrackRow {
        width: list.width
        height: K.Style.rowH
        engine: list.engine
        selected: list.engine !== null && index === list.engine.selectedIndex
        highlight: list.matchText
        live: list.live
        intro: list.rippling
            ? Motion.rippleProgress(list.rippleClock, index, list.rippleStep,
                list.rippleDuration, K.Look.enterCurve)
            : 1
        onPointed: scenePosition => list.pointerAt(index, scenePosition)
        onActivated: list.picked(index)
    }
}

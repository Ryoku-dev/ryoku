pragma ComponentBehavior: Bound

import QtQuick
import "../kit" as K
import "lib/motion.js" as Motion
import "lib/listdiff.js" as ListDiff
import "lib/rows.js" as Rows

// The track list: the engine's rows over one bead. The rows live in a keyed
// model, so a row that stays across a keystroke keeps its delegate, and rows
// that show up as you type rise in. On each open the rows ripple in once.
ListView {
    id: list

    property var engine: null
    property bool live: false
    property string matchText: ""

    // resultKey to row, read by the delegates of the keyed model
    property var keyed: ({})
    property string syncedMode: ""
    // keys that come into view with the current edit, by their place in the cascade
    property var entering: ({})
    readonly property bool motion: K.Look.microMotion && !K.Style.still
    readonly property int digits: Rows.indexDigits(list.count, K.Look.lithic)
    readonly property real indexWidth: Math.ceil(digitMetrics.advanceWidth * list.digits) + 1
    // the user moved the selection, so the next placement glides instead of snapping
    property bool glideNext: false

    property real rippleClock: 0
    property bool rippling: false
    readonly property int rippleStep: 14
    readonly property int rippleDuration: K.Style.ms(Math.round(0.6 * K.Look.enterMs))
    property bool rippleArmed: false
    property point lastPointer: Qt.point(-1, -1)

    signal hovered(int index)
    signal picked(int index)

    model: slots
    clip: true
    interactive: list.contentHeight > list.height
    boundsBehavior: Flickable.StopAtBounds
    cacheBuffer: K.Style.rowH * 4

    ListModel {
        id: slots
    }

    TextMetrics {
        id: digitMetrics
        font.family: K.Look.lithic ? K.Style.mono : K.Style.font
        font.pixelSize: K.Style.caption
        text: "0"
    }

    // Before an open shows: back to the top, and rows kept from the last open
    // are forgotten so the map holds only what this open lists.
    function prepare() {
        list.keyed = ({});
        list.syncedMode = "";
        list.glideNext = false;
        list.sync();
        list.positionViewAtBeginning();
    }

    // Called once the sheet shows: the rows ripple in, and the pointer must
    // move before hover may take the selection.
    function armOpen() {
        list.lastPointer = Qt.point(-1, -1);
        list.rippleArmed = list.motion;
        list.rippling = false;
        rippleAnimation.stop();
        list.startRippleIfReady();
    }

    function startRippleIfReady() {
        if (!list.rippleArmed || slots.count === 0)
            return;
        list.rippleArmed = false;
        list.rippleClock = 0;
        list.rippling = true;
        const span = Math.max(1, Motion.rippleSpan(
            Math.min(slots.count, 9), list.rippleStep, list.rippleDuration));
        rippleAnimation.to = span;
        rippleAnimation.duration = span;
        rippleAnimation.restart();
    }

    // Mirrors the engine's rows into the keyed model. While the sheet shows and
    // the tab stays (typing), rows that come into view rise in; rows already in
    // view stay still, so nothing slides through another row. An open or a tab
    // switch lands at once.
    function sync() {
        const rows = list.engine ? list.engine.rows : [];
        const byKey = {};
        const keys = [];
        for (let i = 0; i < rows.length; i++) {
            const key = String(rows[i].resultKey || "");
            byKey[key] = rows[i];
            keys.push(key);
        }
        const mode = list.engine ? list.engine.mode : "";
        const animate = list.live && list.motion && mode === list.syncedMode;
        list.syncedMode = mode;
        list.entering = animate ? list.enteringKeys(keys) : ({});
        list.keyed = byKey;
        const current = [];
        for (let i = 0; i < slots.count; i++)
            current.push(slots.get(i).key);
        const ops = ListDiff.editScript(current, keys);
        for (let i = 0; i < ops.length; i++) {
            const op = ops[i];
            if (op.op === "remove")
                slots.remove(op.index, 1);
            else if (op.op === "move")
                slots.move(op.from, op.to, 1);
            else
                slots.insert(op.index, { key: op.key });
        }
    }

    // The keys the next list shows in its first screenful that were not in
    // view before, each with its place in the cascade.
    function enteringKeys(keys) {
        const rows = Math.ceil(list.height / K.Style.rowH) + 1;
        const first = Math.max(0, list.indexAt(1, list.contentY + 1));
        const before = {};
        for (let i = first; i < Math.min(slots.count, first + rows); i++)
            before["$" + slots.get(i).key] = true;
        const out = {};
        let order = 0;
        for (let i = 0; i < Math.min(keys.length, rows); i++) {
            if (!before["$" + keys[i]])
                out["$" + keys[i]] = order++;
        }
        return out;
    }

    function maybeEnter(track) {
        const name = "$" + track.key;
        const order = list.entering[name];
        if (order === undefined)
            return;
        delete list.entering[name];
        track.enter(order);
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

    function placeBead(y) {
        bead.moveTo(y, list.glideNext && list.motion);
        list.glideNext = false;
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
            list.sync();
            list.startRippleIfReady();
            Qt.callLater(list.reveal);
        }
        function onSelectedIndexChanged() { Qt.callLater(list.reveal); }
        function onStepped() { list.glideNext = true; }
        function onRan(resultKey) {
            const item = list.itemAtIndex(list.engine.selectedIndex);
            if (item && item.key === resultKey)
                item.pinchNow();
        }
    }

    // ListView keeps its own children out of the scrolling content, so the
    // bead is moved into it to scroll and slide with the rows.
    Bead {
        id: bead
        parent: list.contentItem
        z: -1
        width: list.width
        rowHeight: K.Style.rowH
        visible: list.engine !== null && list.engine.selectedIndex >= 0 && slots.count > 0
    }

    delegate: TrackRow {
        id: track
        width: list.width
        height: K.Style.rowH
        engine: list.engine
        row: list.keyed[track.key] || null
        digits: list.digits
        indexWidth: list.indexWidth
        selected: list.engine !== null && track.index === list.engine.selectedIndex
        highlight: list.matchText
        live: list.live
        intro: list.rippling
            ? Motion.rippleProgress(list.rippleClock, track.index, list.rippleStep,
                list.rippleDuration, K.Look.enterCurve)
            : 1
        onPointed: scenePosition => list.pointerAt(track.index, scenePosition)
        onActivated: list.picked(track.index)
        onSelectedChanged: if (track.selected) list.placeBead(track.y)
        onYChanged: if (track.selected) bead.follow(track.y)
        onIndexChanged: list.maybeEnter(track)
        Component.onCompleted: {
            if (track.selected)
                list.placeBead(track.y);
            list.maybeEnter(track);
        }
    }
}

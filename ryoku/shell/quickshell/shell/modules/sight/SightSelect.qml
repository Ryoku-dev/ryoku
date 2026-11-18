pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons

FocusScope {
    id: root
    required property var frame
    property var previous: null
    property var windows: []
    property bool active: false
    property bool busy: false
    property string errorText: ""
    property rect usable: Qt.rect(0, 0, width, height)
    property real s: 1
    property bool dragging: false
    property bool spaceHeld: false
    property bool altHeld: false
    property point pointer: Qt.point(0, 0)
    property point anchor: Qt.point(0, 0)
    property point pressedAt: Qt.point(0, 0)
    property rect box: Qt.rect(0, 0, 0, 0)
    property rect hovered: Qt.rect(0, 0, 0, 0)
    property bool hoverGlide: false
    property real travel: 0
    readonly property rect hole: dragging || busy ? box : hovered
    readonly property bool hasPrevious: previous && frame && previous.output === frame.output
    readonly property real lithicScale: s
    signal commitRequested(rect rect)
    signal cancelRequested()

    function boundedPoint(x, y) { return Qt.point(Math.max(0, Math.min(width, x)), Math.max(0, Math.min(height, y))); }
    function localRect(rect) {
        const x = Math.max(0, Number(rect.x) - Number(frame.x));
        const y = Math.max(0, Number(rect.y) - Number(frame.y));
        const right = Math.min(width, Number(rect.x) - Number(frame.x) + Number(rect.width));
        const bottom = Math.min(height, Number(rect.y) - Number(frame.y) + Number(rect.height));
        return Qt.rect(x, y, Math.max(0, right - x), Math.max(0, bottom - y));
    }
    function hoverAt(point) {
        var next = Qt.rect(0, 0, 0, 0);
        for (var i = 0; i < windows.length; ++i) {
            const rect = localRect(windows[i]);
            if (point.x >= rect.x && point.y >= rect.y && point.x < rect.x + rect.width && point.y < rect.y + rect.height) {
                next = rect;
                break;
            }
        }
        if (next.x === hovered.x && next.y === hovered.y && next.width === hovered.width && next.height === hovered.height) return;
        hoverGlide = hovered.width > 0 && next.width > 0;
        hovered = next;
    }
    function commit(rect) {
        if (busy || rect.width < 8 || rect.height < 8) return;
        box = rect;
        commitRequested(rect);
    }
    function reset() {
        box = Qt.rect(0, 0, 0, 0);
        hovered = Qt.rect(0, 0, 0, 0);
        dragging = false;
        spaceHeld = false;
        altHeld = false;
        travel = 0;
    }
    onFrameChanged: reset()
    onActiveChanged: {
        if (active) Qt.callLater(function() { root.forceActiveFocus(); });
        else { spaceHeld = false; altHeld = false; dragging = false; }
    }
    Keys.onPressed: event => {
        if (!active) return;
        if (event.key === Qt.Key_Escape) { cancelRequested(); event.accepted = true; }
        else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
            if (hasPrevious) commit(localRect(previous));
            event.accepted = true;
        } else if (event.key === Qt.Key_Space) { spaceHeld = true; event.accepted = true; }
        else if (event.key === Qt.Key_Alt) { altHeld = true; event.accepted = true; }
    }
    Keys.onReleased: event => {
        if (event.key === Qt.Key_Space) { spaceHeld = false; event.accepted = true; }
        else if (event.key === Qt.Key_Alt) { altHeld = false; event.accepted = true; }
    }

    MouseArea {
        anchors.fill: parent
        enabled: root.active && !root.busy
        hoverEnabled: true
        cursorShape: Qt.CrossCursor
        acceptedButtons: Qt.LeftButton | Qt.RightButton
        onPressed: event => {
            if (event.button === Qt.RightButton) { root.cancelRequested(); return; }
            root.pointer = root.boundedPoint(event.x, event.y);
            root.anchor = root.pointer;
            root.pressedAt = root.pointer;
            root.travel = 0;
            root.box = Qt.rect(root.pointer.x, root.pointer.y, 0, 0);
            root.dragging = true;
        }
        onPositionChanged: event => {
            const point = root.boundedPoint(event.x, event.y);
            const old = root.pointer;
            root.pointer = point;
            root.altHeld = (event.modifiers & Qt.AltModifier) !== 0;
            if (!root.dragging) { root.hoverAt(point); return; }
            root.travel = Math.max(root.travel, Math.abs(point.x - root.pressedAt.x), Math.abs(point.y - root.pressedAt.y));
            if (root.spaceHeld && root.box.width > 0 && root.box.height > 0) {
                const dx = Math.max(-root.box.x, Math.min(root.width - root.box.x - root.box.width, point.x - old.x));
                const dy = Math.max(-root.box.y, Math.min(root.height - root.box.y - root.box.height, point.y - old.y));
                root.anchor = Qt.point(root.anchor.x + dx, root.anchor.y + dy);
                root.box = Qt.rect(root.box.x + dx, root.box.y + dy, root.box.width, root.box.height);
            } else {
                root.box = Qt.rect(Math.min(root.anchor.x, point.x), Math.min(root.anchor.y, point.y),
                    Math.abs(point.x - root.anchor.x), Math.abs(point.y - root.anchor.y));
            }
        }
        onReleased: event => {
            if (event.button !== Qt.LeftButton || !root.dragging) return;
            const rect = root.box;
            root.dragging = false;
            if (root.travel < 4) {
                root.hoverAt(root.pointer);
                root.commit(root.hovered);
            } else if (rect.width >= 8 && rect.height >= 8) root.commit(rect);
            else root.box = Qt.rect(0, 0, 0, 0);
        }
        onCanceled: { root.dragging = false; root.box = Qt.rect(0, 0, 0, 0); }
        onExited: if (!root.dragging) { root.hoverGlide = false; root.hovered = Qt.rect(0, 0, 0, 0); }
    }
    Item {
        visible: !root.dragging && !root.busy && root.hovered.width > 0
        width: root.hovered.width; height: root.hovered.height
        transform: Translate {
            x: root.hovered.x; y: root.hovered.y
            Behavior on x { enabled: root.hoverGlide; NumberAnimation { duration: Lithic.duration(140); easing.type: Easing.OutCubic } }
            Behavior on y { enabled: root.hoverGlide; NumberAnimation { duration: Lithic.duration(140); easing.type: Easing.OutCubic } }
        }
        SightPlate { rect: Qt.rect(0, 0, parent.width, parent.height); imageVisible: false; highlighted: true; s: root.s }
    }
    SightPlate { rect: root.box; imageVisible: false; visible: (root.dragging || root.busy) && root.box.width > 0; s: root.s }
    CutRect {
        id: readout
        visible: root.dragging && root.box.width > 0
        width: dimensions.implicitWidth + 16 * root.s; height: 24 * root.s
        x: Math.max(0, Math.min(root.width - width, !root.altHeld && root.pointer.x + 15 * root.s + width <= root.width ? root.pointer.x + 15 * root.s : root.pointer.x - width - 15 * root.s))
        y: Math.max(0, Math.min(root.height - height, !root.altHeld && root.pointer.y + 15 * root.s + height <= root.height ? root.pointer.y + 15 * root.s : root.pointer.y - height - 15 * root.s))
        fill: Lithic.field; strokeColor: Lithic.vein; strokeWidth: 1; cut: 4
        Label { id: dimensions; anchors.centerIn: parent; typeRole: "mono"; text: I18n.tr("%1 × %2").arg(Math.round(root.box.width)).arg(Math.round(root.box.height)) }
    }
    SightLoupe {
        visible: root.dragging && root.altHeld
        frame: root.frame; pointer: root.pointer; s: root.s
        outputWidth: root.width; outputHeight: root.height
    }
    SightHint {
        visible: !root.dragging
        x: root.usable.x + (root.usable.width - width) / 2
        y: Math.max(root.usable.y, root.usable.y + root.usable.height - height - 34 * root.s)
        s: root.s; hasPrevious: root.hasPrevious
        busy: root.busy; errorText: root.errorText
    }
}

import QtQuick

// a tap picks, a long press or a sideways pull lifts; a vertical pull stays with the shade's scroller
MouseArea {
    id: grab

    property Item space: null
    property bool holding: false
    property point origin: Qt.point(0, 0)

    signal tapped()
    signal lifted(point at)
    signal dragged(point at)
    signal dropped(point at)
    signal cancelled()

    function here(mouse) { return grab.mapToItem(grab.space, mouse.x, mouse.y) }
    function lift() {
        grab.holding = true;
        grab.lifted(grab.origin);
    }

    anchors.fill: parent
    hoverEnabled: true
    pressAndHoldInterval: 320
    // once lifted the scroller may not take the pointer back
    preventStealing: grab.holding
    cursorShape: grab.holding ? Qt.ClosedHandCursor : Qt.PointingHandCursor

    onPressed: function (mouse) {
        grab.origin = grab.here(mouse);
        grab.holding = false;
    }
    onPressAndHold: if (!grab.holding) grab.lift()
    onPositionChanged: function (mouse) {
        if (!grab.pressed)
            return;
        var at = grab.here(mouse);
        if (!grab.holding) {
            var dx = at.x - grab.origin.x, dy = at.y - grab.origin.y;
            if (Math.abs(dx) < 8 || Math.abs(dx) < Math.abs(dy))
                return;
            grab.lift();
        }
        grab.dragged(at);
    }
    onReleased: function (mouse) {
        if (!grab.holding) {
            grab.tapped();
            return;
        }
        grab.holding = false;
        grab.dropped(grab.here(mouse));
    }
    onCanceled: {
        if (!grab.holding)
            return;
        grab.holding = false;
        grab.cancelled();
    }
}

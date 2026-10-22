pragma ComponentBehavior: Bound

import QtQuick
import Caelestia.Config
import caelestia.components

MouseArea {
    id: root

    property bool preventStealingDrag: true
    property bool allowExpandGesture: true
    property bool closed: false
    property int startY: 0

    signal closeRequested()
    signal expandRequested(expanded: bool)

    hoverEnabled: true
    cursorShape: pressed ? Qt.ClosedHandCursor : undefined
    acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
    preventStealing: root.preventStealingDrag
    enabled: !root.closed

    drag.target: this
    drag.axis: Drag.XAxis

    onPressed: event => {
        root.startY = event.y;
        if (event.button === Qt.RightButton)
            root.expandRequested(true);
        else if (event.button === Qt.MiddleButton)
            root.closeRequested();
    }

    onPositionChanged: event => {
        if (pressed && root.allowExpandGesture) {
            const diffY = event.y - root.startY;
            if (Math.abs(diffY) > Config.notifs.expandThreshold)
                root.expandRequested(diffY > 0);
        }
    }

    onReleased: event => {
        if (Math.abs(x) < width * Config.notifs.clearThreshold)
            x = 0;
        else
            root.closeRequested();
    }

    Behavior on y {
        enabled: root.LazyListView?.ready ?? false

        Anim {}
    }

    Behavior on opacity {
        Anim {
            type: Anim.DefaultEffects
        }
    }

    Behavior on scale {
        Anim {}
    }

    Behavior on x {
        Anim {}
    }
}

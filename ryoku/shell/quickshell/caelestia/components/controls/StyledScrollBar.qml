import QtQuick
import QtQuick.Templates
import Caelestia.Config
import caelestia.components
import caelestia.services

ScrollBar {
    id: root

    required property Flickable flickable
    property bool shouldBeActive
    property real nonAnimPosition
    property bool animating
    property bool _updatingFromFlickable: false
    property bool _updatingFromUser: false

    readonly property bool isVertical: root.orientation === Qt.Vertical

    onHoveredChanged: {
        if (hovered)
            shouldBeActive = true;
        else
            shouldBeActive = flickable.moving;
    }

    onPositionChanged: {
        if (_updatingFromUser) {
            _updatingFromUser = false;
            return;
        }
        if (position === nonAnimPosition) {
            animating = false;
            return;
        }
        if (!animating && !_updatingFromFlickable && !fullMouse.pressed) {
            nonAnimPosition = position;
        }
    }

    Component.onCompleted: {
        if (flickable) {
            const contentLen = isVertical ? flickable.contentHeight : flickable.contentWidth;
            const len = isVertical ? flickable.height : flickable.width;
            if (contentLen > len) {
                const pos = isVertical ? flickable.contentY - flickable.originY : flickable.contentX - flickable.originX;
                nonAnimPosition = Math.max(0, Math.min(1, pos / (contentLen - len)));
            }
        }
    }
    
    implicitWidth: isVertical ? Tokens.padding.extraSmall : 0
    implicitHeight: isVertical ? 0 : Tokens.padding.extraSmall

    contentItem: StyledRect {
        anchors.left: isVertical ? parent.left : undefined
        anchors.right: isVertical ? parent.right : undefined
        anchors.top: isVertical ? undefined : parent.top
        anchors.bottom: isVertical ? undefined : parent.bottom
        
        opacity: {
            if (root.size === 1)
                return 0;
            if (fullMouse.pressed)
                return 1;
            if (mouse.containsMouse)
                return 0.8;
            if (root.policy === ScrollBar.AlwaysOn || root.shouldBeActive)
                return 0.6;
            return 0;
        }
        radius: Tokens.rounding.full
        color: Colours.palette.m3secondary

        MouseArea {
            id: mouse

            anchors.fill: parent
            cursorShape: Qt.PointingHandCursor
            hoverEnabled: true
            acceptedButtons: Qt.NoButton
        }

        Behavior on opacity {
            Anim {
                type: Anim.DefaultEffects
            }
        }
    }

    Connections {
        function onContentYChanged() { if (root.isVertical) updatePos(); }
        function onContentXChanged() { if (!root.isVertical) updatePos(); }

        function updatePos() {
            if (!root.animating && !fullMouse.pressed) {
                root._updatingFromFlickable = true;
                const contentLen = root.isVertical ? root.flickable.contentHeight : root.flickable.contentWidth;
                const len = root.isVertical ? root.flickable.height : root.flickable.width;
                if (contentLen > len) {
                    // ListViews with delegates of varying size move their origin as
                    // they estimate the content size; positions are relative to it.
                    const pos = root.isVertical ? root.flickable.contentY - root.flickable.originY : root.flickable.contentX - root.flickable.originX;
                    root.nonAnimPosition = Math.max(0, Math.min(1, pos / (contentLen - len)));
                } else {
                    root.nonAnimPosition = 0;
                }
                root._updatingFromFlickable = false;
            }
        }

        target: root.flickable
    }

    Connections {
        function onMovingChanged(): void {
            if (root.flickable.moving)
                root.shouldBeActive = true;
            else
                hideDelay.restart();
        }

        target: root.flickable
    }

    Timer {
        id: hideDelay

        interval: 600
        onTriggered: root.shouldBeActive = root.flickable.moving || root.hovered
    }

    CustomMouseArea {
        id: fullMouse

        function onWheel(event: WheelEvent): void {
            root.animating = true;
            root._updatingFromUser = true;
            let newPos = root.nonAnimPosition;
            const delta = root.isVertical ? event.angleDelta.y : (event.angleDelta.x || event.angleDelta.y);
            if (delta > 0)
                newPos = Math.max(0, root.nonAnimPosition - 0.1);
            else if (delta < 0)
                newPos = Math.min(1 - root.size, root.nonAnimPosition + 0.1);
            root.nonAnimPosition = newPos;
            updateFlickable(newPos);
        }

        function updateFlickable(newPos: real): void {
            if (root.flickable) {
                const contentLen = root.isVertical ? root.flickable.contentHeight : root.flickable.contentWidth;
                const len = root.isVertical ? root.flickable.height : root.flickable.width;
                if (contentLen > len) {
                    const maxContentPos = contentLen - len;
                    const maxPos = 1 - root.size;
                    const contentPos = maxPos > 0 ? (newPos / maxPos) * maxContentPos : 0;
                    const finalPos = Math.max(0, Math.min(maxContentPos, contentPos));
                    if (root.isVertical)
                        root.flickable.contentY = root.flickable.originY + finalPos;
                    else
                        root.flickable.contentX = root.flickable.originX + finalPos;
                }
            }
        }

        anchors.fill: parent
        preventStealing: true

        onPressed: event => {
            root.animating = true;
            root._updatingFromUser = true;
            const evPos = root.isVertical ? event.y : event.x;
            const rLen = root.isVertical ? root.height : root.width;
            const newPos = Math.max(0, Math.min(1 - root.size, evPos / rLen - root.size / 2));
            root.nonAnimPosition = newPos;
            updateFlickable(newPos);
        }

        onPositionChanged: event => {
            root._updatingFromUser = true;
            const evPos = root.isVertical ? event.y : event.x;
            const rLen = root.isVertical ? root.height : root.width;
            const newPos = Math.max(0, Math.min(1 - root.size, evPos / rLen - root.size / 2));
            root.nonAnimPosition = newPos;
            updateFlickable(newPos);
        }
    }

    Behavior on position {
        enabled: !fullMouse.pressed

        Anim {}
    }
}

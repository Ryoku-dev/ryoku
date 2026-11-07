import QtQuick

Item {
    id: root

    property real from: 0
    property real to: 1
    property real value: from
    property real stepSize: 0.05
    property string icon: ""
    property string valueText: ""
    property real _dragValue: value

    signal moved(real value)
    signal committed(real value)

    readonly property bool dragging: sliderPointer.pressed
    readonly property real visualValue: dragging ? _dragValue : value
    readonly property real clampedValue: Math.max(Math.min(from, to),
        Math.min(Math.max(from, to), visualValue))
    readonly property real fraction: to === from ? 0
        : Math.max(0, Math.min(1, (clampedValue - from) / (to - from)))
    readonly property real iconSpan: icon.length > 0 ? Style.icon + Style.s2 : 0
    readonly property real valueSpan: valueText.length > 0
        ? valueLabel.implicitWidth + Style.s2 : 0

    implicitWidth: 220 + iconSpan + valueSpan
    implicitHeight: 24
    activeFocusOnTab: true
    opacity: enabled ? 1 : 0.38

    function snap(candidate) {
        const low = Math.min(from, to);
        const high = Math.max(from, to);
        const bounded = Math.max(low, Math.min(high, candidate));
        if (stepSize <= 0)
            return bounded;
        const snapped = from + Math.round((bounded - from) / stepSize) * stepSize;
        return Math.max(low, Math.min(high, snapped));
    }

    function valueAt(pointerX) {
        const fractionAtPointer = Math.max(0, Math.min(1,
            pointerX / Math.max(1, track.width)));
        return snap(from + fractionAtPointer * (to - from));
    }

    function step(direction) {
        const amount = stepSize > 0 ? stepSize : Math.abs(to - from) / 20;
        committed(snap(value + direction * amount));
    }

    Keys.onPressed: event => {
        if (event.key === Qt.Key_Left || event.key === Qt.Key_Down) {
            step(-1);
            event.accepted = true;
        } else if (event.key === Qt.Key_Right || event.key === Qt.Key_Up) {
            step(1);
            event.accepted = true;
        }
    }

    Glyph {
        visible: root.icon.length > 0
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        name: root.icon
        size: Style.icon
        tone: "dim"
    }

    Item {
        id: track
        x: root.iconSpan
        width: Math.max(1, root.width - root.iconSpan - root.valueSpan)
        height: root.height

        Rectangle {
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width
            height: 4
            color: "transparent"
            border.width: Style.border
            border.color: trackHover.hovered || root.activeFocus ? Style.lineStrong : Style.line
            antialiasing: false
        }

        Rectangle {
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width * root.fraction
            height: 4
            color: Style.ink
            antialiasing: false
        }

        Rectangle {
            x: Math.max(0, Math.min(parent.width - width,
                parent.width * root.fraction - width / 2))
            anchors.verticalCenter: parent.verticalCenter
            width: 6
            height: 17
            color: Style.ink
            antialiasing: false

            Behavior on x {
                enabled: !root.dragging && !Style.still
                NumberAnimation { duration: Style.snap; easing.type: Easing.OutQuad }
            }
        }

        HoverHandler { id: trackHover; cursorShape: Qt.PointingHandCursor }

        MouseArea {
            id: sliderPointer
            anchors.fill: parent
            anchors.topMargin: -8
            anchors.bottomMargin: -8
            enabled: root.enabled
            hoverEnabled: true
            preventStealing: true
            acceptedButtons: Qt.LeftButton
            cursorShape: Qt.PointingHandCursor

            onPressed: mouse => {
                root.forceActiveFocus();
                root._dragValue = root.valueAt(mouse.x);
                root.moved(root._dragValue);
            }
            onPositionChanged: mouse => {
                if (pressed) {
                    root._dragValue = root.valueAt(mouse.x);
                    root.moved(root._dragValue);
                }
            }
            onReleased: mouse => {
                root._dragValue = root.valueAt(mouse.x);
                root.committed(root._dragValue);
            }
            onCanceled: root.committed(root._dragValue)
            onWheel: wheel => {
                const delta = wheel.angleDelta.y !== 0
                    ? wheel.angleDelta.y : wheel.pixelDelta.y;
                if (delta !== 0) {
                    root.step(delta > 0 ? 1 : -1);
                    wheel.accepted = true;
                }
            }
        }
    }

    Label {
        id: valueLabel
        visible: root.valueText.length > 0
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        text: root.valueText
        role: "caption"
        tone: "dim"
        numeric: true
    }
}

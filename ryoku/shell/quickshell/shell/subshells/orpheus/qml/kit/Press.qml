import QtQuick

Item {
    id: root

    property bool tint: true
    property real radius: Style.radius
    readonly property bool hovered: pointer.containsMouse
    readonly property bool pressed: pointer.pressed
    property real _pressScale: pressed ? 0.98 : 1
    readonly property real pressScale: _pressScale
    property real _wheelDelta: 0
    // Only a press that steps something on the wheel takes it; any other one
    // (every list row) lets it through to the card that scrolls underneath.
    property bool wheelSteps: false

    signal down(var mouse)
    signal clicked(var mouse)
    signal rightClicked()
    signal middleClicked()
    signal wheel(int steps)

    anchors.fill: parent

    Rectangle {
        anchors.fill: parent
        visible: root.tint
        radius: root.radius
        color: root.pressed ? Style.tint16 : root.hovered ? Style.tint10 : "transparent"

        Behavior on color {
            enabled: !Style.still
            ColorAnimation { duration: Style.snap }
        }
    }

    MouseArea {
        id: pointer
        anchors.fill: parent
        enabled: root.enabled
        hoverEnabled: true
        acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
        cursorShape: Qt.PointingHandCursor
        onPressed: mouse => root.down(mouse)
        onClicked: mouse => {
            if (mouse.button === Qt.RightButton)
                root.rightClicked();
            else if (mouse.button === Qt.MiddleButton)
                root.middleClicked();
            else
                root.clicked(mouse);
        }
    }

    WheelHandler {
        enabled: root.enabled && root.wheelSteps
        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
        onWheel: event => {
            const delta = event.angleDelta.y !== 0 ? event.angleDelta.y : event.pixelDelta.y;
            root._wheelDelta += delta;
            while (Math.abs(root._wheelDelta) >= 120) {
                const step = root._wheelDelta > 0 ? 1 : -1;
                root._wheelDelta -= step * 120;
                root.wheel(step);
            }
            event.accepted = true;
        }
    }

    Behavior on _pressScale {
        enabled: !Style.still
        NumberAnimation {
            duration: Style.snap
            easing.type: root.pressed ? Easing.OutCubic : Easing.OutBack
        }
    }
}

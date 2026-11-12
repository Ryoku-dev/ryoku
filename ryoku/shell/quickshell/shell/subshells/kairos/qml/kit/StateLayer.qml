import QtQuick

MouseArea {
    id: wash

    property real radius: 0
    property real topLeftRadius: wash.radius
    property real topRightRadius: wash.radius
    property real bottomLeftRadius: wash.radius
    property real bottomRightRadius: wash.radius
    property color tint: Md.ink
    property bool ring: true

    signal activated()
    signal secondary()

    anchors.fill: parent
    hoverEnabled: true
    acceptedButtons: Qt.LeftButton | Qt.RightButton
    cursorShape: wash.enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
    activeFocusOnTab: wash.enabled

    onClicked: function (mouse) {
        if (mouse.button === Qt.RightButton)
            wash.secondary();
        else
            wash.activated();
    }
    Keys.onPressed: function (event) {
        if (event.key === Qt.Key_Space || event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
            // holding enter would arm and then fire a power action
            if (!event.isAutoRepeat)
                wash.activated();
            event.accepted = true;
        }
    }

    Rectangle {
        anchors.fill: parent
        topLeftRadius: wash.topLeftRadius
        topRightRadius: wash.topRightRadius
        bottomLeftRadius: wash.bottomLeftRadius
        bottomRightRadius: wash.bottomRightRadius
        color: wash.tint
        opacity: !wash.enabled ? 0 : wash.pressed ? 0.14 : wash.containsMouse ? 0.08 : 0
        Behavior on opacity {
            enabled: !Md.still
            NumberAnimation { duration: Md.fast }
        }
    }

    Rectangle {
        anchors.fill: parent
        anchors.margins: -3
        visible: wash.ring && wash.activeFocus
        topLeftRadius: wash.topLeftRadius + 3
        topRightRadius: wash.topRightRadius + 3
        bottomLeftRadius: wash.bottomLeftRadius + 3
        bottomRightRadius: wash.bottomRightRadius + 3
        color: "transparent"
        border.width: 2
        border.color: Md.accent
    }
}

import QtQuick

Item {
    id: root

    property string text: ""
    property string icon: ""
    property bool danger: false
    property int holdMs: 800
    // In a list the glyph and label start at the row's inset instead of centring.
    property bool leading: false
    readonly property real contentX: leading ? Style.s3 : (width - baseContent.width) / 2
    property real _progress: 0
    property bool _fired: false
    property bool _holding: false

    signal activated()

    implicitWidth: baseContent.implicitWidth + Style.s6
    implicitHeight: 44
    opacity: enabled ? 1 : 0.38
    scale: pointer.pressed ? 0.98 : 1
    activeFocusOnTab: true

    function beginHold() {
        if (!enabled || _holding)
            return;
        resetDelay.stop();
        drain.stop();
        filling.stop();
        _progress = 0;
        _fired = false;
        _holding = true;
        filling.duration = Math.max(1, holdMs);
        filling.start();
    }

    function endHold() {
        if (!_holding)
            return;
        _holding = false;
        filling.stop();
        if (_fired)
            return;
        drain.stop();
        drain.from = _progress;
        drain.duration = Math.max(1, Style.move * _progress);
        drain.start();
    }

    Keys.onPressed: event => {
        if ((event.key === Qt.Key_Space || event.key === Qt.Key_Return
                || event.key === Qt.Key_Enter) && !event.isAutoRepeat) {
            root.beginHold();
            event.accepted = true;
        }
    }
    Keys.onReleased: event => {
        if ((event.key === Qt.Key_Space || event.key === Qt.Key_Return
                || event.key === Qt.Key_Enter) && !event.isAutoRepeat) {
            root.endHold();
            event.accepted = true;
        }
    }

    Rectangle {
        id: plate
        anchors.fill: parent
        radius: Style.radius
        color: pointer.pressed ? Style.tint16
            : pointer.containsMouse ? Style.tint10 : "transparent"
        border.width: Style.border
        border.color: pointer.containsMouse ? Style.lineStrong : Style.line
        clip: true

        Rectangle {
            anchors.left: parent.left
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            width: parent.width * root._progress
            color: Style.bone
        }

        Behavior on color {
            enabled: !Style.still
            ColorAnimation { duration: Style.snap }
        }
    }

    Row {
        id: baseContent
        x: root.contentX
        y: (root.height - height) / 2
        spacing: Style.s2

        Glyph {
            visible: root.icon.length > 0
            anchors.verticalCenter: parent.verticalCenter
            name: root.icon
            size: Style.icon
            tone: "dim"
        }

        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: root.text
            role: "bodyStrong"
        }
    }

    Item {
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        width: parent.width * root._progress
        clip: true

        Row {
            width: baseContent.width
            height: baseContent.height
            x: root.contentX
            y: (root.height - height) / 2
            spacing: Style.s2

            Glyph {
                visible: root.icon.length > 0
                anchors.verticalCenter: parent.verticalCenter
                name: root.icon
                size: Style.icon
                tone: "onBone"
                filled: true
            }

            Label {
                anchors.verticalCenter: parent.verticalCenter
                text: root.text
                role: "bodyStrong"
                tone: "onBone"
            }
        }
    }

    MouseArea {
        id: pointer
        anchors.fill: parent
        enabled: root.enabled
        hoverEnabled: true
        acceptedButtons: Qt.LeftButton
        cursorShape: Qt.PointingHandCursor
        onPressed: {
            root.forceActiveFocus();
            root.beginHold();
        }
        onReleased: root.endHold()
        onCanceled: root.endHold()
    }

    NumberAnimation {
        id: filling
        target: root
        property: "_progress"
        to: 1
        easing.type: Easing.Linear
        onFinished: {
            if (!root._holding || root._fired)
                return;
            root._fired = true;
            root.activated();
            resetDelay.restart();
        }
    }

    NumberAnimation {
        id: drain
        target: root
        property: "_progress"
        to: 0
        easing.type: Easing.OutQuad
    }

    Timer {
        id: resetDelay
        interval: Math.max(1, Style.move)
        repeat: false
        onTriggered: {
            root._holding = false;
            drain.from = 1;
            drain.duration = Math.max(1, Style.move);
            drain.restart();
        }
    }

    Behavior on scale {
        enabled: !Style.still
        NumberAnimation { duration: Style.snap; easing.type: Easing.OutCubic }
    }
}

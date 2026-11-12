import QtQuick

Item {
    id: slider

    property real value: 0
    property string icon: ""
    property bool iconClickable: false
    property bool muted: false
    property real step: 0.05
    signal moved(real value)
    signal committed(real value)
    signal iconClicked()

    property bool dragging: false
    property real live: 0
    readonly property real shown: slider.dragging ? slider.live : Math.max(0, Math.min(1, slider.value))

    readonly property real gap: 6
    readonly property real handleW: 4
    readonly property real outer: Math.round(slider.height * 0.42)
    readonly property real usable: Math.max(1, slider.width - slider.handleW)
    readonly property real handleX: slider.usable * slider.shown

    implicitHeight: 48
    activeFocusOnTab: true
    opacity: slider.enabled ? 1 : 0.38

    function fracAt(x) { return Math.max(0, Math.min(1, (x - slider.handleW / 2) / slider.usable)) }
    function nudge(d) {
        var v = Math.max(0, Math.min(1, slider.shown + d));
        slider.moved(v);
        slider.committed(v);
    }

    Keys.onLeftPressed: slider.nudge(-slider.step)
    Keys.onRightPressed: slider.nudge(slider.step)
    Keys.onDownPressed: slider.nudge(-slider.step)
    Keys.onUpPressed: slider.nudge(slider.step)

    Rectangle {
        id: active
        anchors { left: parent.left; top: parent.top; bottom: parent.bottom }
        width: Math.max(0, slider.handleX - slider.gap)
        visible: width > 0.5
        topLeftRadius: slider.outer
        bottomLeftRadius: slider.outer
        topRightRadius: Math.min(Md.rXs, width / 2)
        bottomRightRadius: Math.min(Md.rXs, width / 2)
        color: slider.muted ? Md.inkFaint : Md.accent
        Behavior on color {
            enabled: !Md.still
            ColorAnimation { duration: Md.fast }
        }
    }

    Rectangle {
        id: inactive
        anchors { right: parent.right; top: parent.top; bottom: parent.bottom }
        width: Math.max(0, slider.width - slider.handleX - slider.handleW - slider.gap)
        visible: width > 0.5
        topRightRadius: slider.outer
        bottomRightRadius: slider.outer
        topLeftRadius: Math.min(Md.rXs, width / 2)
        bottomLeftRadius: Math.min(Md.rXs, width / 2)
        color: Md.containerHighest

        Rectangle {
            anchors { right: parent.right; rightMargin: slider.outer - 2; verticalCenter: parent.verticalCenter }
            width: 4
            height: 4
            radius: 2
            visible: inactive.width > slider.outer + 8
            color: Md.accent
        }
    }

    Rectangle {
        id: handle
        x: slider.handleX
        anchors.verticalCenter: parent.verticalCenter
        width: slider.handleW
        height: slider.height + (slider.dragging ? 10 : 6)
        radius: width / 2
        color: slider.muted ? Md.inkFaint : Md.accent
        Behavior on height {
            enabled: !Md.still
            NumberAnimation { duration: Md.fast }
        }
    }

    Icon {
        id: glyph
        visible: slider.icon.length > 0
        anchors { left: parent.left; leftMargin: Math.round((slider.height - size) / 2); verticalCenter: parent.verticalCenter }
        name: slider.icon
        size: 22
        filled: true
        color: active.width > glyph.x + glyph.width ? Md.onAccent : Md.ink
    }

    MouseArea {
        anchors.fill: parent
        anchors.topMargin: -6
        anchors.bottomMargin: -6
        cursorShape: Qt.PointingHandCursor
        preventStealing: true
        property bool iconPress: false
        onPressed: function (m) {
            iconPress = slider.iconClickable && m.x < slider.height;
            if (iconPress)
                return;
            slider.dragging = true;
            slider.live = slider.fracAt(m.x);
            slider.moved(slider.live);
        }
        onPositionChanged: function (m) {
            if (!slider.dragging)
                return;
            slider.live = slider.fracAt(m.x);
            slider.moved(slider.live);
        }
        onReleased: function (m) {
            if (iconPress) {
                iconPress = false;
                if (m.x < slider.height)
                    slider.iconClicked();
                return;
            }
            var v = slider.fracAt(m.x);
            slider.dragging = false;
            slider.committed(v);
        }
        onCanceled: {
            if (slider.dragging)
                slider.committed(slider.live);
            slider.dragging = false;
            iconPress = false;
        }
        // sliders sit in scrolling pages, so a plain vertical wheel scrolls the page; sideways or ctrl adjusts
        onWheel: function (w) {
            var sideways = w.angleDelta.x !== 0 && Math.abs(w.angleDelta.x) > Math.abs(w.angleDelta.y);
            if (!sideways && !(w.modifiers & Qt.ControlModifier) && !slider.activeFocus) {
                w.accepted = false;
                return;
            }
            var d = (sideways ? -w.angleDelta.x : w.angleDelta.y) / 120;
            if (d !== 0)
                slider.nudge(d * slider.step);
        }
    }

    Rectangle {
        anchors.fill: parent
        anchors.margins: -3
        visible: slider.activeFocus
        radius: slider.outer + 3
        color: "transparent"
        border.width: 2
        border.color: Md.accent
    }
}

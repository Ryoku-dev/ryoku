import QtQuick
import "Singletons"
import Ryoku.Ui.Singletons

// A labelled value slider: name on the left, live numeric readout on the right,
// a track with a filled portion and a draggable knob below. Click or drag
// anywhere on the row. Emits moved(value) continuously. Ryoshot palette.
Item {
    id: sl

    property string label: ""
    property real from: 0
    property real to: 100
    property real value: 0
    property int decimals: 0
    property string suffix: ""
    property bool bipolar: false
    signal moved(real v)
    signal released()

    implicitWidth: 240
    implicitHeight: 44

    readonly property color idle: Theme.inkDim
    readonly property color vermilion: Theme.accent
    readonly property real frac: sl.to > sl.from ? Math.max(0, Math.min(1, (sl.value - sl.from) / (sl.to - sl.from))) : 0

    function fmt(v) { return (sl.decimals > 0 ? v.toFixed(sl.decimals) : String(Math.round(v))) + sl.suffix; }
    function setFromX(x) {
        var t = track.width > 0 ? Math.max(0, Math.min(1, x / track.width)) : 0;
        sl.moved(sl.from + t * (sl.to - sl.from));
    }

    Text {
        id: lab
        anchors.left: parent.left
        anchors.top: parent.top
        text: sl.label
        color: sl.idle
        font.family: "Geologica"
        font.pixelSize: 12
        font.weight: Font.Medium
    }
    Text {
        anchors.right: parent.right
        anchors.top: parent.top
        text: sl.fmt(sl.value)
        color: ma.pressed ? sl.vermilion : Theme.ink
        font.family: Theme.mono
        font.pixelSize: 12
        font.weight: Font.Medium
    }

    Rectangle {
        id: track
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.bottomMargin: 6
        height: 4
        radius: 2
        color: Theme.hair

        Rectangle {
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            x: sl.bipolar ? Math.min(0.5, sl.frac) * parent.width : 0
            width: sl.bipolar ? Math.abs(sl.frac - 0.5) * parent.width : parent.width * sl.frac
            radius: 2
            color: sl.vermilion
        }
        Rectangle {
            id: knob
            width: 14
            height: 14
            radius: 7
            x: Math.max(0, Math.min(parent.width - width, parent.width * sl.frac - width / 2))
            anchors.verticalCenter: parent.verticalCenter
            color: Theme.ink
            border.width: ma.pressed ? 3 : 0
            border.color: Qt.rgba(Theme.accent.r, Theme.accent.g, Theme.accent.b, 0.4)
            scale: ma.pressed ? 1.15 : (ma.containsMouse ? 1.08 : 1)
            Behavior on scale { NumberAnimation { duration: 80 } }
        }
    }

    MouseArea {
        id: ma
        anchors.fill: parent
        hoverEnabled: true
        preventStealing: true
        function apply(e) { var p = mapToItem(track, e.x, e.y); sl.setFromX(p.x); }
        onPressed: (e) => apply(e)
        onPositionChanged: (e) => { if (pressed) apply(e); }
        onReleased: sl.released()
    }
}

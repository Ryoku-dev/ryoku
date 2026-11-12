import QtQuick
import QtQuick.Shapes

Item {
    id: wavy

    property real progress: 0
    property bool playing: false
    property bool seekable: false
    property color color: Md.accent
    property color trackColor: Md.containerHighest
    property real thickness: 4
    property real amplitude: 3
    property real wavelength: 26
    property real phase: 0
    signal seek(real fraction)

    property bool scrubbing: false
    property real scrub: 0
    readonly property real shown: wavy.scrubbing ? wavy.scrub : Math.max(0, Math.min(1, wavy.progress))
    readonly property real mid: height / 2
    readonly property real headX: (width - wavy.thickness) * wavy.shown + wavy.thickness / 2
    property real swing: wavy.playing ? wavy.amplitude : 0
    Behavior on swing {
        enabled: !Md.still
        NumberAnimation { duration: Md.standard }
    }

    readonly property var wave: {
        var pts = [];
        var end = Math.max(wavy.thickness / 2, wavy.headX);
        var steps = Math.max(2, Math.ceil(end / 3));
        for (var i = 0; i <= steps; i++) {
            var x = wavy.thickness / 2 + (end - wavy.thickness / 2) * i / steps;
            pts.push(Qt.point(x, wavy.mid
                + Math.sin((x - wavy.phase) * 2 * Math.PI / wavy.wavelength) * wavy.swing));
        }
        return pts;
    }

    implicitHeight: 24

    NumberAnimation on phase {
        from: 0
        to: wavy.wavelength
        duration: 1400
        loops: Animation.Infinite
        running: wavy.playing && wavy.visible && !Md.still && !wavy.scrubbing
    }

    Shape {
        anchors.fill: parent
        preferredRendererType: Shape.CurveRenderer

        ShapePath {
            strokeColor: wavy.trackColor
            strokeWidth: wavy.thickness
            fillColor: "transparent"
            capStyle: ShapePath.RoundCap
            PathPolyline {
                path: [Qt.point(Math.min(wavy.width - wavy.thickness / 2, wavy.headX + 10), wavy.mid),
                    Qt.point(wavy.width - wavy.thickness / 2, wavy.mid)]
            }
        }
        ShapePath {
            strokeColor: wavy.color
            strokeWidth: wavy.thickness
            fillColor: "transparent"
            capStyle: ShapePath.RoundCap
            joinStyle: ShapePath.RoundJoin
            PathPolyline { path: wavy.wave }
        }
    }

    Rectangle {
        x: wavy.headX - width / 2
        anchors.verticalCenter: parent.verticalCenter
        width: wavy.thickness
        height: wavy.scrubbing ? 22 : 16
        radius: width / 2
        color: wavy.color
        visible: wavy.seekable
    }

    MouseArea {
        anchors.fill: parent
        anchors.topMargin: -6
        anchors.bottomMargin: -6
        enabled: wavy.seekable
        cursorShape: Qt.PointingHandCursor
        preventStealing: true
        function at(x) { return Math.max(0, Math.min(1, x / Math.max(1, wavy.width))) }
        onPressed: function (m) { wavy.scrubbing = true; wavy.scrub = at(m.x); }
        onPositionChanged: function (m) { if (wavy.scrubbing) wavy.scrub = at(m.x); }
        onReleased: {
            if (wavy.scrubbing)
                wavy.seek(wavy.scrub);
            wavy.scrubbing = false;
        }
        onCanceled: wavy.scrubbing = false
    }
}

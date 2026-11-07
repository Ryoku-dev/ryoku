import QtQuick
import QtQuick.Shapes

Item {
    id: root

    property real progress: 0
    property bool playing: false
    property real amplitude: 2
    property real wavelength: 9
    property real lineWidth: 1.5
    property bool interactive: false
    property real phase: 0
    // The played wave is rebuilt point by point on every phase step. It drifts
    // 10 px a second, so the bar's shared 30 Hz motion clock moves it a third of
    // a pixel per step where an animation ran at the panel's refresh rate.
    readonly property bool drifting: root.playing && root.visible && root.amplitude > 0
        && !Style.still
    property double _origin: 0
    property bool _onClock: false

    function _syncClock(): void {
        if (root.drifting === root._onClock)
            return;
        MotionClock.users += root.drifting ? 1 : -1;
        root._onClock = root.drifting;
        if (root.drifting)
            root._origin = Date.now() - root.phase / Math.max(1, root.wavelength) * 900;
    }

    onDriftingChanged: root._syncClock()
    Component.onCompleted: root._syncClock()
    Component.onDestruction: {
        if (root._onClock)
            MotionClock.users -= 1;
    }

    Connections {
        target: root._onClock ? MotionClock : null
        function onStep(now: double): void {
            root.phase = (now - root._origin) % 900 / 900 * root.wavelength;
        }
    }

    signal seek(real frac)

    readonly property real clampedProgress: Math.max(0, Math.min(1, progress))
    readonly property real effectiveAmplitude: playing ? Math.max(0, amplitude) : 0
    readonly property real midpoint: height / 2
    readonly property real headX: width * clampedProgress
    readonly property var playedPoints: {
        if (headX <= 0)
            return [Qt.point(0, midpoint)];
        if (effectiveAmplitude <= 0)
            return [Qt.point(0, midpoint), Qt.point(headX, midpoint)];
        const points = [];
        const wave = Math.max(1, wavelength);
        const steps = Math.max(2, Math.ceil(headX));
        for (let index = 0; index <= steps; ++index) {
            const x = headX * index / steps;
            const y = midpoint + Math.sin((x + phase) * Math.PI * 2 / wave)
                * effectiveAmplitude;
            points.push(Qt.point(x, y));
        }
        return points;
    }

    implicitWidth: 120
    implicitHeight: Math.max(12, amplitude * 2 + 8)
    scale: pointer.pressed ? 0.98 : 1

    Shape {
        anchors.fill: parent
        preferredRendererType: Shape.CurveRenderer

        ShapePath {
            strokeColor: Style.lineSoft
            strokeWidth: root.lineWidth
            fillColor: "transparent"
            capStyle: ShapePath.RoundCap
            PathPolyline {
                path: [Qt.point(root.headX, root.midpoint),
                    Qt.point(root.width, root.midpoint)]
            }
        }

        ShapePath {
            strokeColor: Style.ink
            strokeWidth: root.lineWidth
            fillColor: "transparent"
            capStyle: ShapePath.RoundCap
            joinStyle: ShapePath.RoundJoin
            PathPolyline { path: root.playedPoints }
        }
    }

    Rectangle {
        width: 4
        height: 4
        radius: 2
        x: root.headX - width / 2
        y: root.midpoint - height / 2
        color: Style.ink
    }

    MouseArea {
        id: pointer
        anchors.fill: parent
        anchors.topMargin: -6
        anchors.bottomMargin: -6
        enabled: root.enabled && root.interactive
        hoverEnabled: true
        acceptedButtons: Qt.LeftButton
        preventStealing: true
        cursorShape: Qt.PointingHandCursor

        function send(pointerX) {
            root.seek(Math.max(0, Math.min(1, pointerX / Math.max(1, width))));
        }

        onPressed: mouse => send(mouse.x)
        onPositionChanged: mouse => {
            if (pressed)
                send(mouse.x);
        }
    }

    Behavior on scale {
        enabled: !Style.still
        NumberAnimation { duration: Style.snap; easing.type: Easing.OutCubic }
    }
}

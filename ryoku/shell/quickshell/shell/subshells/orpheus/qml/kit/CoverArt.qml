import QtQuick
import Quickshell.Widgets

Item {
    id: root

    property url source: ""
    property real size: 112
    property bool round: false
    property bool spinning: false
    property real _rotation: 0
    // One turn every 8 s, stepped by the bar's shared motion clock: at 45
    // degrees a second the rim moves under a pixel per 30 Hz step, and an
    // animation would redraw the bar at the panel's refresh rate all song long.
    readonly property bool turning: root.spinning && root.visible
        && image.status === Image.Ready && !Style.still
    property double _origin: 0
    property bool _onClock: false

    function _syncClock(): void {
        if (root.turning === root._onClock)
            return;
        MotionClock.users += root.turning ? 1 : -1;
        root._onClock = root.turning;
        if (root.turning)
            root._origin = Date.now() - root._rotation / 360 * 8000;
    }

    implicitWidth: size
    implicitHeight: size

    onTurningChanged: root._syncClock()
    Component.onCompleted: root._syncClock()
    Component.onDestruction: {
        if (root._onClock)
            MotionClock.users -= 1;
    }
    onSpinningChanged: {
        if (!spinning)
            _rotation = 0;
    }
    onVisibleChanged: {
        if (!visible)
            _rotation = 0;
    }

    Connections {
        target: root._onClock ? MotionClock : null
        function onStep(now: double): void {
            root._rotation = (now - root._origin) % 8000 / 8000 * 360;
        }
    }

    ClippingRectangle {
        id: artwork
        anchors.fill: parent
        radius: root.round ? width / 2 : Style.radius
        color: Style.tint5
        rotation: root._rotation

        Image {
            id: image
            readonly property int texels: Math.max(1,
                Math.round(root.size * Screen.devicePixelRatio * 2))
            anchors.fill: parent
            source: root.source
            sourceSize: Qt.size(texels, texels)
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
            smooth: true
            mipmap: true
            visible: status === Image.Ready
        }

        Glyph {
            anchors.centerIn: parent
            visible: image.status !== Image.Ready
            name: "music_note"
            size: Math.max(Style.icon, root.size * 0.34)
            tone: "dim"
        }
    }
}

pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Shapes as Shapes
import Lithic

// A transfer's progress in the design's own hand. Lithic and Fluent draw the
// daemon's 64-bucket piece map, so the parts that have landed show where they
// are; Material draws M3's wavy track, flowing only while bytes move; Adwaita a
// plain rounded level. An unknown size shows the map alone, never a guess.
Item {
    id: root

    property real s: 1
    // 0..1, or -1 while the size is unknown
    property real fraction: -1
    // 64 fills of 0..1 (format.js pieces), or empty
    property var pieces: []
    // bytes are moving now
    property bool active: false
    // "" | "paused" | "error" | "done"
    property string tone: ""

    // the state minerals: lapis becoming, jade landed, amber waiting, jasper failed
    readonly property color ink: root.tone === "error" ? Lithic.jasper
        : root.tone === "paused" ? Lithic.amber
        : root.tone === "done" ? Lithic.jade : Lithic.lapis
    readonly property color track: Lithic.alpha(Lithic.fg, Look.dark ? 0.14 : 0.16)
    readonly property bool mapped: (Look.lithic || Look.fluent) && root.pieces.length === 64
    readonly property real level: Math.max(0, Math.min(1, root.fraction))

    implicitHeight: Math.round((Look.material ? 8 : Look.lithic ? 6 : Look.adwaita ? 4 : 3) * root.s)
    implicitWidth: Math.round(160 * root.s)

    property real shown: 0
    Behavior on shown {
        NumberAnimation { duration: Look.dur(260); easing.type: Easing.OutCubic }
    }
    Component.onCompleted: root.shown = Qt.binding(() => root.level)

    // the piece map: one cell per bucket, lit by how much of it has landed
    Row {
        id: map
        anchors.fill: parent
        visible: root.mapped
        spacing: Math.max(1, Math.round(root.s))

        Repeater {
            model: root.mapped ? 64 : 0

            delegate: Rectangle {
                required property int index
                readonly property real fillLevel: Number(root.pieces[index] || 0)

                width: (map.width - map.spacing * 63) / 64
                height: map.height
                color: Lithic.mix(root.track, root.ink, fillLevel)
            }
        }
    }

    // a level, where there is no map to draw
    Item {
        anchors.fill: parent
        visible: !root.mapped && !Look.material

        Rectangle {
            anchors.fill: parent
            radius: Look.lithic ? 0 : height / 2
            color: root.track
        }
        Rectangle {
            width: Math.max(root.fraction < 0 ? 0 : height, parent.width * root.shown)
            height: parent.height
            radius: Look.lithic ? 0 : height / 2
            color: root.ink
            visible: root.fraction >= 0
        }
    }

    // Material: the wave for what is done, a flat track for what is left
    Item {
        id: wavy
        anchors.fill: parent
        visible: Look.material && !root.mapped

        readonly property real stroke: Math.max(2, Math.round(3 * root.s))
        readonly property real wavelength: 18 * root.s
        readonly property real amplitude: (root.height - wavy.stroke) / 2
        readonly property real gap: 4 * root.s
        readonly property real head: Math.max(0, wavy.width * root.shown)
        property real swell: root.active && !Look.still ? 1 : 0
        Behavior on swell {
            NumberAnimation { duration: Look.dur(360); easing.type: Easing.OutCubic }
        }

        Item {
            width: wavy.head
            height: parent.height
            clip: true
            visible: root.fraction >= 0

            Shapes.Shape {
                id: wave
                width: wavy.width + wavy.wavelength * 2
                height: parent.height
                preferredRendererType: Shapes.Shape.CurveRenderer

                NumberAnimation on x {
                    from: -wavy.wavelength
                    to: 0
                    duration: 1100
                    loops: Animation.Infinite
                    running: wavy.swell > 0.001 && wavy.visible && root.visible
                }

                Shapes.ShapePath {
                    strokeColor: root.ink
                    strokeWidth: wavy.stroke
                    fillColor: "transparent"
                    capStyle: Shapes.ShapePath.RoundCap
                    PathSvg {
                        path: {
                            const w = wavy.wavelength, a = wavy.amplitude * wavy.swell, mid = wave.height / 2;
                            const turns = Math.ceil(wave.width / w);
                            let d = "M 0 " + mid;
                            for (let i = 0; i < turns; i++) {
                                const x = i * w;
                                d += " Q " + (x + w / 4) + " " + (mid - 2 * a) + " " + (x + w / 2) + " " + mid;
                                d += " Q " + (x + w * 3 / 4) + " " + (mid + 2 * a) + " " + (x + w) + " " + mid;
                            }
                            return d;
                        }
                    }
                }
            }
        }
        Rectangle {
            x: root.fraction < 0 ? 0 : Math.min(parent.width, wavy.head + wavy.gap)
            anchors.verticalCenter: parent.verticalCenter
            width: Math.max(0, parent.width - x)
            height: wavy.stroke
            radius: height / 2
            color: root.track
        }
    }
}

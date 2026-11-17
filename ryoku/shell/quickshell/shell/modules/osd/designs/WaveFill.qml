pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import QtQuick.Shapes
import Lithic

// The Material level: a pill track filled from its round foot, the fill's
// surface a squiggle that flows while the pop-up is up and lies flat as it
// leaves, and always under reduced motion. The fill above 100% takes the
// error colour; a stop dot marks the track's end and, when the scale runs past
// 100%, another marks where 100% sits.
Item {
    id: root

    // how full the track is (0..1), and where 100% sits on it
    property real fraction: 0
    property real unity: 1
    property color ink: Lithic.sel
    property color overInk: Lithic.jasper
    property color trackInk: Lithic.container
    property color markInk: Lithic.sel
    property bool flowing: false
    property real u: 1

    property real wavelength: 14 * root.u
    property real amplitude: 1.8 * root.u
    property real dot: 4 * root.u
    readonly property real fillTop: root.height - Math.max(root.width, root.level * root.height)
    readonly property real unityY: root.height - root.unity * root.height
    readonly property bool overTop: root.level > root.unity + 0.002
    // a Material track this narrow is a pill
    readonly property real corner: Lithic.cornerRadius(LithicTokens.cut.large, root.width / root.u,
        root.height / root.u, "material") * root.u

    property real level: 0
    Behavior on level {
        NumberAnimation {
            duration: Lithic.duration(380)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: [0.05, 0.7, 0.1, 1, 1, 1]
        }
    }
    Component.onCompleted: root.level = Qt.binding(() => root.fraction)

    property real swell: root.flowing && !Lithic.reduceMotion ? 1 : 0
    Behavior on swell {
        NumberAnimation {
            duration: Lithic.duration(480)
            easing.type: Easing.OutCubic
        }
    }

    Rectangle {
        anchors.fill: parent
        radius: root.corner
        color: root.trackInk
    }

    Item {
        id: fill

        anchors.fill: parent
        layer.enabled: true
        layer.smooth: true
        layer.effect: MultiEffect {
            maskEnabled: true
            maskSource: mask
            maskThresholdMin: 0.5
            maskSpreadAtMin: 1
        }

        Rectangle {
            y: root.fillTop + root.amplitude * 0.5
            width: parent.width
            height: Math.max(0, root.height - y)
            color: root.ink
        }

        Rectangle {
            visible: root.overTop
            y: root.fillTop + root.amplitude * 0.5
            width: parent.width
            height: Math.max(0, root.unityY - y)
            color: root.overInk
        }

        Shape {
            id: wave

            y: root.fillTop - root.amplitude
            width: parent.width + root.wavelength * 2
            height: root.amplitude * 2 + 2 * root.u
            preferredRendererType: Shape.CurveRenderer
            transform: Scale {
                origin.y: root.amplitude
                yScale: root.swell
            }

            NumberAnimation on x {
                from: 0
                to: -root.wavelength
                duration: 1300
                loops: Animation.Infinite
                running: root.swell > 0.001 && root.visible
            }

            ShapePath {
                strokeColor: "transparent"
                fillColor: root.overTop ? root.overInk : root.ink
                PathSvg {
                    path: {
                        const w = root.wavelength, a = root.amplitude;
                        const turns = Math.ceil(wave.width / w);
                        let d = "M 0 " + a;
                        for (let i = 0; i < turns; i++) {
                            const x = i * w;
                            d += " Q " + (x + w / 4) + " " + (-a) + " " + (x + w / 2) + " " + a;
                            d += " Q " + (x + w * 3 / 4) + " " + (3 * a) + " " + (x + w) + " " + a;
                        }
                        return d + " L " + (turns * w) + " " + wave.height + " L 0 " + wave.height + " Z";
                    }
                }
            }
        }
    }

    Item {
        id: mask

        anchors.fill: parent
        visible: false
        layer.enabled: true

        Rectangle {
            anchors.fill: parent
            radius: root.corner
        }
    }

    Rectangle {
        id: stop
        anchors.horizontalCenter: parent.horizontalCenter
        y: root.width / 2 - height / 2
        width: root.dot
        height: width
        radius: width / 2
        color: root.fillTop < y + height ? Lithic.inkOn(root.overTop ? root.overInk : root.ink) : root.markInk
        visible: root.height > root.width * 1.5
    }

    Rectangle {
        anchors.horizontalCenter: parent.horizontalCenter
        visible: root.unity < 0.999 && root.height > root.width * 1.5
        y: root.unityY - height / 2
        width: root.dot
        height: width
        radius: width / 2
        color: root.fillTop < y ? Lithic.inkOn(root.ink) : Lithic.fg2
    }
}

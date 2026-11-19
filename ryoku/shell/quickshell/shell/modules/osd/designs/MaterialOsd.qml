pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Lithic
import "OsdGlyph.js" as OsdGlyph

// Material's pop-up, after Android 17's edge volume slider: a slim pill 12 from
// the right edge, vertically centred, that springs open from a dot. The value
// sits on top, a squiggle-topped level flows below it while it is up, and the
// filled symbol rides in the level's foot; muting bounces the symbol. The
// keyboard layout opens a horizontal pill at the same edge instead.
Item {
    id: root

    property string kind: "volume"
    property real u: 1
    property bool shown: false
    property real value: 0
    property real maxValue: 1
    property bool muted: false
    property string iconName: ""
    property string label: ""

    readonly property string placement: "right"
    readonly property real gap: Math.max(0, 12 * root.u - root.pad)
    readonly property real reveal: Math.max(0, Math.min(1, Math.max(root.across, root.along)))

    readonly property bool keyboard: root.kind === "keyboard"
    readonly property bool over: root.value > 1.005
    readonly property var planes: Lithic.planes("material")
    readonly property real pad: 10 * root.u
    readonly property real dot: 10 * root.u
    readonly property real fullWidth: root.keyboard
        ? Math.round(Math.max(160 * root.u, Math.min(320 * root.u, chipName.implicitWidth + 66 * root.u)))
        : Math.round(44 * root.u)
    readonly property real fullHeight: Math.round((root.keyboard ? 44 : 184) * root.u)
    readonly property color levelInk: root.muted ? Lithic.fg3 : Lithic.sel
    readonly property color overInk: root.muted ? Lithic.fg3 : Lithic.jasper

    implicitWidth: Math.ceil(root.fullWidth + root.pad * 2)
    implicitHeight: Math.ceil(root.fullHeight + root.pad * 2)

    // a dot that springs open, as Material 3 Expressive's spatial springs do:
    // across first, so the dot becomes a circle, then along into the pill, each
    // overshooting a little. Leaving runs it back: the pill to a circle, the
    // circle to nothing. Each step reads where its Behavior is heading, since a
    // binding on shown may not have updated yet when the Behavior starts.
    property real across: 0
    property real along: 0
    Behavior on across {
        id: acrossMotion
        SequentialAnimation {
            PauseAnimation { duration: Lithic.duration(acrossMotion.targetValue > 0.5 ? 0 : 90) }
            NumberAnimation {
                duration: Lithic.duration(acrossMotion.targetValue > 0.5 ? 380 : 170)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: acrossMotion.targetValue > 0.5
                    ? [0.42, 1.67, 0.21, 0.9, 1, 1] : [0.3, 0, 0.8, 0.15, 1, 1]
            }
        }
    }
    Behavior on along {
        id: alongMotion
        SequentialAnimation {
            PauseAnimation { duration: Lithic.duration(alongMotion.targetValue > 0.5 ? 70 : 0) }
            NumberAnimation {
                duration: Lithic.duration(alongMotion.targetValue > 0.5 ? 520 : 180)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: alongMotion.targetValue > 0.5
                    ? [0.38, 1.21, 0.22, 1, 1, 1] : [0.3, 0, 0.8, 0.15, 1, 1]
            }
        }
    }
    Component.onCompleted: {
        root.across = Qt.binding(() => root.shown ? 1 : 0);
        root.along = Qt.binding(() => root.shown ? 1 : 0);
    }
    readonly property real contentOpacity: Math.max(0, Math.min(1, (Math.min(root.across, root.along) - 0.6) / 0.3))

    RectangularShadow {
        anchors.fill: pill
        offset.y: 3 * root.u
        blur: 10 * root.u
        radius: pill.radius
        color: Lithic.alpha(Lithic.scrim, Lithic.dark ? 0.85 : 0.4)
        opacity: pill.opacity
    }

    Rectangle {
        id: pill

        readonly property real shortSide: root.dot + ((root.keyboard ? root.fullHeight : root.fullWidth) - root.dot) * root.across
        readonly property real longSide: Math.max(pill.shortSide,
            root.dot + ((root.keyboard ? root.fullWidth : root.fullHeight) - root.dot) * root.along)

        anchors.centerIn: parent
        width: Math.max(1, root.keyboard ? pill.longSide : pill.shortSide)
        height: Math.max(1, root.keyboard ? pill.shortSide : pill.longSide)
        radius: Lithic.cornerRadius(LithicTokens.cut.large, width / root.u, height / root.u, "material") * root.u
        color: root.planes.card
        opacity: Math.max(0, Math.min(1, root.reveal * 4))

        Item {
            id: face

            anchors.fill: parent
            visible: !root.keyboard
            opacity: root.contentOpacity

            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                y: Math.round(5 * root.u)
                text: Math.round(root.value * 100)
                color: root.muted ? Lithic.fg3 : root.over ? Lithic.jasper : Lithic.fg
                font.family: Lithic.font.sans
                font.weight: Font.DemiBold
                font.pixelSize: Math.round(12.5 * root.u)
                font.features: ({ "tnum": 1 })
                renderType: Text.CurveRendering
            }

            WaveFill {
                id: level
                x: Math.round(6 * root.u)
                y: Math.round(26 * root.u)
                width: parent.width - x * 2
                height: Math.max(width, parent.height - y - x)
                u: root.u
                fraction: Math.max(0, Math.min(1, root.value / root.maxValue))
                unity: 1 / root.maxValue
                ink: root.levelInk
                overInk: root.overInk
                markInk: root.levelInk
                trackInk: root.muted ? Lithic.alpha(Lithic.fg3, 0.16) : Lithic.container
                flowing: root.shown && !root.muted
            }

            Item {
                id: foot
                x: level.x
                y: level.y + level.height - level.width
                width: level.width
                height: level.width

                Text {
                    id: symbol
                    anchors.centerIn: parent
                    text: OsdGlyph.symbol(root.iconName)
                    color: Lithic.inkOn(root.levelInk)
                    font.family: "Material Symbols Rounded"
                    font.variableAxes: ({ "FILL": 1, "wght": 500 })
                    font.pixelSize: Math.round(18 * root.u)
                    renderType: Text.CurveRendering
                }
            }
        }

        Row {
            anchors.verticalCenter: parent.verticalCenter
            x: Math.round(6 * root.u)
            visible: root.keyboard
            spacing: 10 * root.u
            opacity: root.contentOpacity

            Rectangle {
                width: Math.round(32 * root.u)
                height: width
                radius: width / 2
                color: Lithic.sel

                Text {
                    anchors.centerIn: parent
                    text: "keyboard"
                    color: Lithic.onSel
                    font.family: "Material Symbols Rounded"
                    font.variableAxes: ({ "FILL": 1, "wght": 500 })
                    font.pixelSize: Math.round(18 * root.u)
                    renderType: Text.CurveRendering
                }
            }
            Text {
                id: chipName
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, 320 * root.u - 66 * root.u)
                elide: Text.ElideRight
                text: root.label
                color: Lithic.fg
                font.family: Lithic.font.sans
                font.weight: Font.DemiBold
                font.pixelSize: Math.round(14 * root.u)
                renderType: Text.CurveRendering
            }
        }
    }

    // a mute or unmute bounces the symbol: a squash and a wobble that springs back
    onMutedChanged: if (root.shown && !Lithic.reduceMotion) bounce.restart()
    SequentialAnimation {
        id: bounce
        ParallelAnimation {
            NumberAnimation { target: symbol; property: "scale"; to: 1.28; duration: 110; easing.type: Easing.OutQuad }
            NumberAnimation { target: symbol; property: "rotation"; to: -14; duration: 110; easing.type: Easing.OutQuad }
        }
        ParallelAnimation {
            NumberAnimation {
                target: symbol; property: "scale"; to: 1; duration: 420
                easing.type: Easing.BezierSpline; easing.bezierCurve: [0.42, 1.67, 0.21, 0.9, 1, 1]
            }
            NumberAnimation {
                target: symbol; property: "rotation"; to: 0; duration: 520
                easing.type: Easing.BezierSpline; easing.bezierCurve: [0.34, 2.2, 0.4, 0.8, 1, 1]
            }
        }
    }
}

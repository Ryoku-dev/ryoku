pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import QtQuick.Shapes
import Lithic
import "OsdGlyph.js" as OsdGlyph

// Lithic's pop-up is a drawer tab: anchored to the top edge with no gap, so it
// sits right under the bar's zone and slides out from under it. One row: the
// symbol, a single-row tick ruler and a rolling zero-padded figure. Flat where it
// meets the bar, its lower corners cut, a hairline down the sides and along the
// bottom only. It emerges downward from its own top edge, its ticks light in a
// stagger, its figure rolls, one restrained shine crosses it, and it retracts up.
// The keyboard layout shows the symbol and the layout's name.
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

    readonly property string placement: "top"
    readonly property real gap: 0
    readonly property real reveal: root.drop

    readonly property bool keyboard: root.kind === "keyboard"
    readonly property bool over: root.value > 1.005
    readonly property real pad: 10 * root.u
    readonly property real cut: 6 * root.u
    readonly property real side: 12 * root.u
    readonly property real below: 14 * root.u
    readonly property real hairline: Math.max(1, Math.round(root.u))
    readonly property real tabWidth: root.keyboard
        ? Math.round(Math.max(160 * root.u, Math.min(320 * root.u,
            root.pad + 16 * root.u + 8 * root.u + layoutName.implicitWidth + 12 * root.u)))
        : Math.round(232 * root.u)
    readonly property real tabHeight: Math.round(30 * root.u)

    implicitWidth: Math.ceil(root.tabWidth + root.side * 2)
    implicitHeight: Math.ceil(root.tabHeight + root.below)

    property real drop: 0
    Behavior on drop {
        id: dropMotion
        NumberAnimation {
            duration: Lithic.duration(dropMotion.targetValue > 0.5 ? 180 : 150)
            easing.type: dropMotion.targetValue > 0.5 ? Easing.OutCubic : Easing.InCubic
        }
    }

    // the shine crosses once per landing, never while the tab only rests
    property real shine: -0.3
    SequentialAnimation {
        id: shineRun
        PauseAnimation { duration: 140 }
        NumberAnimation {
            target: root
            property: "shine"
            from: -0.3
            to: 1.3
            duration: 760
            easing.type: Easing.OutCubic
        }
    }
    function land() {
        if (root.shown && !Lithic.reduceMotion)
            shineRun.restart();
    }
    onShownChanged: root.land()
    Component.onCompleted: {
        root.drop = Qt.binding(() => root.shown ? 1 : 0);
        root.land();
    }

    // the window's top edge clips the tab as it slides out from under the bar
    Item {
        id: tab

        x: root.side
        y: -Math.ceil(root.tabHeight + root.below) * (1 - root.drop)
        width: root.tabWidth
        height: root.tabHeight

        RectangularShadow {
            anchors.fill: parent
            anchors.topMargin: root.tabHeight / 2
            offset.y: 3 * root.u
            blur: 10 * root.u
            radius: root.cut
            color: Lithic.alpha(Lithic.scrim, Lithic.dark ? 0.8 : 0.35)
        }

        CutRect {
            anchors.fill: parent
            design: "lithic"
            cutTopLeft: 0
            cutTopRight: 0
            cutBottomLeft: root.cut
            cutBottomRight: root.cut
            fill: Lithic.planes("lithic").pane
        }

        // a hairline down the sides and along the bottom; the top is the bar's
        Shape {
            anchors.fill: parent
            preferredRendererType: Shape.CurveRenderer

            ShapePath {
                strokeColor: Lithic.planes("lithic").paneStroke
                strokeWidth: root.hairline
                fillColor: "transparent"
                PathPolyline {
                    path: {
                        const w = tab.width, t = tab.height, c = root.cut, h = root.hairline / 2;
                        return [Qt.point(h, 0), Qt.point(h, t - c), Qt.point(c, t - h),
                            Qt.point(w - c, t - h), Qt.point(w - h, t - c), Qt.point(w - h, 0)];
                    }
                }
            }
        }

        Shape {
            anchors.fill: parent
            visible: root.shine > -0.25 && root.shine < 1.25
            preferredRendererType: Shape.CurveRenderer

            ShapePath {
                strokeColor: "transparent"
                fillGradient: LinearGradient {
                    x1: root.shine * tab.width - 22 * root.u
                    y1: 0
                    x2: root.shine * tab.width + 22 * root.u
                    y2: 12 * root.u
                    GradientStop { position: 0; color: "transparent" }
                    GradientStop {
                        position: 0.5
                        color: Lithic.dark ? Lithic.alpha(Lithic.fg, 0.1) : Lithic.alpha(Lithic.stone4, 0.9)
                    }
                    GradientStop { position: 1; color: "transparent" }
                }
                PathPolyline {
                    path: [
                        Qt.point(0, 0), Qt.point(tab.width, 0),
                        Qt.point(tab.width, tab.height - root.cut),
                        Qt.point(tab.width - root.cut, tab.height),
                        Qt.point(root.cut, tab.height), Qt.point(0, tab.height - root.cut),
                        Qt.point(0, 0)
                    ]
                }
            }
        }

        Text {
            id: glyph
            x: root.pad - 1 * root.u
            anchors.verticalCenter: parent.verticalCenter
            text: OsdGlyph.symbol(root.iconName)
            color: root.muted ? Lithic.jasper : Lithic.fg2
            font.family: Lithic.font.icons
            font.pixelSize: Math.round(16 * root.u)
            renderType: Text.CurveRendering
        }

        TickRuler {
            visible: !root.keyboard
            anchors.left: glyph.right
            anchors.leftMargin: 9 * root.u
            anchors.right: figure.left
            anchors.rightMargin: 10 * root.u
            anchors.verticalCenter: parent.verticalCenter
            height: implicitHeight
            u: root.u
            value: root.value
            maxValue: root.maxValue
            muted: root.muted
        }

        Row {
            id: figure
            visible: !root.keyboard
            anchors.right: parent.right
            anchors.rightMargin: root.pad
            anchors.verticalCenter: parent.verticalCenter
            spacing: 1 * root.u

            OdometerFigure {
                anchors.verticalCenter: parent.verticalCenter
                value: Math.round(root.value * 100)
                size: Math.round(15 * root.u)
                duration: 260
                ink: root.muted ? Lithic.fg3 : root.over ? Lithic.jasper : Lithic.fg
                quietInk: root.over && !root.muted ? Lithic.alpha(Lithic.jasper, 0.45) : Lithic.alpha(Lithic.fg3, 0.5)
            }
            Text {
                y: Math.round(4 * root.u)
                text: "%"
                color: root.over && !root.muted ? Lithic.jasper : Lithic.fg3
                font.family: Lithic.font.mono
                font.pixelSize: Math.round(7 * root.u)
                renderType: Text.CurveRendering
            }
        }

        Text {
            id: layoutName
            visible: root.keyboard
            anchors.left: glyph.right
            anchors.leftMargin: 8 * root.u
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, root.tabWidth - x - 12 * root.u)
            text: root.label
            elide: Text.ElideRight
            color: Lithic.fg
            font.family: Lithic.font.sans
            font.weight: Font.Medium
            font.pixelSize: Math.round(13 * root.u)
            renderType: Text.CurveRendering
        }
    }
}

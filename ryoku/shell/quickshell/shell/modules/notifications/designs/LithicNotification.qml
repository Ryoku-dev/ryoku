pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import QtQuick.Shapes
import Lithic

// Lithic's notification: a cut slip of paper, its top-left and bottom-right
// corners cut, drawn on the pane with a hairline. The header is engraved in
// mono capitals (APP · NOW), the summary and body sit in Geologica, and the
// actions are small cut plaques. Its lifespan burns along the inner foot as a
// fuse of fine ticks that go dark from the right, the burning tick lit in the
// accent. A critical card wears a jasper URGENT stamp and no fuse. It slides in
// from its edge as it fades, then one restrained engraved shine crosses it.
LookCard {
    id: root

    readonly property real u: root.us
    readonly property var planes: Lithic.planes("lithic")
    readonly property real cut: 8 * root.u
    readonly property real hairline: Math.max(1, Math.round(root.u))
    readonly property real pad: 14 * root.u
    readonly property real travel: 24 * root.u
    readonly property bool fused: root.counting && !root.critical
    readonly property real footHeight: root.fused ? 16 * root.u : root.pad

    contentHeight: Math.ceil(content.y + content.height + root.footHeight)
    bodyLines: bodyMeasure.lineCount
    expandMs: 240
    drains: !root.critical
    lifeSteps: 64

    enterMotion: NumberAnimation {
        target: root
        property: "arrive"
        from: 0
        to: 1
        duration: 220
        easing.type: Easing.BezierSpline
        easing.bezierCurve: [0, 0, 0.2, 1, 1, 1]
    }
    exitMotion: NumberAnimation {
        target: root
        property: "gone"
        to: 1
        duration: 160
        easing.type: Easing.BezierSpline
        easing.bezierCurve: [0.4, 0, 1, 1, 1, 1]
    }

    // the shine crosses once, after the slip has landed
    property real shine: -0.4
    onLanded: shineRun.restart()
    SequentialAnimation {
        id: shineRun
        PauseAnimation { duration: 60 }
        NumberAnimation {
            target: root
            property: "shine"
            from: -0.4
            to: 1.4
            duration: 760
            easing.type: Easing.OutCubic
        }
    }

    Text {
        id: bodyMeasure
        visible: false
        width: bodyText.width
        text: root.body
        font: bodyText.font
        lineHeight: bodyText.lineHeight
        wrapMode: Text.WrapAtWordBoundaryOrAnywhere
    }

    // a header affordance: a glyph that brightens, on a cut chip that rises on hover
    component GlyphButton: Item {
        id: button
        property string glyph: ""
        property real turn: 0
        signal clicked()

        width: Math.round(22 * root.u)
        height: width

        CutRect {
            anchors.fill: parent
            design: "lithic"
            cut: LithicTokens.cut.token * root.u
            fill: Lithic.alpha(Lithic.fg, 0.09 * button.hover)
        }
        property real hover: hoverHandler.hovered ? 1 : 0
        Behavior on hover { NumberAnimation { duration: Lithic.motion.hover } }

        Text {
            anchors.centerIn: parent
            text: button.glyph
            color: Lithic.mix(Lithic.fg3, Lithic.fg, button.hover)
            font.family: Lithic.font.icons
            font.pixelSize: Math.round(15 * root.u)
            rotation: button.turn
            renderType: Text.CurveRendering
            Behavior on rotation { NumberAnimation { duration: Lithic.duration(220); easing.type: Easing.BezierSpline; easing.bezierCurve: Lithic.curve } }
        }
        HoverHandler {
            id: hoverHandler
            cursorShape: Qt.PointingHandCursor
        }
        TapHandler {
            onTapped: button.clicked()
        }
    }

    Item {
        id: slip

        width: root.width
        height: root.height
        opacity: Math.max(0, Math.min(1, root.arrive)) * (1 - root.gone)
        transform: Translate {
            x: root.edgeX * (root.travel * (1 - root.arrive) + root.travel * 1.4 * root.gone)
            y: root.edgeY * (root.travel * (1 - root.arrive) + root.travel * 1.4 * root.gone)
        }

        RectangularShadow {
            visible: root.shadows
            anchors.fill: parent
            anchors.margins: root.cut / 2
            offset.y: 8 * root.u
            blur: 24 * root.u
            spread: -2 * root.u
            radius: root.cut / 2
            color: Lithic.alpha(Lithic.scrim, Lithic.dark ? 0.85 : 0.32)
        }

        CutRect {
            anchors.fill: parent
            design: "lithic"
            cutTopLeft: root.cut
            cutTopRight: 0
            cutBottomRight: root.cut
            cutBottomLeft: 0
            fill: root.planes.pane
            strokeColor: root.critical ? Lithic.alpha(Lithic.jasper, 0.7) : root.planes.paneStroke
            strokeWidth: root.hairline
        }

        Shape {
            anchors.fill: parent
            visible: root.shine > -0.3 && root.shine < 1.3
            preferredRendererType: Shape.CurveRenderer

            ShapePath {
                strokeColor: "transparent"
                // a narrow glint, as light catching an engraved edge, not a wash
                fillGradient: LinearGradient {
                    x1: root.shine * slip.width - 30 * root.u
                    y1: 0
                    x2: root.shine * slip.width + 30 * root.u
                    y2: 30 * root.u
                    GradientStop { position: 0.38; color: "transparent" }
                    GradientStop { position: 0.5; color: Lithic.dark ? Lithic.alpha(Lithic.fg, 0.13) : Lithic.alpha(Lithic.stone4, 0.95) }
                    GradientStop { position: 0.62; color: "transparent" }
                }
                PathPolyline {
                    path: [
                        Qt.point(root.cut, 0), Qt.point(slip.width, 0),
                        Qt.point(slip.width, slip.height - root.cut),
                        Qt.point(slip.width - root.cut, slip.height),
                        Qt.point(0, slip.height), Qt.point(0, root.cut),
                        Qt.point(root.cut, 0)
                    ]
                }
            }
        }

        // header: APP · NOW in engraved mono capitals, the stamp, then the controls
        Item {
            id: header
            x: root.pad
            y: Math.round(10 * root.u)
            width: slip.width - root.pad * 2 + 6 * root.u
            height: Math.round(22 * root.u)

            Row {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.max(0, controls.x - 8 * root.u)
                spacing: 8 * root.u
                clip: true

                Text {
                    id: appLabel
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(implicitWidth, parent.width - timeLabel.implicitWidth - (stamp.visible ? stamp.width + parent.spacing : 0) - parent.spacing)
                    text: (root.appName || "·").toUpperCase()
                    color: Lithic.fg2
                    elide: Text.ElideRight
                    font.family: Lithic.font.mono
                    font.pixelSize: Math.round(10 * root.u)
                    font.letterSpacing: 1.4 * root.u
                    renderType: Text.CurveRendering
                }
                Text {
                    id: timeLabel
                    anchors.verticalCenter: parent.verticalCenter
                    visible: root.timeText !== ""
                    text: "· " + root.timeText.toUpperCase()
                    color: Lithic.fg3
                    font.family: Lithic.font.mono
                    font.pixelSize: Math.round(10 * root.u)
                    font.letterSpacing: 1.4 * root.u
                    renderType: Text.CurveRendering
                }
                CutRect {
                    id: stamp
                    anchors.verticalCenter: parent.verticalCenter
                    visible: root.critical
                    width: Math.ceil(stampText.implicitWidth + 10 * root.u)
                    height: Math.round(16 * root.u)
                    design: "lithic"
                    cut: LithicTokens.cut.token * root.u
                    fill: Lithic.jasper

                    Text {
                        id: stampText
                        anchors.centerIn: parent
                        text: Lithic.tr("Urgent").toUpperCase()
                        color: Lithic.inkOn(Lithic.jasper)
                        font.family: Lithic.font.mono
                        font.pixelSize: Math.round(9 * root.u)
                        font.letterSpacing: 1.2 * root.u
                        renderType: Text.CurveRendering
                    }
                }
            }

            Row {
                id: controls
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 2 * root.u

                GlyphButton {
                    visible: root.defaultAction !== null
                    glyph: "open_in_new"
                    onClicked: root.openDefault()
                }
                GlyphButton {
                    visible: root.expandable
                    glyph: "expand_more"
                    turn: root.expanded ? 180 : 0
                    onClicked: root.toggleExpanded()
                }
                GlyphButton {
                    glyph: "close"
                    onClicked: root.closeRequested()
                }
            }
        }

        Column {
            id: content
            x: root.pad
            y: header.y + header.height + 6 * root.u
            width: slip.width - root.pad * 2
            spacing: 12 * root.u

            Row {
                width: parent.width
                spacing: 12 * root.u

                Item {
                    id: art
                    readonly property bool hasImage: picture.status === Image.Ready
                    readonly property bool hasIcon: !art.hasImage && icon.status === Image.Ready
                    // The box and the decode size stay put, and the picture is not
                    // masked through a layer. Growing the box when the image
                    // becomes ready redecodes it, and a layer mask on that
                    // image keeps the shell thread inside polish. Either one
                    // stops the dismiss timer.
                    visible: root.showImages
                    width: Math.round(44 * root.u)
                    height: width

                    CutRect {
                        anchors.fill: parent
                        design: "lithic"
                        cut: LithicTokens.cut.small * root.u
                        fill: root.critical ? Lithic.alpha(Lithic.jasper, 0.14) : Lithic.stone3
                        strokeColor: root.planes.paneStroke
                        strokeWidth: root.hairline
                    }
                    Image {
                        id: picture
                        anchors.fill: parent
                        source: root.showImages ? root.imageSource : ""
                        sourceSize: Qt.size(Math.round(88 * root.u), Math.round(88 * root.u))
                        fillMode: Image.PreserveAspectCrop
                        asynchronous: true
                        smooth: true
                        cache: true
                    }
                    Image {
                        id: icon
                        anchors.centerIn: parent
                        width: Math.round(22 * root.u)
                        height: width
                        visible: art.hasIcon
                        source: root.showImages ? root.appIconSource : ""
                        sourceSize: Qt.size(Math.round(44 * root.u), Math.round(44 * root.u))
                        fillMode: Image.PreserveAspectFit
                        asynchronous: true
                        smooth: true
                    }
                    Text {
                        anchors.centerIn: parent
                        visible: !art.hasImage && !art.hasIcon
                        text: root.critical ? "priority_high" : "notifications"
                        color: root.critical ? Lithic.jasper : Lithic.fg2
                        font.family: Lithic.font.icons
                        font.pixelSize: Math.round(18 * root.u)
                        renderType: Text.CurveRendering
                    }
                }

                Column {
                    width: parent.width - (art.visible ? art.width + parent.spacing : 0)
                    spacing: 3 * root.u

                    Text {
                        width: parent.width
                        visible: text !== ""
                        text: root.summary
                        color: Lithic.fg
                        font.family: Lithic.font.sans
                        font.weight: Font.Medium
                        font.pixelSize: 14 * root.u
                        wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                        renderType: Text.CurveRendering
                    }
                    Text {
                        id: bodyText
                        width: parent.width
                        visible: text !== ""
                        text: root.body
                        color: Lithic.fg2
                        font.family: Lithic.font.sans
                        font.pixelSize: 12.5 * root.u
                        lineHeight: 1.12
                        wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                        maximumLineCount: root.showFull ? 9999 : root.compactLines
                        elide: root.showFull ? Text.ElideNone : Text.ElideRight
                        renderType: Text.CurveRendering
                    }
                }
            }

            Flow {
                id: actionRow
                width: parent.width
                spacing: 6 * root.u
                visible: root.visibleActions.length > 0 && root.showFull
                opacity: root.showFull ? 1 : 0
                Behavior on opacity { NumberAnimation { duration: Lithic.duration(160) } }

                Repeater {
                    model: root.visibleActions

                    delegate: Item {
                        id: plaque
                        required property var modelData
                        required property int index
                        // the first action is the gold plaque, the rest tonal, as Plaque draws them
                        readonly property bool primary: plaque.index === 0
                        readonly property color base: plaque.primary ? Lithic.gold : Lithic.field
                        readonly property color lit: plaque.primary ? Lithic.hoverFill(Lithic.gold)
                            : Lithic.mix(Lithic.field, Lithic.fg, LithicTokens.alpha.hoverPlane)
                        property real hover: plaqueHover.hovered ? 1 : 0
                        Behavior on hover { NumberAnimation { duration: Lithic.motion.hover } }

                        width: Math.ceil(Math.min(actionRow.width, plaqueText.implicitWidth + 24 * root.u))
                        height: Math.round(26 * root.u)

                        CutRect {
                            anchors.fill: parent
                            design: "lithic"
                            cut: LithicTokens.cut.small * root.u
                            fill: Lithic.mix(plaque.base, plaque.lit, plaque.hover)
                        }
                        Text {
                            id: plaqueText
                            anchors.centerIn: parent
                            width: Math.min(implicitWidth, plaque.width - 16 * root.u)
                            text: plaque.modelData.text
                            color: plaque.primary ? Lithic.onGold : Lithic.fg
                            elide: Text.ElideRight
                            font.family: Lithic.font.sans
                            font.weight: Font.Medium
                            font.pixelSize: 12 * root.u
                            renderType: Text.CurveRendering
                        }
                        HoverHandler {
                            id: plaqueHover
                            cursorShape: Qt.PointingHandCursor
                        }
                        TapHandler {
                            onTapped: root.invoke(plaque.modelData)
                        }
                    }
                }
            }
        }

        // the fuse: fine ticks along the inner foot that go dark from the right,
        // one at a time, the burning one lit in the accent
        Item {
            id: fuse
            readonly property int count: root.lifeSteps
            readonly property real pitch: (slip.width - root.pad * 2 - root.cut) / (fuse.count - 1)
            readonly property int lit: fuse.count - root.lifeStep

            visible: root.fused
            x: root.pad
            y: slip.height - Math.round(10 * root.u)
            width: (fuse.count - 1) * fuse.pitch + root.hairline
            height: Math.round(4 * root.u)

            Repeater {
                model: fuse.count
                delegate: Rectangle {
                    id: tick
                    required property int index
                    readonly property bool burning: tick.index === fuse.lit - 1 && fuse.lit < fuse.count
                    x: Math.round(tick.index * fuse.pitch)
                    y: tick.burning || tick.index % 8 === 0 ? 0 : Math.round(fuse.height * 0.4)
                    width: root.hairline
                    height: fuse.height - y
                    color: tick.burning ? Lithic.sel
                        : tick.index < fuse.lit ? Lithic.fg3 : Lithic.alpha(Lithic.fg3, 0.22)
                }
            }
        }
    }
}

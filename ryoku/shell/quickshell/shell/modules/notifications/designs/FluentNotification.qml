pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import QtQuick.Shapes
import Lithic

// Fluent's notification, after the Windows 11 toast: a softly rounded card with
// a layered outline over a faintly tinted acrylic fill and its grain (one small
// static tile). A header of the app's icon, its name and the time; the summary
// in semibold over two lines of body; a hero image across the card when there
// is one; actions as Fluent's standard buttons, sharing the row equally. It
// slides in from its edge on Fluent's decelerate curve, and as it lands one
// reveal light runs once round its outline. A critical card's outline and
// header mark take the error colour.
LookCard {
    id: root

    readonly property real u: root.us
    readonly property real pad: 14 * root.u
    readonly property real travel: 48 * root.u
    readonly property real corner: Lithic.cornerRadius(LithicTokens.cut.large, width / root.u, height / root.u, "fluent") * root.u
    readonly property real control: Lithic.cornerRadius(LithicTokens.cut.small, 32, 32, "fluent") * root.u
    readonly property real hairline: Math.max(1, Math.round(root.u))
    readonly property var planes: Lithic.planes("fluent")
    readonly property bool hero: root.showImages && heroImage.status === Image.Ready

    contentHeight: Math.ceil(content.y + content.height + root.pad)
    bodyLines: bodyMeasure.lineCount
    expandMs: 250
    expandCurve: [0, 0, 0, 1, 1, 1]

    enterMotion: NumberAnimation {
        target: root
        property: "arrive"
        from: 0
        to: 1
        duration: 300
        easing.type: Easing.BezierSpline
        easing.bezierCurve: [0, 0, 0, 1, 1, 1]
    }
    exitMotion: NumberAnimation {
        target: root
        property: "gone"
        to: 1
        duration: 200
        easing.type: Easing.BezierSpline
        easing.bezierCurve: [0.7, 0, 1, 0.5, 1, 1]
    }

    // the reveal light, once round the outline after landing
    property real light: 0
    onLanded: lightRun.restart()
    NumberAnimation {
        id: lightRun
        target: root
        property: "light"
        from: 0
        to: 1
        duration: 820
        easing.type: Easing.InOutSine
    }

    Text {
        id: bodyMeasure
        visible: false
        width: bodyText.width
        text: root.body
        font: bodyText.font
        wrapMode: Text.WrapAtWordBoundaryOrAnywhere
    }

    // one stroke of the reveal light: a dash of `span` whose far end is the head
    component Trail: ShapePath {
        id: trail
        property real span: 60
        property real sw: 1
        fillColor: "transparent"
        strokeWidth: trail.sw
        strokeStyle: ShapePath.DashLine
        capStyle: ShapePath.FlatCap
        dashPattern: [trail.span / trail.sw, reveal.perim / trail.sw]
        dashOffset: (trail.span - reveal.head) / trail.sw
        startX: reveal.o + reveal.r
        startY: reveal.o
        PathLine { x: reveal.o + reveal.w - reveal.r; y: reveal.o }
        PathArc { x: reveal.o + reveal.w; y: reveal.o + reveal.r; radiusX: reveal.r; radiusY: reveal.r }
        PathLine { x: reveal.o + reveal.w; y: reveal.o + reveal.h - reveal.r }
        PathArc { x: reveal.o + reveal.w - reveal.r; y: reveal.o + reveal.h; radiusX: reveal.r; radiusY: reveal.r }
        PathLine { x: reveal.o + reveal.r; y: reveal.o + reveal.h }
        PathArc { x: reveal.o; y: reveal.o + reveal.h - reveal.r; radiusX: reveal.r; radiusY: reveal.r }
        PathLine { x: reveal.o; y: reveal.o + reveal.r }
        PathArc { x: reveal.o + reveal.r; y: reveal.o; radiusX: reveal.r; radiusY: reveal.r }
    }

    // Fluent's subtle button: transparent at rest, a soft fill under the pointer
    component SubtleButton: Item {
        id: button
        property string glyph: ""
        property real turn: 0
        signal clicked()

        width: Math.round(28 * root.u)
        height: width

        property real hover: hoverHandler.hovered ? 1 : 0
        Behavior on hover { NumberAnimation { duration: Lithic.duration(83) } }

        Rectangle {
            anchors.fill: parent
            radius: root.control
            color: Lithic.alpha(Lithic.fg, (tap.pressed ? 0.04 : 0.07) * button.hover)
        }
        Text {
            anchors.centerIn: parent
            text: button.glyph
            color: tap.pressed ? Lithic.fg3 : Lithic.fg2
            font.family: "Material Symbols Outlined"
            font.variableAxes: ({ "wght": 300 })
            font.pixelSize: Math.round(17 * root.u)
            rotation: button.turn
            renderType: Text.CurveRendering
            Behavior on rotation { NumberAnimation { duration: Lithic.duration(167); easing.type: Easing.BezierSpline; easing.bezierCurve: [0, 0, 0, 1, 1, 1] } }
        }
        HoverHandler {
            id: hoverHandler
            cursorShape: Qt.PointingHandCursor
        }
        TapHandler {
            id: tap
            onTapped: button.clicked()
        }
    }

    Item {
        id: toast

        width: root.width
        height: root.height
        opacity: Math.max(0, Math.min(1, root.arrive * 2.2)) * (1 - root.gone)
        transform: Translate {
            x: root.edgeX * root.travel * (1 - root.arrive + 0.6 * root.gone)
            y: root.edgeY * root.travel * (1 - root.arrive + 0.6 * root.gone)
        }

        RectangularShadow {
            visible: root.shadows
            anchors.fill: parent
            offset.y: 8 * root.u
            blur: 28 * root.u
            radius: root.corner
            color: Lithic.alpha(Lithic.scrim, Lithic.dark ? 0.6 : 0.18)
        }

        Rectangle {
            anchors.fill: parent
            radius: root.corner
            color: Lithic.mix(root.planes.card, Lithic.sel, Lithic.dark ? 0.05 : 0.035)
            border.width: root.hairline
            border.color: root.critical ? Lithic.alpha(Lithic.jasper, 0.7) : root.planes.cardStroke
        }

        // the acrylic grain: one static tile, kept off the rounded corners
        Image {
            x: root.corner
            y: root.hairline
            width: toast.width - root.corner * 2
            height: root.corner - root.hairline
            source: "fluent-noise.png"
            fillMode: Image.Tile
            smooth: false
        }
        Image {
            x: root.hairline
            y: root.corner
            width: toast.width - root.hairline * 2
            height: toast.height - root.corner * 2
            source: "fluent-noise.png"
            fillMode: Image.Tile
            smooth: false
        }
        Image {
            x: root.corner
            y: toast.height - root.corner
            width: toast.width - root.corner * 2
            height: root.corner - root.hairline
            source: "fluent-noise.png"
            fillMode: Image.Tile
            smooth: false
        }

        // the inner layer of the outline
        Rectangle {
            anchors.fill: parent
            anchors.margins: root.hairline
            radius: Math.max(0, root.corner - root.hairline)
            color: "transparent"
            border.width: root.hairline
            border.color: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.05 : 0.5)
        }

        // the reveal light: a bright head with a soft tail, once round the outline
        Shape {
            id: reveal
            anchors.fill: parent
            visible: root.light > 0 && root.light < 1
            opacity: Math.sin(Math.PI * root.light)
            preferredRendererType: Shape.GeometryRenderer

            readonly property real o: root.hairline / 2
            readonly property real w: width - root.hairline
            readonly property real h: height - root.hairline
            readonly property real r: Math.max(0, Math.min(root.corner, w / 2, h / 2))
            readonly property real perim: 2 * (w - 2 * r) + 2 * (h - 2 * r) + 2 * Math.PI * r
            readonly property real head: root.light * reveal.perim * 1.15

            Trail { span: 160 * root.u; sw: 4 * root.u; strokeColor: Lithic.alpha(Lithic.sel, 0.22) }
            Trail { span: 90 * root.u; sw: 2 * root.u; strokeColor: Lithic.alpha(Lithic.sel, 0.6) }
            Trail { span: 40 * root.u; sw: 1.5 * root.hairline; strokeColor: Lithic.mix(Lithic.sel, Lithic.fg, 0.45) }
        }

        Item {
            id: header
            x: root.pad
            y: Math.round(9 * root.u)
            width: toast.width - root.pad * 2 + 6 * root.u
            height: Math.round(28 * root.u)

            Item {
                id: appIcon
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                width: Math.round(16 * root.u)
                height: width

                Image {
                    id: iconImage
                    anchors.fill: parent
                    visible: root.showImages && status === Image.Ready && !root.critical
                    source: root.showImages ? root.appIconSource : ""
                    sourceSize: Qt.size(width * 2, height * 2)
                    fillMode: Image.PreserveAspectFit
                    asynchronous: true
                    smooth: true
                }
                Text {
                    anchors.centerIn: parent
                    visible: !iconImage.visible
                    text: root.critical ? "error" : "notifications"
                    color: root.critical ? Lithic.jasper : Lithic.sel
                    font.family: "Material Symbols Outlined"
                    font.variableAxes: ({ "FILL": 1, "wght": 400 })
                    font.pixelSize: Math.round(16 * root.u)
                    renderType: Text.CurveRendering
                }
            }
            Text {
                id: appLabel
                anchors.left: appIcon.right
                anchors.leftMargin: 8 * root.u
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, controls.x - x - timeLabel.implicitWidth - 16 * root.u)
                text: root.appName
                color: Lithic.fg2
                elide: Text.ElideRight
                font.family: Lithic.font.sans
                font.pixelSize: 12 * root.u
                renderType: Text.CurveRendering
            }
            Text {
                id: timeLabel
                anchors.left: appLabel.right
                anchors.leftMargin: 8 * root.u
                anchors.verticalCenter: parent.verticalCenter
                text: root.timeText
                color: Lithic.fg3
                font.family: Lithic.font.sans
                font.pixelSize: 12 * root.u
                renderType: Text.CurveRendering
            }
            Row {
                id: controls
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 2 * root.u

                SubtleButton {
                    visible: root.defaultAction !== null
                    glyph: "open_in_new"
                    onClicked: root.openDefault()
                }
                SubtleButton {
                    visible: root.expandable
                    glyph: "expand_more"
                    turn: root.expanded ? 180 : 0
                    onClicked: root.toggleExpanded()
                }
                SubtleButton {
                    glyph: "close"
                    onClicked: root.closeRequested()
                }
            }
        }

        Column {
            id: content
            x: root.pad
            y: header.y + header.height + 6 * root.u
            width: toast.width - root.pad * 2
            spacing: 2 * root.u

            Item {
                width: parent.width
                height: root.hero ? heroImage.height + 10 * root.u : 0
                visible: root.hero

                Image {
                    id: heroImage
                    width: parent.width
                    height: Math.round(parent.width * 0.42)
                    source: root.showImages ? root.imageSource : ""
                    sourceSize: Qt.size(width * 2, height * 2)
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    smooth: true
                    layer.enabled: root.hero
                    layer.effect: MultiEffect {
                        maskEnabled: true
                        maskSource: heroMask
                    }
                }
                Rectangle {
                    id: heroMask
                    width: heroImage.width
                    height: heroImage.height
                    radius: root.control
                    visible: false
                    layer.enabled: root.hero
                }
            }

            Text {
                width: parent.width
                visible: text !== ""
                text: root.summary
                color: Lithic.fg
                font.family: Lithic.font.sans
                font.weight: Font.DemiBold
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
                font.pixelSize: 13 * root.u
                wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                maximumLineCount: root.showFull ? 9999 : root.compactLines
                elide: root.showFull ? Text.ElideNone : Text.ElideRight
                renderType: Text.CurveRendering
            }

            Item {
                width: parent.width
                height: actionRow.visible ? actionRow.height + 12 * root.u : 0

                Row {
                    id: actionRow
                    y: 12 * root.u
                    width: parent.width
                    spacing: 8 * root.u
                    visible: root.visibleActions.length > 0 && root.showFull
                    opacity: root.showFull ? 1 : 0
                    Behavior on opacity { NumberAnimation { duration: Lithic.duration(167) } }

                    Repeater {
                        model: root.visibleActions

                        delegate: Item {
                            id: standard
                            required property var modelData
                            property real hover: standardHover.hovered ? 1 : 0
                            Behavior on hover { NumberAnimation { duration: Lithic.duration(83) } }

                            width: (actionRow.width - actionRow.spacing * (root.visibleActions.length - 1)) / Math.max(1, root.visibleActions.length)
                            height: Math.round(32 * root.u)

                            Rectangle {
                                anchors.fill: parent
                                radius: root.control
                                color: Lithic.mix(Lithic.alpha(Lithic.fg, Lithic.dark ? 0.06 : 0.03),
                                    Lithic.alpha(Lithic.fg, Lithic.dark ? 0.09 : 0.06), standard.hover)
                                border.width: root.hairline
                                border.color: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.07 : 0.1)
                            }
                            Text {
                                anchors.centerIn: parent
                                width: Math.min(implicitWidth, standard.width - 16 * root.u)
                                text: standard.modelData.text
                                color: standardTap.pressed ? Lithic.fg2 : Lithic.fg
                                elide: Text.ElideRight
                                font.family: Lithic.font.sans
                                font.pixelSize: 13 * root.u
                                renderType: Text.CurveRendering
                            }
                            HoverHandler {
                                id: standardHover
                                cursorShape: Qt.PointingHandCursor
                            }
                            TapHandler {
                                id: standardTap
                                onTapped: root.invoke(standard.modelData)
                            }
                        }
                    }
                }
            }
        }
    }
}

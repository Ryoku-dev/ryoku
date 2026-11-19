pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Lithic

// Material's notification, after Android 17's expressive heads-up card: a
// rounded card on the high surface container with a level-two shadow and no
// line. The app sits in a tonal circle wrapped by a wavy ring that drains over
// the card's lifespan; the label, title and body follow the M3 type scale; the
// chevron rides a tonal circle and the actions are tonal pills. A critical card
// turns its ring and circle to the error colour. It springs in from its edge,
// growing from 0.88 with a little overshoot while its corners tighten, and
// shrinks away as it fades.
LookCard {
    id: root

    readonly property real u: root.us
    readonly property real pad: 16 * root.u
    readonly property real travel: 32 * root.u
    readonly property real rest: Lithic.cornerRadius(LithicTokens.cut.large, width / root.u, height / root.u, "material") * root.u
    readonly property real settled: Math.max(0, Math.min(1, root.arrive))
    readonly property color accent: root.critical ? Lithic.jasper : Lithic.sel
    readonly property color circleFill: root.critical ? Lithic.mix(Lithic.stone4, Lithic.jasper, 0.22) : Lithic.container
    readonly property color circleInk: root.critical ? Lithic.jasper : Lithic.containerInk

    contentHeight: Math.ceil(content.y + content.height + root.pad)
    bodyLines: bodyMeasure.lineCount
    drains: true
    lifeSteps: 180
    expandMs: 380
    expandCurve: [0.34, 1.12, 0.64, 1, 1, 1]

    property real fade: 0
    // M3 Expressive's fast spatial spring in its own published curve form
    // (cubic-bezier 0.42, 1.67, 0.21, 0.90 over 320 ms): it overshoots by about a
    // tenth and lands on time, where a free SpringAnimation rang on for sub-pixel
    // steps that redrew the whole card. The fade is a plain short ease.
    enterMotion: ParallelAnimation {
        NumberAnimation {
            target: root
            property: "arrive"
            from: 0
            to: 1
            duration: 320
            easing.type: Easing.BezierSpline
            easing.bezierCurve: [0.42, 1.67, 0.21, 0.9, 1, 1]
        }
        NumberAnimation {
            target: root
            property: "fade"
            from: 0
            to: 1
            duration: 120
            easing.type: Easing.OutQuad
        }
    }
    exitMotion: NumberAnimation {
        target: root
        property: "gone"
        to: 1
        duration: 150
        easing.type: Easing.BezierSpline
        easing.bezierCurve: [0.3, 0, 0.8, 0.15, 1, 1]
    }
    onArriveChanged: if (root.arrive >= 1 && root.fade < 1 && !root.enterMotion.running) root.fade = 1

    Text {
        id: bodyMeasure
        visible: false
        width: bodyText.width
        text: root.body
        font: bodyText.font
        lineHeight: bodyText.lineHeight
        wrapMode: Text.WrapAtWordBoundaryOrAnywhere
    }

    // a round header control with M3's state layer; `tonal` sets it on the
    // secondary container
    component RoundButton: Item {
        id: button
        property string glyph: ""
        property bool tonal: false
        property real turn: 0
        signal clicked()

        width: Math.round(32 * root.u)
        height: width

        property real hover: hoverHandler.hovered ? 1 : 0
        Behavior on hover { NumberAnimation { duration: Lithic.duration(150) } }
        property real press: tap.pressed ? 1 : 0
        Behavior on press { NumberAnimation { duration: Lithic.duration(90) } }

        Rectangle {
            anchors.fill: parent
            radius: width / 2
            scale: 1 - 0.08 * button.press
            color: button.tonal
                ? Lithic.mix(Lithic.container, Lithic.containerInk, 0.08 * button.hover)
                : Lithic.alpha(Lithic.fg, 0.08 * button.hover)
        }
        Text {
            anchors.centerIn: parent
            text: button.glyph
            color: button.tonal ? Lithic.containerInk : Lithic.fg2
            font.family: "Material Symbols Rounded"
            font.variableAxes: ({ "wght": 500 })
            font.pixelSize: Math.round(20 * root.u)
            rotation: button.turn
            renderType: Text.CurveRendering
            Behavior on rotation {
                NumberAnimation {
                    duration: Lithic.duration(420)
                    easing.type: Easing.BezierSpline
                    easing.bezierCurve: [0.34, 1.36, 0.64, 1, 1, 1]
                }
            }
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
        id: card

        width: root.width
        height: root.height
        opacity: Math.min(root.fade, 1) * (1 - root.gone)
        scale: (0.88 + 0.12 * root.arrive) * (1 - 0.1 * root.gone)
        transformOrigin: root.anchorOrigin
        transform: Translate {
            x: root.edgeX * root.travel * (1 - root.arrive)
            y: root.edgeY * root.travel * (1 - root.arrive)
        }

        readonly property real radius: Math.max(0, root.rest * (1 + (1 - root.arrive) / 3))

        // M3 elevation level 2: a tight key shadow under a wider ambient one
        RectangularShadow {
            visible: root.shadows
            anchors.fill: parent
            offset.y: 2 * root.u
            blur: 10 * root.u
            spread: 2 * root.u
            radius: card.radius
            color: Lithic.alpha(Lithic.scrim, Lithic.dark ? 0.3 : 0.15)
        }
        RectangularShadow {
            visible: root.shadows
            anchors.fill: parent
            offset.y: 1 * root.u
            blur: 3 * root.u
            radius: card.radius
            color: Lithic.alpha(Lithic.scrim, Lithic.dark ? 0.4 : 0.22)
        }

        Rectangle {
            anchors.fill: parent
            radius: card.radius
            color: Lithic.stone4
        }

        Item {
            id: badge
            x: root.pad - 4 * root.u
            y: root.pad - 4 * root.u
            width: Math.round(44 * root.u)
            height: width

            WavyRing {
                anchors.fill: parent
                visible: root.counting || root.critical
                progress: root.counting ? root.remaining : 1
                thickness: 3 * root.u
                amplitude: 1.5 * root.u
                waves: 10
                ink: root.accent
                trackInk: Lithic.alpha(root.accent, 0.2)
            }

            Rectangle {
                id: circle
                anchors.centerIn: parent
                width: Math.round(34 * root.u)
                height: width
                radius: width / 2
                color: root.circleFill
                scale: 0.6 + 0.4 * root.settled
            }

            Item {
                id: picture
                readonly property bool ready: image.status === Image.Ready
                anchors.fill: circle
                visible: root.showImages && picture.ready
                layer.enabled: picture.visible
                layer.effect: MultiEffect {
                    maskEnabled: true
                    maskSource: pictureMask
                }
                Image {
                    id: image
                    anchors.fill: parent
                    source: root.showImages ? root.imageSource : ""
                    sourceSize: Qt.size(width * 2, height * 2)
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    smooth: true
                }
            }
            Rectangle {
                id: pictureMask
                anchors.fill: circle
                radius: width / 2
                visible: false
                layer.enabled: picture.visible
            }
            Image {
                id: icon
                anchors.centerIn: circle
                width: Math.round(20 * root.u)
                height: width
                visible: root.showImages && !picture.ready && status === Image.Ready
                source: root.showImages ? root.appIconSource : ""
                sourceSize: Qt.size(width * 2, height * 2)
                fillMode: Image.PreserveAspectFit
                asynchronous: true
                smooth: true
            }
            Text {
                anchors.centerIn: circle
                visible: !picture.visible && !icon.visible
                text: root.critical ? "priority_high" : "notifications"
                color: root.circleInk
                font.family: "Material Symbols Rounded"
                font.variableAxes: ({ "FILL": 1, "wght": 500 })
                font.pixelSize: Math.round(19 * root.u)
                renderType: Text.CurveRendering
            }
        }

        Item {
            id: header
            x: badge.x + badge.width + 10 * root.u
            y: root.pad - 4 * root.u
            width: card.width - x - root.pad + 6 * root.u
            height: Math.round(32 * root.u)

            Text {
                anchors.left: parent.left
                anchors.right: controls.left
                anchors.rightMargin: 6 * root.u
                anchors.verticalCenter: parent.verticalCenter
                text: root.appName + (root.timeText !== "" ? "  •  " + root.timeText : "")
                color: root.critical ? Lithic.jasper : Lithic.fg2
                elide: Text.ElideRight
                font.family: Lithic.font.sans
                font.weight: Font.Medium
                font.pixelSize: 11.5 * root.u
                font.letterSpacing: 0.3 * root.u
                renderType: Text.CurveRendering
            }
            Row {
                id: controls
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 2 * root.u

                RoundButton {
                    visible: root.defaultAction !== null
                    glyph: "open_in_new"
                    onClicked: root.openDefault()
                }
                RoundButton {
                    visible: root.expandable
                    tonal: true
                    glyph: "expand_more"
                    turn: root.expanded ? 180 : 0
                    onClicked: root.toggleExpanded()
                }
                RoundButton {
                    glyph: "close"
                    onClicked: root.closeRequested()
                }
            }
        }

        Column {
            id: content
            x: header.x
            y: header.y + header.height
            width: card.width - x - root.pad
            spacing: 2 * root.u

            Text {
                width: parent.width
                visible: text !== ""
                text: root.summary
                color: Lithic.fg
                font.family: Lithic.font.sans
                font.weight: Font.Medium
                font.pixelSize: 14 * root.u
                font.letterSpacing: 0.1 * root.u
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
                font.pixelSize: 13.5 * root.u
                lineHeight: 1.18
                wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                maximumLineCount: root.showFull ? 9999 : root.compactLines
                elide: root.showFull ? Text.ElideNone : Text.ElideRight
                renderType: Text.CurveRendering
            }

            Item {
                width: parent.width
                height: actionRow.visible ? actionRow.height + 12 * root.u : 0

                Flow {
                    id: actionRow
                    y: 12 * root.u
                    width: parent.width
                    spacing: 8 * root.u
                    visible: root.visibleActions.length > 0 && root.showFull
                    opacity: root.showFull ? 1 : 0
                    Behavior on opacity { NumberAnimation { duration: Lithic.duration(180) } }

                    Repeater {
                        model: root.visibleActions

                        delegate: Item {
                            id: pill
                            required property var modelData
                            property real hover: pillHover.hovered ? 1 : 0
                            Behavior on hover { NumberAnimation { duration: Lithic.duration(150) } }
                            property real press: pillTap.pressed ? 1 : 0
                            Behavior on press { NumberAnimation { duration: Lithic.duration(90) } }

                            width: Math.ceil(Math.min(actionRow.width, pillText.implicitWidth + 32 * root.u))
                            height: Math.round(34 * root.u)

                            Rectangle {
                                anchors.fill: parent
                                radius: Lithic.cornerRadius(LithicTokens.cut.large, width / root.u, height / root.u, "material") * root.u
                                    * (1 - 0.35 * pill.press)
                                color: Lithic.mix(Lithic.container, Lithic.containerInk, 0.08 * pill.hover)
                            }
                            Text {
                                id: pillText
                                anchors.centerIn: parent
                                width: Math.min(implicitWidth, pill.width - 24 * root.u)
                                text: pill.modelData.text
                                color: Lithic.containerInk
                                elide: Text.ElideRight
                                font.family: Lithic.font.sans
                                font.weight: Font.Medium
                                font.pixelSize: 13.5 * root.u
                                renderType: Text.CurveRendering
                            }
                            HoverHandler {
                                id: pillHover
                                cursorShape: Qt.PointingHandCursor
                            }
                            TapHandler {
                                id: pillTap
                                onTapped: root.invoke(pill.modelData)
                            }
                        }
                    }
                }
            }
        }
    }
}

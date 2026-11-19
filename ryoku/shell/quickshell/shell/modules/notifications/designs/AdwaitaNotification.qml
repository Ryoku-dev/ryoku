pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Lithic

// Adwaita's notification, after GNOME's banner: a rounded card on the view
// background with a one-pixel shade border and libadwaita's soft shadow. A small
// header carries the app's icon, its name in caption-heading and the time in
// caption, both dimmed, and flat circular buttons; the summary is a heading
// over two lines of body; actions are a row of equal flat buttons under a
// separator. Like GNOME it shows no countdown: its polish is the arrival, a
// short slide and fade while the app icon pops into place. A critical card's
// border and icon take the error colour.
LookCard {
    id: root

    readonly property real u: root.us
    readonly property real pad: 14 * root.u
    readonly property real travel: 16 * root.u
    readonly property real corner: Lithic.cornerRadius(LithicTokens.cut.large, width / root.u, height / root.u, "adwaita") * root.u
    readonly property real hairline: Math.max(1, Math.round(root.u))
    readonly property color shade: Lithic.dark ? Lithic.alpha(Lithic.scrim, 0.55) : Lithic.alpha(Lithic.scrim, 0.12)
    readonly property bool hasActions: root.visibleActions.length > 0 && root.showFull

    contentHeight: Math.ceil(texts.y + texts.height + root.pad + (actionBar.visible ? actionBar.height : 0))
    bodyLines: bodyMeasure.lineCount
    expandMs: 250
    expandCurve: [0.25, 0.46, 0.45, 0.94, 1, 1]

    property real pop: 0
    enterMotion: ParallelAnimation {
        NumberAnimation {
            target: root
            property: "arrive"
            from: 0
            to: 1
            duration: 200
            easing.type: Easing.OutQuad
        }
        SequentialAnimation {
            PropertyAction { target: root; property: "pop"; value: 0 }
            PauseAnimation { duration: 70 }
            NumberAnimation {
                target: root
                property: "pop"
                to: 1
                duration: 300
                easing.type: Easing.OutBack
                easing.overshoot: 2.2
            }
        }
    }
    exitMotion: NumberAnimation {
        target: root
        property: "gone"
        to: 1
        duration: 150
        easing.type: Easing.InQuad
    }
    onArriveChanged: if (root.arrive >= 1 && !root.enterMotion.running) root.pop = 1

    Text {
        id: bodyMeasure
        visible: false
        width: bodyText.width
        text: root.body
        font: bodyText.font
        wrapMode: Text.WrapAtWordBoundaryOrAnywhere
    }

    // libadwaita's flat circular button: no fill until hovered
    component CircleButton: Item {
        id: button
        property string glyph: ""
        property real turn: 0
        signal clicked()

        width: Math.round(24 * root.u)
        height: width

        property real hover: hoverHandler.hovered ? 1 : 0
        Behavior on hover { NumberAnimation { duration: Lithic.duration(150) } }

        Rectangle {
            anchors.fill: parent
            radius: width / 2
            color: Lithic.alpha(Lithic.fg, (tap.pressed ? 0.16 : 0.08) * button.hover)
        }
        Text {
            anchors.centerIn: parent
            text: button.glyph
            color: Lithic.fg
            opacity: 0.6 + 0.4 * button.hover
            font.family: "Material Symbols Rounded"
            font.variableAxes: ({ "wght": 600 })
            font.pixelSize: Math.round(16 * root.u)
            rotation: button.turn
            renderType: Text.CurveRendering
            Behavior on rotation { NumberAnimation { duration: Lithic.duration(200); easing.type: Easing.OutQuad } }
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
        id: banner

        width: root.width
        height: root.height
        opacity: Math.max(0, Math.min(1, root.arrive)) * (1 - root.gone)
        transform: Translate {
            x: root.edgeX * root.travel * (1 - root.arrive + root.gone)
            y: root.edgeY * root.travel * (1 - root.arrive + root.gone)
        }

        // libadwaita's card shadow: a close contact shade under a wide soft one
        RectangularShadow {
            visible: root.shadows
            anchors.fill: parent
            offset.y: 4 * root.u
            blur: 22 * root.u
            spread: 2 * root.u
            radius: root.corner
            color: Lithic.alpha(Lithic.scrim, Lithic.dark ? 0.5 : 0.1)
        }
        RectangularShadow {
            visible: root.shadows
            anchors.fill: parent
            offset.y: 1 * root.u
            blur: 3 * root.u
            spread: 1 * root.u
            radius: root.corner
            color: Lithic.alpha(Lithic.scrim, Lithic.dark ? 0.3 : 0.07)
        }

        Rectangle {
            anchors.fill: parent
            radius: root.corner
            // the view background in the light; in the dark libadwaita's banner is
            // a warm grey card that stands off the backdrop, the high container here
            color: Lithic.dark ? Lithic.mix(Lithic.stone4, Lithic.fg, 0.08) : Lithic.ground
            border.width: root.hairline
            border.color: root.critical ? Lithic.alpha(Lithic.jasper, 0.65) : root.shade
        }

        Item {
            id: header
            x: root.pad
            y: Math.round(10 * root.u)
            width: banner.width - root.pad * 2 + 6 * root.u
            height: Math.round(24 * root.u)

            Item {
                id: appIcon
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                width: Math.round(16 * root.u)
                height: width
                scale: 0.8 + 0.2 * root.pop

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
                    text: root.critical ? "warning" : "notifications"
                    color: root.critical ? Lithic.jasper : Lithic.fg2
                    font.family: "Material Symbols Rounded"
                    font.variableAxes: ({ "FILL": 1, "wght": 500 })
                    font.pixelSize: Math.round(16 * root.u)
                    renderType: Text.CurveRendering
                }
            }
            Text {
                id: appLabel
                anchors.left: appIcon.right
                anchors.leftMargin: 8 * root.u
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, controls.x - x - timeLabel.implicitWidth - 14 * root.u)
                text: root.appName
                color: root.critical ? Lithic.jasper : Lithic.fg
                opacity: root.critical ? 1 : 0.72
                elide: Text.ElideRight
                font.family: Lithic.font.sans
                font.weight: Font.Bold
                font.pixelSize: 11.5 * root.u
                renderType: Text.CurveRendering
            }
            Text {
                id: timeLabel
                anchors.left: appLabel.right
                anchors.leftMargin: 6 * root.u
                anchors.verticalCenter: parent.verticalCenter
                text: root.timeText
                color: Lithic.fg
                opacity: 0.62
                font.family: Lithic.font.sans
                font.pixelSize: 11.5 * root.u
                renderType: Text.CurveRendering
            }
            Row {
                id: controls
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 4 * root.u

                CircleButton {
                    visible: root.defaultAction !== null
                    glyph: "open_in_new"
                    onClicked: root.openDefault()
                }
                CircleButton {
                    visible: root.expandable
                    glyph: "expand_more"
                    turn: root.expanded ? 180 : 0
                    onClicked: root.toggleExpanded()
                }
                CircleButton {
                    glyph: "close"
                    onClicked: root.closeRequested()
                }
            }
        }

        Row {
            id: texts
            x: root.pad
            y: header.y + header.height + 4 * root.u
            width: banner.width - root.pad * 2
            spacing: 12 * root.u

            Image {
                id: picture
                readonly property bool shown: root.showImages && status === Image.Ready
                width: picture.shown ? Math.round(40 * root.u) : 0
                height: width
                visible: picture.shown
                source: root.showImages ? root.imageSource : ""
                sourceSize: Qt.size(80 * root.u, 80 * root.u)
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
                smooth: true
                layer.enabled: picture.shown
                layer.effect: MultiEffect {
                    maskEnabled: true
                    maskSource: pictureMask
                }
            }
            Rectangle {
                id: pictureMask
                visible: false
                width: picture.width
                height: picture.height
                radius: Lithic.cornerRadius(LithicTokens.cut.small, width / root.u, height / root.u, "adwaita") * root.u
                layer.enabled: picture.shown
            }

            Column {
                width: parent.width - (picture.shown ? picture.width + parent.spacing : 0)
                spacing: 2 * root.u

                Text {
                    width: parent.width
                    visible: text !== ""
                    text: root.summary
                    color: Lithic.fg
                    font.family: Lithic.font.sans
                    font.weight: Font.Bold
                    font.pixelSize: 14 * root.u
                    wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                    renderType: Text.CurveRendering
                }
                Text {
                    id: bodyText
                    width: parent.width
                    visible: text !== ""
                    text: root.body
                    color: Lithic.fg
                    opacity: 0.85
                    font.family: Lithic.font.sans
                    font.pixelSize: 13 * root.u
                    wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                    maximumLineCount: root.showFull ? 9999 : root.compactLines
                    elide: root.showFull ? Text.ElideNone : Text.ElideRight
                    renderType: Text.CurveRendering
                }
            }
        }

        // equal flat buttons under a separator, split by the same hairline
        Item {
            id: actionBar
            visible: root.hasActions
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: Math.round(40 * root.u)
            opacity: root.showFull ? 1 : 0
            Behavior on opacity { NumberAnimation { duration: Lithic.duration(150) } }

            Rectangle {
                width: parent.width
                height: root.hairline
                color: root.shade
            }
            Row {
                id: buttons
                y: root.hairline
                width: parent.width
                height: parent.height - y

                Repeater {
                    model: root.visibleActions

                    delegate: Item {
                        id: flat
                        required property var modelData
                        required property int index
                        property real hover: flatHover.hovered ? 1 : 0
                        Behavior on hover { NumberAnimation { duration: Lithic.duration(150) } }

                        width: buttons.width / Math.max(1, root.visibleActions.length)
                        height: buttons.height

                        Rectangle {
                            anchors.fill: parent
                            anchors.margins: 4 * root.u
                            radius: Lithic.cornerRadius(LithicTokens.cut.mid, width / root.u, height / root.u, "adwaita") * root.u
                            color: Lithic.alpha(Lithic.fg, (flatTap.pressed ? 0.14 : 0.07) * flat.hover)
                        }
                        Rectangle {
                            visible: flat.index > 0
                            width: root.hairline
                            height: parent.height
                            color: root.shade
                        }
                        Text {
                            anchors.centerIn: parent
                            width: Math.min(implicitWidth, flat.width - 16 * root.u)
                            text: flat.modelData.text
                            color: Lithic.fg
                            elide: Text.ElideRight
                            font.family: Lithic.font.sans
                            font.weight: Font.Bold
                            font.pixelSize: 13 * root.u
                            renderType: Text.CurveRendering
                        }
                        HoverHandler {
                            id: flatHover
                            cursorShape: Qt.PointingHandCursor
                        }
                        TapHandler {
                            id: flatTap
                            onTapped: root.invoke(flat.modelData)
                        }
                    }
                }
            }
        }
    }
}

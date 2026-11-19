import QtQuick
import Lithic
import "../../look"
import "../.."

// Rows kept out of sight until asked for, behind the design's expander: Lithic's
// "+ SHADOW DETAILS" toggle line, Material's list item with a turning chevron,
// Adwaita's AdwExpanderRow inside the boxed list, Fluent's SettingsExpander
// whose rows hang under its header card. The rows open by height; under reduced
// motion they snap.
Item {
    id: ex

    property real u: 1
    property string title: ""
    property string summary: ""
    property string glyph: ""
    property bool expanded: false
    default property alias content: rows.content

    readonly property real headH: Math.round((Look.lithic ? 28 : Look.material ? 56 : Look.adwaita ? 58 : 60) * ex.u)
    readonly property real openH: rows.height + Math.round((Look.lithic ? 12 : Look.material ? 8 : Look.fluent ? 4 : 0) * ex.u)
        + (Look.adwaita ? Math.max(1, Math.round(ex.u)) : 0)
    property real reveal: ex.expanded ? 1 : 0
    Behavior on reveal {
        NumberAnimation {
            duration: Look.dur(Look.material ? 320 : Look.fluent ? 250 : 200)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: Look.material ? Look.springDefault : Look.fluent ? Look.fluentIn : Look.decelerate
        }
    }

    implicitHeight: ex.headH + Math.max(0, ex.openH * ex.reveal)
    width: parent ? parent.width : implicitWidth
    height: implicitHeight

    // Fluent's expander is one card, its header and its rows parted by a hairline
    Rectangle {
        visible: Look.fluent
        anchors.fill: parent
        radius: 4 * ex.u
        color: Look.planes.card
        border.width: Math.max(1, Math.round(ex.u))
        border.color: Look.planes.cardStroke
        antialiasing: true
    }
    Rectangle {
        visible: Look.fluent
        x: Math.max(1, Math.round(ex.u))
        y: ex.headH
        width: parent.width - 2 * x
        height: Math.max(0, parent.height - ex.headH - x)
        bottomLeftRadius: 3 * ex.u
        bottomRightRadius: 3 * ex.u
        color: Lithic.mix(Look.planes.card, Look.planes.side, 0.35)
        Rectangle {
            width: parent.width
            height: Math.max(1, Math.round(ex.u))
            color: Look.planes.cardStroke
        }
    }

    Item {
        id: head
        width: ex.width
        height: ex.headH

        readonly property bool hovered: headHover.hovered
        readonly property bool pressed: headTap.pressed

        // the state layer
        CutRect {
            visible: Look.lithic
            anchors.fill: parent
            anchors.leftMargin: -Math.round(6 * ex.u)
            anchors.rightMargin: -Math.round(6 * ex.u)
            design: "lithic"
            cut: Look.cutSm * ex.u
            fill: head.pressed ? Look.press : head.hovered ? Look.hover : Lithic.alpha(Look.hover, 0)
        }
        Rectangle {
            visible: !Look.lithic
            anchors.fill: parent
            anchors.leftMargin: Look.material ? -Math.round(12 * ex.u) : 0
            anchors.rightMargin: Look.material ? -Math.round(12 * ex.u) : 0
            readonly property bool first: !!ex.parent && !!ex.parent.visibleChildren && ex.parent.visibleChildren[0] === ex
            readonly property bool last: !!ex.parent && !!ex.parent.visibleChildren
                && ex.parent.visibleChildren[ex.parent.visibleChildren.length - 1] === ex
            topLeftRadius: Look.material ? 16 * ex.u : Look.adwaita ? (first ? 12 * ex.u : 0) : 4 * ex.u
            topRightRadius: topLeftRadius
            bottomLeftRadius: Look.material ? 16 * ex.u : Look.adwaita ? (last && ex.reveal === 0 ? 12 * ex.u : 0)
                : ex.reveal > 0 ? 0 : 4 * ex.u
            bottomRightRadius: bottomLeftRadius
            color: head.pressed ? Look.press : head.hovered ? Lithic.alpha(Look.fg, Look.fluent ? 0.04 : 0.08) : "transparent"
            Behavior on color {
                ColorAnimation { duration: Look.dur(Look.stateMs) }
            }
        }

        // Lithic: "+ SHADOW DETAILS", the plus turning to a cross when open
        Row {
            id: lithicTitle
            visible: Look.lithic
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            spacing: Math.round(8 * ex.u)
            Item {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.round(11 * ex.u)
                height: width
                rotation: ex.reveal * 45
                Rectangle { anchors.centerIn: parent; width: parent.width; height: Math.max(1, Math.round(1.5 * ex.u)); color: Look.sel }
                Rectangle { anchors.centerIn: parent; height: parent.width; width: Math.max(1, Math.round(1.5 * ex.u)); color: Look.sel }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: ex.title.toUpperCase()
                color: head.hovered || ex.expanded ? Look.fg : Look.fg2
                font.family: Look.mono
                font.pixelSize: Math.round(9.5 * ex.u)
                font.weight: Font.Bold
                font.letterSpacing: Look.tracking * ex.u
                renderType: Text.CurveRendering
                Behavior on color {
                    ColorAnimation { duration: Look.dur(Look.stateMs) }
                }
            }
        }
        Text {
            visible: Look.lithic && !ex.expanded && lithicTitle.width + implicitWidth + 24 * ex.u < head.width
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            text: ex.summary
            color: Look.fg3
            font.family: Look.mono
            font.pixelSize: Math.round(9.5 * ex.u)
            font.features: ({ "tnum": 1 })
            renderType: Text.CurveRendering
        }

        // the round designs: glyph, title over its summary, the chevron
        Icon {
            id: lead
            visible: !Look.lithic && !Look.adwaita && ex.glyph.length > 0
            x: Look.fluent ? Math.round(14 * ex.u) : 0
            anchors.verticalCenter: parent.verticalCenter
            width: Math.round((Look.material ? 20 : 16) * ex.u)
            height: width
            size: width
            name: ex.glyph
            tint: Look.material ? Look.fg2 : Look.fg
        }
        Column {
            visible: !Look.lithic
            x: lead.visible ? lead.x + lead.width + Math.round((Look.material ? 16 : 14) * ex.u) : Math.round((Look.adwaita ? 12 : 0) * ex.u)
            anchors.verticalCenter: parent.verticalCenter
            width: chevron.x - x - Math.round(12 * ex.u)
            spacing: Math.round(1 * ex.u)
            Text {
                width: parent.width
                elide: Text.ElideRight
                text: ex.title
                color: Look.fg
                font.family: Look.sans
                font.pixelSize: Math.round((Look.material ? 15 : 14) * ex.u)
                renderType: Text.CurveRendering
            }
            Text {
                visible: ex.summary.length > 0
                width: parent.width
                elide: Text.ElideRight
                text: ex.summary
                color: Look.fg2
                font.family: Look.sans
                font.pixelSize: Math.round((Look.material ? 13 : 12) * ex.u)
                font.features: ({ "tnum": 1 })
                renderType: Text.CurveRendering
            }
        }
        // Fluent's chevron sits on its own subtle button
        Rectangle {
            visible: Look.fluent
            anchors.centerIn: chevron
            width: Math.round(32 * ex.u)
            height: width
            radius: 4 * ex.u
            color: head.hovered ? Lithic.alpha(Look.fg, Look.dark ? 0.06 : 0.04) : "transparent"
        }
        Icon {
            id: chevron
            visible: !Look.lithic
            anchors.right: parent.right
            anchors.rightMargin: Math.round((Look.adwaita ? 12 : Look.fluent ? 14 : 0) * ex.u)
            anchors.verticalCenter: parent.verticalCenter
            width: Math.round((Look.fluent ? 12 : 18) * ex.u)
            height: width
            size: width
            name: "chevron"
            tint: Look.material ? Look.fg2 : Look.fg
            rotation: ex.reveal * 180
        }

        HoverHandler {
            id: headHover
            cursorShape: Qt.PointingHandCursor
        }
        TapHandler {
            id: headTap
            onTapped: ex.expanded = !ex.expanded
        }
    }

    // Adwaita's separator under the expander's header row
    Rectangle {
        visible: Look.adwaita && ex.reveal > 0
        y: ex.headH
        width: parent.width
        height: Math.max(1, Math.round(ex.u))
        color: Look.divider
    }

    Item {
        id: drawer
        y: ex.headH + (Look.adwaita ? Math.max(1, Math.round(ex.u)) : 0)
        width: ex.width
        height: Math.max(0, ex.height - y)
        clip: true
        visible: ex.reveal > 0
        opacity: Look.material || Look.fluent ? Math.min(1, ex.reveal * 1.6) : 1

        List {
            id: rows
            x: Look.fluent ? Math.round(44 * ex.u) : 0
            y: Look.lithic ? Math.round(8 * ex.u) : Look.material ? Math.round(4 * ex.u) : 0
            width: parent.width - x - (Look.fluent ? Math.round(14 * ex.u) : 0)
            u: ex.u
            boxed: true
            ruled: true
        }
    }
}

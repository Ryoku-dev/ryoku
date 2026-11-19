import QtQuick
import Lithic
import "../../look"
import "../.."

// One row of a group, in the design's own row: Lithic's mono-caps label with its
// control at the right, Material's list item, Adwaita's boxed-list row, Fluent's
// SettingsCard (a 16 px glyph, the title and a caption, the control at the
// right). The control goes in the trailing slot; block holds what spans the row
// under the title line (a slider, a grid of swatches), or the whole row when the
// row has no title. activatable rows answer hover and press and emit clicked.
Item {
    id: row

    property real u: 1
    property string title: ""
    property string subtitle: ""
    property string glyph: ""
    property bool activatable: false
    // the trailing control takes the line's free width (Adwaita's slider rows)
    property bool fillTrailing: false
    property real titleWidth: 0
    // no card or padding of its own: a row nested in an expander's content
    property bool bare: false
    default property alias trailing: trailingBox.data
    property alias blockSpacing: blockBox.spacing
    property alias block: blockBox.data

    signal clicked()

    readonly property bool hasLine: row.title.length > 0
    readonly property bool card: Look.fluent && !row.bare
    readonly property real padX: Math.round((Look.adwaita ? 12 : row.card ? 14 : 0) * row.u)
    readonly property real padY: Math.round((Look.adwaita ? (row.hasLine ? 0 : 12) : row.card ? 10 : Look.fluent ? 6 : 0) * row.u)
    // a taller control (a dial) can stretch the title line
    property real minLine: 0
    readonly property real lineH: !row.hasLine ? 0
        : Math.max(row.minLine, words.height + Math.round((Look.lithic ? 0 : 12) * row.u),
            Math.round((Look.lithic ? 28 : Look.material ? (row.subtitle ? 56 : 44) : Look.adwaita ? (row.subtitle ? 58 : 50) : (row.subtitle ? 48 : 40)) * row.u))
    readonly property real glyphSize: Math.round((Look.material ? 20 : 16) * row.u)
    readonly property bool showGlyph: row.glyph.length > 0 && (Look.fluent || Look.material)
    readonly property real blockGap: row.hasLine && blockBox.height > 0
        ? Math.round((Look.adwaita ? 0 : Look.fluent ? 2 : 4) * row.u) : 0

    implicitHeight: row.padY * 2 + row.lineH + row.blockGap + blockBox.height
        + (Look.adwaita && row.hasLine && blockBox.height > 0 ? Math.round(12 * row.u) : 0)
    width: parent ? parent.width : implicitWidth
    height: implicitHeight

    readonly property bool hovered: hover.hovered && row.activatable
    readonly property bool pressed: tap.pressed && row.activatable

    // Fluent's card
    Rectangle {
        visible: row.card
        anchors.fill: parent
        radius: 4 * row.u
        color: row.pressed ? Lithic.mix(Look.planes.card, Look.fg, 0.02)
            : row.hovered ? Lithic.mix(Look.planes.card, Look.fg, Look.dark ? 0.05 : 0.03)
            : Look.planes.card
        border.width: Math.max(1, Math.round(row.u))
        border.color: Look.planes.cardStroke
        antialiasing: true
        Behavior on color {
            ColorAnimation { duration: Look.dur(Look.stateMs) }
        }
    }
    // where the row sits in its list, so Adwaita's state layer keeps the boxed list's corners
    readonly property bool firstInList: !!row.parent && !!row.parent.visibleChildren && row.parent.visibleChildren[0] === row
    readonly property bool lastInList: !!row.parent && !!row.parent.visibleChildren
        && row.parent.visibleChildren[row.parent.visibleChildren.length - 1] === row
    // the others' state layer under an activatable row
    Rectangle {
        visible: !row.card && row.activatable && !Look.lithic
        anchors.fill: parent
        anchors.leftMargin: Look.material ? -Math.round(12 * row.u) : 0
        anchors.rightMargin: Look.material ? -Math.round(12 * row.u) : 0
        readonly property real ends: Look.material ? 16 * row.u : Look.adwaita ? 12 * row.u : 4 * row.u
        topLeftRadius: !Look.adwaita || row.firstInList ? ends : 0
        topRightRadius: topLeftRadius
        bottomLeftRadius: !Look.adwaita || row.lastInList ? ends : 0
        bottomRightRadius: bottomLeftRadius
        color: row.pressed ? Look.press : row.hovered ? Look.hover : Lithic.alpha(Look.hover, 0)
        Behavior on color {
            ColorAnimation { duration: Look.dur(Look.stateMs) }
        }
    }
    CutRect {
        visible: Look.lithic && row.activatable
        anchors.fill: parent
        anchors.leftMargin: -Math.round(6 * row.u)
        anchors.rightMargin: -Math.round(6 * row.u)
        design: "lithic"
        cut: Look.cutSm * row.u
        fill: row.pressed ? Look.press : row.hovered ? Look.hover : Lithic.alpha(Look.hover, 0)
    }

    Item {
        id: line
        visible: row.hasLine
        x: row.padX
        y: row.padY
        width: row.width - row.padX * 2
        height: row.lineH

        Icon {
            id: glyphIcon
            visible: row.showGlyph
            anchors.verticalCenter: parent.verticalCenter
            width: row.glyphSize
            height: width
            size: width
            name: row.glyph
            tint: Look.material ? Look.fg2 : Look.fg
        }

        Column {
            id: words
            anchors.verticalCenter: parent.verticalCenter
            x: row.showGlyph ? glyphIcon.width + Math.round((Look.material ? 16 : 14) * row.u) : 0
            width: row.titleWidth > 0 ? row.titleWidth
                : Math.max(0, line.width - x - trailingBox.width - Math.round(12 * row.u))
            spacing: Math.round((Look.lithic ? 2 : 1) * row.u)

            Text {
                width: parent.width
                elide: Text.ElideRight
                text: Look.lithic ? row.title.toUpperCase() : row.title
                color: Look.lithic ? Look.fg2 : Look.fg
                font.family: Look.lithic ? Look.mono : Look.sans
                font.pixelSize: Math.round((Look.lithic ? 9.5 : Look.material ? 15 : 14) * row.u)
                font.weight: Look.lithic ? Font.Bold : Font.Normal
                font.letterSpacing: Look.lithic ? Look.tracking * row.u : 0
                renderType: Text.CurveRendering
            }
            Text {
                visible: row.subtitle.length > 0
                width: parent.width
                wrapMode: Text.Wrap
                maximumLineCount: 2
                elide: Text.ElideRight
                text: row.subtitle
                color: Look.lithic ? Look.fg3 : Look.fg2
                font.family: Look.lithic ? Look.mono : Look.sans
                font.pixelSize: Math.round((Look.lithic ? 9 : Look.material ? 13 : 12) * row.u)
                font.features: ({ "tnum": 1 })
                renderType: Text.CurveRendering
            }
        }

        // a positioner, so a control hidden in this design takes no room
        Row {
            id: trailingBox
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            width: row.fillTrailing ? Math.max(0, line.width - words.x - words.width - Math.round(12 * row.u)) : implicitWidth
        }
    }

    Column {
        id: blockBox
        x: row.padX
        y: row.padY + row.lineH + row.blockGap
        width: row.width - row.padX * 2
    }

    HoverHandler {
        id: hover
        enabled: row.activatable
        cursorShape: Qt.PointingHandCursor
    }
    TapHandler {
        id: tap
        enabled: row.activatable
        onTapped: row.clicked()
    }
}

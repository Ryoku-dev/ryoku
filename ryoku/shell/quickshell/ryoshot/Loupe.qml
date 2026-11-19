import QtQuick
import "look"

// The magnified frozen pixels around one point, shared by every design's loupe.
// Cells are whole device pixels of the frame, drawn nearest-neighbour on a
// faint grid with the pixel under the point boxed; the frame and the readout
// around it belong to the design. The lens fills its own box and is clipped to
// it, so a design can mask it round or inset it from a cut.
Item {
    id: lens

    required property Item source
    // the sampled point in the source's logical coordinates
    property real cx: 0
    property real cy: 0
    property real dpr: 1
    property real u: 1
    // logical width of one magnified pixel before it snaps to whole device pixels
    property real cellSize: 8
    property color markColor: Look.sel
    property color gridColor: Qt.rgba(Look.scrim.r, Look.scrim.g, Look.scrim.b, 0.22)

    readonly property real cell: Math.max(2, Math.round(lens.cellSize * lens.u * lens.dpr)) / lens.dpr
    readonly property int cells: {
        var n = Math.ceil(Math.max(lens.width, lens.height) / lens.cell);
        return n % 2 === 0 ? n + 1 : n;
    }
    readonly property int half: (lens.cells - 1) / 2
    readonly property real span: lens.cells * lens.cell
    readonly property real hair: 1 / lens.dpr
    readonly property real origin: Math.round((lens.width - lens.span) / 2 * lens.dpr) / lens.dpr
    readonly property real originY: Math.round((lens.height - lens.span) / 2 * lens.dpr) / lens.dpr

    clip: true

    Rectangle {
        anchors.fill: parent
        color: Look.ground
    }

    ShaderEffectSource {
        x: lens.origin
        y: lens.originY
        width: lens.span
        height: lens.span
        sourceItem: lens.source
        live: lens.visible
        smooth: false
        mipmap: false
        sourceRect: Qt.rect((Math.floor(lens.cx * lens.dpr) - lens.half) / lens.dpr,
                            (Math.floor(lens.cy * lens.dpr) - lens.half) / lens.dpr,
                            lens.cells / lens.dpr, lens.cells / lens.dpr)
    }

    Repeater {
        model: lens.cells + 1
        Rectangle {
            required property int index
            x: lens.origin + index * lens.cell
            y: 0
            width: lens.hair
            height: lens.height
            color: lens.gridColor
        }
    }
    Repeater {
        model: lens.cells + 1
        Rectangle {
            required property int index
            x: 0
            y: lens.originY + index * lens.cell
            width: lens.width
            height: lens.hair
            color: lens.gridColor
        }
    }

    Rectangle {
        readonly property real w: 2 / lens.dpr
        x: lens.origin + lens.half * lens.cell - w
        y: lens.originY + lens.half * lens.cell - w
        width: lens.cell + w * 2
        height: lens.cell + w * 2
        color: "transparent"
        border.width: w
        border.color: lens.markColor
    }
}

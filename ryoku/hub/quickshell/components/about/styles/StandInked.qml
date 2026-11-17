import QtQuick
import Lithic

// lettering as a manga letterer inks it: the face, a thick ink line all round, and a hard
// extrusion falling away behind. Katakana and kanji set in Noto Sans CJK, Latin in the sans
// at its heaviest, slanted. Given a width to keep to, it sets itself smaller rather than clip
Item {
    id: inked

    property string text
    property bool kana: false
    property real size: 40
    property real spacing: 0
    property color fill: "white"
    property color line: "black"
    property real lineWidth: 2.5
    property color shade: "black"
    property real depth: 6
    // the extrusion's direction, degrees from the right, clockwise
    property real dir: 45
    property real stretch: 1
    // the widest the lettering may be, 0 for any
    property real fitWidth: 0

    readonly property real dx: Math.cos(inked.dir * Math.PI / 180)
    readonly property real dy: Math.sin(inked.dir * Math.PI / 180)
    readonly property real room: inked.fitWidth > 0 ? inked.fitWidth - 2 * inked.lineWidth - inked.depth * Math.abs(inked.dx) : 0
    readonly property real setSize: inked.room > 0 && measure.advanceWidth > inked.room
        ? Math.max(6, inked.size * inked.room / measure.advanceWidth) : inked.size
    readonly property string family: inked.kana ? "Noto Sans CJK JP" : Lithic.font.sans
    readonly property int ring: 12

    implicitWidth: face.implicitWidth + 2 * inked.lineWidth + inked.depth * Math.abs(inked.dx)
    implicitHeight: face.implicitHeight * inked.stretch + 2 * inked.lineWidth + inked.depth * Math.abs(inked.dy)
    width: implicitWidth
    height: implicitHeight

    TextMetrics {
        id: measure
        text: inked.text
        font.family: inked.family
        font.weight: Font.Black
        font.italic: !inked.kana
        font.pixelSize: Math.max(1, Math.round(inked.size))
        font.letterSpacing: inked.spacing
    }

    component Pass: Text {
        textFormat: Text.PlainText
        renderType: Text.QtRendering
        text: inked.text
        font.family: inked.family
        font.weight: Font.Black
        font.italic: !inked.kana
        font.pixelSize: Math.max(1, Math.round(inked.setSize))
        font.letterSpacing: inked.spacing
        transform: Scale { yScale: inked.stretch }
    }

    // the extrusion's own ink edge, then the extrusion, then the line round the face
    Repeater {
        model: inked.depth > 0 && inked.lineWidth > 0 ? inked.ring : 0
        delegate: Pass {
            required property int index
            x: inked.lineWidth + inked.depth * inked.dx + Math.cos(index / inked.ring * 2 * Math.PI) * inked.lineWidth
            y: inked.lineWidth + inked.depth * inked.dy + Math.sin(index / inked.ring * 2 * Math.PI) * inked.lineWidth
            color: inked.line
        }
    }
    Repeater {
        model: inked.depth > 0 ? Math.ceil(inked.depth / 0.8) : 0
        delegate: Pass {
            required property int index
            x: inked.lineWidth + (inked.depth - index * 0.8) * inked.dx
            y: inked.lineWidth + (inked.depth - index * 0.8) * inked.dy
            color: inked.shade
        }
    }
    Repeater {
        model: inked.lineWidth > 0 ? inked.ring : 0
        delegate: Pass {
            required property int index
            x: inked.lineWidth + Math.cos(index / inked.ring * 2 * Math.PI) * inked.lineWidth
            y: inked.lineWidth + Math.sin(index / inked.ring * 2 * Math.PI) * inked.lineWidth
            color: inked.line
        }
    }
    Pass {
        id: face
        x: inked.lineWidth
        y: inked.lineWidth
        color: inked.fill
    }
}

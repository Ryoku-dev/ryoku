import QtQuick
import Lithic

// the tick ruler under a sheet's head, as on ryoku.dev: vein ticks a step apart, every fifth
// one tall, and a first segment in `lead`. It is drawn once and never repaints
Item {
    id: ruler

    property color lead: Lithic.fg

    implicitWidth: 233
    implicitHeight: LithicTokens.space.s2

    Repeater {
        model: Math.floor(ruler.width / LithicTokens.space.s2) + 1
        delegate: Rectangle {
            required property int index
            x: index * LithicTokens.space.s2
            y: ruler.height - height
            width: LithicTokens.size.hairline
            height: index % 5 === 0 ? LithicTokens.space.s2 : LithicTokens.space.s1
            color: Lithic.vein
        }
    }
    Rectangle {
        y: ruler.height - height
        width: LithicTokens.space.s4
        height: 2
        color: ruler.lead
    }
}

import QtQuick
import Ryoku.Ui.Singletons

// Visual options: where no label carries the difference. "Engraved bracket
// cells" and "three islands with concave dips" are both true and both useless
// until you see them, so each tile draws the option instead of naming it.
//
// Tiles draw a 1-bit silhouette, never a screenshot: a screenshot brings its
// own colour and stops being legible at 32px.
Flow {
    id: gal
    property var options: []      // [{ key, origin, draw }]
    // Swap-in tile painter so one Gallery can draw a different catalogue (the
    // visualiser looks via VizStyles); left null it keeps drawing the bar skins.
    property var painter: null
    property string current: ""
    signal chose(string key)

    spacing: 7

    Repeater {
        model: gal.options
        Rectangle {
            id: tile
            required property var modelData
            readonly property bool on: gal.current === modelData.key

            width: 132
            height: 74
            radius: Tokens.radius
            color: tap.pressed ? Tokens.tint16 : (on ? Tokens.tint10 : (th.hovered ? Tokens.tint5 : "transparent"))
            border.width: Tokens.border
            border.color: on ? Tokens.ink : (th.hovered ? Tokens.lineStrong : Tokens.line)
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
            Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

            Canvas {
                id: art
                anchors { left: parent.left; right: parent.right; top: parent.top; margins: 7 }
                height: 32
                onPaint: {
                    var c = getContext("2d");
                    c.reset();
                    (gal.painter || Silhouette).draw(c, tile.modelData.draw, width, height,
                                    tile.on ? 0.98 : 0.62, tile.on ? 0.45 : 0.28);
                }
                Component.onCompleted: requestPaint()
                Connections {
                    target: tile
                    function onOnChanged() { art.requestPaint() }
                }
            }
            Text {
                anchors { left: parent.left; leftMargin: 7; bottom: parent.bottom; bottomMargin: 17 }
                text: tile.modelData.key
                color: tile.on ? Tokens.ink : Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: 12
                font.weight: tile.on ? Font.DemiBold : Font.Normal
            }
            Text {
                anchors { left: parent.left; leftMargin: 7; bottom: parent.bottom; bottomMargin: 5 }
                text: tile.modelData.origin || ""
                color: Tokens.inkFaint
                font.family: Tokens.mono
                font.pixelSize: 9
            }
            Text {
                anchors { right: parent.right; rightMargin: 7; bottom: parent.bottom; bottomMargin: 5 }
                visible: tile.on
                text: "●"
                color: Tokens.ink
                font.pixelSize: 7
            }
            HoverHandler { id: th; cursorShape: Qt.PointingHandCursor }
            TapHandler { id: tap; onTapped: gal.chose(tile.modelData.key) }
        }
    }
}

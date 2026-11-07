import QtQuick
import Ryoku.Ui.Singletons

// A small 1-bit pixel-art glyph on an 8x8 grid, drawn from the brand's own
// vocabulary -- Greek and Japanese, never arcade-alien: a meander (Greek key), a
// torii gate, seigaiha waves, a fluted column, an asanoha star. Ornament for a
// margin: a pixel mark reads as the poster's dingbat without pretending to be an
// icon a control needs. Ink only; it dresses dead space, never state.
Canvas {
    id: px

    property string kind: "meander"
    property color color: Tokens.inkFaint

    readonly property var grids: ({
        // Greek key / meander -- a squared spiral fret.
        "meander": ["11111110", "10000010", "10111010", "10101010", "10101110", "10100000", "11111111", "00000000"],
        // torii gate -- twin lintels over splayed posts.
        "torii":   ["01111110", "11111111", "00100100", "01111110", "00100100", "00100100", "00100100", "01100110"],
        // seigaiha -- stacked wave arcs.
        "wave":    ["00111100", "01000010", "10000001", "00111100", "01000010", "10000001", "00111100", "01000010"],
        // fluted Doric column -- capital, shaft, base.
        "column":  ["11111111", "01111110", "01011010", "01011010", "01011010", "01011010", "01111110", "11111111"],
        // asanoha -- the hemp-leaf hexagonal star.
        "asanoha": ["00011000", "01011010", "10011001", "11111111", "11111111", "10011001", "01011010", "00011000"],
        // a plain 4-point register star, the neutral dingbat.
        "star":    ["00010000", "00010000", "00111000", "11111110", "00111000", "00010000", "00010000", "00000000"]
    })

    onKindChanged: requestPaint()
    onColorChanged: requestPaint()
    onWidthChanged: requestPaint()
    onHeightChanged: requestPaint()
    Component.onCompleted: requestPaint()

    onPaint: {
        var c = getContext("2d");
        c.reset();
        var g = grids[kind] || grids["meander"];
        var rows = g.length, cols = g[0].length;
        var cw = width / cols, ch = height / rows;
        c.fillStyle = px.color;
        for (var y = 0; y < rows; y++)
            for (var x = 0; x < cols; x++)
                if (g[y][x] === "1")
                    c.fillRect(Math.floor(x * cw), Math.floor(y * ch), Math.ceil(cw), Math.ceil(ch));
    }
}

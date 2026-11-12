import QtQuick
import Lithic

// a Code 39 barcode that scans, in the card's ink, with its text below
Item {
    id: code

    property string text: ""
    property color ink: Lithic.fg
    property int barHeight: 28
    property real unit: 2

    readonly property var glyphs: ({
        "0": "bwbWBwBwb", "1": "BwbWbwbwB", "2": "bwBWbwbwB", "3": "BwBWbwbwb",
        "4": "bwbWBwbwB", "5": "BwbWBwbwb", "6": "bwBWBwbwb", "7": "bwbWbwBwB",
        "8": "BwbWbwBwb", "9": "bwBWbwBwb", "A": "BwbwbWbwB", "B": "bwBwbWbwB",
        "C": "BwBwbWbwb", "D": "bwbwBWbwB", "E": "BwbwBWbwb", "F": "bwBwBWbwb",
        "G": "bwbwbWBwB", "H": "BwbwbWBwb", "I": "bwBwbWBwb", "J": "bwbwBWBwb",
        "K": "BwbwbwbWB", "L": "bwBwbwbWB", "M": "BwBwbwbWb", "N": "bwbwBwbWB",
        "O": "BwbwBwbWb", "P": "bwBwBwbWb", "Q": "bwbwbwBWB", "R": "BwbwbwBWb",
        "S": "bwBwbwBWb", "T": "bwbwBwBWb", "U": "BWbwbwbwB", "V": "bWBwbwbwB",
        "W": "BWBwbwbwb", "X": "bWbwBwbwB", "Y": "BWbwBwbwb", "Z": "bWBwBwbwb",
        "-": "bWbwbwBwB", ".": "BWbwbwBwb", " ": "bWBwbwBwb", "*": "bWbwBwBwb"
    })
    readonly property string coded: "*" + code.text.toUpperCase() + "*"

    implicitWidth: code.coded.length * 16 * code.unit
    implicitHeight: code.barHeight + 14
    width: implicitWidth
    height: implicitHeight

    Canvas {
        anchors.fill: parent
        property string coded: code.coded
        property color ink: code.ink
        property real unit: code.unit
        onCodedChanged: requestPaint()
        onInkChanged: requestPaint()
        onUnitChanged: requestPaint()
        onPaint: {
            var c = getContext("2d");
            c.reset();
            c.fillStyle = ink;
            var x = 0;
            for (var i = 0; i < coded.length; i++) {
                var g = code.glyphs[coded[i]];
                if (!g)
                    continue;
                for (var j = 0; j < g.length; j++) {
                    var wide = g[j] === "B" || g[j] === "W";
                    var w = (wide ? 3 : 1) * unit;
                    if (g[j] === "b" || g[j] === "B")
                        c.fillRect(x, 0, w, code.barHeight);
                    x += w;
                }
                x += unit;
            }
        }
    }

    Text {
        y: code.barHeight + 3
        text: code.text.toUpperCase()
        color: Lithic.fg3
        font.family: Lithic.font.mono
        font.pixelSize: 9
        font.letterSpacing: 1.6
    }
}

import QtQuick
import Ryoku.Ui.Singletons

// Code 39, drawn as ink bars, and it scans: tests/ui/barcode.sh reads it back
// with zbarimg. Give it room. A clipped barcode loses its stop bars and stops
// being a barcode, which is a silent failure: it still looks like one.
Item {
    id: code

    property string text: ""
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

    implicitWidth: cv.implicitWidth
    implicitHeight: barHeight + 14

    Canvas {
        id: cv
        anchors.fill: parent
        implicitWidth: {
            var s = "*" + code.text.toUpperCase() + "*";
            return s.length * 16 * code.unit;
        }
        onPaint: {
            var c = getContext("2d");
            c.reset();
            c.fillStyle = Qt.rgba(205 / 255, 196 / 255, 186 / 255, 1);
            var s = "*" + code.text.toUpperCase() + "*";
            var x = 0;
            for (var i = 0; i < s.length; i++) {
                var g = code.glyphs[s[i]];
                if (!g) continue;
                for (var j = 0; j < g.length; j++) {
                    var ch = g[j];
                    var wide = (ch === "B" || ch === "W");
                    var bar = (ch === "b" || ch === "B");
                    var w = (wide ? 3 : 1) * code.unit;
                    if (bar) c.fillRect(x, 0, w, code.barHeight);
                    x += w;
                }
                x += code.unit;   // the inter-character gap
            }
        }
        Component.onCompleted: requestPaint()
        onImplicitWidthChanged: requestPaint()
    }

    Text {
        anchors { left: parent.left; top: cv.top; topMargin: code.barHeight + 3 }
        text: code.text.toUpperCase()
        color: Tokens.inkFaint
        font.family: Tokens.mono
        font.pixelSize: 9
        font.letterSpacing: 1.6
    }
}

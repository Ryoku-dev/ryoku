import QtQuick
import Lithic
import "../../Singletons"

// a live trace for the room under the sidebar's tree: the processor's last readings as a dot
// matrix, newest in the sun, with the figures beside it. It samples only while Settings is
// focused and it is on show (Ambient.on), and stands still otherwise
Item {
    id: root

    readonly property int length: 44
    property var series: []
    readonly property bool sampling: root.visible && Ambient.on

    Binding {
        target: LiveStats
        property: "active"
        value: true
        when: root.sampling
    }
    Connections {
        target: LiveStats
        enabled: root.sampling
        function onTickChanged() {
            root.series = root.series.concat([LiveStats.cpu]).slice(-root.length);
        }
    }

    implicitHeight: 64

    Row {
        id: figures
        x: 8
        spacing: 14

        Repeater {
            model: [
                { name: "CPU", value: LiveStats.cpu + "%" },
                { name: "MEM", value: LiveStats.ram + "%" },
                { name: "TMP", value: LiveStats.cpuTemp > 0 ? LiveStats.cpuTemp + "°" : "--" }
            ]
            delegate: Row {
                required property var modelData
                spacing: 5
                Label {
                    text: modelData.name
                    typeRole: "monoSm"
                    inkRole: "fg3"
                    font.pixelSize: 9
                    font.letterSpacing: 1.2
                }
                Label {
                    text: LiveStats.tick > 0 ? modelData.value : "--"
                    typeRole: "monoSm"
                    inkRole: "fg2"
                    font.pixelSize: 9
                }
            }
        }
    }

    // each reading a column of 3 px dots, as tall as its share
    Canvas {
        id: matrix
        x: 8
        y: figures.height + 8
        width: root.width - 16
        height: root.height - y
        property var series: root.series
        property color ink: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.28 : 0.34)
        property color lead: Lithic.sel
        onSeriesChanged: requestPaint()
        onInkChanged: requestPaint()
        onWidthChanged: requestPaint()
        onPaint: {
            var c = getContext("2d");
            c.reset();
            var step = 5, rows = Math.floor(height / 4);
            var cols = Math.floor(width / step);
            var s = matrix.series.slice(-cols);
            var start = cols - s.length;
            for (var col = 0; col < cols; col++) {
                var at = col - start;
                var v = at >= 0 ? s[at] : 0;
                var lit = Math.max(1, Math.round(rows * v / 100));
                for (var r = 0; r < rows; r++) {
                    var on = r < lit && at >= 0;
                    c.fillStyle = on ? (at === s.length - 1 ? matrix.lead : matrix.ink) : Lithic.alpha(matrix.ink, 0.25);
                    c.fillRect(col * step, height - (r + 1) * 4 + 1, 3, 3);
                }
            }
        }
    }
}

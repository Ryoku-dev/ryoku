import QtQuick
import Ryoku.Ui.Singletons

Canvas {
    id: root

    property var values: []
    property real fixedMax: 0
    property color stroke: Tokens.inkMuted

    renderTarget: Canvas.Image
    antialiasing: true
    onValuesChanged: requestPaint()
    onFixedMaxChanged: requestPaint()
    onWidthChanged: requestPaint()
    onHeightChanged: requestPaint()

    onPaint: {
        var ctx = getContext("2d");
        ctx.reset();
        if (width <= 1 || height <= 1)
            return;
        ctx.strokeStyle = Tokens.lineSoft.toString();
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.moveTo(0, height - 0.5);
        ctx.lineTo(width, height - 0.5);
        ctx.stroke();

        var data = root.values || [];
        if (data.length === 0)
            return;
        var high = root.fixedMax;
        if (high <= 0) {
            high = 1;
            for (var i = 0; i < data.length; i++) high = Math.max(high, Number(data[i]) || 0);
        }
        var step = data.length > 1 ? width / (data.length - 1) : width;
        ctx.strokeStyle = root.stroke.toString();
        ctx.lineWidth = 1.25;
        ctx.beginPath();
        for (var j = 0; j < data.length; j++) {
            var x = data.length > 1 ? j * step : width;
            var y = height - 1 - Math.min(1, Math.max(0, Number(data[j]) || 0) / high) * (height - 3);
            if (j === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
        }
        ctx.stroke();
    }
}

pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: root

    property string title: ""
    property string unit: ""
    property var values: []
    property real fixedMax: 0
    property int samplePeriodSeconds: 5
    property var formatter: null
    property real latest: values.length > 0 ? Number(values[values.length - 1]) : 0
    property real peak: {
        var high = 0;
        for (var i = 0; i < values.length; i++)
            high = Math.max(high, Number(values[i]) || 0);
        return high;
    }
    property real ceiling: fixedMax > 0 ? fixedMax : Math.max(1, peak)

    function displayValue(value) {
        if (formatter)
            return formatter(value);
        value = Number(value) || 0;
        return value.toFixed(value >= 100 ? 0 : 1) + unit;
    }
    function spanText() {
        var seconds = Math.max(samplePeriodSeconds, Math.max(0, values.length - 1) * samplePeriodSeconds);
        return seconds >= 60
            ? I18n.tr("LAST %1 MIN").arg(Math.max(1, Math.round(seconds / 60)))
            : I18n.tr("LAST %1 SEC").arg(seconds);
    }

    implicitHeight: 132
    onValuesChanged: plot.requestPaint()
    onCeilingChanged: plot.requestPaint()

    Text {
        id: label
        anchors.left: parent.left
        anchors.top: parent.top
        text: root.title
        color: Tokens.inkMuted
        font.family: Tokens.ui
        font.pixelSize: 10
        font.weight: Font.Medium
    }

    Text {
        anchors.right: parent.right
        anchors.baseline: label.baseline
        text: root.values.length > 0
            ? I18n.tr("NOW %1").arg(root.displayValue(root.latest))
            : I18n.tr("Waiting for samples")
        color: Tokens.ink
        font.family: Tokens.mono
        font.pixelSize: 10
    }

    Canvas {
        id: plot
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: label.bottom
        anchors.topMargin: Tokens.s2
        anchors.bottom: scale.top
        anchors.bottomMargin: Tokens.s1
        renderTarget: Canvas.Image
        antialiasing: true

        onWidthChanged: requestPaint()
        onHeightChanged: requestPaint()
        onPaint: {
            var ctx = getContext("2d");
            ctx.reset();
            if (width <= 1 || height <= 1)
                return;

            ctx.lineWidth = 1;
            ctx.strokeStyle = Tokens.lineSoft.toString();
            var gy = Math.round((height - 1) / 2) + 0.5;
            ctx.beginPath();
            ctx.moveTo(0, gy);
            ctx.lineTo(width, gy);
            ctx.stroke();

            var data = root.values || [];
            if (data.length === 0)
                return;
            var step = data.length > 1 ? width / (data.length - 1) : width;
            ctx.strokeStyle = Tokens.ink.toString();
            ctx.lineWidth = 1.5;
            ctx.beginPath();
            for (var i = 0; i < data.length; i++) {
                var value = Math.max(0, Number(data[i]) || 0);
                var x = data.length > 1 ? i * step : width;
                var y = height - 2 - Math.min(1, value / Math.max(1, root.ceiling)) * (height - 4);
                if (i === 0)
                    ctx.moveTo(x, y);
                else
                    ctx.lineTo(x, y);
            }
            ctx.stroke();

            ctx.fillStyle = Tokens.bone.toString();
            var lastX = data.length > 1 ? (data.length - 1) * step : width;
            var lastValue = Math.max(0, Number(data[data.length - 1]) || 0);
            var lastY = height - 2 - Math.min(1, lastValue / Math.max(1, root.ceiling)) * (height - 4);
            ctx.beginPath();
            ctx.arc(lastX - 2, lastY, 2.5, 0, Math.PI * 2);
            ctx.fill();
        }
    }
    Text {
        id: scale
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        text: root.values.length > 0
            ? root.spanText() + "   ·   " + I18n.tr("PEAK %1").arg(root.displayValue(root.peak))
            : ""
        color: Tokens.inkFaint
        font.family: Tokens.mono
        font.pixelSize: 8
        font.letterSpacing: 0.5
    }
}

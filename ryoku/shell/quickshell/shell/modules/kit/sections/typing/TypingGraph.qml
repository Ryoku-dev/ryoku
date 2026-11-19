pragma ComponentBehavior: Bound
import QtQuick
import Lithic
import "../../../ask/look"
Canvas {
    id: graph
    required property real s
    property var values: []
    property real ceiling: Math.max(20, ...values.map(v => Number(v) || 0)) * 1.15
    property color ink: Lithic.lapis
    onValuesChanged: requestPaint()
    onInkChanged: requestPaint()
    onWidthChanged: requestPaint()
    onHeightChanged: requestPaint()
    onPaint: {
        const c = getContext("2d"); c.reset();
        const left = 32 * s, bottom = height - 20 * s, top = 8 * s;
        c.font = (10 * s) + 'px "Martian Mono"'; c.fillStyle = Lithic.fg3;
        for (let i = 0; i <= 2; i++) {
            const y = top + (bottom - top) * i / 2;
            c.fillText(String(Math.round(ceiling * (1 - i / 2))), 0, y + 4 * s);
            c.strokeStyle = Look.divider.toString() === "#00000000" ? Lithic.vein : Look.divider;
            c.globalAlpha = 0.45; c.beginPath(); c.moveTo(left, y); c.lineTo(width, y); c.stroke(); c.globalAlpha = 1;
        }
        if (!values.length) return;
        const point = i => [left + i * (width - left - 2 * s) / Math.max(1, values.length - 1), bottom - Number(values[i]) / ceiling * (bottom - top)];
        c.beginPath(); values.forEach((v, i) => { const p = point(i); if (!i) c.moveTo(...p); else c.lineTo(...p); });
        c.strokeStyle = ink; c.lineWidth = 1.5 * s; c.stroke();
        const last = point(values.length - 1); c.lineTo(last[0], bottom); c.lineTo(left, bottom); c.closePath(); c.fillStyle = ink; c.globalAlpha = 0.08; c.fill();
    }
}

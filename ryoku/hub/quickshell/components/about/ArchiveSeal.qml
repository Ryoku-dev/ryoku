import QtQuick
import Lithic

Item {
    id: seal

    property string title: "RYOKU"
    property string sigil: "力"
    property string rarity: "common"
    property string category: "RYOKU"
    property bool locked: false
    property bool animateFoil: false
    property real foilPhase: 0
    readonly property color tint: ({
        common: "#9499a6",
        uncommon: "#86bca6",
        rare: "#65b9e1",
        epic: "#c38be6",
        legendary: "#f1bb6e",
        mythic: "#f17854",
        holographic: "#c5a8f6"
    })[rarity] || "#9499a6"

    implicitWidth: 150
    implicitHeight: 150

    SequentialAnimation on foilPhase {
        running: seal.animateFoil && seal.rarity === "holographic" && !seal.locked
        loops: Animation.Infinite
        NumberAnimation { from: 0; to: 1; duration: 5500 }
        NumberAnimation { from: 1; to: 0; duration: 5500 }
    }

    Canvas {
        id: face
        anchors.fill: parent
        renderTarget: Canvas.Image
        renderStrategy: Canvas.Threaded
        antialiasing: true
        onWidthChanged: requestPaint()
        onHeightChanged: requestPaint()

        function arcText(ctx, word, radius, angle, size, color, clockwise) {
            var text = String(word || "").toUpperCase().slice(0, 23);
            var advance = Math.min(0.16, Math.PI / Math.max(6, text.length + 2));
            var start = angle - advance * (text.length - 1) * (clockwise ? 0.5 : -0.5);
            ctx.font = "bold " + size + "px sans-serif";
            ctx.fillStyle = color;
            ctx.textAlign = "center";
            ctx.textBaseline = "middle";
            for (var i = 0; i < text.length; i++) {
                ctx.save();
                var t = start + advance * i * (clockwise ? 1 : -1);
                ctx.rotate(t);
                ctx.translate(0, -radius);
                ctx.rotate(clockwise ? 0 : Math.PI);
                ctx.fillText(text[i], 0, 0);
                ctx.restore();
            }
        }

        onPaint: {
            var c = getContext("2d");
            c.reset();
            var scale = Math.min(width, height) / 200;
            c.save();
            c.translate(width / 2, height / 2);
            c.scale(scale, scale);

            var base = seal.locked ? "#111318" : "#17181d";
            var ink = seal.locked ? "#535661" : seal.tint.toString();
            if (seal.rarity === "holographic" && !seal.locked) {
                var shine = c.createLinearGradient(-95, -95, 95, 95);
                shine.addColorStop(0, "#73d4e1");
                shine.addColorStop(Math.max(0.01, seal.foilPhase), "#eb94d9");
                shine.addColorStop(1, "#e7bd7b");
                ink = shine;
            }

            c.beginPath();
            c.arc(0, 0, 95, 0, Math.PI * 2);
            c.fillStyle = base;
            c.fill();
            c.lineWidth = 1.5;
            c.strokeStyle = ink;
            c.stroke();

            c.beginPath();
            c.arc(0, 0, 90, 0, Math.PI * 2);
            c.lineWidth = 0.9;
            c.strokeStyle = ink;
            c.stroke();

            for (var i = 0; i < 96; i++) {
                var t = i * Math.PI / 48;
                c.save();
                c.rotate(t);
                c.beginPath();
                c.moveTo(0, -93);
                c.lineTo(0, i % 8 === 0 ? -86 : i % 4 === 0 ? -89 : -91);
                c.lineWidth = i % 8 === 0 ? 1.6 : 0.7;
                c.strokeStyle = ink;
                c.stroke();
                c.restore();
            }

            c.beginPath();
            c.arc(0, 0, 80, 0, Math.PI * 2);
            c.lineWidth = 1;
            c.strokeStyle = ink;
            c.stroke();

            c.beginPath();
            c.arc(0, 0, 49, 0, Math.PI * 2);
            c.lineWidth = 0.75;
            c.strokeStyle = ink;
            c.stroke();

            if (["epic", "legendary", "mythic", "holographic"].indexOf(seal.rarity) >= 0) {
                c.beginPath();
                c.arc(0, 0, 76, Math.PI * 0.82, Math.PI * 1.18);
                c.arc(0, 0, 76, Math.PI * 1.82, Math.PI * 2.18);
                c.lineWidth = 2.2;
                c.strokeStyle = ink;
                c.stroke();
                for (var ornament = 0; ornament < 16; ornament++) {
                    c.save();
                    c.rotate(ornament * Math.PI / 8);
                    c.beginPath();
                    c.moveTo(0, -83);
                    c.lineTo(3, -81);
                    c.lineTo(0, -79);
                    c.lineTo(-3, -81);
                    c.closePath();
                    c.fillStyle = ink;
                    c.fill();
                    c.restore();
                }
            }
            if (seal.rarity === "mythic" || seal.rarity === "holographic") {
                c.beginPath();
                c.arc(0, 0, 43, 0, Math.PI * 2);
                c.lineWidth = 0.8;
                c.strokeStyle = ink;
                c.stroke();
            }

            for (var j = 0; j < 4; j++) {
                c.save();
                c.rotate(j * Math.PI / 2);
                c.fillStyle = ink;
                c.fillRect(-3, -55, 6, 6);
                c.restore();
            }

            c.fillStyle = ink;
            c.font = "bold 46px sans-serif";
            c.textAlign = "center";
            c.textBaseline = "middle";
            c.fillText(seal.locked ? "?" : seal.sigil, 0, 2, 70);

            c.fillStyle = ink;
            c.font = "bold 9px monospace";
            c.fillText("力", 0, -65);
            arcText(c, seal.title, 66, -Math.PI / 2, 10, ink, true);
            arcText(c, seal.category, 70, Math.PI / 2, 8, ink, false);

            c.font = "7px monospace";
            c.fillText(seal.rarity.toUpperCase(), 0, 72);
            c.restore();
        }
    }

    onTintChanged: face.requestPaint()
    onTitleChanged: face.requestPaint()
    onSigilChanged: face.requestPaint()
    onRarityChanged: face.requestPaint()
    onCategoryChanged: face.requestPaint()
    onLockedChanged: face.requestPaint()
    onFoilPhaseChanged: if (animateFoil) face.requestPaint()
}

import QtQuick
import Lithic

Item {
    id: prism

    property bool live: true
    property real phase: 0
    readonly property bool moving: live && visible && !Lithic.reduceMotion

    clip: true
    implicitWidth: 480
    implicitHeight: 270

    NumberAnimation on phase {
        from: 0
        to: 1
        duration: 18000
        loops: Animation.Infinite
        running: prism.moving
    }

    Rectangle {
        anchors.fill: parent
        color: "#080a11"
    }

    Canvas {
        anchors.fill: parent
        renderTarget: Canvas.Image
        renderStrategy: Canvas.Threaded
        antialiasing: true
        onWidthChanged: requestPaint()
        onHeightChanged: requestPaint()

        onPaint: {
            var c = getContext("2d");
            c.reset();
            var w = width, h = height;
            var cx = w * 0.56, cy = h * 0.47;
            c.fillStyle = "#080a11";
            c.fillRect(0, 0, w, h);

            var atmosphere = c.createRadialGradient(cx, cy, 12, cx, cy, Math.max(w, h) * 0.68);
            atmosphere.addColorStop(0, "#25213a");
            atmosphere.addColorStop(0.24, "#18192e");
            atmosphere.addColorStop(0.55, "#10131f");
            atmosphere.addColorStop(1, "#080a11");
            c.fillStyle = atmosphere;
            c.fillRect(0, 0, w, h);

            c.save();
            c.translate(cx, cy);
            for (var r = 0; r < 6; r++) {
                var scale = 42 + r * 43;
                c.beginPath();
                for (var i = 0; i < 8; i++) {
                    var angle = Math.PI * 2 * i / 8 + Math.PI / 8;
                    var x = Math.cos(angle) * scale;
                    var y = Math.sin(angle) * scale;
                    if (!i) c.moveTo(x, y);
                    else c.lineTo(x, y);
                }
                c.closePath();
                c.lineWidth = r % 2 ? 0.7 : 1.3;
                c.strokeStyle = ["#55435d","#4e526c","#5d566c","#3e6175","#453b64","#273b50"][r];
                c.stroke();
            }
            for (var j = 0; j < 24; j++) {
                var a = j * Math.PI / 12;
                c.beginPath();
                c.moveTo(Math.cos(a) * 35, Math.sin(a) * 35);
                c.lineTo(Math.cos(a) * 310, Math.sin(a) * 310);
                c.strokeStyle = j % 3 ? "rgba(106,114,164,0.08)" : "rgba(198,142,132,0.18)";
                c.lineWidth = j % 3 ? 0.6 : 1.1;
                c.stroke();
            }
            c.restore();

            for (var y = 0; y < h; y += 44) {
                c.fillStyle = "rgba(221,219,240,0.017)";
                c.fillRect(0, y, w, 1);
            }
            for (var x = 0; x < w; x += 44) {
                c.fillStyle = "rgba(221,219,240,0.017)";
                c.fillRect(x, 0, 1, h);
            }

            c.fillStyle = "rgba(208,186,209,0.26)";
            c.font = "bold 11px monospace";
            c.textBaseline = "middle";
            c.fillText("力  /  BLACK PRISM", 24, h - 27);
            c.fillStyle = "rgba(190,183,209,0.25)";
            c.textAlign = "right";
            c.fillText("ARCHIVE COMPLETE / 032", w - 24, h - 27);
        }
    }

    Item {
        id: lightWheel
        width: Math.min(prism.width, prism.height) * 0.9
        height: width
        anchors.centerIn: parent
        anchors.horizontalCenterOffset: prism.width * 0.06
        opacity: 0.45

        rotation: prism.moving ? prism.phase * 360 : 28

        Repeater {
            model: 8
            delegate: Rectangle {
                required property int index
                width: lightWheel.width * 0.052
                height: lightWheel.height * 0.48
                x: lightWheel.width / 2 - width / 2
                y: lightWheel.height / 2 - height
                radius: width / 2
                transformOrigin: Item.Bottom
                rotation: index * 45
                gradient: Gradient {
                    GradientStop { position: 0; color: "#04070c" }
                    GradientStop { position: 0.58; color: index % 3 === 0 ? "#80516d" : index % 3 === 1 ? "#4f718b" : "#8b665e" }
                    GradientStop { position: 1; color: "#171724" }
                }
            }
        }
    }

    Rectangle {
        width: Math.max(46, Math.min(prism.width, prism.height) * 0.25)
        height: width
        anchors.centerIn: lightWheel
        radius: width / 2
        color: "#080910"
        border.width: 2
        border.color: "#7a708d"

        Rectangle {
            anchors.centerIn: parent
            width: parent.width * 0.66
            height: width
            radius: width / 2
            color: "#10101a"
            border.width: 1
            border.color: "#bb8079"
        }
    }

    Rectangle {
        width: prism.width * 0.07
        height: prism.height * 1.7
        x: -width + prism.phase * (prism.width + width * 4)
        y: -prism.height * 0.27
        rotation: 22
        opacity: prism.moving ? 0.14 : 0
        gradient: Gradient {
            GradientStop { position: 0; color: "transparent" }
            GradientStop { position: 0.48; color: "#b2a4df" }
            GradientStop { position: 0.52; color: "#e2a5a9" }
            GradientStop { position: 1; color: "transparent" }
        }
    }

    Rectangle {
        anchors.fill: parent
        color: "#080a11"
        opacity: 0.22
    }
}

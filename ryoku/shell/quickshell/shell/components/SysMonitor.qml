pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons

// The rail system monitor card: CPU, memory and (when a sensor is found)
// temperature as eased ring gauges, a CPU-history sparkline that repaints only
// on each poll, and an uptime + memory readout. Claims Sysinfo and Session only
// while shown, so polling stops when the card closes.
Column {
    id: root

    property real s: 1
    property bool active: false

    spacing: 12 * root.s

    // one poll owner while shown; also keeps Session's uptime tick fresh.
    property bool _watching: false
    function _watch(on) {
        Sysinfo.setActive(root, on);
        if (on !== root._watching) {
            Session.watchers += on ? 1 : -1;
            root._watching = on;
        }
    }
    onActiveChanged: root._watch(root.active)
    Component.onCompleted: root._watch(root.active)
    Component.onDestruction: root._watch(false)

    // ── ring gauges ──────────────────────────────────────────────────────────
    Row {
        anchors.horizontalCenter: parent.horizontalCenter
        spacing: 22 * root.s

        SysRing {
            s: root.s
            label: "CPU"
            value: Sysinfo.cpu
            readout: Math.round(Sysinfo.cpu * 100) + "%"
            ringColor: Theme.primary
        }
        SysRing {
            s: root.s
            label: I18n.tr("MEMORY")
            value: Sysinfo.mem
            readout: Math.round(Sysinfo.mem * 100) + "%"
            ringColor: Theme.tertiary
        }
        SysRing {
            visible: Sysinfo.hasTemp
            s: root.s
            label: I18n.tr("TEMP")
            value: Math.min(1, Sysinfo.tempC / 100)
            readout: Math.round(Sysinfo.tempC) + "\u00b0"
            ringColor: Theme.vermLit
        }
    }

    // ── CPU history sparkline ─────────────────────────────────────────────────
    Item {
        width: parent.width
        height: 32 * root.s

        Canvas {
            id: spark
            anchors.fill: parent
            onPaint: {
                var ctx = getContext("2d");
                ctx.reset();
                var h = Sysinfo.cpuHistory;
                var n = h.length;
                if (n < 2)
                    return;
                var w = width, hh = height;
                function px(i) { return i / (n - 1) * w; }
                function py(v) { return hh - Math.max(0, Math.min(1, v)) * (hh - 2) - 1; }
                ctx.beginPath();
                ctx.moveTo(0, hh);
                for (var i = 0; i < n; i++)
                    ctx.lineTo(px(i), py(h[i]));
                ctx.lineTo(w, hh);
                ctx.closePath();
                ctx.fillStyle = Qt.alpha(Theme.primary, 0.16);
                ctx.fill();
                ctx.beginPath();
                for (var j = 0; j < n; j++) {
                    if (j === 0)
                        ctx.moveTo(px(j), py(h[j]));
                    else
                        ctx.lineTo(px(j), py(h[j]));
                }
                ctx.lineWidth = 1.6 * root.s;
                ctx.strokeStyle = Theme.primary;
                ctx.lineJoin = "round";
                ctx.lineCap = "round";
                ctx.stroke();
            }
            Connections {
                target: Sysinfo
                function onCpuHistoryChanged() { spark.requestPaint(); }
            }
        }
    }

    // ── uptime + memory detail ────────────────────────────────────────────────
    Item {
        width: parent.width
        height: upText.implicitHeight

        Text {
            id: upText
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("UP %1").arg(Session.uptimeText)
            color: Theme.onSurfaceVariant
            font.family: Theme.mono
            font.pixelSize: 9.5 * root.s
            font.capitalization: Font.AllUppercase
            font.letterSpacing: 0.5 * root.s
        }
        Text {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            visible: Sysinfo.memTotalGiB > 0
            text: Sysinfo.memUsedGiB.toFixed(1) + " / " + Sysinfo.memTotalGiB.toFixed(1) + " GiB"
            color: Theme.onSurfaceVariant
            font.family: Theme.mono
            font.pixelSize: 9.5 * root.s
        }
    }
}

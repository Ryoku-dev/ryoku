import QtQuick
import QtQuick.Layouts
import "../"

ColumnLayout {
    id: root

    property string agentId: ""
    property var usage: ({ minutes: 0, pct: 0, resetTs: 0 })
    property bool general: true
    property bool limited: false
    property real nowSec: 0

    readonly property var pace: AgentsFormat.pace(agentId, usage, general, nowSec)
    readonly property color barColor: AgentsFormat.levelColor(usage.pct, limited)

    spacing: Scaler.s(5)

    RowLayout {
        Layout.fillWidth: true

        Text {
            text: AgentsFormat.windowLabel(root.usage)
            font.family: ThemeBackend.fontFamily
            font.pixelSize: Scaler.s(13)
            font.weight: Font.Bold
            color: ThemeBackend.subtext1
        }

        Item { Layout.fillWidth: true }

        Text {
            text: root.usage.pct + "%"
            font.family: ThemeBackend.fontFamily
            font.pixelSize: Scaler.s(18)
            font.weight: Font.Black
            color: root.barColor
        }
    }

    Rectangle {
        Layout.fillWidth: true
        Layout.preferredHeight: Scaler.s(10)
        radius: height / 2
        color: Qt.alpha(ThemeBackend.surface2, 0.6)

        Rectangle {
            width: parent.width * Math.min(100, root.usage.pct) / 100
            height: parent.height
            radius: parent.radius
            color: root.barColor
            Behavior on width { NumberAnimation { duration: 400; easing.type: Easing.OutCubic } }
        }

        Rectangle {
            visible: !!root.pace
            x: parent.width * Math.min(100, root.pace ? root.pace.expected : 0) / 100 - width / 2
            y: -Scaler.s(3)
            width: Scaler.s(2)
            height: parent.height + Scaler.s(6)
            radius: width / 2
            color: ThemeBackend.text
            opacity: 0.7
        }
    }

    Text {
        Layout.fillWidth: true
        text: {
            let parts = [];
            const left = AgentsFormat.countdown(root.usage.resetTs, root.nowSec);
            if (left)
                parts.push(AgentsFormat.tr("resets", "Resets in {in} · {at}",
                    { in: left, at: AgentsFormat.clock(root.usage.resetTs, root.nowSec) }));
            const p = root.pace;
            if (p) {
                if (p.delta > 5) parts.push(AgentsFormat.tr("ahead", "{n}% ahead of pace", { n: p.delta }));
                else if (p.delta < -5) parts.push(AgentsFormat.tr("under", "{n}% under pace", { n: -p.delta }));
                else parts.push(AgentsFormat.tr("onPace", "On pace"));
                if (p.exhaustTs > 0)
                    parts.push(AgentsFormat.tr("runsOut", "runs out ~{at}",
                        { at: AgentsFormat.clock(p.exhaustTs, root.nowSec) }));
            }
            return parts.join("  ·  ");
        }
        visible: text !== ""
        wrapMode: Text.WordWrap
        font.family: ThemeBackend.fontFamily
        font.pixelSize: Scaler.s(11)
        color: root.pace && root.pace.delta > 5 ? ThemeBackend.peach : ThemeBackend.subtext0
    }
}

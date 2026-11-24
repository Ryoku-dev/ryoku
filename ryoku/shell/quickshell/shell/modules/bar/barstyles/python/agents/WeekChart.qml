import QtQuick
import QtQuick.Layouts
import "../"

ColumnLayout {
    id: root

    property var recent: []
    property real nowSec: 0

    readonly property var days: AgentsFormat.week(recent, nowSec)
    readonly property real peak: Math.max.apply(null, days.map(d => d.tokens))
    readonly property real total: days.reduce((sum, d) => sum + d.tokens, 0)

    visible: total > 0
    spacing: Scaler.s(6)

    RowLayout {
        Layout.fillWidth: true

        Text {
            text: AgentsFormat.tr("thisWeek", "This week")
            font.family: ThemeBackend.fontFamily
            font.pixelSize: Scaler.s(12)
            font.weight: Font.Bold
            font.capitalization: Font.AllUppercase
            color: ThemeBackend.overlay1
        }

        Item { Layout.fillWidth: true }

        Text {
            text: AgentsFormat.tr("weekTotal", "{n} tokens", { n: AgentsFormat.tokens(root.total) })
            font.family: ThemeBackend.fontFamily
            font.pixelSize: Scaler.s(12)
            font.weight: Font.Bold
            color: ThemeBackend.subtext1
        }
    }

    RowLayout {
        Layout.fillWidth: true
        Layout.preferredHeight: Scaler.s(58)
        spacing: Scaler.s(6)

        Repeater {
            model: root.days

            ColumnLayout {
                id: day
                required property var modelData
                Layout.fillWidth: true
                Layout.fillHeight: true
                spacing: Scaler.s(3)

                Item {
                    Layout.fillWidth: true
                    Layout.fillHeight: true

                    Rectangle {
                        anchors.bottom: parent.bottom
                        anchors.horizontalCenter: parent.horizontalCenter
                        width: Math.min(parent.width, Scaler.s(22))
                        height: day.modelData.tokens > 0
                            ? Math.max(Scaler.s(3), parent.height * day.modelData.tokens / root.peak)
                            : Scaler.s(2)
                        radius: Scaler.s(3)
                        color: day.modelData.isToday ? ThemeBackend.peach
                            : (day.modelData.tokens > 0 ? ThemeBackend.mauve : ThemeBackend.surface2)
                        opacity: day.modelData.future ? 0.3 : (day.modelData.tokens > 0 ? 0.9 : 0.6)
                    }
                }

                Text {
                    Layout.alignment: Qt.AlignHCenter
                    text: day.modelData.label
                    font.family: ThemeBackend.fontFamily
                    font.pixelSize: Scaler.s(10)
                    font.weight: day.modelData.isToday ? Font.Black : Font.Normal
                    color: day.modelData.isToday ? ThemeBackend.peach : ThemeBackend.subtext0
                    opacity: day.modelData.future ? 0.5 : 1.0
                }
            }
        }
    }
}

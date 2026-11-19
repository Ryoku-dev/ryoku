pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Lithic
import Ryoku.Ui.Singletons
import "../../../ask/look"
Flickable {
    id: root
    required property real s
    required property var store
    contentWidth: width; contentHeight: content.height + 60 * s; clip: true
    ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }
    Column {
        id: content
        x: 30 * root.s; y: 28 * root.s; width: root.width - 60 * root.s
        spacing: 24 * root.s
        Text { text: I18n.tr("Practice, measured"); color: Lithic.fg; font.family: Look.sans; font.pixelSize: 20 * root.s }
        Row {
            spacing: 46 * root.s
            Repeater {
                model: [{label: I18n.tr("Completed"), value: root.store.testsCompleted}, {label: I18n.tr("Time typing"), value: I18n.tr("%1 min").arg(Math.round(root.store.secondsTyping / 60))}, {label: I18n.tr("Average speed"), value: I18n.tr("%1 wpm").arg(root.store.results.length ? Math.round(root.store.results.reduce((a,r) => a + r.wpm, 0) / root.store.results.length) : 0)}, {label: I18n.tr("Completion"), value: (root.store.testsStarted ? Math.round(root.store.testsCompleted / root.store.testsStarted * 100) : 0) + "%"}]
                delegate: Column { required property var modelData; spacing: 8 * root.s; Text { text: modelData.label; color: Lithic.fg2; font.family: Look.sans; font.pixelSize: 12 * root.s } Text { text: modelData.value; color: Lithic.fg; font.family: Look.figures; font.pixelSize: 26 * root.s } }
            }
        }
        Text { text: I18n.tr("Speed · last 40 tests"); color: Lithic.fg2; font.family: Look.sans; font.pixelSize: 13 * root.s }
        TypingGraph { width: parent.width; height: 130 * root.s; s: root.s; values: root.store.results.slice(0, 40).reverse().map(r => r.wpm) }
        Text { text: I18n.tr("Practice · last 12 weeks"); color: Lithic.fg2; font.family: Look.sans; font.pixelSize: 13 * root.s }
        Row {
            spacing: 4 * root.s
            Repeater {
                model: 84
                delegate: Rectangle {
                    required property int index
                    readonly property string day: Qt.formatDate(new Date(Date.now() - (83 - index) * 86400000), "yyyy-MM-dd")
                    readonly property int count: Number(root.store.activity.find(a => a.d === day)?.n || 0)
                    width: Math.max(3 * root.s, (content.width - 83 * 4 * root.s) / 84); height: 36 * root.s
                    radius: Look.lithic ? 0 : 2 * root.s
                    color: count ? Lithic.mix(Look.well, Lithic.jade, Math.min(1, 0.25 + count * 0.12)) : Look.well
                    HoverHandler { id: hover }
                    ToolTip.visible: hover.hovered
                    ToolTip.text: I18n.tr("%1 · %2 tests").arg(day).arg(count)
                }
            }
        }
        Text { text: I18n.tr("Personal bests"); color: Lithic.fg; font.family: Look.sans; font.pixelSize: 16 * root.s }
        Repeater {
            model: root.store.personalBests.slice().sort((a,b) => b.wpm - a.wpm)
            delegate: Row {
                required property var modelData
                width: content.width; height: 30 * root.s
                Text { width: content.width * 0.5; text: I18n.tr(parent.modelData.mode) + " " + parent.modelData.modeValue + " · " + parent.modelData.language; color: Lithic.fg2; font.family: Look.sans; font.pixelSize: 12 * root.s }
                Text { text: I18n.tr("%1 wpm · %2% accuracy").arg(parent.modelData.wpm).arg(parent.modelData.accuracy); color: Lithic.fg; font.family: Look.figures; font.pixelSize: 12 * root.s }
            }
        }
        Text { visible: !root.store.personalBests.length; text: I18n.tr("Your best tests will appear here"); color: Lithic.fg3; font.family: Look.sans; font.pixelSize: 13 * root.s }
    }
}

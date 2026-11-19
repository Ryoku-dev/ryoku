pragma ComponentBehavior: Bound
import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../../ask/look"
Item {
    id: root
    required property real s
    required property var engine
    property bool personalBest: false
    signal restart(bool reuseSeed)
    Column {
        anchors.centerIn: parent
        width: Math.min(parent.width - 64 * root.s, 860 * root.s)
        spacing: 28 * root.s
        Text { text: root.personalBest ? I18n.tr("A new personal best") : I18n.tr("Test complete"); color: root.personalBest ? Lithic.jade : Lithic.fg; font.family: Look.sans; font.pixelSize: 22 * root.s }
        Row {
            spacing: 48 * root.s
            Repeater {
                model: [{label: I18n.tr("Words a minute"), value: Math.round(root.engine.wpm)}, {label: I18n.tr("Accuracy"), value: Math.round(root.engine.accuracy) + "%"}, {label: I18n.tr("Raw speed"), value: Math.round(root.engine.rawWpm)}, {label: I18n.tr("Consistency"), value: Math.round(root.engine.consistency()) + "%"}]
                delegate: Column { required property var modelData; spacing: 6 * root.s; Text { text: modelData.label; color: Lithic.fg2; font.family: Look.sans; font.pixelSize: 12 * root.s } Text { text: modelData.value; color: Lithic.fg; font.family: Look.figures; font.pixelSize: 38 * root.s } }
            }
        }
        TypingGraph { width: parent.width; height: 150 * root.s; s: root.s; values: root.engine.samples.map(v => v.wpm) }
        Text { text: I18n.tr("%1 seconds · %2 correct · %3 incorrect · %4 extra · %5 missed").arg(Math.round(root.engine.elapsedSeconds)).arg(root.engine.characterBreakdown.correct).arg(root.engine.characterBreakdown.incorrect).arg(root.engine.characterBreakdown.extra).arg(root.engine.characterBreakdown.missed); color: Lithic.fg2; font.family: Look.sans; font.pixelSize: 12 * root.s }
        Row { spacing: 10 * root.s; TypingButton { s: root.s; text: I18n.tr("Next test"); chosen: true; onClicked: root.restart(false) } TypingButton { s: root.s; text: I18n.tr("Repeat these words"); onClicked: root.restart(true) } Text { anchors.verticalCenter: parent.verticalCenter; text: I18n.tr("Esc returns to the test"); color: Lithic.fg3; font.family: Look.sans; font.pixelSize: 12 * root.s } }
    }
}

pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Lithic
import Ryoku.Ui.Singletons
import "../../../ask/look"
Item {
    id: root
    required property real s
    required property var store
    property string query: ""
    readonly property var results: store.results.filter(r => [r.language, r.mode, String(Math.round(r.wpm)), Qt.formatDateTime(new Date(r.timestamp), "dd MMM yyyy")].join(" ").toLowerCase().includes(query.trim().toLowerCase()))
    function scroll(delta) { list.contentY = Math.max(0, Math.min(list.contentHeight - list.height, list.contentY + delta)); }
    Item {
        anchors.fill: parent; anchors.margins: 30 * root.s
        Text { id: head; text: I18n.tr("Your pace over time"); color: Lithic.fg; font.family: Look.sans; font.pixelSize: 20 * root.s }
        Text { anchors.right: parent.right; anchors.verticalCenter: head.verticalCenter; text: I18n.tr("%1 tests").arg(root.results.length); color: Lithic.fg2; font.family: Look.figures; font.pixelSize: 12 * root.s }
        TypingGraph { id: graph; y: 46 * root.s; width: parent.width; height: Math.min(130 * root.s, parent.height * 0.24); s: root.s; values: root.results.slice(0, 40).reverse().map(v => v.wpm) }
        Row {
            id: labels
            y: graph.y + graph.height + 12 * root.s
            width: parent.width; height: 24 * root.s
            Repeater { model: [I18n.tr("Date"), I18n.tr("Test"), I18n.tr("WPM"), I18n.tr("Accuracy"), I18n.tr("Consistency")]; delegate: Text { required property string modelData; required property int index; width: labels.width * [0.30, 0.22, 0.20, 0.14, 0.14][index]; text: modelData; color: Lithic.fg3; font.family: Look.sans; font.pixelSize: 11 * root.s } }
        }
        ListView {
            id: list
            anchors.top: labels.bottom; anchors.bottom: parent.bottom; width: parent.width
            clip: true; model: root.results
            ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }
            delegate: Item {
                id: row
                required property var modelData
                required property int index
                width: list.width; height: 36 * root.s
                Shape { anchors.fill: parent; fill: row.index % 2 === 0 ? Look.well : "transparent"; cut: Look.cutRow * root.s }
                Row {
                    anchors.fill: parent
                    Repeater {
                        model: [Qt.formatDateTime(new Date(row.modelData.timestamp), "dd MMM yyyy · hh:mm"), I18n.tr(row.modelData.mode) + " " + row.modelData.modeValue, String(Math.round(row.modelData.wpm)), row.modelData.accuracy + "%", row.modelData.consistency + "%"]
                        delegate: Text { required property string modelData; required property int index; width: row.width * [0.30, 0.22, 0.20, 0.14, 0.14][index]; height: row.height; verticalAlignment: Text.AlignVCenter; text: modelData; color: index === 2 ? Lithic.fg : Lithic.fg2; font.family: index > 1 ? Look.figures : Look.sans; font.pixelSize: 12 * root.s; elide: Text.ElideRight }
                    }
                }
            }
        }
        Text { anchors.centerIn: list; visible: !root.results.length; text: root.query ? I18n.tr("No matching tests") : I18n.tr("Finish a test to start your history"); color: Lithic.fg2; font.family: Look.sans; font.pixelSize: 14 * root.s }
    }
}

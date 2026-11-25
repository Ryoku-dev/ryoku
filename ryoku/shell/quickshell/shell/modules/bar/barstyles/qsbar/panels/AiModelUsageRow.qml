import QtQuick
import "../modules"
import Ryoku.Ui.Singletons

// One model's share of a provider's recent tokens in the AI usage panel:
// name and total, a meter against the busiest model, and the token breakdown.
Item {
    id: row
    required property var theme
    property string name: ""
    property string totalLabel: ""
    property string inputLabel: ""
    property string outputLabel: ""
    property string reasoningLabel: ""
    property string cacheReadLabel: ""
    property string cacheWriteLabel: ""
    property string todayLabel: ""
    property int pct: 0

    height: 42

    UiText {
        id: modelName
        anchors.left: parent.left; anchors.top: parent.top
        width: parent.width * 0.68
        text: row.name
        elide: Text.ElideRight
        color: row.theme.ink
        font.family: row.theme.mono; font.pixelSize: 10; font.weight: Font.Medium
    }
    UiText {
        anchors.right: parent.right; anchors.top: parent.top
        text: row.totalLabel
        color: row.theme.seal
        font.family: row.theme.mono; font.pixelSize: 10; font.weight: Font.Medium
    }
    Rectangle {
        id: modelMeter
        anchors.left: parent.left; anchors.right: parent.right
        anchors.top: modelName.bottom; anchors.topMargin: 5
        height: 6; radius: 3
        color: Qt.rgba(row.theme.seal.r, row.theme.seal.g, row.theme.seal.b, 0.14)
        Rectangle {
            width: parent.width * Math.max(0, Math.min(100, row.pct)) / 100
            height: parent.height; radius: 3
            color: row.theme.seal
            Behavior on width { NumberAnimation { duration: 300 } }
        }
    }
    UiText {
        anchors.left: parent.left; anchors.right: parent.right
        anchors.top: modelMeter.bottom; anchors.topMargin: 4
        text: I18n.tr("I %1  O %2").arg(row.inputLabel).arg(row.outputLabel)
            + (row.reasoningLabel !== "0" ? I18n.tr("  R %1").arg(row.reasoningLabel) : "")
            + (row.cacheReadLabel !== "0" ? I18n.tr("  CR %1").arg(row.cacheReadLabel) : "")
            + (row.cacheWriteLabel !== "0" ? I18n.tr("  CW %1").arg(row.cacheWriteLabel) : "")
            + (row.todayLabel !== "0" ? I18n.tr("  today %1").arg(row.todayLabel) : "")
        elide: Text.ElideRight
        color: row.theme.sumiHi
        font.family: row.theme.mono; font.pixelSize: 9
    }
}

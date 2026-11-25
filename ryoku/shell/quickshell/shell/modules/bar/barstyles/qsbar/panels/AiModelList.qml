import QtQuick
import "../modules"
import Ryoku.Ui.Singletons

// The per-model breakdown under a provider card in the AI usage panel
// (OpenCode, Grok): a MODELS header and one AiModelUsageRow per model.
Column {
    id: list
    required property var theme
    property bool shown: false
    property var models: []

    visible: shown && models.length > 0
    spacing: 12

    Item {
        width: list.width; height: 16
        UiText {
            anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("MODELS")
            color: list.theme.sumiHi
            font.family: list.theme.mono; font.pixelSize: 10; font.letterSpacing: 1
        }
        UiText {
            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("recent")
            color: list.theme.sumi
            font.family: list.theme.mono; font.pixelSize: 10
        }
    }
    Repeater {
        model: list.visible ? list.models : []
        AiModelUsageRow {
            required property var modelData
            width: list.width
            theme: list.theme
            name: modelData.name || ""
            totalLabel: modelData.totalLabel || ""
            inputLabel: modelData.inputLabel || "0"
            outputLabel: modelData.outputLabel || "0"
            reasoningLabel: modelData.reasoningLabel || "0"
            cacheReadLabel: modelData.cacheReadLabel || "0"
            cacheWriteLabel: modelData.cacheWriteLabel || "0"
            todayLabel: modelData.todayLabel || "0"
            pct: parseInt(modelData.pct) || 0
        }
    }
}

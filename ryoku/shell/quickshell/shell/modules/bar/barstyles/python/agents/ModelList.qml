import QtQuick
import QtQuick.Layouts
import "../"

ColumnLayout {
    id: root

    property var models: []

    visible: models.length > 0
    spacing: Scaler.s(4)

    Text {
        text: AgentsFormat.tr("models", "Models")
        font.family: ThemeBackend.fontFamily
        font.pixelSize: Scaler.s(12)
        font.weight: Font.Bold
        font.capitalization: Font.AllUppercase
        color: ThemeBackend.overlay1
    }

    Repeater {
        model: root.models

        RowLayout {
            id: row
            required property var modelData
            readonly property string today: String(modelData.todayLabel || "")
            Layout.fillWidth: true
            spacing: Scaler.s(8)

            Text {
                Layout.fillWidth: true
                text: String(row.modelData.name || "")
                elide: Text.ElideRight
                font.family: ThemeBackend.fontFamily
                font.pixelSize: Scaler.s(12)
                color: ThemeBackend.text
            }

            Text {
                visible: row.today !== "" && row.today !== "0"
                text: AgentsFormat.tr("modelToday", "today {n}", { n: row.today })
                font.family: ThemeBackend.fontFamily
                font.pixelSize: Scaler.s(11)
                color: ThemeBackend.subtext0
            }

            Text {
                text: String(row.modelData.totalLabel || "")
                font.family: ThemeBackend.fontFamily
                font.pixelSize: Scaler.s(12)
                font.weight: Font.Bold
                color: ThemeBackend.subtext1
            }
        }
    }
}

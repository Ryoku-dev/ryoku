import QtQuick
import QtQuick.Layouts
import stage.modules.common
import stage.modules.common.widgets
import stage.services

ColumnLayout {
    id: root

    property string currentValue: "ryoku"
    signal selected(string value)

    Layout.fillWidth: true
    spacing: Appearance.sizes.space2

    EditPanelSectionLabel { text: Translation.tr("Design") }

    GridLayout {
        Layout.fillWidth: true
        columns: 2
        columnSpacing: Appearance.sizes.space2
        rowSpacing: Appearance.sizes.space2

        Repeater {
            model: [
                { value: "ryoku", title: Translation.tr("Ryoku"), detail: Translation.tr("Paper islands"), glyph: "dock" },
                { value: "python", title: Translation.tr("Python"), detail: Translation.tr("Flowing glass"), glyph: "blur_on" },
                { value: "shima", title: Translation.tr("Shima"), detail: Translation.tr("Notched capsule"), glyph: "line_curve" },
                { value: "none", title: Translation.tr("None"), detail: Translation.tr("No dock surface"), glyph: "block" }
            ]

            delegate: Rectangle {
                id: card
                required property var modelData
                required property int index

                Layout.fillWidth: true
                implicitHeight: 108
                radius: Appearance.rounding.small
                color: root.currentValue === modelData.value
                    ? Appearance.colors.colSecondaryContainer : Appearance.colors.colLayer1
                border.width: 1
                border.color: root.currentValue === modelData.value
                    ? Appearance.colors.colSecondary : Appearance.colors.colOutlineVariant

                ColumnLayout {
                    anchors.fill: parent
                    anchors.margins: Appearance.sizes.space3
                    spacing: Appearance.sizes.space2

                    Item {
                        Layout.fillWidth: true
                        implicitHeight: 38

                        Rectangle {
                            anchors.horizontalCenter: parent.horizontalCenter
                            anchors.bottom: parent.bottom
                            width: card.modelData.value === "none" ? 42 : card.modelData.value === "python" ? 94 : 82
                            height: card.modelData.value === "none" ? 2 : 30
                            radius: card.modelData.value === "ryoku" ? 7 : height / 2
                            color: card.modelData.value === "none"
                                ? Appearance.colors.colOutlineVariant : Appearance.colors.colSurfaceContainerHighest

                            Row {
                                anchors.centerIn: parent
                                spacing: card.modelData.value === "ryoku" ? 7 : 4
                                visible: card.modelData.value !== "none"
                                Repeater {
                                    model: 4
                                    Rectangle {
                                        width: 12
                                        height: 12
                                        radius: card.modelData.value === "shima" ? 6 : 4
                                        color: index === 1 ? Appearance.colors.colSecondary : Appearance.colors.colOnSurface
                                    }
                                }
                            }
                        }
                    }

                    RowLayout {
                        Layout.fillWidth: true
                        MaterialSymbol {
                            text: card.modelData.glyph
                            iconSize: 18
                            color: Appearance.colors.colOnSurface
                        }
                        StyledText {
                            Layout.fillWidth: true
                            text: card.modelData.title
                            color: Appearance.colors.colOnSurface
                            font.weight: Font.DemiBold
                        }
                        MaterialSymbol {
                            visible: root.currentValue === card.modelData.value
                            text: "check_circle"
                            iconSize: 18
                            color: Appearance.colors.colSecondary
                        }
                    }
                    StyledText {
                        Layout.fillWidth: true
                        text: card.modelData.detail
                        color: Appearance.colors.colOnSurfaceVariant
                        font.pixelSize: Appearance.font.pixelSize.small
                    }
                }

                MouseArea {
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: root.selected(card.modelData.value)
                }
            }
        }
    }
}

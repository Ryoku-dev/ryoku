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

            // The selected card is the inverted plate, so EVERYTHING on it -
            // glyph, title, detail - has to read through the inverse pair. A
            // plain colOnSurface is the ink for the paper cards, which is the
            // same family as the bone plate on a light palette and the opposite
            // family on a dark one: the chosen design then painted itself
            // invisible, which is the "text and colour are bad on some
            // wallpapers" report.
            delegate: Rectangle {
                id: card
                required property var modelData
                required property int index

                readonly property bool chosen: root.currentValue === modelData.value
                readonly property color plate: chosen
                    ? Appearance.colors.colSecondaryContainer : Appearance.colors.colLayer1
                readonly property color ink: chosen
                    ? Appearance.colors.colOnSecondaryContainer : Appearance.colors.colOnSurface
                readonly property color inkDim: chosen
                    ? Appearance.colors.colOnSecondaryContainer : Appearance.colors.colOnSurfaceVariant

                Layout.fillWidth: true
                implicitHeight: 108
                radius: Appearance.rounding.small
                color: card.plate
                border.width: 1
                border.color: chosen
                    ? Appearance.colors.colSecondary : Appearance.colors.colOutlineVariant
                ColumnLayout {
                    anchors.fill: parent
                    anchors.margins: Appearance.sizes.space3
                    spacing: Appearance.sizes.space2

                    Item {
                        Layout.fillWidth: true
                        implicitHeight: 38

                        // The dock's own preview plate: an ink wash on paper, the
                        // inverse wash on the selected plate, so the miniature
                        // reads on both.
                        Rectangle {
                            anchors.horizontalCenter: parent.horizontalCenter
                            anchors.bottom: parent.bottom
                            width: card.modelData.value === "none" ? 42 : card.modelData.value === "python" ? 94 : 82
                            height: card.modelData.value === "none" ? 2 : 30
                            radius: card.modelData.value === "ryoku" ? 7 : height / 2
                            color: card.modelData.value === "none"
                                ? Appearance.colors.colOutlineVariant
                                : card.chosen
                                    ? Appearance.withAlpha(card.ink, 0.14)
                                    : Appearance.colors.colSurfaceContainerHighest

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
                                        color: index === 1
                                            ? (card.chosen ? Appearance.colors.colLayer0
                                                : Appearance.colors.colSecondary)
                                            : card.ink
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
                            color: card.ink
                        }
                        StyledText {
                            Layout.fillWidth: true
                            text: card.modelData.title
                            color: card.ink
                            font.weight: Font.DemiBold
                        }
                        MaterialSymbol {
                            visible: card.chosen
                            text: "check_circle"
                            iconSize: 18
                            color: card.chosen ? card.ink : Appearance.colors.colSecondary
                        }
                    }
                    StyledText {
                        Layout.fillWidth: true
                        text: card.modelData.detail
                        color: card.inkDim
                        opacity: card.chosen ? 0.78 : 1
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

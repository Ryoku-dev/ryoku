pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Ryoku.Ui.Singletons
import inir.modules.cheatsheet
import inir.modules.common
import inir.modules.iris.components
import inir.modules.iris.style
import inir.services

Item {
    id: root

    ColumnLayout {
        anchors.fill: parent
        spacing: Math.round(10 * IrisStyle.density)

        Rectangle {
            Layout.fillWidth: true
            implicitHeight: heading.implicitHeight + Math.round(24 * IrisStyle.density)
            radius: IrisStyle.radiusTile
            color: IrisStyle.readingCard
            RowLayout {
                id: heading
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.margins: Math.round(12 * IrisStyle.density)
                spacing: Math.round(10 * IrisStyle.density)
                IrisSquircle {
                    width: Math.round(36 * IrisStyle.density)
                    height: width
                    tint: IrisStyle.identity.indigo
                    glyph: "keyboard"
                }
                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 1
                    IrisText {
                        text: Translation.tr("Keyboard shortcuts")
                        font.pixelSize: IrisStyle.typeTitleLarge
                        font.weight: IrisStyle.weight(Font.Bold)
                    }
                    IrisText {
                        Layout.fillWidth: true
                        text: Translation.tr("This is the live Ryoku keybind catalogue. Edit or rebind shortcuts in Ryoku Hub.")
                        color: IrisStyle.muted
                        font.pixelSize: IrisStyle.typeMeta
                        wrapMode: Text.WordWrap
                    }
                }
                IrisButton {
                    text: Translation.tr("Open Ryoku Hub")
                    emphasized: true
                    onClicked: Spawn.run(["ryoku-shell", "hub", "open", "keybinds"])
                }
            }
        }

        Flickable {
            Layout.fillWidth: true
            Layout.fillHeight: true
            contentHeight: shortcutList.implicitHeight
            boundsBehavior: Flickable.StopAtBounds
            clip: true
            ScrollBar.vertical: IrisScrollBar {}

            ColumnLayout {
                id: shortcutList
                width: parent.width
                spacing: Math.round(10 * IrisStyle.density)

                IrisText {
                    Layout.alignment: Qt.AlignHCenter
                    visible: !KeybindCatalogue.loaded
                    text: Translation.tr("Loading shortcuts...")
                    color: IrisStyle.muted
                }
                IrisText {
                    Layout.alignment: Qt.AlignHCenter
                    visible: KeybindCatalogue.loaded && KeybindCatalogue.categories.length === 0
                    text: Translation.tr("No shortcuts are available")
                    color: IrisStyle.muted
                }

                Repeater {
                    model: KeybindCatalogue.categories
                    Rectangle {
                        id: category
                        required property var modelData
                        Layout.fillWidth: true
                        implicitHeight: categoryBody.implicitHeight + Math.round(14 * IrisStyle.density)
                        radius: IrisStyle.radiusTile
                        color: IrisStyle.readingCard
                        ColumnLayout {
                            id: categoryBody
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.top: parent.top
                            anchors.margins: Math.round(7 * IrisStyle.density)
                            spacing: 0
                            IrisText {
                                Layout.fillWidth: true
                                Layout.leftMargin: Math.round(9 * IrisStyle.density)
                                Layout.topMargin: Math.round(4 * IrisStyle.density)
                                Layout.bottomMargin: Math.round(6 * IrisStyle.density)
                                text: category.modelData.name
                                color: IrisStyle.label
                                font.weight: IrisStyle.weight(Font.DemiBold)
                            }
                            Repeater {
                                model: category.modelData.binds
                                CheatsheetKeybindRow {
                                    required property var modelData
                                    Layout.fillWidth: true
                                    keybindData: modelData
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

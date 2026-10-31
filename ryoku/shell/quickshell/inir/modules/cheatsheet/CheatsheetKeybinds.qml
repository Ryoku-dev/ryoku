pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import inir.modules.common
import inir.modules.common.widgets
import inir.modules.iris.components
import inir.modules.iris.style
import inir.services

Item {
    id: root

    property string query: ""
    readonly property var categories: {
        const terms = root.query.toLowerCase().split(/\s+/).filter(term => term.length > 0)
        if (terms.length === 0)
            return KeybindCatalogue.categories
        return KeybindCatalogue.categories.map(category => ({
            name: category.name,
            binds: category.binds.filter(bind => {
                const text = [category.name, bind?.label ?? bind?.desc ?? "", bind?.hint ?? "", KeybindCatalogue.keysOf(bind).join(" ")].join(" ").toLowerCase()
                return terms.every(term => text.includes(term))
            })
        })).filter(category => category.binds.length > 0)
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: Math.round(8 * IrisStyle.density)

        Rectangle {
            Layout.fillWidth: true
            implicitHeight: Math.round(38 * IrisStyle.density)
            radius: height / 2
            color: search.activeFocus ? IrisStyle.fill : IrisStyle.fillQuiet
            border.width: search.activeFocus ? 1 : 0
            border.color: IrisStyle.tintBorder(IrisStyle.accent)
            MaterialSymbol {
                anchors.left: parent.left
                anchors.leftMargin: Math.round(13 * IrisStyle.density)
                anchors.verticalCenter: parent.verticalCenter
                text: "search"
                iconSize: Math.round(18 * IrisStyle.density)
                color: IrisStyle.muted
            }
            TextInput {
                id: search
                anchors.left: parent.left
                anchors.leftMargin: Math.round(42 * IrisStyle.density)
                anchors.right: parent.right
                anchors.rightMargin: Math.round(14 * IrisStyle.density)
                anchors.verticalCenter: parent.verticalCenter
                color: IrisStyle.text
                selectionColor: IrisStyle.accentContainer
                font.family: IrisStyle.fontMain
                font.pixelSize: IrisStyle.typeLabel
                onTextChanged: root.query = text
                Keys.onEscapePressed: event => {
                    if (text.length > 0) {
                        text = ""
                        event.accepted = true
                    } else {
                        event.accepted = false
                    }
                }
                IrisText {
                    anchors.verticalCenter: parent.verticalCenter
                    visible: search.text.length === 0
                    text: Translation.tr("Search an action or a key")
                    color: IrisStyle.muted
                    font.pixelSize: search.font.pixelSize
                }
            }
        }

        Flickable {
            Layout.fillWidth: true
            Layout.fillHeight: true
            contentHeight: list.implicitHeight
            boundsBehavior: Flickable.StopAtBounds
            clip: true
            ScrollBar.vertical: IrisScrollBar {}

            ColumnLayout {
                id: list
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
                    visible: KeybindCatalogue.loaded && root.categories.length === 0
                    text: root.query.length > 0 ? Translation.tr("No shortcut matches this search") : Translation.tr("No shortcuts are available")
                    color: IrisStyle.muted
                }

                Repeater {
                    model: root.categories
                    Rectangle {
                        id: category
                        required property var modelData
                        Layout.fillWidth: true
                        implicitHeight: categoryColumn.implicitHeight + Math.round(14 * IrisStyle.density)
                        radius: IrisStyle.radiusTile
                        color: IrisStyle.fillQuiet
                        ColumnLayout {
                            id: categoryColumn
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.top: parent.top
                            anchors.margins: Math.round(7 * IrisStyle.density)
                            spacing: 0
                            IrisText {
                                Layout.fillWidth: true
                                Layout.leftMargin: Math.round(9 * IrisStyle.density)
                                Layout.rightMargin: Math.round(9 * IrisStyle.density)
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

import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: rail

    property var tabs: []
    property string current: ""
    property real reveal: 1

    signal selected(string id)

    implicitWidth: 282 * Theme.scale

    Rectangle {
        anchors.fill: parent
        color: Theme.withAlpha(Theme.surfaceVariant, 0.22)
        border.width: 1
        border.color: Theme.withAlpha(Theme.outline, 0.3)
    }

    Column {
        anchors.fill: parent
        anchors.margins: 8 * Theme.scale
        spacing: 5 * Theme.scale

        Row {
            width: parent.width
            height: 28 * Theme.scale
            spacing: 7 * Theme.scale

            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: "力"
                color: Theme.primary
                font.family: Theme.jp
                font.pixelSize: Theme.fs(18)
                renderType: Text.NativeRendering
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("SOURCES")
                color: Theme.withAlpha(Theme.surfaceText, 0.68)
                font.family: Theme.sans
                font.weight: Font.DemiBold
                font.pixelSize: Theme.fontTiny
                font.letterSpacing: 1.5
                renderType: Text.NativeRendering
            }
        }

        Rectangle {
            width: parent.width
            height: 1
            color: Theme.withAlpha(Theme.outline, 0.28)
        }

        Item {
            width: parent.width
            height: parent.height - y

            Flickable {
                id: sourceScroll
                anchors.fill: parent
                anchors.rightMargin: sourceColumn.implicitHeight > height ? 6 * Theme.scale : 0
                clip: true
                contentWidth: width
                contentHeight: sourceColumn.implicitHeight
                boundsBehavior: Flickable.StopAtBounds

                Column {
                    id: sourceColumn
                    width: sourceScroll.width
                    spacing: 1 * Theme.scale

                    Repeater {
                        model: rail.tabs
                        delegate: Rectangle {
                            id: entry
                            required property var modelData
                            required property int index
                            width: sourceColumn.width
                            height: 30 * Theme.scale
                            radius: Theme.radius
                            readonly property bool selected: rail.current === modelData.id
                            readonly property bool available: modelData.enabled === true
                            color: selected
                                ? Theme.surfaceText
                                : (tap.pressed
                                   ? Theme.withAlpha(Theme.surfaceText, 0.14)
                                   : (hover.hovered && available
                                      ? Theme.withAlpha(Theme.surfaceText, 0.07)
                                      : "transparent"))
                            opacity: rail.reveal

                            Behavior on color { ColorAnimation { duration: Theme.fast } }

                            Row {
                                anchors.fill: parent
                                anchors.leftMargin: 9 * Theme.scale
                                anchors.rightMargin: 8 * Theme.scale
                                spacing: 7 * Theme.scale

                                Text {
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: entry.selected ? "//" : (entry.index < 9 ? "0" : "") + String(entry.index + 1)
                                    color: entry.selected
                                        ? Theme.withAlpha(Theme.surface, 0.58)
                                        : Theme.withAlpha(Theme.surfaceText, entry.available ? 0.34 : 0.2)
                                    font.family: Theme.sans
                                    font.pixelSize: Theme.fontTiny
                                    renderType: Text.NativeRendering
                                }

                                Text {
                                    width: parent.width - 42 * Theme.scale
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: I18n.tr(entry.modelData.label)
                                    elide: Text.ElideRight
                                    color: entry.selected
                                        ? Theme.surface
                                        : Theme.withAlpha(Theme.surfaceText, entry.available ? 0.84 : 0.34)
                                    font.family: Theme.sans
                                    font.weight: Font.Medium
                                    font.pixelSize: Theme.fontBase
                                    renderType: Text.NativeRendering
                                }

                                Text {
                                    visible: !entry.available
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: "\uf023"
                                    color: entry.selected
                                        ? Theme.withAlpha(Theme.surface, 0.52)
                                        : Theme.withAlpha(Theme.surfaceText, 0.28)
                                    font.family: Theme.icon
                                    font.pixelSize: Theme.fs(11)
                                    renderType: Text.NativeRendering
                                }
                            }

                            HoverHandler {
                                id: hover
                                cursorShape: entry.available && !entry.selected ? Qt.PointingHandCursor : Qt.ArrowCursor
                            }
                            TapHandler {
                                id: tap
                                enabled: entry.available && !entry.selected
                                onTapped: rail.selected(entry.modelData.id)
                            }
                        }
                    }
                }
            }

            Rectangle {
                visible: sourceScroll.contentHeight > sourceScroll.height + 1
                anchors.top: parent.top
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                width: 2 * Theme.scale
                color: Theme.withAlpha(Theme.surfaceText, 0.05)

                Rectangle {
                    width: parent.width
                    height: Math.max(18 * Theme.scale,
                        parent.height * sourceScroll.height / Math.max(sourceScroll.contentHeight, 1))
                    y: (parent.height - height) * sourceScroll.visibleArea.yPosition
                    radius: width / 2
                    color: Theme.withAlpha(Theme.surfaceText, 0.42)
                }
            }
        }
    }
}

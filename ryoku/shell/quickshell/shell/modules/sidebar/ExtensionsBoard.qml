pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../ask/look"

Item {
    id: root

    required property real s
    required property var plugins
    required property bool active
    // extension cards that grow cap themselves by this
    property real viewportHeight: Math.round(380 * root.s)
    signal requestClose()

    readonly property real inset: Math.round(12 * root.s)

    implicitHeight: root.plugins.length > 0 ? cards.implicitHeight : emptyState.implicitHeight
    height: implicitHeight

    Column {
        id: cards
        visible: root.plugins.length > 0
        width: root.width
        spacing: Math.round(QsLook.gap * root.s)

        Repeater {
            model: root.plugins
            delegate: Plate {
                id: plate
                required property var modelData
                required property int index
                s: root.s
                width: cards.width
                height: host.height + root.inset * 2
                visible: host.implicitHeight > 0

                SidebarCardHost {
                    id: host
                    x: root.inset
                    y: root.inset
                    width: plate.width - root.inset * 2
                    s: root.s
                    cardId: plate.modelData.id
                    pluginEntry: plate.modelData.entry
                    cardIndex: plate.index
                    open: root.active
                    reveal: root.active ? 1 : 0
                    tabActive: root.active
                    viewportHeight: root.viewportHeight
                    onRequestClose: root.requestClose()
                }
            }
        }
    }

    Column {
        id: emptyState
        visible: root.plugins.length === 0
        width: root.width
        topPadding: Math.round(28 * root.s)
        bottomPadding: Math.round(28 * root.s)
        spacing: Math.round(8 * root.s)

        Symbol {
            anchors.horizontalCenter: parent.horizontalCenter
            s: root.s
            name: "extension"
            size: 32
            ink: Lithic.fg3
        }
        Text {
            width: parent.width
            horizontalAlignment: Text.AlignHCenter
            text: I18n.tr("No extension cards yet")
            color: Lithic.fg
            font.family: QsLook.sans
            font.pixelSize: Math.round(14 * root.s)
            font.weight: Font.Medium
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }
        Text {
            width: parent.width
            horizontalAlignment: Text.AlignHCenter
            text: I18n.tr("Add-ons that bring a card show it here.")
            color: Lithic.fg3
            font.family: QsLook.sans
            font.pixelSize: Math.round(12 * root.s)
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }
    }
}

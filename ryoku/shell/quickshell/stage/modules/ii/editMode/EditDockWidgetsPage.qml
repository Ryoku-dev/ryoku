import QtQuick
import QtQuick.Layouts
import Quickshell
import stage.modules.common
import stage.modules.common.widgets
import stage.services
import shell.services as ShellServices

StyledFlickable {
    id: root

    contentHeight: column.implicitHeight
    clip: true
    readonly property var pins: ShellServices.Dock.pinnedOrStarter()

    ColumnLayout {
        id: column
        width: root.width
        spacing: 4

        EditPanelSectionLabel { text: Translation.tr("Pinned app order") }

        EditPanelNotice {
            Layout.fillWidth: true
            symbol: "drag_indicator"
            text: Translation.tr("Use the arrows to set the order. Tap a row to remove that app.")
        }

        Repeater {
            model: root.pins
            delegate: EditPanelRow {
                required property string modelData
                required property int index
                readonly property var entry: DesktopEntries.heuristicLookup(modelData)

                Layout.fillWidth: true
                Layout.topMargin: index === 0 ? 6 : 0
                first: index === 0
                last: index === root.pins.length - 1
                iconSource: ShellServices.Dock.iconFor(modelData)
                symbol: iconSource === "" ? "apps" : ""
                title: entry?.name ?? modelData
                subtitle: Translation.tr("Tap to unpin")
                trailingKind: "stepper"
                valueText: String(index + 1)
                stepDownEnabled: index > 0
                stepUpEnabled: index < root.pins.length - 1
                onStepDown: ShellServices.Dock.movePinned(index, index - 1)
                onStepUp: ShellServices.Dock.movePinned(index, index + 1)
                onActivated: ShellServices.Dock.togglePin(modelData)
            }
        }

        EditPanelNotice {
            visible: root.pins.length === 0
            Layout.fillWidth: true
            symbol: "info"
            text: Translation.tr("Choose apps from the Dock catalogue to pin them here.")
        }

        Item {
            Layout.fillWidth: true
            implicitHeight: 8
        }
    }
}

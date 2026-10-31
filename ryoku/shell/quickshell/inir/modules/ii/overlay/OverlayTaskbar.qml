pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import inir
import inir.modules.common
import inir.modules.common.widgets
import inir.modules.iris.components
import inir.modules.iris.style
import inir.services

Rectangle {
    id: root

    implicitWidth: row.implicitWidth + Math.round(16 * IrisStyle.density)
    implicitHeight: Math.round(48 * IrisStyle.density)
    radius: height / 2
    color: IrisStyle.bodySurface
    border.width: 1
    border.color: IrisStyle.border
    opacity: GlobalStates.overlayOpen ? 1 : 0
    scale: GlobalStates.overlayOpen ? 1 : 0.94
    Behavior on opacity { NumberAnimation { duration: IrisStyle.emergeDuration; easing.type: IrisStyle.feedbackEasing } }
    Behavior on scale { NumberAnimation { duration: IrisStyle.emergeDuration; easing.type: IrisStyle.feedbackEasing } }

    RowLayout {
        id: row
        anchors.centerIn: parent
        spacing: Math.round(2 * IrisStyle.density)

        Repeater {
            model: OverlayContext.availableWidgets
            IrisIconButton {
                id: button
                required property var modelData
                readonly property bool selectedTool: Array.from(Persistent.states.overlay.open ?? []).includes(modelData.identifier)
                materialIcon: modelData.materialSymbol
                selected: selectedTool
                onClicked: {
                    const current = Array.from(Persistent.states.overlay.open ?? [])
                    Persistent.states.overlay.open = selectedTool
                        ? current.filter(identifier => identifier !== modelData.identifier)
                        : current.concat([modelData.identifier])
                }
                Accessible.name: Translation.tr(OverlayLook.titles[modelData.identifier] ?? modelData.identifier)
                StyledToolTip { text: Translation.tr(OverlayLook.titles[button.modelData.identifier] ?? button.modelData.identifier) }
            }
        }

        Rectangle {
            Layout.preferredWidth: 1
            Layout.preferredHeight: Math.round(24 * IrisStyle.density)
            color: IrisStyle.hairlineStrong
        }

        IrisIconButton {
            materialIcon: "close"
            onClicked: GlobalStates.overlayOpen = false
            Accessible.name: Translation.tr("Close tools")
        }
    }
}

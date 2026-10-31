pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Quickshell
import Quickshell.Wayland
import inir
import inir.modules.common
import inir.modules.common.widgets
import inir.modules.iris.components
import inir.modules.iris.settings
import inir.modules.iris.style
import inir.services

Scope {
    id: root

    property int currentPage: 0
    readonly property bool open: GlobalStates.cheatsheetOpen

    PanelWindow {
        id: window
        visible: root.open || card.opacity > 0
        screen: GlobalStates.focusedScreen
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        WlrLayershell.namespace: "quickshell:iris-cheatsheet"
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.keyboardFocus: root.open ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
        anchors { top: true; bottom: true; left: true; right: true }

        Shortcut {
            sequence: StandardKey.Cancel
            enabled: root.open
            onActivated: GlobalStates.cheatsheetOpen = false
        }

        Rectangle {
            anchors.fill: parent
            color: IrisStyle.scrim
            opacity: root.open ? 0.78 : 0
            Behavior on opacity { NumberAnimation { duration: IrisStyle.emergeDuration; easing.type: IrisStyle.feedbackEasing } }
        }

        MouseArea {
            anchors.fill: parent
            enabled: root.open
            onClicked: GlobalStates.cheatsheetOpen = false
        }

        Rectangle {
            id: card
            anchors.centerIn: parent
            width: Math.min(parent.width - 64, 1080 * IrisStyle.density)
            height: Math.min(parent.height - 64, 740 * IrisStyle.density)
            radius: IrisStyle.radiusPanel
            color: IrisStyle.bodySurface
            border.width: 1
            border.color: IrisStyle.border
            opacity: root.open ? 1 : 0
            scale: root.open ? 1 : 0.96
            Behavior on opacity { NumberAnimation { duration: IrisStyle.emergeDuration; easing.type: IrisStyle.feedbackEasing } }
            Behavior on scale { NumberAnimation { duration: IrisStyle.morphDuration; easing.type: Easing.BezierSpline; easing.bezierCurve: IrisStyle.morphCurve } }

            MouseArea { anchors.fill: parent; acceptedButtons: Qt.AllButtons }

            ColumnLayout {
                anchors.fill: parent
                anchors.margins: Math.round(14 * IrisStyle.density)
                spacing: Math.round(10 * IrisStyle.density)

                RowLayout {
                    Layout.fillWidth: true
                    spacing: Math.round(8 * IrisStyle.density)
                    IrisSquircle {
                        width: Math.round(36 * IrisStyle.density)
                        height: width
                        tint: IrisStyle.identity.indigo
                        glyph: "keyboard"
                    }
                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 0
                        IrisText { text: Translation.tr("Shortcuts"); font.pixelSize: IrisStyle.typeTitleLarge; font.weight: IrisStyle.weight(Font.Bold) }
                        IrisText { text: Translation.tr("The keys Ryoku is using now"); color: IrisStyle.muted; font.pixelSize: IrisStyle.typeMeta }
                    }
                    Repeater {
                        model: [
                            { label: Translation.tr("Keybinds"), icon: "keyboard" },
                            { label: Translation.tr("Elements"), icon: "experiment" }
                        ]
                        IrisButton {
                            required property var modelData
                            required property int index
                            text: modelData.label
                            emphasized: root.currentPage === index
                            quiet: root.currentPage !== index
                            onClicked: root.currentPage = index
                        }
                    }
                    IrisIconButton {
                        materialIcon: "close"
                        onClicked: GlobalStates.cheatsheetOpen = false
                        Accessible.name: Translation.tr("Close")
                    }
                }

                Rectangle {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    radius: IrisStyle.radiusTile
                    color: IrisStyle.readingCard
                    clip: true
                    Loader {
                        anchors.fill: parent
                        anchors.margins: Math.round(8 * IrisStyle.density)
                        sourceComponent: root.currentPage === 0 ? keybindsPage : elementsPage
                    }
                }
            }
        }
    }

    Component { id: keybindsPage; CheatsheetKeybinds {} }
    Component { id: elementsPage; CheatsheetPeriodicTable {} }
}

pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services

ColumnLayout {
    id: root

    required property PopoutState popouts

    property real scaleOffset: 1.0
    property real fontScale: 1.0
    property bool _isSidebarOpen: false

    width: Math.max(300 * scaleOffset, _isSidebarOpen ? (Visibilities.sidebarWidthFor(Tokens.sizes.sidebar.width) * scaleOffset) - Tokens.padding.extraLargeIncreased : 0)
    implicitWidth: Math.max(300 * scaleOffset, _isSidebarOpen ? (Visibilities.sidebarWidthFor(Tokens.sizes.sidebar.width) * scaleOffset) - Tokens.padding.extraLargeIncreased : 0)
    spacing: Tokens.spacing.medium * scaleOffset

    StyledText {
        Layout.topMargin: Tokens.padding.medium * root.scaleOffset
        Layout.leftMargin: Tokens.padding.small * root.scaleOffset
        text: qsTr("Notifications")
        font.weight: 500
        font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
    }

    StyledRect {
        Layout.fillWidth: true
        implicitWidth: cardLayout.implicitWidth + Tokens.padding.medium * 2 * root.scaleOffset
        implicitHeight: cardLayout.implicitHeight + Tokens.padding.medium * 2 * root.scaleOffset
        radius: Tokens.rounding.medium * root.scaleOffset
        color: Colours.tPalette.m3surfaceContainer
        clip: true

        ColumnLayout {
            id: cardLayout

            width: parent.width - Tokens.padding.medium * 2 * root.scaleOffset
            x: Tokens.padding.medium * root.scaleOffset
            y: Tokens.padding.medium * root.scaleOffset
            spacing: Tokens.spacing.medium * root.scaleOffset

            PopoutToggleRow {
                scaleOffset: root.scaleOffset
                fontScale: root.fontScale
                rightMargin: Tokens.padding.small * root.scaleOffset
                label: qsTr("Do not disturb")
                checked: Notifs.dnd
                toggle.onToggled: Notifs.dnd = checked
            }

            StyledText {
                text: Notifs.dnd ? qsTr("Notifications off") : qsTr("%1 unread").arg(Notifs.openCount)
                color: Colours.palette.m3onSurfaceVariant
                font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
            }
        }
    }

    IconTextButton {
        Layout.fillWidth: true
        inactiveColour: Colours.palette.m3primaryContainer
        inactiveOnColour: Colours.palette.m3onPrimaryContainer
        verticalPadding: Tokens.padding.small * root.scaleOffset
        text: qsTr("Clear all")
        icon: "clear_all"

        onClicked: Notifs.clear()
    }
}

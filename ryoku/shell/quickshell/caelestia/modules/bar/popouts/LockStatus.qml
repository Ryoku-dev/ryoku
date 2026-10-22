import QtQuick.Layouts
import Caelestia.Config
import caelestia.components
import caelestia.services

ColumnLayout {
    id: root

    required property PopoutState popouts

    property real scaleOffset: 1.0
    property real fontScale: 1.0
    property bool _isSidebarOpen: false

    implicitWidth: Math.max(300 * scaleOffset, _isSidebarOpen ? (Visibilities.sidebarWidthFor(Tokens.sizes.sidebar.width) * scaleOffset) - Tokens.padding.extraLargeIncreased : 0)
    spacing: Tokens.spacing.medium * scaleOffset

    StyledText {
        Layout.topMargin: Tokens.padding.medium * root.scaleOffset
        Layout.leftMargin: Tokens.padding.small * root.scaleOffset
        text: qsTr("Keyboard Locks")
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
            spacing: Tokens.spacing.small * root.scaleOffset

            StyledText {
                text: Kwin.capsLock ? qsTr("Caps Lock enabled") : qsTr("Caps Lock disabled")
                font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
            }

            StyledText {
                text: Kwin.numLock ? qsTr("Num Lock enabled") : qsTr("Num Lock disabled")
                font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
            }
        }
    }
}

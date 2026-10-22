pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.utils

ColumnLayout {
    id: root

    required property PopoutState popouts
    required property string label
    required property int subPageIdx

    property real scaleOffset: 1.0
    property real fontScale: 1.0
    property bool _isSidebarOpen: false

    width: card.implicitWidth
    implicitWidth: card.implicitWidth
    spacing: Tokens.spacing.medium * scaleOffset

    StyledRect {
        id: card

        Layout.fillWidth: true
        implicitWidth: cardLayout.implicitWidth + Tokens.padding.medium * 2 * root.scaleOffset
        implicitHeight: cardLayout.implicitHeight + Tokens.padding.medium * 2 * root.scaleOffset
        radius: Tokens.rounding.medium * root.scaleOffset
        color: Colours.tPalette.m3surfaceContainer
        clip: true

        ColumnLayout {
            id: cardLayout

            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Tokens.padding.medium * root.scaleOffset
            spacing: Tokens.spacing.small * root.scaleOffset

            StyledRect {
                id: settingsItem

                Layout.fillWidth: true
                implicitWidth: settingsRow.implicitWidth + Tokens.padding.medium * 2 * root.scaleOffset
                implicitHeight: settingsRow.implicitHeight + Tokens.padding.small * 2 * root.scaleOffset

                radius: Tokens.rounding.medium * root.scaleOffset
                color: "transparent"

                StateLayer {
                    anchors.fill: parent
                    radius: settingsItem.radius

                    onClicked: {
                        root.popouts.hasCurrent = false;
                        Hub.openPage(root.subPageIdx);
                    }
                }

                RowLayout {
                    id: settingsRow

                    anchors.centerIn: parent
                    spacing: Tokens.spacing.small * root.scaleOffset

                    MaterialIcon {
                        Layout.alignment: Qt.AlignVCenter
                        text: "settings"
                        color: Colours.palette.m3onSurfaceVariant
                        fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                    }

                    StyledText {
                        Layout.alignment: Qt.AlignVCenter
                        text: root.label
                        color: Colours.palette.m3onSurface
                        font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
                    }
                }
            }
        }
    }
}

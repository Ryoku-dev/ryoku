pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import M3Shapes
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services as Services
import caelestia.modules.bar.components as BarComponents

ColumnLayout {
    id: root

    required property var popouts
    property var days: BarComponents.GithubStore.days || []
    property int total: BarComponents.GithubStore.total || 0
    property string username: BarComponents.GithubStore.username || ""
    property string lastError: BarComponents.GithubStore.lastError || ""

    property real scaleOffset: 1.0
    property real fontScale: 1.0
    property bool _isSidebarOpen: false

    width: 300 * scaleOffset
    spacing: Tokens.spacing.small * scaleOffset

    StyledText {
        Layout.topMargin: Tokens.padding.medium * root.scaleOffset
        Layout.leftMargin: Tokens.padding.small * root.scaleOffset
        text: qsTr("GitHub")
        font.weight: 500
        font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
    }

    StyledRect {
        Layout.fillWidth: true
        implicitWidth: cardLayout.implicitWidth + Tokens.padding.medium * 2 * root.scaleOffset
        implicitHeight: cardLayout.implicitHeight + Tokens.padding.medium * 2 * root.scaleOffset
        radius: Tokens.rounding.medium * root.scaleOffset
        color: Services.Colours.tPalette.m3surfaceContainer
        clip: true

        ColumnLayout {
            id: cardLayout

            width: parent.width - Tokens.padding.medium * 2 * root.scaleOffset
            x: Tokens.padding.medium * root.scaleOffset
            y: Tokens.padding.medium * root.scaleOffset
            spacing: Tokens.spacing.small * root.scaleOffset

            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.spacing.small * root.scaleOffset

                MaterialIcon {
                    text: "person"
                    color: Services.Colours.palette.m3onSurfaceVariant
                    fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                }

                StyledText {
                    Layout.fillWidth: true
                    text: root.username.length > 0 ? `@${root.username}` : qsTr("Not authenticated")
                    color: Services.Colours.palette.m3onSurface
                    font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
                }
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.spacing.small * root.scaleOffset
                visible: root.lastError.length === 0

                MaterialIcon {
                    text: "history"
                    color: Services.Colours.palette.m3onSurfaceVariant
                    fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                }

                StyledText {
                    Layout.fillWidth: true
                    text: qsTr("Last 7 days")
                    color: Services.Colours.palette.m3onSurfaceVariant
                    font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
                }

                StyledText {
                    text: qsTr("%1 commits").arg(root.total)
                    font.weight: 600
                    color: Services.Colours.palette.m3onSurface
                    font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
                }
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.spacing.small * root.scaleOffset
                visible: root.lastError.length > 0

                MaterialIcon {
                    text: "error"
                    color: Services.Colours.palette.m3error
                    fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                }

                StyledText {
                    Layout.fillWidth: true
                    text: root.lastError
                    color: Services.Colours.palette.m3error
                    wrapMode: Text.Wrap
                    font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
                }
            }

        }
    }

    IconTextButton {
        Layout.fillWidth: true
        inactiveColour: Services.Colours.palette.m3primaryContainer
        inactiveOnColour: Services.Colours.palette.m3onPrimaryContainer
        verticalPadding: Tokens.padding.small * root.scaleOffset
        text: qsTr("Open profile")
        icon: "open_in_new"

        onClicked: {
            root.popouts.hasCurrent = false;
            Qt.openUrlExternally("https://github.com/" + root.username);
        }
    }
}

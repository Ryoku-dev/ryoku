pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell.Services.Mpris
import Caelestia.Config
import caelestia.components
import caelestia.services
import caelestia.modules.dashboard.dash as DashMedia

// Bar popout reusing the dashboard mini player, with a source line on top.
ColumnLayout {
    id: root

    required property var popouts

    property real scaleOffset: 1.0
    property real fontScale: 1.0
    property bool _isSidebarOpen: false

    readonly property MprisPlayer player: Players.active
    readonly property string sourceName: Players.getIdentity(root.player)
    readonly property real miniWidth: 228

    width: root.miniWidth + Tokens.padding.large * 2
    spacing: Tokens.spacing.small

    RowLayout {
        Layout.fillWidth: true
        spacing: Tokens.spacing.small
        visible: root.sourceName !== ""

        StyledText {
            Layout.fillWidth: true
            text: root.sourceName
            font: Tokens.font.label.medium
            color: Colours.palette.m3onSurfaceVariant
            elide: Text.ElideRight
        }
    }

    Item {
        Layout.alignment: Qt.AlignHCenter
        Layout.preferredWidth: root.miniWidth
        Layout.preferredHeight: 340

        DashMedia.Media {
            width: root.miniWidth
            extraButtons: true
            showGif: false
        }
    }
}

pragma ComponentBehavior: Bound

import QtQuick
import Caelestia.Config
import caelestia.components
import caelestia.services
import caelestia.utils

StyledRect {
    id: root

    readonly property bool isHorizontal: Config.bar.position === "top" || Config.bar.position === "bottom"
    readonly property int barThickness: Math.round(Tokens.sizes.bar.innerWidth * Math.max(0.6, !isNaN(Config.bar.scale) ? Config.bar.scale : 1.0))

    color: Colours.tPalette.m3surfaceContainer
    radius: Tokens.rounding.full

    implicitWidth: isHorizontal ? (icon.implicitWidth + Tokens.padding.medium * 2) : barThickness
    implicitHeight: isHorizontal ? barThickness : (icon.implicitHeight + Tokens.padding.medium * 2)

    MaterialIcon {
        id: icon

        anchors.centerIn: parent
        text: {
            if (Notifs.dnd)
                return "notifications_off";
            if (Notifs.openCount > 0)
                return "notifications_unread";
            return "notifications";
        }
        color: Colours.palette.m3secondary
    }

    StateLayer {
        radius: root.radius
        acceptedButtons: Qt.LeftButton | Qt.RightButton
        Accessible.name: qsTr("Notifications and sidebar toggle")
        Accessible.role: Accessible.Button
        Accessible.description: qsTr("Left-click to open the sidebar. Right-click to toggle Do Not Disturb")

        onClicked: mouse => {
            if (mouse.button === Qt.RightButton) {
                Notifs.dnd = !Notifs.dnd;
            } else {
                const vis = Visibilities.getForActive();
                vis.sidebar = !vis.sidebar;
            }
        }
    }
}
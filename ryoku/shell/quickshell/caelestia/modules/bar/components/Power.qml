import QtQuick
import Caelestia.Config
import caelestia.components
import caelestia.services
import caelestia.utils

Item {
    id: root

    required property DrawerVisibilities visibilities


    implicitWidth: icon.implicitHeight + Tokens.padding.small
    implicitHeight: icon.implicitHeight

    StateLayer {
        // Cursed workaround to make the height larger than the parent
        anchors.fill: undefined
        anchors.centerIn: parent
        implicitWidth: implicitHeight
        implicitHeight: icon.implicitHeight + Tokens.padding.small
        radius: Tokens.rounding.full
        Accessible.name: qsTr("Power and session menu")
        Accessible.role: Accessible.Button
        Accessible.description: qsTr("Opens the power, restart, and logout menu")
        acceptedButtons: Qt.LeftButton | Qt.RightButton
        onClicked: mouse => {
            if (mouse.button === Qt.RightButton) {
                Hub.open("shellstyle");
                return;
            }
            root.visibilities.session = !root.visibilities.session
        }
    }

    MaterialIcon {
        id: icon

        anchors.centerIn: parent
        anchors.horizontalCenterOffset: Centering.pixelAlign(parent.width, width)

        text: "power_settings_new"
        color: Colours.palette.m3error
        fontStyle: Tokens.font.icon.builders.small.weight(Font.Bold).build()
    }
}

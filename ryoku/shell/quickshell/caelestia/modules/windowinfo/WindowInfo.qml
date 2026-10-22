import QtQuick
import QtQuick.Layouts
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.services

StyledRect {
    id: root

    property string clientAddress: ""
    property var client: {
        if (Kwin.windowList.length > 0) {
            if (clientAddress !== "") {
                for (let i = 0; i < Kwin.windowList.length; ++i) {
                    if (Kwin.windowList[i].address === clientAddress) {
                        return Kwin.windowList[i];
                    }
                }
            } else {
                for (let i = 0; i < Kwin.windowList.length; ++i) {
                    if (Kwin.activeWindow && Kwin.windowList[i].address === Kwin.activeWindow.address) {
                        return Kwin.windowList[i];
                    }
                }
            }
        }
        return null;
    }

    signal closeRequested()

    color: Colours.palette.m3surfaceContainer
    radius: Tokens.rounding.large
    clip: true
    implicitWidth: 1100
    implicitHeight: 650

    RowLayout {
        id: child

        anchors.fill: parent
        anchors.margins: Tokens.padding.large
        spacing: Tokens.spacing.medium

        Preview {
            client: root.client
            Layout.fillWidth: true
            Layout.fillHeight: true
        }
        ColumnLayout {
            spacing: Tokens.spacing.medium

            Details {
                client: root.client
                Layout.fillWidth: true
                Layout.fillHeight: true
            }
            Buttons {
                id: buttons

                client: root.client
                onCloseRequested: root.closeRequested()
                Layout.fillWidth: true
            }
            Layout.preferredWidth: 420
            Layout.fillHeight: true
        }
    }
}

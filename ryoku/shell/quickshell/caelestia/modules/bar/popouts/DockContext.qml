pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Caelestia
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.utils

ColumnLayout {
    id: root

    required property PopoutState popouts
    property var model: popouts.dockModel

    property bool isPinned: {
        if (!model)
            return false;
        const current = GlobalConfig.bar.dock.pinnedApps || [];
        for (let i = 0; i < current.length; i++) {
            if (model.id === current[i] || (model.entry && model.entry.id === current[i])) {
                return true;
            }
        }
        return false;
    }

    property real scaleOffset: 1.0
    property real fontScale: 1.0
    property bool _isSidebarOpen: false

    width: 200 * scaleOffset
    implicitWidth: 200 * scaleOffset
    spacing: Tokens.spacing.medium * scaleOffset

    StyledRect {
        Layout.fillWidth: true
        implicitHeight: cardLayout.implicitHeight + Tokens.padding.medium * 2 * root.scaleOffset
        radius: Tokens.rounding.medium * root.scaleOffset
        color: Colours.tPalette.m3surfaceContainer
        clip: true
        visible: model && model.entry != null

        ColumnLayout {
            id: cardLayout

            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Tokens.padding.medium * root.scaleOffset
            spacing: Tokens.spacing.small * root.scaleOffset

            StyledRect {
                id: pinItem

                Layout.fillWidth: true
                implicitHeight: pinLabel.implicitHeight

                radius: Tokens.rounding.full
                color: "transparent"

                StateLayer {
                    anchors.margins: -Tokens.padding.medium / 2 * root.scaleOffset
                    anchors.leftMargin: -Tokens.padding.medium * root.scaleOffset
                    anchors.rightMargin: -Tokens.padding.medium * root.scaleOffset

                    radius: pinItem.radius

                    onClicked: {
                        if (isPinned) {
                            const current = GlobalConfig.bar.dock.pinnedApps ? [...GlobalConfig.bar.dock.pinnedApps] : [];
                            let index = current.indexOf(model.id);
                            if (index === -1 && model.entry)
                                index = current.indexOf(model.entry.id);
                            if (index !== -1) {
                                current.splice(index, 1);
                                GlobalConfig.bar.dock.pinnedApps = current;
                            }
                        } else {
                            const current = GlobalConfig.bar.dock.pinnedApps ? [...GlobalConfig.bar.dock.pinnedApps] : [];
                            const idToPin = model.entry ? model.entry.id : model.id;
                            if (!current.includes(idToPin)) {
                                current.push(idToPin);
                                GlobalConfig.bar.dock.pinnedApps = current;
                            }
                        }
                        root.popouts.hasCurrent = false;
                    }
                }

                StyledText {
                    id: pinLabel

                    anchors.left: parent.left
                    text: isPinned ? qsTr("Unpin from dock") : qsTr("Pin to dock")
                    font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
                }
            }

            StyledRect {
                id: newWinItem

                Layout.fillWidth: true
                implicitHeight: newWinLabel.implicitHeight

                radius: Tokens.rounding.full
                color: "transparent"

                StateLayer {
                    anchors.margins: -Tokens.padding.medium / 2 * root.scaleOffset
                    anchors.leftMargin: -Tokens.padding.medium * root.scaleOffset
                    anchors.rightMargin: -Tokens.padding.medium * root.scaleOffset

                    radius: newWinItem.radius

                    onClicked: {
                        if (model.entry)
                            Launch.launchEntry(model.entry);

                        root.popouts.hasCurrent = false;
                    }
                }

                StyledText {
                    id: newWinLabel

                    anchors.left: parent.left
                    text: qsTr("Open new window")
                    font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
                }
            }
        }
    }

    IconTextButton {
        Layout.fillWidth: true
        inactiveColour: Colours.palette.m3primaryContainer
        inactiveOnColour: Colours.palette.m3onPrimaryContainer
        verticalPadding: Tokens.padding.small * root.scaleOffset
        text: qsTr("End task")
        icon: "close"
        visible: {
            if (!model || !model.toplevels || model.toplevels.length === 0) return false;
            return !model.toplevels.some(t => t.title && String(t.title).startsWith("Nexus"));
        }

        onClicked: {
            for (const toplevel of model.toplevels) {
                if (toplevel.pid) {
                    Quickshell.execDetached({ command: ["kill", "-15", String(toplevel.pid)] });
                } else if (Kwin.windowList.length > 0) {
                    Kwin.closeWindow(toplevel.address);
                } else {
                    Kwin.dispatch(Kwin.usingLua ? `hl.dsp.window.close({ window = "address:0x${toplevel.address}" })` : `closewindow address:0x${toplevel.address}`);
                }
            }
            root.popouts.hasCurrent = false;
        }
    }
}

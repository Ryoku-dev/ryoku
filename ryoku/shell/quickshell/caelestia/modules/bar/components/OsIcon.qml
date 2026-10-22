import QtQuick
import Quickshell.Io
import Quickshell.Widgets
import Caelestia.Config
import caelestia.components
import caelestia.components.effects
import caelestia.services
import caelestia.utils

Item {
    id: root

    required property var bar

    implicitWidth: Math.round(Tokens.font.body.large.pointSize * 1.2)
    implicitHeight: Math.round(Tokens.font.body.large.pointSize * 1.2)

    StateLayer {
        anchors.fill: undefined
        anchors.centerIn: parent
        implicitWidth: root.implicitWidth + Tokens.padding.medium
        implicitHeight: root.implicitHeight + Tokens.padding.medium
        radius: Tokens.rounding.full
        acceptedButtons: Qt.LeftButton | Qt.RightButton
        onClicked: mouse => {
            if (mouse.button === Qt.RightButton) {
                const popouts = root.bar.popouts;
                if (!popouts)
                    return;
                if (popouts.hasCurrent && popouts.currentName === "osiconcontext") {
                    popouts.hasCurrent = false;
                } else {
                    popouts.currentName = "osiconcontext";
                    popouts.currentCenter = root.bar.isHorizontal ? root.mapToItem(null, root.implicitWidth / 2, 0).x : (root.mapToItem(null, 0, root.implicitHeight / 2).y ?? 0);
                    popouts.hasCurrent = true;
                }
                return;
            }
            // The Caelestia preset turns its own launcher off (launcher.enabled),
            // so the toggle below would do nothing: open the Ryoku launcher.
            if (!Config.launcher.enabled) {
                launcherProc.running = true;
                return;
            }
            const visibilities = Visibilities.getForActive();
            visibilities.launcher = !visibilities.launcher;
        }
    }

    Process {
        id: launcherProc
        command: ["ryoku-shell", "launcher"]
    }

    Loader {
        asynchronous: true
        anchors.centerIn: parent
        sourceComponent: {
            if (SysInfo.isDefaultLogo) {
                return caelestiaLogo;
            } else if (GlobalConfig.general.logo && GlobalConfig.general.logo !== "caelestia") {
                return customIcon;
            } else {
                return distroIcon;
            }
        }
    }

    Component {
        id: caelestiaLogo

        Logo {
            implicitWidth: Math.round(Tokens.font.body.large.pointSize * 1.6)
            implicitHeight: Math.round(Tokens.font.body.large.pointSize * 1.6)
        }
    }

    Component {
        id: distroIcon

        ColouredIcon {
            source: SysInfo.osLogo
            implicitSize: Math.round(Tokens.font.body.large.pointSize * 1.2)
            colour: Colours.palette.m3tertiary
        }
    }

    Component {
        id: customIcon

        Loader {
            sourceComponent: SysInfo.recolourCustomLogo ? colouredIconComponent : iconImageComponent
        }
    }

    Component {
        id: colouredIconComponent

        ColouredIcon {
            source: SysInfo.osLogo
            implicitSize: Math.round(Tokens.font.body.large.pointSize * 1.2 * (SysInfo.customLogoSize / 100))
            colour: Colours.palette.m3tertiary
        }
    }

    Component {
        id: iconImageComponent

        IconImage {
            source: SysInfo.osLogo
            implicitWidth: Math.round(Tokens.font.body.large.pointSize * 1.2 * (SysInfo.customLogoSize / 100))
            implicitHeight: Math.round(Tokens.font.body.large.pointSize * 1.2 * (SysInfo.customLogoSize / 100))
        }
    }
}

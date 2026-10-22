pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Caelestia.Components
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services

Item {
    id: root

    required property DrawerVisibilities visibilities

    function session(action: string): void {
        root.visibilities.dashboard = false;
        HostState.session(action);
    }

    // Without a page the Hub opens on its home page, as the Ryoku Controls gear does.
    // Opens before closing: closing the dashboard unloads this item, so nothing
    // scheduled here would still run.
    function openSettings(page: string): void {
        Hub.open(page);
        root.visibilities.dashboard = false;
    }

    anchors.fill: parent
    implicitHeight: layout.implicitHeight + layout.anchors.margins * 2


    ColumnLayout {
        id: layout

        anchors.fill: parent
        anchors.margins: Tokens.padding.large
        spacing: Tokens.spacing.medium

        // every Bottom controls chip of the Controls page, in its order; the ones beyond
        // the slots scroll in
        ButtonCarousel {
            id: carousel

            Layout.fillWidth: true
            slots: 7
            spacing: Tokens.spacing.small

            Toggle {
                visible: HostState.element("lock")
                icon: "lock"
                isToggle: false
                inactiveOnColour: Colours.palette.m3onSurfaceVariant
                onClicked: {
                    root.visibilities.dashboard = false;
                    Quickshell.execDetached(["loginctl", "lock-session"]);
                    Audio.playLock();
                }
            }

            Toggle {
                visible: HostState.element("sleep")
                icon: "dark_mode"
                isToggle: false
                inactiveOnColour: Colours.palette.m3onSurfaceVariant
                onClicked: root.session("suspend")
            }

            Toggle {
                visible: HostState.element("logout")
                icon: "logout"
                isToggle: false
                inactiveOnColour: Colours.palette.m3onSurfaceVariant
                onClicked: root.session("logout")
            }

            Toggle {
                visible: HostState.element("restart")
                icon: "restart_alt"
                isToggle: false
                inactiveOnColour: Colours.palette.m3onSurfaceVariant
                onClicked: root.session("reboot")
            }

            Toggle {
                visible: HostState.element("powerOff")
                icon: "power_settings_new"
                isToggle: false
                inactiveOnColour: Colours.palette.m3onSurfaceVariant
                onClicked: root.session("shutdown")
            }

            Toggle {
                visible: HostState.element("nightLight")
                icon: "bedtime"
                checked: HyprSunset.active
                onClicked: HyprSunset.toggleNightLight()
            }

            Toggle {
                visible: HostState.element("keepAwake")
                icon: "coffee"
                checked: IdleInhibitor.enabled
                onClicked: IdleInhibitor.enabled = !IdleInhibitor.enabled
            }

            Toggle {
                visible: HostState.element("doNotDisturb")
                icon: "notifications_off"
                checked: Notifs.dnd
                onClicked: Notifs.dnd = !Notifs.dnd
            }

            Toggle {
                visible: HostState.element("micMute")
                icon: Audio.sourceMuted ? "mic_off" : "mic"
                checked: !Audio.sourceMuted
                onClicked: {
                    const audio = Audio.source?.audio;
                    if (audio)
                        audio.muted = !audio.muted;
                }
            }

            Toggle {
                visible: HostState.element("gamingMode")
                icon: "gamepad"
                checked: GameMode.enabled
                onClicked: GameMode.enabled = !GameMode.enabled
            }

            Toggle {
                visible: HostState.element("panelSettings")
                icon: "settings"
                isToggle: false
                inactiveOnColour: Colours.palette.m3onSurfaceVariant
                onClicked: root.openSettings("")
            }

            // the Caelestia look: transparency, rounding, borders, scale and animations
            Toggle {
                icon: "palette"
                isToggle: false
                inactiveOnColour: Colours.palette.m3onSurfaceVariant
                onClicked: root.openSettings("shellstyle")
            }

            Toggle {
                visible: HostState.element("pluginCards") && HostState.extensionCount > 0
                icon: "extension"
                isToggle: false
                inactiveOnColour: Colours.palette.m3onSurfaceVariant
                onClicked: HostState.openExtensions()
            }
        }
    }

    component Toggle: IconButton {
        inactiveColour: Colours.layer(Colours.palette.m3surfaceContainerHighest, 2)
        width: carousel.slotWidth
        isToggle: true
        isRound: true
        shapeMorph: true
        padding: Tokens.padding.medium
    }
}

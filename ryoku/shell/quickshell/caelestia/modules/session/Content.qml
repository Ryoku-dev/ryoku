pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia
import Caelestia.Config
import Caelestia.Services
import caelestia.components
import caelestia.components.controls
import caelestia.components.effects
import caelestia.services
import caelestia.utils

Column {
    id: root

    required property DrawerVisibilities visibilities

    padding: Tokens.padding.large
    rightPadding: CUtils.clamp(padding - Config.border.thickness, 0, padding)
    spacing: Tokens.spacing.large

    SessionButton {
        id: lock

        visible: !Paths.hosted || HostState.element("lock")
        action: "lock"
        icon: "lock"
        command: ["loginctl", "lock-session"]

        KeyNavigation.down: logout
    }

    SessionButton {
        id: logout

        visible: !Paths.hosted || HostState.element("logout")
        action: "logout"
        icon: Config.session.icons.logout
        command: ["sh", "-c", "qdbus6 org.kde.Shutdown /Shutdown org.kde.Shutdown.logout 2>/dev/null"]

        KeyNavigation.up: lock
        KeyNavigation.down: shutdown

        Component.onCompleted: forceActiveFocus()

        Connections {
            function onLauncherChanged(): void {
                if (!root.visibilities.launcher)
                    logout.forceActiveFocus();
            }

            target: root.visibilities
        }
    }

    SessionButton {
        id: shutdown

        visible: !Paths.hosted || HostState.element("powerOff")
        action: "shutdown"
        icon: Config.session.icons.shutdown
        command: Config.session.commands.shutdown

        KeyNavigation.up: logout
        KeyNavigation.down: hibernate
    }

    Item {
        width: Tokens.sizes.session.button
        height: Tokens.sizes.session.button

        AnimatedImage {
            anchors.fill: parent
            sourceSize.width: width * ((QsWindow.window as QsWindow)?.devicePixelRatio ?? 1)
            playing: visible
            asynchronous: true
            speed: Config.general.sessionGifSpeed
            source: Paths.absolutePath(Config.paths.sessionGif)
            fillMode: AnimatedImage.PreserveAspectFit
            opacity: GlobalConfig.general.caelestiaMode ? 0 : 1

            Behavior on opacity { Anim { type: Anim.Standard } }

            visible: Config.paths.sessionGif !== ""
        }

        AnimatedImage {
            anchors.fill: parent
            sourceSize.width: width * ((QsWindow.window as QsWindow)?.devicePixelRatio ?? 1)
            playing: visible
            asynchronous: true
            speed: Config.general.sessionGifSpeed
            source: Paths.absolutePath("root:/assets/dino.gif")
            fillMode: AnimatedImage.PreserveAspectFit
            opacity: GlobalConfig.general.caelestiaMode ? 1 : 0

            Behavior on opacity { Anim { type: Anim.Standard } }
            
            layer.enabled: true

            layer.effect: Colouriser {
                colorizationColor: Colours.palette.m3onSurface
                sourceColor: "white"
            }
        }

        AnimatedImage {
            anchors.fill: parent
            sourceSize.width: width * ((QsWindow.window as QsWindow)?.devicePixelRatio ?? 1)
            playing: visible
            asynchronous: true
            speed: Config.general.sessionGifSpeed
            source: Paths.absolutePath("root:/assets/dino.gif")
            fillMode: AnimatedImage.PreserveAspectFit
            opacity: GlobalConfig.general.caelestiaMode ? 1 : 0

            Behavior on opacity { Anim { type: Anim.Standard } }
            
            layer.enabled: true

            layer.effect: Colouriser {
                colorizationColor: Colours.palette.m3onSurface
                sourceColor: "white"
            }
        }
    }

    SessionButton {
        id: hibernate

        visible: !Paths.hosted || HostState.element("sleep")
        action: "suspend"
        icon: Config.session.icons.hibernate
        command: Config.session.commands.hibernate

        KeyNavigation.up: shutdown
        KeyNavigation.down: reboot
    }

    SessionButton {
        id: reboot

        visible: !Paths.hosted || HostState.element("restart")
        action: "reboot"
        icon: Config.session.icons.reboot
        command: Config.session.commands.reboot

        KeyNavigation.up: hibernate
    }

    component SessionButton: IconButton {
        id: button

        required property list<string> command
        required property string action

        function exec(): void {
            // hosted, the host shell runs the action the way its own Controls do
            if (Paths.hosted) {
                root.visibilities.session = false;
                HostState.session(action);
                return;
            }
            if (!SessionManager.exec(command))
                Quickshell.execDetached(command);
        }

        implicitWidth: Tokens.sizes.session.button
        implicitHeight: Tokens.sizes.session.button

        inactiveColour: activeFocus ? Colours.palette.m3secondaryContainer : Colours.tPalette.m3surfaceContainer
        inactiveOnColour: activeFocus ? Colours.palette.m3onSecondaryContainer : Colours.palette.m3onSurface
        radius: pressed ? Tokens.rounding.medium : activeFocus ? Tokens.rounding.extraLarge : Tokens.rounding.largeIncreased
        font: Tokens.font.icon.builders.large.scale(1.3).build()
        onClicked: exec()

        Keys.onEnterPressed: exec()
        Keys.onReturnPressed: exec()
        Keys.onEscapePressed: root.visibilities.session = false
        Keys.onPressed: event => {
            if (!Config.session.vimKeybinds)
                return;

            if (event.modifiers & Qt.ControlModifier) {
                if ((event.key === Qt.Key_J || event.key === Qt.Key_N) && KeyNavigation.down) {
                    KeyNavigation.down.focus = true;
                    event.accepted = true;
                } else if ((event.key === Qt.Key_K || event.key === Qt.Key_P) && KeyNavigation.up) {
                    KeyNavigation.up.focus = true;
                    event.accepted = true;
                }
            } else if (event.key === Qt.Key_Tab && KeyNavigation.down) {
                KeyNavigation.down.focus = true;
                event.accepted = true;
            } else if (event.key === Qt.Key_Backtab || (event.key === Qt.Key_Tab && (event.modifiers & Qt.ShiftModifier))) {
                if (KeyNavigation.up) {
                    KeyNavigation.up.focus = true;
                    event.accepted = true;
                }
            }
        }
    }
}

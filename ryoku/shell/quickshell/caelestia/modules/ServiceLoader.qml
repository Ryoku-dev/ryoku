import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.services

Scope {
    Component.onCompleted: {
        ConfigMigrations;
        Notifs;
    }

    Timer {
        id: deferredServices

        interval: 250
        repeat: false
        running: true

        onTriggered: {
            IdleInhibitor;
            GameMode;
            Players;
            Brightness;
            Weather.reload();
            WorkspaceTrackerGuard;

            if (GlobalConfig.utilities.vpn.enabled)
                VPN;
        }
    }
}

pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Caelestia
import Caelestia.Config
import caelestia.services
import caelestia.utils

Singleton {
    id: root

    property alias enabled: props.enabled

    property bool restoreVideoWallpaper: false

    property bool autoEnabled: false

    readonly property var _windows: Kwin.windowList

    function _matchesRule(w): bool {
        const rules = GlobalConfig.utilities.gameMode.autoEnableRegexes || [];
        if (rules.length === 0 || !w)
            return false;
        const fields = [w.class || "", w.initialClass || "", w.title || ""];
        for (let i = 0; i < rules.length; i++) {
            const rule = String(rules[i] || "");
            if (rule === "")
                continue;
            for (let f = 0; f < fields.length; f++) {
                if (fields[f] === rule)
                    return true;
                try {
                    if (new RegExp(rule).test(fields[f]))
                        return true;
                } catch (e) {
                }
            }
        }
        return false;
    }

    on_WindowsChanged: {
        const wins = root._windows || [];
        let matched = false;
        for (let i = 0; i < wins.length; i++)
            if (root._matchesRule(wins[i])) { matched = true; break; }

        if (matched && !root.enabled) {
            root.autoEnabled = true;
            root.enabled = true;
        } else if (!matched && root.enabled && root.autoEnabled) {
            root.autoEnabled = false;
            root.enabled = false;
        }
    }

    onEnabledChanged: {
        if (!enabled)
            root.autoEnabled = false;

        if (enabled) {
            root.restoreVideoWallpaper = !GlobalConfig.background.videoWallpaperPaused;
            if (root.restoreVideoWallpaper)
                GlobalConfig.background.videoWallpaperPaused = true;

            Kwin.setGameMode(true);

            if (GlobalConfig.utilities.toasts.gameModeChanged)
                Toaster.toast(qsTr("Game mode enabled"), qsTr("Animations, blur, gaps and shadows are off"), "gamepad");
        } else {
            if (root.restoreVideoWallpaper) {
                GlobalConfig.background.videoWallpaperPaused = false;
                root.restoreVideoWallpaper = false;
            }

            Kwin.setGameMode(false);

            if (GlobalConfig.utilities.toasts.gameModeChanged)
                Toaster.toast(qsTr("Game mode disabled"), qsTr("Desktop effects restored"), "gamepad");
        }
    }

    PersistentProperties {
        id: props

        property bool enabled: false

        reloadableId: "gameMode"
    }


    IpcHandler {
        function isEnabled(): bool {
            return props.enabled;
        }

        function toggle(): void {
            props.enabled = !props.enabled;
        }

        function enable(): void {
            props.enabled = true;
        }

        function disable(): void {
            props.enabled = false;
        }

        target: "gameMode"
    }
}

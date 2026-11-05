// Python: the serpantinum shell, ported from Serpantinum by ilyamiro
// (https://github.com/ilyamiro/serpantinum), GNU AGPL-3.0-or-later.
//
// A top or edge bar of pill widgets that open into one morphing stage:
// network, sound, calendar, media, a system panel, a floating quick-actions
// rail, a dock and a song pill, with the style's own settings guide. Ryoku
// loads a barstyle Scene once per monitor via Frame's Loader; like QS Bar and
// the frame family, this Scene hosts the whole-desktop system once on the
// primary output. The bar's own Variants still fan one band per screen; a
// screen whose bar is hidden in the Hub gets no band.
//
// Ryogami, ryoshot, qylock, the launcher selected in Ryoku Settings, the
// clipboard, Super+Escape sidebar, Hub, updates, idle and the shared dock and
// desktop-widget hosts stay Ryoku-owned. Python provides its own bar settings,
// policy prompt, banners and OSD while the style is active.
//
// Every surface below keeps Serpantinum's layout and motion; only the data
// plane (the window-manager seam, the daemon's settings store, the palette
// and the shell's live services) and the Ryoku-owned surfaces differ.
//
// This file is part of Python. Python is free software: you can redistribute
// it and/or modify it under the terms of the GNU Affero General Public
// License as published by the Free Software Foundation, either version 3 of
// the License, or (at your option) any later version.

pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import "."
import shell.services as ShellServices
import "guide" as PythonGuide
import "polkit" as PythonPolkit
import "../../../../services/lib/screens.js" as Screens

Item {
    id: scene

    // The screen, set by Frame's per-monitor Loader.
    property var modelData: null

    width: 0
    height: 0

    // Host the single system on exactly one Scene: the primary output, the
    // first entry of the shared deduped screen list.
    readonly property bool isPrimary: {
        const list = Screens.uniqueByName(Quickshell.screens)
        return list.length > 0 && !!scene.modelData
            && list[0].name === scene.modelData.name
    }

    Loader {
        active: scene.isPrimary
        sourceComponent: Component {
            Item {
                Main {}
                Bar {}
                PopoutManager {}

                PythonGuide.SettingsWindow { id: settingsHost }
                PythonPolkit.PythonPolkit {}

                Connections {
                    target: ShellServices.ShellState
                    function onSurfaceRequested(id, mon, ctx) {
                        if (id === "python-settings")
                            settingsHost.toggle();
                    }
                }

                Loader {
                    active: Config.getSetting("general", {}).quickactions !== false
                    sourceComponent: Floating {}
                }
            }
        }
    }
}

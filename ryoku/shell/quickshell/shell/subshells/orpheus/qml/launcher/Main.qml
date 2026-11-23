pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import "../../../../modules/launcher/shared/providers" as SharedProviders
import "vessel"

// The Orpheus launcher. Where Orpheus is the bar on the requested screen the
// launcher pours out of its voice island as a card; anywhere else it hangs
// from the top screen edge in a window of its own.
Scope {
    id: root

    // launcher contract, see docs/launcher.md
    property string monitor: ""
    // the Orpheus scene the launcher pours from, null while it uses its own window
    property var scene: null
    property bool standaloneOpen: false
    // the standalone window is built on first use and kept, unmapped, while this host lives
    property bool standaloneWanted: false

    readonly property bool sceneShown: root.scene !== null && root.scene.launcherShown === true
    readonly property bool standaloneShown: standaloneLoader.item !== null
        && standaloneLoader.item.shown === true
    readonly property bool open: root.scene !== null
        ? root.scene.launcherOpen === true : root.standaloneOpen
    // true until the vessel in use has fully closed, the host releases on false
    readonly property bool shown: root.sceneShown || root.standaloneOpen || root.standaloneShown
    readonly property var sheet: root.scene !== null ? root.scene.launcherSheet
        : standaloneLoader.item ? standaloneLoader.item.sheet : null

    function resolveMonitor(mon) {
        const wanted = String(mon || "");
        if (wanted !== "")
            return wanted;
        const screens = Quickshell.screens;
        for (let i = 0; i < screens.length; ++i) {
            if (screens[i] && screens[i].name === Wm.focusedOutput)
                return Wm.focusedOutput;
        }
        return screens.length > 0 && screens[0] ? String(screens[0].name) : "";
    }

    function show(mon) {
        const name = root.resolveMonitor(mon);
        const target = Vessels.sceneFor(name);
        if (root.scene !== null && root.scene !== target)
            root.scene.closeLauncher();
        root.monitor = name;
        if (target !== null) {
            root.standaloneOpen = false;
            root.scene = target;
            target.openLauncher();
            return;
        }
        root.scene = null;
        root.standaloneWanted = true;
        root.standaloneOpen = true;
    }

    function hide() {
        if (root.scene !== null)
            root.scene.closeLauncher();
        root.standaloneOpen = false;
    }

    function toggle(mon) {
        if (root.open)
            root.hide();
        else
            root.show(mon);
    }

    function stateDump() {
        const dump = root.sheet && typeof root.sheet.stateDump === "function"
            ? root.sheet.stateDump() : ({});
        dump.open = root.open;
        dump.shown = root.shown;
        dump.monitor = root.monitor;
        dump.vessel = root.scene !== null ? "orpheus" : root.standaloneWanted ? "standalone" : "";
        return dump;
    }

    // A scene that leaves (the bar switched away, the screen unplugged) takes
    // its card with it, so the launcher reads as closed.
    Connections {
        target: Vessels
        function onRevisionChanged() {
            if (root.scene !== null && Vessels.sceneFor(root.monitor) !== root.scene)
                root.scene = null;
        }
    }

    // the shell drops this host fifteen seconds after a close; until then the
    // Orpheus card stays built so a reopen only pours
    Component.onCompleted: Vessels.hold()
    Component.onDestruction: Vessels.release()

    SharedProviders.Providers {
        id: sharedProviders
    }

    LazyLoader {
        id: standaloneLoader
        active: root.standaloneWanted

        Standalone {
            monitor: root.monitor
            open: root.standaloneOpen
            onDismissed: root.hide()
        }
    }
}

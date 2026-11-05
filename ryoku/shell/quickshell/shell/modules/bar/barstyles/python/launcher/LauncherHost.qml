pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import "." as LauncherUi

Scope {
    id: root

    property bool hostVisible: false
    property string monitor: ""
    property var hostScreen: null

    function screenFor(name) {
        const screens = Quickshell.screens || [];
        const wanted = name || Wm.focusedOutput;
        for (let i = 0; i < screens.length; i++) {
            if (screens[i].name === wanted) return screens[i];
        }
        return screens.length > 0 ? screens[0] : null;
    }

    function show(name) {
        unloadTimer.stop();
        root.monitor = name || Wm.focusedOutput;
        root.hostScreen = root.screenFor(root.monitor);
        surface.active = true;
        root.hostVisible = true;
        unloadTimer.stop();
    }

    function hide() {
        root.hostVisible = false;
        unloadTimer.restart();
    }

    function toggle(name) {
        if (root.hostVisible) root.hide();
        else root.show(name);
    }

    Timer {
        id: unloadTimer
        interval: 800
        onTriggered: surface.active = false
    }

    Loader {
        id: surface
        active: false
        asynchronous: false
        sourceComponent: Component {
            LauncherUi.Launcher {
                host: root
                onCloseRequested: root.hide()
            }
        }
    }
}

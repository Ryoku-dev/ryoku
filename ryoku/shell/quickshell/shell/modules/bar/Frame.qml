pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services

Scope {
    id: root

    property var modelData: null
    property url retainedEntry: ""
    property string retainedId: ""
    property bool retainedPrimaryHosted: false
    property int retries: 0
    property bool retryReady: true

    readonly property bool primaryScreen: root.modelData
        && ShellState.screens.length > 0
        && ShellState.screens[0].name === root.modelData.name
    readonly property bool shouldHost: !root.retainedPrimaryHosted || root.primaryScreen

    function syncActiveScene() {
        if (Shells.activePending)
            return;
        root.retainedEntry = Shells.activeEntry;
        root.retainedId = Shells.active;
        root.retainedPrimaryHosted = Shells.primaryHosted;
    }

    function injectModelData() {
        if (sceneLoader.item)
            sceneLoader.item.modelData = root.modelData;
    }

    Component.onCompleted: root.syncActiveScene()
    onModelDataChanged: root.injectModelData()

    Loader {
        id: sceneLoader

        active: root.retryReady && root.shouldHost && root.retainedEntry !== ""
        source: root.retainedEntry

        onLoaded: {
            root.retries = 0;
            root.injectModelData();
        }
        onStatusChanged: {
            if (status !== Loader.Error || root.retainedId === "")
                return;
            if (root.retries < 10) {
                root.retries++;
                retryTimer.restart();
                return;
            }
            const brokenId = root.retainedId;
            root.retries = 0;
            Qt.callLater(Shells.markBroken, brokenId);
        }
    }

    Timer {
        id: retryTimer

        interval: 800
        onTriggered: {
            root.retryReady = false;
            Qt.callLater(() => root.retryReady = true);
        }
    }

    Connections {
        target: Shells

        function onActiveChanged() {
            root.retries = 0;
            retryTimer.stop();
            root.retryReady = true;
            root.syncActiveScene();
        }
        function onActiveEntryChanged() { root.syncActiveScene(); }
        function onActivePendingChanged() { root.syncActiveScene(); }
        function onPrimaryHostedChanged() { root.syncActiveScene(); }
    }
}

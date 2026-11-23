pragma Singleton
import QtQuick
import Quickshell

// The Orpheus scenes by screen name, so the launcher can pour out of the bar
// on that screen instead of opening its own window.
Singleton {
    id: root

    property var scenes: ({})
    property int revision: 0

    function register(screenName, scene) {
        const name = String(screenName || "");
        if (name === "" || !scene || root.scenes[name] === scene)
            return;
        const next = Object.assign({}, root.scenes);
        next[name] = scene;
        root.scenes = next;
        root.revision += 1;
    }

    function unregister(screenName, scene) {
        const name = String(screenName || "");
        if (name === "" || root.scenes[name] !== scene)
            return;
        const next = Object.assign({}, root.scenes);
        delete next[name];
        root.scenes = next;
        root.revision += 1;
    }

    function sceneFor(screenName) {
        const scene = root.scenes[String(screenName || "")];
        return scene ? scene : null;
    }
}

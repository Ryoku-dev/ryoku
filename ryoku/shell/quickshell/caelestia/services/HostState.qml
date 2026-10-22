pragma Singleton

import QtQuick

// What a host shell (the Ryoku shell running the Caelestia drawers, see Paths.hosted) says
// about the parts the host owns the choice of. Standing alone, the Caelestia shell shows
// everything its config lists.
QtObject {
    // the bar's dock entry: the host turns it on from its own dock settings
    property bool dock: true
    // the Controls layout of the host: section id -> shown. A section it does not list
    // (or standing alone) shows.
    property var controlsSections: ({})
    // the shown sections in the order the Controls page keeps them, and the elements it hides
    property var controlsOrder: ["vitals", "connections", "powerProfile", "media", "levels", "bottomControls"]
    property var controlsHidden: []

    // the host's plugin cards, as a component the dashboard shows on its Extensions tab
    property Component extensions: null
    property int extensionCount: 0

    // a session button was pressed; the host runs it (lock, suspend, logout, reboot, shutdown)
    signal session(string action)
    // the dashboard should show its Extensions tab
    signal openExtensions

    function section(id): bool {
        return controlsSections[id] !== false;
    }

    function element(id): bool {
        return controlsHidden.indexOf(id) < 0;
    }

    function allows(entry): bool {
        return entry.id !== "dock" || dock;
    }
}

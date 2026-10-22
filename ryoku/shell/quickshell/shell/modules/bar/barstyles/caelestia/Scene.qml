pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import caelestia.modules as CM
import caelestia.modules.drawers as Drawers
import caelestia.services as CS
import shell.services
import "../../../sidebar"

// The Caelestia style: the Caelestia shell's own drawers (frame, bar, dashboard, session,
// sidebar, launcher, OSD, popouts) running inside this shell, from the Caelestia library.
// They span every output themselves, so this scene hosts them once, from the first output,
// as the python style does. The Caelestia config is ~/.config/ryoku/caelestia/shell.json.
Item {
    id: scene

    property var modelData: null
    width: 0
    height: 0

    readonly property bool primary: modelData !== null && Quickshell.screens.length > 0
        && modelData.name === Quickshell.screens[0].name

    // the dock in the bar is the Caelestia design of the Ryoku dock, chosen in its settings
    Binding {
        target: CS.HostState
        property: "dock"
        value: Dock.design === "caelestia" && Dock.cfg("enabled", false)
    }

    // the dashboard cards follow what the Hub's Controls page shows and hides
    Binding {
        target: CS.HostState
        property: "controlsSections"
        value: {
            const shown = {};
            for (const s of SidebarState.controls.sections)
                shown[s.id] = s.visible;
            return shown;
        }
    }

    Binding {
        target: CS.HostState
        property: "controlsOrder"
        value: SidebarState.visibleSections.map(s => s.id)
    }

    Binding {
        target: CS.HostState
        property: "controlsHidden"
        value: SidebarState.controls.hidden
    }

    Connections {
        target: CS.HostState

        function onSession(action: string): void {
            SessionActions.run(action);
        }
    }

    // the plugin cards of the Controls page, as an Extensions tab of the dashboard
    SidebarPlugins {
        id: pluginHost

        active: scene.primary && SidebarState.elementVisible("pluginCards")
    }

    Binding {
        target: CS.HostState
        property: "extensionCount"
        value: pluginHost.active ? pluginHost.cards().length : 0
    }

    Binding {
        target: CS.HostState
        property: "extensions"
        value: extensionsBoard
    }

    Component {
        id: extensionsBoard

        ExtensionsBoard {
            s: 1
            plugins: pluginHost.cards()
            active: true
        }
    }

    Loader {
        active: scene.primary
        sourceComponent: Item {
            CM.ServiceLoader {}
            Drawers.Drawers {}
            CM.Shortcuts {}
        }
    }
}

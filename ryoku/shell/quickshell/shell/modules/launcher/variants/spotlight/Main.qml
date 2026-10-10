// Spotlight is the Shima frame's morphing-glass search. When Shima is active
// its per-output chassis already owns the palette, so this variant only drives
// the shared search state. Every other bar style gets a standalone layer-shell
// host around the same palette component.

//@ pragma UseQApplication

import QtQuick
import Quickshell
import shell.services
import inir
import inir.services.deferred
import inir.modules.common as Iris
import "." as SpotlightVariant

Scope {
    id: root
    property string screenName: ""

    // ── launcher contract (docs/launcher.md) ─────────────────────────────────
    readonly property bool shown: GlobalStates.searchOpen
    // Shima's internal style id remains "iris" so existing settings keep working.
    readonly property bool frameHosts: Config.barStyle === "iris"
        && (Iris.Config.options?.iris?.modules?.palette ?? true)

    function show(mon) {
        root.screenName = String(mon ?? "")
        GlobalStates.searchOpen = true
    }
    function hide() {
        GlobalStates.searchOpen = false
    }
    function toggle(mon) {
        root.shown ? root.hide() : root.show(mon)
    }
    function stateDump() {
        return {
            open: GlobalStates.searchOpen,
            query: LauncherSearch.query ?? "",
            resultCount: (LauncherSearch.results ?? []).length
        }
    }

    Loader {
        active: !root.frameHosts
        sourceComponent: Component {
            SpotlightVariant.StandaloneSurface {
                open: GlobalStates.searchOpen
                screenName: root.screenName
            }
        }
    }
}

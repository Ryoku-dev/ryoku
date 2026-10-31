// iRiS: the frame family ported from iNiR — an island on any screen edge that
// grows into whatever you clicked, with morphing glass surfaces, bubbles, a
// dock, a studio and its own settings overlay. Ryoku loads a barstyle Scene
// once per monitor via Frame's Loader, so this Scene hosts the single panel
// system one time on the primary output; the family's own Variants fan its
// surfaces across every screen. The boot ladder mirrors the reference shell:
// panels wait for the config mirror, then the first frame, then the deferred
// tier, so the desktop comes up in the same order the family was built for.

pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import inir
import inir.modules.common
import inir.modules.iris
import inir.modules.iris.critical
import inir.services
import inir.services.deferred
import shell.services as Ryoku
import "../../../../services/lib/screens.js" as Screens

Item {
    id: scene

    // The screen, set by Frame's per-monitor Loader.
    property var modelData: null

    width: 0
    height: 0

    // Host the single panel system on exactly one Scene: the primary output,
    // the first entry of the shared deduped screen list (services/lib/
    // screens.js). Matching by name keeps exactly one host even when a
    // duplicate output announce briefly swaps the ShellScreen object.
    readonly property bool isPrimary: {
        const list = Screens.uniqueByName(Quickshell.screens)
        return list.length > 0 && !!scene.modelData
            && list[0].name === scene.modelData.name
    }

    Loader {
        active: scene.isPrimary
        sourceComponent: Component {
            Item {
                // The critical tier: the bar/island, the reservations surface.
                ShellIrisCriticalPanels {}

                // The deferred tier: side panels, popups, studio, settings,
                // OSD, session, Spotlight, cheatsheet and floating tools.
                Loader {
                    active: Config.ready && GlobalStates.deferredPanelsReady
                    sourceComponent: Component { ShellIrisPanelsImpl {} }
                }

                // The reference's boot ladder, re-driven whenever the shell
                // hot-reloads (singletons keep their one-shot state).
                Component.onCompleted: {
                    // GlobalActions owns the `globalActions` IPC target; it is a
                    // lazy singleton, so without this nudge the target stays
                    // unregistered until the palette first opens.
                    void GlobalActions.allActions
                    void KeybindCatalogue.categories
                    GlobalStates.shellEntryReady = false
                    GlobalStates.deferredPanelsReady = false
                    if (Config.ready)
                        entryTimer.restart()
                }

                Timer {
                    id: entryTimer
                    interval: Appearance.animationsEnabled ? 200 : 0
                    onTriggered: {
                        GlobalStates.shellEntryReady = true
                        deferredTimer.start()
                    }
                }

                Timer {
                    id: deferredTimer
                    interval: 500
                    onTriggered: GlobalStates.deferredPanelsReady = true
                }

                Connections {
                    target: Ryoku.ShellState
                    function onSurfaceRequested(id, mon, ctx): void {
                        switch (id) {
                        case "launcher":
                            GlobalStates.searchOpen = !GlobalStates.searchOpen
                            break
                        case "clipboard":
                            if (GlobalStates.searchOpen) {
                                GlobalStates.searchOpen = false
                            } else {
                                LauncherSearch.ensurePrefix(Config.options?.search?.prefix?.clipboard ?? ";")
                                GlobalStates.irisSpotlightQuery = LauncherSearch.query
                                GlobalStates.searchOpen = true
                            }
                            break
                        case "settings":
                            if (GlobalStates.settingsOverlayOpen)
                                GlobalStates.settingsOverlayOpen = false
                            else
                                GlobalStates.openSettings()
                            break
                        case "cheatsheet":
                            GlobalStates.cheatsheetOpen = !GlobalStates.cheatsheetOpen
                            break
                        case "tools":
                            GlobalStates.overlayOpen = !GlobalStates.overlayOpen
                            break
                        }
                    }
                }

                IpcHandler {
                    target: "clipboard"
                    function open(): void {
                        LauncherSearch.ensurePrefix(Config.options?.search?.prefix?.clipboard ?? ";")
                        GlobalStates.irisSpotlightQuery = LauncherSearch.query
                        GlobalStates.searchOpen = true
                    }
                    function close(): void { GlobalStates.searchOpen = false }
                    function toggle(): void {
                        if (GlobalStates.searchOpen) close()
                        else open()
                    }
                }

                IpcHandler {
                    target: "cheatsheet"
                    function open(): void { GlobalStates.cheatsheetOpen = true }
                    function close(): void { GlobalStates.cheatsheetOpen = false }
                    function toggle(): void { GlobalStates.cheatsheetOpen = !GlobalStates.cheatsheetOpen }
                }

                IpcHandler {
                    target: "session"
                    function open(): void { GlobalStates.sessionOpen = true }
                    function close(): void { GlobalStates.sessionOpen = false }
                    function toggle(): void { GlobalStates.sessionOpen = !GlobalStates.sessionOpen }
                }

                IpcHandler {
                    target: "overlay"
                    function toggle(): void { GlobalStates.overlayOpen = !GlobalStates.overlayOpen }
                    function tool(identifier: string, action: string): string {
                        const state = Persistent.states.overlay
                        if (!state || typeof state[identifier] !== "object") return "Unknown tool"
                        const open = Array.from(state.open ?? []).includes(identifier)
                        const wanted = action === "on" ? true : action === "off" ? false : !open
                        state.open = wanted
                            ? Array.from(state.open ?? []).filter(id => id !== identifier).concat([identifier])
                            : Array.from(state.open ?? []).filter(id => id !== identifier)
                        return wanted ? "on" : "off"
                    }
                }

                Connections {
                    target: Config
                    function onReadyChanged() {
                        if (Config.ready && !GlobalStates.shellEntryReady)
                            entryTimer.restart()
                    }
                }
            }
        }
    }
}

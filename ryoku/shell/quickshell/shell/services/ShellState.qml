pragma Singleton
pragma ComponentBehavior: Bound

import Quickshell
import Ryoku.Ui.Singletons
import "lib/screens.js" as Screens

// Shared per-monitor open/close state for every shell surface: the single source
// of truth the CustomShortcut handlers flip and each resident surface binds its
// visibility to, replacing the old per-surface process plus `ryoku-shell state`
// round-trip. One PersistentProperties object per screen (built by Variants over
// Quickshell.screens, the caelestia ScreenState pattern) so a flag set on one
// monitor never leaks to another; a hotplugged monitor gains its own state on the
// fly. Surfaces read forScreen(modelData); keybinds usually target forActive().
Singleton {
    id: root

    // The compositor's screen list deduped to one entry per physical output (see
    // lib/screens.js). A duplicate output announce -- e.g. the modeset a GPU app
    // triggers on first launch -- would otherwise fan two of every per-monitor
    // surface: two bars, two OSDs, two state slices ("the desktop tweaks out").
    // Every consumer (shell.qml, the bar's VariantRoot, the launchers) reads this
    // one list so they agree on one surface set per output. Reactive to
    // Quickshell.screens, so a genuine hotplug still flows through.
    readonly property var screens: Screens.uniqueByName(Quickshell.screens)

    // State for a specific screen, or null before its per-monitor instance is
    // built (a binding can evaluate ahead of screen hotplug). Matched on output
    // name, so a screen the compositor destroyed and recreated still resolves.
    function forScreen(screen) {
        return Screens.sliceForScreen(states.instances, screen);
    }

    // State for the focused output; falls back to the first screen so a caller
    // before focus is known still gets a live target.
    function forActive() {
        const slice = Screens.sliceForName(states.instances, Wm.focusedOutput);
        if (slice)
            return slice;
        const list = states.instances;
        return list.length > 0 ? list[0] : null;
    }

    // --- Session-action confirmation (contract 13 sec 2c, 8) ---------------
    // A frame-bar logout/reboot/shutdown click asks for confirmation on its own
    // monitor; the per-screen RyokuConfirmationDialog runs the action through
    // SessionActions only on the positive press. Ported from the reference pill
    // root (shell.qml 60-78); singleton-level so any monitor can raise it.
    property string sessionAction: ""            // "" | "logout" | "reboot" | "shutdown"
    property string sessionActionMonitor: ""
    readonly property var sessionCopy: ({
        "logout":   { message: I18n.tr("Are you sure you want to log out?"),  positive: I18n.tr("Logout") },
        "reboot":   { message: I18n.tr("Are you sure you want to reboot?"),   positive: I18n.tr("Reboot") },
        "shutdown": { message: I18n.tr("Are you sure you want to shut down?"), positive: I18n.tr("Shutdown") }
    })
    readonly property string sessionMessage: root.sessionAction !== "" ? root.sessionCopy[root.sessionAction].message : ""
    readonly property string sessionPositive: root.sessionAction !== "" ? root.sessionCopy[root.sessionAction].positive : ""

    function askSessionAction(id, mon) {
        root.sessionActionMonitor = (mon && mon !== "") ? mon
            : (root.screens.length > 0 ? root.screens[0].name : "");
        root.sessionAction = id;
    }
    function clearSessionAction() {
        root.sessionAction = "";
        root.sessionActionMonitor = "";
    }

    // --- Surface-request bus ----------------------------------------------
    // Daemon- and root-driven surface prompts reach the owning monitor's
    // FrameMenuManager through these singleton signals. Each Frame connects and
    // opens/closes only when the request targets its screen (mon "" broadcasts to
    // every monitor, matching the reference). Ported from the reference pill root
    // (shell.qml 32-35, 317-339).
    signal surfaceRequested(string id, string mon, var context)
    signal surfaceClosed(string id, string mon)
    signal keyringPromptChanged(int promptId)
    function requestSurface(id, mon, context) { root.surfaceRequested(id, mon, context); }
    function closeSurface(id, mon) { root.surfaceClosed(id, mon); }

    // Open a surface on the focused monitor: the menu global-shortcut handlers
    // call this so a keybind lands on the active screen, matching the old
    // `ryoku-shell menu <id>` which routed to the daemon's activeMonitor.
    function requestSurfaceActive(id, context) {
        root.surfaceRequested(id, Wm.focusedOutput, context);
    }

    // Keyboard-return bounce bridge. A dismissed keyboard surface (the per-monitor
    // FrameSurfaceLifecycle in Frame.qml) pulses the single root-level kbBounce
    // helper in shell.qml to hand the keyboard back to the compositor. The signal
    // lives here because the lifecycle is per-monitor while kbBounce is one shared
    // window; shell.qml owns the pulse, Frame.qml calls restoreFocus().
    signal focusRestoreRequested()
    function restoreFocus() { root.focusRestoreRequested(); }


    Variants {
        id: states
        model: root.screens

        PersistentProperties {
            id: slice
            required property var modelData

            // Surface toggles, one per today's IpcHandler target so each keybind
            // becomes an in-process flip:
            property bool launcherOpen: false           // launcher
            property bool overviewOpen: false           // overview (Super+Tab expo)
            property bool clipboardOpen: false          // clipboard overlay (Super+V)

            // The frame bar's master reveal for this monitor. Resting policy is
            // revealed: each edge then follows its Config reveal flag, and the
            // bar toggle shortcut flips this to show or hide every edge at once.
            property bool barRevealed: true

            // The visualiser's layer on this monitor. Whether it runs at all is the
            // persisted Config.enabled, so a restart and the Hub switch agree with it.
            property bool visualizerOverlay: false

            // Placement takes the pointer over the look's own box. Leaving it hands
            // the layer back, so the spectrum drops behind windows again unless the
            // overlay is what the user chose.
            property bool visualizerPlacing: false

            // A place for the on-screen-display and notification surfaces to
            // signal activity when they migrate (Phase 5).
            property bool osdVisible: false
            property bool notificationsVisible: false
        }
    }
}

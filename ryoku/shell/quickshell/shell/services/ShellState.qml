pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import "lib/screens.js" as Screens
import "../modules/kit/lib/route.js" as KitRoute

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
    // one list so they agree on one surface set per output.
    //
    // The list only changes when the SET of outputs changes. QtWayland inserts a
    // nameless 0x0 placeholder while no output exists, so an unplug/replug can
    // signal screens several times with the same real outputs underneath; each
    // signal would hand every per-screen Variants a fresh array and rebuild all
    // of it. Those rebuilds are what crashed the shell on monitor power-off
    // (#312, upstream quickshell#796), so the placeholder churn is filtered out
    // here and only a genuine output change is published.
    // A plain property, not a binding: a binding re-evaluates on every screens
    // signal and hands every consumer a fresh array regardless of what changed
    // -- the churn being filtered. Only the settle timer below writes it.
    property var screens: []

    Component.onCompleted: root.screens = Screens.uniqueByName(Quickshell.screens)

    Connections {
        target: Quickshell
        function onScreensChanged() { settleScreens.restart(); }
    }

    // Rebuild per-monitor surfaces once the screen list settles, not on every
    // signal of a hotplug storm. QtWayland re-signals screens several times
    // while outputs are coming and going (a nameless placeholder is added,
    // then removed), and each signal used to rebuild every Variants inside
    // the teardown window -- the rebuild that crashed the shell when monitors
    // were powered off (#312, upstream quickshell#796). Settling also makes a
    // quick off/on cancel out: the list is unchanged by the time the timer
    // fires, so nothing rebuilds at all.
    Timer {
        id: settleScreens
        interval: 350
        onTriggered: {
            const next = Screens.uniqueByName(Quickshell.screens);
            if (Screens.sameOutputs(root.screens, next))
                return;
            root.screens = next;
        }
    }

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
    // Frame menu managers mounted by the active style; each one draws surface
    // requests such as the polkit island, so the shell's own prompt stands down.
    property int surfaceHosts: 0
    signal surfaceRequested(string id, string mon, var context)
    signal surfaceClosed(string id, string mon)
    signal keyringPromptChanged(int promptId)

    function sliceForMonitor(mon) {
        if (mon && typeof mon === "object")
            return root.forScreen(mon);
        if (typeof mon === "string" && mon !== "") {
            const named = Screens.sliceForName(states.instances, mon);
            if (named)
                return named;
        }
        return root.forActive();
    }

    function setAskMode(screen, mode) {
        if (mode !== "ask" && mode !== "web")
            return;
        const slice = root.sliceForMonitor(screen);
        if (!slice)
            return;
        slice.askMode = mode;
    }

    function closeTransientSurfaces(mon) {
        const slice = root.sliceForMonitor(mon);
        if (!slice)
            return;
        slice.askOpen = false;
        root.closeKit();
    }

    function kitCommand(mon) {
        const name = mon && typeof mon === "object" ? mon.name
            : typeof mon === "string" && mon !== "" ? mon : Wm.focusedOutput;
        return ["ryoku-kit", "--qml", Quickshell.shellPath("kit.qml"), "--monitor", name || ""];
    }

    function openKit(mon, section, page, args) {
        const slice = root.sliceForMonitor(mon);
        if (slice)
            slice.askOpen = false;
        const command = root.kitCommand(mon);
        if (args)
            command.push("--args", JSON.stringify(args));
        if (section)
            command.push(section, page || "");
        Spawn.run(command);
        root.surfaceClosed("sidebar-left", slice ? slice.modelData.name : "");
    }

    function toggleKit(mon) {
        const slice = root.sliceForMonitor(mon);
        if (slice)
            slice.askOpen = false;
        Spawn.run(root.kitCommand(mon).concat(["--toggle"]));
        root.surfaceClosed("sidebar-left", slice ? slice.modelData.name : "");
    }

    function closeKit() {
        Spawn.run(["qs", "-p", Quickshell.shellPath("kit.qml"), "ipc", "call", "kit", "close"]);
    }

    function openKitDownloads(mon, args) {
        const slice = root.sliceForMonitor(mon);
        if (slice)
            root.openKit(mon, "downloads", "queue", args);
    }

    function routeOwnedSurface(id, mon) {
        const split = id.indexOf("#");
        const base = split >= 0 ? id.substring(0, split) : id;
        const route = split >= 0 ? id.substring(split + 1) : "";
        const slice = root.sliceForMonitor(mon);
        if (!slice)
            return false;

        // cheatsheet#<section>[/<page>] (ryoku-kit) opens Kit there and never
        // closes it. The bare id is the toggle, which a bar style may own
        // instead, so ShellBody decides that one.
        const kit = KitRoute.parse(id);
        if (kit.kit) {
            if (kit.toggle)
                return false;
            root.openKit(mon, kit.section, kit.page);
            return true;
        }

        if (base !== "ask")
            return false;

        if (route === "" && slice.askOpen) {
            slice.askOpen = false;
            return true;
        }

        slice.askMode = route === "web" ? "web" : "ask";
        slice.askOpen = true;
        root.closeKit();
        root.surfaceClosed("sidebar-left", slice.modelData.name);
        return true;
    }

    function requestSurface(id, mon, context) {
        const value = id || "";
        const base = value.split("#")[0];
        if (base === "sidebar-left")
            root.closeTransientSurfaces(mon);
        else
            root.routeOwnedSurface(value, mon);
        root.surfaceRequested(value, mon || "", context);
    }

    function closeSurface(id, mon) {
        const base = (id || "").split("#")[0];
        const slice = root.sliceForMonitor(mon);
        if (slice && (base === "" || base === "ask"))
            slice.askOpen = false;
        if (base === "" || base === KitRoute.BASE)
            root.closeKit();
        root.surfaceClosed(id, mon);
    }

    // Open a desktop widget's right-click menu from off-surface (a keybind, the
    // daemon, or a verification harness). niri routes context menus differently
    // from Hyprland, so the shell exposes the menu as an addressable surface
    // rather than relying on a pointer event landing on the tile.
    signal widgetMenuRequested(string mon, string widget)
    function requestWidgetMenu(mon, widget) { root.widgetMenuRequested(mon, widget); }

    // Open a desktop widget's inspector (the Customize sheet) from off-surface,
    // the same addressable routing as the right-click menu above.
    signal widgetCustomizeRequested(string mon, string widget)
    function requestWidgetCustomize(mon, widget) { root.widgetCustomizeRequested(mon, widget); }

    // Open a surface on the focused monitor: the menu global-shortcut handlers
    // call this so a keybind lands on the active screen, matching the old
    // `ryoku-shell menu <id>` which routed to the daemon's activeMonitor.
    //
    // context travels to the surface untouched, the way requestSurface already
    // carries the keyring's prompt id. A caller that has a trigger on screen
    // passes { ownerRect: <global rect> } so the surface opens welded to it
    // instead of taking its config anchor: that is what lets a bar style open a
    // shell-owned menu under its own widget rather than in a screen corner.
    function requestSurfaceActive(id, screenName, context) {
        const monitor = typeof screenName === "string" && screenName !== ""
            ? screenName : Wm.focusedOutput;
        root.requestSurface(id, monitor, context);
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
            property bool askOpen: false
            property string askMode: "ask"

            // The frame bar's master reveal for this monitor. Resting policy is
            // revealed: each edge then follows its Config reveal flag, and the
            // bar toggle shortcut flips this to show or hide every edge at once.
            property bool barRevealed: true

            // The visualiser's layer on this monitor. Whether it runs at all is the
            // persisted Config.enabled, so a restart and the Hub switch agree with it.
            property bool visualizerOverlay: false

            // A place for the on-screen-display and notification surfaces to
            // signal activity when they migrate (Phase 5).
            property bool osdVisible: false
            property bool notificationsVisible: false
        }
    }
}

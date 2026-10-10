//@ pragma UseQApplication
// Threaded render loop: the frame's blob melt is a per-frame spring plus live
// scene-graph MultiEffects (menu glass blur, blobs); threaded is vsync-locked and
// frees the GUI thread so those effects get regular frame deltas and never ghost
// or stutter. `basic` idled a touch cheaper on NVIDIA but smeared the live effects
// when switching menu pages, so this matches the pill's proven render setup.
//@ pragma DefaultEnv QSG_RENDER_LOOP=threaded
//@ pragma DefaultEnv QS_DROP_EXPENSIVE_FONTS=1
//@ pragma DefaultEnv QT_QUICK_FLICKABLE_WHEEL_DECELERATION=10000
pragma ComponentBehavior: Bound

import Quickshell
import shell.services
import "modules/visualizer/Singletons" as VizCfg
import "modules/stage/Singletons" as StageCfg
import "modules/stage" as StageEditor
import "components"
import "modules/wallpaper"
import "modules/wallpaper/Singletons" as WallCfg
import "modules/desktop"
import "modules/visualizer"
import "modules/bar"
import "modules/sidebar"
import "modules/dock"
import "modules/launcher"
import "modules/overview"
import "modules/clipboard"
import QtQuick
import Quickshell.Io
import Quickshell.Wayland
import "modules/osd"
import "modules/notifications"
import "modules/capture"
import "modules/ask"
import "modules/confirm"
import Ryoku.Ui.Singletons

/**
 * The single resident Ryoku shell instance.
 *
 * One ShellRoot for the whole desktop. It brings the shared service singletons
 * online, holds the per-monitor ShellState every surface binds its visibility
 * to, and registers the shell's in-QML global shortcuts. Each monitor
 * gets one Scope carrying that screen's ShellState slice (st); every migrated
 * surface is instantiated once inside it and binds its screen and its visibility
 * to the slice, so a keybind that flips a flag on the active monitor reveals or
 * hides exactly this monitor's copy in-process, where the old shell spawned a
 * ryoku-shell client per press across separate surface processes.
 *
 * The ryoku-shell daemon launches this instance as `qs -c shell`, the live
 * desktop. Where the compositor offers a global-shortcuts protocol its binds
 * dispatch straight to the shortcuts registered here.
 *
 * UseQApplication is declared once for the whole shell (the tray needs Qt
 * Widgets), replacing the six per-surface copies the old multi-process shell paid.
 */
ShellRoot {
    id: root
    // One deferred-build wave for the cheap event-driven surfaces (the OSDs and
    // the notification column), armed 1.5 s into boot so a keypress or an
    // arriving toast never waits on a component build. Everything heavier is
    // built on its own first open and torn down a grace period after every
    // close, so nothing parses, instantiates or commits for a surface nobody
    // asked for. The grace is what protects the animations: a surface is only
    // ever destroyed while it sits closed.
    property int warm: 0
    Timer { interval: 1500; running: true; repeat: false; onTriggered: root.warm = 1 }

    // Construct the shared services (ShellState's per-monitor state now, heavier
    // providers as surfaces migrate) at load rather than on the first keybind.
    ServiceLoader {
        services: [ShellState, ScreenTime, Keypresses, KeyboardLayout]
    }

    readonly property string reloadStatePath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-reload-cover.json"
    readonly property string reloadCoverBin: Quickshell.env("RYOKU_SHELL_DIR")
        ? Quickshell.env("RYOKU_SHELL_DIR") + "/scripts/ryoku-reload-cover"
        : "ryoku-reload-cover"
    property string reloadToken: ""
    property bool reloadFinishSent: false
    property bool reloadReleaseArmed: false
    property var reloadScreens: ({})
    function setReloadScreenReady(name: string, ready: bool): void {
        var next = ({});
        for (var key in reloadScreens)
            next[key] = reloadScreens[key];
        next[name] = ready;
        reloadScreens = next;
        finishReloadCover();
    }
    function finishReloadCover(): void {
        if (!reloadToken || reloadFinishSent)
            return;
        for (var i = 0; i < Quickshell.screens.length; i++) {
            var screen = Quickshell.screens[i];
            if (!screen || !reloadScreens[screen.name])
                return;
        }
        if (!reloadReleaseArmed) {
            reloadReleaseArmed = true;
            reloadHold.restart();
        }
    }

    FileView {
        id: reloadState
        path: root.reloadStatePath
        blockLoading: true
        printErrors: false
        // One read at load, never again: a shell that was already up when a
        // reload armed the cover must not steal the next instance's finish by
        // re-reading the token mid-teardown.
        watchChanges: false
        onLoaded: {
            try {
                const token = JSON.parse(text() || "{}").token;
                root.reloadToken = typeof token === "string" && /^[0-9a-f]{32}$/.test(token) ? token : "";
            } catch (error) {
                root.reloadToken = "";
            }
            root.finishReloadCover();
        }
    }
    Timer {
        id: reloadHold
        interval: 1500
        onTriggered: {
            if (!root.reloadToken || root.reloadFinishSent)
                return;
            root.reloadFinishSent = true;
            reloadFinish.command = [root.reloadCoverBin, "finish", root.reloadToken];
            reloadFinish.running = true;
        }
    }

    Process {
        id: reloadFinish
    }

    // Power Saver strips compositor blur and shadow too (the heaviest present-time
    // GPU cost), reusing the decoration.lua path lowPowerMode already takes. Perf
    // folds the active power profile into its switches; mirror the profile-driven
    // "saver" flag to a cache the compositor reads, and reload it when it flips
    // so it re-reads the value live. Seeded once at load with no reload
    // (login already parsed the right value); only a later profile change reloads.
    FileView {
        id: hyprPerf
        path: (Quickshell.env("XDG_CACHE_HOME") || (Quickshell.env("HOME") + "/.cache")) + "/ryoku/hypr-perf.json"
        printErrors: false
        property bool armed: false
        onSaved: if (hyprPerf.armed) Wm.reloadConfig("")
        JsonAdapter { id: hyprPerfA; property bool saver: false }
        Component.onCompleted: {
            hyprPerfA.saver = Perf.saver;
            hyprPerf.writeAdapter();
        }
    }
    Connections {
        target: Perf
        function onSaverChanged() {
            hyprPerf.armed = true;
            hyprPerfA.saver = Perf.saver;
            hyprPerf.writeAdapter();
        }
    }

    // One per-monitor surface stack. Each screen gets a Scope carrying its
    // ShellState slice (st); every resident surface binds its screen and its
    // visibility to that slice, so flipping a flag on the active monitor reveals
    // or hides this monitor's copy. The order here reads top-to-bottom only; the
    // Wayland layer each surface maps on decides the real stacking.
    Variants {
        model: ShellState.screens

        Scope {
            id: perScreen
            required property var modelData
            readonly property var st: ShellState.forScreen(modelData)
            readonly property bool reloadReady: wallpaper.reloadReady && desktop.reloadReady
            onReloadReadyChanged: root.setReloadScreenReady(modelData.name, reloadReady)
            Component.onCompleted: root.setReloadScreenReady(modelData.name, reloadReady)

            // Always-on backdrop and desktop widget layer.
            Wallpaper {
                id: wallpaper
                screen: perScreen.modelData
            }
            Desktop {
                id: desktop
                screen: perScreen.modelData
                active: true
                widgetsEnabled: Tokens.widgetsEnabledFor(perScreen.modelData.name)
                wallpaperUrl: wallpaper.wallpaperUrl
                wallpaperPath: wallpaper.wallpaperPath
                wallpaperFit: wallpaper.fit
                wallpaperTransition: wallpaper.transition
                videoUrl: wallpaper.videoUrl
                wallpaperLive: wallpaper.live
                videoMuted: wallpaper.videoMuted
                videoVolume: wallpaper.videoVolume
                videoPaused: wallpaper.videoPaused
            }

            // A blurred copy of the wallpaper for the compositor's overview
            // backdrop, mapped below the desktop so it shows only in the
            // overview. Built only where the capability and the user's setting
            // both ask for it; a box with the backdrop off never pays for it.
            LazyLoader {
                id: backdropLoader
                activeAsync: Wm.caps.overviewBackdrop === true && WallCfg.OverviewBackdropConfig.enabled
                OverviewBackdrop {
                    screen: perScreen.modelData
                    available: Wm.caps.overviewBackdrop === true
                    overviewOpen: Wm.overviewOpen
                    wallpaperUrl: wallpaper.wallpaperUrl
                }
            }

            // Stage now renders entirely inside the desktop surface (one stack:
            // backdrop, layers, widgets), so there is no separate Background
            // surface here (docs/stage.md). A silent visualizer is unloaded on
            // every output when the performance policy asks for it; live audio
            // re-creates the surfaces immediately. Placement runs in the Stage
            // Editor now: the desktop hosts the look while the mode frames this
            // monitor, so this surface exists for the plain and overlay cases only.
            LazyLoader {
                id: vizLoader
                activeAsync: VizCfg.Config.enabled && Perf.visualizerResident
                Visualizer {
                    id: perScreenViz
                    screen: perScreen.modelData
                    mode: !VizCfg.Config.enabled ? "off"
                        : (perScreen.st && perScreen.st.visualizerOverlay ? "overlay" : "desktop")
                    // The desktop hosts the visualizer behind the cut-outs while the
                    // stage is on, and inside the lifted desktop while the Stage
                    // Editor frames this monitor; this surface steps aside (cava
                    // keeps running). A rebuilt monitor stack (DPMS off/on, a lid
                    // close/open) starts with no wallpaper frame, so
                    // hostsVisualizer briefly reads false even for a stage wallpaper:
                    // stay suppressed until the frame lands, or the surface maps
                    // full-screen and pulses for a few seconds on every resume.
                    suppressed: desktop.hostsVisualizer || !wallpaper.reloadReady
                }
            }

            // The frame bar (Phase 2): reads its own reveal from this slice.
            Frame {
                modelData: perScreen.modelData
                // park the record island flush beside this monitor's dock band
                dockLaneEdge: dockLoader.item ? dockLoader.item.edge : ""
                dockLaneSize: dockLoader.item ? dockLoader.item.bandSize : 0
                dockLaneCenter: dockLoader.item ? dockLoader.item.bandCenter : 0
            }

            LazyLoader {
                id: controlsLoader
                property bool open: SidebarState.isOpen(perScreen.modelData)
                property bool showNow: false
                activeAsync: open || controlsHold.running
                onItemChanged: if (item) Qt.callLater(() => controlsLoader.showNow = controlsLoader.open)
                onOpenChanged: {
                    showNow = open && item !== null;
                    if (!open && active) controlsHold.restart();
                }
                Sidebar {
                    screen: perScreen.modelData
                    active: controlsLoader.showNow
                }
            }
            Timer { id: controlsHold; interval: SidebarState.exitDuration + 48 }


            LazyLoader {
                id: askLoader
                property bool open: perScreen.st ? perScreen.st.askOpen : false
                property bool showNow: false
                activeAsync: open || askHold.running
                onItemChanged: if (item) Qt.callLater(() => askLoader.showNow = askLoader.open)
                onOpenChanged: {
                    showNow = open && item !== null;
                    if (!open && active) askHold.restart();
                }
                AskBar {
                    screen: perScreen.modelData
                    active: askLoader.showNow
                    mode: perScreen.st ? perScreen.st.askMode : "ask"
                    tool: perScreen.st ? perScreen.st.askTool : ""
                    onRequestClose: if (perScreen.st) perScreen.st.askOpen = false
                    onModeChangeRequested: mode => ShellState.setAskMode(perScreen.modelData, mode)
                }
            }
            Timer { id: askHold; interval: 15000 }

            AskBubble {
                screen: perScreen.modelData
                enabled: Config.askBubble.enabled
                    && Config.askBubble.screen === perScreen.modelData.name
            }

            // One cheap host per monitor loads only the selected dock design.
            // The surface itself remains independent of the active bar style.
            // The dock stays through the Stage Editor session (like the bar):
            // the dock catalogue edits it live, and the mode's viewport clears
            // the edge it occupies (GlobalStates.dockInsets).
            UniversalDockHost {
                id: dockLoader
                screen: perScreen.modelData
            }

            // The dock's right-click context menu: a full-screen overlay on the
            // monitor that owns the open menu (the thin dock strip cannot host it).
            LazyLoader {
                id: dockMenuLoader
                property bool open: Dock.design === "ryoku" && Dock.menuOpen && Dock.menuScreen === perScreen.modelData.name
                activeAsync: open || dockMenuHold.running
                onOpenChanged: if (!open && active) dockMenuHold.restart()
                DockMenuOverlay {
                    screen: perScreen.modelData
                }
            }
            Timer { id: dockMenuHold; interval: 2000 }

            // Toggle-driven overlays: built on first open (async, so the key
            // press never blocks on a component build) and destroyed 15 s after
            // every close, once the slide-out has long finished.
            // The inner surface must be BORN closed and opened one tick later:
            // its reveal is driven by an active-change, and an item created with
            // the flag already true would never animate (or show) at all.
            LazyLoader {
                id: launcherLoader
                property bool open: perScreen.st ? perScreen.st.launcherOpen : false
                property bool showNow: false
                activeAsync: open || launcherHold.running
                onItemChanged: if (launcherLoader.item) Qt.callLater(function() { launcherLoader.showNow = launcherLoader.open; })
                onOpenChanged: {
                    if (launcherLoader.open) {
                        if (launcherLoader.item) launcherLoader.showNow = true;
                        return;
                    }
                    launcherLoader.showNow = false;
                    if (launcherLoader.active) launcherHold.restart();
                }
                Launcher {
                    screen: perScreen.modelData
                    active: launcherLoader.showNow
                    onRequestClose: if (perScreen.st) perScreen.st.launcherOpen = false
                }
            }
            Timer { id: launcherHold; interval: 15000 }
            LazyLoader {
                id: overviewLoader
                property bool open: perScreen.st ? perScreen.st.overviewOpen : false
                property bool showNow: false
                activeAsync: open || overviewHold.running
                onItemChanged: if (overviewLoader.item) Qt.callLater(function() { overviewLoader.showNow = overviewLoader.open; })
                onOpenChanged: {
                    if (overviewLoader.open) {
                        if (overviewLoader.item) overviewLoader.showNow = true;
                        return;
                    }
                    overviewLoader.showNow = false;
                    if (overviewLoader.active) overviewHold.restart();
                }
                OverviewSurface {
                    screen: perScreen.modelData
                    active: overviewLoader.showNow
                    onRequestClose: if (perScreen.st) perScreen.st.overviewOpen = false
                }
            }
            Timer { id: overviewHold; interval: 15000 }
            LazyLoader {
                id: clipboardLoader
                property bool open: perScreen.st ? perScreen.st.clipboardOpen : false
                property bool showNow: false
                activeAsync: open || clipboardHold.running
                onItemChanged: if (clipboardLoader.item) Qt.callLater(function() { clipboardLoader.showNow = clipboardLoader.open; })
                onOpenChanged: {
                    if (clipboardLoader.open) {
                        if (clipboardLoader.item) clipboardLoader.showNow = true;
                        return;
                    }
                    clipboardLoader.showNow = false;
                    if (clipboardLoader.active) clipboardHold.restart();
                }
                ClipboardSurface {
                    screen: perScreen.modelData
                    active: clipboardLoader.showNow
                    onRequestClose: if (perScreen.st) perScreen.st.clipboardOpen = false
                }
            }
            Timer { id: clipboardHold; interval: 15000 }
            // Shell-wide per-monitor surfaces. The OSDs and the popup column are
            // small and event-driven (a keypress or an arriving toast must never
            // wait on a build), so they join the first wave and stay resident.
            // The capture overlays are per-flow: built on the flow's first
            // signal, kept across the flow's pauses, dropped 20 s after it ends.
            LazyLoader {
                id: osdVolumeLoader
                activeAsync: root.warm >= 1
                OsdWindow {
                    modelData: perScreen.modelData
                    kind: "volume"
                }
            }
            LazyLoader {
                id: osdMicLoader
                activeAsync: root.warm >= 1
                OsdWindow {
                    modelData: perScreen.modelData
                    kind: "mic"
                }
            }
            LazyLoader {
                id: osdBrightnessLoader
                activeAsync: root.warm >= 1
                OsdWindow {
                    modelData: perScreen.modelData
                    kind: "brightness"
                }
            }
            LazyLoader {
                id: osdKeyboardLoader
                activeAsync: root.warm >= 1
                KeyboardOsdWindow {
                    modelData: perScreen.modelData
                }
            }
            LazyLoader {
                id: notifsLoader
                activeAsync: root.warm >= 1 || Notifs.popups.length > 0
                NotificationPopups {
                    modelData: perScreen.modelData
                }
            }
            LazyLoader {
                id: regionLoader
                property bool open: Recorder.anyActive
                activeAsync: open || regionHold.running
                onOpenChanged: if (!open && active) regionHold.restart()
                RegionOverlay {
                    modelData: perScreen.modelData
                }
            }
            Timer { id: regionHold; interval: 20000 }
            LazyLoader {
                id: captureLoader
                property bool open: Capture.selecting !== ""
                activeAsync: open || captureHold.running
                onOpenChanged: if (!open && active) captureHold.restart()
                CaptureOverlay {
                    modelData: perScreen.modelData
                }
            }
            Timer { id: captureHold; interval: 20000 }
            LazyLoader {
                id: cameraLoader
                property bool open: Camera.active
                activeAsync: open || cameraHold.running
                onOpenChanged: if (!open && active) cameraHold.restart()
                CameraOverlay {
                    modelData: perScreen.modelData
                }
            }
            Timer { id: cameraHold; interval: 20000 }
            LazyLoader {
                id: keypressLoader
                property bool open: Keypresses.active
                activeAsync: open || keypressHold.running
                onOpenChanged: if (!open && active) keypressHold.restart()
                KeypressOverlay {
                    modelData: perScreen.modelData
                }
            }
            Timer { id: keypressHold; interval: 20000 }
            // The floating recording island: the one recording HUD on every bar
            // style, a standalone layer-shell surface rather than frame chrome. It
            // shows on the output being recorded -- the screen holding the region
            // box, else the focused output, else the first screen -- so a
            // multi-monitor setup gets one island on the right screen.
            LazyLoader {
                id: recordIslandLoader
                readonly property bool isTarget: {
                    var s = perScreen.modelData;
                    var m = /^(\d+)x(\d+)\+(\d+)\+(\d+)$/.exec(Recorder.regionGeom);
                    if (m) {
                        var ox = parseInt(m[3]);
                        var oy = parseInt(m[4]);
                        return ox >= s.x && ox < s.x + s.width && oy >= s.y && oy < s.y + s.height;
                    }
                    if (Wm.focusedOutput && Wm.focusedOutput.length > 0)
                        return s.name === Wm.focusedOutput;
                    return ShellState.screens.length > 0 && ShellState.screens[0].name === s.name;
                }
                property bool open: isTarget && (Recorder.anyActive || Recorder.countingDown)
                activeAsync: open || recordIslandHold.running
                onOpenChanged: if (!open && active) recordIslandHold.restart()
                RecordIsland {
                    modelData: perScreen.modelData
                }
            }
            Timer { id: recordIslandHold; interval: 20000 }
            // Shown only on the monitor whose frame bar raised it; the positive
            // button runs the power action through the daemon, then clears.
            LazyLoader {
                id: confirmLoader
                property bool open: ShellState.sessionAction !== ""
                activeAsync: open || confirmHold.running
                onOpenChanged: if (!open && active) confirmHold.restart()
                RyokuConfirmationDialog {
                    modelData: perScreen.modelData
                    action: ShellState.sessionActionMonitor === perScreen.modelData.name ? ShellState.sessionAction : ""
                    message: ShellState.sessionMessage
                    positiveLabel: ShellState.sessionPositive
                    negativeLabel: I18n.tr("Cancel")
                    onConfirmed: a => { SessionActions.run(a); ShellState.clearSessionAction(); }
                    onCancelled: ShellState.clearSessionAction()
                }
            }
            Timer { id: confirmHold; interval: 5000 }
        }
    }

    // The Stage Editor: the ported desktop chrome, mounted once (it is
    // per-screen inside) and driven by the Edit widgets session. Built only
    // while that session is on (docs/stage.md).
    StageEditor.StageEditorHost {
    }


    // The single surface-toggle mapping. Every shell surface id resolves to one
    // transition here: a per-monitor ShellState flip, a global config toggle, or
    // a request onto the shared surface bus. A CustomShortcut press and the
    // `ryoku-shell <id>` fallback both end at the same transition, whether the
    // surface is hosted by the frame or by a sidebar.
    function toggleSurface(id) {
        const st = ShellState.forActive();
        switch (id) {
        case "barToggle":
            if (st)
                st.barRevealed = !st.barRevealed;
            break;
        case "launcher":
            if (st)
                st.launcherOpen = !st.launcherOpen;
            break;
        case "overview":
            if (Wm.caps.nativeOverview)
                Wm.toggleOverview();
            else if (Wm.caps.windowGeometry && st)
                st.overviewOpen = !st.overviewOpen;
            break;
        case "visualizer":
            VizCfg.Config.setEnabled(!VizCfg.Config.enabled);
            break;
        case "visualizer-overlay":
            if (st)
                st.visualizerOverlay = !st.visualizerOverlay;
            break;
        case "visualizer-place":
            root.placeVisualizer(!StageCfg.StageSession.active);
            break;
        case "quicksettings":
            ShellState.requestSurfaceActive("sidebar-left", undefined);
            break;
        case "wallpaper-menu":
            ShellState.requestSurfaceActive("wallpaper", undefined);
            break;
        case "clipboard":
            if (st)
                st.clipboardOpen = !st.clipboardOpen;
            break;
        case "screenshot":
            Quickshell.execDetached(["sh", "-c", "flock -n -o /tmp/ryoshot.lock qs -c ryoshot"]);
            break;
        case "ask":
            ShellState.requestSurfaceActive("ask", undefined);
            break;
        case "compress":
            ShellState.requestSurfaceActive("ask#tools/compress", undefined);
            break;
        case "install":
            ShellState.requestSurfaceActive("ask#tools/install", undefined);
            break;
        }
    }

    // With no global-shortcuts protocol a keybind reaches the shell by spawning
    // `ryoku-shell <id>`. Shell-wide flag surfaces are handled here; frame menus
    // and sidebars consume the same surfaceRequested bus in their per-monitor
    // hosts, so each request still has exactly one owner.
    Connections {
        target: ShellState
        function onSurfaceRequested(id, mon, ctx) {
            switch (id) {
            case "barToggle":
            case "launcher":
            case "overview":
            case "clipboard":
            case "screenshot":
            case "visualizer":
            case "visualizer-overlay":
            case "visualizer-place":
                root.toggleSurface(id);
                break;
            }
        }
    }

    // In-process global shortcuts. Each dispatches to the one toggleSurface
    // mapping above, so the compositor's global-shortcut bind and a `ryoku-shell`
    // spawn drive the identical transition. Names match the compositor binds.
    CustomShortcut {
        name: "barToggle"
        description: I18n.tr("Toggle the Ryoku frame bar on the active monitor")
        onPressed: root.toggleSurface("barToggle")
    }
    CustomShortcut {
        name: "launcher"
        description: I18n.tr("Toggle the app launcher on the active monitor")
        onPressed: root.toggleSurface("launcher")
    }
    CustomShortcut {
        name: "overview"
        description: I18n.tr("Toggle the workspace overview on the active monitor")
        onPressed: root.toggleSurface("overview")
    }
    // On/off is the persisted key, so the keybind, the Hub switch and the next
    // restart all read the same answer. Only the layer is per-monitor memory.
    CustomShortcut {
        name: "visualizer"
        description: I18n.tr("Cycle the desktop audio visualiser off and on")
        onPressed: root.toggleSurface("visualizer")
    }
    CustomShortcut {
        name: "visualizer-overlay"
        description: I18n.tr("Toggle the audio visualiser overlay over windows")
        onPressed: root.toggleSurface("visualizer-overlay")
    }
    CustomShortcut {
        name: "visualizer-place"
        description: I18n.tr("Grab the audio visualiser's ring or orb and drag it into place")
        onPressed: root.toggleSurface("visualizer-place")
    }

    // Aiming a look happens in the Stage Editor now (docs/stage.md): the bind,
    // the desktop menu row and the Hub hand-off all enter the edit session on
    // the active monitor, where the look wears its placement grip like every
    // other widget. A hidden spectrum aims nothing, so placing it shows it
    // first. The editor's own Done/Escape leaves the session, so `off` only
    // unwinds a session that is open.
    function placeVisualizer(on) {
        const st = ShellState.forActive();
        if (!st)
            return;
        const mon = (st.modelData && st.modelData.name) ? st.modelData.name : "";
        if (!on) {
            if (StageCfg.StageSession.active)
                StageCfg.StageSession.leave();
            return;
        }
        if (!VizCfg.Config.enabled)
            VizCfg.Config.setEnabled(true);
        StageCfg.StageSession.enterWidgets(mon, "visualizer");
    }

    // --- Root machinery (ported from the reference pill root) --------------

    // Bring the durable services online and prewarm the slow scans so the first
    // open of each surface is instant. Ported from pill/shell.qml 170-194:
    // device restore + ddc prewarm and re-arming the persisted Keep-Awake /
    // Game Mode external inhibitors.
    Component.onCompleted: {
        Devices.restore();
        root.syncCaffeine(Flags.keepAwake ? "start" : "stop");
        if (Flags.gameMode)
            root.syncGameMode("start");
        Devices.probeDisplays();
    }

    // Keep-Awake's durable inhibitor lives outside the shell so it survives a
    // reload/restart: ryoku-cmd-caffeine runs systemd-inhibit independent of our
    // lifetime, while the Wayland IdleInhibitor below only gives compositor-level
    // effect. Every surface toggle just flips Flags.keepAwake. (pill 246-257)
    function syncCaffeine(action) {
        Quickshell.execDetached(["ryoku-cmd-caffeine", action]);
    }
    Connections {
        target: Flags
        function onKeepAwakeChanged() {
            root.syncCaffeine(Flags.keepAwake ? "start" : "stop");
        }
    }

    // Game mode's compositor and WiFi tuning lives outside the shell, same shape
    // as Keep-Awake: ryoku-cmd-game-mode (on PATH) drives it so the tuning
    // survives a reload. The deck toggle just flips Flags.gameMode. It is
    // compositor tuning though (Hyprland live config eval), so it only fires
    // where the window manager can run it -- the deck tile and the launcher
    // action hide it there, and this stands down to match instead of running a
    // script that would no-op.
    function syncGameMode(action) {
        if (Wm.caps.liveConfigEval !== true)
            return;
        Quickshell.execDetached(["ryoku-cmd-game-mode", action]);
    }
    Connections {
        target: Flags
        function onGameModeChanged() {
            root.syncGameMode(Flags.gameMode ? "start" : "stop");
        }
    }

    // Do-not-disturb: the notification server suppresses popups while the flag is
    // set (the deck toggle owns the flag). (pill 196-200)
    Binding {
        target: Notifs
        property: "dnd"
        value: Flags.dnd
    }

    // Compositor-level idle inhibitor, mapped only while Keep-Awake is on. It
    // dies with the shell; the external caffeine bridge above keeps the durable
    // one. (pill 202-214)
    PanelWindow {
        id: inhibitWin
        visible: Flags.keepAwake
        implicitWidth: 1
        implicitHeight: 1
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        WlrLayershell.layer: WlrLayer.Background
        WlrLayershell.namespace: "ryoku-frame-inhibit"
        WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
        anchors { top: true; left: true }
        IdleInhibitor { window: inhibitWin; enabled: Flags.keepAwake }
    }

    // keyboard-return bounce. The frame overlay never unmaps, and dropping an
    // Exclusive grab on a mapped layer strands the keyboard (the window looks
    // active but cannot type). This 1x1 helper takes the grab and unmaps, which
    // makes Hyprland hand the keyboard back. A per-monitor FrameSurfaceLifecycle
    // pulses it through ShellState.focusRestoreRequested when a keyboard surface
    // is dismissed, since the lifecycle is per-monitor but this window is single.
    // (pill 216-237, 304-308)
    property bool kbBounce: false
    Timer {
        id: kbBounceTimer
        interval: 90
        onTriggered: root.kbBounce = false
    }
    PanelWindow {
        id: kbBounceWin
        visible: root.kbBounce
        implicitWidth: 1
        implicitHeight: 1
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.namespace: "ryoku-frame-kbfocus"
        WlrLayershell.keyboardFocus: WlrKeyboardFocus.Exclusive
        anchors { top: true; left: true }
    }
    Connections {
        target: ShellState
        function onFocusRestoreRequested() {
            root.kbBounce = true;
            kbBounceTimer.restart();
        }
    }

    // the self-view is a recording companion: when the last capture stops, clear
    // it so it does not linger. (pill 872-878)
    Connections {
        target: Recorder
        function onAnyActiveChanged() {
            if (!Recorder.anyActive)
                Camera.active = false;
        }
    }

    // The polkit agent streams its prompt on a topic rather than pushing a
    // surface, so raise and drop the island from the state itself. (pill 321-329)
    Connections {
        target: Polkit
        function onActiveChanged() {
            // The iRiS bar style draws this prompt in its own dialog.
            if (Polkit.active) {
                if (Config.barStyle !== "iris")
                    ShellState.requestSurface("polkit", "", undefined);
            } else {
                ShellState.closeSurface("polkit", "");
            }
        }
    }

    // Stash steps a running task's own surface aside when it needs the auth
    // island. (pill 332-335)
    Connections {
        target: Stash
        function onAuthStepAside(mon, id) { ShellState.closeSurface(id, mon); }
    }

    // Daemon-facing surface channel (Phase 10 points the daemon here). Target
    // "shell" avoids the live pill's "pill" target; each call maps onto the
    // ShellState surface bus. Ported from pill/shell.qml 341-364; the SocketServer
    // fast path (pill 402-411) is deliberately omitted so this instance owns no
    // runtime socket and produces zero live side effects alongside the daemon.
    IpcHandler {
        target: "shell"
        // An empty monitor means "wherever focus is": the daemon sends "" when
        // its focused-output cache is cold.
        function openSurface(mon: string, id: string): void {
            if (mon && mon.length > 0)
                ShellState.requestSurface(id, mon, undefined);
            else
                ShellState.requestSurfaceActive(id, undefined);
        }
        function closeSurface(mon: string, id: string): void { ShellState.closeSurface(id, mon); }
        function keyringPrompt(payload: string): void {
            Keyring.apply(payload);
            ShellState.keyringPromptChanged(Keyring.promptId);
            ShellState.requestSurface("keyring", Keyring.mon !== "" ? Keyring.mon
                : (ShellState.screens.length > 0 ? ShellState.screens[0].name : ""),
                { promptId: Keyring.promptId });
        }
        function keyringHide(): void {
            Keyring.clear();
            ShellState.closeSurface("keyring", "");
        }
        function voiceShow(mon: string): void { ShellState.requestSurface("voice", mon, undefined); }
        function voiceOff(mon: string): void { ShellState.requestSurface("voice-off", mon, undefined); }
        function voiceHide(): void { ShellState.closeSurface("voice", ""); }
        function pluginPopout(mon: string, id: string): void { ShellState.requestSurface("plugin:" + id, mon, undefined); }
        function bar(mon: string, id: string): void { ShellState.requestSurface(id, mon, undefined); }
        function closeAllMenus(mon: string): void { ShellState.closeSurface("", mon); }
        function sessionConfirm(mon: string, action: string): void { ShellState.askSessionAction(action, mon); }
    }

    // The Hub's "Place on the desktop" button lands here: a slider cannot let a
    // user drop a ring where they want it, so the shell hands them the shape.
    IpcHandler {
        target: "visualizer"
        function place(): string {
            const st = ShellState.forActive();
            if (!st)
                return "No active display is available";
            root.placeVisualizer(true);
            return "ok";
        }
        function done(): string {
            const st = ShellState.forActive();
            if (!st)
                return "No active display is available";
            root.placeVisualizer(false);
            return "ok";
        }
        function setEnabled(value: bool): string { VizCfg.Config.setEnabled(value); return "ok"; }
        function cycleStyle(direction: int): string {
            if (direction !== -1 && direction !== 1)
                return "Style direction must be -1 or 1";
            VizCfg.Config.cycleStyle(direction);
            return "ok";
        }
        function setFps(value: int): string {
            if ([30, 45, 60].indexOf(value) < 0)
                return "Frame rate must be 30, 45 or 60";
            VizCfg.Config.setFps(value);
            return "ok";
        }
        function setAdaptive(value: bool): string { VizCfg.Config.setAdaptive(value); return "ok"; }
        function setGain(value: real): string {
            if (!isFinite(value) || value < 0.5 || value > 2)
                return "Gain must be between 0.5 and 2";
            VizCfg.Config.setGain(value);
            return "ok";
        }
        function setSmoothing(value: real): string {
            if (!isFinite(value) || value < 0 || value > 1)
                return "Smoothing must be between 0 and 1";
            VizCfg.Config.setSmoothing(value);
            return "ok";
        }
    }

    // Ryoku Settings edits the Stage file through the shell's canonical typed
    // service. The Hub never becomes a second writer for stage.json.
    IpcHandler {
        target: "stage-settings"
        function setQuality(value: string): string {
            if (["draft", "standard", "fine"].indexOf(value) < 0)
                return "Quality must be draft, standard or fine";
            StageCfg.Config.setQuality(value);
            return "ok";
        }
        function setEdge(value: real): string {
            if (!isFinite(value) || value < 0 || value > 1)
                return "Edge softness must be between 0 and 1";
            StageCfg.Config.setEdge(value);
            return "ok";
        }
        function setShadow(value: real): string {
            if (!isFinite(value) || value < 0 || value > 1)
                return "Shadow must be between 0 and 1";
            StageCfg.Config.setShadow(value);
            return "ok";
        }
        function setShadowAngle(value: real): string {
            if (!isFinite(value) || value < 0 || value > 360)
                return "Shadow angle must be between 0 and 360";
            StageCfg.Config.setShadowAngle(value);
            return "ok";
        }
        function resetLook(): string { StageCfg.Config.resetLook(); return "ok"; }
        function setAmount(value: string): string {
            if (["subtle", "normal", "strong"].indexOf(value) < 0)
                return "Amount must be subtle, normal or strong";
            StageCfg.Config.setAmount(value);
            return "ok";
        }
        function setIdle(value: string): string {
            if (["none", "float", "breathe", "sway"].indexOf(value) < 0)
                return "Idle motion must be none, float, breathe or sway";
            StageCfg.Config.setIdle(value);
            return "ok";
        }
        function setMusic(value: bool): string { StageCfg.Config.setMusic(value); return "ok"; }
        function setMouse(value: bool): string { StageCfg.Config.setMouse(value); return "ok"; }
        function setSpeed(value: real): string {
            if (!isFinite(value) || value < 0.25 || value > 2)
                return "Idle speed must be between 0.25 and 2";
            StageCfg.Config.setSpeed(value);
            return "ok";
        }
        function setMusicLevel(value: real): string {
            if (!isFinite(value) || value < 0 || value > 1)
                return "Music intensity must be between 0 and 1";
            StageCfg.Config.setMusicLevel(value);
            return "ok";
        }
        function setSensitivity(value: real): string {
            if (!isFinite(value) || value < 0 || value > 2)
                return "Pointer sensitivity must be between 0 and 2";
            StageCfg.Config.setSensitivity(value);
            return "ok";
        }
        function setRange(value: real): string {
            if (!isFinite(value) || value < 0 || value > 2)
                return "Pointer range must be between 0 and 2";
            StageCfg.Config.setRange(value);
            return "ok";
        }
        function setBackdrop(value: real): string {
            if (!isFinite(value) || value < 0 || value > 1)
                return "Backdrop drift must be between 0 and 1";
            StageCfg.Config.setBackdrop(value);
            return "ok";
        }
        function setVisualizerFront(value: bool): string { StageCfg.Config.setFront("visualizer", value); return "ok"; }
    }

    // Enter or leave the desktop-widget editor without the right-click menu, so
    // a keybind or the daemon can open it. niri has no global-shortcut protocol,
    // so every shell surface is driven this way; the editor is no exception.
    IpcHandler {
        id: desktopIpc
        target: "desktop"
        function editWidgets(mon: string): void {
            desktopIpc.editSection("widgets", mon);
        }
        // Open the Stage Editor straight on one catalogue (widgets, wallpaper,
        // style, visualizer, depth): the Hub's hand-offs and keybinds land on
        // the editor they mean. Already open, it switches catalogue in place.
        function editSection(section: string, mon: string): void {
            var m = mon;
            if (!m || m.length === 0) {
                const st = ShellState.forActive();
                m = (st && st.modelData) ? st.modelData.name : "";
            }
            // The editor targets one monitor by exact name; a cold focused-output
            // cache (niri via a keybind before a window is focused) yields "",
            // which matches no desktop, so fall back to the first output.
            if (!m || m.length === 0)
                m = ShellState.screens.length > 0 ? ShellState.screens[0].name : "";
            if (StageCfg.StageSession.onMonitor(m))
                StageCfg.StageSession.openSection(section);
            else
                StageCfg.StageSession.enterWidgets(m, section);
        }
        function editDone(): void { StageCfg.StageSession.leave(); }
        // Open a widget's right-click menu on a monitor (empty = focused).
        function menu(widget: string, mon: string): void {
            var m = mon;
            if (!m || m.length === 0) {
                const st = ShellState.forActive();
                m = (st && st.modelData) ? st.modelData.name : "";
            }
            ShellState.requestWidgetMenu(m, widget);
        }
        // Open a widget's inspector (Customize sheet) on a monitor (empty =
        // focused), the off-surface twin of the right-click Customize row.
        function customize(widget: string, mon: string): void {
            var m = mon;
            if (!m || m.length === 0) {
                const st = ShellState.forActive();
                m = (st && st.modelData) ? st.modelData.name : "";
            }
            ShellState.requestWidgetCustomize(m, widget);
        }
    }

    IpcHandler {
        target: "keypresses"
        readonly property bool active: Keypresses.active
        readonly property string status: Keypresses.backendStatus
        readonly property string lastEvent: Keypresses.lastEventSignature
        readonly property string theme: Keypresses.theme
        readonly property string mode: Keypresses.mode
        readonly property real revision: Keypresses.previewRevision
        function activate(theme: string, mode: string, revision: string): void {
            Keypresses.activatePreview(theme, mode, revision);
        }
        function deactivate(revision: string): void {
            Keypresses.deactivatePreview(revision);
        }
        function toggle(): void { Keypresses.toggle(); }
    }
    IpcHandler {
        target: "camera"
        readonly property bool active: Camera.active
        function toggle(): void { Camera.toggle(); }
    }
    // Surface global shortcuts dispatch through the mapping above so compositor
    // global shortcuts and the `ryoku-shell <id>` fallback behave identically.
    CustomShortcut {
        name: "quicksettings"
        description: I18n.tr("Open the left sidebar on the active monitor")
        onPressed: root.toggleSurface("quicksettings")
    }
    CustomShortcut {
        name: "wallpaper-menu"
        description: I18n.tr("Open the wallpaper and theme menu on the active monitor")
        onPressed: root.toggleSurface("wallpaper-menu")
    }
    CustomShortcut {
        name: "clipboard"
        description: I18n.tr("Open the clipboard history on the active monitor")
        onPressed: root.toggleSurface("clipboard")
    }
    CustomShortcut {
        name: "ask"
        description: I18n.tr("Ask Rashin")
        onPressed: root.toggleSurface("ask")
    }
    CustomShortcut {
        name: "compress"
        description: I18n.tr("Open Tools to compress media")
        onPressed: root.toggleSurface("compress")
    }
    CustomShortcut {
        name: "install"
        description: I18n.tr("Open Tools to install a package")
        onPressed: root.toggleSurface("install")
    }

}

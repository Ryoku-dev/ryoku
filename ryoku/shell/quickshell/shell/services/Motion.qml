pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Ui.Singletons

Singleton {
    id: root

    // Reduce motion (or the lowPowerMode master, or the Power Saver profile) collapses
    // every animation to an instant cut: durations go to 0, so Behaviors and
    // transitions stop forcing per-frame repaints -- the single biggest shell-animation
    // win on a weak GPU. The decision is Perf's (it folds shell.json theme.motion.reduce
    // with performance.json and the power profile); the dwell and scheduling timers below
    // (osdHide, notifHide, startupReveal) are functional, not motion, so reduce never gates them.
    readonly property bool reduce: Perf.reduceMotion

    // Global animation tempo: the Speed setting (shell.json theme.motion.scale,
    // default 1.0) scales every duration below, so the whole shell speeds up or eases
    // off in one place; reduce still wins by collapsing to an instant cut.
    readonly property real speed: Tokens.motionScale
    function dur(ms) { return root.reduce ? 0 : Math.round(ms * root.speed); }

    // Bar, spacer, hover and startup reveal/hide. Measured ease-out-cubic over
    // 250 ms (measure/MOTION-MEASURED.md; contract 02 sec 5). Interrupt: reverse
    // from the current position, which a RevealBehavior retarget reproduces.
    readonly property int barReveal: root.dur(250)
    readonly property int barRevealCurve: Easing.OutCubic

    // Left/right side menu slide. A reveal that reverses rather than restarting, 250 ms, ease-out-cubic,
    // same envelope as the bar (contract 05 sec 5; contract 01 sec 5). Interrupt:
    // reverse from current.
    readonly property int menuSlide: root.dur(250)
    readonly property int menuSlideCurve: Easing.OutCubic
    // Full-height sidebar entrance/exit. This follows iNiR's default slide:
    // translate the already-sized panel, with a slower emphasized settle in
    // and a short emphasized acceleration out. The panel never relayouts while
    // moving.
    readonly property int sidebarEnter: root.dur(400)
    readonly property int sidebarExit: root.dur(200)
    readonly property var sidebarEnterCurve: [0.05, 0.7, 0.1, 1, 1, 1]
    readonly property var sidebarExitCurve: [0.3, 0, 0.8, 0.15, 1, 1]
    readonly property var sidebarFadeInCurve: [0, 0, 0, 1, 1, 1]
    readonly property var sidebarFadeOutCurve: [0.3, 0, 1, 1, 1, 1]

    // Sidebar page push: the one continuous navigation move, 420 ms on a long
    // OutQuint settle so the incoming page glides in and eases to rest, never
    // snaps. The single language for every sidebar page transition.
    readonly property int push: root.dur(420)
    readonly property int pushCurve: Easing.OutQuint

    // Corner/top/bottom scale grow that restarts from its current position. 200 ms easeInOutQuad, set
    // explicitly in source (contract 01 sec 5; contract 05 sec 5). Interrupt:
    // restart from the current position toward the new target.
    readonly property int diagonal: root.dur(200)
    readonly property int diagonalCurve: Easing.InOutQuad

    // Menu swap within one stack region: a stacked crossfade, linear opacity,
    // 200 ms (contract 01 sec 5; contract 05 sec 5).
    readonly property int crossfade: root.dur(200)
    readonly property int crossfadeCurve: Easing.Linear

    // Revealer row/button expand: child height SlideDown, 200 ms ease-out-cubic
    // (contract 16 sec 5; contract 06 sec 5). Notification cards (contract 12
    // sec 5), dynamic-list entries and the weather inner slide share this.
    readonly property int rowReveal: root.dur(200)
    readonly property int rowRevealCurve: Easing.OutCubic

    // Revealed-content fade and the chevron 0->90 turn: CSS `ease` =
    // cubic-bezier(0.25, 0.1, 0.25, 1) over 200 ms (contract 16 sec 5;
    // contract 06 sec 5). Thumbnail hover is the same curve over 150 ms
    // (contract 08 sec 5).
    readonly property int rowFade: root.dur(200)
    readonly property int chevronRotate: root.dur(200)
    readonly property int thumbHover: root.dur(150)
    readonly property int easeType: Easing.Bezier
    readonly property var easeCurve: [0.25, 0.1, 0.25, 1, 1, 1]

    // Desktop wallpaper swap crossfade, 200 ms (contract 08 sec 5,
    // TRANSITION_DURATION_MS). Linear opacity like the menu crossfade.
    readonly property int wallpaperFade: root.dur(200)

    // Weather outer-stack crossfade, 250 ms, set in source (contract 08 sec 5).
    readonly property int weatherFade: root.dur(250)

    // OSD auto-hide hold: the OSD has NO show/hide animation (contract 12 sec 5);
    // only this 1000 ms dwell from the last value update is timed, and a new
    // value resets it. A dwell timer, not motion, so reduce does not gate it.
    readonly property int osdHide: 1000

    // Notification popup entrance/exit (contract 12 sec 5, eye-candy pass): a
    // toast glides in from the screen edge it is anchored to and glides back out
    // the same way. Both are long enough to read as one continuous move rather
    // than a cut: the entrance decelerates into place on the emphasized settle
    // the sidebar uses, and the exit starts gently before accelerating away, so
    // a toast never appears to be yanked off screen.
    readonly property int notifIn: root.dur(420)
    readonly property int notifOut: root.dur(340)
    readonly property var notifInCurve: [0.05, 0.7, 0.1, 1, 1, 1]
    readonly property var notifOutCurve: [0.4, 0, 0.2, 1, 1, 1]

    // Notification container unmap delay: measured from the moment the popup list
    // empties, so the last card's exit finishes before the surface drops
    // (contract 12 sec 5). It has to outlast notifOut with a frame to spare, or
    // the last toast is cut off mid-slide. A scheduling timer, not motion.
    readonly property int notifHide: 420

    // Bars auto-reveal once, 1000 ms after startup (contract 02 sec 5). A one-shot
    // startup delay, not motion, so reduce keeps the timing and only drops the
    // slide.
    readonly property int startupReveal: 1000

    // Retained transitional timings for remaining Popout, Deck, Stash,
    // KeyringSurface, and WaveMeter consumers.
    readonly property int fast:     root.dur(140)
    readonly property int standard: root.dur(300)
    readonly property int morph:    root.dur(420)
    readonly property int hover:    root.dur(100)
    readonly property int spatial:  root.dur(500)
    readonly property int effects:  root.dur(200)
    readonly property int easeStandard: Easing.OutCubic
    readonly property var spatialCurve: [0.38, 1.21, 0.22, 1, 1, 1]
    readonly property var effectsCurve: [0.34, 0.8, 0.34, 1, 1, 1]
    readonly property real rSmall: 7
    readonly property real rTile:  13
}

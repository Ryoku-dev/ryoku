pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

// Perf is the shell's one performance-policy singleton and the single reader of
// ~/.config/ryoku/performance.json (the file Ryoku Settings' Performance page
// writes). It folds four inputs into the effective switches every surface obeys:
//
//   1. the explicit user toggles in performance.json (lowPowerMode is the master),
//   2. the active power profile (PowerProfiles), when powerProfileEffects is on,
//   3. the live battery state (Battery), for invisible savings while discharging,
//   4. Game Mode (Flags.gameMode), which outranks all of them.
//
// Consumers read the derived booleans (blurDisabled, shadowsDisabled, reduceMotion,
// visualizerFrozen, pillFrozen) instead of re-deriving `lowPower || flag`: the
// "lowPowerMode implies ..." and "Power Saver implies ..." rules live here once.
// Motion reads reduceMotion/motionSpeed from here too, so the whole shell shares
// one file watcher rather than a copy per module.
//
// Tiers: only Power Saver forces eye-candy off (it behaves like lowPowerMode).
// Balanced and Performance leave the explicit toggles untouched, so the default
// profile never overrides a user's choice and the desktop stays smooth. Battery
// thrift (slower background polling, and not waking a suspended dGPU) is graceful
// and applies whenever discharging, independent of the profile, because it costs
// the user nothing they can see.
//
// Game Mode is the one tier that ignores the user's explicit toggles, and it is
// meant to: the compositor is already stripped and the CPU is already pinned to
// performance, so leaving the shell's own eye-candy running would spend the
// headroom that was just bought. It is also the only tier where the shell is
// competing with a specific foreground process for the same GPU, which is why it
// goes further than Saver and drops the audio analyser outright rather than
// merely freezing it when idle. Nothing here is persisted: it lasts exactly as
// long as the toggle, and every switch returns to the user's own settings on
// exit.
Singleton {
    id: root

    readonly property bool lowPower: adapter.lowPowerMode
    readonly property real motionSpeed: {
        const v = adapter.motionSpeed;
        return (typeof v === "number" && v > 0 && v <= 8) ? v : 1.0;
    }

    // The two persisted user preferences the Super+Escape quick settings flips.
    // Perf is the shell's only reader of performance.json, so its write lives
    // here too rather than a second copy in a route: setting a key rewrites the
    // file (atomic, watched) and every derived switch above re-folds from it.
    // lowPower already exposes adapter.lowPowerMode above; this is its counterpart.
    readonly property bool reduceMotionPref: adapter.reduceMotion
    function setLowPower(on) { adapter.lowPowerMode = on; file.writeAdapter(); }
    function setReduceMotion(on) { adapter.reduceMotion = on; file.writeAdapter(); }
    // The bar's silent gap drift, flipped from the bar control centre's Gap
    // animation card as well as the Performance page. Same write path as above.
    readonly property bool ambientBarMotionPref: adapter.ambientBarMotion
    function setAmbientBarMotion(on) { adapter.ambientBarMotion = on; file.writeAdapter(); }

    // Power profile -> tier. With powerProfileEffects off, or no power-profiles-daemon
    // (a desktop reports no profiles), the tier is Balanced so nothing is forced.
    readonly property int tierSaver: 0
    readonly property int tierBalanced: 1
    readonly property int tierPerformance: 2
    readonly property int tier: {
        if (!adapter.powerProfileEffects || !PowerProfiles.available)
            return root.tierBalanced;
        switch (PowerProfiles.profile) {
        case "power-saver": return root.tierSaver;
        case "performance": return root.tierPerformance;
        default: return root.tierBalanced;
        }
    }
    readonly property bool saver: root.tier === root.tierSaver

    readonly property bool onBattery: Battery.present && Battery.discharging

    // Game Mode. Read straight off the persisted flag rather than mirrored here,
    // so a relogin mid-session comes back in the same state the toggle left.
    readonly property bool gaming: Flags.gameMode

    // Effective eye-candy switches: explicit toggle, or the lowPowerMode master, or
    // the Power Saver tier, or Game Mode. Performance/Balanced add nothing.
    readonly property bool reduceMotion:     lowPower || adapter.reduceMotion   || saver || gaming
    readonly property bool blurDisabled:     lowPower || adapter.disableBlur    || saver || gaming
    readonly property bool shadowsDisabled:  lowPower || adapter.disableShadows || saver || gaming

    // The two audio-driven surfaces freeze when there is nothing to show, which is
    // what "WhenIdle" always meant and never did: `freezeVisualizerWhenIdle` and
    // `freezePillWhenIdle` were folded in as plain always-off switches, so the only
    // choices were "analyse silence forever" (their default) or "never analyse at
    // all". The first is what shipped, and it cost two cava processes plus a
    // permanently damaged surface keeping the compositor awake.
    //
    // "Sounding" is real audio reaching the sink, not just a media player's
    // reported state. Keying idle off MPRIS alone froze the analysers whenever
    // sound had no MPRIS interface -- games, browser tabs, system sounds -- or
    // when the player under-reported between tracks (#61). The ground truth is an
    // active PipeWire playback stream (Audio.streams, itself settled); Media.playing
    // stays as a fast-path so a player that flips to playing before its stream is
    // listed still counts instantly. The hard tiers still win outright below, so
    // Saver, lowPowerMode and Game Mode keep the analysers off even mid-track.
    //
    // Bridge brief gaps: a stream tears down and rebuilds for a beat between
    // tracks, so dropping cava on every blip stuttered the visualiser and pill.
    // The idle->active edge is instant; active->idle is debounced by audioGrace.
    // This is imperative on purpose: `property bool audioIdle: !sounding` would be
    // a live binding that flips the instant the signal changes, defeating the
    // grace (the same binding-vs-imperative trap as the analysers' `running`).
    // onSoundingChanged + audioGrace are the sole controllers; the initial state
    // is seeded once.
    // Surface residency has its own 30-second grace. Rebuilding one PanelWindow
    // per output during a short track gap costs more than keeping them warm.
    readonly property bool sounding: Media.playing || Audio.streams.length > 0
    readonly property bool visualizerHardFrozen: lowPower || saver || gaming
    readonly property bool unloadVisualizerWhenSilent: adapter.unloadVisualizerWhenSilent
    property bool audioIdle: true
    property bool visualizerResident: true

    function syncVisualizerResidency() {
        visualizerResidencyGrace.stop();
        if (root.visualizerHardFrozen) {
            root.visualizerResident = false;
        } else if (root.sounding || !root.unloadVisualizerWhenSilent) {
            root.visualizerResident = true;
        } else if (root.visualizerResident) {
            visualizerResidencyGrace.restart();
        }
    }

    Component.onCompleted: {
        root.audioIdle = !root.sounding;
        root.syncVisualizerResidency();
    }
    onSoundingChanged: {
        if (root.sounding) {
            audioGrace.stop();
            root.audioIdle = false;
        } else {
            audioGrace.restart();
        }
        root.syncVisualizerResidency();
    }
    onUnloadVisualizerWhenSilentChanged: root.syncVisualizerResidency()
    onVisualizerHardFrozenChanged: root.syncVisualizerResidency()

    Timer { id: audioGrace; interval: 4000; onTriggered: root.audioIdle = true }
    Timer {
        id: visualizerResidencyGrace
        interval: 30000
        onTriggered: {
            if (root.unloadVisualizerWhenSilent && !root.sounding
                    && !root.visualizerHardFrozen)
                root.visualizerResident = false;
        }
    }
    readonly property bool visualizerFrozen: root.visualizerHardFrozen || audioIdle
    readonly property bool pillFrozen: root.visualizerHardFrozen || audioIdle

    // Ambient motion: the bar's stream drifting on a passive sine while the desktop
    // is SILENT (with audio playing it reacts regardless -- that is streamLive's
    // audioLive term, not this). This passive drift is the shell's one
    // continuously-repainting idle surface, and it is why a default box idled far
    // above the shells in #60: caelestia and end-4 only animate the bar while a
    // player isPlaying, and a Waybar box has no GPU canvas at all.
    //
    // By default the silent drift is Performance-only: Balanced and Saver leave the
    // bar still when nothing plays, idling quiet like those shells. A user who
    // wants it anyway opts in with ambientBarMotion (the Performance page's "Bar
    // drifts when silent"), which runs the drift on Balanced and Performance alike.
    // lowPowerMode, Game Mode and Power Saver still force it off, and reduceMotion
    // is enforced by the consumer (streamLive gates on !reduceMotion), so the
    // motion toggle stops it too.
    //
    // fullRate answers "may the drift run at the full frame rate it was drawn for".
    // The silent drift runs at full rate; loud playback (paceBusy) is always full
    // rate on every profile, and a fully dark frame always backs off.
    readonly property bool ambientMotion: (tier === tierPerformance || adapter.ambientBarMotion) && !(lowPower || gaming || saver)
    readonly property bool fullRate: ambientMotion

    // Graceful cost knobs. Multiply a base poll interval by pollFactor: a second of
    // staleness in a stat readout is invisible, so sampling slows on battery / Saver.
    // Game Mode goes further: nobody is reading a stat readout mid-match, and each
    // poll is a process spawn competing with the game for the same cores.
    // msaa is the sample count for vector layers: crisp by default, halved under Saver.
    readonly property int pollFactor: gaming ? 4 : (onBattery || saver) ? 2 : 1
    readonly property int msaa: (lowPower || saver || gaming) ? 2 : 8

    FileView {
        id: file
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/performance.json"
        watchChanges: true
        atomicWrites: true
        printErrors: false
        onFileChanged: reload()
        JsonAdapter {
            id: adapter
            property bool lowPowerMode: false
            property bool reduceMotion: false
            property bool disableBlur: false
            property bool disableShadows: false
            property real motionSpeed: 1.0
            property bool powerProfileEffects: true
            property bool ambientBarMotion: false
            property bool unloadVisualizerWhenSilent: true
        }
    }
}

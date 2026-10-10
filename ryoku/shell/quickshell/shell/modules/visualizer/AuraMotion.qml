pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui
import "Singletons"

// The `aura` look's motion: the shared cava bands folded into twelve sectors and
// followed with the organic field's asymmetric attack/release, plus the peaks
// that ride them, the beat pulse, the transient onset and the phase that turns
// the current. AuraField paints from what this hands it and knows no audio.
//
// The aura twin of Motion, and like Motion it runs on one Timer rather than
// vsync and halves its rate while only the opted-in idle wave is moving. Without
// that ambient motion it stops once the release reaches rest. The same adaptive
// tier applies: under load the field sheds no effects, it just updates less often.
Item {
    id: motion

    required property VizItem cfg
    property bool active: true

    readonly property int sectors: 12

    // --- outputs, straight into AuraField ---------------------------------
    property var bands: []
    property var peaks: []
    property real energy: 0
    property real pulse: 0
    property real onset: 0
    property real phase: 0

    // eased sector values from the frame before this one, so the follow filter
    // has something to move from.
    property var held: []
    property real prevEnergy: 0
    property real activity: 0
    property real dt: 1 / 30

    // --- budget ------------------------------------------------------------
    property real govOverrun: 1
    property int govTier: 0
    property real govSince: 0
    readonly property bool govOn: Config.adaptive
    readonly property int fps: {
        if (!motion.govOn || motion.govTier === 0)
            return Config.fps;
        return motion.govTier === 1 ? Math.min(Config.fps, 30) : Math.min(Config.fps, 24);
    }

    // The rates the organic field uses, scaled by smoothing and by the user's
    // attack and release: a transient snaps up, everything else sinks gently.
    readonly property real smooth08: motion.cfg.smoothing * 8
    readonly property real attackRate: 24 * motion.cfg.auraAttack
        / (1 + motion.smooth08 * 0.22)
    readonly property real releaseRate: 8 * motion.cfg.auraRelease
        / (1 + motion.smooth08 * 0.20)
    readonly property real energyAttack: 12 * motion.cfg.auraAttack
        / (1 + motion.smooth08 * 0.18)
    readonly property real energyRelease: 3.2 * motion.cfg.auraRelease
        / (1 + motion.smooth08 * 0.16)

    readonly property bool sounding: Spectrum.energy > 0.04 || motion.activity > 0.02
    // Idle wave is an explicit visual choice. Plain audio-idle stops spectrum
    // analysis, while only the hard performance tiers stop ambient motion.
    readonly property bool frozen: Performance.visualizerFrozen
    readonly property bool hardFrozen: Performance.visualizerHardFrozen
    readonly property bool ambient: motion.cfg.idleWave && !motion.hardFrozen
    readonly property bool animating: motion.sounding || motion.ambient
        || motion.energy > 0.004

    Timer {
        id: ticker
        interval: Math.round(1000 / (motion.sounding ? motion.fps : Math.max(20, motion.fps / 2)))
        running: motion.active && Config.enabled && motion.animating
        repeat: true
        property real last: 0
        // A stop leaves `last` stale: the first tick after a hard freeze or
        // inactive surface would otherwise clock the whole frozen span as one
        // frame and spike the governor. Reseed that first tick.
        onRunningChanged: if (!ticker.running) ticker.last = 0;
        onTriggered: {
            var now = Date.now();
            var raw = ticker.last > 0 ? (now - ticker.last) / 1000 : ticker.interval / 1000;
            ticker.last = now;
            if (motion.govOn)
                motion.governor(raw, ticker.interval / 1000);
            motion.tick(Math.min(0.05, raw));
        }
    }

    // Climb and descend tiers on a slow average of the overrun, with a dwell, so
    // a single hitch never trips a change and the tier cannot oscillate.
    function governor(raw, asked) {
        // A gap far past the asked interval is a resume, not sustained overrun:
        // counting it would trip a tier the instant animation restarts.
        if (raw > asked * 4)
            return;
        var ratio = Math.min(3, asked > 0 ? raw / asked : 1);
        motion.govOverrun += (ratio - motion.govOverrun) * 0.1;
        var now = Date.now();
        if (now - motion.govSince < 2500)
            return;
        if (motion.govOverrun > 1.6 && motion.govTier < 2) {
            motion.govTier += 1;
            motion.govSince = now;
        } else if (motion.govOverrun < 1.15 && motion.govTier > 0) {
            motion.govTier -= 1;
            motion.govSince = now;
        }
    }

    function tick(dt) {
        motion.dt = dt;
        // activity rises fast on the first beat and releases slowly, so a gap
        // between tracks does not flicker the field off.
        var goal = Spectrum.energy > 0.04 ? 1 : 0;
        motion.activity += (goal - motion.activity)
            * (1 - Math.exp(-dt / (goal > motion.activity ? 0.05 : 1.1)));

        // Once analysis freezes, do not keep replaying the last musical frame.
        // The opted-in idle phase may continue from an empty source until the
        // 30-second residency grace unloads the surface.
        var source = motion.frozen ? [] : Spectrum.levels;
        var shaped = AuraMath.applyProfile(source,
                                           motion.cfg.auraProfile,
                                           motion.cfg.auraAccent);
        var raw = AuraMath.sectors(shaped);
        var eased = new Array(motion.sectors);
        for (var i = 0; i < motion.sectors; i++) {
            var from = (motion.held && i < motion.held.length) ? motion.held[i] : 0;
            eased[i] = AuraMath.follow(from, raw[i], dt,
                                       motion.attackRate, motion.releaseRate);
        }
        motion.held = eased;

        // one contrast pass over the frame's own spread, and the weighted energy.
        var f = AuraMath.frame(eased);
        motion.bands = f.levels;

        // Peaks trail the live sectors and only fall: a hit stays drawn as the
        // crest moves past it.
        var pk = motion.peaks;
        var np = new Array(motion.sectors);
        var peakRate = 2.1 * motion.cfg.auraRelease / (1 + motion.smooth08 * 0.14);
        for (var p = 0; p < motion.sectors; p++) {
            var pc = (pk && p < pk.length) ? pk[p] - dt * peakRate : 0;
            np[p] = f.levels[p] > pc ? f.levels[p] : Math.max(0, pc);
        }
        motion.peaks = np;

        motion.energy = AuraMath.follow(motion.energy, f.energy, dt,
                                        motion.energyAttack, motion.energyRelease);
        var rise = Math.max(0, motion.energy - motion.prevEnergy);
        motion.prevEnergy = motion.energy;
        motion.onset = AuraMath.follow(motion.onset, Math.min(1, rise * 7.5), dt, 28, 4.8);

        // The pulse follows sustained bass and transients, deliberately quicker
        // than the field's own envelope, so the current reads as musical instead
        // of merely wobbling.
        var bass = Math.max(f.levels[0] || 0, f.levels[1] || 0);
        motion.pulse = AuraMath.follow(motion.pulse,
            Math.min(1, bass * 0.72 + motion.energy * 0.42 + motion.onset * 0.88),
            dt, 18, 5.2);

        // The phase integrates the field's turn: faster the louder it is, and it
        // never runs backwards, so the current always flows the chosen way.
        motion.phase = (motion.phase + dt * motion.cfg.auraMotionSpeed
            * (0.055 + (motion.ambient ? motion.cfg.auraIdleMotion : 0) * 0.035
               + motion.energy * 0.075 + motion.onset * 0.12)) % 1;
    }

    // A stopped Timer must leave a resting field behind, not the last frame of
    // the music.
    onAnimatingChanged: if (!motion.animating) {
        motion.energy = 0;
        motion.pulse = 0;
        motion.onset = 0;
        motion.peaks = [];
        motion.held = [];
    }

    onHardFrozenChanged: if (motion.hardFrozen) {
        motion.activity = 0;
        motion.energy = 0;
        motion.prevEnergy = 0;
        motion.pulse = 0;
        motion.onset = 0;
        motion.bands = [];
        motion.peaks = [];
        motion.held = [];
    }

    // Turning the governor off must not leave a stale cap latched: re-enabling
    // adaptive quality starts clean at the full frame rate.
    onGovOnChanged: if (!motion.govOn) {
        motion.govTier = 0;
        motion.govOverrun = 1;
    }
}

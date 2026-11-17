pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui
import "Singletons"

// The spectrum's motion, kept apart from its drawing: cava's bands (or the
// monitor waveform, for the scope) become eased levels, falling peaks, an "is
// music playing" signal and a rotation. Nothing here knows what a look looks
// like.
//
// One Timer drives it at the configured rate rather than vsync. On silence it
// briefly eases into the chosen rest shape, then stops and leaves that shape
// still. The adaptive tier lowers the live rate under sustained overrun without
// shedding effects.
Item {
    id: motion

    // the look being drawn: only the scope reads a waveform instead of bands.
    property string style: "bars"
    property bool active: true
    // the instance being eased; every per-viz knob is read from here, the budget
    // (fps/adaptive) and master switch stay global on Config.
    required property VizItem cfg

    readonly property bool scope: motion.style === "line"
    readonly property int bands: motion.scope
        ? Math.max(16, Math.min(128, motion.cfg.bars * 2))
        : Math.max(4, Math.min(128, motion.cfg.bars))

    // --- outputs -----------------------------------------------------------
    property var levels: []
    property var peaks: []
    readonly property real energy: motion.maxLevel
    // Master fade. With the idle wave off, the picture releases with the
    // activity signal so a fixed-baseline look (the ring, the orb) clears on
    // silence instead of freezing when the Timer stops.
    readonly property real fade: motion.wantIdleWave ? 1
        : Math.max(0, Math.min(1, (motion.activity - 0.02) / 0.25))
    property real spinDeg: 0

    property real activity: 0
    property real idlePhase: 0
    property real maxLevel: 0

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

    readonly property bool sourceSounding: !Spectrum.silent
        || (motion.scope && !Waveform.silent)
    readonly property bool sounding: motion.sourceSounding || motion.activity > 0.02
    readonly property bool hardFrozen: Performance.visualizerHardFrozen
    readonly property bool wantIdleWave: motion.cfg.idleWave && !motion.hardFrozen
    readonly property bool wantSpin: motion.cfg.spin !== 0 && !motion.hardFrozen
    readonly property bool wantPeaks: motion.cfg.peaks
        && (motion.style === "bars" || motion.style === "segments")
    property bool idleSettled: false
    readonly property bool animating: motion.sourceSounding || motion.activity > 0.02
        || (motion.wantIdleWave ? !motion.idleSettled : motion.maxLevel > 0.004)

    readonly property bool foldBands: motion.cfg.mirror && !motion.cfg.isPolar

    function rawLevel(i) {
        var l = Spectrum.levels;
        var s = SpectrumMath.srcIndex(i, motion.bands, motion.foldBands);
        var v = (l && s < l.length) ? l[s] : 0;
        return Math.min(1, Math.pow(v, 0.72) * motion.cfg.gain);
    }
    function idleLevel(i) {
        return 0.012 + 0.02 * (0.5 + 0.5 * Math.sin(
            SpectrumMath.srcIndex(i, motion.bands, motion.foldBands) * 0.4 + motion.idlePhase));
    }

    Timer {
        id: ticker
        interval: Math.round(1000 / (motion.sourceSounding ? motion.fps : Math.max(20, motion.fps / 2)))
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
        var goal = motion.sourceSounding ? 1 : 0;
        if (goal > 0)
            motion.idleSettled = false;
        motion.activity += (goal - motion.activity)
            * (1 - Math.exp(-dt / (goal > motion.activity ? 0.05 : 1.1)));
        if (motion.wantIdleWave && motion.sourceSounding)
            motion.idlePhase += dt * (Math.PI * 2 / 6);
        if (motion.wantSpin && motion.sourceSounding)
            motion.spinDeg = (motion.spinDeg + dt * motion.cfg.spin) % 360;

        if (motion.scope) {
            motion.stepScope(dt);
            return;
        }

        var n = motion.bands;
        var prev = motion.levels;
        var idleAmt = motion.wantIdleWave ? (1 - motion.activity) : 0;
        // smoothing stretches the decay, and a touch of the attack.
        var decay = 0.06 + 0.20 * motion.cfg.smoothing;
        var attack = 0.035 + 0.02 * motion.cfg.smoothing;
        var out = new Array(n);
        var mx = 0;
        var maxDelta = 0;
        for (var i = 0; i < n; i++) {
            var target = motion.activity * motion.rawLevel(i) + idleAmt * motion.idleLevel(i);
            var cur = (prev && i < prev.length) ? prev[i] : 0;
            out[i] = SpectrumMath.ease(cur, target, dt, attack, decay);
            maxDelta = Math.max(maxDelta, Math.abs(out[i] - target));
            if (out[i] > mx)
                mx = out[i];
        }
        motion.levels = out;
        motion.maxLevel = mx;

        if (motion.wantPeaks) {
            var pk = motion.peaks;
            var np = new Array(n);
            for (var p = 0; p < n; p++) {
                var pc = (pk && p < pk.length) ? pk[p] - dt * 0.5 : 0;
                np[p] = out[p] > pc ? out[p] : Math.max(0, pc);
            }
            motion.peaks = np;
        } else if (motion.peaks.length) {
            motion.peaks = [];
        }

        if (!motion.sourceSounding && motion.activity < 0.003
                && motion.wantIdleWave && maxDelta < 0.0005) {
            var rest = new Array(n);
            for (var r = 0; r < n; r++)
                rest[r] = motion.idleLevel(r);
            motion.levels = rest;
            motion.peaks = [];
            motion.idleSettled = true;
        }
    }

    // The scope carries the real playback waveform, folded down to the band
    // count and centred on 0.5, the slot the shader reads for it.
    //
    // It normalises against a decaying peak the way cava does internally: a
    // waveform read straight off the monitor is a flat line at any ordinary
    // listening volume, since the samples are a few percent of full scale.
    property real scopePeak: 0.15

    // The scope rests on its centre line, 0.5 per band. An empty array reads as
    // 0 in every slot, the bottom of the trace, and the shader draws that as a
    // full-height zig-zag until music plays (a reload in silence never ticks).
    function restScope() {
        var flat = new Array(motion.bands);
        for (var f = 0; f < motion.bands; f++)
            flat[f] = 0.5;
        motion.levels = flat;
        motion.maxLevel = 0;
    }

    function stepScope(dt) {
        var wav = Waveform.samples;
        var n = motion.bands;
        if (!wav || wav.length < 2 || Waveform.silent) {
            motion.restScope();
            if (!motion.sourceSounding && motion.activity < 0.003)
                motion.idleSettled = true;
            return;
        }
        var folded = SpectrumMath.resample(wav, n);
        var frame = 0;
        for (var j = 0; j < n; j++) {
            var m = Math.abs(folded[j]);
            if (m > frame)
                frame = m;
        }
        motion.scopePeak = Math.max(frame, motion.scopePeak - dt * 0.25);
        var norm = motion.cfg.gain / Math.max(0.02, motion.scopePeak);
        var out = new Array(n);
        var mx = 0;
        for (var i = 0; i < n; i++) {
            var v = Math.tanh(folded[i] * norm * 0.9);
            out[i] = SpectrumMath.scopeMap(v);
            var a = Math.abs(v);
            if (a > mx)
                mx = a;
        }
        motion.levels = out;
        motion.maxLevel = mx;
    }

    onHardFrozenChanged: if (motion.hardFrozen) {
        motion.activity = 0;
        motion.maxLevel = 0;
        if (motion.scope)
            motion.restScope();
        else
            motion.levels = [];
        motion.peaks = [];
    }

    // A stopped Timer must leave a resting picture behind, not the last frame
    // of the music.
    onAnimatingChanged: if (!motion.animating) {
        motion.maxLevel = 0;
        if (motion.peaks.length)
            motion.peaks = [];
    }
    onBandsChanged: {
        if (motion.scope)
            motion.restScope();
        else
            motion.levels = [];
        motion.peaks = [];
    }
    onScopeChanged: if (motion.scope && !motion.animating) motion.restScope()
    Component.onCompleted: if (motion.scope) motion.restScope()
    onWantIdleWaveChanged: motion.idleSettled = false

    // Turning the governor off must not leave a stale cap latched: re-enabling
    // adaptive quality starts clean at the full frame rate.
    onGovOnChanged: if (!motion.govOn) {
        motion.govTier = 0;
        motion.govOverrun = 1;
    }
}

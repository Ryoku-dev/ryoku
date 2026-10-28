pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui
import "Singletons"

// The desktop spectrum: geometry and colour only. Motion (or AuraMotion for the
// edge field) eases cava into levels, Ryoku.Ui.SpectrumField or Ryoku.Ui.AuraField
// draws them in one GPU pass, and this decides what the wallpaper behind should
// make them look like. Eight ramp stops are lit slice by slice across the region
// the look covers, so a spectrum crossing a bright sky and a dark tree stays
// legible along its whole width. The aura look instead paints a wallpaper-lit
// triad through AuraField, which has its own palette rather than a ramp.
Item {
    id: root

    // The instance this surface paints. Every per-viz knob is read from here, so
    // one draw path serves the primary and each extra visualiser alike.
    required property VizItem cfg

    // The stable instance id is independent of the roster position. Repeater
    // delegates can move after a delete without inheriting another view's
    // preview, selection, or governor state.
    required property string instanceId
    required property int index

    readonly property string style: root.cfg.styleId
    readonly property bool aura: root.style === "aura"
    readonly property bool polar: field.polar
    readonly property var preview: Config.previewAt(root.instanceId)

    // What the placement overlay needs: the look's box, and a colour lit for the
    // same wallpaper. The edge field owns the whole screen, like the frame, so
    // its box is the screen itself.
    readonly property rect boxRect: root.aura ? Qt.rect(0, 0, root.width, root.height)
        : field.boxRect
    readonly property color guide: root.ramp.length > 0 ? root.ramp[root.ramp.length - 1] : "white"

    AuraMotion {
        id: auraMotion
        cfg: root.cfg
        active: root.visible && Config.enabled && root.aura
    }

    Motion {
        id: motion
        cfg: root.cfg
        style: root.style
        active: root.visible && Config.enabled && !root.aura
    }

    // The active instance owns the settings-popup "capped" readout: publish the
    // driving motion's governor tier into Config, cleared when this view is not
    // the active one, is not running, or adaptive quality is off. One publisher
    // per view (the live family, aura or bars) keeps the two motions from racing.
    readonly property int govTier: root.aura ? auraMotion.govTier : motion.govTier
    readonly property bool govDriving: root.aura ? auraMotion.active : motion.active
    function publishGovTier() {
        if (root.instanceId === Config.activeId)
            Config.activeGovTier = (root.govDriving && Config.adaptive) ? root.govTier : 0;
    }
    onGovTierChanged: root.publishGovTier()
    onGovDrivingChanged: root.publishGovTier()
    Component.onCompleted: root.publishGovTier()
    Connections {
        target: Config
        function onActiveChanged() { root.publishGovTier(); }
        function onAdaptiveChanged() { root.publishGovTier(); }
    }

    // Normalised for the wallpaper luminance map: a turned look sits on a different
    // patch of picture than its box does.
    readonly property real nx: Math.max(0, Math.min(1, field.coverRect.x / Math.max(1, root.width)))
    readonly property real ny: Math.max(0, Math.min(1, field.coverRect.y / Math.max(1, root.height)))
    readonly property real nw: Math.max(0.01, Math.min(1, field.coverRect.width / Math.max(1, root.width)))
    readonly property real nh: Math.max(0.01, Math.min(1, field.coverRect.height / Math.max(1, root.height)))

    readonly property real fieldLstar: Scheme.lstarAt(root.nx, root.ny, root.nw, root.nh)
    // One direction for the whole sweep: per stop, neighbours would flip between
    // near-white and near-black over a mid-tone picture.
    readonly property int fieldSide: Scheme.side(root.fieldLstar)

    readonly property var ramp: {
        // A two-stop gradient wins: eight stops sweep from the first pinned colour
        // to the second across the spectrum, exactly as chosen, no wallpaper relight.
        if (root.cfg.gradient) {
            var g0 = root.cfg.customColor;
            var g1 = root.cfg.color2Value;
            var grad = [];
            for (var j = 0; j < 8; j++)
                grad.push(Qt.tint(g0, Qt.rgba(g1.r, g1.g, g1.b, j / 7)));
            return grad;
        }
        // A single pinned colour is respected exactly: the same gentle bass->treble
        // walk the wallpaper ramp uses, but no re-lighting against the picture.
        if (root.cfg.hasCustomColor) {
            var base = root.cfg.customColor;
            var pinned = [];
            for (var k = 0; k < 8; k++) {
                var tk = k / 7;
                var fk = 1 + (tk - 0.5) * 0.36;
                pinned.push(fk >= 1 ? Qt.lighter(base, fk) : Qt.darker(base, 1 / fk));
            }
            return pinned;
        }
        var out = [];
        for (var i = 0; i < 8; i++) {
            var t = i / 7;
            // A polar look reads one tone: its stops sweep a ring, not the picture.
            var l = root.polar ? root.fieldLstar
                : (field.vertical ? Scheme.lstarAt(root.nx, root.ny + t * root.nh * 0.875, root.nw, root.nh / 8)
                                  : Scheme.lstarAt(root.nx + t * root.nw * 0.875, root.ny, root.nw / 8, root.nh));
            out.push(Scheme.colorAt(t, l, root.fieldSide));
        }
        return out;
    }

    SpectrumField {
        id: field
        anchors.fill: parent
        visible: !root.aura

        levels: motion.levels
        peaks: motion.peaks
        energy: motion.energy
        fade: motion.fade
        ramp: root.ramp

        style: root.style
        shape: root.cfg.shape
        thickness: root.cfg.thickness
        reflection: root.cfg.reflection
        segments: root.cfg.segments
        // cfg owns the rule, so the bar dims the switch this binding ignores.
        peakCaps: root.cfg.peaks && root.cfg.peaksApply
        glow: root.cfg.bloom
        boxX: root.preview ? root.preview.x : root.cfg.x
        boxY: root.preview ? root.preview.y : root.cfg.y
        boxW: root.cfg.w
        boxH: root.cfg.h
        grow: root.cfg.grow
        angle: root.cfg.angle
        tiltX: root.cfg.tiltX
        tiltY: root.cfg.tiltY
        spin: motion.spinDeg
    }

    // The edge field: one pass over the whole screen, lit by the same wallpaper
    // the bars are, as a triad rather than an eight-stop ramp. A complete
    // pinned triad wins exactly as chosen; one pinned colour is respected the
    // way every other look respects it - the field walks that one hue - and
    // only with nothing pinned does the wallpaper light it.
    readonly property var auraTriad: {
        if (root.cfg.hasAuraTriad)
            return [root.cfg.customColor, root.cfg.auraColor2, root.cfg.auraColor3];
        if (root.cfg.hasCustomColor) {
            var base = root.cfg.customColor;
            return [Qt.darker(base, 1.25), base, Qt.lighter(base, 1.25)];
        }
        var l = root.fieldLstar;
        return [Scheme.colorAt(0.12, l, root.fieldSide),
                Scheme.colorAt(0.50, l, root.fieldSide),
                Scheme.colorAt(0.88, l, root.fieldSide)];
    }

    AuraField {
        id: auraField
        anchors.fill: parent
        visible: root.aura

        bands: auraMotion.bands
        peaks: auraMotion.peaks
        energy: auraMotion.energy
        pulse: auraMotion.pulse
        onset: auraMotion.onset
        phase: auraMotion.phase

        edges: root.cfg.auraEdges
        depth: root.cfg.auraDepth
        span: root.cfg.auraSpan
        taper: root.cfg.auraTaper
        cornerRadius: root.cfg.auraCornerRadius
        join: root.cfg.auraJoin
        cornerBlend: root.cfg.auraCornerBlend
        flow: root.cfg.auraFlow
        material: root.cfg.auraMaterial
        shape: root.cfg.auraShape
        effect: root.cfg.auraEffect
        effectStrength: root.cfg.auraEffectStrength
        colorMode: root.cfg.auraColorMode
        colorSpeed: root.cfg.auraColorSpeed
        bodyOpacity: root.cfg.auraBodyOpacity
        crestStrength: root.cfg.auraCrestStrength
        glow: root.cfg.auraGlow
        glowSpread: root.cfg.auraGlowSpread
        audioRange: root.cfg.auraAudioRange
        thickness: root.cfg.auraThickness
        detail: root.cfg.auraDetail
        bassDrive: root.cfg.auraBassDrive
        trebleDrive: root.cfg.auraTrebleDrive
        transientStrength: root.cfg.auraTransient
        beatGlow: root.cfg.auraBeatGlow
        compression: root.cfg.auraCompression
        sensitivity: root.cfg.auraSensitivity
        idleMotion: root.cfg.auraIdleMotion
        pulseStrength: 0.9
        primaryColor: root.auraTriad[0]
        secondaryColor: root.auraTriad[1]
        tertiaryColor: root.auraTriad[2]
        opacity: root.cfg.auraOpacity
    }
}

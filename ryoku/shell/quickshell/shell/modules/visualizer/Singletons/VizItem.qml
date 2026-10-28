pragma ComponentBehavior: Bound
import QtQuick

// One visualiser's settings, normalised. A plain JSON object goes in as `data`
// (the primary's flat keys, or one entry of the extras list); reactive,
// defaulted, derived properties come out. The renderer (VisualizerView, Motion)
// reads a VizItem rather than the Config singleton, so one draw path serves
// every instance on the desktop. Config builds one of these for the instance
// being edited; the Visualizer surface builds one per instance it paints.
QtObject {
    id: item

    // The raw per-viz object. Missing keys fall back to the canonical defaults
    // below, so a half-written extras entry never paints black or nothing.
    property var data: ({})

    function val(key, def) {
        var d = item.data;
        return (d && d[key] !== undefined && d[key] !== null) ? d[key] : def;
    }

    readonly property string instanceId: "" + item.val("id", "")

    // The looks the renderer knows; the polar three sit at the tail so one index
    // decides the family, `frame` is the whole-screen bar ring, and `aura` is the
    // whole-screen edge field drawn by its own shader rather than the SDF one.
    readonly property var knownStyles: ["bars", "split", "dots", "segments", "wave",
                                        "ribbon", "curtain", "line", "frame", "radial",
                                        "orb", "spiral", "aura"]

    readonly property string rawStyle: "" + item.val("style", "bars")
    readonly property string styleId: item.knownStyles.indexOf(item.rawStyle) >= 0 ? item.rawStyle
        : (item.rawStyle === "circle" ? "orb" : "bars")
    readonly property bool isPolar: item.styleId === "radial" || item.styleId === "orb"
        || item.styleId === "spiral"
    readonly property bool isAura: item.styleId === "aura"
    readonly property bool peaksApply: item.styleId === "bars" || item.styleId === "segments"
        || item.styleId === "frame"
    readonly property bool mirrorApplies: !item.isPolar && !item.isAura && item.styleId !== "frame"

    readonly property string shape:  "" + item.val("shape", "rounded")
    readonly property int bars:      Math.max(16, Math.min(128, Math.round(item.val("bars", 64))))
    readonly property real thickness: item.val("thickness", 0.58)
    readonly property real bloom:    item.val("bloom", 0.6)
    readonly property real reflection: item.val("reflection", 0.1)
    readonly property bool idleWave: item.val("idleWave", true)
    readonly property bool mirror:   item.val("mirror", false)
    readonly property int segments:  item.val("segments", 10)
    readonly property real gain:     item.val("gain", 1.0)
    readonly property real smoothing: item.val("smoothing", 0.5)
    readonly property bool peaks:    item.val("peaks", false)
    readonly property real spin:     item.val("spin", 0)

    readonly property real x:    item.val("x", 0)
    readonly property real y:    item.val("y", 0.58)
    readonly property real w:    item.val("w", 1)
    readonly property real h:    item.val("h", 0.42)
    readonly property string grow: "" + item.val("grow", "up")
    readonly property real angle: item.val("angle", 0)
    readonly property real tiltX: item.val("tiltX", 0)
    readonly property real tiltY: item.val("tiltY", 0)

    // Colour: an exact pinned #rrggbb, or "" to follow the wallpaper/theme accent.
    // A gradient adds a second stop; both must be valid hex for it to apply.
    readonly property string rawColor:  "" + item.val("color", "")
    readonly property string rawColor2: "" + item.val("color2", "")
    readonly property bool hasCustomColor: /^#[0-9a-fA-F]{6}$/.test(item.rawColor)
    readonly property bool hasColor2: /^#[0-9a-fA-F]{6}$/.test(item.rawColor2)
    readonly property color customColor: item.hasCustomColor ? item.rawColor : "#a7c080"
    readonly property color color2Value: item.hasColor2 ? item.rawColor2 : "#7fae52"
    readonly property bool gradient: item.val("gradient", false) === true
        && item.hasCustomColor && item.hasColor2

    // ── the aura look ────────────────────────────────────────────────────
    // The edge field's knobs ride flat `aura`-prefixed keys, like every other
    // look knob. Each reader falls back to the default of the iNiR edge
    // visualiser it ports, so a half-written entry never paints black.
    // Which screen edges carry the field; unset lights the two vertical rails.
    readonly property var auraEdges: {
        var e = item.val("auraEdges", null);
        return (e && e.length) ? e : ["left", "right"];
    }
    readonly property real auraDepth:          item.val("auraDepth", 180)
    readonly property real auraSpan:           item.val("auraSpan", 1.0)
    readonly property real auraTaper:          item.val("auraTaper", 0.14)
    readonly property real auraCornerRadius:   item.val("auraCornerRadius", 24)
    readonly property string auraJoin:         "" + item.val("auraJoin", "auto")
    readonly property real auraCornerBlend:    item.val("auraCornerBlend", 0.55)
    readonly property string auraFlow:         "" + item.val("auraFlow", "clockwise")
    readonly property string auraMaterial:     "" + item.val("auraMaterial", "silk")
    readonly property string auraShape:        "" + item.val("auraShape", "flow")
    readonly property string auraEffect:       "" + item.val("auraEffect", "clean")
    readonly property real auraEffectStrength: item.val("auraEffectStrength", 0.38)
    readonly property string auraColorMode:    "" + item.val("auraColorMode", "flow")
    readonly property real auraColorSpeed:     item.val("auraColorSpeed", 0.35)
    readonly property real auraBodyOpacity:    item.val("auraBodyOpacity", 0.32)
    readonly property real auraCrestStrength:  item.val("auraCrestStrength", 0.9)
    readonly property real auraGlow:           item.val("auraGlow", 0.52)
    readonly property real auraGlowSpread:     item.val("auraGlowSpread", 0.48)
    readonly property real auraAudioRange:     item.val("auraAudioRange", 0.78)
    readonly property real auraThickness:      item.val("auraThickness", 0.22)
    readonly property real auraDetail:         item.val("auraDetail", 0.42)
    readonly property real auraBassDrive:      item.val("auraBassDrive", 0.88)
    readonly property real auraTrebleDrive:    item.val("auraTrebleDrive", 0.68)
    readonly property real auraTransient:      item.val("auraTransient", 0.9)
    readonly property real auraBeatGlow:       item.val("auraBeatGlow", 0.64)
    readonly property real auraCompression:    item.val("auraCompression", 0.12)
    readonly property real auraMotionSpeed:    item.val("auraMotionSpeed", 1.0)
    readonly property real auraIdleMotion:     item.val("auraIdleMotion", 0.14)
    readonly property real auraAttack:         item.val("auraAttack", 1.05)
    readonly property real auraRelease:        item.val("auraRelease", 0.82)
    readonly property string auraProfile:      "" + item.val("auraProfile", "flat")
    readonly property real auraAccent:         item.val("auraAccent", 0.7)
    readonly property real auraSensitivity:    item.val("auraSensitivity", 0.72)
    readonly property real auraOpacity:        item.val("auraOpacity", 1.0)

    // The triad is the field's ramp: the shared pinned `color` is its first
    // stop, and the aura stops join it. With no complete triad pinned the
    // field paints the wallpaper-lit one.
    readonly property string rawAuraColor2: "" + item.val("auraColor2", "")
    readonly property string rawAuraColor3: "" + item.val("auraColor3", "")
    readonly property bool hasAuraTriad: /^#[0-9a-fA-F]{6}$/.test(item.rawColor)
        && /^#[0-9a-fA-F]{6}$/.test(item.rawAuraColor2)
        && /^#[0-9a-fA-F]{6}$/.test(item.rawAuraColor3)
    readonly property color auraColor2: /^#[0-9a-fA-F]{6}$/.test(item.rawAuraColor2) ? item.rawAuraColor2 : "#64dbcf"
    readonly property color auraColor3: /^#[0-9a-fA-F]{6}$/.test(item.rawAuraColor3) ? item.rawAuraColor3 : "#ffb2cf"
}

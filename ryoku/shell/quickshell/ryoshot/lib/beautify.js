// Beautify's model: the preset backgrounds, the named looks, the default
// style, and the stage geometry (the composition's size, where it rests on
// the output, and the lift between the selection and that rest).

var presets = [
    { a: "#4facfe", b: "#00c6a7", ang: 135 },
    { a: "#f6543e", b: "#f7b733", ang: 135 },
    { a: "#7b4397", b: "#dc2430", ang: 135 },
    { a: "#11998e", b: "#38ef7d", ang: 135 },
    { a: "#ee9ca7", b: "#ffdde1", ang: 135 },
    { a: "#334155", b: "#0f172a", ang: 135 },
    { a: "#e2342a", b: "#14120f", ang: 135 },
    { a: "#00c3ff", b: "#ffff1c", ang: 135 },
    { a: "#ffecd2", b: "#fcb69f", ang: 135 },
    { a: "#3e6868", b: "#0f1514", ang: 135 },
    { a: "#ff6a88", b: "#ff99ac", ang: 135 },
    { a: "#141e30", b: "#243b55", ang: 135 }
];

var palette = [
    "#ffffff", "#0e0d0b", "#e2342a", "#f3701e", "#f7b733", "#38ef7d",
    "#4facfe", "#2b6cb0", "#7b4397", "#ee9ca7", "#3e6868", "#c7bfae"
];

var ratios = {
    "auto": 0, "1:1": 1, "4:3": 4 / 3, "3:2": 1.5, "16:9": 16 / 9, "9:16": 9 / 16,
    "x": 16 / 9, "instagram": 1, "story": 9 / 16, "linkedin": 1.91, "youtube": 16 / 9, "pinterest": 2 / 3
};

// Ember is also the style a first session starts from.
var defaults = {
    bgKind: "preset", bgPreset: 6, bgSolid: "#2b3350", bgGradA: "#4facfe", bgGradB: "#00c6a7",
    bgGradAngle: 135, bgImage: "", padding: 90, roundness: 24, borderW: 0, borderColor: "#ffffff",
    shadowStrength: 45, shadowBlur: 55, shadowDist: 20, shadowAngle: 90,
    adjBright: 0, adjContrast: 0, adjSat: 0, ratioKey: "auto", watermark: false
};

var styleKeys = Object.keys(defaults);

// A look writes only the keys it names, so Bare strips the frame and leaves
// the size, the adjustments and the output alone.
var looks = [
    { id: "ember", name: "Ember", cfg: { bgKind: "preset", bgPreset: 6, padding: 90, roundness: 24, borderW: 0, shadowStrength: 45, shadowBlur: 55, shadowDist: 20, shadowAngle: 90 } },
    { id: "ocean", name: "Ocean", cfg: { bgKind: "preset", bgPreset: 0, padding: 96, roundness: 26, borderW: 0, shadowStrength: 42, shadowBlur: 60, shadowDist: 22, shadowAngle: 100 } },
    { id: "sunset", name: "Sunset", cfg: { bgKind: "preset", bgPreset: 1, padding: 88, roundness: 22, borderW: 0, shadowStrength: 50, shadowBlur: 50, shadowDist: 24, shadowAngle: 110 } },
    { id: "mono", name: "Mono", cfg: { bgKind: "solid", bgSolid: "#14120f", padding: 72, roundness: 16, borderW: 0, shadowStrength: 32, shadowBlur: 42, shadowDist: 16, shadowAngle: 90 } },
    { id: "paper", name: "Paper", cfg: { bgKind: "solid", bgSolid: "#f5efe4", padding: 64, roundness: 12, borderW: 1, borderColor: "#dcd3c2", shadowStrength: 18, shadowBlur: 30, shadowDist: 10, shadowAngle: 90 } },
    { id: "bare", name: "Bare", cfg: { bgKind: "none", padding: 0, roundness: 0, borderW: 0, shadowStrength: 0 } }
];

// Every key the style lacks takes its default.
function complete(style) {
    var s = {};
    for (var i = 0; i < styleKeys.length; i++) {
        var k = styleKeys[i];
        s[k] = style && style[k] !== undefined && style[k] !== null ? style[k] : defaults[k];
    }
    return s;
}

function sameValue(a, b) {
    if (typeof a === "number" || typeof b === "number")
        return Math.abs(Number(a) - Number(b)) < 0.5;
    return String(a).toLowerCase() === String(b).toLowerCase();
}

// A look is chosen while every value it names still holds.
function matches(style, cfg) {
    for (var k in cfg) {
        if (style[k] === undefined || !sameValue(style[k], cfg[k]))
            return false;
    }
    return true;
}

// The background a style resolves to: a gradient (a preset or a custom one),
// a solid, an image or nothing.
function background(style) {
    var kind = style.bgKind;
    if (kind === "preset") {
        var p = presets[Math.max(0, Math.min(presets.length - 1, Math.round(style.bgPreset)))];
        return { kind: "gradient", a: p.a, b: p.b, angle: p.ang, solid: "", image: "" };
    }
    if (kind === "gradient")
        return { kind: "gradient", a: String(style.bgGradA), b: String(style.bgGradB), angle: style.bgGradAngle, solid: "", image: "" };
    if (kind === "solid")
        return { kind: "solid", a: "", b: "", angle: 0, solid: String(style.bgSolid), image: "" };
    if (kind === "image")
        return { kind: "image", a: "", b: "", angle: 0, solid: "", image: String(style.bgImage || "") };
    return { kind: "none", a: "", b: "", angle: 0, solid: "", image: "" };
}

function clamp(v, lo, hi) {
    return hi < lo ? lo : Math.max(lo, Math.min(v, hi));
}

// The composition's size for a shot of w x h at a padding and a ratio key:
// the padded shot, widened or heightened to the ratio.
function frame(w, h, padding, ratioKey) {
    var minW = w + 2 * padding, minH = h + 2 * padding;
    var r = ratios[ratioKey] || 0;
    if (r <= 0)
        return { w: minW, h: minH };
    if (r >= minW / minH)
        return { w: minH * r, h: minH };
    return { w: minW, h: minW / r };
}

// The free area beside the panel: the output less the panel, its edge margin
// and the stage margin, less what the readouts need on each side.
// out {w,h}; panel {w, edge}; margin; reserve {top,right,bottom,left}
function freeArea(out, panel, margin, reserve) {
    var r = reserve || {};
    var x = margin + (r.left || 0);
    var y = margin + (r.top || 0);
    var right = out.w - panel.w - panel.edge - margin - (r.right || 0);
    var bottom = out.h - margin - (r.bottom || 0);
    return { x: x, y: y, w: Math.max(1, right - x), h: Math.max(1, bottom - y) };
}

// Where the composition rests: no larger than the selection shows the shot
// (scale0, logical px per image px), shrunk to fit the free area, centred on
// the selection's centre and then clamped into the area.
// comp {w,h} image px; centre {x,y} logical; area {x,y,w,h}
function rest(comp, scale0, centre, area) {
    var fit = Math.min(area.w / comp.w, area.h / comp.h);
    var s = Math.min(fit, scale0);
    var hw = comp.w * s / 2, hh = comp.h * s / 2;
    return {
        scale: s,
        x: clamp(centre.x, area.x + hw, area.x + area.w - hw),
        y: clamp(centre.y, area.y + hh, area.y + area.h - hh)
    };
}

function lerp(a, b, t) {
    return a + (b - a) * t;
}

// The padding a drag sets: the pointer's travel past the edge, in image px at
// the scale shown, on top of where the drag began.
function dragPadding(start, travel, scale) {
    return Math.round(clamp(start + travel / Math.max(scale, 1e-6), 0, 240));
}

if (typeof module !== "undefined")
    module.exports = { presets: presets, palette: palette, ratios: ratios, defaults: defaults, styleKeys: styleKeys,
        looks: looks, complete: complete, matches: matches, background: background, frame: frame,
        freeArea: freeArea, rest: rest, lerp: lerp, clamp: clamp, dragPadding: dragPadding };

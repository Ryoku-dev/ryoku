pragma ComponentBehavior: Bound
import QtQuick
import "lib/spectrum.js" as SpectrumMath
import "lib/place.js" as PlaceMath

// The audio spectrum, drawn once for the whole desktop. Every look lives in
// shaders/spectrum.frag; this turns a band array, a palette and a set of
// normalised knobs into that shader's uniforms, and sizes the pass to the region
// the look occupies so no fragment is spent on empty screen.
//
// It owns geometry, not motion and not colour: the desktop feeds it cava and a
// wallpaper-lit ramp, the Hub feeds it a synthetic signal and ink tones, and
// both get the same picture. Fill it with the surface the spectrum belongs to.
Item {
    id: root

    // 0..1 per band, already eased. The `line` look reads the same slots as a
    // waveform centred on 0.5.
    property var levels: []
    property var peaks: []
    property real energy: 0
    property real fade: 1
    // up to eight colour stops swept along the spectrum; short lists hold.
    property var ramp: []

    property string style: "bars"
    property string shape: "rounded"
    property real thickness: 0.58        // band width, fraction of its slot
    property real reflection: 0
    property int segments: 10
    property bool peakCaps: false
    property real glow: 0.6
    property real spin: 0                // degrees, integrated by the caller; negative turns the other way
    property bool flipped: false         // polar looks only: the whole ring as seen in a mirror

    // The box the look lives in, as fractions of this surface: move it and size
    // it anywhere. Bands grow from the box edge named by `grow`; a polar look
    // centres in the box and takes its radius from the shorter side.
    property real boxX: 0
    property real boxY: 0.58
    property real boxW: 1
    property real boxH: 0.42
    property string grow: "up"           // up | down | center | left | right
    // The whole look turns about the box centre, so a spectrum can sit at any
    // angle rather than only along an axis. The box stays axis-aligned: turning
    // the drawn pass costs one transform, where turning the geometry would mean
    // rotating every band's maths and the reflection with it.
    property real angle: 0               // degrees, clockwise
    // Bounded well short of edge-on: a look flat to the viewer cannot be seen.
    property real tiltX: 0               // degrees, lean the far edge back
    property real tiltY: 0               // degrees, lean one side back

    readonly property var styles: ["bars", "split", "dots", "segments", "wave",
                                   "ribbon", "curtain", "line", "frame", "radial", "orb", "spiral"]
    // An unknown look falls back to bars rather than painting nothing.
    readonly property int styleIndex: Math.max(0, root.styles.indexOf(root.style))
    // The polar three sit at the tail; the frame is a whole-screen edge look.
    readonly property bool polar: root.styleIndex >= 9
    readonly property bool frame: root.style === "frame"
    readonly property bool vertical: root.grow === "left" || root.grow === "right"
    readonly property bool centred: (root.grow === "center" && root.style !== "curtain")
        || root.style === "split"

    readonly property int bands: Math.max(1, Math.min(128, root.levels ? root.levels.length : 0))
    readonly property real ui: Math.max(0.75, Math.min(2.5, Math.min(width, height) / 900))

    // the box in px, clamped so a dragged look always keeps a usable size
    readonly property real bw: Math.max(24, root.boxW * root.width)
    readonly property real bh: Math.max(24, root.boxH * root.height)
    readonly property real bx: root.boxX * root.width
    readonly property real by: root.boxY * root.height

    readonly property real acrossFull: root.vertical ? root.bw : root.bh
    readonly property real alongFull: root.vertical ? root.bh : root.bw
    // The frame wraps the whole screen: its axis is the perimeter and its bars
    // grow inward up to a fraction of the shorter screen edge.
    readonly property real framePerim: 2 * (root.width + root.height)
    readonly property real frameDepth: Math.max(6, Math.min(root.width, root.height) * 0.06)
    readonly property real maxLen: root.frame ? root.frameDepth
        : Math.max(2, Math.round(root.acrossFull - root.reflectPx))
    readonly property real reflectPx: (root.grow === "up" && !root.polar && !root.centred)
        ? Math.round(root.acrossFull * root.reflection) : 0

    // A ring only has so much circumference: 64 bands around a legible radius
    // leaves 4px slivers that read as fur rather than bars, so a polar look folds
    // the spectrum down to what fits (about 14px a band, scaled with the screen)
    // and averages the rest in.
    readonly property int drawBands: root.polar
        ? Math.max(10, Math.min(root.bands, Math.floor(2 * Math.PI * root.r0 / (14 * root.ui))))
        : root.bands

    // The bloom is sized to the shape it wraps, never to the screen: a skirt
    // wider than the gap between two bands dissolves the spectrum into one mass.
    readonly property real slotPx: (root.frame ? root.framePerim : root.alongFull) / root.drawBands
    readonly property real shapePx: root.frame
        ? Math.max(2, root.framePerim / (root.drawBands * 3) * root.thickness)
        : (root.polar
           ? Math.max(3, 2 * Math.PI * root.r0 / root.drawBands * root.thickness)
           : Math.max(2, root.slotPx * root.thickness))
    readonly property real glowPx: Math.max(1.5, Math.min(root.shapePx * 0.9, 12 * root.ui))
        * (0.35 + 0.65 * Math.max(0, Math.min(1, root.glow)))
    readonly property real margin: Math.ceil(root.glow > 0 ? root.glowPx * 3.5 + 2 : 2 * root.ui)

    // A polar look centres in the box and takes its radius from the shorter side,
    // so a square box shows the whole shape. The orb is mostly core and a little
    // travel, so it breathes; the rings are mostly travel.
    readonly property real radius: Math.max(6, Math.min(root.bw, root.bh) / 2)
    readonly property real r0: root.radius * (root.style === "orb" ? 0.62 : 0.38)
    readonly property real rMax: root.radius - root.r0

    // The box plus room for the bloom to fall off, centred on the box so a turn
    // about the pass centre is a turn about the box centre.
    readonly property rect passRect: Qt.rect(pass.x, pass.y, pass.width, pass.height)
    // The box itself, for a host drawing placement guides.
    readonly property rect boxRect: root.frame ? Qt.rect(0, 0, root.width, root.height)
        : Qt.rect(root.bx, root.by, root.bw, root.bh)
    // What the turned look covers, for a host sampling the wallpaper under it.
    readonly property rect coverRect: {
        if (root.frame)
            return Qt.rect(0, 0, root.width, root.height);
        var a = root.angle * Math.PI / 180;
        var c = Math.abs(Math.cos(a)), s = Math.abs(Math.sin(a));
        var w = root.bw * c + root.bh * s;
        var h = root.bw * s + root.bh * c;
        return Qt.rect(root.bx + (root.bw - w) / 2, root.by + (root.bh - h) / 2, w, h);
    }

    Item {
        id: pass

        // The frame owns the whole screen and never turns; every other look sits
        // in its box plus room for the bloom to fall off, centred so a turn about
        // the pass centre is a turn about the box centre.
        x: root.frame ? 0 : root.bx - root.margin
        y: root.frame ? 0 : root.by - root.margin
        width: root.frame ? root.width : root.bw + 2 * root.margin
        height: root.frame ? root.height : root.bh + 2 * root.margin
        transform: [
            Scale {
                origin.x: pass.width / 2
                xScale: root.polar && root.flipped ? -1 : 1
            },
            Matrix4x4 {
                matrix: {
                    var m = root.frame
                        ? PlaceMath.flat(PlaceMath.boxMatrix(0, 0, 0, pass.width, pass.height))
                        : PlaceMath.flat(PlaceMath.boxMatrix(root.angle, root.tiltX, root.tiltY,
                                                             pass.width, pass.height));
                    return Qt.matrix4x4(m[0], m[1], m[2], m[3], m[4], m[5], m[6], m[7],
                                        m[8], m[9], m[10], m[11], m[12], m[13], m[14], m[15]);
                }
            }
        ]

        ShaderEffect {
            id: fx
            anchors.fill: parent
            visible: root.fade > 0.002 && root.bands > 0 && root.width > 1 && root.height > 1
            blending: true
            fragmentShader: Qt.resolvedUrl("shaders/spectrum.frag.qsb")

            property real style: root.styleIndex
            property real posMode: root.frame ? 0
                : (root.grow === "down" ? 1
                : (root.grow === "center" ? 2
                : (root.grow === "left" ? 3 : (root.grow === "right" ? 4 : 0))))
            property real pad: root.frame ? 0 : root.margin
            property real bands: root.drawBands
            property real maxLen: root.maxLen
            property real minLen: Math.max(1.5, 2 * root.ui) * (root.fade > 0.9 ? 1 : root.fade)
            property real thickness: Math.max(0.05, Math.min(1, root.thickness))
            property real shapeW: root.shapePx
            property real capR: root.shape === "rounded" ? root.shapePx * 0.5
                                                         : Math.min(2 * root.ui, root.shapePx * 0.2)
            property real segN: Math.max(3, Math.min(24, root.segments))
            property real segGap: Math.max(1.5 * root.ui, root.maxLen / Math.max(3, root.segments) * 0.26)
            property real gapPx: Math.max(2 * root.ui, root.maxLen * 0.06)
            property real glowAmt: Math.max(0, Math.min(1, root.glow))
            property real glowPx: root.glowPx
            property real reflectPx: root.reflectPx
            property real peakOn: root.peakCaps ? 1 : 0
            property real r0: root.r0
            property real rMax: root.rMax
            property real spinRad: root.spin * Math.PI / 180
            property real energy: Math.max(0, Math.min(1, root.energy))
            property real fade: Math.max(0, Math.min(1, root.fade))
            property real aa: 0.85
            property vector2d res: Qt.vector2d(width, height)
            property vector2d origin: Qt.vector2d(root.margin + root.bw / 2,
                                                  root.margin + root.bh / 2)

            property vector4d c0: root.stop(0)
            property vector4d c1: root.stop(1)
            property vector4d c2: root.stop(2)
            property vector4d c3: root.stop(3)
            property vector4d c4: root.stop(4)
            property vector4d c5: root.stop(5)
            property vector4d c6: root.stop(6)
            property vector4d c7: root.stop(7)

            property matrix4x4 lv0
            property matrix4x4 lv1
            property matrix4x4 lv2
            property matrix4x4 lv3
            property matrix4x4 lv4
            property matrix4x4 lv5
            property matrix4x4 lv6
            property matrix4x4 lv7
            property matrix4x4 pk0
            property matrix4x4 pk1
            property matrix4x4 pk2
            property matrix4x4 pk3
            property matrix4x4 pk4
            property matrix4x4 pk5
            property matrix4x4 pk6
            property matrix4x4 pk7
        }
    }

    // A short ramp holds its last colour, so a two-colour palette still sweeps.
    function stop(i) {
        var r = root.ramp;
        if (!r || r.length === 0)
            return Qt.vector4d(1, 1, 1, 1);
        var c = r[Math.min(i, r.length - 1)];
        return Qt.vector4d(c.r, c.g, c.b, 1);
    }

    // Sixteen levels to a mat4, in the row-major order the shader unpacks.
    function chunk(src, base) {
        var v = new Array(16);
        for (var i = 0; i < 16; i++) {
            var x = (src && base + i < src.length) ? src[base + i] : 0;
            v[i] = (typeof x === "number" && x === x) ? x : 0;
        }
        return Qt.matrix4x4(v[0], v[1], v[2], v[3], v[4], v[5], v[6], v[7],
                            v[8], v[9], v[10], v[11], v[12], v[13], v[14], v[15]);
    }

    // Only the matrices the band count reaches are written, so 64 bands cost
    // four assignments a frame rather than sixty-four. A folded ring averages
    // its groups first, so quiet bands still pull their bar down instead of
    // being dropped.
    function push() {
        var lv = root.levels;
        var pk = root.peaks;
        if (root.drawBands < root.bands) {
            lv = SpectrumMath.resample(lv, root.drawBands);
            if (root.peakCaps)
                pk = SpectrumMath.resample(pk, root.drawBands);
        }
        var n = Math.ceil(root.drawBands / 16);
        for (var i = 0; i < n; i++)
            fx["lv" + i] = root.chunk(lv, i * 16);
        if (root.peakCaps)
            for (var j = 0; j < n; j++)
                fx["pk" + j] = root.chunk(pk, j * 16);
    }

    onLevelsChanged: root.push()
    onPeaksChanged: if (root.peakCaps) root.push()
    Component.onCompleted: root.push()
}

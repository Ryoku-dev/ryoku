import QtQuick
import QtQuick.Shapes
import Qt5Compat.GraphicalEffects
import "Singletons"
import "look"
import "lib/redact.js" as Redact

// The annotations that read the captured pixels back instead of drawing over
// them: blur, redaction, the zoom lens and the spotlight. AnnLayer strokes
// vectors and never samples the frame, so the two stay apart.
Item {
    id: fx

    required property Item frame
    required property int sx
    required property int sy

    property var model: null
    property var draft: null
    property int revision: 0

    readonly property var kinds: ["blur", "pixelate", "redact", "magnify", "spotlight"]

    function isEffect(t) { return fx.kinds.indexOf(t) !== -1; }

    function items() {
        var src = fx.model ? fx.model.items : [];
        var out = [];
        for (var i = 0; i < src.length; i++)
            if (src[i] && fx.isEffect(src[i].type)) out.push(src[i]);
        if (fx.draft && fx.isEffect(fx.draft.type)) out.push(fx.draft);
        return out;
    }

    function spotlights() {
        var all = fx.items(), out = [];
        for (var i = 0; i < all.length; i++)
            if (all[i].type === "spotlight") out.push(all[i]);
        return out;
    }

    // Cap the block count so redacting a whole monitor cannot spawn tens of
    // thousands of rectangles; the coarser block is still a redaction.
    function blockFor(w, h) {
        var wanted = Config.mosaicBlock > 0 ? Config.mosaicBlock : 12;
        return Math.max(wanted, Math.ceil(Math.sqrt(Math.max(1, w * h) / 4000)));
    }

    Repeater {
        id: effects
        model: { fx.revision; return fx.items(); }

        Item {
            id: cell
            required property var modelData
            readonly property var a: modelData
            readonly property bool valid: a !== undefined && a !== null
                && a.points !== undefined && a.points.length >= 2
            readonly property real rx: valid ? Math.min(a.points[0].x, a.points[1].x) - fx.sx : 0
            readonly property real ry: valid ? Math.min(a.points[0].y, a.points[1].y) - fx.sy : 0
            readonly property real rw: valid ? Math.abs(a.points[1].x - a.points[0].x) : 0
            readonly property real rh: valid ? Math.abs(a.points[1].y - a.points[0].y) : 0
            readonly property string kind: valid ? a.type : ""
            readonly property bool isRedact: kind === "redact" || kind === "pixelate"
            readonly property bool isMosaic: isRedact && a.style !== "solid"
            readonly property bool isMag: kind === "magnify"
            readonly property real magD: Math.min(rw, rh)
            readonly property real magZoom: Config.zoomFactor > 0 ? Config.zoomFactor : 2.0

            x: rx
            y: ry
            width: rw
            height: rh
            visible: valid && rw > 0 && rh > 0 && kind !== "spotlight"
            clip: true

            ShaderEffectSource {
                id: blurSrc
                anchors.fill: parent
                sourceItem: fx.frame
                live: false
                onSourceRectChanged: scheduleUpdate()
                recursive: false
                sourceRect: Qt.rect(cell.rx, cell.ry, cell.rw, cell.rh)
                visible: false
            }

            FastBlur {
                anchors.fill: parent
                source: blurSrc
                radius: Config.blurRadius
                visible: cell.kind === "blur"
            }

            // A redaction with no sampled palette yet paints solid, so the
            // source is never briefly readable while the sampler runs.
            Rectangle {
                anchors.fill: parent
                visible: cell.isRedact && (!cell.isMosaic || !cell.a.pal || cell.a.pal.length === 0)
                color: Look.redactSolid
            }

            Repeater {
                model: {
                    if (!cell.isMosaic || !cell.a.pal || cell.a.pal.length === 0) return [];
                    var block = fx.blockFor(cell.rw, cell.rh);
                    var seed = cell.a.seed || Redact.seedFor({ x: cell.rx, y: cell.ry, w: cell.rw, h: cell.rh });
                    return Redact.blockPlan(cell.rw, cell.rh, block, seed, cell.a.pal.length);
                }

                Rectangle {
                    required property var modelData
                    x: modelData.x
                    y: modelData.y
                    width: modelData.w
                    height: modelData.h
                    color: cell.a.pal[modelData.index % cell.a.pal.length]
                }
            }

            ShaderEffectSource {
                id: magSrc
                width: cell.magD
                height: cell.magD
                anchors.centerIn: parent
                sourceItem: fx.frame
                live: false
                onSourceRectChanged: scheduleUpdate()
                recursive: false
                sourceRect: Qt.rect(cell.rx + cell.rw / 2 - cell.magD / (2 * cell.magZoom),
                                    cell.ry + cell.rh / 2 - cell.magD / (2 * cell.magZoom),
                                    cell.magD / cell.magZoom, cell.magD / cell.magZoom)
                visible: false
            }

            Rectangle {
                id: magMask
                width: cell.magD
                height: cell.magD
                anchors.centerIn: parent
                radius: cell.magD / 2
                visible: false
                layer.enabled: true
            }

            OpacityMask {
                width: cell.magD
                height: cell.magD
                anchors.centerIn: parent
                source: magSrc
                maskSource: magMask
                visible: cell.isMag
            }

            Rectangle {
                width: cell.magD
                height: cell.magD
                anchors.centerIn: parent
                radius: cell.magD / 2
                color: "transparent"
                border.color: Look.ink
                border.width: Math.max(2, cell.magD * 0.03)
                visible: cell.isMag
            }
        }
    }

    // One dim pass for every spotlight, punched through by the union of their
    // shapes. Dimming per spotlight would darken twice where two overlap.
    Item {
        id: spotDim
        anchors.fill: parent
        visible: { fx.revision; return fx.spotlights().length > 0; }

        Item {
            id: holes
            anchors.fill: parent
            visible: false
            layer.enabled: true

            Repeater {
                model: { fx.revision; return fx.spotlights(); }

                LensShape {
                    required property var modelData
                    readonly property var a: modelData
                    x: Math.min(a.points[0].x, a.points[1].x) - fx.sx
                    y: Math.min(a.points[0].y, a.points[1].y) - fx.sy
                    width: Math.abs(a.points[1].x - a.points[0].x)
                    height: Math.abs(a.points[1].y - a.points[0].y)
                    shape: a.shape || "ellipse"
                    fill: Look.ink
                }
            }
        }

        Rectangle {
            id: dimPlate
            anchors.fill: parent
            color: Look.spotlightDim
            visible: false
            layer.enabled: true
        }

        OpacityMask {
            anchors.fill: parent
            source: dimPlate
            maskSource: holes
            invert: true
        }
    }

    Repeater {
        id: lenses
        model: { fx.revision; return fx.spotlights(); }

        Item {
            id: lens
            required property var modelData
            readonly property var a: modelData
            readonly property real lx: Math.min(a.points[0].x, a.points[1].x) - fx.sx
            readonly property real ly: Math.min(a.points[0].y, a.points[1].y) - fx.sy
            readonly property real lw: Math.abs(a.points[1].x - a.points[0].x)
            readonly property real lh: Math.abs(a.points[1].y - a.points[0].y)
            readonly property real zoom: Math.max(1.0, Math.min(4.0, a.magnification || 2.0))

            x: lx
            y: ly
            width: lw
            height: lh
            visible: lw > 1 && lh > 1

            // Sample a window one zoom factor smaller than the lens, kept
            // inside the frame so a lens near an edge does not sample past it.
            readonly property real sw: lw / zoom
            readonly property real sh: lh / zoom
            readonly property real sxp: Math.max(0, Math.min(fx.width - sw, lx + lw / 2 - sw / 2))
            readonly property real syp: Math.max(0, Math.min(fx.height - sh, ly + lh / 2 - sh / 2))

            ShaderEffectSource {
                id: lensSrc
                anchors.fill: parent
                sourceItem: fx.frame
                live: false
                onSourceRectChanged: scheduleUpdate()
                recursive: false
                sourceRect: Qt.rect(lens.sxp, lens.syp, lens.sw, lens.sh)
                visible: false
            }

            LensShape {
                id: lensMask
                anchors.fill: parent
                shape: lens.a.shape || "ellipse"
                fill: Look.ink
                visible: false
                layer.enabled: true
            }

            OpacityMask {
                anchors.fill: parent
                source: lensSrc
                maskSource: lensMask
            }

            LensShape {
                anchors.fill: parent
                shape: lens.a.shape || "ellipse"
                stroke: lens.a.color !== undefined ? lens.a.color : Look.ink
                strokeW: Math.max(2, (lens.a.width || 4) / 2)
            }
        }
    }
}

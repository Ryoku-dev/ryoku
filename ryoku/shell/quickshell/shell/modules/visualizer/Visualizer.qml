//@ pragma DefaultEnv QSG_RENDER_LOOP = threaded
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import "Singletons"

// desktop audio visualiser module entry. click-through, palette-tinted cava
// spectrum across the bottom of one monitor. mode drives visibility and layer:
// "off" hides it, "desktop" draws on the wallpaper behind every window
// (WlrLayer.Bottom), "overlay" raises it over windows (WlrLayer.Top). the
// controller instantiates one per screen and binds `screen`/`mode`; cava only
// runs while enabled.
Item {
    id: root

    // monitor this instance draws on, supplied by the controller.
    property var screen

    // off | desktop | overlay. defaults from Config so a standalone instance
    // still honours the persisted enabled flag; the controller overrides it.
    property string mode: Config.enabled ? "desktop" : "off"

    // The desktop hosts this look while the stage is on or the editor frames
    // the monitor. This separate surface steps aside while the shared feeds
    // remain owned by the hosted renderer.
    property bool suppressed: false

    readonly property bool active: root.mode !== "off"
    readonly property bool raised: root.mode === "overlay"

    function setting(data, key, fallback) {
        return data && data[key] !== undefined && data[key] !== null
            ? data[key] : fallback;
    }

    function joinRect(a, b) {
        if (!a)
            return b;
        if (!b)
            return a;
        var x0 = Math.min(a.x, b.x);
        var y0 = Math.min(a.y, b.y);
        var x1 = Math.max(a.x + a.width, b.x + b.width);
        var y1 = Math.max(a.y + a.height, b.y + b.height);
        return Qt.rect(x0, y0, x1 - x0, y1 - y0);
    }

    function auraRect(data, sw, sh) {
        var edges = root.setting(data, "auraEdges", ["left", "right"]);
        if (!edges || edges.length === 0)
            edges = ["left", "right"];
        var depth = Math.max(24, Number(root.setting(data, "auraDepth", 180)));
        var horizontalDepth = Math.min(depth, sh * 0.45) + 8;
        var verticalDepth = Math.min(depth, sw * 0.45) + 8;
        var out = null;
        if (edges.indexOf("top") >= 0)
            out = root.joinRect(out, Qt.rect(0, 0, sw, horizontalDepth));
        if (edges.indexOf("bottom") >= 0)
            out = root.joinRect(out, Qt.rect(0, sh - horizontalDepth, sw, horizontalDepth));
        if (edges.indexOf("left") >= 0)
            out = root.joinRect(out, Qt.rect(0, 0, verticalDepth, sh));
        if (edges.indexOf("right") >= 0)
            out = root.joinRect(out, Qt.rect(sw - verticalDepth, 0, verticalDepth, sh));
        return out || Qt.rect(0, 0, sw, sh);
    }

    function instanceRect(data, sw, sh) {
        var rawStyle = "" + root.setting(data, "style", "bars");
        var style = rawStyle === "circle" ? "orb" : rawStyle;
        if (style === "frame")
            return Qt.rect(0, 0, sw, sh);
        if (style === "aura")
            return root.auraRect(data, sw, sh);

        var nw = Math.max(0.04, Math.min(1, Number(root.setting(data, "w", 1))));
        var nh = Math.max(0.03, Math.min(1, Number(root.setting(data, "h", 0.42))));
        var bw = Math.max(24, nw * sw);
        var bh = Math.max(24, nh * sh);
        var bars = Math.max(16, Math.min(128,
            Math.round(Number(root.setting(data, "bars", 64)))));
        var thickness = Number(root.setting(data, "thickness", 0.58));
        var ui = Math.max(0.75, Math.min(2.5, Math.min(sw, sh) / 900));
        var polar = style === "radial" || style === "orb" || style === "spiral";
        var vertical = root.setting(data, "grow", "up") === "left"
            || root.setting(data, "grow", "up") === "right";
        var shapePx;
        if (polar) {
            var radius = Math.max(6, Math.min(bw, bh) / 2);
            var r0 = radius * (style === "orb" ? 0.62 : 0.38);
            var drawBands = Math.max(10, Math.min(bars,
                Math.floor(2 * Math.PI * r0 / (14 * ui))));
            shapePx = Math.max(3, 2 * Math.PI * r0 / drawBands * thickness);
        } else {
            var along = vertical ? bh : bw;
            shapePx = Math.max(2, along / bars * thickness);
        }
        var glow = Math.max(0, Math.min(1, Number(root.setting(data, "bloom", 0.6))));
        var glowPx = Math.max(1.5, Math.min(shapePx * 0.9, 12 * ui))
            * (0.35 + 0.65 * glow);
        var margin = Math.ceil(glow > 0 ? glowPx * 3.5 + 2 : 2 * ui);
        var passW = bw + margin * 2;
        var passH = bh + margin * 2;
        var angle = Number(root.setting(data, "angle", 0)) * Math.PI / 180;
        var c = Math.abs(Math.cos(angle));
        var s = Math.abs(Math.sin(angle));
        var rw = passW * c + passH * s;
        var rh = passW * s + passH * c;
        var cx = Number(root.setting(data, "x", 0)) * sw + bw / 2;
        var cy = Number(root.setting(data, "y", 0.58)) * sh + bh / 2;
        return Qt.rect(cx - rw / 2, cy - rh / 2, rw, rh);
    }

    function surfaceRect() {
        var sw = Math.max(1, root.screen ? root.screen.width : 1);
        var sh = Math.max(1, root.screen ? root.screen.height : 1);
        var out = null;
        var list = Config.list;
        for (var i = 0; i < list.length; i++)
            out = root.joinRect(out, root.instanceRect(list[i], sw, sh));
        if (!out)
            return Qt.rect(0, 0, sw, sh);
        var x0 = Math.max(0, Math.floor(out.x));
        var y0 = Math.max(0, Math.floor(out.y));
        var x1 = Math.min(sw, Math.ceil(out.x + out.width));
        var y1 = Math.min(sh, Math.ceil(out.y + out.height));
        return Qt.rect(x0, y0, Math.max(1, x1 - x0), Math.max(1, y1 - y0));
    }

    function hasPlacementPreview() {
        for (var key in Config.previews)
            return true;
        return false;
    }

    readonly property rect drawRect: root.surfaceRect()
    readonly property bool compactSurface: !root.raised
        && Spectrum.placementHolds === 0 && !root.hasPlacementPreview()

    PanelWindow {
        id: win

        screen: root.screen
        visible: root.active && !root.suppressed
        color: "transparent"

        // The curtain hangs off the bar, so its surface honours the bar's
        // exclusive zone and starts where the bar ends. Every other look owns
        // the whole screen and ignores reservations.
        exclusionMode: Config.styleId === "curtain" ? ExclusionMode.Normal : ExclusionMode.Ignore
        WlrLayershell.layer: root.raised ? WlrLayer.Top : WlrLayer.Bottom
        WlrLayershell.namespace: "ryoku-visualizer"
        WlrLayershell.keyboardFocus: WlrKeyboardFocus.None

        // empty input region: every click falls through to windows above, so the
        // visualiser shares the desktop without ever intercepting it. Placement
        // runs inside the lifted desktop surface (the Stage Editor's grip), not
        // here: a surface masked click-through does not start taking a pointer
        // again just because the region is swapped.
        mask: emptyRegion
        Region { id: emptyRegion }

        anchors {
            top: true
            left: true
            right: !root.compactSurface
            bottom: !root.compactSurface
        }
        margins.left: root.compactSurface ? root.drawRect.x : 0
        margins.top: root.compactSurface ? root.drawRect.y : 0
        implicitWidth: root.compactSurface ? root.drawRect.width : 1
        implicitHeight: root.compactSurface ? root.drawRect.height : 1

        // One view per instance: the primary plus every extra, each painting its
        // own look on the shared surface.
        Item {
            x: root.compactSurface ? -root.drawRect.x : 0
            y: root.compactSurface ? -root.drawRect.y : 0
            width: Math.max(1, root.screen ? root.screen.width : win.width)
            height: Math.max(1, root.screen ? root.screen.height : win.height)
            Repeater {
                id: rep
                model: Config.count
                delegate: VisualizerView {
                    id: vizView
                    anchors.fill: parent
                    instanceId: Config.idAt(vizView.index)
                    cfg: VizItem { data: Config.dataById(vizView.instanceId) }
                }
            }
        }
    }

}

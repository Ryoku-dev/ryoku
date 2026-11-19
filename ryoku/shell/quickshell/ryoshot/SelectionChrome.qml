import QtQuick
import Quickshell
import "look"
import "designs"
import "lib/cornerplace.js" as Place

// Everything drawn over one output's frozen frame: the scrim with the
// selection cut out of it, and the design's selection (frame, grips, readout,
// guides, window hover, annotation box and loupe). This file holds the state
// every design reads; each design only draws it.
Item {
    id: chrome

    property real u: 1
    property real dpr: 1
    property bool ready: false
    property Item plate: null
    property int sx: 0
    property int sy: 0
    property string screenName: ""
    // Beautify is up: the selection's frame and readouts step out
    property bool away: false

    property var globalSel: null
    property var pressPoint: null
    property bool front: false
    property bool capturing: false
    property bool editing: false
    property bool record: false
    property bool picking: false
    property string pickHex: ""
    property var annBox: null

    // the resize grip being dragged and the one under the pointer
    property string gripRole: ""
    property string hotGrip: ""

    property bool pointerIn: false
    property real px: 0
    property real py: 0

    // the window (or, by the Screen target, the output) a click would take
    property var hoverRect: null
    property bool hoverScreen: false

    readonly property var sel: chrome.globalSel
        ? { x: chrome.globalSel.x - chrome.sx, y: chrome.globalSel.y - chrome.sy, w: chrome.globalSel.w, h: chrome.globalSel.h }
        : null
    readonly property bool hasArea: chrome.sel !== null && chrome.sel.w >= 1 && chrome.sel.h >= 1
    readonly property bool dragging: chrome.front && chrome.capturing && chrome.sel !== null
    readonly property bool live: chrome.dragging || chrome.gripRole !== ""
    readonly property bool crosshair: chrome.ready && chrome.front && chrome.pointerIn

    // The corner being placed, on this output only, with the selection's
    // outward direction there; the readout and the loupe hang off it.
    readonly property var corner: {
        if (!chrome.live) return null;
        var c = Place.movingCorner(chrome.globalSel, chrome.pressPoint, chrome.gripRole);
        if (!c) return null;
        var lx = c.x - chrome.sx, ly = c.y - chrome.sy;
        if (lx < 0 || ly < 0 || lx > chrome.width || ly > chrome.height) return null;
        return { x: lx, y: ly, dx: c.dx, dy: c.dy };
    }
    // the loupe's point: the corner being placed, or the pointer while picking a colour
    readonly property var loupePoint: chrome.corner
        ? chrome.corner
        : (chrome.picking && chrome.pointerIn && chrome.ready ? { x: chrome.px, y: chrome.py, dx: 1, dy: 1 } : null)

    readonly property var hover: {
        var r = chrome.hoverRect;
        if (!r || chrome.globalSel !== null || !chrome.ready || chrome.picking) return null;
        var lx = r.x - chrome.sx, ly = r.y - chrome.sy;
        if (lx + r.w <= 0 || ly + r.h <= 0 || lx >= chrome.width || ly >= chrome.height) return null;
        return { x: lx, y: ly, w: r.w, h: r.h };
    }
    readonly property string hoverApp: {
        var r = chrome.hoverRect;
        if (!r) return "";
        if (chrome.hoverScreen) return chrome.screenName;
        var entry = r.appId ? DesktopEntries.heuristicLookup(r.appId) : null;
        if (entry && entry.name) return entry.name;
        if (r.appId) {
            var tail = r.appId.split(".").pop().replace(/[-_]+/g, " ");
            return tail.charAt(0).toUpperCase() + tail.slice(1);
        }
        return r.title || "";
    }

    // one device pixel, and a length snapped to the device pixel grid, so
    // hairlines stay crisp at a fractional output scale
    readonly property real hair: 1 / chrome.dpr
    // the scrim darkens the frame in light and dark alike, so lines drawn
    // straight on it take whichever of the design's ink and ground is lighter
    function luma(c) { return 0.2126 * c.r + 0.7152 * c.g + 0.0722 * c.b; }
    readonly property color lightInk: chrome.luma(Look.fg) > chrome.luma(Look.ground) ? Look.fg : Look.ground
    function snap(v) { return Math.round(v * chrome.dpr) / chrome.dpr; }
    function px2(n) { return Math.max(1, Math.round(n * chrome.u * chrome.dpr)) / chrome.dpr; }

    function sizeText(w, h) { return Math.round(w) + " \u00d7 " + Math.round(h); }
    readonly property string selSizeText: chrome.sel ? chrome.sizeText(chrome.sel.w, chrome.sel.h) : ""
    readonly property string originText: chrome.sel
        ? "+" + Math.round(chrome.sel.x) + " +" + Math.round(chrome.sel.y)
        : ""
    readonly property string pointText: chrome.loupePoint
        ? Math.round(chrome.loupePoint.x) + ", " + Math.round(chrome.loupePoint.y)
        : ""
    readonly property string hoverSizeText: chrome.hover ? chrome.sizeText(chrome.hover.w, chrome.hover.h) : ""

    // the readout and the loupe beside the corner, off the cursor and on screen
    function placeAround(point, readoutW, readoutH, loupeW, loupeH) {
        return Place.aroundCorner(point, { dx: point.dx, dy: point.dy },
            { w: readoutW, h: readoutH }, { w: loupeW, h: loupeH },
            { w: chrome.width, h: chrome.height }, 20 * chrome.u, 8 * chrome.u, 8 * chrome.u);
    }
    function placeBeside(point, w, h) {
        return Place.besidePoint(point, { w: w, h: h }, { w: chrome.width, h: chrome.height }, 20 * chrome.u, 8 * chrome.u);
    }

    Item {
        id: scrim
        anchors.fill: parent
        visible: chrome.ready
        readonly property real x0: chrome.hasArea ? Math.max(0, Math.min(chrome.width, chrome.snap(chrome.sel.x))) : 0
        readonly property real y0: chrome.hasArea ? Math.max(0, Math.min(chrome.height, chrome.snap(chrome.sel.y))) : 0
        readonly property real x1: chrome.hasArea ? Math.max(0, Math.min(chrome.width, chrome.snap(chrome.sel.x + chrome.sel.w))) : 0
        readonly property real y1: chrome.hasArea ? Math.max(0, Math.min(chrome.height, chrome.snap(chrome.sel.y + chrome.sel.h))) : 0

        Rectangle {
            color: Look.scrim
            width: parent.width
            height: chrome.hasArea ? scrim.y0 : parent.height
        }
        Rectangle {
            visible: chrome.hasArea
            color: Look.scrim
            y: scrim.y1
            width: parent.width
            height: parent.height - scrim.y1
        }
        Rectangle {
            visible: chrome.hasArea
            color: Look.scrim
            y: scrim.y0
            width: scrim.x0
            height: scrim.y1 - scrim.y0
        }
        Rectangle {
            visible: chrome.hasArea
            color: Look.scrim
            x: scrim.x1
            y: scrim.y0
            width: parent.width - scrim.x1
            height: scrim.y1 - scrim.y0
        }
    }

    Loader {
        id: designLoader
        // a design's own `chrome` property would shadow the id inside its Component
        readonly property Item host: chrome
        anchors.fill: parent
        active: chrome.ready
        visible: opacity > 0
        opacity: chrome.away ? 0 : 1
        Behavior on opacity {
            NumberAnimation { duration: Look.dur(chrome.away ? Look.exitMs : Look.enterMs); easing.type: Easing.OutCubic }
        }
        sourceComponent: Look.material ? materialLook
            : Look.adwaita ? adwaitaLook
            : Look.fluent ? fluentLook
            : lithicLook
    }

    Component { id: lithicLook; LithicSelection { chrome: designLoader.host } }
    Component { id: materialLook; MaterialSelection { chrome: designLoader.host } }
    Component { id: adwaitaLook; AdwaitaSelection { chrome: designLoader.host } }
    Component { id: fluentLook; FluentSelection { chrome: designLoader.host } }
}

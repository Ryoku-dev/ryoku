pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Shapes as Shapes
import Lithic
import Ryoku.Ui.Singletons
import "../ask/look"
import "lib/sections.js" as Sections

// The rail and the page's outline. Six tabs rest set back from the page, each
// with its one live figure; the open one is the page's own plane grown out
// sideways, drawn as a single path with it (no seam). Sizes: kit-layouts SPEC 2, 4.
Item {
    id: root

    required property real s
    property bool active: false
    // the section being opened, the one being left, and how far the change is
    property string to: ""
    property string from: ""
    property real k: 1
    // 1 while all of Kit is searched: no tab is out
    property real folded: 0
    // {id: label}, {id: {text, value, tone}}, {id: {text, tone}}
    property var labels: ({})
    property var figures: ({})
    property var lit: ({})
    // all of Kit: {id: count}, and the section of the selected result
    property bool counting: false
    property var counts: ({})
    property string chosen: ""

    readonly property real railWidth: Math.round(160 * root.s)
    readonly property real tabHeight: Math.round(64 * root.s)
    readonly property real tabPitch: Math.round(70 * root.s)
    readonly property real tabTop: Math.round(64 * root.s)
    readonly property color recess: Look.lithic ? Lithic.stone2
        : Look.material ? Lithic.mix(Lithic.stone4, Lithic.ground, 0.55)
        : Look.adwaita ? Look.planes.side
        : Lithic.mix(Look.planes.pane, Lithic.ground, 0.6)
    readonly property color recessStroke: Look.lithic ? Lithic.alpha(Lithic.vein, 0.8)
        : Look.fluent ? Lithic.alpha(Lithic.vein, 0.6)
        : Look.adwaita ? Lithic.alpha(Lithic.fg, Look.dark ? 0.08 : 0.1)
        : "transparent"
    readonly property color track: Lithic.alpha(Lithic.fg, Look.dark ? 0.13 : 0.14)

    signal picked(string id)

    function slotY(i) {
        return root.tabTop + i * root.tabPitch;
    }
    function openOf(i) {
        const out = i === Sections.index(root.to) ? (root.from.length > 0 ? root.k : 1)
            : root.from.length > 0 && i === Sections.index(root.from) ? 1 - root.k : 0;
        return Math.max(0, Math.min(1, out)) * (1 - root.folded);
    }
    function toneInk(tone, fallback) {
        return tone === "ok" ? Lithic.jade : tone === "wait" ? Lithic.amber
            : tone === "error" ? Lithic.jasper : tone === "busy" ? Lithic.lapis : fallback;
    }

    Shapes.Shape {
        id: outline
        anchors.fill: parent
        preferredRendererType: Shapes.Shape.CurveRenderer

        readonly property bool round: !Look.lithic
        readonly property real x0: root.railWidth + 0.5
        readonly property real x1: root.width - 0.5
        readonly property real y1: root.height - 0.5
        readonly property real frameCorner: outline.round
            ? Lithic.cornerRadius(Look.cutFrame * root.s, root.width - root.railWidth, root.height, Look.design)
            : Look.cutFrame * root.s
        readonly property real tabCorner: outline.round ? Math.min(outline.frameCorner, root.tabHeight / 3) : Look.cutCard * root.s
        readonly property real fillet: outline.round ? Math.min(outline.tabCorner, 10 * root.s) : Look.cutCard * root.s
        // every tab's reach, so the path follows each change of any of them
        readonly property var reach: [root.openOf(0), root.openOf(1), root.openOf(2), root.openOf(3), root.openOf(4), root.openOf(5)]

        function turn(r, x, y, convex) {
            return outline.round ? " A " + r + " " + r + " 0 0 " + (convex ? 1 : 0) + " " + x + " " + y : " L " + x + " " + y;
        }

        Shapes.ShapePath {
            fillColor: Look.frame
            strokeColor: Qt.colorEqual(Look.frameStroke, "transparent") ? Look.frame : Look.frameStroke
            strokeWidth: 1
            joinStyle: Shapes.ShapePath.MiterJoin
            PathSvg {
                path: {
                    const c = outline.frameCorner, x0 = outline.x0, x1 = outline.x1, y1 = outline.y1;
                    // Lithic's slip keeps its top-right and bottom-left square
                    const quiet = Look.lithic ? 0 : c;
                    let d = "M " + (x0 + c) + " 0.5 L " + (x1 - quiet) + " 0.5";
                    if (quiet > 0)
                        d += outline.turn(quiet, x1, 0.5 + quiet, true);
                    d += " L " + x1 + " " + (y1 - c) + outline.turn(c, x1 - c, y1, true);
                    d += " L " + (x0 + quiet) + " " + y1;
                    if (quiet > 0)
                        d += outline.turn(quiet, x0, y1 - quiet, true);
                    for (let i = outline.reach.length - 1; i >= 0; --i) {
                        const depth = outline.reach[i] * (x0 - 0.5);
                        if (depth < 2)
                            continue;
                        const t = Math.min(outline.tabCorner, depth / 2), f = Math.min(outline.fillet, depth / 2);
                        const xl = x0 - depth, ya = root.slotY(i) + 0.5, yb = ya + root.tabHeight;
                        d += " L " + x0 + " " + (yb + f) + outline.turn(f, x0 - f, yb, false);
                        d += " L " + (xl + t) + " " + yb + outline.turn(t, xl, yb - t, true);
                        d += " L " + xl + " " + (ya + t) + outline.turn(t, xl + t, ya, true);
                        d += " L " + (x0 - f) + " " + ya + outline.turn(f, x0, ya - f, false);
                    }
                    d += " L " + x0 + " " + (0.5 + c) + outline.turn(c, x0 + c, 0.5, true) + " Z";
                    return d;
                }
            }
        }
    }

    Repeater {
        model: Sections.ORDER

        delegate: Item {
            id: tab

            required property string modelData
            required property int index
            readonly property real open: root.openOf(tab.index)
            readonly property var figure: root.figures[tab.modelData] || ({ text: "", value: -1, tone: "" })
            readonly property var news: root.lit[tab.modelData] || null
            readonly property bool isLit: !root.counting && tab.news !== null && tab.open < 0.5
            readonly property int found: Number(root.counts[tab.modelData] || 0)
            readonly property bool dim: root.counting && tab.found === 0
            readonly property bool marked: root.counting ? root.chosen === tab.modelData : tab.open > 0.5
            readonly property string line: root.counting
                ? (tab.found === 0 ? I18n.tr("No match") : tab.found === 1 ? I18n.tr("1 match") : I18n.tr("%1 matches").arg(tab.found))
                : tab.isLit ? String(tab.news.text || "") : String(tab.figure.text || "")
            readonly property real level: root.counting || tab.isLit ? -1 : Number(tab.figure.value === undefined ? -1 : tab.figure.value)

            y: root.slotY(tab.index)
            width: root.railWidth
            height: root.tabHeight

            Shape {
                x: Math.round(8 * root.s)
                width: Math.round(144 * root.s)
                height: parent.height
                opacity: 1 - tab.open
                visible: opacity > 0.01
                s: root.s
                cutSize: Look.cutCard
                fill: tab.isLit ? Lithic.mix(root.recess, root.toneInk(String(tab.news.tone || ""), Lithic.jade), Look.dark ? 0.12 : 0.14)
                    : hover.containsMouse ? Lithic.mix(root.recess, Lithic.fg, Look.dark ? 0.05 : 0.04) : root.recess
                strokeColor: root.recessStroke
                strokeWidth: Qt.colorEqual(root.recessStroke, "transparent") ? 0 : 1
                Behavior on fill {
                    enabled: root.active
                    ColorAnimation { duration: Look.dur(Look.stateMs) }
                }
            }

            Item {
                x: Math.round((20 - 6 * tab.open) * root.s)
                width: Math.round(142 * root.s) - x
                height: parent.height

                Symbol {
                    y: Math.round(12 * root.s)
                    width: Math.round(18 * root.s)
                    height: width
                    s: root.s
                    name: Sections.GLYPHS[tab.modelData]
                    size: 17
                    ink: tab.dim ? Lithic.fg3 : tab.marked ? Look.plateMark : Lithic.fg2
                    chosen: tab.marked
                }
                Text {
                    x: Math.round(26 * root.s)
                    y: Math.round(11 * root.s)
                    width: parent.width - x - Math.round(14 * root.s)
                    text: String(root.labels[tab.modelData] || "")
                    color: tab.dim ? Lithic.fg3 : tab.marked ? Lithic.fg : Lithic.fg2
                    font.family: Look.sans
                    font.pixelSize: Math.round(13.5 * root.s)
                    font.weight: Font.Medium
                    elide: Text.ElideRight
                    renderType: Text.NativeRendering
                }
                Text {
                    anchors.right: parent.right
                    y: Math.round(13 * root.s)
                    text: tab.index + 1
                    color: Lithic.fg3
                    font.family: Look.figures
                    font.pixelSize: Math.round(10.5 * Look.figureScale * root.s)
                    font.features: ({ "tnum": 1 })
                    renderType: Text.NativeRendering
                }
                Row {
                    x: Math.round(26 * root.s)
                    y: Math.round(37 * root.s)
                    spacing: Math.round(8 * root.s)

                    Text {
                        id: figureText
                        anchors.verticalCenter: parent.verticalCenter
                        // a wide face or a long translation shrinks a little before it is cut
                        width: tab.level >= 0 ? Math.min(implicitWidth, Math.round(44 * root.s)) : Math.round(96 * root.s)
                        fontSizeMode: tab.level >= 0 ? Text.FixedSize : Text.HorizontalFit
                        minimumPixelSize: Math.round(8 * root.s)
                        visible: text.length > 0
                        text: tab.line
                        color: root.counting ? (tab.found > 0 ? Lithic.fg : Lithic.fg3)
                            : tab.isLit ? root.toneInk(String(tab.news.tone || ""), Lithic.jade)
                            : root.toneInk(String(tab.figure.tone || ""), Lithic.fg2)
                        font.family: Look.figures
                        font.pixelSize: Math.round(11.5 * Look.figureScale * root.s)
                        font.features: ({ "tnum": 1 })
                        elide: Text.ElideRight
                        renderType: Text.NativeRendering
                    }
                    // a level (the phone's charge) is a blade, never a ring
                    Rectangle {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: tab.level >= 0
                        width: Math.round(44 * root.s)
                        height: Math.max(3, Math.round(3 * root.s))
                        radius: Look.lithic || Look.fluent ? 0 : height / 2
                        color: root.track

                        Rectangle {
                            width: Math.round(parent.width * Math.max(0, Math.min(1, tab.level)))
                            height: parent.height
                            radius: parent.radius
                            color: Lithic.jade
                        }
                    }
                }
            }

            // the slot is the page's to the scrim's eye: a press here never closes Kit
            MouseArea {
                id: hover
                anchors.fill: parent
                z: -1
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                acceptedButtons: Qt.AllButtons
                onClicked: mouse => {
                    if (mouse.button === Qt.LeftButton)
                        root.picked(tab.modelData);
                }
            }
        }
    }
}

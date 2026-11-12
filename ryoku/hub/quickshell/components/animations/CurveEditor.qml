import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// a cubic bezier from (0, 0) to (1, 1) with two handles to drag; time runs
// left to right, 0 to 1, and progress may dip below 0 or overshoot past 1
Item {
    id: ed

    property real x0: 0.25
    property real y0: 0.1
    property real x1: 0.25
    property real y1: 1
    signal moved(real x0, real y0, real x1, real y1)

    readonly property real pad: LithicTokens.space.s4
    readonly property real yMin: -1
    readonly property real yMax: 2

    implicitWidth: 300
    implicitHeight: 280

    function px(x) { return ed.pad + x * (ed.width - 2 * ed.pad); }
    function py(y) { return ed.height - ed.pad - (y - ed.yMin) / (ed.yMax - ed.yMin) * (ed.height - 2 * ed.pad); }
    function ux(p) { return Math.max(0, Math.min(1, (p - ed.pad) / (ed.width - 2 * ed.pad))); }
    function uy(p) { return Math.max(ed.yMin, Math.min(ed.yMax, ed.yMin + (ed.height - ed.pad - p) / (ed.height - 2 * ed.pad) * (ed.yMax - ed.yMin))); }

    onX0Changed: plot.requestPaint()
    onY0Changed: plot.requestPaint()
    onX1Changed: plot.requestPaint()
    onY1Changed: plot.requestPaint()
    onWidthChanged: plot.requestPaint()
    onHeightChanged: plot.requestPaint()

    // the palette can change under the plot
    readonly property string inks: String(Lithic.gold) + String(Lithic.vein) + String(Lithic.fg3)
    onInksChanged: plot.requestPaint()

    CutRect {
        anchors.fill: parent
        fill: Lithic.field
        strokeColor: Lithic.edge
        strokeWidth: LithicTokens.size.hairline
        cut: LithicTokens.cut.mid
    }

    Canvas {
        id: plot
        anchors.fill: parent
        onPaint: {
            var ctx = getContext("2d");
            ctx.reset();
            ctx.lineWidth = 1;
            ctx.strokeStyle = String(Lithic.vein);
            ctx.beginPath();
            ctx.moveTo(ed.px(0), ed.py(0));
            ctx.lineTo(ed.px(1), ed.py(0));
            ctx.moveTo(ed.px(0), ed.py(1));
            ctx.lineTo(ed.px(1), ed.py(1));
            ctx.stroke();
            ctx.strokeStyle = String(Lithic.fg3);
            ctx.beginPath();
            ctx.moveTo(ed.px(0), ed.py(0));
            ctx.lineTo(ed.px(ed.x0), ed.py(ed.y0));
            ctx.moveTo(ed.px(1), ed.py(1));
            ctx.lineTo(ed.px(ed.x1), ed.py(ed.y1));
            ctx.stroke();
            ctx.strokeStyle = String(Lithic.gold);
            ctx.lineWidth = 2;
            ctx.beginPath();
            ctx.moveTo(ed.px(0), ed.py(0));
            ctx.bezierCurveTo(ed.px(ed.x0), ed.py(ed.y0), ed.px(ed.x1), ed.py(ed.y1), ed.px(1), ed.py(1));
            ctx.stroke();
        }
    }

    Label {
        x: ed.px(0) + LithicTokens.space.s1
        y: ed.py(0) + LithicTokens.space.s1
        text: I18n.tr("Start")
        typeRole: "caption"
        inkRole: "fg3"
    }
    Label {
        x: ed.px(1) - width - LithicTokens.space.s1
        y: ed.py(1) - height - LithicTokens.space.s1
        text: I18n.tr("Settled")
        typeRole: "caption"
        inkRole: "fg3"
    }

    Handle {
        hx: ed.x0
        hy: ed.y0
        accessibleName: I18n.tr("First handle")
        onDragged: (nx, ny) => ed.moved(nx, ny, ed.x1, ed.y1)
    }
    Handle {
        hx: ed.x1
        hy: ed.y1
        accessibleName: I18n.tr("Second handle")
        onDragged: (nx, ny) => ed.moved(ed.x0, ed.y0, nx, ny)
    }

    // a handle drags with the pointer and steps with the arrow keys
    component Handle: CutRect {
        id: handle
        property real hx: 0
        property real hy: 0
        property string accessibleName: ""
        signal dragged(real nx, real ny)

        readonly property real side: 16
        width: side
        height: side
        x: ed.px(handle.hx) - side / 2
        y: ed.py(handle.hy) - side / 2
        fill: drag.active || handle.activeFocus ? Lithic.fg : Lithic.stone4
        strokeColor: Lithic.fg
        strokeWidth: LithicTokens.size.hairline
        cut: LithicTokens.cut.token
        activeFocusOnTab: true
        Accessible.role: Accessible.Slider
        Accessible.name: handle.accessibleName
        Accessible.description: handle.hx.toFixed(2) + ", " + handle.hy.toFixed(2)

        Keys.onLeftPressed: handle.dragged(Math.max(0, handle.hx - 0.01), handle.hy)
        Keys.onRightPressed: handle.dragged(Math.min(1, handle.hx + 0.01), handle.hy)
        Keys.onUpPressed: handle.dragged(handle.hx, Math.min(ed.yMax, handle.hy + 0.01))
        Keys.onDownPressed: handle.dragged(handle.hx, Math.max(ed.yMin, handle.hy - 0.01))

        FocusRing {
            x: -outset
            y: -outset
            width: parent.width + outset * 2
            height: parent.height + outset * 2
            keyboardFocusVisible: handle.activeFocus
            cut: LithicTokens.cut.token
        }

        DragHandler {
            id: drag
            target: null
            property point start
            onActiveChanged: {
                if (active) {
                    drag.start = Qt.point(ed.px(handle.hx), ed.py(handle.hy));
                    handle.forceActiveFocus();
                }
            }
            onTranslationChanged: {
                if (active)
                    handle.dragged(ed.ux(drag.start.x + translation.x), ed.uy(drag.start.y + translation.y));
            }
        }
        HoverHandler {
            cursorShape: Qt.SizeAllCursor
        }
    }
}

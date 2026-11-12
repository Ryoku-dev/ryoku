import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "model.js" as Model

// the displays drawn to scale; a tile drags to rearrange and a click selects it
CutRect {
    id: canvas

    // the Displays page: draft, tick, selected, mainName, dragging, frozen, drag and drop calls
    required property var page

    implicitHeight: 280
    cut: LithicTokens.cut.small
    fill: Lithic.stone1
    clip: true

    // fit every display that is on into 84% of the canvas, centred
    function computeView() {
        var minX = 1e9, minY = 1e9, maxX = -1e9, maxY = -1e9, any = false;
        var d = canvas.page.draft;
        for (var i = 0; i < d.length; i++) {
            var m = d[i];
            if (m.disabled)
                continue;
            any = true;
            minX = Math.min(minX, m.x);
            minY = Math.min(minY, m.y);
            maxX = Math.max(maxX, m.x + Model.footW(m));
            maxY = Math.max(maxY, m.y + Model.footH(m));
        }
        var b = any ? { x: minX, y: minY, w: Math.max(1, maxX - minX), h: Math.max(1, maxY - minY) }
                    : { x: 0, y: 0, w: 1, h: 1 };
        if (canvas.width <= 0 || canvas.height <= 0)
            return { k: 0.05, ox: 0, oy: 0 };
        var k = Math.min(canvas.width * 0.84 / b.w, canvas.height * 0.84 / b.h);
        if (!isFinite(k) || k <= 0)
            k = 0.05;
        return { k: k, ox: (canvas.width - b.w * k) / 2 - b.x * k, oy: (canvas.height - b.h * k) / 2 - b.y * k };
    }

    // a drag freezes the fit so the canvas does not rescale under the pointer
    readonly property var view: {
        canvas.page.tick;
        return canvas.page.dragging ? canvas.page.frozen : canvas.computeView();
    }

    Repeater {
        model: canvas.page.monCount

        delegate: CutRect {
            id: tile
            required property int index
            // the draft is edited in place, so readers also follow the tick
            readonly property int tick: canvas.page.tick
            readonly property var m: { tile.tick; return canvas.page.draft[tile.index] || null; }
            readonly property bool chosen: canvas.page.selected === tile.index
            readonly property bool on: { tile.tick; return tile.m ? !tile.m.disabled : false; }
            readonly property bool main: { tile.tick; return tile.m ? tile.m.name === canvas.page.mainName && tile.on : false; }
            readonly property color ink: tile.chosen ? Lithic.onSel : (tile.on ? Lithic.fg : Lithic.fg3)

            visible: !!tile.m
            x: tile.m ? tile.m.x * canvas.view.k + canvas.view.ox : 0
            y: tile.m ? tile.m.y * canvas.view.k + canvas.view.oy : 0
            width: tile.m ? Math.max(56, Model.footW(tile.m) * canvas.view.k) : 56
            height: tile.m ? Math.max(40, Model.footH(tile.m) * canvas.view.k) : 40
            z: tile.chosen ? 1 : 0
            fill: tile.chosen ? Lithic.sel
                : !tile.on ? Lithic.stone1
                : hover.hovered ? Lithic.hoverPlane(Lithic.stone4) : Lithic.stone4
            cut: LithicTokens.cut.small
            strokeColor: tile.chosen ? "transparent" : Lithic.edge
            strokeWidth: LithicTokens.size.hairline

            Accessible.role: Accessible.Button
            Accessible.name: tile.m ? tile.m.name : ""

            ChiselColorBehavior on fill {
                duration: hover.hovered ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
            }

            Hatch {
                anchors.fill: parent
                visible: !tile.on
            }

            Column {
                anchors.centerIn: parent
                width: parent.width - LithicTokens.space.s3 * 2
                spacing: LithicTokens.space.s1

                Label {
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    text: tile.m ? tile.m.name : ""
                    typeRole: "mono"
                    ink: tile.ink
                }
                Label {
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    text: {
                        tile.tick;
                        if (!tile.m)
                            return "";
                        if (!tile.on)
                            return I18n.tr("Off");
                        if (tile.m.mirror !== "")
                            return I18n.tr("Copy of %1").replace("%1", tile.m.mirror);
                        return tile.m.width + " \u00d7 " + tile.m.height + ", " + Math.round(tile.m.scale * 100) + "%";
                    }
                    typeRole: "caption"
                    numeric: true
                    ink: tile.chosen ? Lithic.onSel2 : Lithic.fg2
                }
            }

            Label {
                visible: tile.main && tile.height >= 64
                anchors.left: parent.left
                anchors.top: parent.top
                anchors.margins: LithicTokens.space.s2
                text: I18n.tr("Main")
                typeRole: "caption"
                ink: tile.chosen ? Lithic.onSel2 : Lithic.fg2
            }

            HoverHandler {
                id: hover
                cursorShape: Qt.PointingHandCursor
            }

            // the page moves the tile from the reported deltas so its fit and snap stay in charge
            DragHandler {
                id: drag
                target: null
                property real lastX: 0
                property real lastY: 0
                onActiveChanged: {
                    if (drag.active) {
                        drag.lastX = 0;
                        drag.lastY = 0;
                        canvas.page.selectIndex(tile.index);
                    } else {
                        canvas.page.endDrag(tile.index, canvas.computeView().k);
                    }
                }
                onTranslationChanged: {
                    var dx = drag.translation.x - drag.lastX;
                    var dy = drag.translation.y - drag.lastY;
                    drag.lastX = drag.translation.x;
                    drag.lastY = drag.translation.y;
                    canvas.page.dragMonitor(tile.index, dx / canvas.view.k, dy / canvas.view.k, canvas.computeView());
                }
            }

            TapHandler {
                onTapped: canvas.page.selectIndex(tile.index)
            }
        }
    }

    Label {
        visible: canvas.page.monCount === 0
        anchors.centerIn: parent
        width: Math.min(parent.width - LithicTokens.space.s5 * 2, 420)
        horizontalAlignment: Text.AlignHCenter
        text: !canvas.page.listed ? I18n.tr("Looking for displays")
            : canvas.page.listFailed ? I18n.tr("Your displays could not be read.")
            : I18n.tr("No displays found.")
        typeRole: "body"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}

import QtQuick
import "Singletons"

// A page body of cards, laid into balanced columns.
//
// A settings page used to hand-place one column of cards, which on a page-wide
// window leaves a label at one edge and its control at the other. This takes the
// same children, measures them, and lays them into as many columns as the measure
// holds -- splitting them so the columns end level -- while a child marked
// `fullWidth: true` takes a band across the columns and splits the flow.
//
// Children stay where they were declared (this only positions them), so a page
// keeps owning its ids and bindings; the width of one column is published as
// `colWidth` for a delegate that sizes itself to its column.
Item {
    id: root

    property real spacing: Tokens.s4          // between cards down a column
    property real columnSpacing: Tokens.s5    // between columns
    // The Hub opens page-wide, so the body uses the width it is given: three
    // columns once the measure holds them, in step with the schema sheet's own
    // grid, so a hand-built page and a data page read at the same card width.
    property int maxColumns: 3
    property real minColumnWidth: 460
    // The height of the body this grid sits in, so the container reports the
    // full body height even when its cards are short. Content itself always
    // anchors to the top: a page that floats in the middle of a void reads as
    // lost, and a height-dependent centre visibly hops as cards measure in.
    property real fillTo: 0
    // True once no lay is pending and none has run for a few frames: the
    // cards stand where they will stay. A host that shows a page only when
    // its grid has stopped moving (the Hub's page switch) waits on this
    // instead of watching the cards' geometry.
    readonly property bool settled: !settle.running && !relayoutTimer.running && !quiet.running
    default property alias content: stage.data

    // A page with two blocks does not get three columns: the grid takes as many
    // columns as it has blocks (up to what the measure holds) and the cards share
    // the space left over, so a short page fills the window instead of hugging
    // its left edge.
    readonly property int capacity: width >= maxColumns * minColumnWidth + (maxColumns - 1) * columnSpacing
        ? maxColumns : 1
    // Counted without reading a child's width: the column count and the column
    // width both follow from this, and the width comes back to the children, so
    // asking childrenInOrder() (which filters on width) here is a binding loop.
    readonly property int blocks: {
        var n = 0;
        for (var i = 0; i < stage.children.length; i++) {
            var c = stage.children[i];
            if (c && c.visible !== false) n++;
        }
        return n;
    }
    readonly property int columns: Math.max(1, Math.min(capacity, blocks))
    readonly property real colWidth: Math.min(Tokens.cardWide,
        Math.max(200, Math.floor((width - (columns - 1) * columnSpacing) / columns)))

    implicitHeight: layoutHeight
    height: implicitHeight
    property real layoutHeight: 0
    // what the cards actually need, before `fillTo` pads the body out. A page
    // measuring its own spare room wants this number, not the padded one.
    property real contentHeight: 0

    // The coordinate space the children were declared into; they are positioned
    // in place, so nothing has to be reparented to be laid out.
    Item {
        id: stage
        anchors.fill: parent
    }

    function childrenInOrder() {
        var list = [];
        for (var i = 0; i < stage.children.length; i++) {
            var c = stage.children[i];
            if (c && c.visible !== false && c.width !== 0)
                list.push(c);
        }
        return list;
    }

    // Which column each card takes. A run of cards keeps its order and is cut
    // into as many contiguous columns as there are cards to spread, choosing the
    // cuts that make the tallest column the shortest -- the law the two-column
    // page used, solved for any number of columns, so three columns balance like
    // two did and none is left empty while another stacks.
    function splitRun(heights) {
        var n = heights.length;
        var out = [];
        for (var z0 = 0; z0 < n; z0++) out.push(0);
        if (n === 0 || root.columns <= 1)
            return out;

        var units = [];
        for (var i = 0; i < n; i++)
            units.push(Math.max(1, Math.round(heights[i] / 8)));
        var prefix = [0];
        for (var p = 0; p < n; p++)
            prefix.push(prefix[p] + units[p]);
        var load = function (a, b) { return prefix[b] - prefix[a]; };

        // a column with no card in it is not a column: a run of two cards gets two
        var cols = Math.min(root.columns, n);
        var INF = 1e9;
        var best = [], from = [];
        for (var c = 0; c <= cols; c++) {
            var row = [], back = [];
            for (var q = 0; q <= n; q++) { row.push(INF); back.push(0); }
            best.push(row); from.push(back);
        }
        best[0][0] = 0;
        for (var col = 1; col <= cols; col++) {
            for (var end = 1; end <= n; end++) {
                for (var cut = 0; cut < end; cut++) {         // this column holds [cut, end)
                    if (best[col - 1][cut] >= INF) continue;
                    var cand = Math.max(best[col - 1][cut], load(cut, end));
                    if (cand < best[col][end]) {
                        best[col][end] = cand;
                        from[col][end] = cut;
                    }
                }
            }
        }
        var c2 = cols, e = n;
        while (c2 > 0) {
            var s = from[c2][e];
            for (var x = s; x < e; x++)
                out[x] = c2 - 1;
            e = s;
            c2--;
        }
        return out;
    }

    property bool _laying: false
    // the visible-child list and the column split it produced: the split is kept
    // until that list changes, so a card growing does not resend it elsewhere
    property var _kids: []
    property var _pick: null

    function lay() {
        if (root._laying || root.width <= 0)
            return;
        root._laying = true;
        try {
            // qualified: lay() is handed to Qt.callLater unbound, and a bare call
            // inside it resolves against the wrong scope and throws
            root.layoutCards();
        } catch (e) {
            console.warn("CardColumns: layout failed: " + e);
        }
        root._laying = false;
    }

    function layoutCards() {

        var kids = root.childrenInOrder();
        var heights = [];
        for (var m = 0; m < kids.length; m++) {
            var kk = kids[m];
            var full = kk.fullWidth === true;
            var w = full ? root.width : root.colWidth;
            if (Math.abs(kk.width - w) > 0.5)
                kk.width = w;
            heights.push(kk.height);
        }

        // Runs of cards, split by the bands between them. The split is decided
        // once per SET of visible blocks and then kept: a drawer that unfolds
        // changes a card's height, and re-splitting on that would send cards
        // hopping to another column while the reader is looking at them. Only a
        // block appearing, disappearing or changing visibility re-splits.
        var changed = kids.length !== root._kids.length;
        if (!changed) {
            for (var q = 0; q < kids.length; q++)
                if (kids[q] !== root._kids[q]) { changed = true; break; }
        }
        if (changed) {
            root._kids = kids.slice();
            root._pick = null;
        }

        if (!root._pick) {
            var assign = [];
            var i = 0;
            while (i < kids.length) {
                if (kids[i].fullWidth === true) {
                    assign[i] = -1;
                    i++;
                    continue;
                }
                var run = [];
                var start = i;
                while (i < kids.length && kids[i].fullWidth !== true) {
                    run.push(heights[i]);
                    i++;
                }
                var picks = root.splitRun(run);
                for (var r = 0; r < run.length; r++)
                    assign[start + r] = picks[r];
            }
            root._pick = assign;
        }
        var assign2 = root._pick;

        // place: one cursor per column, bands flush both to the same line
        var cursors = [];
        for (var c = 0; c < root.columns; c++) cursors.push(0);
        var lowest = 0;
        for (var j = 0; j < kids.length; j++) {
            var kid = kids[j];
            if (assign2[j] === -1) {
                var floorY = 0;
                for (var f = 0; f < cursors.length; f++) floorY = Math.max(floorY, cursors[f]);
                kid.x = 0;
                kid.y = floorY;
                for (var g = 0; g < cursors.length; g++) cursors[g] = floorY + kid.height + root.spacing;
                lowest = Math.max(lowest, floorY + kid.height);
            } else {
                var col = Math.min(assign2[j] === undefined ? 0 : assign2[j], root.columns - 1);
                kid.x = col * (root.colWidth + root.columnSpacing);
                kid.y = cursors[col];
                cursors[col] += kid.height + root.spacing;
                lowest = Math.max(lowest, cursors[col] - root.spacing);
            }
        }
        // Content anchors to the top on every page. Centring a short page was
        // tried and read as a jump: the block's place depended on its height, so
        // a late-measured card visibly hopped, and two pages with the same first
        // card put it at two different heights. One place, always the top, is
        // what lets a reader build a map of the page.
        var content = kids.length === 0 ? 0 : lowest;
        root.contentHeight = content;
        root.layoutHeight = Math.max(content, Math.max(0, root.fillTo));
        root._laying = false;
        quiet.restart();
    }
    Timer { id: quiet; interval: 48; repeat: false }

    // A timer rather than Qt.callLater: the Hub crossfades pages, so a deferred
    // call can outlive the grid it belongs to and run against a dead object. A
    // timer is owned by the component and dies with it, and still keeps the lay
    // out of the width-change notification that asked for it.
    function relayout() { relayoutTimer.restart() }
    Timer { id: relayoutTimer; interval: 16; repeat: false; onTriggered: root.lay() }
    onWidthChanged: root.relayout()
    onColumnsChanged: root.relayout()
    onFillToChanged: root.relayout()
    Component.onCompleted: {
        // onChildrenChanged fired for these children before the Connections
        // below existed, so the initial set is wired here or a card whose
        // height settles late (a Flow re-wrap after its width lands) would
        // never trigger a re-lay and the next card would overlap it.
        root.wireChildren();
        // The first lay is synchronous so the page's first painted frame is
        // already composed; deferring it shows one frame of unlaid cards and
        // reads as a flicker on every page switch.
        root.lay();
        // Widths are only final once the page's own bindings have settled (a
        // card bound to `colWidth` after us), so one deferred pass catches what
        // the synchronous one could not. After that, changes arrive through the
        // event wiring below, never a poll.
        root.relayout();
    }

    Timer {
        id: settle
        interval: 16
        repeat: false
        onTriggered: root.lay()
    }
    // Throttle, not debounce: a page whose cards fill in from a burst of
    // backend replies (Graphics & Power) changes a card every few ms, and
    // restarting the timer on each change kept postponing the lay, so the
    // cards sat stacked over each other until the burst ended. Starting the
    // timer only when idle lays within one frame of the first change and
    // again for whatever arrived during it.
    function kick() { if (!settle.running) settle.start(); }

    // A card that grows (a group unfolds, a row appears, a Flow re-wraps after
    // its width lands) re-lays the page. A delegate declared under `pragma
    // ComponentBehavior: Bound` refuses a dynamic property, so which children
    // are already wired is tracked here.
    property var _wired: []
    function wireChildren() {
        for (var i = 0; i < stage.children.length; i++) {
            var c = stage.children[i];
            if (!c || root._wired.indexOf(c) !== -1)
                continue;
            root._wired.push(c);
            c.heightChanged.connect(root.kick);
            c.visibleChanged.connect(root.kick);
        }
    }
    Connections {
        target: stage
        function onChildrenChanged() { root.wireChildren(); root.kick(); }
    }
}

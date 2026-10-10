import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "ReloadCoverModel.js" as ReloadCoverModel

// Renders a page from its schema as grouped, compact rows. A setting is a row
// of data; which card it lands in comes from its group and what draws it from
// its kind, so adding one is an edit to the schema and nothing else.
//
// Each group is a SettingCard (a bordered, collapsible sheet); each setting is a
// SettingRow inside it (label + description on the left, control on the right,
// or a band beneath for a control that needs room). This replaced the bento
// grid of value-hero cards, which read as scattered tiles; rows in a card read
// as one instrument sheet.
//
// The draft object holds live values and is the page's own; this only reads it
// and reports edits back. The one file it writes is the weather resolver cache
// (the picked place's coords the weather widgets read), on a location pick.
Item {
    id: sheet

    // Page-supplied blocks that belong ABOVE the rows and must scroll WITH them.
    // Pinning them outside the scroll area steals the viewport: a page with a
    // tall lead block leaves the rows a thin strip to scroll in.
    default property alias lead: leadSlot.data

    property var schema: []          // [{ tab, group, key, label, desc, ctl, src, caps?, opts, lo, hi, unit, pct }]
    property var draft: null         // the page's live values
    property var defaults: ({})      // factory values, for the struck default
    property string tab: ""
    // the tab last shown, so a swap can slide in the direction of travel
    property string prevTab: ""
    function tabIndexIn(t) {
        var g = [];
        for (var i = 0; i < schema.length; i++)
            if (g.indexOf(schema[i].tab) < 0) g.push(schema[i].tab);
        return g.indexOf(t);
    }
    property string query: ""
    // progressive disclosure: rows tagged `adv: true` are the deep knobs, hidden
    // until Advanced is on. search still reaches them (the query branch ignores
    // this), so nothing is ever truly buried.
    property bool advanced: false
    // the row a search jump lands on: switched to, scrolled into view, and
    // flashed. Cleared shortly after so the wash is a pulse, not a highlight.
    property string spotlightKey: ""
    property string reloadCoverError: ""
    property bool reloadCoverImportBusy: false

    signal edited(string key, var value)

    // key -> the live SettingRow, so a search jump can find and scroll to it.
    property var rowItems: ({})

    // A row can gate itself on OTHER settings through `when`: { key: [values] }.
    // Without it the visualiser page shows knobs that do nothing for the chosen
    // look (segment count on plain bars, reflection on an orb), the confusion
    // this removes. Every named key's live draft value must sit in its list; a
    // missing value fails the gate, so a knob for a look you are not on stays
    // hidden — except before the draft has loaded, where the row shows rather
    // than blank the page. Read straight from the draft, since `when` names
    // keys other than this row's own.
    function passes(r) {
        if (!r.when) return true;
        if (!draft || Object.keys(draft).length === 0) return true;
        for (var k in r.when)
            if (r.when[k].indexOf(draft[k]) < 0) return false;
        return true;
    }

    readonly property var rows: {
        var q = query.toLowerCase();
        return schema.filter(function (r) {
            if (!sheet.passes(r)) return false;
            if (r.adv && !sheet.advanced && query === "") return false;
            if (r.tab !== sheet.tab && query === "") return false;
            if (query === "") return true;
            return (r.label + " " + (r.desc || "") + " " + r.key).toLowerCase().indexOf(q) >= 0;
        });
    }
    readonly property var groups: {
        var g = [];
        for (var i = 0; i < rows.length; i++)
            if (g.indexOf(rows[i].group) < 0) g.push(rows[i].group);
        return g;
    }

    // ── card grid ─────────────────────────────────────────────────────────
    // A row wants its control within a glance of its label, so a card never
    // grows past `cardMax`: a wide page gets another column of cards instead of
    // a longer eye-travel, and a narrow one stays a single column. Group order is
    // preserved, filled sequentially, so a reader walks column one top to bottom
    // and then column two rather than zig-zagging across the page.
    // Derived from the PAGE width, never the flick's: the flick is sized from
    // these, and reading them back would be a binding loop that silently pins the
    // sheet to one column.
    // The Hub fills the screen, so the grid uses it: three columns of cards once
    // the page can hold them, which is what keeps a page-wide window from being
    // a narrow ribbon of cards floating in a field of paper.
    readonly property int cardMax: 520
    function fitsColumns(n) { return (width - 28) >= (n * cardMax + (n - 1) * Tokens.s5) }
    readonly property int capacity: fitsColumns(3) ? 3 : (fitsColumns(2) ? 2 : 1)
    // The grid's geometry follows the PAGE WIDTH alone, never how many groups the
    // open tab happens to have. Deriving it from the tab's group count made a
    // one-group tab (BORDERS, MOTION) shrink the sheet to a narrow column in the
    // middle of the window and drag the page's head and tabs sideways with it, so
    // switching tabs read as the whole page shuffling.
    readonly property int gridColumns: capacity
    // how many columns the groups are BUCKETED into: a page with two groups does
    // not get three columns, so the cards take the space instead of hugging the
    // left edge with one column empty beside them
    readonly property int columns: Math.max(1, Math.min(capacity, groups.length))
    readonly property int cardW: Math.min(Tokens.cardWide,
        Math.max(360, Math.floor((width - 28 - (gridColumns - 1) * Tokens.s5) / gridColumns)))
    // The column's own geometry, so a page's head can sit on the same grid.
    readonly property real sheetWidth: col.width
    readonly property real sheetX: flick.x + col.x

    // Split `all` into `count` buckets, shortest column first, so the two sides
    // end up the same height instead of one long column beside one card and a
    // void. A group keeps its order inside its column; several short groups can
    // therefore land in one column past a taller one, which is the price of a
    // balanced page and reads as a grid rather than a queue.
    function bucketGroups(all, count) {
        if (count <= 1 || all.length <= 1) return [all];
        var heights = [];
        for (var g = 0; g < all.length; g++) {
            // every group carries its card's own header (~46px, three compact
            // rows): counting only the rows made a column holding many short
            // cards look light and took more of them, which is how one column
            // ended up far taller than the others
            var n = 3;
            for (var r = 0; r < rows.length; r++)
                if (rows[r].group === all[g]) n += sheet.ctlBlock(rows[r]) ? 6 : 1;
            heights.push(n);
        }
        var buckets = [];
        var loads = [];
        for (var c = 0; c < count; c++) { buckets.push([]); loads.push(0); }
        for (var i = 0; i < all.length; i++) {
            var least = 0;
            for (var k = 1; k < count; k++)
                if (loads[k] < loads[least] - 0.001) least = k;
            buckets[least].push(all[i]);
            loads[least] += heights[i] + 1;
        }
        return buckets;
    }

    function val(r) {
        if (!draft) return "";
        var v = draft[r.key];
        return v === undefined ? "" : v;
    }
    function shown(r) {
        var v = val(r);
        if (r.ctl === "list") return "";
        if (r.ctl === "sw") return v ? "ON" : "OFF";
        if (r.ctl === "slid" && r.pct) return String(Math.round(v * 100));
        if (r.ctl === "multi") return String((v || []).length);
        if (r.ctl === "color") return String(v).toUpperCase();
        return String(v);
    }
    function shownDef(r) {
        if (r.ctl === "reload-cover") return I18n.tr("DEFAULT");
        if (r.ctl === "list") return "";
        var d = defaults[r.key];
        if (d === undefined) return "";
        if (r.ctl === "sw") return d ? "ON" : "OFF";
        if (r.ctl === "slid" && r.pct) return String(Math.round(d * 100));
        if (r.ctl === "multi") return String((d || []).length);
        return String(d);
    }
    function isChanged(r) {
        var v = val(r), d = defaults[r.key];
        if (r.ctl === "reload-cover")
            return JSON.stringify(ReloadCoverModel.normalize(v)) !== JSON.stringify(ReloadCoverModel.normalize(d));
        if (d === undefined) return false;
        if (r.ctl === "multi" || r.ctl === "list") return JSON.stringify(v || []) !== JSON.stringify(d || []);
        return v !== d;
    }
    // A typed number is clamped into the row's own range and stored in the kind
    // the row speaks (a whole number for a stepper, a ratio for a slider), so a
    // typo cannot write a value the control could never produce.
    function commitNumber(r, text) {
        var n = Number(String(text).replace(",", "."));
        if (isNaN(n))
            return;
        if (r.ctl === "step") {
            var lo = (r.from === undefined) ? 0 : Number(r.from);
            var hi = (r.to === undefined) ? 100 : Number(r.to);
            sheet.edited(r.key, Math.max(lo, Math.min(hi, Math.round(n))));
            return;
        }
        if (r.ctl === "slid") {
            var v = r.pct ? n / 100 : n;
            var l = (r.lo === undefined) ? 0 : Number(r.lo);
            var h = (r.hi === undefined) ? (r.pct ? 1 : 100) : Number(r.hi);
            sheet.edited(r.key, Math.max(l, Math.min(h, v)));
        }
    }

    function resetRow(r) {
        if (r.ctl === "reload-cover") {
            sheet.edited(r.key, ReloadCoverModel.empty());
            return;
        }
        var d = defaults[r.key];
        if (d !== undefined) sheet.edited(r.key, d);
    }

    // A row's option list. Most rows write theirs in the schema; a row whose
    // list is really a system table (`set`) reads it from the singleton that
    // owns it, so the table lives in exactly one place. Every consumer of
    // `opts` goes through here.
    function optsFor(r) {
        if (r.set === "languages") return I18n.pickerOptions;
        if (r.set === "locales") return I18n.localeOptions;
        return r.opts || [];
    }
    function labelsFor(r) {
        if (r.set === "languages") return I18n.pickerLabels;
        return ({});
    }

    // ── list control: a record-array editor ─────────────────────────────────
    // The record shape comes from the row's `fields` spec; a row without one
    // falls back to the window-rule shape (class, title, action, value). Records
    // are JSON objects keyed by field name, and every edit hands back a fresh
    // array so the Repeater rebuilds the card owning a focused field cleanly.
    function listFields(r) {
        if (r.fields && r.fields.length) return r.fields;
        return [
            { "name": "class", "label": I18n.tr("Class"), "ctl": "text", "eg": "firefox" },
            { "name": "title", "label": I18n.tr("Title"), "ctl": "text" },
            { "name": "action", "label": I18n.tr("Action"), "ctl": "chips", "opts": r.opts || [] },
            { "name": "value", "label": I18n.tr("Value"), "ctl": "text" }
        ];
    }
    function listArr(r) { var a = sheet.val(r); return Array.isArray(a) ? a : []; }
    // a fresh record: an explicit field `def` wins, else a kind-appropriate
    // empty (a switch off, the first option, the low end of a range, blank text).
    function listNewRecord(fields) {
        var rec = {};
        for (var i = 0; i < fields.length; i++) {
            var f = fields[i];
            if (f.def !== undefined) rec[f.name] = f.def;
            else if (f.ctl === "sw") rec[f.name] = false;
            else if (f.ctl === "seg" || f.ctl === "chips") rec[f.name] = (f.opts && f.opts.length) ? f.opts[0] : "";
            else if (f.ctl === "step" || f.ctl === "slid") rec[f.name] = (f.lo !== undefined) ? Number(f.lo) : 0;
            else rec[f.name] = "";
        }
        return rec;
    }
    function listPatch(r, i, name, v) {
        var a = sheet.listArr(r).slice();
        a[i] = Object.assign({}, a[i]);
        a[i][name] = v;
        sheet.edited(r.key, a);
    }
    function listAdd(r) {
        var a = sheet.listArr(r).slice();
        a.push(sheet.listNewRecord(sheet.listFields(r)));
        sheet.edited(r.key, a);
    }
    function listRemove(r, i) {
        var a = sheet.listArr(r).slice();
        a.splice(i, 1);
        sheet.edited(r.key, a);
    }
    function listClear(r) { sheet.edited(r.key, []); }

    // inline vs band, and how wide, decided once from the control kind. A
    // control that needs room (chips, a segmented bar of 3+, a demo)
    // gets a band whose height is its own; a picker or field gets a fixed foot
    // band; everything else sits inline at the row's right.
    function ctlBlock(r) {
        var c = r.ctl, n = sheet.optsFor(r).length;
        if (c === "chips" || c === "multi" || c === "layoutdemo" || c === "reload-cover" || c === "list") return true;
        if (c === "seg" && n >= 3) return true;
        return false;
    }
    function ctlFoot(r) {
        var c = r.ctl;
        if (c === "pick" || c === "text" || c === "color" || c === "location" || c === "action" || c === "app" || c === "timezone") return 32;
        return 0;
    }
    function ctlWidth(r, w) {
        var c = r.ctl, n = sheet.optsFor(r).length;
        if (c === "sw") return 54;
        if (c === "step") return 58;
        if (c === "slid") return Math.min(240, Math.max(160, Math.round(w * 0.34)));
        if (c === "seg") return Math.max(140, 74 * Math.max(2, n));
        return 54;
    }
    // the compact readout: a number for a stepper or slider; nothing for a
    // toggle (the switch is the state) or a control that shows its own value.
    function rowValue(r) {
        if (r.ctl === "step" || r.ctl === "slid") return sheet.shown(r);
        return "";
    }
    function rowUnit(r) {
        if (r.ctl === "step" || r.ctl === "slid") return r.pct ? "%" : (r.unit || "");
        return "";
    }

    // search jump: switch to the row's tab, then scroll it to centre and flash.
    function focusKey(key) {
        var r = null;
        for (var i = 0; i < schema.length; i++) if (schema[i].key === key) { r = schema[i]; break; }
        if (!r) return;
        if (r.tab && r.tab !== sheet.tab) sheet.tab = r.tab;
        sheet.spotlightKey = "";
        scrollPending.key = key;
        scrollPending.tries = 0;
        scrollTimer.restart();
    }
    QtObject { id: scrollPending; property string key: ""; property int tries: 0 }
    Timer {
        id: scrollTimer
        interval: 40
        repeat: false
        onTriggered: {
            var it = sheet.rowItems[scrollPending.key];
            if (!it && scrollPending.tries < 12) { scrollPending.tries++; scrollTimer.restart(); return; }
            if (!it) return;
            var y = it.mapToItem(col, 0, 0).y;
            var target = Math.max(0, Math.min(y - flick.height / 2 + it.height / 2, Math.max(0, col.height - flick.height)));
            scrollAnim.to = target;
            scrollAnim.restart();
            sheet.spotlightKey = scrollPending.key;
            clearSpot.restart();
        }
    }
    NumberAnimation { id: scrollAnim; target: flick; property: "contentY"; duration: Tokens.move; easing.type: Tokens.ease }
    Timer { id: clearSpot; interval: 1600; onTriggered: sheet.spotlightKey = "" }

    Flickable {
        id: flick
        // The sheet IS the column: the flick is sized to the cards it holds and
        // centred in the page, so the scroll rail hangs off the column's edge
        // instead of the window's, and the margins either side stay equal.
        width: Math.min(parent.width - 14, sheet.gridColumns * sheet.cardW + (sheet.gridColumns - 1) * Tokens.s5 + 14)
        x: Math.round((parent.width - width) / 2)
        // The card columns span the column they were given, so a two-column sheet
        // fills the measure and a one-column one is not left-aligned in it.
        property int columnSpan: sheet.gridColumns
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        contentHeight: Math.max(col.height + Tokens.s5, height)
        clip: true
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        // The sheet uses the page it was given. A settings row wants its control
        // within a short glance of its label, so a wide window gets TWO bounded
        // columns of cards rather than one column with a label at the left edge
        // and its control a screen away. `sheet.columns` decides from the width
        // available; groups keep their order down column one, then column two, so
        // reading stays top-to-bottom inside a column instead of zig-zagging.
        Column {
            id: col
            width: flick.width - 14
            spacing: Tokens.s5
            // Content anchors to the top on every page: a centred block's place
            // depends on its height, so centring hopped visibly as cards measured
            // in, and the same card sat at two heights on two pages. One place,
            // always the top, is what lets a reader build a map of the page.

            // A Column, not an Item measured by childrenRect: it sizes itself
            // from its visible children, so a page that hides its block on a
            // tab costs no height and no spacing gap, and one that shows it
            // needs no height of its own.
            Column {
                id: leadSlot
                width: col.width
            }

            Row {
                id: cardColumns
                width: col.width
                spacing: Tokens.s5
                readonly property var buckets: sheet.bucketGroups(sheet.groups, sheet.columns)

                // A tab change swaps the whole card set, which otherwise snaps.
                // The motion is a settle slide in the direction of travel (the
                // same one the Hub uses between pages), never a fade: fading
                // the swap put the words through a half-transparent phase,
                // which read as a flash. The columns clip to the sheet, so the
                // slide never paints over the rail.
                Connections {
                    target: sheet
                    function onTabChanged() {
                        var dir = sheet.tabIndexIn(sheet.tab) >= sheet.tabIndexIn(sheet.prevTab) ? 1 : -1;
                        tabSlide.from = dir * Tokens.s6;
                        tabSlide.to = 0;
                        tabSlide.restart();
                        sheet.prevTab = sheet.tab;
                    }
                }
                NumberAnimation {
                    id: tabSlide
                    target: cardColumns; property: "x"
                    duration: Tokens.move; easing.type: Tokens.ease
                }

                Repeater {
                    model: cardColumns.buckets

                    Column {
                        id: cardColumn
                        required property var modelData
                        width: sheet.cardW
                        spacing: Tokens.s5

                        Repeater {
                            model: cardColumn.modelData

                                    SettingCard {
                                        id: card
                                        required property string modelData
                                        width: cardColumn.width
                                        title: I18n.tr(modelData === "" ? "OTHER" : modelData)
                                        summary: card.groupRows.length + " " + I18n.tr("SETTINGS")

                            readonly property var groupRows: sheet.rows.filter(function (r) { return r.group === card.modelData })

                                        Repeater {
                                            model: card.groupRows
                                            SettingRow {
                                    id: srow
                                    required property var modelData
                                    required property int index
                                    readonly property var r: modelData

                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    divider: index > 0

                                    label: I18n.tr(r.label)
                                    desc: I18n.tr(r.desc || "")
                                    eg: I18n.tr(r.eg || "")
                                    // a number can be typed where one can be nudged
                                    editableValue: r.ctl === "step" || r.ctl === "slid"
                                    onValueCommitted: (text) => sheet.commitNumber(r, text)
                                    value: sheet.rowValue(r)
                                    unit: sheet.rowUnit(r)
                                    def: sheet.shownDef(r)
                                    changed: sheet.isChanged(r)
                                    source: r.src ? (r.src.slice(-5) === ".json" ? r.src : r.src + ".json") : ""
                                    block: sheet.ctlBlock(r)
                                    footH: sheet.ctlBlock(r) ? 0 : sheet.ctlFoot(r)
                                    controlWidth: sheet.ctlWidth(r, card.width)
                                    spotlight: sheet.spotlightKey !== "" && sheet.spotlightKey === r.key
                                    onResetRequested: sheet.resetRow(r)

                                    Component.onCompleted: { var m = sheet.rowItems; m[r.key] = srow; sheet.rowItems = m; }
                                    Component.onDestruction: { if (sheet.rowItems[r.key] === srow) delete sheet.rowItems[r.key]; }

                                    Loader {
                                        anchors.fill: parent
                                        sourceComponent: {
                                            switch (srow.r.ctl) {
                                            case "sw": return swC;
                                            case "step": return stepC;
                                            case "slid": return slidC;
                                            case "seg": return segC;
                                            case "chips": return chipsC;
                                            case "multi": return multiC;
                                            case "list": return listC;
                                            case "pick": return pickC;
                                            case "reload-cover": return reloadCoverC;
                                            case "app": return appC;
                                            case "location": return locationC;
                                            case "color": return colorC;
                                            case "action": return actionC;
                                            case "layoutdemo": return layoutDemoC;
                                            case "timezone": return timezoneC;
                                            default: return textC;
                                            }
                                        }
                                    }

                                    Component {
                                        id: actionC
                                        Btn {
                                            anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                                            text: I18n.tr(srow.r.actionLabel || "Generate")
                                            onAct: {
                                                if (srow.r.key === "i18nGenerate")
                                                    Spawn.run(["kitty", "--class", "ryoku-i18n", "-e", "sh", "-c",
                                                        "ryoku-i18n llm " + I18n.lang + "; echo; read -n1 -rsp 'Done. Press any key to close…'; echo"]);
                                            }
                                        }
                                    }
                                    Component {
                                        id: timezoneC
                                        Row {
                                            anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                                            spacing: Tokens.s3
                                            Text {
                                                anchors.verticalCenter: parent.verticalCenter
                                                text: (srow.r.tzCurrent && ("" + srow.r.tzCurrent).length) ? ("" + srow.r.tzCurrent).replace(/_/g, " ") : "\u2014"
                                                color: Tokens.ink
                                                font.family: Tokens.mono
                                                font.pixelSize: Tokens.fSmall
                                            }
                                            Btn {
                                                anchors.verticalCenter: parent.verticalCenter
                                                text: I18n.tr("CHANGE ON MAP")
                                                onAct: sheet.timezonePick(srow.r)
                                            }
                                        }
                                    }
                                    Component {
                                        id: layoutDemoC
                                        Item {
                                            id: demo
                                            anchors.fill: parent
                                            implicitHeight: 168
                                            readonly property string layout: {
                                                var v = sheet.draft ? sheet.draft["appearance.layout"] : "";
                                                return (v === "master" || v === "scrolling") ? v : "dwindle";
                                            }
                                            readonly property var blurbs: ({
                                                "dwindle": "Each new window splits the focused frame in two, so the layout spirals into smaller and smaller frames.",
                                                "master": "One big master frame keeps the focus; every other window stacks down the side beside it.",
                                                "scrolling": "Windows line up in one endless horizontal row; the strip pans sideways to keep the focused column in view."
                                            })
                                            Rectangle {
                                                id: demoScreen
                                                anchors { left: parent.left; top: parent.top; bottom: parent.bottom }
                                                width: Math.min(360, demo.width * 0.5)
                                                color: "transparent"
                                                radius: Tokens.radius
                                                border.width: Tokens.border
                                                border.color: Tokens.line
                                                AnimatedImage {
                                                    anchors.fill: parent
                                                    anchors.margins: Tokens.s3
                                                    source: Qt.resolvedUrl("art/tiling-" + demo.layout + ".gif")
                                                    fillMode: Image.PreserveAspectFit
                                                    playing: true
                                                    cache: false
                                                    asynchronous: true
                                                    onStatusChanged: if (status === Image.Ready) playing = true
                                                }
                                            }
                                            Column {
                                                anchors { left: demoScreen.right; leftMargin: Tokens.s5; right: parent.right; verticalCenter: demoScreen.verticalCenter }
                                                spacing: Tokens.s2
                                                Text {
                                                    text: demo.layout.toUpperCase()
                                                    color: Tokens.ink
                                                    font.family: Tokens.ui
                                                    font.pixelSize: Tokens.fValue
                                                    font.weight: Font.Light
                                                }
                                                Text {
                                                    width: parent.width
                                                    text: demo.blurbs[demo.layout]
                                                    color: Tokens.inkMuted
                                                    font.family: Tokens.ui
                                                    font.pixelSize: 12
                                                    wrapMode: Text.WordWrap
                                                }
                                            }
                                        }
                                    }
                                    Component {
                                        id: swC
                                        Sw {
                                            anchors.right: parent.right
                                            anchors.verticalCenter: parent.verticalCenter
                                            on: !!sheet.val(srow.r)
                                            onToggled: (v) => sheet.edited(srow.r.key, v)
                                        }
                                    }
                                    Component {
                                        id: stepC
                                        Step {
                                            anchors.right: parent.right
                                            anchors.verticalCenter: parent.verticalCenter
                                            value: Number(sheet.val(srow.r)) || 0
                                            from: Number(srow.r.lo) || 0
                                            to: Number(srow.r.hi) || 100
                                            onModified: (v) => sheet.edited(srow.r.key, v)
                                        }
                                    }
                                    Component {
                                        id: slidC
                                        Slid {
                                            anchors.fill: parent
                                            value: Number(sheet.val(srow.r)) || 0
                                            from: Number(srow.r.lo) || 0
                                            to: Number(srow.r.hi) || 1
                                            onModified: (v) => sheet.edited(srow.r.key, v)
                                        }
                                    }
                                    Component {
                                        id: segC
                                        Seg {
                                            anchors.left: parent.left
                                            anchors.right: parent.right
                                            anchors.verticalCenter: parent.verticalCenter
                                            options: sheet.optsFor(srow.r)
                                            labels: srow.r.optLabels || ({})
                                            current: String(sheet.val(srow.r))
                                            onChose: (k) => sheet.edited(srow.r.key, k)
                                        }
                                    }
                                    Component {
                                        id: chipsC
                                        Chips {
                                            anchors.fill: parent
                                            options: sheet.optsFor(srow.r)
                                            // optLabels lets a row offer readable chips over
                                            // literal stored values, so a user picks "Thirds"
                                            // rather than typing proportions.
                                            labels: srow.r.optLabels || ({})
                                            current: String(sheet.val(srow.r))
                                            onChose: (k) => sheet.edited(srow.r.key, k)
                                        }
                                    }
                                    Component {
                                        id: multiC
                                        Multi {
                                            anchors.fill: parent
                                            options: sheet.optsFor(srow.r)
                                            chosen: sheet.val(srow.r) || []
                                            onToggled: (k) => {
                                                var l = (sheet.val(srow.r) || []).slice();
                                                var i = l.indexOf(k);
                                                if (i >= 0) l.splice(i, 1); else l.push(k);
                                                sheet.edited(srow.r.key, l);
                                            }
                                        }
                                    }
                                    Component {
                                        id: listC
                                        // one card per record; each field draws itself from the
                                        // row's `fields` spec through the sheet's own primitives.
                                        Column {
                                            id: lst
                                            anchors { left: parent.left; right: parent.right; top: parent.top }
                                            spacing: Tokens.s2
                                            readonly property var r: srow.r
                                            readonly property var fields: sheet.listFields(lst.r)
                                            readonly property var records: sheet.listArr(lst.r)

                                            // header: entry count, clear all, add.
                                            Item {
                                                width: parent.width
                                                height: Tokens.ctlH
                                                Text {
                                                    anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                                                    text: lst.records.length === 1 ? I18n.tr("%1 ENTRY").arg(1) : I18n.tr("%1 ENTRIES").arg(lst.records.length)
                                                    color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                                                }
                                                Row {
                                                    anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                                                    spacing: Tokens.s3
                                                    Btn {
                                                        anchors.verticalCenter: parent.verticalCenter
                                                        text: I18n.tr("CLEAR ALL")
                                                        armed: lst.records.length > 0
                                                        onAct: sheet.listClear(lst.r)
                                                    }
                                                    IconBtn {
                                                        anchors.verticalCenter: parent.verticalCenter
                                                        glyph: "+"
                                                        onAct: sheet.listAdd(lst.r)
                                                    }
                                                }
                                            }

                                            Repeater {
                                                model: lst.records
                                                delegate: Rectangle {
                                                    id: card
                                                    required property int index
                                                    required property var modelData
                                                    width: lst.width
                                                    height: fieldsCol.implicitHeight + Tokens.s3 * 2
                                                    radius: Tokens.radius
                                                    color: "transparent"
                                                    border.width: Tokens.border
                                                    border.color: Tokens.line

                                                    IconBtn {
                                                        anchors { right: parent.right; top: parent.top; topMargin: Tokens.s3; rightMargin: Tokens.s3 }
                                                        z: 2
                                                        glyph: "\u2212"
                                                        onAct: sheet.listRemove(lst.r, card.index)
                                                    }

                                                    Column {
                                                        id: fieldsCol
                                                        anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s3 }
                                                        anchors.rightMargin: Tokens.s3 + 26
                                                        spacing: Tokens.s2

                                                        Repeater {
                                                            model: lst.fields
                                                            delegate: Item {
                                                                id: fld
                                                                required property var modelData
                                                                readonly property var f: fld.modelData
                                                                readonly property string fname: fld.f.name
                                                                readonly property var fval: card.modelData[fld.fname]
                                                                readonly property bool inlineCtl: fld.f.ctl === "sw" || fld.f.ctl === "step" || fld.f.ctl === "slid"
                                                                readonly property real fbandH: (fld.f.ctl === "seg" || fld.f.ctl === "chips") ? Tokens.ctlH : 30
                                                                width: fieldsCol.width
                                                                height: fld.inlineCtl ? Tokens.ctlH : (flabel.height + Tokens.s1 + fld.fbandH)

                                                                Text {
                                                                    id: flabel
                                                                    anchors { left: parent.left; top: parent.top }
                                                                    anchors.right: fld.inlineCtl ? inlineSlot.left : parent.right
                                                                    anchors.rightMargin: fld.inlineCtl ? Tokens.s3 : 0
                                                                    anchors.verticalCenter: fld.inlineCtl ? parent.verticalCenter : undefined
                                                                    text: I18n.tr(fld.f.label || fld.fname)
                                                                    color: Tokens.inkMuted
                                                                    font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                                                    elide: Text.ElideRight
                                                                }

                                                                // inline control: switch, stepper, slider.
                                                                Item {
                                                                    id: inlineSlot
                                                                    visible: fld.inlineCtl
                                                                    anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                                                                    width: fld.f.ctl === "slid" ? Math.min(200, Math.max(140, Math.round(fieldsCol.width * 0.4))) : (fld.f.ctl === "step" ? 58 : 54)
                                                                    height: Tokens.ctlH
                                                                    Sw {
                                                                        visible: fld.f.ctl === "sw"
                                                                        anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                                                                        on: !!fld.fval
                                                                        onToggled: (v) => sheet.listPatch(lst.r, card.index, fld.fname, v)
                                                                    }
                                                                    Step {
                                                                        visible: fld.f.ctl === "step"
                                                                        anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                                                                        value: Number(fld.fval) || 0
                                                                        from: fld.f.lo !== undefined ? Number(fld.f.lo) : 0
                                                                        to: fld.f.hi !== undefined ? Number(fld.f.hi) : 100
                                                                        onModified: (v) => sheet.listPatch(lst.r, card.index, fld.fname, v)
                                                                    }
                                                                    Slid {
                                                                        visible: fld.f.ctl === "slid"
                                                                        anchors.fill: parent
                                                                        value: Number(fld.fval) || 0
                                                                        from: fld.f.lo !== undefined ? Number(fld.f.lo) : 0
                                                                        to: fld.f.hi !== undefined ? Number(fld.f.hi) : 1
                                                                        onModified: (v) => sheet.listPatch(lst.r, card.index, fld.fname, v)
                                                                    }
                                                                }

                                                                // banded control: segmented, chips, colour, text.
                                                                Item {
                                                                    visible: !fld.inlineCtl
                                                                    anchors { left: parent.left; right: parent.right; top: flabel.bottom; topMargin: Tokens.s1 }
                                                                    height: fld.fbandH
                                                                    Seg {
                                                                        visible: fld.f.ctl === "seg"
                                                                        anchors.fill: parent
                                                                        options: fld.f.opts || []
                                                                        current: String(fld.fval === undefined ? "" : fld.fval)
                                                                        onChose: (k) => sheet.listPatch(lst.r, card.index, fld.fname, k)
                                                                    }
                                                                    Chips {
                                                                        visible: fld.f.ctl === "chips"
                                                                        anchors.fill: parent
                                                                        options: fld.f.opts || []
                                                                        labels: fld.f.optLabels || ({})
                                                                        current: String(fld.fval === undefined ? "" : fld.fval)
                                                                        onChose: (k) => sheet.listPatch(lst.r, card.index, fld.fname, k)
                                                                    }
                                                                    ColorField {
                                                                        visible: fld.f.ctl === "color"
                                                                        anchors.fill: parent
                                                                        value: String(fld.fval === undefined ? "" : fld.fval)
                                                                        onChosen: (v) => sheet.listPatch(lst.r, card.index, fld.fname, v)
                                                                    }
                                                                    Field {
                                                                        visible: fld.f.ctl !== "seg" && fld.f.ctl !== "chips" && fld.f.ctl !== "color"
                                                                        anchors.fill: parent
                                                                        tabular: true
                                                                        placeholder: fld.f.eg || ""
                                                                        text: fld.fval === undefined ? "" : String(fld.fval)
                                                                        onCommitted: (v) => { if (v !== (fld.fval === undefined ? "" : String(fld.fval))) sheet.listPatch(lst.r, card.index, fld.fname, v); }
                                                                    }
                                                                }
                                                            }
                                                        }
                                                    }
                                                }
                                            }
                                        }
                                    }
                                    Component {
                                        id: reloadCoverC
                                        ReloadCoverControl {
                                            anchors.fill: parent
                                            descriptor: ReloadCoverModel.normalize(sheet.val(srow.r))
                                            errorText: sheet.reloadCoverError
                                            busy: sheet.reloadCoverImportBusy
                                            onAddRequested: sheet.reloadCoverPick(srow.r)
                                            onDefaultRequested: sheet.reloadCoverDefault(srow.r)
                                            onEnabledToggled: (enabled) => {
                                                var descriptor = Object.assign({}, ReloadCoverModel.normalize(sheet.val(srow.r)));
                                                descriptor.enabled = enabled;
                                                sheet.edited(srow.r.key, descriptor);
                                            }
                                        }
                                    }
                                    Component {
                                        id: pickC
                                        PickBar {
                                            anchors.fill: parent
                                            value: String(sheet.val(srow.r))
                                            count: sheet.optsFor(srow.r).length
                                            labels: sheet.labelsFor(srow.r)
                                            onOpened: sheet.openPick(srow.r)
                                        }
                                    }
                                    Component {
                                        id: textC
                                        Rectangle {
                                            anchors.fill: parent
                                            color: "transparent"
                                            radius: Tokens.radius
                                            border.width: ti.activeFocus ? 2 : Tokens.border
                                            border.color: ti.activeFocus ? Tokens.ink : Tokens.line
                                            TextInput {
                                                id: ti
                                                anchors.fill: parent
                                                anchors.leftMargin: 8
                                                anchors.rightMargin: 8
                                                verticalAlignment: Text.AlignVCenter
                                                clip: true
                                                autoScroll: activeFocus
                                                color: Tokens.ink
                                                font.family: Tokens.ui
                                                font.pixelSize: 12
                                                selectByMouse: true
                                                text: String(sheet.val(srow.r))
                                                onEditingFinished: sheet.edited(srow.r.key, text)
                                                onTextEdited: sheet.edited(srow.r.key, text)
                                            }
                                        }
                                    }
                                    Component {
                                        id: colorC
                                        ColorField {
                                            anchors.fill: parent
                                            value: String(sheet.val(srow.r))
                                            onChosen: (v) => sheet.edited(srow.r.key, v)
                                        }
                                    }
                                    Component {
                                        id: appC
                                        Item {
                                            anchors.fill: parent
                                            Btn {
                                                id: defBtn
                                                anchors.right: parent.right
                                                anchors.verticalCenter: parent.verticalCenter
                                                visible: String(sheet.val(srow.r)) !== ""
                                                text: I18n.tr("DEFAULT")
                                                onAct: sheet.edited(srow.r.key, "")
                                            }
                                            Btn {
                                                id: chooseBtn
                                                anchors.right: defBtn.visible ? defBtn.left : parent.right
                                                anchors.rightMargin: defBtn.visible ? Tokens.s2 : 0
                                                anchors.verticalCenter: parent.verticalCenter
                                                text: I18n.tr("CHOOSE…")
                                                onAct: sheet.appPick(srow.r)
                                            }
                                            Text {
                                                anchors.left: parent.left
                                                anchors.right: chooseBtn.left
                                                anchors.rightMargin: Tokens.s2
                                                anchors.verticalCenter: parent.verticalCenter
                                                elide: Text.ElideRight
                                                readonly property string cmd: String(sheet.val(srow.r))
                                                text: cmd.length ? cmd : I18n.tr("ryotunes (YouTube Music)")
                                                color: cmd.length ? Tokens.ink : Tokens.inkMuted
                                                font.family: Tokens.ui
                                                font.pixelSize: 12
                                            }
                                        }
                                    }
                                    Component {
                                        id: locationC
                                        Item {
                                            id: locRoot
                                            anchors.fill: parent
                                            readonly property string stateDir: (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku"

                                            Rectangle {
                                                id: locField
                                                anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter }
                                                height: 30
                                                color: "transparent"
                                                radius: Tokens.radius
                                                border.width: lti.activeFocus ? 2 : Tokens.border
                                                border.color: lti.activeFocus ? Tokens.ink : Tokens.line
                                                TextInput {
                                                    id: lti
                                                    anchors.fill: parent
                                                    anchors.leftMargin: 8
                                                    anchors.rightMargin: 8
                                                    verticalAlignment: Text.AlignVCenter
                                                    clip: true
                                                    autoScroll: activeFocus
                                                    color: Tokens.ink
                                                    font.family: Tokens.ui
                                                    font.pixelSize: 12
                                                    selectByMouse: true
                                                    text: String(sheet.val(srow.r))
                                                    onTextEdited: debounce.restart()
                                                    onEditingFinished: sheet.edited(srow.r.key, text)
                                                }
                                                Text {
                                                    anchors.verticalCenter: parent.verticalCenter
                                                    anchors.left: parent.left
                                                    anchors.leftMargin: 8
                                                    visible: lti.text === "" && !lti.activeFocus
                                                    text: I18n.tr("Empty locates by IP")
                                                    color: Tokens.inkFaint
                                                    font.family: Tokens.ui
                                                    font.pixelSize: 12
                                                }
                                            }

                                            Timer {
                                                id: debounce
                                                interval: 300
                                                onTriggered: {
                                                    var q = lti.text.trim();
                                                    if (q.length < 2) { locPop.close(); return; }
                                                    geo.command = ["curl", "-s", "--max-time", "6",
                                                        "https://geocoding-api.open-meteo.com/v1/search?count=6&language=en&format=json&name=" + encodeURIComponent(q)];
                                                    geo.running = false;
                                                    geo.running = true;
                                                }
                                            }

                                            Process {
                                                id: geo
                                                stdout: StdioCollector {
                                                    onStreamFinished: {
                                                        var out = [];
                                                        try {
                                                            var j = JSON.parse(this.text);
                                                            if (j && Array.isArray(j.results)) {
                                                                for (var i = 0; i < j.results.length; i++) {
                                                                    var rr = j.results[i];
                                                                    if (typeof rr.latitude === "number" && typeof rr.longitude === "number")
                                                                        out.push({ name: rr.name || "", admin1: rr.admin1 || "", country: rr.country || "", lat: rr.latitude, lon: rr.longitude });
                                                                }
                                                            }
                                                        } catch (e) {}
                                                        locList.model = out;
                                                        if (out.length > 0 && lti.activeFocus) locPop.open(); else locPop.close();
                                                    }
                                                }
                                            }

                                            FileView {
                                                id: locCache
                                                path: locRoot.stateDir + "/weather-loc.json"
                                                blockLoading: true
                                                printErrors: false
                                            }

                                            Popup {
                                                id: locPop
                                                parent: locField
                                                y: -locPop.height - 2
                                                width: locField.width
                                                padding: 1
                                                focus: false
                                                closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside
                                                implicitHeight: Math.min(locList.count, 6) * 28 + 2
                                                background: Rectangle {
                                                    color: Tokens.paperLift
                                                    radius: Tokens.radius
                                                    border.width: Tokens.border
                                                    border.color: Tokens.lineStrong
                                                }
                                                contentItem: ListView {
                                                    id: locList
                                                    clip: true
                                                    model: []
                                                    delegate: Rectangle {
                                                        id: lrow
                                                        required property var modelData
                                                        width: ListView.view.width
                                                        height: 28
                                                        color: lhov.hovered ? Tokens.tint10 : "transparent"
                                                        Text {
                                                            anchors.verticalCenter: parent.verticalCenter
                                                            anchors.left: parent.left
                                                            anchors.right: parent.right
                                                            anchors.leftMargin: 8
                                                            anchors.rightMargin: 8
                                                            elide: Text.ElideRight
                                                            text: lrow.modelData.name + (lrow.modelData.admin1 ? "  ·  " + lrow.modelData.admin1 : "") + (lrow.modelData.country ? "  ·  " + lrow.modelData.country : "")
                                                            color: Tokens.ink
                                                            font.family: Tokens.ui
                                                            font.pixelSize: 12
                                                        }
                                                        HoverHandler { id: lhov; cursorShape: Qt.PointingHandCursor }
                                                        TapHandler {
                                                            onTapped: {
                                                                lti.text = lrow.modelData.name;
                                                                sheet.edited(srow.r.key, lrow.modelData.name);
                                                                locCache.setText(JSON.stringify({ query: lrow.modelData.name, city: lrow.modelData.name, lat: lrow.modelData.lat, lon: lrow.modelData.lon }));
                                                                locPop.close();
                                                            }
                                                        }
                                                    }
                                                }
                                            }
                                        }
                                    }
                                }
                            }
                        }
                     }
                 }
             }
            }
        }
    }

    signal pickRequested(var row)
    function openPick(r) { pickRequested(r) }


    signal appPickRequested(var row)
    function appPick(r) { appPickRequested(r) }

    signal timezonePickRequested(var row)
    function timezonePick(r) { timezonePickRequested(r) }

    signal reloadCoverPickRequested(var row)
    function reloadCoverPick(r) { reloadCoverPickRequested(r) }

    signal reloadCoverDefaultRequested(var row)
    function reloadCoverDefault(r) { reloadCoverDefaultRequested(r) }

    Column {
        anchors.centerIn: parent
        // Only a search can come up empty. A tab that carries no settings rows
        // at all is showing its page's own blocks, and "nothing matches" would
        // be answering a question nobody asked.
        visible: sheet.rows.length === 0 && sheet.query !== ""
        spacing: Tokens.s2
        Text {
            text: I18n.tr("NO MATCH")
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fRow
            font.letterSpacing: 2
            anchors.horizontalCenter: parent.horizontalCenter
        }
        Text {
            text: I18n.tr("nothing here matches “%1”").arg(sheet.query)
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            anchors.horizontalCenter: parent.horizontalCenter
        }
    }
}

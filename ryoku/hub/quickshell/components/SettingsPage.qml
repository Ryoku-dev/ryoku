import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../registry/registry.js" as Registry

// one settings page drawn from its schema plus the provider rows mapped to it
Item {
    id: page

    required property var hub
    property string pageId: ""
    property var schema: null
    property string stateLine: ""
    // any row of this page hidden because the window manager lacks it
    property bool someHidden: false
    // section id -> Component drawn above (body) or below (footer) its slab
    property var bodies: ({})
    property var footers: ({})
    // row id -> Component drawn as the control of a row with control: "custom"
    property var controls: ({})
    // section id -> Component drawn inside the section's More options, after its rows
    property var moreBodies: ({})
    // section id -> its More options is open; a rebuild recreates sections, this keeps them as they were
    property var openFolds: ({})
    // section id -> Component drawn inside the section's slab, above its rows
    property var slabBodies: ({})
    // rows -> rows, applied once the provider rows join the schema's
    property var adaptRows: null
    // a live illustration of the page's settings, shown beside the page's head
    property Component headerPreview: null
    // the page's scroll view, for bespoke content that measures itself in it
    readonly property Flickable flickable: flick

    // [{ id, title, missing, rows, more, hidden }] for the sections on screen
    property var layout: []
    property string layoutSignature: ""
    property string highlightId: ""
    property int currentSection: 0
    property bool indexHeld: false
    // after a landing the resting pointer shows no hover until it moves, so
    // the row it happens to rest on never reads as the found one
    property bool pointerQuiet: false
    property point quietAt: Qt.point(-1, -1)
    // row id -> delegate, kept in place so lookups never rebind
    property var rowItems: ({})
    property var sectionItems: ({})

    // the page's sections by name, for the outline the sidebar hangs under the open page
    readonly property var outline: page.layout.map(s => I18n.tr(s.title))
    // how far the page is read, 0..1, or -1 while it fits, for the masthead's ruler
    readonly property real readProgress: flick.contentHeight > flick.height + 1
        ? flick.contentY / (flick.contentHeight - flick.height) : -1
    // every row the page draws now, across its sections and their More options
    function shownRows() {
        var out = [];
        page.layout.forEach(s => { out = out.concat(s.rows, s.more); });
        return out;
    }

    // how many drawn rows stand away from their defaults; the header offers Reset page then
    readonly property int changedCount: {
        page.layoutSignature;
        Changes.revision;
        return page.shownRows().filter(r => Changes.isChanged(r)).length;
    }
    readonly property bool hasChanges: page.changedCount > 0

    function resetPage() {
        Changes.resetPage(page.shownRows(), page.pageId, page.schema ? page.schema.title : "");
    }

    // sections made by the page's first build rise in; a later rebuild's simply stand
    property bool revealing: true
    Connections {
        target: sectionModel
        function onCountChanged() {
            if (sectionModel.count > 0)
                endReveal.start();
        }
    }
    Timer {
        id: endReveal
        interval: 0
        onTriggered: page.revealing = false
    }

    readonly property bool ready: Changes.desktopReady && Settings.loaded && ProviderSchema.ready
    // the scroll rail sits at the right edge, so the column never runs under it; past its
    // widest the column stands centred in the pane
    readonly property real usableWidth: Math.max(0, page.width - LithicTokens.size.scrollThumb - LithicTokens.space.s3)
    readonly property real columnWidth: Math.min(page.usableWidth, 960)
    readonly property real columnX: Math.max(0, Math.min((page.width - page.columnWidth) / 2, page.usableWidth - page.columnWidth))

    function whenMet(row) {
        if (!row.when)
            return true;
        var v = Changes.value(row.when.bind);
        for (var i = 0; i < row.when.is.length; i++)
            if (JSON.stringify(row.when.is[i]) === JSON.stringify(v))
                return true;
        return false;
    }

    // rows with template: true only feed search; the page draws them itself,
    // and a search landing on one scrolls to its section (opening More options
    // when the row says more: true)
    function allRows() {
        if (!page.schema)
            return [];
        var rows = page.schema.rows.filter(r => r.template !== true).concat(page.hub.providerRowsFor(page.pageId));
        return page.adaptRows ? page.adaptRows(rows) : rows;
    }

    function rebuild() {
        if (!page.schema || !page.ready)
            return;
        var rows = page.allRows();
        var out = [], sig = [], anyHidden = false;
        for (var s = 0; s < page.schema.sections.length; s++) {
            var sec = page.schema.sections[s];
            var mine = rows.filter(r => r.section === sec.id);
            var avail = mine.filter(r => page.hub.rowAvailable(r));
            var shown = avail.filter(r => page.whenMet(r));
            var base = shown.filter(r => r.more !== true);
            var more = shown.filter(r => r.more === true);
            var gone = avail.length === 0 && mine.length > 0;
            if (avail.length < mine.length)
                anyHidden = true;
            // a section that lost every row says why, when it knows
            var bespoke = !!(page.bodies[sec.id] || page.footers[sec.id] || page.moreBodies[sec.id] || page.slabBodies[sec.id]);
            if (shown.length === 0 && !bespoke && !(gone && sec.missing))
                continue;
            out.push({ id: sec.id, title: sec.title, missing: gone ? (sec.missing || "") : "", rows: base, more: more,
                       moreBody: !!page.moreBodies[sec.id] });
            // a row whose words or options change under the same id is redrawn too
            var rowSig = r => r.id + "=" + r.label + "=" + (r.detail || "") + "=" + page.hub.unhonoredReason(r) + "=" + JSON.stringify(r.options || []) + "=" + (r.unavailable || "")
                + "=" + (r.control || "") + "=" + JSON.stringify(r.bind || null);
            sig.push(sec.id + ":" + base.map(rowSig).join(",") + "|" + more.map(rowSig).join(",") + "|" + gone + "|" + !!page.moreBodies[sec.id]);
        }
        page.someHidden = anyHidden;
        var signature = sig.join(";");
        if (signature !== page.layoutSignature) {
            page.layoutSignature = signature;
            var data = {};
            out.forEach(s => data[s.id] = s);
            page.sectionData = data;
            page.syncSections(out.map(s => s.id));
            page.layout = out;
        }
    }

    // section id -> its layout entry, read by the kept delegates
    property var sectionData: ({})
    ListModel { id: sectionModel }

    // bring the delegate list to these ids in this order, keeping the ones that stay
    function syncSections(ids) {
        for (var i = sectionModel.count - 1; i >= 0; i--)
            if (ids.indexOf(sectionModel.get(i).sid) < 0)
                sectionModel.remove(i);
        for (var j = 0; j < ids.length; j++) {
            var at = -1;
            for (var k = j; k < sectionModel.count; k++) {
                if (sectionModel.get(k).sid === ids[j]) {
                    at = k;
                    break;
                }
            }
            if (at < 0)
                sectionModel.insert(j, { sid: ids[j] });
            else if (at !== j)
                sectionModel.move(at, j, 1);
        }
    }

    onReadyChanged: page.rebuild()
    onSchemaChanged: page.rebuild()
    onBodiesChanged: page.rebuild()
    onFootersChanged: page.rebuild()
    onMoreBodiesChanged: page.rebuild()
    onSlabBodiesChanged: page.rebuild()
    onAdaptRowsChanged: page.rebuild()
    Component.onCompleted: {
        page.rebuild();
    }
    // a page the Hub keeps loaded while hidden catches up when it is shown again
    property bool stale: false
    function refresh() {
        if (page.visible)
            page.rebuild();
        else
            page.stale = true;
    }
    onVisibleChanged: {
        if (page.visible && page.stale) {
            page.stale = false;
            page.rebuild();
        }
    }
    Connections {
        target: Changes
        function onRevisionChanged() { page.refresh(); }
    }
    Connections {
        target: Settings
        function onRevisionChanged() { page.refresh(); }
        function onOverlayRevisionChanged() { page.refresh(); }
    }
    Connections {
        target: ProviderSchema
        function onRevisionChanged() { page.refresh(); }
    }

    function sectionOfRow(rowId) {
        for (var s = 0; s < page.layout.length; s++) {
            var sec = page.layout[s];
            if (sec.rows.some(r => r.id === rowId))
                return { section: sec, more: false };
            if (sec.more.some(r => r.id === rowId))
                return { section: sec, more: true };
        }
        return null;
    }

    // a row hidden by its condition leads to the row it depends on
    function landingRow(rowId) {
        if (page.sectionOfRow(rowId))
            return rowId;
        var rows = page.allRows();
        var target = rows.find(r => r.id === rowId);
        if (target && target.when) {
            var key = Changes.keyOf(target.when.bind);
            var parentRow = rows.find(r => r.bind && Changes.keyOf(r.bind) === key);
            if (parentRow && page.sectionOfRow(parentRow.id))
                return parentRow.id;
        }
        return "";
    }

    // a fresh page's column is placed a frame after it is built; landings wait for it
    Timer {
        id: placed
        property bool done: false
        // scrolls asked for before then, run once the column is placed
        property var queued: []
        interval: 60
        running: true
        onTriggered: {
            placed.done = true;
            var run = placed.queued;
            placed.queued = [];
            run.forEach(fn => fn());
            if (pendingFocus.rowId !== "" && page.ready && page.layout.length > 0)
                pendingFocus.restart();
        }
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (!page.ready || page.layout.length === 0 || !placed.done) {
            pendingFocus.rowId = rowId;
            return;
        }
        var id = page.landingRow(rowId);
        if (id === "") {
            // a row the page draws once its data arrives (the bar style's own options) is
            // waited for a moment, then the landing falls back to a template or section
            var known = page.allRows().some(r => r.id === rowId) || page.layout.some(s => s.id === rowId)
                || (!!page.schema && page.schema.rows.some(r => r.id === rowId && r.template === true));
            if (page.awaitRow !== rowId)
                page.awaitUntil = Date.now() + 3000;
            if (!known && Date.now() < page.awaitUntil) {
                page.awaitRow = rowId;
                return;
            }
            page.awaitRow = "";
            page.landOnTemplate(rowId);
            return;
        }
        page.awaitRow = "";
        var where = page.sectionOfRow(id);
        var secItem = page.sectionItems[where.section.id];
        if (where.more && secItem && !secItem.moreOpen) {
            secItem.openMore();
            landTimer.rowId = id;
            landTimer.restart();
            return;
        }
        page.land(id);
    }

    function landOnTemplate(rowId) {
        var row = page.schema ? page.schema.rows.find(r => r.id === rowId && r.template === true) : null;
        if (!row) {
            // a deep link may name a section rather than a row
            var sectionAt = page.layout.findIndex(s => s.id === rowId);
            if (sectionAt >= 0)
                page.chooseSection(sectionAt);
            return;
        }
        var at = page.layout.findIndex(s => s.id === row.section);
        if (at < 0)
            return;
        var secItem = page.sectionItems[row.section];
        if (row.more === true && page.layout[at].moreBody && secItem) {
            if (!secItem.moreOpen) {
                secItem.openMore();
                moreLand.section = row.section;
                moreLand.restart();
            } else {
                page.landInMore(row.section);
            }
            return;
        }
        page.chooseSection(at);
    }

    // the bespoke content inside a section's More options, a third from the top, with keyboard
    // focus on its first control so typing never goes to the control focused before
    function landInMore(sectionId) {
        var secItem = page.sectionItems[sectionId];
        if (!secItem || !secItem.moreBodyItem)
            return;
        var body = secItem.moreBodyItem;
        page.reveal(body);
        var next = body.nextItemInFocusChain(true);
        if (next && page.contains(body, next))
            next.forceActiveFocus(Qt.TabFocusReason);
    }

    // scroll any item inside the page to a third from the top
    function reveal(item) {
        if (!item)
            return;
        if (!placed.done) {
            placed.queued = placed.queued.concat([() => page.reveal(item)]);
            return;
        }
        page.indexHeld = false;
        page.scrollTo(item.mapToItem(flick.contentItem, 0, 0).y - flick.height / 3);
    }

    function land(id) {
        var item = page.rowItems[id];
        if (!placed.done) {
            placed.queued = placed.queued.concat([() => page.land(id)]);
            return;
        }
        if (!item)
            return;
        var y = item.mapToItem(flick.contentItem, 0, 0).y;
        // the index marks the landed row's section, whether or not the page has to scroll
        var at = page.layout.findIndex(s => page.sectionItems[s.id] && page.contains(page.sectionItems[s.id], item));
        page.indexHeld = at >= 0;
        page.heldY = -1;
        if (at >= 0)
            page.currentSection = at;
        page.highlightId = "";
        scrollAnim.stop();
        scrollAnim.landing = id;
        page.pointerQuiet = true;
        page.quietAt = pointerWatch.point.scenePosition;
        page.scrollTo(y - flick.height / 3);
    }

    // whether the keys were in use, read from the mark the focused control shows
    property bool keysInUse: false
    function handBackReason() {
        return page.keysInUse ? Qt.TabFocusReason : Qt.OtherFocusReason;
    }

    // keyboard focus that lands on a control out of view brings it into view, once it settles:
    // a redraw can drop focus on a tall container for a moment before it comes back
    readonly property Item focusedItem: page.Window.activeFocusItem
    onFocusedItemChanged: {
        // a hidden control handed focus back (a dialog's opener its action hid) says nothing
        // about the keys, and gives the keys on to what is left of its section
        var hidden = page.hiddenHere(page.focusedItem);
        if (page.focusedItem && !hidden && page.focusedItem.visualFocus !== undefined)
            page.keysInUse = page.focusedItem.visualFocus === true;
        var row = hidden ? "" : page.rowOf(page.focusedItem);
        if (row) {
            page.lastFocusRow = row;
            page.lastFocusSection = page.layout.map(s => s.id).find(id => page.sectionItems[id] && page.contains(page.sectionItems[id], page.focusedItem)) || "";
        }
        else if (page.focusLost(page.focusedItem) && (page.lastFocusRow !== "" || page.lastFocusSection !== ""))
            refocus.restart();
        // a control the page draws outside any row now has the keys: an older row is no
        // longer where focus should come back to, its section is
        else if (page.focusedItem && page.focusedItem.activeFocusOnTab && page.contains(flick.contentItem, page.focusedItem)) {
            page.lastFocusRow = "";
            page.lastFocusSection = page.layout.map(s => s.id).find(id => page.sectionItems[id] && page.contains(page.sectionItems[id], page.focusedItem)) || "";
        }
        if (!settleFocus.running)
            page.focusSeen = [];
        if (page.focusedItem)
            page.focusSeen.push(page.focusedItem);
        settleFocus.restart();
    }
    // focus that stays on a control while it hides (a form its own Return closed) moves on too
    Connections {
        target: page.focusedItem
        ignoreUnknownSignals: true
        function onVisibleChanged() {
            if (page.hiddenHere(page.focusedItem) && (page.lastFocusRow !== "" || page.lastFocusSection !== ""))
                refocus.restart();
        }
    }
    // the item that held focus before this change, and every item focus passed through since
    property Item focusBefore: null
    property var focusSeen: []
    function contains(outer, inner) {
        for (var p = inner; p; p = p.parent)
            if (p === outer)
                return true;
        return false;
    }
    Timer {
        id: settleFocus
        interval: 0
        onTriggered: {
            var settled = page.Window.activeFocusItem;
            var before = page.focusBefore;
            page.focusSeen = page.focusSeen.filter(x => !!x);
            // focus that fell from a control (now disabled while it works) to something holding
            // it goes back to that control's row, and the page stays where it is
            // (a row that is a Tab stop itself was reached on purpose, Shift+Tab from its control)
            if (before && settled && settled !== before && !settled.activeFocusOnTab
                    && page.contains(settled, before) && page.contains(flick.contentItem, settled)) {
                page.focusSeen = [];
                var row = page.rowOf(before);
                if (row !== "" && page.rowItems[row])
                    page.focusRowItem(page.rowItems[row], page.handBackReason());
                // nothing in the row can take the keys now (its control works): the next Tab
                // stop does (a Cancel the work shows), while the page stays where it is
                if (page.Window.activeFocusItem === settled) {
                    var next = settled.nextItemInFocusChain(true);
                    if (next && next !== settled) {
                        page.focusQuietly = true;
                        next.forceActiveFocus(page.handBackReason());
                    }
                }
                return;
            }
            // a container that took focus on its way to a control inside it (an opened form)
            // is what comes into view; one that merely holds the control focus left is not
            var target = settled;
            for (var i = 0; i < page.focusSeen.length; i++) {
                var seen = page.focusSeen[i];
                if (seen !== settled && page.contains(seen, settled) && !(before && page.contains(seen, before))) {
                    target = seen;
                    break;
                }
            }
            page.focusSeen = [];
            page.focusBefore = settled;
            // a control that turned off (view only now) makes Qt pass focus down the Tab chain,
            // and a landing may hand focus on past its row; either way the page stays where it is
            var quiet = page.focusQuietly;
            page.focusQuietly = false;
            if (quiet || (before && before !== settled && (!before.enabled || !before.visible)))
                return;
            page.keepInView(target);
        }
    }
    property bool focusQuietly: false

    // the row that last held keyboard focus: when it is redrawn (its words changed) or a
    // dialog it opened closes, focus falls back to a container holding the page (the
    // window, the page's loader), and goes back to that row instead
    property string lastFocusRow: ""
    function rowOf(item) {
        for (var p = item; p; p = p.parent) {
            if (p.modelData && p.modelData.id !== undefined && page.rowItems[p.modelData.id] === p)
                return p.modelData.id;
            // rows a page draws itself register under their own ids
            for (var id in page.rowItems)
                if (page.rowItems[id] === p)
                    return id;
        }
        return "";
    }
    function hiddenHere(item) {
        return !!item && !item.visible && page.contains(flick.contentItem, item);
    }
    function focusLost(item) {
        if (!item)
            return false;
        if (page.hiddenHere(item))
            return true;
        for (var p = page.parent; p; p = p.parent)
            if (p === item)
                return true;
        return item === page.Window.contentItem;
    }
    Timer {
        id: refocus
        interval: 0
        // the page may still be rebuilding the row's section (a store re-read), so try a few times
        property int tries: 0
        onTriggered: {
            if (!page.visible || !page.focusLost(page.Window.activeFocusItem)) {
                refocus.tries = 0;
                return;
            }
            // the row's own action removed it (a remove button): the keys go on in its section
            if (page.rowItems[page.lastFocusRow])
                page.focusRowItem(page.rowItems[page.lastFocusRow], page.handBackReason());
            else if (page.lastFocusSection !== "")
                page.focusInSection(page.lastFocusSection, page.handBackReason());
            if (page.focusLost(page.Window.activeFocusItem) && refocus.tries < 10) {
                refocus.tries++;
                refocus.interval = 100;
                refocus.restart();
            } else {
                refocus.tries = 0;
                refocus.interval = 0;
            }
        }
    }
    property string lastFocusSection: ""

    function keepInView(item) {
        if (!item || scrollAnim.running)
            return;
        var p = item;
        while (p && p !== flick.contentItem)
            p = p.parent;
        if (!p)
            return;
        var y = item.mapToItem(flick.contentItem, 0, 0).y;
        var margin = LithicTokens.space.s5;
        // an open notice covers the foot of the page, so a control stops above it
        var foot = margin + (page.hub && page.hub.noticeInset ? page.hub.noticeInset : 0);
        // something taller than the view (an opened form) shows from its top
        if (y < flick.contentY + margin || item.height > flick.height - margin - foot)
            page.scrollTo(y - margin);
        else if (y + item.height > flick.contentY + flick.height - foot)
            page.scrollTo(y + item.height - flick.height + foot);
    }

    function scrollTo(y) {
        var target = Math.max(0, Math.min(y, flick.contentHeight - flick.height));
        scrollAnim.stop();
        scrollAnim.from = flick.contentY;
        scrollAnim.to = target;
        scrollAnim.start();
    }

    function chooseSection(index) {
        var sec = page.layout[index];
        var item = sec ? page.sectionItems[sec.id] : null;
        if (!item)
            return;
        page.currentSection = index;
        page.indexHeld = true;
        page.heldY = -1;
        page.scrollTo(item.mapToItem(flick.contentItem, 0, 0).y);
        scrollAnim.focusSection = sec.id;
    }

    // keyboard focus on a row: the row itself if it acts, else its control (schema rows),
    // else the first control inside it (page-drawn rows). reason defaults to the keyboard's, so
    // a landing shows its mark; focus handed back after pointer use passes the pointer's
    function focusRowItem(landed, reason) {
        if (!landed)
            return;
        var why = reason === undefined ? Qt.TabFocusReason : reason;
        if (landed.settingRow.focusPolicy !== Qt.NoFocus)
            landed.settingRow.forceActiveFocus(why);
        else if (landed.rowControl)
            landed.rowControl.focusControl(why);
        else {
            // the first control inside the row; a row with none usable (plain text, view only,
            // busy) hands focus to the next Tab stop, so Tab carries on from there, while the
            // page stays on the row
            var next = landed.settingRow.nextItemInFocusChain(true);
            var inside = !!next && next !== landed.settingRow && page.contains(landed, next);
            if (inside)
                next.forceActiveFocus(why);
            else if (next && next !== landed.settingRow && !landed.settingRow.activeFocusOnTab) {
                page.focusQuietly = true;
                next.forceActiveFocus(why);
            } else
                landed.settingRow.forceActiveFocus(why);
        }
    }

    // after the outline moves the page: the first thing in the section that takes the keys, in
    // reading order (a body drawn above the rows comes first), else its first row
    function focusInSection(sid, reason) {
        var why = reason === undefined ? Qt.TabFocusReason : reason;
        var sec = page.sectionItems[sid];
        var next = sec ? sec.nextItemInFocusChain(true) : null;
        if (next && page.contains(sec, next)) {
            next.forceActiveFocus(why);
            return;
        }
        var data = page.sectionData[sid];
        var first = data && data.rows.length > 0 ? page.rowItems[data.rows[0].id] : null;
        // a section with nothing to focus (a plain list): the keys rest on the page, so no
        // control left off screen acts on the next key and Tab goes on into the page
        if (first)
            page.focusRowItem(first, why);
        else
            page.forceActiveFocus(Qt.OtherFocusReason);
    }

    // the index follows the reading position; after a jump it holds the chosen section
    // until the page moves away from where the jump left it (wheel, keys, a landing)
    property real heldY: -1
    function followReading() {
        if (page.layout.length === 0)
            return;
        if (page.indexHeld) {
            if (scrollAnim.running || page.heldY < 0 || Math.abs(flick.contentY - page.heldY) < 1)
                return;
            page.indexHeld = false;
        }
        if (flick.contentY >= flick.contentHeight - flick.height - 1) {
            page.currentSection = page.layout.length - 1;
            return;
        }
        var current = 0;
        for (var i = 0; i < page.layout.length; i++) {
            var item = page.sectionItems[page.layout[i].id];
            if (!item)
                continue;
            var top = item.mapToItem(flick.contentItem, 0, 0).y;
            var visibleAbove = top + item.height - flick.contentY;
            if (top <= flick.contentY + flick.height / 3 && !(visibleAbove < item.height / 3 && i + 1 < page.layout.length))
                current = i;
        }
        page.currentSection = current;
    }

    // a landing asked for before the sections existed waits for the column to place them
    Timer {
        id: pendingFocus
        property string rowId: ""
        interval: 60
        onTriggered: {
            var id = pendingFocus.rowId;
            pendingFocus.rowId = "";
            page.focusRow(id);
        }
    }
    onLayoutChanged: {
        if (pendingFocus.rowId === "" && page.awaitRow !== "")
            pendingFocus.rowId = page.awaitRow;
        if (pendingFocus.rowId !== "")
            pendingFocus.restart();
    }
    property string awaitRow: ""
    property real awaitUntil: 0

    Timer {
        id: landTimer
        property string rowId: ""
        interval: LithicTokens.motion.unfold + 20
        onTriggered: page.land(landTimer.rowId)
    }

    Timer {
        id: moreLand
        property string section: ""
        interval: LithicTokens.motion.unfold + 20
        onTriggered: page.landInMore(moreLand.section)
    }

    Timer {
        id: highlightClear
        interval: Lithic.reduceMotion ? LithicTokens.motion.placardLife : LithicTokens.motion.highlight + 100
        onTriggered: page.highlightId = ""
    }

    NumberAnimation {
        id: scrollAnim
        // the row to spotlight once the scroll stops, or the section whose first control takes focus
        property string landing: ""
        property string focusSection: ""
        target: flick
        property: "contentY"
        duration: Lithic.duration(LithicTokens.motion.navigate)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Lithic.curve
        onStopped: {
            if (scrollAnim.focusSection !== "") {
                var sid = scrollAnim.focusSection;
                scrollAnim.focusSection = "";
                page.heldY = flick.contentY;
                page.focusInSection(sid);
                return;
            }
            // any other scroll (a control brought into view) lets the index follow again
            if (scrollAnim.landing === "") {
                page.followReading();
                return;
            }
            page.highlightId = scrollAnim.landing;
            page.heldY = flick.contentY;
            page.focusRowItem(page.rowItems[scrollAnim.landing]);
            scrollAnim.landing = "";
            highlightClear.restart();
        }
    }

    Flickable {
        id: flick
        anchors.fill: parent
        contentWidth: width
        contentHeight: body.y + body.height + LithicTokens.size.settingsInsetVertical
        boundsBehavior: Flickable.StopAtBounds
        clip: true
        onContentYChanged: page.followReading()
        onMovementStarted: page.indexHeld = false
        // content that shrinks (a form closing) never leaves the page scrolled past its end
        function clampEnd() {
            var end = Math.max(0, flick.contentHeight - flick.height);
            if (!scrollAnim.running && !flick.moving && flick.contentY > end)
                flick.contentY = end;
        }
        onContentHeightChanged: flick.clampEnd()
        onHeightChanged: flick.clampEnd()

        Column {
            id: body
            x: page.columnX
            width: page.columnWidth
            spacing: 0

            PageHeader {
                width: parent.width
                hub: page.hub
                pageId: page.pageId
                preview: page.headerPreview
                title: page.schema ? I18n.tr(page.schema.title) : ""
                lead: page.schema && page.schema.lead ? I18n.tr(page.schema.lead) : ""
                stateLine: page.stateLine
                note: page.someHidden ? I18n.tr("Some options are hidden because this window manager does not support them.") : ""
                facts: {
                    page.layoutSignature;
                    var two = n => (n < 10 ? "0" : "") + n;
                    var rows = page.shownRows().length;
                    return [
                        { label: I18n.tr("Sections"), value: two(page.layout.length) },
                        rows > 0 ? { label: I18n.tr("Settings"), value: two(rows) } : null,
                        { label: I18n.tr("Changed"), value: two(page.changedCount), lit: page.changedCount > 0 }
                    ].filter(f => f !== null);
                }

                // a page whose rows stand away from their defaults offers to put them back
                Plaque {
                    visible: page.hasChanges
                    kind: "quiet"
                    small: true
                    glyph: "restart_alt"
                    text: I18n.tr("Reset page")
                    accessibleName: I18n.tr("Reset this page to its defaults")
                    onClicked: page.resetPage()
                }
            }

            // one delegate per section id, kept while the section stays, so a rebuild
            // does not recreate open folds (they would unfold again and the page would jump)
            Repeater {
                model: sectionModel
                delegate: Section {
                    id: sec
                    required property string sid
                    required property int index
                    readonly property var modelData: page.sectionData[sec.sid]
                        || ({ id: sec.sid, title: "", missing: "", rows: [], more: [], moreBody: false })
                    readonly property bool moreOpen: moreFold.open
                    readonly property Item moreBodyItem: moreBodyLoader.item
                    readonly property bool hasMore: sec.modelData.more.length > 0 || sec.modelData.moreBody
                    function openMore() {
                        moreFold.open = true;
                    }
                    width: body.width
                    title: I18n.tr(modelData.title)
                    // numbered and ruled, as a sheet numbers its parts
                    ruled: true
                    mark: (sec.index < 9 ? "0" : "") + (sec.index + 1) + " / " + (sectionModel.count < 10 ? "0" : "") + sectionModel.count
                    Component.onCompleted: {
                        page.sectionItems[sec.sid] = sec;
                        // the first sections rise in turn as the page opens; later ones stand
                        if (page.revealing && sec.index < 5)
                            rise.play();
                    }
                    Reveal {
                        id: rise
                        item: sec
                        order: sec.index + 1
                    }
                    Component.onDestruction: {
                        if (page.sectionItems[sec.sid] === sec)
                            delete page.sectionItems[sec.sid];
                    }

                    Column {
                        width: parent.width
                        spacing: LithicTokens.space.s2

                        Loader {
                            width: parent.width
                            active: !!page.bodies[sec.modelData.id]
                            visible: active
                            sourceComponent: page.bodies[sec.modelData.id] || null
                        }

                        Slab {
                            width: parent.width
                            visible: sec.modelData.rows.length > 0 || sec.hasMore || !!page.slabBodies[sec.modelData.id]
                            fill: Tone.card

                            Column {
                                width: parent.width

                                Loader {
                                    width: parent.width
                                    active: !!page.slabBodies[sec.modelData.id]
                                    visible: active
                                    sourceComponent: page.slabBodies[sec.modelData.id] || null
                                }

                                Repeater {
                                    model: sec.modelData.rows
                                    delegate: RowItem { host: page; afterBody: !!page.slabBodies[sec.modelData.id] }
                                }

                                Item {
                                    visible: sec.hasMore
                                    width: 1
                                    height: sec.modelData.rows.length > 0 || !!page.slabBodies[sec.modelData.id] ? LithicTokens.size.settingsRowGap : 0
                                }

                                Disclosure {
                                    id: moreFold
                                    visible: sec.hasMore
                                    width: parent.width
                                    text: I18n.tr("More options")
                                    // set while the fold is built, so a recreated fold opens without its unfold animation
                                    open: page.openFolds[sec.modelData.id] === true
                                    onOpenChanged: {
                                        page.openFolds[sec.modelData.id] = moreFold.open;
                                        if (moreFold.open)
                                            moreFold.built = true;
                                    }
                                    // the rows inside are made the first time the fold opens, then kept
                                    property bool built: page.openFolds[sec.modelData.id] === true

                                    Repeater {
                                        model: moreFold.built ? sec.modelData.more : []
                                        delegate: RowItem { host: page }
                                    }
                                    Loader {
                                        id: moreBodyLoader
                                        width: parent ? parent.width : 0
                                        active: sec.modelData.moreBody
                                        visible: active
                                        sourceComponent: page.moreBodies[sec.modelData.id] || null
                                    }
                                }
                            }
                        }

                        Loader {
                            width: parent.width
                            active: !!page.footers[sec.modelData.id]
                            visible: active
                            sourceComponent: page.footers[sec.modelData.id] || null
                        }

                        Label {
                            width: parent.width
                            visible: sec.modelData.missing.length > 0
                            text: I18n.tr(sec.modelData.missing)
                            typeRole: "detail"
                            inkRole: "fg3"
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }
                    }
                }
            }

            Section {
                visible: related.count > 0
                width: body.width
                title: I18n.tr("Related")

                Flow {
                    width: parent.width
                    spacing: LithicTokens.space.s2

                    Repeater {
                        id: related
                        model: page.schema && page.schema.related ? page.schema.related.filter(id => page.hub.pageAvailable(id)) : []
                        delegate: Plaque {
                            required property var modelData
                            kind: "tonal"
                            small: true
                            glyph: "arrow_forward"
                            text: I18n.tr(Registry.page(modelData).title)
                            onClicked: page.hub.navigate(modelData, "")
                        }
                    }
                }
            }
        }
    }

    // scrolling under a resting pointer keeps its position; only a real move counts
    HoverHandler {
        id: pointerWatch
        onPointChanged: {
            if (!page.pointerQuiet)
                return;
            var p = pointerWatch.point.scenePosition;
            if (Math.abs(p.x - page.quietAt.x) + Math.abs(p.y - page.quietAt.y) > 2)
                page.pointerQuiet = false;
        }
    }

    ScrollRail {
        flickable: flick
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.right: parent.right
    }

    component RowItem: Column {
        id: item
        required property var modelData
        required property int index
        required property Item host
        // a bespoke body sits above the first row, so that row keeps its gap and divider
        property bool afterBody: false
        readonly property alias settingRow: settingRow
        readonly property alias rowControl: rowControl
        readonly property string error: Changes.error(item.modelData.bind)
        width: parent ? parent.width : 0
        spacing: LithicTokens.space.s1

        Component.onCompleted: item.host.rowItems[item.modelData.id] = item
        Component.onDestruction: {
            if (item.host.rowItems[item.modelData.id] === item)
                delete item.host.rowItems[item.modelData.id];
        }

        SettingRow {
            id: settingRow
            hoverEnabled: !item.host.pointerQuiet
            unavailable: item.modelData.unavailable ? I18n.tr(item.modelData.unavailable) : ""
            width: parent.width
            title: I18n.tr(item.modelData.label)
            // what the window manager says it cannot keep replaces the row's own detail
            detail: {
                var cannot = item.host.hub.unhonoredReason(item.modelData);
                return cannot ? I18n.tr(cannot) : item.modelData.detail ? I18n.tr(item.modelData.detail) : "";
            }
            changed: Changes.isChanged(item.modelData)
            first: item.index === 0 && !item.afterBody
            highlighted: item.host.highlightId === item.modelData.id
            activatable: ["toggle", "handoff", "action", "link"].indexOf(item.modelData.control) >= 0 || item.modelData.activatable === true
            onActivated: rowControl.activate()
            onResetRequested: Changes.set(item.modelData.bind, Changes.defaultOf(item.modelData), item.modelData.changeLabel || item.modelData.label, item.host.pageId)

            RowControl {
                id: rowControl
                row: item.modelData
                pageId: item.host.pageId
                overlayParent: item.host.hub.overlayItem
                hub: item.host.hub
                custom: item.host.controls[item.modelData.id] || null
            }
        }

        Row {
            visible: item.error.length > 0
            width: parent.width
            spacing: LithicTokens.space.s3

            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, parent.width - retry.width - LithicTokens.space.s3)
                text: I18n.tr(item.error)
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: retry
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: Changes.retry(item.modelData.bind)
            }
        }
    }
}

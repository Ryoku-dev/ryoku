import QtQuick
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/displays"
import "../components/displays/model.js" as Model
import "../components/displays/arrange.js" as Arrange
import "../schema/displays.js" as Schema
import "../registry/registry.js" as Registry

// modes and arrangement apply together and are kept or reverted; the rest changes at once
Item {
    id: page

    required property var hub
    readonly property string pageId: "displays"

    // the draft is edited in place; tick makes readers rebind and monCount keeps tiles alive
    property var draft: []
    property var liveRows: []
    property int monCount: 0
    property int tick: 0
    property int selected: 0
    property string selectedName: ""
    property string committed: "[]"
    property string mainName: ""
    property bool listed: false
    property bool listFailed: false
    property bool dragging: false
    property var frozen: ({ k: 1, ox: 0, oy: 0 })

    property var profiles: []
    property var presets: []
    property bool applying: false
    property string applyError: ""
    property string layoutsError: ""

    property string highlightId: ""
    property int currentSection: 0
    property bool indexHeld: false
    property var rowItems: ({})
    // a row asked for before the displays were read lands once they are
    property string pendingRow: ""

    readonly property var sel: {
        page.tick;
        return page.selected >= 0 && page.selected < page.draft.length ? page.draft[page.selected] : null;
    }
    readonly property bool selOn: {
        page.tick;
        return page.sel !== null && !page.sel.disabled;
    }
    readonly property bool canMirror: Settings.supports("outputMirror")
    readonly property bool canHdr: Settings.supports("outputHdr")
    readonly property bool canNight: Settings.supports("nightLight")
    readonly property bool clock24: Settings.get("general.clock_format_24_h") === true

    readonly property bool dirty: {
        page.tick;
        return page.monCount > 0 && JSON.stringify(page.specsAll()) !== page.committed;
    }
    readonly property int enabledCount: {
        page.tick;
        return page.draft.filter(m => !m.disabled).length;
    }

    readonly property bool sideIndex: page.width >= LithicTokens.size.settingsExpandedMax
        + LithicTokens.size.sectionIndexWidth + LithicTokens.size.sectionIndexGap
    readonly property real columnWidth: page.sideIndex
        ? Math.min(page.width - LithicTokens.size.sectionIndexWidth - LithicTokens.size.sectionIndexGap, LithicTokens.size.settingsExpandedMax)
        : Math.min(page.width, page.hub.sizeClass === "expanded" ? LithicTokens.size.settingsExpandedMax : LithicTokens.size.settingsMediumMax)
    readonly property real groupX: page.sideIndex
        ? Math.max(0, (page.width - page.columnWidth - LithicTokens.size.sectionIndexWidth - LithicTokens.size.sectionIndexGap) / 2)
        : Math.max(0, (page.width - page.columnWidth) / 2)
    readonly property real columnX: page.sideIndex
        ? page.groupX + LithicTokens.size.sectionIndexWidth + LithicTokens.size.sectionIndexGap
        : page.groupX

    readonly property string stateLine: {
        page.tick;
        if (!page.listed || page.listFailed || page.monCount === 0)
            return "";
        if (page.monCount === 1)
            return I18n.tr("One display connected.");
        if (page.monCount === 2)
            return I18n.tr("Two displays connected. The main one is %1.").replace("%1", page.mainName);
        return I18n.tr("%1 displays connected. The main one is %2.").replace("%1", page.monCount).replace("%2", page.mainName);
    }

    function specsAll() {
        return page.draft.map(m => Model.spec(m));
    }

    function loadRows(rows, tidy) {
        var d = rows.map(r => Model.clone(r));
        page.draft = d;
        page.monCount = d.length;
        page.mainName = Arrange.deriveMain(page.monsView());
        page.committed = JSON.stringify(page.specsAll());
        // a display left apart from the rest strands the pointer, so opening proposes the fix
        if (tidy && page.tidyGaps())
            page.normalize();
        var at = d.findIndex(m => m.name === page.selectedName);
        page.selected = at >= 0 ? at : 0;
        page.selectedName = d.length > 0 ? d[page.selected].name : "";
        page.tick++;
    }

    function selectIndex(i) {
        if (i < 0 || i >= page.draft.length)
            return;
        page.selected = i;
        page.selectedName = page.draft[i].name;
    }

    function setField(key, v) {
        if (!page.sel)
            return;
        page.sel[key] = v;
        if (key === "transform" || key === "scale" || key === "disabled") {
            if (key === "transform")
                page.sel.scale = Model.snapScale(page.sel, page.sel.scale);
            if (page.tidyGaps())
                page.normalize();
        }
        page.tick++;
    }

    function setMode(key, w, h, rate) {
        var m = page.sel;
        if (!m)
            return;
        m.mode = key;
        m.width = w;
        m.height = h;
        m.refresh = rate;
        m.scale = Model.snapScale(m, m.scale);
        if (page.tidyGaps())
            page.normalize();
        page.tick++;
    }

    function setModeKey(key) {
        var p = Model.parseMode(key);
        if (p)
            page.setMode(p.key, p.w, p.h, p.rate);
    }

    function setCustomMode(w, h, hz) {
        page.setMode(w + "x" + h + "@" + hz, w, h, hz);
    }

    function monsView() {
        return page.draft.map(m => ({ name: m.name, x: m.x, y: m.y, w: Model.footW(m), h: Model.footH(m), disabled: m.disabled }));
    }

    function applyMons(mons) {
        for (var i = 0; i < page.draft.length; i++)
            for (var j = 0; j < mons.length; j++)
                if (mons[j].name === page.draft[i].name) {
                    page.draft[i].x = mons[j].x;
                    page.draft[i].y = mons[j].y;
                    break;
                }
    }

    function tidyGaps() {
        var mons = page.monsView();
        var changed = Arrange.tidyGaps(mons);
        if (changed)
            page.applyMons(mons);
        return changed;
    }

    function normalize() {
        var mons = page.monsView();
        Arrange.rebaseToMain(mons, page.mainName);
        page.applyMons(mons);
    }

    function dragMonitor(i, dx, dy, view) {
        if (i < 0 || i >= page.draft.length)
            return;
        if (!page.dragging) {
            page.frozen = view;
            page.dragging = true;
        }
        page.draft[i].x += dx;
        page.draft[i].y += dy;
        page.tick++;
    }

    function nearestSnap(v, cands, th) {
        var best = v, bd = th;
        for (var i = 0; i < cands.length; i++) {
            var d = Math.abs(v - cands[i]);
            if (d < bd) {
                bd = d;
                best = cands[i];
            }
        }
        return best;
    }

    // on drop: snap the near edges, keep every display touching another, re-anchor on the main one
    function endDrag(i, k) {
        if (i < 0 || i >= page.draft.length) {
            page.dragging = false;
            return;
        }
        var m = page.draft[i];
        var th = Math.max(8, 22 / Math.max(0.0001, k));
        var xs = [0], ys = [0];
        for (var j = 0; j < page.draft.length; j++) {
            if (j === i || page.draft[j].disabled)
                continue;
            var o = page.draft[j];
            xs.push(o.x, o.x + Model.footW(o), o.x - Model.footW(m));
            ys.push(o.y, o.y + Model.footH(o), o.y - Model.footH(m));
        }
        m.x = Math.round(page.nearestSnap(m.x, xs, th));
        m.y = Math.round(page.nearestSnap(m.y, ys, th));
        var mons = page.monsView();
        if (!Arrange.touchesAny(mons, i)) {
            Arrange.attachFlush(mons, i);
            page.applyMons(mons);
        }
        page.normalize();
        page.dragging = false;
        page.tick++;
    }

    function setMain() {
        if (!page.selOn)
            return;
        page.mainName = page.sel.name;
        page.normalize();
        page.tick++;
    }

    // every display that is on, side by side in their current order, mirroring cleared
    function extend() {
        var order = page.draft.map((m, i) => i).sort((a, b) => page.draft[a].x - page.draft[b].x);
        var x = 0;
        for (var k = 0; k < order.length; k++) {
            var m = page.draft[order[k]];
            m.mirror = "";
            if (m.disabled)
                continue;
            m.x = x;
            m.y = 0;
            x += Model.footW(m);
        }
        page.mainName = Arrange.deriveMain(page.monsView());
        page.tick++;
    }

    function matchScale() {
        for (var i = 0; i < page.draft.length; i++)
            if (!page.draft[i].disabled)
                page.draft[i].scale = Model.dpiScale(page.draft[i]);
        if (page.tidyGaps())
            page.normalize();
        page.tick++;
    }

    function mirrorAll() {
        var src = page.mainName;
        if (!src) {
            var first = page.draft.find(m => !m.disabled);
            src = first ? first.name : "";
        }
        if (!src)
            return;
        for (var j = 0; j < page.draft.length; j++)
            if (!page.draft[j].disabled)
                page.draft[j].mirror = page.draft[j].name === src ? "" : src;
        page.tick++;
    }

    function discard() {
        page.applyError = "";
        page.loadRows(page.liveRows, false);
    }

    // apply, then ask to keep; no answer within 15 seconds goes back
    function apply() {
        if (!page.dirty || page.applying)
            return;
        var prev = page.committed;
        var next = JSON.stringify(page.specsAll());
        page.applying = true;
        page.applyError = "";
        page.run(["ryoku-hub", "outputs", "apply", next], function (ok, out, err) {
            page.applying = false;
            if (!ok) {
                page.applyError = err.trim() || I18n.tr("The window manager did not answer.");
                return;
            }
            page.committed = next;
            page.tick++;
            page.askKeep(prev);
            readback.restart();
        });
    }

    function askKeep(prev) {
        page.hub.confirm({
            title: I18n.tr("Keep these display settings?"),
            text: I18n.tr("Your displays go back to how they were unless you keep the change."),
            actionText: I18n.tr("Keep"),
            cancelText: I18n.tr("Revert"),
            destructive: false,
            timeout: 15
        }, function () {}, function () { page.revertTo(prev); });
    }

    function revertTo(layout) {
        readback.stop();
        page.applying = true;
        page.run(["ryoku-hub", "outputs", "apply", layout], function (ok, out, err) {
            page.applying = false;
            if (!ok)
                page.applyError = err.trim() || I18n.tr("The window manager did not answer.");
            readback.restart();
        });
    }

    // saved layouts: whole arrangements and single resolutions in one list

    readonly property var savedItems: {
        var out = [];
        for (var i = 0; i < page.profiles.length; i++)
            out.push({ kind: "layout", name: page.profiles[i].name, matches: page.profiles[i].matches === true });
        for (var j = 0; j < page.presets.length; j++)
            out.push({ kind: "resolution", name: page.presets[j].name, mode: page.presets[j].mode, scale: page.presets[j].scale });
        return out;
    }

    function savedDetail(item) {
        if (item.kind === "layout")
            return item.matches ? I18n.tr("Every display, ready for the displays connected now")
                                : I18n.tr("Every display, for displays that are not all connected now");
        var p = Model.parseMode(item.mode);
        var res = p ? p.w + " \u00d7 " + p.h + ", " + Math.round(p.rate) + " Hz" : item.mode;
        return I18n.tr("One display: %1 at %2 scale").replace("%1", res).replace("%2", Math.round(item.scale * 100) + "%");
    }

    function saveLayout(name) {
        name = String(name || "").trim();
        if (name === "")
            return;
        page.layoutsError = "";
        page.run(["ryoku-hub", "outputs", "save", name, page.committed], function (ok, out, err) {
            if (!ok)
                page.layoutsError = err.trim();
            page.reloadSaved();
        });
        saveLayoutField.text = "";
    }

    function saveResolution(name) {
        name = String(name || "").trim();
        if (name === "" || !page.sel)
            return;
        page.layoutsError = "";
        page.run(["ryoku-hub", "outputs", "preset-save", name, page.sel.mode, "" + page.sel.scale], function (ok, out, err) {
            if (!ok)
                page.layoutsError = err.trim();
            page.reloadSaved();
        });
        saveResolutionField.text = "";
    }

    function useSaved(item) {
        if (item.kind === "resolution") {
            if (!page.selOn)
                return;
            page.setModeKey(item.mode);
            page.sel.scale = Model.snapScale(page.sel, item.scale);
            if (page.tidyGaps())
                page.normalize();
            page.tick++;
            return;
        }
        if (page.applying)
            return;
        var prev = page.committed;
        page.applying = true;
        page.layoutsError = "";
        page.run(["ryoku-hub", "outputs", "load", item.name], function (ok, out, err) {
            page.applying = false;
            if (!ok) {
                page.layoutsError = err.trim();
                return;
            }
            page.askKeep(prev);
            readback.restart();
        });
    }

    function deleteSaved(item) {
        page.hub.confirm({
            title: I18n.tr("Delete %1?").replace("%1", item.name),
            text: item.kind === "layout" ? I18n.tr("The saved layout goes. Your displays stay as they are.")
                                         : I18n.tr("The saved resolution goes. Your displays stay as they are."),
            actionText: I18n.tr("Delete")
        }, function () {
            page.layoutsError = "";
            page.run(["ryoku-hub", "outputs", item.kind === "layout" ? "rm" : "preset-rm", item.name], function (ok, out, err) {
                if (!ok)
                    page.layoutsError = err.trim();
                page.reloadSaved();
            });
        }, null);
    }

    function sizeBind(name) {
        return { store: "settings", key: "displays.ui_scale." + name, fallback: 1 };
    }

    function sizePct(name) {
        var v = Changes.value(page.sizeBind(name));
        return typeof v === "number" && v > 0 ? Math.round(v * 100) : 100;
    }

    function rowVisible(id) {
        var r = Schema.row(id);
        if (!r)
            return false;
        if (r.needs && r.needs.cap && !Settings.supports(r.needs.cap))
            return false;
        if (!r.when)
            return true;
        var v = Changes.value(r.when.bind);
        return r.when.is.some(x => JSON.stringify(x) === JSON.stringify(v));
    }

    function sectionItem(i) {
        return [secArrangement, secDisplay, secSize, secLayouts, secNight][i] || null;
    }

    function chooseSection(index) {
        var item = page.sectionItem(index);
        if (!item || !item.visible)
            return;
        page.currentSection = index;
        page.indexHeld = true;
        page.scrollTo(item.mapToItem(flick.contentItem, 0, 0).y);
    }

    function followReading() {
        if (page.indexHeld)
            return;
        if (flick.contentY >= flick.contentHeight - flick.height - 1) {
            page.currentSection = 4;
            return;
        }
        var current = 0;
        for (var i = 0; i < 5; i++) {
            var item = page.sectionItem(i);
            if (!item || !item.visible)
                continue;
            var top = item.mapToItem(flick.contentItem, 0, 0).y;
            var visibleAbove = top + item.height - flick.contentY;
            if (top <= flick.contentY + flick.height / 3 && !(visibleAbove < item.height / 3 && i < 4))
                current = i;
        }
        page.currentSection = current;
    }

    function scrollTo(y) {
        var target = Math.max(0, Math.min(y, flick.contentHeight - flick.height));
        scrollAnim.stop();
        scrollAnim.from = flick.contentY;
        scrollAnim.to = target;
        scrollAnim.start();
    }

    // a hidden row leads to the row that shows it
    readonly property var landing: ({
        "night-start": "night-schedule", "night-end": "night-schedule", "night-margin": "night-schedule",
        "sdr-brightness": "colour", "saved": "save-layout",
        "mirror-all": "arrange", "extend": "arrange", "match-scale": "arrange", "set-main": "arrange"
    })

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (!page.listed) {
            page.pendingRow = rowId;
            return;
        }
        var id = rowId;
        if (id === "position" && page.sel !== null) {
            landTimer.rowId = id;
            landTimer.interval = moreDisplay.open ? 60 : LithicTokens.motion.unfold + 60;
            moreDisplay.open = true;
            landTimer.restart();
            return;
        }
        var item = page.rowItems[id];
        if ((!item || !item.visible) && page.landing[id]) {
            id = page.landing[id];
            item = page.rowItems[id];
        }
        if (!item || !item.visible)
            return;
        landTimer.rowId = id;
        landTimer.interval = 60;
        landTimer.restart();
    }

    function land(id) {
        var item = page.rowItems[id];
        if (!item)
            return;
        page.indexHeld = false;
        page.highlightId = "";
        scrollAnim.landing = id;
        page.scrollTo(item.mapToItem(flick.contentItem, 0, 0).y - flick.height / 3);
    }

    function run(argv, done) {
        var p = commandProc.createObject(page, { command: argv });
        p.finished.connect(function (ok, out, err) {
            if (done)
                done(ok, out, err);
            p.destroy();
        });
        p.running = true;
    }

    function reload() {
        listProc.running = true;
    }

    function reloadSaved() {
        profilesProc.running = true;
        presetsProc.running = true;
    }

    Component {
        id: commandProc
        Process {
            id: proc
            signal finished(bool ok, string out, string err)
            stdout: StdioCollector { id: cout }
            stderr: StdioCollector { id: cerr }
            onExited: (code, status) => proc.finished(code === 0, cout.text, cerr.text)
        }
    }

    Process {
        id: listProc
        command: ["ryoku-hub", "outputs"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var arr = JSON.parse(this.text);
                    if (!Array.isArray(arr))
                        throw "not a list";
                    page.liveRows = arr;
                    page.loadRows(arr, true);
                    page.listFailed = false;
                } catch (e) {
                    page.listFailed = true;
                }
                page.listed = true;
                if (page.pendingRow !== "") {
                    Qt.callLater(page.focusRow, page.pendingRow);
                    page.pendingRow = "";
                }
            }
        }
    }

    Process {
        id: profilesProc
        command: ["ryoku-hub", "outputs", "profiles"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var v = JSON.parse(this.text);
                    page.profiles = Array.isArray(v) ? v : [];
                } catch (e) {
                    page.profiles = [];
                }
            }
        }
    }

    Process {
        id: presetsProc
        command: ["ryoku-hub", "outputs", "presets"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var v = JSON.parse(this.text);
                    page.presets = Array.isArray(v) ? v : [];
                } catch (e) {
                    page.presets = [];
                }
            }
        }
    }

    // read back what the window manager settled on
    Timer {
        id: readback
        interval: 700
        onTriggered: {
            page.reload();
            page.reloadSaved();
        }
    }

    Timer {
        id: landTimer
        property string rowId: ""
        interval: LithicTokens.motion.unfold + 20
        onTriggered: {
            landTimer.interval = LithicTokens.motion.unfold + 20;
            page.land(landTimer.rowId);
        }
    }

    Timer {
        id: highlightClear
        interval: Lithic.reduceMotion ? LithicTokens.motion.placardLife : LithicTokens.motion.highlight + 100
        onTriggered: page.highlightId = ""
    }

    NumberAnimation {
        id: scrollAnim
        property string landing: ""
        target: flick
        property: "contentY"
        duration: Lithic.duration(LithicTokens.motion.navigate)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Lithic.curve
        onStopped: {
            if (scrollAnim.landing === "")
                return;
            page.highlightId = scrollAnim.landing;
            scrollAnim.landing = "";
            highlightClear.restart();
        }
    }

    // the night light lives as long as the window so its undo works from any page
    Component.onCompleted: Changes.ensureStore("nightlight", Qt.resolvedUrl("../components/displays/NightLightStore.qml"))

    Flickable {
        id: flick
        anchors.fill: parent
        contentWidth: width
        contentHeight: body.y + body.height + LithicTokens.size.settingsInsetVertical
        bottomMargin: applyBar.visible ? applyBar.height + LithicTokens.space.s5 : 0
        boundsBehavior: Flickable.StopAtBounds
        clip: true
        onContentYChanged: page.followReading()
        onMovementStarted: page.indexHeld = false

        Column {
            id: body
            x: page.columnX
            width: page.columnWidth
            spacing: 0

            Label {
                width: parent.width
                text: I18n.tr(Schema.title)
                typeRole: "name"
            }
            Item { width: 1; height: LithicTokens.space.s2 }
            Label {
                width: parent.width
                text: I18n.tr(Schema.lead)
                typeRole: "body"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Label {
                width: parent.width
                visible: page.stateLine.length > 0
                topPadding: LithicTokens.space.s1
                text: page.stateLine
                typeRole: "detail"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Item { width: 1; height: LithicTokens.space.s5 }

            SectionIndex {
                visible: !page.sideIndex
                width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
                sizeClass: "medium"
                sections: Schema.sections.map(s => I18n.tr(s.title))
                currentIndex: page.currentSection
                overlayParent: page.hub.overlayItem
                onActivated: index => page.chooseSection(index)
            }
            Item {
                visible: !page.sideIndex
                width: 1
                height: LithicTokens.space.s5
            }

            Section {
                id: secArrangement
                width: body.width
                title: I18n.tr("Arrangement")

                Slab {
                    width: parent.width

                    Column {
                        width: parent.width
                        spacing: 0

                        ArrangeCanvas {
                            id: canvas
                            width: parent.width
                            height: Math.round(Math.max(200, Math.min(320, parent.width * 0.34)))
                            page: page
                        }

                        Item { width: 1; height: LithicTokens.space.s3 }

                        Item {
                            id: arrangeFoot
                            width: parent.width
                            readonly property bool stacked: hint.implicitWidth + actions.implicitWidth + LithicTokens.space.s4 > width
                            height: arrangeFoot.stacked ? hint.height + LithicTokens.space.s2 + actions.height
                                                        : Math.max(hint.height, actions.height)

                            Label {
                                id: hint
                                width: arrangeFoot.stacked ? parent.width : Math.min(implicitWidth, parent.width - actions.width - LithicTokens.space.s4)
                                anchors.verticalCenter: arrangeFoot.stacked ? undefined : parent.verticalCenter
                                text: I18n.tr("Drag a display to where it sits on your desk. Edges snap together.")
                                typeRole: "detail"
                                inkRole: "fg2"
                                wrapMode: Text.Wrap
                                elide: Text.ElideNone
                            }

                            Row {
                                id: actions
                                anchors.right: parent.right
                                y: arrangeFoot.stacked ? hint.height + LithicTokens.space.s2 : (parent.height - height) / 2
                                spacing: LithicTokens.space.s2

                                Plaque {
                                    visible: page.canMirror && page.enabledCount > 1
                                    kind: "quiet"
                                    small: true
                                    glyph: "content_copy"
                                    text: I18n.tr("Mirror")
                                    onClicked: page.mirrorAll()
                                }
                                Plaque {
                                    visible: page.enabledCount > 1
                                    kind: "quiet"
                                    small: true
                                    glyph: "view_week"
                                    text: I18n.tr("Extend")
                                    onClicked: page.extend()
                                }
                                Plaque {
                                    enabled: page.enabledCount > 0
                                    kind: "quiet"
                                    small: true
                                    glyph: "aspect_ratio"
                                    text: I18n.tr("Match scale")
                                    onClicked: page.matchScale()
                                }
                                Plaque {
                                    id: setMainPlaque
                                    readonly property bool isMain: page.sel !== null && page.sel.name === page.mainName
                                    enabled: page.selOn && !setMainPlaque.isMain
                                    kind: "quiet"
                                    small: true
                                    glyph: setMainPlaque.isMain ? "check" : "star"
                                    text: setMainPlaque.isMain ? I18n.tr("Main display") : I18n.tr("Set as main")
                                    onClicked: page.setMain()
                                }
                            }
                        }
                    }
                }
                Component.onCompleted: page.rowItems["arrange"] = secArrangement
            }

            Section {
                id: secDisplay
                width: body.width
                title: I18n.tr("Each display")

                Slab {
                    width: parent.width
                    visible: page.sel !== null
                    implicitHeight: visible ? contentImplicitHeight + inset * 2 : 0

                    Column {
                        width: parent.width

                        SettingRow {
                            width: parent.width
                            first: true
                            title: page.sel ? page.sel.name : ""
                            detail: {
                                if (!page.sel)
                                    return "";
                                var who = [page.sel.make, page.sel.model].filter(s => s && s.length > 0).join(" ");
                                if (page.sel.name === page.mainName && page.selOn)
                                    return who.length > 0 ? I18n.tr("%1, the main display").replace("%1", who) : I18n.tr("The main display");
                                return who;
                            }

                            Picker {
                                id: displayPicker
                                visible: page.monCount > 1
                                width: 200
                                options: { page.tick; return page.draft.map(m => m.name); }
                                currentIndex: page.selected
                                overlayParent: page.hub.overlayItem
                                Accessible.name: I18n.tr("Display")
                                onActivated: index => {
                                    page.selectIndex(index);
                                    displayPicker.currentIndex = Qt.binding(() => page.selected);
                                }
                            }
                        }

                        SpecRow {
                            host: page
                            rowId: "enabled"
                            activatable: page.selOn ? page.enabledCount > 1 : true
                            unavailable: page.selOn && page.enabledCount <= 1 ? I18n.tr("Your only display that is on stays on") : ""
                            onActivated: page.setField("disabled", page.selOn)

                            Toggle {
                                checked: page.selOn
                                Accessible.name: I18n.tr("Use this display")
                                onToggled: page.setField("disabled", !checked)
                            }
                        }

                        SpecRow {
                            host: page
                            rowId: "resolution"
                            unavailable: page.selOn ? "" : I18n.tr("Turn the display on to change this")

                            Picker {
                                id: sizePicker
                                readonly property var list: { page.tick; return page.sel ? Model.sizes(page.sel) : []; }
                                width: 220
                                options: sizePicker.list.map(s => s.custom
                                    ? I18n.tr("%1 (custom)").replace("%1", s.w + " \u00d7 " + s.h)
                                    : s.w + " \u00d7 " + s.h).concat([I18n.tr("Custom resolution\u2026")])
                                currentIndex: { page.tick; return page.sel ? sizePicker.list.findIndex(s => s.w === page.sel.width && s.h === page.sel.height) : -1; }
                                overlayParent: page.hub.overlayItem
                                Accessible.name: I18n.tr("Resolution")
                                onActivated: index => {
                                    if (index >= sizePicker.list.length) {
                                        customDialog.openFor(page.sel);
                                    } else {
                                        var s = sizePicker.list[index];
                                        if (!s.custom)
                                            page.setModeKey(Model.modeForSize(page.sel, s.w, s.h));
                                    }
                                    sizePicker.currentIndex = Qt.binding(() => { page.tick; return page.sel ? sizePicker.list.findIndex(s => s.w === page.sel.width && s.h === page.sel.height) : -1; });
                                }
                            }
                        }

                        SpecRow {
                            host: page
                            rowId: "refresh"
                            unavailable: page.selOn ? "" : I18n.tr("Turn the display on to change this")

                            Picker {
                                id: ratePicker
                                readonly property var list: { page.tick; return page.sel ? Model.rates(page.sel) : []; }
                                width: 220
                                options: ratePicker.list.map(r => r.custom
                                    ? I18n.tr("%1 (custom)").replace("%1", Model.rateText(r.rate, ratePicker.list))
                                    : Model.rateText(r.rate, ratePicker.list))
                                currentIndex: { page.tick; return page.sel ? ratePicker.list.findIndex(r => r.key === page.sel.mode) : -1; }
                                overlayParent: page.hub.overlayItem
                                Accessible.name: I18n.tr("Refresh rate")
                                onActivated: index => {
                                    page.setModeKey(ratePicker.list[index].key);
                                    ratePicker.currentIndex = Qt.binding(() => { page.tick; return page.sel ? ratePicker.list.findIndex(r => r.key === page.sel.mode) : -1; });
                                }
                            }
                        }

                        SpecRow {
                            host: page
                            rowId: "scale"
                            unavailable: page.selOn ? "" : I18n.tr("Turn the display on to change this")

                            Stepper {
                                id: scaleStep
                                readonly property var ladder: { page.tick; return page.sel ? Model.scaleLadder(page.sel) : [1]; }
                                from: 0
                                to: scaleStep.ladder.length - 1
                                value: { page.tick; return page.sel ? Model.nearestIdx(scaleStep.ladder, page.sel.scale) : 0; }
                                textFromValue: (v, locale) => Math.round((scaleStep.ladder[v] || 1) * 100) + "%"
                                accessibleName: I18n.tr("Scale")
                                onValueModified: page.setField("scale", scaleStep.ladder[scaleStep.value])
                            }
                        }

                        SpecRow {
                            host: page
                            rowId: "rotation"
                            unavailable: page.selOn ? "" : I18n.tr("Turn the display on to change this")

                            Segmented {
                                id: rotationSeg
                                model: [I18n.tr("Normal"), "90\u00b0", "180\u00b0", "270\u00b0"]
                                currentIndex: { page.tick; return page.sel ? Model.rotation(page.sel.transform) : 0; }
                                accessibleName: I18n.tr("Rotation")
                                onActivated: index => {
                                    page.setField("transform", Model.withRotation(page.sel.transform, index));
                                    rotationSeg.currentIndex = Qt.binding(() => { page.tick; return page.sel ? Model.rotation(page.sel.transform) : 0; });
                                }
                            }
                        }

                        SpecRow {
                            host: page
                            rowId: "vrr"
                            activatable: true
                            unavailable: page.selOn ? "" : I18n.tr("Turn the display on to change this")
                            onActivated: page.setField("vrr", !page.sel.vrr)

                            Toggle {
                                checked: { page.tick; return page.sel ? page.sel.vrr === true : false; }
                                Accessible.name: I18n.tr("Adaptive sync")
                                onToggled: page.setField("vrr", checked)
                            }
                        }

                        SpecRow {
                            host: page
                            rowId: "colour"
                            visible: page.canHdr
                            unavailable: page.selOn ? "" : I18n.tr("Turn the display on to change this")

                            Segmented {
                                id: colourSeg
                                readonly property var keys: ["srgb", "wide", "hdr"]
                                model: [I18n.tr("Standard"), I18n.tr("Wide"), I18n.tr("HDR")]
                                currentIndex: { page.tick; return page.sel ? Math.max(0, colourSeg.keys.indexOf(page.sel.colorMode)) : 0; }
                                accessibleName: I18n.tr("Colour")
                                onActivated: index => {
                                    page.setField("colorMode", colourSeg.keys[index]);
                                    colourSeg.currentIndex = Qt.binding(() => { page.tick; return page.sel ? Math.max(0, colourSeg.keys.indexOf(page.sel.colorMode)) : 0; });
                                }
                            }
                        }

                        SpecRow {
                            host: page
                            rowId: "sdr-brightness"
                            visible: { page.tick; return page.canHdr && page.sel !== null && page.sel.colorMode === "hdr"; }
                            unavailable: page.selOn ? "" : I18n.tr("Turn the display on to change this")

                            Stepper {
                                id: sdrStep
                                from: 0
                                to: Model.sdrLadder.length - 1
                                value: { page.tick; return page.sel ? Model.nearestIdx(Model.sdrLadder, page.sel.sdrBrightness) : 0; }
                                textFromValue: (v, locale) => Math.round(Model.sdrLadder[v] * 100) + "%"
                                accessibleName: I18n.tr("Everyday content brightness")
                                onValueModified: page.setField("sdrBrightness", Model.sdrLadder[sdrStep.value])
                            }
                        }

                        SpecRow {
                            host: page
                            rowId: "mirror-of"
                            visible: page.canMirror && page.monCount > 1
                            unavailable: page.selOn ? "" : I18n.tr("Turn the display on to change this")

                            Picker {
                                id: mirrorPicker
                                readonly property var list: {
                                    page.tick;
                                    return [""].concat(page.draft.filter((m, i) => i !== page.selected && !m.disabled).map(m => m.name));
                                }
                                width: 200
                                options: mirrorPicker.list.map(n => n === "" ? I18n.tr("Nothing") : n)
                                currentIndex: { page.tick; return page.sel ? Math.max(0, mirrorPicker.list.indexOf(page.sel.mirror)) : 0; }
                                overlayParent: page.hub.overlayItem
                                Accessible.name: I18n.tr("Show a copy of")
                                onActivated: index => {
                                    page.setField("mirror", mirrorPicker.list[index]);
                                    mirrorPicker.currentIndex = Qt.binding(() => { page.tick; return page.sel ? Math.max(0, mirrorPicker.list.indexOf(page.sel.mirror)) : 0; });
                                }
                            }
                        }

                        Item {
                            width: 1
                            height: LithicTokens.size.settingsRowGap
                        }

                        Disclosure {
                            id: moreDisplay
                            width: parent.width
                            text: I18n.tr("More options")

                            SpecRow {
                                host: page
                                rowId: "position"
                                first: true
                                unavailable: page.selOn ? "" : I18n.tr("Turn the display on to change this")

                                Label {
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: "X"
                                    typeRole: "label"
                                    inkRole: "fg3"
                                }
                                TextField {
                                    id: posX
                                    width: 110
                                    placeholderText: "X"
                                    inputMethodHints: Qt.ImhFormattedNumbersOnly
                                    validator: IntValidator { bottom: -32768; top: 32768 }
                                    Accessible.name: I18n.tr("Horizontal position")
                                    onEditingFinished: {
                                        var n = parseInt(posX.text, 10);
                                        if (!isNaN(n) && page.sel && n !== page.sel.x)
                                            page.setField("x", n);
                                    }
                                    Binding on text {
                                        when: !posX.activeFocus
                                        value: { page.tick; return page.sel ? String(page.sel.x) : ""; }
                                    }
                                }
                                Label {
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: "Y"
                                    typeRole: "label"
                                    inkRole: "fg3"
                                }
                                TextField {
                                    id: posY
                                    width: 110
                                    placeholderText: "Y"
                                    inputMethodHints: Qt.ImhFormattedNumbersOnly
                                    validator: IntValidator { bottom: -32768; top: 32768 }
                                    Accessible.name: I18n.tr("Vertical position")
                                    onEditingFinished: {
                                        var n = parseInt(posY.text, 10);
                                        if (!isNaN(n) && page.sel && n !== page.sel.y)
                                            page.setField("y", n);
                                    }
                                    Binding on text {
                                        when: !posY.activeFocus
                                        value: { page.tick; return page.sel ? String(page.sel.y) : ""; }
                                    }
                                }
                            }
                        }
                    }
                }

                Label {
                    width: parent.width
                    visible: page.sel === null
                    text: !page.listed ? I18n.tr("Looking for displays") : I18n.tr("No displays to set up.")
                    typeRole: "detail"
                    inkRole: "fg3"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
            }

            Section {
                id: secSize
                width: body.width
                title: I18n.tr("Desktop size")

                Slab {
                    width: parent.width

                    Column {
                        width: parent.width

                        BoundRow {
                            host: page
                            rowId: "desktop-size"
                            first: true
                        }

                        Repeater {
                            model: { page.tick; return page.draft.map(m => m.name); }
                            delegate: Column {
                                id: sizeRow
                                required property string modelData
                                required property int index
                                readonly property var bind: page.sizeBind(sizeRow.modelData)
                                readonly property string label: I18n.tr("Desktop size on %1").replace("%1", sizeRow.modelData)
                                width: parent.width
                                spacing: LithicTokens.space.s1
                                Component.onCompleted: if (sizeRow.index === 0) page.rowItems["desktop-size-display"] = sizeRow

                                SettingRow {
                                    width: parent.width
                                    title: I18n.tr("On %1").replace("%1", sizeRow.modelData)
                                    detail: page.sizePct(sizeRow.modelData) === 100
                                        ? I18n.tr("Same as every display")
                                        : I18n.tr("Larger or smaller than on the others, on this display only")
                                    changed: page.sizePct(sizeRow.modelData) !== 100
                                    highlighted: sizeRow.index === 0 && page.highlightId === "desktop-size-display"
                                    onResetRequested: Changes.set(sizeRow.bind, 1, sizeRow.label, page.pageId)

                                    Stepper {
                                        from: 50
                                        to: 200
                                        stepSize: 5
                                        unit: "%"
                                        value: page.sizePct(sizeRow.modelData)
                                        accessibleName: sizeRow.label
                                        onValueModified: Changes.set(sizeRow.bind, value / 100, sizeRow.label, page.pageId)
                                    }
                                }

                                ErrorLine {
                                    bind: sizeRow.bind
                                }
                            }
                        }
                    }
                }
            }

            Section {
                id: secLayouts
                width: body.width
                title: I18n.tr("Saved layouts")

                Slab {
                    width: parent.width

                    Column {
                        width: parent.width

                        SpecRow {
                            host: page
                            rowId: "save-layout"
                            first: true
                            detail: page.dirty
                                ? I18n.tr("Keeps your displays as they are now; changes waiting to be applied are not included")
                                : I18n.tr(Schema.row("save-layout").detail)

                            TextField {
                                id: saveLayoutField
                                width: 200
                                placeholderText: I18n.tr("Name")
                                Accessible.name: I18n.tr("Layout name")
                                onAccepted: page.saveLayout(saveLayoutField.text)
                            }
                            Plaque {
                                anchors.verticalCenter: parent.verticalCenter
                                kind: "tonal"
                                small: true
                                text: I18n.tr("Save")
                                enabled: saveLayoutField.text.trim().length > 0 && page.monCount > 0
                                accessibleName: I18n.tr("Save the current layout")
                                onClicked: page.saveLayout(saveLayoutField.text)
                            }
                        }

                        SpecRow {
                            host: page
                            rowId: "save-resolution"
                            unavailable: page.selOn ? "" : I18n.tr("Turn the display on to save its resolution")

                            TextField {
                                id: saveResolutionField
                                width: 200
                                placeholderText: I18n.tr("Name")
                                Accessible.name: I18n.tr("Resolution name")
                                onAccepted: page.saveResolution(saveResolutionField.text)
                            }
                            Plaque {
                                anchors.verticalCenter: parent.verticalCenter
                                kind: "tonal"
                                small: true
                                text: I18n.tr("Save")
                                enabled: saveResolutionField.text.trim().length > 0
                                accessibleName: I18n.tr("Save this display's resolution")
                                onClicked: page.saveResolution(saveResolutionField.text)
                            }
                        }

                        Repeater {
                            model: page.savedItems
                            delegate: SettingRow {
                                id: savedRow
                                required property var modelData
                                required property int index
                                width: parent.width
                                title: savedRow.modelData.name
                                detail: page.savedDetail(savedRow.modelData)
                                highlighted: savedRow.index === 0 && page.highlightId === "saved"
                                Component.onCompleted: if (savedRow.index === 0) page.rowItems["saved"] = savedRow

                                Plaque {
                                    anchors.verticalCenter: parent.verticalCenter
                                    kind: "tonal"
                                    small: true
                                    text: savedRow.modelData.kind === "layout" ? I18n.tr("Apply") : I18n.tr("Use here")
                                    enabled: savedRow.modelData.kind === "layout" ? !page.applying : page.selOn
                                    accessibleName: (savedRow.modelData.kind === "layout" ? I18n.tr("Apply %1") : I18n.tr("Use %1 on this display"))
                                        .replace("%1", savedRow.modelData.name)
                                    onClicked: page.useSaved(savedRow.modelData)
                                }
                                IconPlaque {
                                    anchors.verticalCenter: parent.verticalCenter
                                    kind: "quiet"
                                    small: true
                                    glyph: "delete"
                                    accessibleName: I18n.tr("Delete %1").replace("%1", savedRow.modelData.name)
                                    onClicked: page.deleteSaved(savedRow.modelData)
                                }
                            }
                        }

                        Label {
                            width: parent.width
                            visible: page.savedItems.length === 0
                            topPadding: LithicTokens.size.settingsRowGap
                            text: I18n.tr("Nothing saved yet. Name the layout above and save it to bring it back later.")
                            typeRole: "detail"
                            inkRole: "fg3"
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }

                        Label {
                            width: parent.width
                            visible: page.layoutsError.length > 0
                            topPadding: LithicTokens.space.s2
                            text: page.layoutsError
                            typeRole: "detail"
                            ink: Lithic.jasper
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }
                    }
                }
            }

            Section {
                id: secNight
                width: body.width
                title: I18n.tr("Night light")

                Column {
                    width: parent.width
                    spacing: LithicTokens.space.s2

                    Slab {
                        width: parent.width
                        visible: page.canNight

                        Column {
                            width: parent.width

                            BoundRow {
                                host: page
                                rowId: "night-on"
                                first: true
                            }
                            BoundRow {
                                host: page
                                rowId: "night-temp"
                                custom: tempControl
                                unavailable: Changes.value(Schema.row("night-on").bind) === true ? "" : I18n.tr("Turn on Warm the screen to set how warm it gets")
                            }
                            BoundRow {
                                host: page
                                rowId: "night-schedule"
                                custom: scheduleControl
                            }
                            BoundRow {
                                host: page
                                rowId: "night-start"
                                custom: startControl
                                visible: page.rowVisible("night-start")
                            }
                            BoundRow {
                                host: page
                                rowId: "night-end"
                                custom: endControl
                                visible: page.rowVisible("night-end")
                            }
                            BoundRow {
                                host: page
                                rowId: "night-margin"
                                custom: marginControl
                                visible: page.rowVisible("night-margin")
                            }
                        }
                    }

                    Label {
                        width: parent.width
                        visible: !page.canNight
                        text: I18n.tr(Schema.sections[4].missing)
                        typeRole: "detail"
                        inkRole: "fg3"
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                }
            }

            Section {
                width: body.width
                title: I18n.tr("Related")

                Flow {
                    width: parent.width
                    spacing: LithicTokens.space.s2

                    Repeater {
                        model: Schema.related.filter(id => page.hub.pageAvailable(id))
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

    SectionIndex {
        visible: page.sideIndex
        x: page.groupX
        y: 0
        width: LithicTokens.size.sectionIndexWidth
        sizeClass: "expanded"
        sections: Schema.sections.map(s => I18n.tr(s.title))
        currentIndex: page.currentSection
        onActivated: index => page.chooseSection(index)
    }

    ScrollRail {
        flickable: flick
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.right: parent.right
    }

    // staged display changes wait here for one Apply
    CutRect {
        id: applyBar
        visible: page.dirty || page.applying || page.applyError.length > 0
        x: page.columnX
        width: page.columnWidth
        anchors.bottom: parent.bottom
        anchors.bottomMargin: LithicTokens.space.s4
        height: Math.max(LithicTokens.size.factsRowHeight, barText.implicitHeight) + LithicTokens.space.s3 * 2
        cut: LithicTokens.cut.mid
        fill: Lithic.stone4
        strokeColor: Lithic.vein
        strokeWidth: LithicTokens.size.hairline

        Label {
            id: barText
            anchors.left: parent.left
            anchors.leftMargin: LithicTokens.space.s4
            anchors.right: barActions.left
            anchors.rightMargin: LithicTokens.space.s4
            anchors.verticalCenter: parent.verticalCenter
            text: page.applyError.length > 0
                ? I18n.tr("Your displays did not take the change: %1").replace("%1", page.applyError)
                : page.applying ? I18n.tr("Applying")
                : I18n.tr("Display changes are waiting. Apply them to try them out.")
            typeRole: "body"
            ink: page.applyError.length > 0 ? Lithic.jasper : Lithic.fg
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Row {
            id: barActions
            anchors.right: parent.right
            anchors.rightMargin: LithicTokens.space.s3
            anchors.verticalCenter: parent.verticalCenter
            spacing: LithicTokens.space.s2

            Plaque {
                kind: "quiet"
                text: page.dirty ? I18n.tr("Discard") : I18n.tr("Dismiss")
                enabled: !page.applying
                onClicked: page.dirty ? page.discard() : (page.applyError = "")
            }
            Plaque {
                visible: page.dirty || page.applying
                kind: "gold"
                glyph: "check"
                text: I18n.tr("Apply")
                busy: page.applying
                enabled: page.dirty && !page.applying
                onClicked: page.apply()
            }
        }
    }

    Dialog {
        id: customDialog
        parent: page.hub.overlayItem
        title: I18n.tr("Custom resolution")
        property string error: ""
        actions: [
            { text: I18n.tr("Cancel"), kind: "quiet", closeOnTrigger: true },
            { text: I18n.tr("Use it"), kind: "gold", closeOnTrigger: false }
        ]
        function openFor(m) {
            customDialog.error = "";
            customW.text = m ? String(m.width) : "";
            customH.text = m ? String(m.height) : "";
            customHz.text = m ? String(Math.round(m.refresh)) : "";
            customDialog.open();
        }
        function submit() {
            var w = parseInt(customW.text, 10), h = parseInt(customH.text, 10), hz = parseInt(customHz.text, 10);
            if (!(w >= 320 && h >= 200 && hz >= 20 && w <= 16384 && h <= 16384 && hz <= 480)) {
                customDialog.error = I18n.tr("Use a width from 320 to 16384, a height from 200 to 16384 and a refresh rate from 20 to 480.");
                return;
            }
            page.setCustomMode(w, h, hz);
            customDialog.close();
        }
        onActionTriggered: index => {
            if (index === 1)
                customDialog.submit();
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s4

            Label {
                width: parent.width
                text: I18n.tr("Type the size and refresh rate. The display tries it when you apply, and goes back unless you keep it.")
                typeRole: "body"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }

            Row {
                spacing: LithicTokens.space.s3

                TextField {
                    id: customW
                    width: 120
                    label: I18n.tr("Width")
                    validator: IntValidator { bottom: 0; top: 16384 }
                    onAccepted: customDialog.submit()
                }
                TextField {
                    id: customH
                    width: 120
                    label: I18n.tr("Height")
                    validator: IntValidator { bottom: 0; top: 16384 }
                    onAccepted: customDialog.submit()
                }
                TextField {
                    id: customHz
                    width: 120
                    label: I18n.tr("Refresh rate")
                    validator: IntValidator { bottom: 0; top: 480 }
                    onAccepted: customDialog.submit()
                }
            }

            Label {
                width: parent.width
                visible: customDialog.error.length > 0
                text: customDialog.error
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }
    }

    Component {
        id: tempControl
        Stepper {
            readonly property var row: Schema.row("night-temp")
            from: 1000
            textFromValue: (v, locale) => String(v)
            to: 6500
            stepSize: 100
            unit: "K"
            value: Number(Changes.value(row.bind)) || 4000
            accessibleName: I18n.tr("Warmth")
            onValueModified: Changes.set(row.bind, value, row.label, page.pageId)
        }
    }

    Component {
        id: scheduleControl
        Picker {
            id: schedulePicker
            readonly property var row: Schema.row("night-schedule")
            readonly property int at: row.options.findIndex(o => o.value === Changes.value(row.bind))
            width: 220
            options: row.options.map(o => I18n.tr(o.label))
            currentIndex: schedulePicker.at
            overlayParent: page.hub.overlayItem
            Accessible.name: I18n.tr("Turn on by itself")
            onActivated: index => {
                Changes.set(row.bind, row.options[index].value, row.label, page.pageId);
                schedulePicker.currentIndex = Qt.binding(() => schedulePicker.at);
            }
        }
    }

    Component {
        id: startControl
        Stepper {
            readonly property var row: Schema.row("night-start")
            from: 0
            to: 1425
            stepSize: 15
            wrap: true
            value: Number(Changes.value(row.bind)) || 0
            textFromValue: (v, locale) => Model.clockText(v, page.clock24)
            accessibleName: I18n.tr("Starts at")
            onValueModified: Changes.set(row.bind, value, row.label, page.pageId)
        }
    }

    Component {
        id: endControl
        Stepper {
            readonly property var row: Schema.row("night-end")
            from: 0
            to: 1425
            stepSize: 15
            wrap: true
            value: Number(Changes.value(row.bind)) || 0
            textFromValue: (v, locale) => Model.clockText(v, page.clock24)
            accessibleName: I18n.tr("Ends at")
            onValueModified: Changes.set(row.bind, value, row.label, page.pageId)
        }
    }

    Component {
        id: marginControl
        Stepper {
            readonly property var row: Schema.row("night-margin")
            from: 0
            to: 180
            stepSize: 15
            unit: I18n.tr("min")
            value: Number(Changes.value(row.bind)) || 0
            accessibleName: I18n.tr("Wait after sunset")
            onValueModified: Changes.set(row.bind, value, row.label, page.pageId)
        }
    }

    // a setting row titled from the schema, found by search
    component SpecRow: SettingRow {
        id: specRow
        required property Item host
        property string rowId: ""
        readonly property var spec: Schema.row(specRow.rowId)
        width: parent ? parent.width : 0
        title: specRow.spec ? I18n.tr(specRow.spec.label) : ""
        detail: specRow.spec && specRow.spec.detail ? I18n.tr(specRow.spec.detail) : ""
        highlighted: specRow.host.highlightId === specRow.rowId
        Component.onCompleted: specRow.host.rowItems[specRow.rowId] = specRow
        Component.onDestruction: {
            if (specRow.host.rowItems[specRow.rowId] === specRow)
                delete specRow.host.rowItems[specRow.rowId];
        }
    }

    // a schema row written through Changes, with its changed mark, reset and error
    component BoundRow: Column {
        id: bound
        required property Item host
        property string rowId: ""
        property Component custom: null
        property bool first: false
        property string unavailable: ""
        readonly property var spec: Schema.row(bound.rowId)
        width: parent ? parent.width : 0
        spacing: LithicTokens.space.s1
        Component.onCompleted: bound.host.rowItems[bound.rowId] = bound
        Component.onDestruction: {
            if (bound.host.rowItems[bound.rowId] === bound)
                delete bound.host.rowItems[bound.rowId];
        }

        SettingRow {
            width: parent.width
            first: bound.first
            title: I18n.tr(bound.spec.label)
            detail: bound.spec.detail ? I18n.tr(bound.spec.detail) : ""
            unavailable: bound.unavailable
            changed: Changes.isChanged(bound.spec)
            highlighted: bound.host.highlightId === bound.rowId
            activatable: bound.spec.control === "toggle"
            onActivated: Changes.set(bound.spec.bind, Changes.value(bound.spec.bind) !== true, bound.spec.label, "displays")
            onResetRequested: Changes.set(bound.spec.bind, Changes.defaultOf(bound.spec), bound.spec.label, "displays")

            Loader {
                sourceComponent: bound.custom ? bound.custom : standard
            }

            Component {
                id: standard
                RowControl {
                    row: bound.spec
                    pageId: "displays"
                    overlayParent: bound.host.hub.overlayItem
                }
            }
        }

        ErrorLine {
            bind: bound.spec.bind
        }
    }

    component ErrorLine: Row {
        id: errorLine
        property var bind: null
        readonly property string error: Changes.error(errorLine.bind)
        visible: errorLine.error.length > 0
        width: parent ? parent.width : 0
        spacing: LithicTokens.space.s3

        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, parent.width - retry.width - LithicTokens.space.s3)
            text: errorLine.error
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
            onClicked: Changes.retry(errorLine.bind)
        }
    }
}

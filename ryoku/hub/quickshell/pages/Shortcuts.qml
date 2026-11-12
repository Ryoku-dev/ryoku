import QtQuick
import Quickshell
import Quickshell.Wayland
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/shortcuts"
import "../components/shortcuts/binds.js" as Binds
import "../schema/shortcuts.js" as Schema
import "../registry/registry.js" as Registry

// every keyboard shortcut by group, the user's own, and the modifier key
Item {
    id: page

    required property var hub
    readonly property string pageId: "shortcuts"

    property var categories: []
    property bool legendLoaded: false
    property string query: ""
    property Item recordingField: null
    property int recordCustom: -1
    // a shortcut added but not saved yet: it shows after the saved ones until a write holds it
    property bool draftOpen: false
    property int draftBase: 0

    property var layout: []
    property string layoutSignature: ""
    property string highlightId: ""
    property int currentSection: 0
    property bool indexHeld: false
    // row id -> item, kept in place so lookups never rebind
    property var rowItems: ({})
    property var sectionItems: ({})

    readonly property var rebindsBind: ({ store: "desktop", key: "keybindRebinds" })
    readonly property var offBind: ({ store: "desktop", key: "keybindDisabled" })
    readonly property var customBind: ({ store: "desktop", key: "keybinds" })

    readonly property bool ready: Changes.desktopReady && Settings.loaded && ProviderSchema.ready && page.legendLoaded
    readonly property var rebinds: Changes.value(page.rebindsBind) || ({})
    readonly property var disabled: Changes.value(page.offBind) || []
    readonly property var custom: Changes.value(page.customBind) || []
    readonly property var binds: {
        var out = [];
        for (var c = 0; c < page.categories.length; c++)
            out = out.concat((page.categories[c].binds || []).filter(b => b.kind !== "custom"));
        return out;
    }
    readonly property var holders: Binds.holders(page.binds, page.rebinds, page.disabled, page.custom)
    readonly property var modifierRows: {
        ProviderSchema.revision;
        return page.hub.providerRowsFor(page.pageId).filter(r => r.section === "modifier" && page.hub.rowAvailable(r));
    }
    readonly property var actionOptions: [
        { key: "exec", label: "Run a command" },
        { key: "close", label: "Close window" },
        { key: "fullscreen", label: "Full screen" },
        { key: "togglefloating", label: "Float or tile" }
    ]
    // faces for the store tokens ChordField has none for
    readonly property var faces: ({
        "{n}": I18n.tr("1 … 0"),
        "KP_{n}": I18n.tr("Num 1 … 0"),
        "mouse_up": I18n.tr("Scroll up"),
        "mouse_down": I18n.tr("Scroll down"),
        "mouse:272": I18n.tr("Left button"),
        "mouse:273": I18n.tr("Right button"),
        "XF86TouchpadToggle": I18n.tr("Touchpad"),
        "XF86TouchpadOn": I18n.tr("Touchpad on"),
        "XF86TouchpadOff": I18n.tr("Touchpad off")
    })
    readonly property real offWidth: Math.max(offMeasure.implicitWidth, onMeasure.implicitWidth)
    readonly property var errors: [page.rebindsBind, page.offBind, page.customBind]
        .filter(b => Changes.error(b).length > 0)

    readonly property bool sideIndex: page.layout.length >= 4
        && page.width >= LithicTokens.size.settingsExpandedMax + LithicTokens.size.sectionIndexWidth + LithicTokens.size.sectionIndexGap
    readonly property real columnWidth: page.sideIndex
        ? Math.min(page.width - LithicTokens.size.sectionIndexWidth - LithicTokens.size.sectionIndexGap, LithicTokens.size.settingsExpandedMax)
        : Math.min(page.width, page.hub.sizeClass === "expanded" ? LithicTokens.size.settingsExpandedMax : LithicTokens.size.settingsMediumMax)
    readonly property real groupX: page.sideIndex
        ? Math.max(0, (page.width - page.columnWidth - LithicTokens.size.sectionIndexWidth - LithicTokens.size.sectionIndexGap) / 2)
        : Math.max(0, (page.width - page.columnWidth) / 2)
    readonly property real columnX: page.sideIndex
        ? page.groupX + LithicTokens.size.sectionIndexWidth + LithicTokens.size.sectionIndexGap
        : page.groupX

    function shown(chord) {
        return Binds.shown(chord, page.faces);
    }

    function holderName(h) {
        return h.kind === "bind" ? I18n.tr(h.label) : I18n.tr("one of your own shortcuts");
    }

    function actionLabel(key) {
        var o = page.actionOptions.find(o => o.key === key);
        return I18n.tr(o ? o.label : "Run a command");
    }

    // why the window manager cannot run a custom shortcut, from the saved legend
    function customReason(index) {
        var entry = page.custom[index] || {};
        for (var c = 0; c < page.categories.length; c++) {
            var row = (page.categories[c].binds || []).find(b => b.id === "custom." + index);
            if (row && row.unhonored && Binds.norm(row.chord) === Binds.norm(entry.keys || ""))
                return I18n.tr(row.unhonored);
        }
        return "";
    }

    function bindLabel(bind) {
        return I18n.tr("%1 shortcut").arg(I18n.tr(bind.label));
    }

    function rebind(bind, chord) {
        Changes.set(page.rebindsBind, Binds.withRebind(page.rebinds, bind.combo, chord), page.bindLabel(bind), page.pageId);
    }

    function setOff(bind, off) {
        Changes.set(page.offBind, Binds.withOff(page.disabled, bind.combo, off), page.bindLabel(bind), page.pageId);
    }

    function resetBind(bind) {
        if (Binds.isRebound(page.rebinds, bind.combo))
            page.rebind(bind, "");
        if (Binds.isOff(page.disabled, bind.combo))
            page.setOff(bind, false);
    }

    function writeCustom(list, notice) {
        Changes.set(page.customBind, list, I18n.tr("Your own shortcuts"), page.pageId, notice || "");
    }

    // a shortcut just added shows at the end of the list, and is saved with its first keys, action
    // or command, so adding it and recording its keys is one change with one Undo
    function patchCustom(index, field, value) {
        var list = page.custom.slice();
        var adding = page.draftShown() && index === list.length;
        var blank = { keys: "", action: "exec", value: "", release: false };
        if (adding && value === blank[field])
            return;
        list[index] = Object.assign({}, adding ? blank : list[index]);
        list[index][field] = value;
        if (field === "action" && value !== "exec")
            list[index].value = "";
        page.writeCustom(list, adding ? I18n.tr("Shortcut added") : "");
        if (adding)
            page.draftOpen = false;
    }

    // read from the saved list itself, so a rebuild during the write that saves it keeps one row
    function draftShown() {
        return page.draftOpen && page.custom.length <= page.draftBase;
    }

    function addCustom() {
        filter.clear();
        page.query = "";
        if (page.draftShown()) {
            var open = page.rowItems["custom." + page.custom.length];
            if (open) {
                page.land(open);
                open.chordField.startRecording();
            }
            return;
        }
        page.recordCustom = page.custom.length;
        page.draftBase = page.custom.length;
        page.draftOpen = true;
        page.rebuild();
    }

    function removeCustom(index) {
        if (page.draftShown() && index === page.custom.length) {
            page.draftOpen = false;
            page.rebuild();
            return;
        }
        var list = page.custom.slice();
        list.splice(index, 1);
        page.writeCustom(list, I18n.tr("Shortcut removed"));
    }

    // a field records: hold the window manager's shortcuts so they reach it
    function noteRecording(field, on) {
        if (on) {
            if (page.recordingField === null)
                page.enterRecordMode();
            page.recordingField = field;
            recordTimeout.restart();
        } else if (page.recordingField === field) {
            page.recordingField = null;
            recordTimeout.stop();
            page.exitRecordMode();
        }
    }

    // a do-nothing submap backs the inhibitor up where the window manager has one
    function enterRecordMode() {
        if (Settings.supports("submap"))
            Settings.send("wm.act", { action: "submap.enter", args: ["record"] });
    }

    function exitRecordMode() {
        if (Settings.supports("submap"))
            Settings.send("wm.act", { action: "submap.reset", args: [] });
    }

    function sectionFor(name) {
        var s = Schema.sections.find(s => s.group === name);
        return s ? { id: s.id, title: s.title } : { id: String(name).toLowerCase().replace(/[^a-z0-9]+/g, "-"), title: name };
    }

    function bindText(bind, title) {
        return [I18n.tr(bind.label), bind.label, bind.hint ? I18n.tr(bind.hint) : "", bind.hint || "",
                title, I18n.tr(title), (bind.keys || []).join(" "),
                Binds.toField(Binds.effective(page.rebinds, bind.combo)).split("+").join(" "),
                Binds.familyWords(bind.combo)].join(" ");
    }

    function customText(entry) {
        return [entry.value || "", page.actionLabel(entry.action || "exec"), Binds.toField(entry.keys || "").split("+").join(" ")].join(" ");
    }

    function rebuild() {
        if (!page.ready)
            return;
        var out = [], sig = [];
        var q = page.query;
        for (var c = 0; c < page.categories.length; c++) {
            var cat = page.categories[c];
            var all = (cat.binds || []).filter(b => b.kind !== "custom");
            if (all.length === 0)
                continue;
            var sec = page.sectionFor(cat.name);
            var here = all.filter(b => !b.unhonored);
            var missing = all.filter(b => b.unhonored);
            var shown = here.filter(b => Binds.matches(q, page.bindText(b, sec.title)));
            var gone = missing.filter(b => Binds.matches(q, page.bindText(b, sec.title)));
            if (shown.length === 0 && (q.length > 0 || gone.length === 0))
                continue;
            // the unavailable ones, gathered under the reason the window manager gives
            var reasons = [];
            gone.forEach(b => {
                var why = I18n.tr(b.unhonored);
                var known = reasons.find(r => r.why === why);
                if (known)
                    known.names.push(I18n.tr(b.label));
                else
                    reasons.push({ why: why, names: [I18n.tr(b.label)] });
            });
            var note = reasons.map(r => I18n.tr("Not available: %1. %2").arg(r.names.join(", ")).arg(r.why)).join("\n");
            out.push({ id: sec.id, title: sec.title, kind: "group", binds: shown, missing: note });
            sig.push(sec.id + ":" + shown.map(b => b.id).join(",") + "|" + note);
        }
        var customRows = [];
        for (var i = 0; i < page.custom.length; i++)
            if (Binds.matches(q, page.customText(page.custom[i])))
                customRows.push(i);
        if (page.draftShown() && q.length === 0)
            customRows.push(page.custom.length);
        if (q.length === 0 || customRows.length > 0) {
            out.push({ id: "custom", title: "Your own shortcuts", kind: "custom", indices: customRows, missing: "" });
            sig.push("custom:" + customRows.join(","));
        }
        var mods = page.modifierRows.filter(r => Binds.matches(q, [r.label, I18n.tr(r.label), r.detail || ""].join(" ")));
        if (mods.length > 0) {
            out.push({ id: "modifier", title: "Modifier key", kind: "modifier", rows: mods, missing: "" });
            sig.push("modifier:" + mods.map(r => r.id).join(","));
        }
        var signature = sig.join(";");
        if (signature !== page.layoutSignature) {
            page.layoutSignature = signature;
            page.layout = out;
        }
    }

    onReadyChanged: page.rebuild()
    onQueryChanged: page.rebuild()
    onCategoriesChanged: page.rebuild()
    onModifierRowsChanged: page.rebuild()
    onCustomChanged: page.rebuild()

    // the window manager's list of shortcuts; a newer read replaces one still running, and a read
    // that fails or hangs ends in plain words with Retry, keeping the list it had
    property string legendError: ""
    property int legendGen: 0
    function loadLegend() {
        var gen = ++page.legendGen;
        page.legendError = "";
        Changes.run(["ryoku-hub", "keybinds"], (ok, out, err, code, late) => {
            if (gen !== page.legendGen)
                return;
            var parsed = null;
            try {
                parsed = JSON.parse(out);
            } catch (e) {}
            page.legendLoaded = true;
            if (!ok || !parsed || !Array.isArray(parsed.categories)) {
                page.legendError = page.readError(err, code, late);
                return;
            }
            page.categories = parsed.categories;
        }, { timeout: 15000 });
    }

    function readError(err, code, late) {
        var lead = I18n.tr("The list of shortcuts could not be read.");
        if (late)
            return lead + " " + I18n.tr("It did not answer in time. Retry, and if it fails again, update Ryoku.");
        // Changes.run already put a missing program in plain words
        if (code === -1)
            return lead + " " + String(err || "").trim();
        var lines = String(err || "").trim().split("\n").filter(l => l.trim().length > 0);
        var why = lines.length > 0 ? lines[lines.length - 1].trim() : "";
        return lead + " " + I18n.tr("Retry, and if it fails again, update Ryoku.") + (why ? " (" + why + ")" : "");
    }

    Component.onCompleted: page.loadLegend()

    // the saved legend names custom shortcuts the window manager cannot run
    Connections {
        target: Changes
        function onDesktopCommittedChanged() {
            page.loadLegend();
        }
    }

    // never leave the keyboard held: a ceiling ends any recording
    Timer {
        id: recordTimeout
        interval: 15000
        onTriggered: {
            if (page.recordingField)
                page.recordingField.cancelRecording();
        }
    }

    ShortcutInhibitor {
        window: page.QsWindow ? page.QsWindow.window : null
        enabled: page.recordingField !== null
    }

    function sectionOfRow(rowId) {
        // a section link names the section itself
        if (page.layout.some(s => s.id === rowId))
            return rowId;
        var schemaRow = Schema.rows.find(r => r.id === rowId);
        if (schemaRow)
            return schemaRow.section;
        if (rowId.indexOf("custom.") === 0)
            return "custom";
        if (page.modifierRows.some(r => r.id === rowId))
            return "modifier";
        var bind = page.binds.find(b => b.id === rowId);
        if (!bind)
            return "";
        var cat = page.categories.find(c => (c.binds || []).indexOf(bind) >= 0);
        return cat ? page.sectionFor(cat.name).id : "";
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (!page.ready || page.layout.length === 0) {
            pendingFocus.rowId = rowId;
            return;
        }
        if (page.query.length > 0 && !page.rowItems[rowId] && !page.sectionItems[rowId]) {
            filter.clear();
            page.query = "";
            Qt.callLater(page.focusRow, rowId);
            return;
        }
        var target = page.rowItems[rowId];
        if (!target) {
            var sec = page.sectionOfRow(rowId);
            var secItem = page.sectionItems[sec];
            if (!secItem)
                return;
            target = rowId === "add-shortcut" ? page.rowItems["add-shortcut"] : secItem.firstRow();
            if (!target) {
                page.chooseSection(page.layout.findIndex(s => s.id === sec));
                return;
            }
        }
        page.land(target);
    }

    function land(item) {
        var y = item.mapToItem(flick.contentItem, 0, 0).y;
        page.indexHeld = false;
        page.highlightId = "";
        scrollAnim.stop();
        scrollAnim.landing = item;
        page.scrollTo(y - flick.height / 3);
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
        page.scrollTo(item.mapToItem(flick.contentItem, 0, 0).y);
    }

    // the index follows the reading position
    function followReading() {
        if (page.indexHeld || page.layout.length === 0)
            return;
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

    QtObject {
        id: pendingFocus
        property string rowId: ""
    }
    onLayoutChanged: {
        if (pendingFocus.rowId !== "") {
            var id = pendingFocus.rowId;
            pendingFocus.rowId = "";
            Qt.callLater(page.focusRow, id);
        }
    }

    Timer {
        id: highlightClear
        interval: Lithic.reduceMotion ? LithicTokens.motion.placardLife : LithicTokens.motion.highlight + 100
        onTriggered: page.highlightId = ""
    }

    NumberAnimation {
        id: scrollAnim
        property Item landing: null
        target: flick
        property: "contentY"
        duration: Lithic.duration(LithicTokens.motion.navigate)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Lithic.curve
        onStopped: {
            var landed = scrollAnim.landing;
            if (!landed)
                return;
            scrollAnim.landing = null;
            page.highlightId = landed.rowId;
            if (landed.focusTarget)
                landed.focusTarget.forceActiveFocus(Qt.OtherFocusReason);
            highlightClear.restart();
        }
    }

    // widths of the Turn off plaque in both states, so the field never shifts
    Plaque {
        id: offMeasure
        visible: false
        small: true
        text: I18n.tr("Turn off")
    }
    Plaque {
        id: onMeasure
        visible: false
        small: true
        text: I18n.tr("Turn on")
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
            Item { width: 1; height: LithicTokens.space.s5 }

            Flow {
                width: parent.width
                spacing: LithicTokens.space.s3

                Search {
                    id: filter
                    width: Math.min(body.width, LithicTokens.size.settingsControlMax)
                    placeholderText: I18n.tr("Filter shortcuts")
                    accessibleName: I18n.tr("Filter shortcuts")
                    enabled: page.ready
                    onTextChanged: page.query = text.trim()
                }

                SectionIndex {
                    visible: !page.sideIndex && page.layout.length >= 4
                    width: Math.min(body.width, LithicTokens.size.settingsControlMax)
                    sizeClass: "medium"
                    sections: page.layout.map(s => I18n.tr(s.title))
                    currentIndex: page.currentSection
                    overlayParent: page.hub.overlayItem
                    onActivated: index => page.chooseSection(index)
                }
            }
            Item { width: 1; height: LithicTokens.space.s5 }

            Repeater {
                model: page.errors
                delegate: Row {
                    required property var modelData
                    width: body.width
                    spacing: LithicTokens.space.s3
                    bottomPadding: LithicTokens.space.s4

                    Label {
                        anchors.verticalCenter: parent.verticalCenter
                        width: Math.min(implicitWidth, parent.width - retry.width - LithicTokens.space.s3)
                        text: Changes.error(parent.modelData)
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
                        onClicked: Changes.retry(parent.modelData)
                    }
                }
            }

            // the first read: say so, with a working mark once it takes a moment
            Row {
                visible: !page.legendLoaded
                spacing: LithicTokens.space.s2
                bottomPadding: LithicTokens.space.s5
                Working {
                    anchors.verticalCenter: parent.verticalCenter
                    active: !page.legendLoaded && legendWait.elapsed
                    visible: active
                }
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Reading the shortcuts")
                    typeRole: "detail"
                    inkRole: "fg2"
                }
                Timer {
                    id: legendWait
                    property bool elapsed: false
                    interval: 1000
                    running: !page.legendLoaded
                    onTriggered: elapsed = true
                }
            }

            Label {
                visible: page.ready && page.query.length > 0 && page.layout.length === 0
                width: parent.width
                bottomPadding: LithicTokens.space.s5
                text: I18n.tr("No shortcuts match \u201c%1\u201d").arg(page.query)
                typeRole: "body"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }

            Label {
                visible: page.legendLoaded && page.legendError === "" && page.binds.length === 0
                width: parent.width
                bottomPadding: LithicTokens.space.s5
                text: I18n.tr("The window manager did not list its shortcuts, so they cannot be changed here right now.")
                typeRole: "body"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }

            Row {
                visible: page.legendError !== ""
                width: parent.width
                spacing: LithicTokens.space.s3
                bottomPadding: LithicTokens.space.s5

                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(implicitWidth, parent.width - legendRetry.width - LithicTokens.space.s3)
                    text: page.legendError
                    typeRole: "detail"
                    ink: Lithic.jasper
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Plaque {
                    id: legendRetry
                    kind: "quiet"
                    small: true
                    glyph: "refresh"
                    text: I18n.tr("Retry")
                    onClicked: page.loadLegend()
                }
            }

            Repeater {
                model: page.layout
                delegate: Section {
                    id: sec
                    required property var modelData
                    width: body.width
                    title: I18n.tr(modelData.title)
                    Component.onCompleted: page.sectionItems[modelData.id] = sec
                    Component.onDestruction: {
                        if (page.sectionItems[modelData.id] === sec)
                            delete page.sectionItems[modelData.id];
                    }

                    function firstRow() {
                        var m = sec.modelData;
                        if (m.kind === "group")
                            return m.binds.length > 0 ? page.rowItems[m.binds[0].id] || null : null;
                        if (m.kind === "custom")
                            return m.indices.length > 0 ? page.rowItems["custom." + m.indices[0]] || null : page.rowItems["add-shortcut"] || null;
                        return m.rows.length > 0 ? page.rowItems[m.rows[0].id] || null : null;
                    }

                    Column {
                        width: parent.width
                        spacing: LithicTokens.space.s2

                        Label {
                            visible: sec.modelData.kind === "custom"
                            width: parent.width
                            text: I18n.tr("Keys you add here run a command or act on the focused window. Shortcuts written by hand in the window manager's own files are not listed.")
                            typeRole: "detail"
                            inkRole: "fg2"
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }

                        Slab {
                            width: parent.width
                            visible: sec.modelData.kind !== "group" || sec.modelData.binds.length > 0

                            Column {
                                width: parent.width

                                Repeater {
                                    model: sec.modelData.kind === "group" ? sec.modelData.binds : []
                                    delegate: BindRow {
                                        id: bindRow
                                        required property var modelData
                                        required property int index
                                        readonly property string rowId: modelData.id
                                        width: parent ? parent.width : 0
                                        bind: modelData
                                        host: page
                                        first: index === 0
                                        Component.onCompleted: page.rowItems[bindRow.rowId] = bindRow
                                        Component.onDestruction: {
                                            if (page.rowItems[bindRow.rowId] === bindRow)
                                                delete page.rowItems[bindRow.rowId];
                                        }
                                    }
                                }

                                Repeater {
                                    model: sec.modelData.kind === "custom" ? sec.modelData.indices : []
                                    delegate: CustomRow {
                                        id: customRow
                                        required property var modelData
                                        required property int index
                                        width: parent ? parent.width : 0
                                        customIndex: modelData
                                        host: page
                                        first: index === 0
                                        bottomPadding: LithicTokens.space.s1
                                        Component.onCompleted: {
                                            page.rowItems[customRow.rowId] = customRow;
                                            if (page.recordCustom === customRow.customIndex) {
                                                page.recordCustom = -1;
                                                Qt.callLater(() => {
                                                    page.land(customRow);
                                                    customRow.chordField.startRecording();
                                                });
                                            }
                                        }
                                        Component.onDestruction: {
                                            if (page.rowItems[customRow.rowId] === customRow)
                                                delete page.rowItems[customRow.rowId];
                                        }
                                    }
                                }

                                Label {
                                    visible: sec.modelData.kind === "custom" && page.custom.length === 0 && !page.draftOpen
                                    width: parent.width
                                    text: I18n.tr("You have no shortcuts of your own yet.")
                                    typeRole: "body"
                                    inkRole: "fg2"
                                    wrapMode: Text.Wrap
                                    elide: Text.ElideNone
                                }

                                Item {
                                    visible: sec.modelData.kind === "custom"
                                    width: 1
                                    height: LithicTokens.size.settingsRowGap
                                }

                                Plaque {
                                    id: addPlaque
                                    readonly property string rowId: "add-shortcut"
                                    readonly property Item focusTarget: addPlaque
                                    visible: sec.modelData.kind === "custom"
                                    kind: "tonal"
                                    small: true
                                    glyph: "add"
                                    text: I18n.tr("Add a shortcut")
                                    enabled: page.ready
                                    onClicked: page.addCustom()
                                    Component.onCompleted: {
                                        if (sec.modelData.kind === "custom")
                                            page.rowItems[addPlaque.rowId] = addPlaque;
                                    }
                                    Component.onDestruction: {
                                        if (page.rowItems[addPlaque.rowId] === addPlaque)
                                            delete page.rowItems[addPlaque.rowId];
                                    }
                                }

                                Repeater {
                                    model: sec.modelData.kind === "modifier" ? sec.modelData.rows : []
                                    delegate: Column {
                                        id: modRow
                                        required property var modelData
                                        required property int index
                                        readonly property string rowId: modelData.id
                                        readonly property Item focusTarget: null
                                        readonly property string error: Changes.error(modRow.modelData.bind)
                                        width: parent ? parent.width : 0
                                        spacing: LithicTokens.space.s1
                                        Component.onCompleted: page.rowItems[modRow.rowId] = modRow
                                        Component.onDestruction: {
                                            if (page.rowItems[modRow.rowId] === modRow)
                                                delete page.rowItems[modRow.rowId];
                                        }

                                        SettingRow {
                                            width: parent.width
                                            title: I18n.tr(modRow.modelData.label)
                                            detail: modRow.modelData.detail ? I18n.tr(modRow.modelData.detail) : ""
                                            changed: Changes.isChanged(modRow.modelData)
                                            first: modRow.index === 0
                                            highlighted: page.highlightId === modRow.rowId
                                            activatable: modRow.modelData.control === "toggle"
                                            onActivated: modControl.activate()
                                            onResetRequested: Changes.set(modRow.modelData.bind, Changes.defaultOf(modRow.modelData), modRow.modelData.label, page.pageId)

                                            RowControl {
                                                id: modControl
                                                row: modRow.modelData
                                                pageId: page.pageId
                                                overlayParent: page.hub.overlayItem
                                            }
                                        }

                                        Row {
                                            visible: modRow.error.length > 0
                                            width: parent.width
                                            spacing: LithicTokens.space.s3

                                            Label {
                                                anchors.verticalCenter: parent.verticalCenter
                                                width: Math.min(implicitWidth, parent.width - modRetry.width - LithicTokens.space.s3)
                                                text: modRow.error
                                                typeRole: "detail"
                                                ink: Lithic.jasper
                                                wrapMode: Text.Wrap
                                                elide: Text.ElideNone
                                            }
                                            Plaque {
                                                id: modRetry
                                                kind: "quiet"
                                                small: true
                                                glyph: "refresh"
                                                text: I18n.tr("Retry")
                                                onClicked: Changes.retry(modRow.modelData.bind)
                                            }
                                        }
                                    }
                                }
                            }
                        }

                        Label {
                            width: parent.width
                            visible: sec.modelData.missing.length > 0
                            text: sec.modelData.missing
                            typeRole: "detail"
                            inkRole: "fg3"
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }
                    }
                }
            }

            Section {
                visible: page.query.length === 0 && page.hub.pageAvailable("default-apps")
                width: body.width
                title: I18n.tr("Related")

                Plaque {
                    kind: "tonal"
                    small: true
                    glyph: "arrow_forward"
                    text: I18n.tr("Change which apps your shortcuts open")
                    onClicked: page.hub.navigate("default-apps", "")
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
        sections: page.layout.map(s => I18n.tr(s.title))
        currentIndex: page.currentSection
        onActivated: index => page.chooseSection(index)
    }

    ScrollRail {
        flickable: flick
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.right: parent.right
    }
}

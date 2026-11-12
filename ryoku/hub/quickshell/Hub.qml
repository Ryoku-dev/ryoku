import QtQuick
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import Ryoku.Text
import "Singletons"
import "components"
import "registry/registry.js" as Registry
import "registry/summaries.js" as Summaries
import "registry/provider-map.js" as ProviderMap
import "schema/index.js" as Schemas

// the Settings frame: a rail of categories, the category's pages, the page
FocusScope {
    id: hub

    property string categoryId: "appearance"
    property string pageId: ""
    // compact drills down: categories, then pages, then the work pane
    property string compactLevel: "categories"
    // "home" lists every category; "category" lists one category's pages
    property string listMode: "home"
    readonly property bool homeList: frame.compact ? hub.compactLevel === "categories" : hub.listMode === "home"
    // compact has no rail, so its category list starts with Home
    readonly property int homeOffset: frame.compact ? 1 : 0
    property string query: ""
    property int listCursor: -1
    property string lastSection: ""
    property string usersProfileId: ""
    property bool quitting: false
    property string loadedKey: ""

    readonly property string sizeClass: frame.sizeClass
    readonly property Item overlayItem: overlayLayer
    readonly property var changes: Changes
    readonly property bool searching: hub.query.trim().length > 0

    readonly property var gate: ({
        known: Settings.loaded,
        provider: Settings.provider,
        supports: function (cap) { return Settings.supports(cap); }
    })
    readonly property var category: Registry.category(hub.categoryId)
    readonly property var categoryPages: {
        Settings.revision;
        return Registry.pagesOf(hub.categoryId, hub.gate);
    }

    // provider rows placed in the new pages, refreshed with the provider schema
    readonly property var providerPlaced: {
        ProviderSchema.revision;
        var mapped = ProviderMap.mapRows(ProviderSchema.rows);
        if (mapped.unknown.length > 0 && Quickshell.env("RYOKU_HUB_DEV") === "1")
            console.warn("hub: provider rows with no page: " + mapped.unknown.map(r => r.key).join(", "));
        return mapped.pages;
    }

    function providerRowsFor(id) {
        return hub.providerPlaced[id] || [];
    }

    function pageAvailable(id) {
        return Registry.available(id, hub.gate);
    }

    function rowAvailable(row) {
        var n = row.needs || {};
        if (!Settings.supports(n.cap, n.hostCap))
            return false;
        if (row.bind && row.bind.store === "desktop") {
            if (!Settings.modelsKey(Changes.storePath(row.bind.key)))
                return false;
            if (Changes.desktopReady && Changes.value(row.bind) === undefined)
                return false;
        }
        return true;
    }

    function summaryContext() {
        Changes.revision;
        Settings.revision;
        return {
            tr: s => I18n.tr(s),
            settings: k => Settings.get(k),
            desktop: k => Changes.value({ store: "desktop", key: k }),
            desktopReady: Changes.desktopReady,
            provider: Settings.provider,
            barStyle: Settings.get("barStyle") || "",
            updateReady: Updates.available,
            updateKnown: Updates.lastChecked !== null,
            monitors: SysInfo.sysMonitors,
            battery: SysInfo.sysBattery
        };
    }
    readonly property var summaries: hub.summaryContext()

    // navigation

    function setCategory(id) {
        hub.categoryId = id;
        hub.listMode = "category";
        frame.currentDestination = Math.max(0, Registry.categoryIndex(id));
        hub.listCursor = -1;
    }

    // the list pane lists every category and the rail selects nothing
    function showHomeList() {
        hub.listMode = "home";
        frame.currentDestination = -1;
        hub.listCursor = -1;
    }

    // a page id, alias or "page/row"; unknown lands on Home
    function navigate(target, rowId) {
        var id = Registry.resolve(target);
        var row = rowId || Registry.rowPart(target);
        if (id === "" || !hub.pageAvailable(id)) {
            hub.goHome();
            return;
        }
        var p = Registry.page(id);
        if (p.handoff) {
            Spawn.run(p.handoff);
            return;
        }
        hub.setCategory(p.category);
        hub.compactLevel = "work";
        if (hub.pageId !== id) {
            hub.pageId = id;
            hub.lastSection = id;
            Quickshell.execDetached(["ryoku-hub", "config", "set", "section", id]);
        }
        hub.showContent(row);
    }

    function goHome() {
        hub.pageId = "";
        hub.showHomeList();
        hub.compactLevel = frame.compact ? "work" : "categories";
        hub.showContent("");
    }

    function openCategory(index) {
        var c = Registry.categories[index];
        if (!c)
            return;
        if (c.id === "updates") {
            hub.navigate("updates", "");
            return;
        }
        hub.setCategory(c.id);
        if (hub.sizeClass === "medium" && hub.pageId !== "") {
            hub.pageId = "";
            hub.showContent("");
        }
        hub.compactLevel = "pages";
    }

    function openUsersProfile(profileId) {
        hub.usersProfileId = String(profileId || "");
        hub.navigate("users", "");
    }

    // a confirmation over the whole window: yes() on Confirm, no() on anything
    // else; with options.timeout (seconds) it counts down and answers no by itself
    function confirm(options, yes, no) {
        confirmDialog.title = options.title || "";
        confirmDialog.lead = options.text || "";
        confirmDialog.actionText = options.actionText || I18n.tr("Confirm");
        confirmDialog.cancelText = options.cancelText || I18n.tr("Cancel");
        confirmDialog.destructive = options.destructive !== false;
        confirmDialog.requiredName = options.requiredName || "";
        confirmDialog.yes = yes || null;
        confirmDialog.no = no || null;
        confirmDialog.remaining = options.timeout > 0 ? Math.round(options.timeout) : 0;
        confirmDialog.reset();
        confirmDialog.open();
        if (confirmDialog.remaining > 0)
            countdown.restart();
    }

    // Escape and Alt+Left: results, then the page, then the window
    function back() {
        if (hub.searching) {
            hub.clearSearch();
            return;
        }
        if (frame.compact) {
            if (hub.compactLevel === "work") {
                hub.compactLevel = hub.pageId !== "" ? "pages" : "categories";
                return;
            }
            if (hub.compactLevel === "pages") {
                hub.compactLevel = "categories";
                hub.showHomeList();
                return;
            }
            hub.requestQuit();
            return;
        }
        if (hub.pageId !== "") {
            hub.pageId = "";
            hub.showContent("");
            return;
        }
        if (hub.listMode !== "home") {
            hub.showHomeList();
            return;
        }
        hub.requestQuit();
    }

    function requestQuit() {
        if (hub.quitting)
            return;
        hub.quitting = true;
        Changes.flush(() => Qt.quit());
    }

    // the work pane: Home, a migrated page, or the moving-soon note
    function showContent(rowId) {
        var key = hub.pageId === "" ? "home" : hub.pageId;
        if (key !== hub.loadedKey) {
            hub.loadedKey = key;
            var p = hub.pageId === "" ? null : Registry.page(hub.pageId);
            if (!p)
                content.setSource(Qt.resolvedUrl("components/Home.qml"), { hub: hub });
            else if (p.file)
                content.setSource(Qt.resolvedUrl("pages/" + p.file + ".qml"), { hub: hub });
            else
                content.setSource(Qt.resolvedUrl("components/MovingSoon.qml"), { hub: hub, pageId: hub.pageId });
            rise.restart();
        }
        if (rowId && content.item)
            content.item.focusRow(rowId);
    }

    // search

    function focusSearch() {
        if (frame.compact && hub.compactLevel === "work")
            hub.compactLevel = hub.pageId !== "" ? "pages" : "categories";
        if (frame.sizeClass === "medium" && hub.pageId !== "") {
            hub.pageId = "";
            hub.showContent("");
        }
        search.forceActiveFocus();
        search.selectAll();
    }

    function clearSearch() {
        search.clear();
        hub.query = "";
    }

    readonly property var synonyms: ({
        "transparency": "opacity", "transparent": "opacity", "seethrough": "opacity", "opacity": "transparency",
        "darkmode": "dark", "lightmode": "light", "color": "colour", "colour": "color",
        "hotkey": "shortcut", "hotkeys": "shortcuts", "keybind": "shortcut", "keybinds": "shortcuts",
        "wifi": "wi-fi network", "internet": "network wi-fi", "monitor": "display", "monitors": "displays",
        "resolution": "displays", "rounding": "corners radius", "corners": "rounding", "radius": "corners",
        "frost": "blur", "glass": "blur", "titlebar": "title bars", "titlebars": "title bars",
        "gap": "gaps", "spacing": "gaps", "screenshot": "capture", "recording": "capture",
        "dictation": "voice typing", "speech": "voice typing", "lock": "lock screen", "screensaver": "lock screen",
        "startup": "startup apps", "autostart": "startup", "gpu": "graphics", "battery": "power",
        "font": "fonts", "cursor": "pointer", "outline": "border", "frame": "border"
    })

    function entryCrumb(parts) {
        return parts.filter(p => p && p.length > 0).join(" › ");
    }

    function both(text) {
        var t = I18n.tr(text);
        return t === text ? text : text + " " + t;
    }

    function schemaFor(file) {
        return Schemas.forFile(file);
    }

    readonly property var searchEntries: {
        Settings.revision;
        ProviderSchema.revision;
        Changes.desktopReady;
        var out = [];
        var ids = Registry.allPageIds();
        for (var i = 0; i < ids.length; i++) {
            var id = ids[i];
            if (!hub.pageAvailable(id))
                continue;
            var p = Registry.page(id);
            var cat = Registry.category(p.category);
            var kw = p.keywords.join(" ") + " " + hub.both(cat.label);
            if (id === "window-manager" && Settings.provider)
                kw += " " + Settings.provider;
            out.push({
                kind: p.handoff ? "handoff" : "page", page: id, row: "", command: p.handoff || [],
                display: I18n.tr(p.title), label: hub.both(p.title), desc: "", kw: kw,
                group: "", tab: "", sectionName: hub.both(cat.label), isPage: true,
                groupKey: id, groupTitle: I18n.tr(p.title), crumb: I18n.tr(cat.label)
            });
            var schema = hub.schemaFor(p.file);
            if (!schema)
                continue;
            var titles = {};
            for (var s = 0; s < schema.sections.length; s++)
                titles[schema.sections[s].id] = schema.sections[s].title;
            var rows = schema.rows.concat(hub.providerRowsFor(id));
            for (var r = 0; r < rows.length; r++) {
                var row = rows[r];
                if (!hub.rowAvailable(row))
                    continue;
                var sec = titles[row.section] || "";
                var opts = (row.options || []).map(o => hub.both(o.label)).join(" ");
                out.push({
                    kind: "row", page: id, row: row.id, command: [],
                    display: I18n.tr(row.label), label: hub.both(row.label),
                    desc: row.detail ? hub.both(row.detail) : "",
                    kw: (row.keywords || []).join(" ") + " " + opts,
                    group: hub.both(sec), tab: "", sectionName: hub.both(p.title), isPage: false,
                    groupKey: id, groupTitle: I18n.tr(p.title),
                    crumb: hub.entryCrumb([I18n.tr(cat.label), I18n.tr(p.title), I18n.tr(sec)])
                });
            }
        }
        var wp = Registry.page("wallpaper-colours");
        for (var h = 0; h < Registry.handoffEntries.length; h++) {
            var e = Registry.handoffEntries[h];
            out.push({
                kind: "handoff", page: "wallpaper-colours", row: "", command: e.command,
                display: I18n.tr(e.label), label: hub.both(e.label), desc: "", kw: e.keywords.join(" "),
                group: "", tab: "", sectionName: hub.both(wp.title), isPage: false,
                groupKey: "wallpaper-colours", groupTitle: I18n.tr(wp.title),
                crumb: hub.entryCrumb([I18n.tr("Appearance"), I18n.tr(wp.title)])
            });
        }
        return out;
    }
    onSearchEntriesChanged: searchIndex.setEntries(hub.searchEntries)

    HubSearchIndex {
        id: searchIndex
        Component.onCompleted: {
            setSynonyms(hub.synonyms);
            setEntries(hub.searchEntries);
        }
    }

    readonly property var results: searchIndex.revision >= 0 && hub.searching ? searchIndex.query(hub.query, 60) : []
    readonly property var nearby: {
        if (!hub.searching || hub.results.length > 0)
            return [];
        var seen = {}, out = [];
        var words = hub.query.toLowerCase().split(/\s+/).filter(w => w.length >= 3);
        for (var i = 0; i < words.length && out.length < 3; i++) {
            var hits = searchIndex.query(words[i].slice(0, 3), 20);
            for (var j = 0; j < hits.length && out.length < 3; j++)
                if (hits[j].isPage && !seen[hits[j].page]) {
                    seen[hits[j].page] = true;
                    out.push(hits[j]);
                }
        }
        return out;
    }

    function choose(entry) {
        if (!entry)
            return;
        if (entry.kind === "handoff") {
            Spawn.run(entry.command);
            return;
        }
        hub.navigate(entry.page, entry.row);
    }

    function moveCursor(delta) {
        if (hub.searching) {
            resultsList.move(delta);
            return;
        }
        var n = hub.homeList ? Registry.categories.length + hub.homeOffset : hub.categoryPages.length;
        if (n === 0)
            return;
        hub.listCursor = hub.listCursor < 0 ? (delta > 0 ? 0 : n - 1) : Math.max(0, Math.min(n - 1, hub.listCursor + delta));
        var item = pageRepeater.itemAt(hub.listCursor - (hub.homeList ? hub.homeOffset : 0));
        if (item)
            listFlick.contentY = Math.max(0, Math.min(item.y, Math.max(0, item.y + item.height - listFlick.height), listFlick.contentHeight - listFlick.height));
    }

    function openCursor() {
        if (hub.searching) {
            resultsList.activate();
            return;
        }
        if (hub.listCursor < 0)
            return;
        if (hub.homeList) {
            if (hub.listCursor < hub.homeOffset)
                hub.goHome();
            else
                hub.openCategory(hub.listCursor - hub.homeOffset);
            return;
        }
        var p = hub.categoryPages[hub.listCursor];
        if (p)
            hub.navigate(p.id, "");
    }

    Keys.onPressed: event => {
        var ctrl = (event.modifiers & Qt.ControlModifier) !== 0;
        if (ctrl && (event.key === Qt.Key_K || event.key === Qt.Key_F)) {
            hub.focusSearch();
            event.accepted = true;
        } else if (ctrl && event.key === Qt.Key_Z) {
            Changes.undo();
            event.accepted = true;
        } else if (event.key === Qt.Key_Slash && event.modifiers === Qt.NoModifier) {
            hub.focusSearch();
            event.accepted = true;
        } else if (event.key === Qt.Key_Left && (event.modifiers & Qt.AltModifier)) {
            hub.back();
            event.accepted = true;
        } else if (event.key === Qt.Key_Escape) {
            hub.back();
            event.accepted = true;
        }
    }

    Process {
        command: ["ryoku-hub", "config", "get", "section"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                if (hub.lastSection === "")
                    hub.lastSection = this.text.trim();
            }
        }
    }

    Connections {
        target: Changes
        function onAnnounced(entry) { placards.announce(entry); }
        function onUndone(entry) {
            var key = Changes.keyOf(entry.bind);
            placards.cards.filter(p => p.key === key).forEach(p => p.close());
        }
    }

    Component.onCompleted: {
        hub.showHomeList();
        hub.compactLevel = "categories";
        hub.showContent("");
    }

    AppFrame {
        id: frame
        anchors.fill: parent
        destinations: Registry.categories.map(c => ({ label: I18n.tr(c.label), glyph: c.glyph }))
        showNew: false
        footIndex: Registry.categories.length - 1
        notchIndex: Updates.available ? Registry.categories.length - 1 : -1
        compactBar: false
        brandMark: Component {
            RyokuMark {
                ink: Lithic.onSel
                accent: Lithic.jasper
            }
        }
        showList: frame.compact ? true : (frame.sizeClass === "expanded" || hub.pageId === "")
        compactView: hub.compactLevel === "work" ? "work" : "list"
        windowControlsComponent: closeControl
        onHomeTriggered: hub.goHome()
        onDestinationTriggered: (index, label) => hub.openCategory(index)
        onCloseRequested: hub.requestQuit()

        Item {
            id: workArea
            anchors.fill: parent

            Plaque {
                id: backPlaque
                visible: frame.sizeClass !== "expanded" && (hub.pageId !== "" || frame.compact)
                kind: "quiet"
                small: true
                glyph: "arrow_back"
                text: I18n.tr("Back")
                onClicked: hub.back()
            }

            Item {
                id: stage
                property real lift: 0
                y: (backPlaque.visible ? backPlaque.height + LithicTokens.space.s3 : 0) + stage.lift
                width: parent.width
                height: parent.height - (backPlaque.visible ? backPlaque.height + LithicTokens.space.s3 : 0)

                Loader {
                    id: content
                    anchors.fill: parent
                }
            }

            ParallelAnimation {
                id: rise
                NumberAnimation {
                    target: stage
                    property: "opacity"
                    from: 0
                    to: 1
                    duration: Lithic.duration(LithicTokens.motion.navigate)
                    easing.type: Easing.BezierSpline
                    easing.bezierCurve: Lithic.curve
                }
                NumberAnimation {
                    target: stage
                    property: "lift"
                    from: LithicTokens.size.workPaneRise
                    to: 0
                    duration: Lithic.duration(LithicTokens.motion.navigate)
                    easing.type: Easing.BezierSpline
                    easing.bezierCurve: Lithic.curve
                }
            }
        }

        listData: [
            Item {
                id: listArea
                anchors.fill: parent
                anchors.leftMargin: LithicTokens.size.listInset
                anchors.rightMargin: LithicTokens.size.listInset

                readonly property bool categoryList: hub.homeList

                Column {
                    id: listHead
                    width: parent.width
                    spacing: LithicTokens.space.s2

                    Plaque {
                        visible: frame.compact && hub.compactLevel === "pages"
                        kind: "quiet"
                        small: true
                        glyph: "arrow_back"
                        text: I18n.tr("Back")
                        onClicked: hub.back()
                    }
                    Label {
                        width: parent.width
                        leftPadding: LithicTokens.space.s3
                        text: listArea.categoryList ? I18n.tr("Settings") : (hub.category ? I18n.tr(hub.category.label) : "")
                        typeRole: "section"
                    }
                    Label {
                        width: parent.width
                        visible: !listArea.categoryList && text.length > 0
                        leftPadding: LithicTokens.space.s3
                        rightPadding: LithicTokens.space.s3
                        text: Summaries.categorySentence(hub.categoryId, hub.summaries)
                        typeRole: "body"
                        inkRole: "fg2"
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                    Item { width: 1; height: LithicTokens.space.s1 }
                    Search {
                        id: search
                        width: parent.width
                        placeholderText: I18n.tr("Search settings")
                        shortcutHint: "/"
                        accessibleName: I18n.tr("Search settings")
                        Keys.forwardTo: [escapeGuard]
                        Keys.onUpPressed: hub.moveCursor(-1)
                        Keys.onDownPressed: hub.moveCursor(1)
                        Keys.onReturnPressed: hub.openCursor()
                        Keys.onEnterPressed: hub.openCursor()
                        onTextChanged: hub.query = text
                    }
                    Item { width: 1; height: LithicTokens.space.s2 }
                }

                // an empty search hands Escape on to the frame
                Item {
                    id: escapeGuard
                    Keys.onEscapePressed: event => {
                        if (search.text.length === 0) {
                            hub.back();
                            event.accepted = true;
                        } else {
                            event.accepted = false;
                        }
                    }
                }

                Flickable {
                    id: listFlick
                    visible: !hub.searching
                    anchors.top: listHead.bottom
                    anchors.bottom: parent.bottom
                    width: parent.width
                    contentHeight: pageColumn.height + LithicTokens.space.s4
                    boundsBehavior: Flickable.StopAtBounds
                    clip: true

                    Column {
                        id: pageColumn
                        width: parent.width
                        spacing: LithicTokens.size.rowListGap

                        PageRow {
                            visible: hub.homeOffset > 0 && listArea.categoryList
                            width: parent.width
                            glyph: "home"
                            text: I18n.tr("Home")
                            cursor: hub.listCursor === 0 && listArea.categoryList
                            onClicked: hub.goHome()
                        }

                        Repeater {
                            id: pageRepeater
                            model: listArea.categoryList ? Registry.categories : hub.categoryPages
                            delegate: PageRow {
                                required property var modelData
                                required property int index
                                width: pageColumn.width
                                glyph: listArea.categoryList ? modelData.glyph : ""
                                text: listArea.categoryList ? I18n.tr(modelData.label) : I18n.tr(modelData.title)
                                summary: listArea.categoryList ? Summaries.categorySentence(modelData.id, hub.summaries)
                                                               : Summaries.pageSummary(modelData.id, hub.summaries)
                                trailingGlyph: !listArea.categoryList && modelData.handoff ? "open_in_new" : ""
                                selected: !listArea.categoryList && modelData.id === hub.pageId
                                cursor: hub.listCursor === index + (listArea.categoryList ? hub.homeOffset : 0)
                                onClicked: listArea.categoryList ? hub.openCategory(index) : hub.navigate(modelData.id, "")
                            }
                        }
                    }
                }

                ScrollRail {
                    flickable: listFlick
                    visible: !hub.searching && overflowing
                    anchors.top: listFlick.top
                    anchors.bottom: listFlick.bottom
                    anchors.right: parent.right
                    anchors.rightMargin: -LithicTokens.size.listInset + 2
                }

                SearchResults {
                    id: resultsList
                    visible: hub.searching
                    anchors.top: listHead.bottom
                    anchors.bottom: parent.bottom
                    width: parent.width
                    hub: hub
                    query: hub.query
                    results: hub.results
                    nearby: hub.nearby
                    onChosen: entry => hub.choose(entry)
                }
            }
        ]

        overlayData: [
            Item {
                id: overlayLayer
                anchors.fill: parent

                Confirm {
                    id: confirmDialog
                    property var yes: null
                    property var no: null
                    property string lead: ""
                    property int remaining: 0
                    overlayParent: overlayLayer
                    text: confirmDialog.remaining > 0
                        ? (confirmDialog.lead.length > 0 ? confirmDialog.lead + " " : "")
                          + I18n.tr("Reverting in %1 s.").replace("%1", confirmDialog.remaining)
                        : confirmDialog.lead
                    function answer(accepted) {
                        countdown.stop();
                        var run = accepted ? confirmDialog.yes : confirmDialog.no;
                        confirmDialog.yes = null;
                        confirmDialog.no = null;
                        if (run)
                            run();
                    }
                    onConfirmed: confirmDialog.answer(true)
                    onCancelled: confirmDialog.answer(false)
                    // Escape or the scrim close it without a button
                    onOpenedChanged: {
                        if (!confirmDialog.opened && (confirmDialog.yes || confirmDialog.no))
                            confirmDialog.answer(false);
                    }
                }

                Timer {
                    id: countdown
                    interval: 1000
                    repeat: true
                    onTriggered: {
                        confirmDialog.remaining--;
                        if (confirmDialog.remaining <= 0) {
                            confirmDialog.answer(false);
                            confirmDialog.close();
                        }
                    }
                }

                Item {
                    id: placards
                    anchors.fill: parent
                    property var cards: []

                    // newest at the foot; a repeat of the same setting replaces its notice
                    function announce(entry) {
                        var key = Changes.keyOf(entry.bind);
                        var message = entry.notice ? entry.notice : I18n.tr("%1 changed").replace("%1", I18n.tr(entry.label));
                        if (placards.cards.length > 0 && placards.cards[0].key === key) {
                            placards.cards[0].entry = entry;
                            placards.cards[0].message = message;
                            return;
                        }
                        var stale = placards.cards.filter(p => p.key === key);
                        var keep = [placardComponent.createObject(placards, { entry: entry, key: key, message: message })]
                            .concat(placards.cards.filter(p => p.key !== key));
                        var drop = stale.concat(keep.slice(LithicTokens.size.placardStackCount));
                        placards.cards = keep.slice(0, LithicTokens.size.placardStackCount);
                        for (var i = 0; i < placards.cards.length; i++)
                            placards.cards[i].stackIndex = i;
                        for (var j = 0; j < drop.length; j++)
                            drop[j].close();
                        placards.cards[0].open();
                    }

                    function closed(card) {
                        if (placards.cards.indexOf(card) >= 0) {
                            placards.cards = placards.cards.filter(p => p !== card);
                            for (var i = 0; i < placards.cards.length; i++)
                                placards.cards[i].stackIndex = i;
                        }
                        card.destroy();
                    }
                }
            }
        ]
    }

    Component {
        id: placardComponent
        Placard {
            id: card
            property var entry: null
            property string key: ""
            autoDismiss: true
            actionText: I18n.tr("Undo")
            onActionTriggered: Changes.undoEntry(card.entry)
            onClosed: placards.closed(card)
        }
    }

    Component {
        id: closeControl
        IconPlaque {
            small: true
            kind: "quiet"
            glyph: "close"
            accessibleName: I18n.tr("Close")
            onClicked: hub.requestQuit()
        }
    }
}

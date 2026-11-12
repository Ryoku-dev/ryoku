import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Bluetooth
import Lithic
import Ryoku.Ui.Singletons
import Ryoku.Ui as RyokuUi
import Ryoku.Text
import "Singletons"
import "components"
import "registry/registry.js" as Registry
import "registry/summaries.js" as Summaries
import "registry/provider-map.js" as ProviderMap
import "schema/index.js" as Schemas
import "ShellIndex.js" as ShellIndex

// the Settings frame: a rail of categories, the category's pages, the page
FocusScope {
    id: hub

    property string categoryId: "appearance"
    property string pageId: ""
    // About is the one page that shows the package counts and the rest of the full read-out
    onPageIdChanged: {
        if (hub.pageId === "about")
            SysInfo.load();
    }
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

    // the provider schema is read once a page or search needs it; Home never does
    readonly property bool providerWanted: hub.pageId !== "" || hub.searchArmed

    // provider rows placed in the new pages, refreshed with the provider schema
    readonly property var providerPlaced: {
        if (!hub.providerWanted)
            return ({});
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

    // a window manager setting the window manager reports it cannot keep: its reason, in the
    // provider's words, else "". The row stays choosable so a value it does keep can be picked
    function unhonoredReason(row) {
        if (!row || !row.bind || row.bind.store !== "desktop" || !hub.providerWanted)
            return "";
        ProviderSchema.revision;
        var u = (ProviderSchema.unhonored || []).find(x => x && x.key === "desktop." + row.bind.key);
        return u && u.reason ? String(u.reason) : "";
    }

    function rowAvailable(row) {
        var n = row.needs || {};
        if (!Settings.supports(n.cap, n.hostCap))
            return false;
        if (row.control === "link" && !hub.pageAvailable(Registry.resolve(row.link)))
            return false;
        // a rule property the window manager lists no action for; unknown until the list lands
        if (n.ruleAction && Settings.windowRuleActions.length > 0 && Settings.windowRuleActions.indexOf(n.ruleAction) < 0)
            return false;
        // a look override the window manager neither writes nor lists an action for; unknown until the list lands
        if (n.lookField && Settings.appOverrideFields.length > 0 && Settings.appOverrideFields.indexOf(n.lookField) < 0
                && Settings.windowRuleActions.indexOf(n.orRuleAction) < 0)
            return false;
        if (n.layerRule && hub.layerRuleNames !== null && hub.layerRuleNames.indexOf(n.layerRule) < 0)
            return false;
        // needs.unless: { bind, is: [values] } hides the row while its store reads one of them
        // (a page's own state, such as a profile account); unknown until the store answers
        if (n.unless && n.unless.bind) {
            var said = Changes.value(n.unless.bind);
            if (said !== undefined && (n.unless.is || [true]).some(v => Changes.same(v, said)))
                return false;
        }
        if (row.bind && row.bind.store === "desktop") {
            if (!Settings.modelsKey(Changes.storePath(row.bind.key)))
                return false;
            if (Changes.desktopReady && Changes.value(row.bind) === undefined)
                return false;
        }
        return true;
    }

    // what the provider's layer rule list can set (its fields and action choices), null until known
    readonly property var layerRuleNames: {
        if (!hub.providerWanted)
            return null;
        ProviderSchema.revision;
        if (!ProviderSchema.ready)
            return null;
        var rows = ProviderSchema.rowsFor("layerrules"), out = [];
        rows.filter(r => r.ctl === "list" && String(r.key).indexOf("[") < 0).forEach(list => {
            (list.fields || []).forEach(f => out.push(f.name));
            var action = rows.find(r => r.key === list.key + "[i].action");
            if (action)
                out = out.concat(action.opts || []);
        });
        return out;
    }

    function summaryContext() {
        Changes.revision;
        Settings.revision;
        Live.revision;
        Attention.items;
        return {
            tr: s => I18n.tr(s),
            settings: k => Settings.get(k),
            desktop: k => Changes.value({ store: "desktop", key: k }),
            text: path => Live.text(path),
            textRead: path => Live.textRead(path),
            latest: Updates.latestName || Updates.latestVersion,
            value: bind => Changes.value(bind),
            topic: name => Live.topic(name),
            config: name => Live.config(name),
            attention: id => Attention.items.find(i => i.id === id) || null,
            bluetooth: () => hub.bluetoothState(),
            language: () => hub.languageName(),
            version: Updates.currentName || Updates.currentVersion,
            distro: SysInfo.sysDistro,
            supports: cap => Settings.supports(cap),
            models: key => Settings.modelsKey(key),
            hostSupports: cap => Host.supports(cap),
            desktopReady: Changes.desktopReady,
            provider: Settings.provider,
            barStyle: Settings.get("barStyle") || "",
            updateReady: Updates.available,
            updateKnown: Updates.lastChecked !== null,
            serviceDown: hub.serviceDown,
            // the outputs the compositor drives now, so turning one off shows at once
            monitors: String(Quickshell.screens.length),
            battery: SysInfo.sysBattery
        };
    }

    // { present, on, connected: [names] }, read from BlueZ as it changes
    function bluetoothState() {
        var a = Bluetooth.defaultAdapter;
        if (!a)
            return { present: false, on: false, connected: [] };
        var connected = Bluetooth.devices.values.filter(d => d.connected).map(d => RyokuUi.BtName.label(d));
        return { present: true, on: a.enabled, connected: connected };
    }

    // the chosen language under its own name, or "System language"
    function languageName() {
        var code = I18n._code(Settings.get("language") || "auto");
        if (!code)
            return I18n.tr("System language");
        var l = I18n.langs.find(x => x.code === code);
        return l ? l.native : code;
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

    // a dialog or popover left open over the page closes when Settings goes elsewhere, so keys
    // never change a setting under it (notices stay)
    function closeOverlays() {
        var walk = item => {
            (item.children || []).forEach(child => {
                if (child === placards)
                    return;
                if (child.opened === true && typeof child.close === "function")
                    child.close();
                walk(child);
            });
        };
        walk(overlayLayer);
    }

    // a page id, alias or "page/row"; unknown lands on Home
    function navigate(target, rowId) {
        hub.closeOverlays();
        var id = Registry.resolve(target);
        var row = rowId || Registry.rowPart(target);
        if (id === "" || !hub.pageAvailable(id)) {
            hub.goHome();
            return;
        }
        var p = Registry.page(id);
        if (p.handoff) {
            Spawn.runApp(p.handoff, { label: String(p.handoff[0]).replace(/^.*\//, "") });
            return;
        }
        hub.setCategory(p.category);
        hub.compactLevel = "work";
        var again = hub.pageId === id;
        if (!again) {
            hub.pageId = id;
            hub.lastSection = id;
            Quickshell.execDetached(["ryoku-hub", "config", "set", "section", id]);
        }
        hub.showContent(row);
        // choosing the open page again returns it to its start (a page with sub-views)
        if (again && !row && content.item && typeof content.item.reset === "function")
            content.item.reset();
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

    // a shell word, quoted
    function shq(s) {
        return "'" + String(s).replace(/'/g, "'\\''") + "'";
    }

    // the user's terminal ($TERMINAL, kitty when unset) runs $1 under the window id $2;
    // each terminal names its window and takes a command its own way
    readonly property string terminalScript: "t=\"${TERMINAL:-kitty}\"; id=\"$2\"; case \"${t##*/}\" in "
        + "foot|footclient) exec \"$t\" --app-id=\"$id\" sh -c \"$1\";; "
        + "kitty) exec \"$t\" --class=\"$id\" sh -c \"$1\";; "
        + "wezterm) exec \"$t\" start -- sh -c \"$1\";; "
        + "ghostty|alacritty) exec \"$t\" --class=\"$id\" -e sh -c \"$1\";; "
        + "*) exec \"$t\" -e sh -c \"$1\";; esac"

    // run a shell line in a terminal window, which waits for Enter before it closes.
    // With done, the terminal is Settings' child: done(ok, code) once it closed (ok false when
    // it could not start) and the process is returned so the caller can stop it. Without
    // done it is launched detached, so closing Settings never cuts the work short
    function runInTerminal(line, windowId, done, closeText) {
        var full = line + "; printf '\\n%s\\n' " + hub.shq(closeText || I18n.tr("Press Enter to close this window.")) + "; read -r _";
        var argv = ["sh", "-c", hub.terminalScript, "sh", full, windowId || "dev.ryoku.settings"];
        if (!done) {
            Spawn.runApp(argv, { label: "terminal" });
            return null;
        }
        return Changes.run(argv, function (ok, out, err, code) { done(ok, code); });
    }

    // the one way Settings logs out: it asks first, Cancel focused
    function logOut() {
        hub.confirm({
            title: I18n.tr("Log out now?"),
            text: I18n.tr("Every open app closes, and work you have not saved is lost."),
            actionText: I18n.tr("Log out")
        }, () => Spawn.run(Registry.logOut));
    }

    function openUsersProfile(profileId) {
        hub.usersProfileId = String(profileId || "");
        hub.navigate("users", "");
    }

    // a confirmation over the whole window: yes() on Confirm, no() on anything
    // else; with options.timeout (seconds) it counts down and answers no by itself
    function confirm(options, yes, no) {
        confirmLoader.active = true;
        var d = confirmLoader.item;
        d.title = options.title || "";
        d.lead = options.text || "";
        d.actionText = options.actionText || I18n.tr("Confirm");
        d.cancelText = options.cancelText || I18n.tr("Cancel");
        d.destructive = options.destructive !== false;
        d.requiredName = options.requiredName || "";
        d.yes = yes || null;
        d.no = no || null;
        d.remaining = options.timeout > 0 ? Math.round(options.timeout) : 0;
        d.reset();
        d.open();
        if (d.remaining > 0)
            countdown.restart();
    }

    // how much of the work pane's foot the newest notice covers, so pages keep controls above it
    readonly property real noticeInset: placards.cards.length > 0 && placards.cards[0]
        ? placards.cards[0].height + LithicTokens.space.s4 : 0

    // the desktop service unreachable for a few seconds (a short gap at start is normal)
    property bool serviceDown: false
    Timer {
        interval: 3000
        running: !Bus.connected
        onTriggered: hub.serviceDown = true
    }
    Connections {
        target: Bus
        function onConnectedChanged() {
            if (Bus.connected)
                hub.serviceDown = false;
        }
    }

    // Escape and Alt+Left: results, then the search field, then the page, then the window
    function back() {
        if (hub.searching) {
            hub.clearSearch();
            return;
        }
        // a search just opened and left empty: Escape leaves the field, never the window
        if (search.activeFocus) {
            if (content.item)
                content.item.forceActiveFocus(Qt.OtherFocusReason);
            else
                hub.forceActiveFocus(Qt.OtherFocusReason);
            return;
        }
        // a page with its own sub-view steps out of it first; back() returns true when it did
        var onPage = hub.pageId !== "" && (!frame.compact || hub.compactLevel === "work");
        if (onPage && content.item && typeof content.item.back === "function" && content.item.back())
            return;
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
            // the keys stay in the work pane: Home's first control takes them, so Tab goes on
            // there, not to the rail
            if (content.item) {
                var first = content.item.nextItemInFocusChain(true);
                var inside = false;
                for (var p = first; p; p = p.parent)
                    inside = inside || p === content.item;
                if (first && inside)
                    first.forceActiveFocus(Qt.TabFocusReason);
                else
                    content.item.forceActiveFocus(Qt.OtherFocusReason);
            }
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

    // the work pane: Home or a page (a hand-off page never opens here)
    function showContent(rowId) {
        var key = hub.pageId === "" ? "home" : hub.pageId;
        if (key !== hub.loadedKey) {
            hub.loadedKey = key;
            var p = hub.pageId === "" ? null : Registry.page(hub.pageId);
            if (p && p.file)
                content.setSource(Qt.resolvedUrl("pages/" + p.file + ".qml"), { hub: hub });
            else
                content.setSource(Qt.resolvedUrl("components/Home.qml"), { hub: hub });
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

    // "Category › Page › Section"; a page named like its category ("Updates › Updates") shows once
    function entryCrumb(parts) {
        return parts.filter((p, i) => p && p.length > 0 && p !== parts[i - 1]).join(" › ");
    }

    function both(text) {
        var t = I18n.tr(text);
        return t === text ? text : text + " " + t;
    }

    function schemaFor(file) {
        return Schemas.forFile(file);
    }

    // the search index is built when search is first used, then once per change to what it
    // reads: the revisions one change moves arrive apart, so rebuilds wait a moment to gather
    property bool searchArmed: false
    // the bar style in use's own options editor ("nacre", "frame-rails" or none): the Bar page
    // shows only that editor's option rows, so search lists only those
    LazyLoader {
        id: shellIndex
        active: hub.searchArmed
        ShellIndexFile {}
    }
    readonly property string barEditor: {
        Settings.revision;
        var index = shellIndex.item ? shellIndex.item.index : null;
        if (!index)
            return "";
        var style = String(Settings.get("barStyle") || "") || index.fallback;
        var entry = ShellIndex.items(index, ["barstyles", "subshells"]).find(e => e.id === style);
        return entry && entry.settings ? String(entry.settings.hubEditor || "") : "";
    }
    readonly property var searchSources: hub.searchArmed ? [Settings.revision, Settings.loaded, Settings.provider, ProviderSchema.revision,
        ProviderSchema.ready, Changes.desktopReady, Changes.revision, Host.ready, hub.barEditor, I18n.tr("Search settings")] : []
    onSearchSourcesChanged: {
        if (hub.searchArmed)
            searchRebuild.restart();
    }
    onSearchingChanged: {
        if (hub.searching)
            hub.armSearch();
    }
    function armSearch() {
        if (hub.searchArmed)
            return;
        hub.searchArmed = true;
        searchIndex.setEntries(hub.buildSearchEntries());
        // arming itself moved the sources; the build above already covers them
        searchRebuild.stop();
    }
    Timer {
        id: searchRebuild
        interval: 150
        onTriggered: searchIndex.setEntries(hub.buildSearchEntries())
    }

    function buildSearchEntries() {
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
            // each section is a result of its own, landing on its first row, unless a row
            // of that section already carries the same words; a section left with only its note
            // (nothing this window manager offers there) lands on the section itself
            for (var t = 0; t < schema.sections.length; t++) {
                var section = schema.sections[t];
                var first = rows.find(x => x.section === section.id && x.more !== true && hub.rowAvailable(x))
                    || rows.find(x => x.section === section.id && hub.rowAvailable(x));
                var same = rows.some(x => x.section === section.id && String(x.label).toLowerCase() === String(section.title).toLowerCase());
                // the page shows the note only for a section that has rows, none of them offered
                var noteOnly = !first && !!section.missing && rows.some(x => x.section === section.id);
                if (same || (!first && !noteOnly))
                    continue;
                out.push({
                    kind: "row", page: id, row: first ? first.id : section.id, command: [],
                    display: I18n.tr(section.title), label: hub.both(section.title), desc: "", kw: "",
                    group: hub.both(section.title), tab: "", sectionName: hub.both(p.title), isPage: false,
                    groupKey: id, groupTitle: I18n.tr(p.title),
                    crumb: hub.entryCrumb([I18n.tr(cat.label), I18n.tr(p.title)])
                });
            }
            for (var r = 0; r < rows.length; r++) {
                var row = rows[r];
                if (!hub.rowAvailable(row) || (row.editor && row.editor !== hub.barEditor))
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

    HubSearchIndex {
        id: searchIndex
        Component.onCompleted: setSynonyms(hub.synonyms)
    }

    // the index splits entries on "_", "-", "." and "/", so the query is split the same way
    // ("wpa_supplicant", "user.zsh")
    readonly property string searchText: hub.query.replace(/[_\-.\/]+/g, " ")
    // the index scores each word; an entry that holds the query as a whole phrase (in its name,
    // words or detail) comes first, keeping the index's order otherwise
    readonly property var results: {
        var found = searchIndex.revision >= 0 && hub.searching ? searchIndex.query(hub.searchText, 60) : [];
        var plain = s => String(s || "").replace(/[_\-.\/]+/g, " ").replace(/\s+/g, " ").trim().toLowerCase();
        var phrase = plain(hub.searchText);
        if (phrase.indexOf(" ") < 0)
            return found;
        var holds = e => plain(e.display).indexOf(phrase) >= 0 || plain(e.kw).indexOf(phrase) >= 0 || plain(e.desc).indexOf(phrase) >= 0;
        return found.filter(holds).concat(found.filter(e => !holds(e)));
    }
    readonly property var nearby: {
        if (!hub.searching || hub.results.length > 0)
            return [];
        var seen = {}, out = [];
        var words = hub.searchText.toLowerCase().split(/\s+/).filter(w => w.length >= 3);
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
            Spawn.runApp(entry.command, { label: String(entry.command[0]).replace(/^.*\//, "") });
            return;
        }
        hub.navigate(entry.page, entry.row);
        // the keyboard follows the choice into the page; its landing then takes the row
        if (content.item)
            content.item.forceActiveFocus(Qt.OtherFocusReason);
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
            // on a page, the newest change made there; on Home, the newest of all
            Changes.undo(hub.pageId);
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
        // a deep link from the launcher arrives with the process, so Home is never built first
        hub.pendingStart = Quickshell.env("RYOKU_HUB_START") || "";
        var startId = hub.pendingStart === "" ? "" : Registry.resolve(hub.pendingStart);
        var startPage = startId !== "" ? Registry.page(startId) : null;
        if (startPage && !startPage.handoff) {
            // the first frame already shows the page's category, only the page waits
            hub.setCategory(startPage.category);
            hub.compactLevel = "work";
        } else {
            hub.showHomeList();
            hub.compactLevel = "categories";
        }
        if (hub.pendingStart === "") {
            hub.showContent("");
            return;
        }
        startLimit.start();
    }

    // the start page is built once the window has shown its first frame, so the window comes up
    // as fast as Home's; a page gated on what the desktop supports opens once the settings frame
    // says it can
    property string pendingStart: ""
    property bool framed: false
    function openPendingStart(force) {
        if (hub.pendingStart === "")
            return;
        var id = Registry.resolve(hub.pendingStart);
        if (!force && (!hub.framed || id === "" || !hub.pageAvailable(id)))
            return;
        var start = hub.pendingStart;
        hub.pendingStart = "";
        startLimit.stop();
        hub.navigate(start, "");
    }
    Connections {
        target: hub.Window.window
        enabled: !hub.framed
        function onFrameSwapped() {
            hub.framed = true;
            hub.openPendingStart(false);
        }
    }
    Connections {
        target: Settings
        function onRevisionChanged() { hub.openPendingStart(false); }
    }
    // a page the desktop never offers lands on Home, as a deep link to it always has
    Timer {
        id: startLimit
        interval: 3000
        onTriggered: hub.openPendingStart(true)
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
                        Keys.onUpPressed: hub.moveCursor(-1)
                        Keys.onDownPressed: hub.moveCursor(1)
                        Keys.onReturnPressed: hub.openCursor()
                        Keys.onEnterPressed: hub.openCursor()
                        onTextChanged: hub.query = text
                        // build the index as the field is reached, before the first key
                        onActiveFocusChanged: {
                            if (activeFocus)
                                hub.armSearch();
                        }
                    }
                    Item { width: 1; height: LithicTokens.space.s2 }
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
                            cursor: hub.listCursor === 0 && !!listArea.categoryList
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
                    query: hub.searchText
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

                // made the first time something asks for a confirmation
                Loader {
                    id: confirmLoader
                    active: false
                    sourceComponent: Component {
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
                    }
                }

                Timer {
                    id: countdown
                    interval: 1000
                    repeat: true
                    onTriggered: {
                        var d = confirmLoader.item;
                        d.remaining--;
                        if (d.remaining <= 0) {
                            d.answer(false);
                            d.close();
                        }
                    }
                }

                Item {
                    id: placards
                    anchors.fill: parent
                    property var cards: []

                    // newest at the foot; a drag or held key on one setting updates its own notice,
                    // while discrete changes (those with a notice of their own) each get a card
                    function announce(entry) {
                        var key = Changes.keyOf(entry.bind);
                        var top = placards.cards.length > 0 ? placards.cards[0] : null;
                        if (top && top.key === key && !entry.notice && !top.entry.notice) {
                            top.entry = entry;
                            top.open();
                            return;
                        }
                        var stale = entry.notice ? [] : placards.cards.filter(p => p.key === key && !p.entry.notice);
                        var keep = [placardComponent.createObject(placards, { entry: entry, key: key })]
                            .concat(placards.cards.filter(p => stale.indexOf(p) < 0));
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
            // worded as it shows, so a change of language (the language row itself) reads in
            // the new language
            message: !card.entry ? "" : card.entry.notice ? card.entry.notice
                : I18n.tr("%1 changed").replace("%1", I18n.tr(card.entry.label))
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

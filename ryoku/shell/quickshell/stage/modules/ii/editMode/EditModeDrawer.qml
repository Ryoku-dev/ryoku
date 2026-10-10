import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Widgets
import Ryoku.Ui
import Ryoku.Ui.Singletons
import stage
import stage.services
import stage.modules.common
import stage.modules.common.widgets
import stage.modules.common.functions
import stage.modules.ii.background.widgets
import stage.modules.ii.background.shortcuts
import shell.services as ShellServices

/**
 * Edit Mode's panel: the surface that slides in from the right of the card.
 *
 * Six catalogues - the desktop's widgets, the bar, the dock, the lock
 * screen's own switches, the wallpaper (each screen's picture, the colour
 * source, the framing) and the style (presets, theme, palette) - and each
 * of them is a ROOT with sub-pages rather
 * than a list of accordions. That is the change: sections that expanded in
 * place put eighty rows in one scroll and left no room for anything a section
 * might want to say about itself, which is why the bar's style variants and
 * the dock's own appearance had nowhere to live and stayed in Settings. A page
 * per destination gives each one the whole panel.
 *
 * The address is `GlobalStates.editDrawerSection` plus `editDrawerPage`, held
 * shell-wide because a right-click on the desktop or the bar asks for a
 * catalogue before the panel that shows it exists, and the toolbar's Bar
 * button asks for one page in particular.
 *
 * The panel reports gestures and writes only preferences: every LAYOUT write
 * is made by the surface that owns the geometry, so a drop can be turned into
 * a canvas point and a history entry recorded in one place.
 */
Item {
    id: root

    property string screenName: ""
    // Where the drag ghost lives: an ancestor that is not clipped by the
    // panel's reveal, so the ghost can follow the pointer out over the card.
    property Item ghostParent: null

    signal addRequested(string widgetId, real dropX, real dropY)
    // A catalogue row's click: one more of this widget. Not a toggle - a
    // widget can be placed more than once, so a row has no on/off state to
    // flip; removal belongs to the copy, through its own menu or by dragging
    // it back into the panel.
    signal addInstanceRequested(string widgetId)
    signal barRemoveRequested(string componentId)
    // A component sent to a named list from its own page: unlike the add
    // above, it moves one that is already placed.
    signal barPlaceRequested(string componentId, string bucket)
    // A bar component carried out of the catalogue: where the pointer is, in
    // the chrome's coordinates, for the surface to hand to that screen's bar.
    signal barDragMoved(string componentId, real x, real y)
    signal barDropRequested(string componentId, real x, real y)
    signal barDragCancelled()
    signal addAppRequested(string appId, real dropX, real dropY)
    signal toggleAppOnHomeScreenRequested(string appId)
    signal addAppPairRequested(string firstAppId, string secondAppId, string name)
    signal addFolderRequested(string folderName, var appsList)
    signal clearHomeScreenAppsRequested()
    signal lockLayoutResetRequested()
    // A whole surface back to the shell's defaults: "widgets" (every desktop
    // widget removed), "bar", "dock" or "lockIslands". The surface answers
    // with one history entry, so the reset is one Ctrl+Z.
    signal resetRequested(string what)

    readonly property string section: GlobalStates.editDrawerSection
    // The address, validated against the catalogue showing it: a page belongs
    // to the section that minted it, and one left over from another section is
    // simply that section's root. Checked here rather than cleared on every
    // section change, so "open the bar's appearance page" is one intent
    // instead of two assignments whose order decides the answer.
    readonly property string page: root.pageValidFor(root.section, GlobalStates.editDrawerPage)
        ? GlobalStates.editDrawerPage : ""
    property var dragMetadata: null

    // A provider-added catalogue (Config.extraSections) showing now, or null.
    readonly property var extraSection: root.extraSectionFor(root.section)
    function extraSectionFor(section) {
        return Config.extraSections.find(s => s.section === section) ?? null;
    }

    function pageValidFor(section, page) {
        if (page === "")
            return true;
        if (root.extraSectionFor(section))
            return false;
        if (section === "apps")
            return true;
        if (section === "widgets")
            // The apps page is a desktop page: on the lock tab it is not a
            // valid address even when left over from the desktop side.
            return page.startsWith("category:") || ((page === "desktopApps" || page === "desktopIcons") && !root.lockTab);
        if (section === "bar")
            return page === "appearance" || page.startsWith("component:");
        if (section === "dock")
            return page === "appearance" || page === "widgets" || page.startsWith("apps:");
        if (section === "style")
            return page === "colours";
        if (section === "wallpaper")
            return page.startsWith("wallpapers");
        return false;
    }

    // ── Navigation ───────────────────────────────────────────────────────────
    // Which way the next page arrives from: 1 going deeper, -1 coming back.
    // The same two-value scalar ChatControlBar's canvas uses, for the same
    // reason - one Loader swapping its content has no other way to say which
    // direction the step was.
    property int navDirection: 1
    readonly property bool atRoot: root.page === "" || root.searching
    readonly property string backSection: {
        if (!root.extraSection)
            return "";
        const destination = "" + (root.extraSection.back ?? "");
        if (destination !== "")
            return destination;
        return root.extraSection.hidden === true ? "widgets" : "";
    }
    readonly property bool canGoBack: !root.searching
        && (root.page !== "" || root.backSection !== "")

    // Milliseconds between one row of a page entering and the next. The panel
    // is 380px wide and a page is a short run of rows, so this is smaller than
    // the transcript's: enough to read as filling, not as a queue.
    readonly property int staggerStep: 22

    // 0 while a page is arriving, 1 once it has. The Behavior would animate
    // the way BACK to 0 as well, which would fade the outgoing page out before
    // fading the incoming one in - two half-second beats for one step - so the
    // reset is made with the Behavior held off and only the rise is animated.
    property real pageReveal: 1
    property bool pageRevealSnapping: false
    Behavior on pageReveal {
        enabled: !Appearance.reducedMotion && !root.pageRevealSnapping
        animation: Appearance.animation.elementMoveFast.numberAnimation.createObject(root)
    }
    onContentKeyChanged: {
        root.pageRevealSnapping = true;
        root.pageReveal = 0;
        root.pageRevealSnapping = false;
        root.pageReveal = 1;
    }

    function openPage(page) {
        root.navDirection = 1;
        GlobalStates.editDrawerPage = page;
    }

    function goBack() {
        root.navDirection = -1;
        if (root.page !== "") {
            GlobalStates.editDrawerPage = "";
            return;
        }
        if (root.backSection !== "") {
            GlobalStates.editDrawerPage = "";
            GlobalStates.editDrawerSection = root.backSection;
        }
    }


    // ── The query ────────────────────────────────────────────────────────────
    // The dock's catalogue alone runs to two hundred rows. A query FLATTENS the
    // catalogue it filters, pages and all: someone typing is after one row, not
    // after where it lives. The lock screen's switches are not worth a box.
    readonly property bool searchable: root.section !== "lock" && root.section !== "style" && root.section !== "wallpaper"
        && root.extraSection === null
    property string query: ""
    readonly property string needle: root.query.trim().toLowerCase()
    readonly property bool searching: root.searchable && root.needle !== ""
    // Whether the chrome's surface has to hold the keyboard. It is None the
    // rest of the time, deliberately: the desktop's canvas answers Escape and
    // the arrows from another surface, and a chrome that takes focus swallows
    // them (see EditModeChromeSurface).
    //
    // Intent, NOT `searchField.activeFocus`: active focus needs an ACTIVE
    // window, a window is only active while the compositor gives its surface
    // the keyboard, and the surface only asks for the keyboard because of this
    // flag - reading activeFocus here is a deadlock that no click can break.
    // The field says when it wants the keyboard and says when it has lost it.
    property bool searchWanted: false
    property bool searchHeld: false
    // A second field that wants the keyboard the same way - the Style
    // catalogue's preset name. One at a time: asking for it lets the search
    // go, and the other way round.
    property bool fieldWanted: false
    property Item fieldItem: null
    readonly property bool searchFocused: root.searchWanted || root.fieldWanted

    function requestFieldFocus(item) {
        if (!item)
            return;
        root.searchWanted = false;
        root.searchHeld = false;
        searchField.focus = false;
        root.fieldItem = item;
        root.fieldWanted = true;
        item.forceActiveFocus();
    }

    function releaseFieldFocus() {
        root.fieldWanted = false;
        if (root.fieldItem)
            root.fieldItem.focus = false;
        root.fieldItem = null;
    }

    // Fuzzysort, the matcher the launcher already searches these same apps
    // with: "fx" finds Firefox, which a substring test never will, and the
    // results come back best-first. Targets are prepared where each catalogue
    // is built - preparing two hundred of them per keystroke is the shape that
    // made the launcher slow ([[launcher-perf-optimization]]).
    function prepared(title, id) {
        return Fuzzy.prepare(`${title ?? ""} ${id ?? ""} `);
    }
    function fuzzyPick(rows) {
        return Fuzzy.go(root.needle, rows, {
            "all": false,
            "key": "hay",
            "limit": 80,
            "threshold": 0.3
        }).map(result => result.obj);
    }

    function releaseSearchFocus() {
        root.searchWanted = false;
        root.searchHeld = false;
        searchField.focus = false;
        root.releaseFieldFocus();
    }

    function clearSearch() {
        searchField.text = "";
        root.releaseSearchFocus();
    }

    // Whether a point in the chrome's coordinates is on the field itself -
    // asked by the click-anywhere-else catcher, which must not fight the
    // field's own press or the clear button beside it.
    function pointInSearchField(x, y) {
        const from = root.ghostParent ?? root;
        const p = from.mapToItem(searchRow, x, y);
        if (p.x >= 0 && p.y >= 0 && p.x <= searchRow.width && p.y <= searchRow.height)
            return true;
        if (!root.fieldWanted || !root.fieldItem)
            return false;
        const q = from.mapToItem(root.fieldItem, x, y);
        return q.x >= 0 && q.y >= 0 && q.x <= root.fieldItem.width && q.y <= root.fieldItem.height;
    }

    onSectionChanged: root.clearSearch()

    // Ctrl+F, from the canvas. Only the screen being edited answers: every
    // screen draws a panel, and the keyboard is on one of them.
    function focusSearch() {
        if (!root.searchable || root.screenName !== GlobalStates.editModeMonitor)
            return;
        root.searchWanted = true;
        searchField.forceActiveFocus();
        searchField.selectAll();
    }

    Connections {
        target: GlobalStates
        // Leaving takes the query with it: a panel reopened on last week's
        // half-typed word is a panel that looks broken.
        function onEditDrawerOpenChanged() {
            if (!GlobalStates.editDrawerOpen)
                root.clearSearch();
        }
        function onEditModeChanged() {
            if (!GlobalStates.editMode)
                root.clearSearch();
        }
        function onEditSearchFocusRequested() {
            root.focusSearch();
        }
        function onEditSearchReleaseRequested() {
            root.releaseSearchFocus();
        }
    }

    // A desktop widget carried back over this panel: the release removes it.
    readonly property bool dropWouldRemove: root.screenName !== ""
        && GlobalStates.editDrawerDropScreen === root.screenName

    // ── Catalogues ───────────────────────────────────────────────────────────
    // Both lists follow the store: the island's own, or Ryoku's through the
    // provider the shell installs (stage.md). The rows never see the difference.
    readonly property var activeWidgets: Config.activeWidgets
    readonly property var usedBarIds: {
        if (root.section !== "bar")
            return [];
        const layouts = Config.options.bar.layouts;
        const ids = [];
        for (const bucket of ["left", "center", "right"])
            for (const entry of (layouts[bucket] ?? []))
                if (entry && entry.id) ids.push(entry.id);
        return ids;
    }
    readonly property bool barCentreBlocked: ShellModePolicy.barCenterActive
    // Every bar component, not only the ones going spare: one already on the
    // bar is named as such and its row still opens its page.
    //
    // Whether a component is PLACED is deliberately not in here. Folding it in
    // made the model a function of `bar.layouts`, so adding or removing one
    // component built a fresh array, reset the view, and replayed the whole
    // list's entrance - a cascade every time you placed a widget. The rows ask
    // `usedBarIds` for themselves instead, which re-evaluates one binding.
    readonly property var barCatalogue: {
        if (root.section !== "bar")
            return [];
        return (BarComponentRegistry.allComponents ?? []).map(component => ({
            "component": component,
            "hay": root.searching ? root.prepared(component.title, component.id) : ""
        }));
    }
    readonly property var barRows: root.searching ? root.fuzzyPick(root.barCatalogue) : root.barCatalogue

    // How many copies of a widget the desktop holds - and, on the Lockscreen
    // tab, how many of them the lock actually shows (any behaviour but hide).
    // A count rather than a boolean because a widget can be placed more than
    // once, which is also why the rows carry a number instead of a checkmark.
    function widgetCount(widgetId) {
        let count = 0;
        for (const entry of root.activeWidgets) {
            if (!entry || entry.widgetId !== widgetId)
                continue;
            if (root.lockTab && (entry.lockBehavior || "hide") === "hide")
                continue;
            count++;
        }
        return count;
    }

    // The catalogue's categories, named and ordered as Settings names and
    // orders them. A category the registry hands out that is not listed here -
    // an extension's own - gets one of its own at the end, so a widget can
    // never be added to the registry and then be missing from this list.
    readonly property var widgetCategoryOrder: [
        { "key": "Clock", "title": Translation.tr("Clocks"), "icon": "schedule" },
        { "key": "Media", "title": Translation.tr("Media players"), "icon": "play_circle" },
        { "key": "Weather", "title": Translation.tr("Weather"), "icon": "cloud" },
        { "key": "Date", "title": Translation.tr("Date & calendar"), "icon": "calendar_today" },
        { "key": "Photo", "title": Translation.tr("Photo"), "icon": "image" },
        { "key": "Devices", "title": Translation.tr("Devices & Bluetooth"), "icon": "earbuds", "merge": ["Bluetooth"] },
        { "key": "Utility", "title": Translation.tr("Utility"), "icon": "build" },
        { "key": "System", "title": Translation.tr("System"), "icon": "tune" },
        { "key": "Resources", "title": Translation.tr("Resources"), "icon": "monitor_heart" }
    ]

    // Group a flat catalogue (rows with widgetId/name/icon/category) into the
    // drawer's group shape, honouring widgetCategoryOrder where it applies.
    function groupRows(catalogue) {
        const groups = [];
        const byKey = {};
        for (const widget of (catalogue ?? [])) {
            const key = widget?.category ?? "";
            let group = byKey[key];
            if (!group) {
                const named = root.widgetCategoryOrder.find(c => c.key === key);
                group = {
                    "key": key === "" ? "other" : key,
                    "title": named ? named.title : (key === "" ? Translation.tr("Other") : key),
                    "icon": named ? named.icon : "widgets",
                    "items": []
                };
                byKey[key] = group;
                groups.push(group);
            }
            group.items.push(widget);
        }
        return groups.filter(group => group.items.length > 0);
    }

    readonly property var widgetGroups: {
        // With Ryoku's store mounted the catalogue is Ryoku's roster: the
        // desktop's real widgets, grouped by their own set names. The
        // reference's registry only serves the island's self-contained mode.
        if (Config.widgetProvider)
            return root.groupRows(Config.widgetCatalogue);
        const groups = [];
        const byKey = {};
        for (const category of root.widgetCategoryOrder) {
            const group = { "key": category.key, "title": category.title, "icon": category.icon, "items": [] };
            groups.push(group);
            byKey[category.key] = group;
            for (const alias of (category.merge ?? []))
                byKey[alias] = group;
        }
        for (const widget of (WidgetsRegistry.allWidgets ?? [])) {
            const key = widget?.category ?? "";
            let group = byKey[key];
            if (!group) {
                group = {
                    "key": key === "" ? "other" : key,
                    "title": key === "" ? Translation.tr("Other") : key,
                    "icon": "widgets",
                    "items": []
                };
                byKey[key] = group;
                groups.push(group);
            }
            group.items.push(widget);
        }
        return groups.filter(group => group.items.length > 0);
    }

    function widgetGroupByKey(key) {
        return root.widgetGroups.find(group => group.key === key) ?? null;
    }

    // How many KINDS in this category are placed, not how many copies: the
    // header reads "3/20", which is about the catalogue, and a copy count
    // there could exceed the number of rows below it.
    function widgetGroupAdded(group) {
        let count = 0;
        for (const widget of group.items)
            if (root.widgetCount(widget.widgetId) > 0)
                count++;
        return count;
    }

    // The flattened list a query searches, prepared once per catalogue rather
    // than per keystroke.
    readonly property var widgetSearchRows: {
        if (root.section !== "widgets" || !root.searching)
            return [];
        const rows = [];
        for (const group of root.widgetGroups)
            for (const widget of group.items)
                rows.push({ "widget": widget, "hay": root.prepared(widget.name, widget.widgetId) });
        return rows;
    }

    // What the widget list is showing: a query's answer, or the open category.
    readonly property var widgetItems: {
        if (root.searching)
            return root.fuzzyPick(root.widgetSearchRows).map(row => row.widget);
        if (!root.page.startsWith("category:"))
            return [];
        const group = root.widgetGroupByKey(root.page.substring(9));
        return group ? group.items : [];
    }

    // The dock catalogue reads the shell's canonical desktop ids. Running
    // aliases are resolved before grouping so one application never appears in
    // both Open now and On the dock under different names.
    readonly property var dockGroups: {
        if (root.section !== "dock")
            return [];
        const pinnedIds = ShellServices.Dock.pinnedOrStarter();
        const taken = {};
        for (const id of pinnedIds)
            taken[ShellServices.Dock.canonicalId(id)] = true;
        const running = [];
        for (const client of ShellServices.Dock.clients) {
            const id = ShellServices.Dock.canonicalId(client.className);
            if (!id || taken[id] || running.some(app => app.appId === id))
                continue;
            taken[id] = true;
            running.push({ appId: id });
        }
        const rest = Array.from(AppSearch.list ?? [])
            .filter(entry => entry && entry.id && !entry.noDisplay
                && !taken[ShellServices.Dock.canonicalId(entry.id)]);
        const item = (appId, pinned, name) => ({
            "appId": ShellServices.Dock.canonicalId(appId),
            "pinned": pinned,
            "name": name || root.appName(appId)
        });
        return [
            {
                "key": "pinned",
                "title": Translation.tr("On the dock"),
                "icon": "keep",
                "items": pinnedIds.filter(id => !!id).map(id => item(id, true, ""))
            },
            {
                "key": "running",
                "title": Translation.tr("Open now"),
                "icon": "select_window",
                "items": running.map(app => item(app.appId, false, ""))
            },
            {
                "key": "installed",
                "title": Translation.tr("All apps"),
                "icon": "apps",
                "items": rest.map(entry => item(entry.id, false, entry.name))
            }
        ];
    }

    function appName(appId) {
        return DesktopEntries.heuristicLookup(appId)?.name ?? appId;
    }

    function dockGroupByKey(key) {
        return root.dockGroups.find(group => group.key === key) ?? null;
    }

    readonly property var dockSearchRows: {
        if (root.section !== "dock" || !root.searching)
            return [];
        const rows = [];
        for (const group of root.dockGroups)
            for (const app of group.items)
                rows.push({ "app": app, "hay": root.prepared(app.name, app.appId) });
        return rows;
    }

    readonly property var dockItems: {
        if (root.searching)
            return root.fuzzyPick(root.dockSearchRows).map(row => row.app);
        if (!root.page.startsWith("apps:"))
            return [];
        const group = root.dockGroupByKey(root.page.substring(5));
        return group ? group.items : [];
    }

    readonly property var homeAppsItems: {
        if (root.section !== "apps")
            return [];
        void GlobalStates.homeScreenAppsRevision;
        const q = root.needle;
        const all = Array.from(AppSearch.list ?? []).filter(e => e && e.id && !e.noDisplay);
        const isAppOnHome = GlobalStates.isAppOnHomeScreenHandler ?? (() => false);
        const mapped = all.map(entry => ({
            "id": entry.id,
            "name": entry.name ?? entry.id,
            "genericName": entry.genericName ?? "",
            "comment": entry.comment ?? "",
            "onScreen": isAppOnHome(entry.id)
        }));
        if (!q)
            return mapped;
        return mapped.filter(item => item.name.toLowerCase().includes(q)
            || item.id.toLowerCase().includes(q)
            || item.genericName.toLowerCase().includes(q));
    }
    property string desktopAppsDestination: ""
    // Read the adapter property explicitly: itemsFor() is a function call, and
    // QML otherwise misses the late states.json load that populates this list.
    readonly property var desktopFolders: {
        void Persistent.states.desktopShortcutsJson;
        return DesktopShortcuts.folders(root.screenName);
    }
    readonly property string resolvedDesktopAppsDestination: root.desktopFolders
        .some(folder => folder.id === root.desktopAppsDestination) ? root.desktopAppsDestination : ""
    readonly property var desktopDestinationOptions: [""]
        .concat(root.desktopFolders.map(folder => folder.id)).concat(["__new__"])
    readonly property var desktopDestinationLabels: {
        const labels = {
            "": Translation.tr("Desktop"),
            "__new__": Translation.tr("+ New folder")
        };
        for (const folder of root.desktopFolders) {
            const name = folder.name.length > 22 ? folder.name.slice(0, 21) + "…" : folder.name;
            labels[folder.id] = folder.count > 0 ? `${name} (${folder.count})` : name;
        }
        return labels;
    }
    readonly property var desktopAppsItems: {
        void Persistent.states.desktopShortcutsJson;
        if (root.section !== "widgets" || root.page !== "desktopApps")
            return [];
        const q = root.needle;
        const all = Array.from(AppSearch.list ?? []).filter(e => e && e.id && !e.noDisplay);
        const mapped = all.map(entry => {
            const location = DesktopShortcuts.locate(root.screenName, entry.id);
            let placement = "";
            if (location.where === "desktop")
                placement = Translation.tr("On the desktop");
            else if (location.isStack)
                placement = Translation.tr("In the %1 stack").arg(location.folderName);
            else if (location.where === "folder")
                placement = Translation.tr("In %1").arg(location.folderName);
            return {
                "id": entry.id,
                "name": entry.name ?? entry.id,
                "genericName": entry.genericName ?? "",
                "comment": entry.comment ?? "",
                "onScreen": location.where !== "",
                "placement": placement
            };
        });
        if (!q)
            return mapped;
        return mapped.filter(item => item.name.toLowerCase().includes(q)
            || item.id.toLowerCase().includes(q)
            || item.genericName.toLowerCase().includes(q));
    }
    readonly property int desktopAppCount: {
        void Persistent.states.desktopShortcutsJson;
        return DesktopShortcuts.placedAppIds(root.screenName).length;
    }
    readonly property int desktopIconCount: {
        void Persistent.states.desktopShortcutsJson;
        return DesktopShortcuts.itemsFor(root.screenName).length;
    }

    function createDesktopFolder() {
        const folderId = DesktopShortcuts.newFolder(root.screenName, "");
        if (folderId) {
            root.desktopAppsDestination = folderId;
            if (DesktopShortcuts.hidden)
                DesktopShortcuts.setHidden(false);
        }
        return folderId;
    }
    function chooseDesktopDestination(key) {
        if (key === "__new__")
            root.createDesktopFolder();
        else
            root.desktopAppsDestination = key;
    }
    function toggleAppOnDesktop(appId) {
        if (!appId || !Persistent.ready || Persistent.blockWrites)
            return;
        if (DesktopShortcuts.locate(root.screenName, appId).where) {
            DesktopShortcuts.removeApp(root.screenName, appId);
            return;
        }
        const app = DesktopShortcuts.application(appId);
        if (!app)
            return;
        const destination = root.resolvedDesktopAppsDestination;
        if (destination) {
            DesktopShortcuts.addToFolder(root.screenName, destination, [app]);
        } else {
            const items = DesktopShortcuts.itemsFor(root.screenName);
            const screen = Quickshell.screens.find(s => s.name === root.screenName);
            const grid = DesktopShortcuts.grid(root.screenName);
            const cell = DesktopShortcuts.firstFree(grid, DesktopShortcuts.takenCells(grid, items));
            if (!cell)
                return;
            const point = DesktopShortcuts.cellPos(grid, cell.col, cell.row);
            DesktopShortcuts.add(root.screenName, [app], point.x, point.y, "",
                screen?.width ?? grid.area.screenWidth, screen?.height ?? grid.area.screenHeight);
        }
        if (DesktopShortcuts.hidden)
            DesktopShortcuts.setHidden(false);
    }

    readonly property bool lockTab: GlobalStates.editLockPreview
    onLockTabChanged: {
        if (root.lockTab ? (root.section === "bar" || root.section === "dock" || root.section === "apps") : root.section === "lock")
            GlobalStates.editDrawerSection = "widgets";
    }
    readonly property bool anyLockFork: root.activeWidgets.some(entry =>
        WidgetPlacement.fork(entry, root.screenName, true) !== null)

    // The lock's own switches, written straight to config: preferences, not
    // layout edits, so no history entry - same as their Settings toggles.
    readonly property var lockSwitches: [
        { "key": "nowPlaying", "group": "lock", "symbol": "music_note", "title": Translation.tr("Now playing") },
        { "key": "sports", "group": "lock", "symbol": "sports_soccer", "title": Translation.tr("Sports") },
        { "key": "showAlarm", "group": "lock", "symbol": "alarm", "title": Translation.tr("Next alarm") },
        { "key": "showWeather", "group": "lock", "symbol": "partly_cloudy_day", "title": Translation.tr("Weather") },
        { "key": "showLockedText", "group": "lock", "symbol": "lock", "title": Translation.tr("\"Locked\" text") },
        { "key": "showIndicator", "group": "fingerprint", "symbol": "fingerprint", "title": Translation.tr("Fingerprint indicator") }
    ]
    function lockSwitchTarget(entry) {
        return entry.group === "fingerprint" ? Config.options.lock.security.fingerprint : Config.options.lock;
    }

    // Everything the lock screen's toolbars can hide, so a remove badge there
    // has a way back. LockIslands' vocabulary; the password field is not on it
    // because a lock screen with no way to type into it is not a lock screen.
    readonly property var lockIslandNames: ({
        "battery": Translation.tr("Battery"),
        "capsLock": Translation.tr("Caps Lock"),
        "alarm": Translation.tr("Next alarm"),
        "weather": Translation.tr("Weather"),
        "keyboardLayout": Translation.tr("Keyboard layout"),
        "keepAwake": Translation.tr("Keep awake"),
        "mode": Translation.tr("Active mode"),
        "sleep": Translation.tr("Sleep"),
        "power": Translation.tr("Shut down"),
        "reboot": Translation.tr("Restart")
    })
    readonly property var lockIslandSymbols: ({
        "battery": "battery_full", "capsLock": "keyboard_capslock", "alarm": "alarm",
        "weather": "partly_cloudy_day", "keyboardLayout": "keyboard", "keepAwake": "coffee",
        "mode": "target", "sleep": "dark_mode", "power": "power_settings_new",
        "reboot": "restart_alt"
    })
    readonly property var hiddenLockIslands: Array.from(Config.options.lock.islands.hidden ?? [])

    function showLockIsland(id) {
        Config.setLockIslandHidden(id, false);
    }

    // ── The page the panel is showing ────────────────────────────────────────
    readonly property string contentKey: root.searching
        ? ("search:" + root.section)
        : (root.section + "/" + root.page)

    readonly property string headerTitle: {
        if (root.searching)
            return Translation.tr("Results");
        if (root.page === "") {
            if (root.extraSection)
                return root.extraSection.label;
            if (root.section === "apps")
                return Translation.tr("Home screen apps");
            if (root.section === "bar")
                return Translation.tr("Bar");
            if (root.section === "dock")
                return PanelFamily.touchFirst ? Translation.tr("Taskbar") : Translation.tr("Dock");
            if (root.section === "lock")
                return Translation.tr("Lock screen");
            if (root.section === "style")
                return Translation.tr("Style");
            if (root.section === "wallpaper")
                return Translation.tr("Wallpaper");
            return Translation.tr("Widgets");
        }
        if (root.page.startsWith("wallpapers")) {
            const target = root.wallpaperPageTarget;
            if (target === "lockscreen")
                return Translation.tr("Lock screen wallpaper");
            if (target === "lightmode")
                return Translation.tr("Light mode wallpaper");
            if (target === "screen")
                return Translation.tr("Wallpaper for %1").arg(root.screenName);
            return Translation.tr("Wallpaper");
        }
        if (root.section === "apps") {
            if (root.page === "createPair")
                return Translation.tr("New App Pair");
            if (root.page === "createFolder")
                return Translation.tr("New App Folder");
        }
        if (root.page === "colours")
            return Translation.tr("Colour scheme");
        if (root.page === "desktopApps")
            return Translation.tr("Add apps to desktop");
        if (root.page === "desktopIcons")
            return Translation.tr("Desktop icons");
        if (root.page.startsWith("category:"))
            return root.widgetGroupByKey(root.page.substring(9))?.title ?? Translation.tr("Widgets");
        if (root.page.startsWith("apps:"))
            return root.dockGroupByKey(root.page.substring(5))?.title ?? Translation.tr("Apps");
        if (root.page.startsWith("component:"))
            return BarComponentRegistry.getComponent(root.page.substring(10))?.title ?? Translation.tr("Widget");
        if (root.page === "appearance")
            return root.section === "dock"
                ? (PanelFamily.touchFirst ? Translation.tr("Taskbar appearance & items") : Translation.tr("Dock design & settings"))
                : Translation.tr("Bar appearance");
        if (root.page === "widgets")
            return Translation.tr("Pinned apps & order");
        return Translation.tr("Edit");
    }

    readonly property string headerSymbol: {
        if (root.extraSection)
            return root.extraSection.icon;
        if (root.section === "apps") {
            if (root.page === "createPair")
                return "splitscreen";
            if (root.page === "createFolder")
                return "create_new_folder";
            return "apps";
        }
        if (root.section === "bar")
            return "dock_to_bottom";
        if (root.section === "dock")
            return PanelFamily.touchFirst ? "dock_to_bottom" : "dock";
        if (root.section === "lock")
            return "lock";
        if (root.section === "style")
            return "palette";
        if (root.section === "wallpaper")
            return "wallpaper";
        if (root.page === "desktopApps")
            return "apps";
        if (root.page === "desktopIcons")
            return "grid_view";
        return "widgets";
    }

    function toGhost(sceneX, sceneY) {
        const host = root.ghostParent ?? root;
        return host.mapFromItem(null, sceneX, sceneY);
    }

    function beginGhost(metadata, sceneX, sceneY) {
        root.dragMetadata = metadata;
        root.moveGhost(sceneX, sceneY);
    }

    function moveGhost(sceneX, sceneY) {
        const p = root.toGhost(sceneX, sceneY);
        ghost.x = p.x - ghost.width / 2;
        ghost.y = p.y - ghost.height / 2;
    }

    Rectangle {
        id: panel
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        width: Appearance.sizes.editModeDrawerWidth
        color: Appearance.colors.colLayer0
        radius: Appearance.rounding.small
        border.width: 1
        border.color: Appearance.colors.colOutline
        clip: true

        // The remove tint: lit while a desktop widget is carried over the panel.
        Rectangle {
            anchors.fill: parent
            radius: panel.radius
            color: root.dropWouldRemove ? Appearance.colors.colLayer1Active : "transparent"
            Behavior on color {
                enabled: !Appearance.reducedMotion
                animation: Appearance.animation.elementMoveFast.colorAnimation.createObject(this)
            }
        }

        ColumnLayout {
            id: column
            anchors.fill: parent
            anchors.margins: Appearance.sizes.space4
            spacing: Appearance.sizes.space3
            // The contents arrive after the panel: faded on the panel's own scalar.
            opacity: Math.max(0, Math.min(1, (GlobalStates.editDrawerProgress - 0.4) / 0.6))

            // ── Header ───────────────────────────────────────────────────────
            RowLayout {
                Layout.fillWidth: true
                spacing: Appearance.sizes.space3

                // The section mark becomes a back action on sub-pages and
                // provider pages with an explicit return destination.
                Rectangle {
                    Layout.alignment: Qt.AlignVCenter
                    implicitWidth: Appearance.sizes.controlHeight
                    implicitHeight: Appearance.sizes.controlHeight
                    radius: Appearance.rounding.small
                    color: !root.canGoBack ? "transparent"
                        : backMouse.containsPress
                            ? Appearance.colors.colLayer1Active
                            : backMouse.containsMouse
                                ? Appearance.colors.colLayer1Hover
                                : "transparent"
                    border.width: 1
                    border.color: root.canGoBack
                        ? Appearance.colors.colOutlineVariant : "transparent"

                    Behavior on color {
                        enabled: !Appearance.reducedMotion
                        animation: Appearance.animation.elementMoveFast.colorAnimation.createObject(this)
                    }

                    MaterialSymbol {
                        anchors.centerIn: parent
                        text: root.canGoBack ? "arrow_back" : root.headerSymbol
                        iconSize: 18
                        color: Appearance.colors.colOnSurface
                    }

                    MouseArea {
                        id: backMouse
                        anchors.fill: parent
                        enabled: root.canGoBack
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.goBack()
                    }
                }

                ColumnLayout {
                    Layout.fillWidth: true
                    Layout.minimumWidth: 0
                    spacing: 1

                    RowLayout {
                        Layout.fillWidth: true
                        Layout.minimumWidth: 0
                        spacing: Appearance.sizes.space2

                        Rectangle {
                            implicitWidth: 2
                            implicitHeight: 10
                            radius: 1
                            color: Appearance.colors.colOnSurfaceVariant
                        }
                        StyledText {
                            text: Translation.tr("Stage editor").toUpperCase()
                            font.family: Appearance.font.family.monospace
                            font.pixelSize: Appearance.font.pixelSize.smallest
                            font.weight: Font.Medium
                            font.letterSpacing: Appearance.font.trackLabel
                            color: Appearance.colors.colSubtext
                        }
                        Rectangle {
                            Layout.fillWidth: true
                            Layout.minimumWidth: 0
                            implicitHeight: 1
                            color: Appearance.colors.colOutlineVariant
                        }
                    }

                    StyledText {
                        Layout.fillWidth: true
                        Layout.minimumWidth: 0
                        text: root.headerTitle
                        font.family: Appearance.font.family.title
                        font.pixelSize: Appearance.font.pixelSize.larger
                        font.weight: Font.Medium
                        color: Appearance.colors.colOnSurface
                        elide: Text.ElideRight
                    }
                }
            }


            StyledText {
                Layout.fillWidth: true
                Layout.leftMargin: Appearance.sizes.space1
                Layout.rightMargin: Appearance.sizes.space1
                visible: root.atRoot && !root.searching && text.length > 0
                text: root.extraSection ? root.extraSection.intro
                    : root.section === "apps"
                    ? Translation.tr("Add apps, pairs or folders to the home screen, or drag to place them.")
                    : root.section === "widgets"
                    ? (root.lockTab
                        ? Translation.tr("Drag a widget onto the lock screen to place it, or click to add one. Drag one back here to remove it.")
                        : Translation.tr("Drag a widget onto the desktop to place it, or click to add one. Drag one back here to remove it."))
                    : root.section === "lock"
                        ? Translation.tr("What the lock screen shows besides your widgets.")
                    : root.section === "style"
                        ? Translation.tr("Presets, light or dark, and the colours everything is drawn in. Changes apply at once; undo takes them back.")
                    : root.section === "wallpaper"
                        ? (root.lockTab
                            ? Translation.tr("The picture behind the lock screen. Position and zoom are set on the Desktop tab.")
                            : Translation.tr("Each screen's picture and how it sits. While this is open, drag the desktop to move the wallpaper and use the wheel or a pinch to zoom."))
                    : root.section === "bar"
                        ? Translation.tr("Drag a widget onto the bar to drop it where you want it, or open one to change how it looks.")
                        : (PanelFamily.touchFirst
                            ? Translation.tr("Configure taskbar appearance and items.")
                            : Translation.tr("Pin apps, and choose how the dock itself is drawn."))
                font.family: Appearance.font.family.main
                font.pixelSize: Appearance.font.pixelSize.small
                font.weight: Font.Normal
                color: Appearance.colors.colSubtext
                wrapMode: Text.Wrap
                lineHeightMode: Text.ProportionalHeight
                lineHeight: 1.35
            }

            // ── Search ───────────────────────────────────────────────────────
            // One field for all three catalogues: it filters whichever is
            // showing, and the surface only takes the keyboard while it holds
            // focus.
            Item {
                id: searchRow
                // A root-level affordance: a query flattens the catalogue, so
                // it belongs to the catalogue rather than to a page inside it.
                // Typing on a page still works - the results take the panel
                // over and clearing the field puts the page back.
                visible: root.searchable && root.atRoot
                Layout.fillWidth: true
                Layout.leftMargin: Appearance.sizes.space1
                Layout.rightMargin: Appearance.sizes.space1
                implicitHeight: 36

                ToolbarTextField {
                    id: searchField
                    anchors.fill: parent
                    Layout.fillHeight: false
                    leftPadding: 32
                    rightPadding: 32
                    placeholderText: root.section === "apps" || (root.section === "widgets" && root.page === "desktopApps")
                        ? Translation.tr("Search applications")
                        : root.section === "dock" ? Translation.tr("Search apps")
                        : root.section === "bar" ? Translation.tr("Search bar widgets")
                        : Translation.tr("Search widgets")
                    onTextChanged: root.query = searchField.text
                    onPressed: root.searchWanted = true
                    // Focus moved on inside the panel - a row, a section
                    // button - so the keyboard goes back to the desktop.
                    onActiveFocusChanged: {
                        if (searchField.activeFocus) {
                            root.searchHeld = true;
                            return;
                        }
                        if (!root.searchHeld)
                            return;
                        root.searchHeld = false;
                        root.searchWanted = false;
                    }
                    // The first Escape empties the field, the second gives the
                    // keyboard back - and with it the mode's own Escape ladder.
                    Keys.onEscapePressed: event => {
                        if (searchField.text !== "") {
                            searchField.text = "";
                            return;
                        }
                        root.releaseSearchFocus();
                        event.accepted = true;
                    }
                }

                MaterialSymbol {
                    anchors.left: parent.left
                    anchors.leftMargin: Appearance.sizes.space3
                    anchors.verticalCenter: parent.verticalCenter
                    text: "search"
                    iconSize: 18
                    color: Appearance.colors.colSubtext
                }

                FadeLoader {
                    anchors.right: parent.right
                    anchors.rightMargin: Appearance.sizes.space1
                    anchors.verticalCenter: parent.verticalCenter
                    shown: searchField.text !== ""
                    sourceComponent: RippleButton {
                        implicitWidth: 26
                        implicitHeight: 26
                        buttonRadius: Appearance.rounding.small
                        colBackground: "transparent"
                        colBackgroundHover: Appearance.withAlpha(Appearance.m3colors.m3onSurface, 0.09)
                        colBackgroundActive: Appearance.withAlpha(Appearance.m3colors.m3onSurface, 0.16)
                        colRipple: Appearance.withAlpha(Appearance.m3colors.m3onSurface, 0.16)
                        borderWidth: 1
                        borderColor: Appearance.withAlpha(Appearance.m3colors.m3onSurface, 0.18)
                        onClicked: {
                            searchField.text = "";
                            searchField.forceActiveFocus();
                        }
                        contentItem: MaterialSymbol {
                            horizontalAlignment: Text.AlignHCenter
                            text: "close"
                            iconSize: 16
                            color: Appearance.colors.colOnSurfaceVariant
                        }
                    }
                }
            }

            // The reading pane is a bounded paper layer. Its own six-pixel clip
            // keeps scrolled labels and thumbnails inside the drawer surface.
            ClippingRectangle {
                Layout.fillWidth: true
                Layout.fillHeight: true
                color: Appearance.withAlpha(Appearance.m3colors.m3surface, 0.965)
                radius: Appearance.rounding.small

                Loader {
                    id: pageLoader
                    anchors.fill: parent
                    // The page changes under one Loader, so the arrival is
                    // animated here rather than by each page: they would all
                    // have to carry the same block and could not agree on the
                    // direction, which only the navigation knows.
                    //
                    // One scalar, one Behavior, no one-shot animation: the
                    // mode's motion audit allows a duration and a curve only
                    // through an Appearance tier, and a hand-built
                    // `ParallelAnimation` cannot get one. `root.pageReveal`
                    // carries both the fade and the slide.
                    opacity: root.pageReveal
                    transform: Translate {
                        x: (1 - root.pageReveal) * 28 * root.navDirection
                    }

                    sourceComponent: {
                        if (root.extraSection)
                            return root.extraSection.page;
                        if (root.searching)
                            return root.section === "bar" ? barListPage
                                : root.section === "dock" ? dockAppListPage
                                : root.section === "apps" ? appsListPage
                                : root.section === "widgets" && root.page === "desktopApps" ? desktopAppsPage
                                : widgetListPage;
                        if (root.section === "apps") {
                            if (root.page === "createPair")
                                return createPairPage;
                            if (root.page === "createFolder")
                                return createFolderPage;
                            return appsListPage;
                        }
                        if (root.section === "widgets")
                            return root.page === "desktopApps" ? desktopAppsPage
                                : root.page === "desktopIcons" ? desktopIconsPage
                                : root.page.startsWith("category:") ? widgetListPage : widgetCategoriesPage;
                        if (root.section === "lock")
                            return lockPage;
                        if (root.section === "style")
                            return root.page === "colours" ? colourPage : stylePage;
                        if (root.section === "wallpaper")
                            return root.page.startsWith("wallpapers") ? wallpaperPage : wallpaperRootPage;
                        if (root.section === "bar") {
                            if (root.page === "appearance")
                                return barAppearancePage;
                            if (root.page.startsWith("component:"))
                                return barComponentPage;
                            return barRootPage;
                        }
                        if (root.page === "appearance")
                            return PanelFamily.touchFirst ? tabletDockAppearancePage : dockAppearancePage;
                        if (root.page === "widgets")
                            return dockWidgetsPage;
                        if (root.page.startsWith("apps:"))
                            return dockAppListPage;
                        return dockRootPage;
                    }
                }
            }
        }
    }

    // ── Pages ────────────────────────────────────────────────────────────────

    // The widget catalogue's root: one row per category, opening a page of its
    // own. The count says how many of it are already placed.
    Component {
        id: widgetCategoriesPage

        StyledListView {
            id: categoryList
            // The page fills top-down. Only the views whose MODEL is stable
            // get this: a view whose array is rebuilt on every placement
            // change would replay the whole cascade each time.
            staggerStep: root.staggerStep
            clip: true
            spacing: 3
            model: root.widgetGroups

            delegate: EditPanelRow {
                required property var modelData
                required property int index
                readonly property int added: root.widgetGroupAdded(modelData)
                width: categoryList.width
                first: index === 0
                last: index === root.widgetGroups.length - 1
                symbol: modelData.icon
                title: modelData.title
                valueText: added > 0 ? `${added}/${modelData.items.length}` : `${modelData.items.length}`
                trailingKind: "chevron"
                onActivated: root.openPage("category:" + modelData.key)
            }

            // The tablet's app catalogue, ported: a row at the end of the
            // widget list opens the same check/add app page, writing desktop
            // shortcut icons instead of home-screen ones. Hidden on the lock
            // tab, where this section catalogues lock widgets, not desktops.
            // The clear row below keeps its own rule: shown only while there
            // is something to clear, so the root is not a place that offers
            // to delete nothing.
            //
            // A ListView gives its footer NO item spacing: the footer lands
            // flush on the last delegate, which glued "Add apps manually"
            // onto the last category row. The wrapper's 10+3 gutter is the
            // same one the tablet page's footer below keeps.
            footer: Item {
                width: categoryList.width
                height: footerColumn.implicitHeight + 13
                visible: !root.lockTab || root.activeWidgets.length > 0

                ColumnLayout {
                    id: footerColumn
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.topMargin: 10
                    spacing: 10

                    EditPanelRow {
                        id: desktopAppsRow
                        Layout.fillWidth: true
                        visible: !root.lockTab
                        first: true
                        last: true
                        symbol: "add_to_home_screen"
                        title: Translation.tr("Add apps to desktop")
                        subtitle: Translation.tr("Place apps on the desktop or in a folder")
                        valueText: root.desktopAppCount > 0 ? `${root.desktopAppCount}` : ""
                        trailingKind: "chevron"
                        onActivated: root.openPage("desktopApps")
                    }
                    // The desktop icons' look - size, spacing, origin, labels,
                    // backgrounds, badges - on a page of its own. It lives
                    // beside the shortcuts row because it speaks of the
                    // shortcuts, not of the widget canvas above it.
                    EditPanelRow {
                        Layout.fillWidth: true
                        visible: !root.lockTab
                        first: true
                        last: true
                        symbol: "grid_view"
                        title: Translation.tr("Desktop icons")
                        subtitle: Translation.tr("Arrange icons, folders, labels and marks")
                        valueText: root.desktopIconCount > 0 ? `${root.desktopIconCount}` : ""
                        trailingKind: "chevron"
                        onActivated: root.openPage("desktopIcons")
                    }

                    // A clean slate, one Ctrl+Z away.
                    EditPanelRow {
                        id: clearRow
                        Layout.fillWidth: true
                        visible: root.activeWidgets.length > 0
                        first: true
                        last: true
                        destructive: true
                        symbol: "delete_sweep"
                        title: Translation.tr("Remove every widget")
                        valueText: `${root.activeWidgets.length}`
                        trailingKind: "none"
                        onActivated: root.resetRequested("widgets")
                    }
                }
            }
        }
    }

    Component {
        id: desktopIconsPage
        EditDesktopIconsPage {
            screenName: root.screenName
            onAddAppsToFolder: folderId => {
                root.desktopAppsDestination = folderId;
                root.openPage("desktopApps");
            }
        }
    }

    Component {
        id: desktopAppsPage

        Item {
            SettingCard {
                id: destinationCard
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                title: Translation.tr("DESTINATION")
                collapsible: false

                SettingRow {
                    width: parent.width
                    label: Translation.tr("Add apps to")
                    desc: Translation.tr("Placed apps can be removed from any location below")
                    block: true
                    Chips {
                        width: parent.width
                        options: root.desktopDestinationOptions
                        labels: root.desktopDestinationLabels
                        current: root.resolvedDesktopAppsDestination
                        onChose: key => root.chooseDesktopDestination(key)
                    }
                }
                SettingRow {
                    width: parent.width
                    visible: Config.options.background.desktopIcons.stacks ?? false
                    divider: true
                    label: Translation.tr("New desktop apps join the Apps stack")
                }
            }

            StyledListView {
                id: desktopAppList
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: destinationCard.bottom
                anchors.topMargin: Tokens.s3
                anchors.bottom: parent.bottom
                popin: false
                animateAppearance: false
                clip: true
                spacing: Tokens.s1
                model: root.desktopAppsItems

                // Every toggle rewrites the store, which hands the list a new
                // model and would throw the reader back to the top.
                property real heldY: -1
                function toggle(appId) {
                    desktopAppList.heldY = desktopAppList.contentY;
                    root.toggleAppOnDesktop(appId);
                }
                onModelChanged: {
                    if (desktopAppList.heldY < 0)
                        return;
                    const y = desktopAppList.heldY;
                    desktopAppList.heldY = -1;
                    Qt.callLater(() => desktopAppList.contentY = Math.min(y,
                        Math.max(0, desktopAppList.contentHeight - desktopAppList.height)));
                }

                delegate: Rectangle {
                    id: appRow
                    required property var modelData
                    required property int index
                    width: desktopAppList.width
                    height: Tokens.rowH + Tokens.s2
                    radius: Tokens.radius
                    color: modelData.onScreen ? Tokens.bone
                        : rowTap.pressed ? Tokens.tint16
                        : rowHover.hovered ? Tokens.tint5 : "transparent"
                    border.width: Tokens.border
                    border.color: modelData.onScreen ? Tokens.bone
                        : rowHover.hovered ? Tokens.lineStrong : Tokens.line
                    opacity: Persistent.ready && !Persistent.blockWrites ? 1 : 0.45
                    activeFocusOnTab: true

                    Behavior on color {
                        ColorAnimation { duration: Tokens.snap }
                    }

                    Image {
                        id: appIcon
                        anchors.left: parent.left
                        anchors.leftMargin: Tokens.s3
                        anchors.verticalCenter: parent.verticalCenter
                        width: Tokens.s6
                        height: Tokens.s6
                        sourceSize: Qt.size(width, height)
                        source: Quickshell.iconPath(AppSearch.guessIcon(appRow.modelData.id ?? ""), "image-missing")
                        fillMode: Image.PreserveAspectFit
                    }

                    Column {
                        anchors.left: appIcon.right
                        anchors.leftMargin: Tokens.s3
                        anchors.right: stateMark.left
                        anchors.rightMargin: Tokens.s3
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 1
                        Text {
                            width: parent.width
                            text: appRow.modelData.name ?? appRow.modelData.id
                            color: appRow.modelData.onScreen ? Tokens.inkOnBone : Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fRow
                            font.weight: Font.Medium
                            elide: Text.ElideRight
                        }
                        Text {
                            width: parent.width
                            visible: text.length > 0
                            text: appRow.modelData.placement
                                || appRow.modelData.genericName || appRow.modelData.comment || ""
                            color: appRow.modelData.onScreen ? Tokens.inkOnBoneDim : Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            elide: Text.ElideRight
                        }
                    }

                    Text {
                        id: stateMark
                        anchors.right: parent.right
                        anchors.rightMargin: Tokens.s3
                        anchors.verticalCenter: parent.verticalCenter
                        text: appRow.modelData.onScreen ? Translation.tr("ON") : "+"
                        color: appRow.modelData.onScreen ? Tokens.inkOnBone : Tokens.inkDim
                        font.family: appRow.modelData.onScreen ? Tokens.mono : Tokens.ui
                        font.pixelSize: appRow.modelData.onScreen ? Tokens.fTiny : Tokens.fBody
                        font.letterSpacing: appRow.modelData.onScreen ? Tokens.trackLabel : 0
                    }

                    HoverHandler {
                        id: rowHover
                        enabled: Persistent.ready && !Persistent.blockWrites
                        cursorShape: Qt.PointingHandCursor
                    }
                    TapHandler {
                        id: rowTap
                        enabled: Persistent.ready && !Persistent.blockWrites
                        onTapped: desktopAppList.toggle(appRow.modelData.id ?? "")
                    }
                    Keys.onPressed: event => {
                        if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter
                            || event.key === Qt.Key_Space) && !event.isAutoRepeat) {
                            desktopAppList.toggle(appRow.modelData.id ?? "");
                            event.accepted = true;
                        }
                    }
                }
            }

            Empty {
                anchors.centerIn: parent
                visible: desktopAppList.count === 0
                caption: Translation.tr("No applications match this search.")
            }
        }
    }

    // A category's widgets, or a query's answer: a click adds or removes,
    // a drag places the widget where the pointer is let go.
    Component {
        id: widgetListPage

        // The empty line is a SIBLING of the list, never a child of it: a
        // plain child of a ListView is parented into its content item, which
        // scrolls and is zero-sized when the model is empty - exactly the case
        // the line exists for.
        Item {
            StyledListView {
                id: widgetList
                anchors.fill: parent
                staggerStep: root.staggerStep
                clip: true
                spacing: 3
                model: root.widgetItems

                delegate: EditPanelRow {
                    id: widgetRow
                    required property var modelData
                    required property int index
                    readonly property int placed: root.widgetCount(modelData.widgetId)

                    width: widgetList.width
                    first: index === 0
                    last: index === root.widgetItems.length - 1
                    symbol: modelData.icon ?? "widgets"
                    title: modelData.name ?? modelData.widgetId
                    subtitle: modelData.description ?? ""
                    // The row always offers ANOTHER one, and says how many are
                    // out there. A checkmark would be claiming the row is a
                    // switch, and it is not one any more.
                    //
                    // Once one is placed the count becomes a stepper: the plus
                    // it already had, and the minus it never did. Taking a
                    // widget back off meant hunting the copy down on the
                    // desktop and using its own menu - fine for one, absurd for
                    // the five a stray click leaves behind, and impossible for
                    // a copy that landed under another window's worth of
                    // widgets. The minus takes the last one placed.
                    valueText: widgetRow.placed > 0 ? `×${widgetRow.placed}` : ""
                    trailingKind: widgetRow.placed > 0 ? "stepper" : "add"
                    stepDownEnabled: widgetRow.placed > 0
                    onStepDown: Config.removeLastWidgetInstance(modelData.widgetId)
                    onStepUp: root.addInstanceRequested(modelData.widgetId)
                    draggable: true
                    dragOwner: widgetList

                    onActivated: root.addInstanceRequested(modelData.widgetId)
                    onDragBegan: root.dragMetadata = modelData
                    onDragMovedTo: (x, y) => root.moveGhost(x, y)
                    onDragFinished: (x, y) => {
                        root.dragMetadata = null;
                        const p = root.toGhost(x, y);
                        root.addRequested(modelData.widgetId, p.x, p.y);
                    }
                    onDragCancelled: root.dragMetadata = null
                }
            }

            StyledText {
                anchors.centerIn: parent
                visible: widgetList.count === 0
                text: Translation.tr("No matches")
                font.pixelSize: Appearance.font.pixelSize.small
                color: Appearance.colors.colOnSurfaceVariant
            }
        }
    }

    // The bar's root: what the bar looks like, then every component it can
    // hold. One list, dropped where you want it - the three buckets this
    // replaced asked for the destination before the widget was even picked.
    Component {
        id: barRootPage

        ColumnLayout {
            spacing: 3

            EditPanelRow {
                Layout.fillWidth: true
                first: true
                last: true
                symbol: "palette"
                title: Translation.tr("Bar appearance")
                subtitle: Translation.tr("Position, size, corners and background")
                onActivated: root.openPage("appearance")
            }

            EditPanelRow {
                Layout.fillWidth: true
                Layout.topMargin: 3
                first: true
                last: true
                destructive: true
                symbol: "reset_wrench"
                title: Translation.tr("Reset the bar's layout")
                subtitle: Translation.tr("The widgets and groups the shell ships with")
                trailingKind: "none"
                onActivated: root.resetRequested("bar")
            }

            EditPanelNotice {
                Layout.fillWidth: true
                Layout.topMargin: 6
                visible: root.barCentreBlocked
                text: Translation.tr("The bar's centre group is unavailable: the Dynamic Island is drawn over it. Widgets that were there have been moved to the right group.")
            }

            EditPanelSectionLabel {
                text: Translation.tr("Widgets")
            }

            Item {
                Layout.fillWidth: true
                Layout.fillHeight: true

                StyledListView {
                    id: barList
                    anchors.fill: parent
                    staggerStep: root.staggerStep
                    clip: true
                    spacing: 3
                    model: root.barRows

                    delegate: EditPanelRow {
                        id: barRow
                        required property var modelData
                        required property int index
                        readonly property var entry: modelData.component
                        readonly property bool used: root.usedBarIds.indexOf(barRow.entry.id) !== -1

                        width: barList.width
                        first: index === 0
                        last: index === root.barRows.length - 1
                        symbol: entry.icon ?? "widgets"
                        title: entry.title ?? entry.id
                        subtitle: barRow.used ? Translation.tr("On the bar") : ""
                        trailingKind: "chevron"
                        draggable: !barRow.used
                        dragOwner: barList

                        // The row opens the widget's page, which is where its
                        // looks and its group live. Adding is the drag - or
                        // the page's own placement chips, which is the answer
                        // for a pointer that would rather not carry anything
                        // across the screen.
                        onActivated: root.openPage("component:" + barRow.entry.id)
                        onDragBegan: root.dragMetadata = {
                            "icon": barRow.entry.icon ?? "widgets",
                            "name": barRow.entry.title ?? barRow.entry.id
                        }
                        onDragMovedTo: (x, y) => {
                            root.moveGhost(x, y);
                            const p = root.toGhost(x, y);
                            root.barDragMoved(barRow.entry.id, p.x, p.y);
                        }
                        onDragFinished: (x, y) => {
                            root.dragMetadata = null;
                            const p = root.toGhost(x, y);
                            root.barDropRequested(barRow.entry.id, p.x, p.y);
                        }
                        onDragCancelled: {
                            root.dragMetadata = null;
                            root.barDragCancelled();
                        }
                    }
                }

                StyledText {
                    anchors.centerIn: parent
                    visible: barList.count === 0
                    text: Translation.tr("No matches")
                    font.pixelSize: Appearance.font.pixelSize.small
                    color: Appearance.colors.colOnSurfaceVariant
                }
            }
        }
    }

    // The same list on its own, for a query: the appearance row above belongs
    // to the root, not to a set of results.
    Component {
        id: barListPage

        Item {
            StyledListView {
                id: barSearchList
                anchors.fill: parent
                staggerStep: root.staggerStep
                clip: true
                spacing: 3
                model: root.barRows

                delegate: EditPanelRow {
                    id: barSearchRow
                    required property var modelData
                    required property int index
                    readonly property var entry: modelData.component

                    width: barSearchList.width
                    first: index === 0
                    last: index === root.barRows.length - 1
                    symbol: entry.icon ?? "widgets"
                    title: entry.title ?? entry.id
                    subtitle: root.usedBarIds.indexOf(barSearchRow.entry.id) !== -1
                        ? Translation.tr("On the bar") : ""
                    trailingKind: "chevron"
                    onActivated: root.openPage("component:" + barSearchRow.entry.id)
                }
            }

            StyledText {
                anchors.centerIn: parent
                visible: barSearchList.count === 0
                text: Translation.tr("No matches")
                font.pixelSize: Appearance.font.pixelSize.small
                color: Appearance.colors.colOnSurfaceVariant
            }
        }
    }

    Component {
        id: barAppearancePage
        EditBarAppearancePage {}
    }

    Component {
        id: barComponentPage

        EditBarComponentPage {
            id: componentPage
            componentId: root.page.startsWith("component:") ? root.page.substring(10) : ""
            onPlaceRequested: bucket => root.barPlaceRequested(componentPage.componentId, bucket)
            onRemoveRequested: root.barRemoveRequested(componentPage.componentId)
        }
    }

    // The dock's root: how it is drawn, what it carries, and the three answers
    // to "why is this app in the list".
    Component {
        id: dockRootPage

        StyledFlickable {
            id: dockRoot
            contentHeight: dockColumn.implicitHeight
            clip: true

            ColumnLayout {
                id: dockColumn
                width: dockRoot.width
                spacing: 3

                EditPanelRow {
                    visible: !PanelFamily.touchFirst
                    Layout.fillWidth: true
                    first: true
                    last: false
                    symbol: "palette"
                    title: Translation.tr("Dock design & settings")
                    subtitle: Translation.tr("Design, placement, behaviour and design details")
                    onActivated: root.openPage("appearance")
                }

                EditPanelRow {
                    visible: !PanelFamily.touchFirst
                    Layout.fillWidth: true
                    first: false
                    last: true
                    symbol: "keep"
                    title: Translation.tr("Pinned apps & order")
                    subtitle: Translation.tr("Remove pins or move them into place")
                    onActivated: root.openPage("widgets")
                }

                EditPanelRow {
                    visible: PanelFamily.touchFirst
                    Layout.fillWidth: true
                    first: true
                    last: true
                    symbol: "dock_to_bottom"
                    title: Translation.tr("Taskbar appearance & items")
                    subtitle: Translation.tr("Height, search pill, navigation and buttons")
                    onActivated: root.openPage("appearance")
                }

                EditPanelRow {
                    Layout.fillWidth: true
                    Layout.topMargin: 3
                    first: true
                    last: true
                    destructive: true
                    symbol: "reset_wrench"
                    title: PanelFamily.touchFirst ? Translation.tr("Reset the taskbar") : Translation.tr("Reset the dock")
                    subtitle: PanelFamily.touchFirst
                        ? Translation.tr("Restore tablet dock defaults")
                        : Translation.tr("The pins and the order the shell ships with")
                    trailingKind: "none"
                    onActivated: root.resetRequested("dock")
                }

                EditPanelSectionLabel {
                    text: Translation.tr("Apps")
                }

                Repeater {
                    model: root.dockGroups

                    delegate: EditPanelRow {
                        required property var modelData
                        required property int index
                        staggerIndex: index
                        Layout.fillWidth: true
                        first: index === 0
                        last: index === root.dockGroups.length - 1
                        symbol: modelData.icon
                        title: modelData.title
                        valueText: `${modelData.items.length}`
                        onActivated: root.openPage("apps:" + modelData.key)
                    }
                }

                Item {
                    Layout.fillWidth: true
                    implicitHeight: 8
                }
            }
        }
    }

    Component {
        id: dockAppListPage

        Item {
            StyledListView {
                id: appList
                anchors.fill: parent
                // Pinning moves an app between groups and rebuilds this list.
                // Entrance animations would replay the whole page on each click.
                popin: false
                animateAppearance: false
                clip: true
                spacing: 3
                model: root.dockItems

                delegate: EditPanelRow {
                    required property var modelData
                    required property int index
                    width: appList.width
                    first: index === 0
                    last: index === root.dockItems.length - 1
                    iconSource: ShellServices.Dock.iconFor(modelData.appId ?? "")
                    title: modelData.name ?? modelData.appId
                    trailingKind: modelData.pinned === true ? "check" : "add"
                    onActivated: ShellServices.Dock.togglePin(modelData.appId ?? "")
                }
            }

            StyledText {
                anchors.centerIn: parent
                visible: appList.count === 0
                text: Translation.tr("Nothing here")
                font.pixelSize: Appearance.font.pixelSize.small
                color: Appearance.colors.colOnSurfaceVariant
            }
        }
    }

    Component {
        id: dockAppearancePage
        EditDockAppearancePage {}
    }

    Component {
        id: tabletDockAppearancePage
        EditTabletDockAppearancePage {}
    }

    Component {
        id: dockWidgetsPage
        EditDockWidgetsPage {}
    }

    Component {
        id: appsListPage

        Item {
            StyledListView {
                id: appsList
                anchors.fill: parent
                popin: false
                animateAppearance: false
                clip: true
                spacing: 3
                model: root.homeAppsItems

                header: ColumnLayout {
                    width: appsList.width
                    spacing: 3
                    visible: !root.searching

                    EditPanelRow {
                        Layout.fillWidth: true
                        symbol: "splitscreen"
                        title: Translation.tr("New App Pair")
                        subtitle: Translation.tr("Split two apps side-by-side on home screen")
                        trailingKind: "chevron"
                        first: true
                        onActivated: root.openPage("createPair")
                    }

                    EditPanelRow {
                        Layout.fillWidth: true
                        symbol: "create_new_folder"
                        title: Translation.tr("New App Folder")
                        subtitle: Translation.tr("Group multiple apps into a folder")
                        trailingKind: "chevron"
                        last: true
                        onActivated: root.openPage("createFolder")
                    }

                    Item {
                        Layout.fillWidth: true
                        Layout.preferredHeight: 6
                    }
                }

                delegate: EditPanelRow {
                    required property var modelData
                    required property int index
                    width: appsList.width
                    first: index === 0
                    last: index === root.homeAppsItems.length - 1
                    iconSource: Quickshell.iconPath(AppSearch.guessIcon(modelData.id ?? ""), "image-missing")
                    title: modelData.name ?? modelData.id
                    subtitle: modelData.genericName || modelData.comment || ""
                    trailingKind: modelData.onScreen ? "check" : "add"
                    valueText: modelData.onScreen ? Translation.tr("On home") : ""
                    draggable: true
                    dragOwner: appsList
                    onActivated: root.toggleAppOnHomeScreenRequested(modelData.id ?? "")
                    onDragBegan: root.dragMetadata = { icon: "apps", name: modelData.name, appId: modelData.id }
                    onDragMovedTo: (x, y) => root.moveGhost(x, y)
                    onDragFinished: (x, y) => {
                        root.dragMetadata = null;
                        const p = root.toGhost(x, y);
                        root.addAppRequested(modelData.id ?? "", p.x, p.y);
                    }
                    onDragCancelled: root.dragMetadata = null
                }

                footer: Item {
                    width: appsList.width
                    height: clearHomeRow.height + 13
                    visible: (GlobalStates.isAppOnHomeScreenHandler !== null)

                    EditPanelRow {
                        id: clearHomeRow
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.top: parent.top
                        anchors.topMargin: 10
                        first: true
                        last: true
                        destructive: true
                        symbol: "delete_sweep"
                        title: Translation.tr("Clear home screen icons")
                        subtitle: Translation.tr("Remove all app icons from this workspace")
                        trailingKind: "none"
                        onActivated: root.clearHomeScreenAppsRequested()
                    }
                }
            }

            StyledText {
                anchors.centerIn: parent
                visible: appsList.count === 0
                text: Translation.tr("No applications found")
                color: Appearance.colors.colOnSurfaceVariant
            }
        }
    }

    Component {
        id: createPairPage

        Item {
            id: pairRoot
            property string firstAppId: ""
            property string secondAppId: ""

            ColumnLayout {
                anchors.fill: parent
                spacing: 8

                EditPanelRow {
                    Layout.fillWidth: true
                    first: true
                    last: true
                    symbol: "splitscreen"
                    title: {
                        const e1 = TaskbarApps.getCachedDesktopEntry(pairRoot.firstAppId);
                        const e2 = TaskbarApps.getCachedDesktopEntry(pairRoot.secondAppId);
                        if (e1 && e2) return e1.name + " & " + e2.name;
                        if (e1) return e1.name + " & …";
                        return Translation.tr("Select 2 apps below");
                    }
                    subtitle: !pairRoot.firstAppId
                        ? Translation.tr("1. Choose first app")
                        : (!pairRoot.secondAppId ? Translation.tr("2. Choose second app") : Translation.tr("Ready to add to home screen"))
                    trailingKind: (pairRoot.firstAppId.length > 0 || pairRoot.secondAppId.length > 0) ? "value" : "none"
                    valueText: (pairRoot.firstAppId.length > 0 || pairRoot.secondAppId.length > 0) ? Translation.tr("Reset") : ""
                    onActivated: {
                        pairRoot.firstAppId = "";
                        pairRoot.secondAppId = "";
                    }
                }

                StyledListView {
                    id: pairAppList
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    clip: true
                    spacing: 3
                    model: Array.from(AppSearch.list ?? []).filter(e => e && e.id && !e.noDisplay)

                    delegate: EditPanelRow {
                        required property var modelData
                        required property int index
                        width: pairAppList.width
                        iconSource: Quickshell.iconPath(AppSearch.guessIcon(modelData.id ?? ""), "image-missing")
                        title: modelData.name ?? modelData.id
                        trailingKind: modelData.id === pairRoot.firstAppId || modelData.id === pairRoot.secondAppId ? "check" : "add"
                        onActivated: {
                            if (!pairRoot.firstAppId) {
                                pairRoot.firstAppId = modelData.id;
                            } else if (!pairRoot.secondAppId && modelData.id !== pairRoot.firstAppId) {
                                pairRoot.secondAppId = modelData.id;
                            } else if (modelData.id === pairRoot.firstAppId) {
                                pairRoot.firstAppId = "";
                            } else if (modelData.id === pairRoot.secondAppId) {
                                pairRoot.secondAppId = "";
                            }
                        }
                    }
                }

                Item {
                    Layout.fillWidth: true
                    implicitHeight: 48
                    visible: pairRoot.firstAppId.length > 0 && pairRoot.secondAppId.length > 0

                    EditPanelRow {
                        anchors.fill: parent
                        first: true
                        last: true
                        symbol: "splitscreen"
                        title: Translation.tr("Add Pair to Home Screen")
                        trailingKind: "none"
                        onActivated: {
                            root.addAppPairRequested(pairRoot.firstAppId, pairRoot.secondAppId, "");
                            root.goBack();
                        }
                    }
                }
            }
        }
    }

    Component {
        id: createFolderPage

        Item {
            id: folderRoot
            property var selectedApps: []
            property string folderName: Translation.tr("Folder")

            ColumnLayout {
                anchors.fill: parent
                spacing: 8

                Item {
                    Layout.fillWidth: true
                    Layout.leftMargin: 6
                    Layout.rightMargin: 6
                    implicitHeight: 46

                    ToolbarTextField {
                        id: folderNameInput
                        anchors.fill: parent
                        leftPadding: 42
                        colBackground: Appearance.colors.colLayer1
                        text: folderRoot.folderName
                        placeholderText: Translation.tr("Folder name")
                        onTextChanged: folderRoot.folderName = text

                        MaterialSymbol {
                            anchors.left: parent.left
                            anchors.leftMargin: 12
                            anchors.verticalCenter: parent.verticalCenter
                            text: "folder"
                            iconSize: 22
                            color: Appearance.colors.colPrimary
                        }
                    }
                }

                StyledText {
                    Layout.fillWidth: true
                    text: Translation.tr("Select apps to include (%1 selected):").arg(folderRoot.selectedApps.length)
                    font.pixelSize: Appearance.font.pixelSize.smaller
                    color: Appearance.colors.colSubtext
                }

                StyledListView {
                    id: folderAppList
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    clip: true
                    spacing: 3
                    model: Array.from(AppSearch.list ?? []).filter(e => e && e.id && !e.noDisplay)

                    delegate: EditPanelRow {
                        required property var modelData
                        required property int index
                        width: folderAppList.width
                        iconSource: Quickshell.iconPath(AppSearch.guessIcon(modelData.id ?? ""), "image-missing")
                        title: modelData.name ?? modelData.id
                        readonly property bool isSelected: folderRoot.selectedApps.indexOf(modelData.id) !== -1
                        trailingKind: isSelected ? "check" : "none"
                        onActivated: {
                            const list = folderRoot.selectedApps.slice();
                            const idx = list.indexOf(modelData.id);
                            if (idx === -1) {
                                list.push(modelData.id);
                            } else {
                                list.splice(idx, 1);
                            }
                            folderRoot.selectedApps = list;
                        }
                    }
                }

                Item {
                    Layout.fillWidth: true
                    implicitHeight: 48
                    visible: folderRoot.selectedApps.length >= 2

                    EditPanelRow {
                        anchors.fill: parent
                        first: true
                        last: true
                        symbol: "create_new_folder"
                        title: Translation.tr("Create Folder on Home Screen")
                        trailingKind: "none"
                        onActivated: {
                            root.addFolderRequested(folderRoot.folderName.trim() || Translation.tr("Folder"), folderRoot.selectedApps);
                            root.goBack();
                        }
                    }
                }
            }
        }
    }

    // The Style catalogue: presets, the theme and the palette, with the swatch
    // grid a page down.
    Component {
        id: stylePage
        EditStylePage {
            onOpenPageRequested: page => root.openPage(page)
            onFieldFocusRequested: field => root.requestFieldFocus(field)
            onFieldFocusReleased: root.releaseFieldFocus()
        }
    }

    // The Wallpaper catalogue: this screen's picture, the colour source and
    // the framing, with the folder a page down.
    Component {
        id: wallpaperRootPage
        EditWallpaperRootPage {
            screenName: root.screenName
            onOpenPageRequested: page => root.openPage(page)
        }
    }

    // Which wallpaper the folder page sets. "wallpapers:lockscreen",
    // "wallpapers:lightmode" and "wallpapers:screen" are rows asking for their
    // own; a bare "wallpapers" follows the tab, the screen and the theme, the
    // way the card does.
    readonly property string wallpaperPageTarget: {
        if (root.page === "wallpapers:lockscreen")
            return "lockscreen";
        if (root.page === "wallpapers:lightmode")
            return "lightmode";
        if (root.page === "wallpapers:screen")
            return "screen";
        const background = Config.options.background;
        if (GlobalStates.editLockPreview && (background.useSeparateLockscreenWallpaper ?? false))
            return "lockscreen";
        if (!GlobalStates.editLockPreview && WallpaperLayout.hasOwn(root.screenName))
            return "screen";
        if ((background.useSeparateLightModeWallpaper ?? false) && !Appearance.m3colors.darkmode)
            return "lightmode";
        return "desktop";
    }

    Component {
        id: wallpaperPage
        EditWallpaperPage {
            target: root.wallpaperPageTarget
            screenName: root.screenName
        }
    }

    Component {
        id: colourPage
        EditColourPage {}
    }

    // The lock screen's own face: the switches, whatever its toolbars have
    // been asked to hide, and the way back to the desktop's layout.
    Component {
        id: lockPage

        StyledFlickable {
            id: lockRoot
            contentHeight: lockColumn.implicitHeight
            clip: true

            ColumnLayout {
                id: lockColumn
                width: lockRoot.width
                spacing: 3

                Repeater {
                    model: root.lockSwitches

                    delegate: EditPanelRow {
                        required property var modelData
                        required property int index
                        readonly property var target: root.lockSwitchTarget(modelData)
                        staggerIndex: index
                        Layout.fillWidth: true
                        first: index === 0
                        last: index === root.lockSwitches.length - 1
                        symbol: modelData.symbol
                        title: modelData.title
                        trailingKind: "switch"
                        switchChecked: target[modelData.key] ?? true
                        onActivated: target[modelData.key] = !(target[modelData.key] ?? true)
                    }
                }

                EditPanelSectionLabel {
                    visible: root.hiddenLockIslands.length > 0
                    text: Translation.tr("Hidden from the toolbars")
                }

                Repeater {
                    model: root.hiddenLockIslands

                    delegate: EditPanelRow {
                        required property string modelData
                        required property int index
                        staggerIndex: index
                        Layout.fillWidth: true
                        first: index === 0
                        last: index === root.hiddenLockIslands.length - 1
                        symbol: root.lockIslandSymbols[modelData] ?? "widgets"
                        title: root.lockIslandNames[modelData] ?? modelData
                        trailingKind: "add"
                        onActivated: root.showLockIsland(modelData)
                    }
                }

                EditPanelRow {
                    Layout.fillWidth: true
                    Layout.topMargin: 10
                    first: true
                    last: false
                    rowEnabled: root.anyLockFork
                    symbol: "reset_wrench"
                    title: Translation.tr("Use desktop layout")
                    subtitle: Translation.tr("Drop the positions this screen's lock keeps of its own")
                    trailingKind: "none"
                    onActivated: root.lockLayoutResetRequested()
                }

                EditPanelRow {
                    Layout.fillWidth: true
                    first: false
                    last: true
                    destructive: true
                    symbol: "view_agenda"
                    title: Translation.tr("Reset the islands")
                    subtitle: Translation.tr("Their order, and everything hidden from them")
                    trailingKind: "none"
                    onActivated: root.resetRequested("lockIslands")
                }

                // Clock formats, the notification list, the blur behind it,
                // fingerprint: pages of forms, and this catalogue is about
                // what sits on the lock screen, not how each part is set up.
                EditPanelRow {
                    Layout.fillWidth: true
                    Layout.topMargin: 10
                    symbol: "settings"
                    title: Translation.tr("All lock screen settings")
                    subtitle: Translation.tr("Leaves Edit Mode")
                    trailingKind: "chevron"
                    onActivated: GlobalStates.openSettingsFromEditMode("lockScreen")
                }

                Item {
                    Layout.fillWidth: true
                    implicitHeight: 8
                }
            }
        }
    }

    // The drag ghost: the row's name carried under the pointer, parented
    // outside the reveal so it is not clipped at the panel's edge.
    Rectangle {
        id: ghost
        parent: root.ghostParent ?? root
        visible: root.dragMetadata !== null
        z: 100
        width: ghostRow.implicitWidth + 24
        height: 40
        radius: Appearance.rounding.full
        color: Appearance.m3colors.m3surfaceContainer

        RowLayout {
            id: ghostRow
            anchors.centerIn: parent
            spacing: 8
            MaterialSymbol {
                text: root.dragMetadata?.icon ?? "widgets"
                iconSize: 20
                color: Appearance.colors.colOnSurface
            }
            StyledText {
                text: root.dragMetadata?.name ?? ""
                font.pixelSize: Appearance.font.pixelSize.small
                color: Appearance.colors.colOnSurface
            }
        }
    }

}

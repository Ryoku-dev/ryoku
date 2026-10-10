pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"
import "lib/store.js" as StoreLogic

Rectangle {
    id: app

    implicitWidth: 1180
    implicitHeight: 760
    color: Tokens.paper
    focus: true

    property string view: "discover"
    property string categoryID: ""
    property string query: ""
    property bool searchOpen: false
    property string providerFilter: ""
    property string pluginFilter: ""
    property string omarchySort: "popular"
    property string selectedKey: ""
    property var previewItem: null
    property var detailItem: null
    property bool detailOpen: false
    property real gridOffset: 0
    property var searchContext: null
    property var detailContext: null
    property rect detailOriginRect: Qt.rect(0, 0, 0, 0)
    property bool reducedMotion: performance.lowPowerMode || performance.reduceMotion
    readonly property bool nomarchyActive: shellSettings.barStyle === "nomarchy"
    onNomarchyActiveChanged: {
        if (nomarchyActive) {
            nomarchyRefresh.restart();
        } else if (categoryID === "omarchy-plugins"
                   || (detailItem && String(detailItem.category || "") === "omarchy-plugins")) {
            openRoute("discover");
        }
    }
    readonly property bool catalogLoading: Store.loading && Store.items.length === 0
    readonly property bool catalogError: !Store.loading && Store.items.length === 0 && Store.error !== ""
    // A nav-open (from `ryostore open` / `settings`) can arrive before the
    // catalogue's categories load, with nothing to validate the route against;
    // stash it and apply once the categories arrive so the store lands right.
    property string pendingRoute: ""
    onNavigationCategoriesChanged: if (app.pendingRoute !== "" && app.validRoute(app.pendingRoute)) app.openRoute(app.pendingRoute)

    readonly property var searchableItems: Store.items
            .filter(item => String(item.category || "") !== "omarchy-plugins" || app.nomarchyActive)
            .map(item => {
                const copy = {};
                Object.keys(item).forEach(key => copy[key] = item[key]);
                const category = Store.category(item.category);
                copy.categoryName = category ? category.name : item.category;
                copy.searchIndex = StoreLogic.searchText(copy);
                return copy;
            })
    // Decor is one tab over three picture catalogues: the decor art itself, the
    // launcher's hero art, and the fastfetch emblems. They stay separate
    // categories because they install into different folders, but the header
    // shows one plate and the subtab strip switches between them.
    readonly property var decorFamily: ["decors", "launcher-images", "fastfetch-emblems"]
    readonly property var navigationCategories: StoreLogic.sortCategories(Store.categories)
            .filter(category => Number(category.count || 0) > 0
                    && app.decorFamily.indexOf(String(category.id || "")) <= 0
                    && (String(category.id || "") !== "omarchy-plugins" || app.nomarchyActive))
    // Discover rotates daily: the day number seeds the hero pick and the order in
    // StoreLogic.collection, so the landing holds still while you browse but
    // changes each day. Absent on category/search/library views, which ignore it.
    readonly property int discoverSeed: Math.floor(Date.now() / 86400000)
    readonly property var collection: StoreLogic.collection(searchableItems, {
        view: view,
        categoryID: categoryID,
        query: query,
        provider: app.themesBrowse && app.providerFilter !== "" && app.providerFilter !== "__mine__" ? app.providerFilter : "",
        installedOnly: app.themesBrowse && app.providerFilter === "__mine__",
        pluginKind: app.pluginsBrowse ? app.pluginFilter : "",
        seed: app.discoverSeed,
        omarchySort: app.omarchySort
    })
    readonly property var selectedItem: itemForKey(selectedKey, collection)
    readonly property var resolvedDetail: detailItem
            ? itemForKey(StoreLogic.itemKey(detailItem), searchableItems) || detailItem
            : null
    readonly property int selectedIndex: indexForKey(selectedKey, collection)
    readonly property int libraryCount: StoreLogic.installed(searchableItems).length
    readonly property int updateCount: searchableItems.filter(item => item.updateAvailable === true).length
    readonly property string positionText: collection.length > 0 && selectedIndex >= 0
            ? String(selectedIndex + 1) + " / " + String(collection.length)
            : ""
    readonly property bool showHero: view === "discover" && categoryID === "" && !searchOpen && collection.length > 0
    readonly property string contentMotionKey: [
        view,
        categoryID,
        query,
        providerFilter,
        pluginFilter,
        omarchySort
    ].join("|")
    readonly property var currentCategory: categoryID !== "" ? Store.category(categoryID) : null
    readonly property string pageTitle: searchOpen ? I18n.tr("Search")
            : (view === "library" ? I18n.tr("Library")
               : (currentCategory ? I18n.tr(String(currentCategory.name || currentCategory.id))
                  : I18n.tr("Discover")))
    readonly property string pageGroup: searchOpen ? I18n.tr("SEARCH")
            : (view === "library" ? I18n.tr("YOUR STORE") : I18n.tr("BROWSE"))
    readonly property string pageDescription: searchOpen
            ? I18n.tr("Results from every category, with installed state kept in view.")
            : (view === "library"
               ? (updateCount > 0
                  ? I18n.tr("%1 installed pieces, %2 ready to update.").arg(libraryCount).arg(updateCount)
                  : I18n.tr("%1 installed pieces, all current.").arg(libraryCount))
               : (currentCategory
                  ? String(currentCategory.description || I18n.tr("Browse this collection."))
                  : I18n.tr("A changing edit of additions from across Ryoku.")))
    // The Themes category browses per provider through a subtab strip; the filter
    // narrows the collection to one provider or to the installed library.
    readonly property bool themesBrowse: app.categoryID === "colorschemes" && app.view === "discover" && !app.searchOpen
    // The Decor tab browses its three catalogues through the same strip; here a
    // plate is a category, so picking one simply routes to it.
    readonly property bool decorBrowse: app.decorFamily.indexOf(app.categoryID) >= 0
            && app.view === "discover" && !app.searchOpen
    readonly property var decorTabs: {
        var out = [];
        var cats = Store.categories;
        for (var i = 0; i < app.decorFamily.length; i++) {
            for (var j = 0; j < cats.length; j++) {
                if (String(cats[j].id) !== app.decorFamily[i] || Number(cats[j].count || 0) <= 0)
                    continue;
                out.push({ "key": String(cats[j].id), "label": String(cats[j].name || cats[j].id) });
            }
        }
        return out;
    }
    // The Plugins category browses through the same strip: ALL / BAR / DESKTOP,
    // where BAR is the plugins hosted on the bar (topbarGlyph) and DESKTOP is
    // everything else. The filter narrows the collection by StoreLogic.pluginKind.
    readonly property bool pluginsBrowse: app.categoryID === "plugins" && app.view === "discover" && !app.searchOpen
    readonly property var pluginTabs: [
        { "key": "bar", "label": I18n.tr("BAR") },
        { "key": "desktop", "label": I18n.tr("DESKTOP") }
    ]
    readonly property bool omarchyPluginsBrowse: app.categoryID === "omarchy-plugins"
            && app.view === "discover" && !app.searchOpen
    readonly property var omarchySortTabs: [
        { "key": "popular", "label": I18n.tr("POPULAR") },
        { "key": "new", "label": I18n.tr("NEW") },
        { "key": "verified", "label": I18n.tr("VERIFIED") }
    ]
    readonly property var themeProviders: {
        var seen = ({});
        var out = [];
        var its = Store.items;
        for (var i = 0; i < its.length; i++) {
            if (its[i].category !== "colorschemes")
                continue;
            var pv = (its[i].metadata && its[i].metadata.provider) ? its[i].metadata.provider : "Community";
            if (!seen[pv]) { seen[pv] = true; out.push(pv); }
        }
        out.sort();
        return out;
    }
    readonly property int themeInstallable: {
        if (!app.themesBrowse || app.providerFilter === "" || app.providerFilter === "__mine__")
            return 0;
        var n = 0;
        var its = Store.items;
        for (var i = 0; i < its.length; i++) {
            var it = its[i];
            if (it.category !== "colorschemes")
                continue;
            var pv = (it.metadata && it.metadata.provider) ? it.metadata.provider : "Community";
            if (pv === app.providerFilter && it.installed !== true && it.downloadPaused !== true
                    && it.unavailable !== true)
                n++;
        }
        return n;
    }

    function itemForKey(key, items) {
        const source = Array.isArray(items) ? items : [];
        for (let i = 0; i < source.length; i++)
            if (StoreLogic.itemKey(source[i]) === key)
                return source[i];
        return null;
    }

    function indexForKey(key, items) {
        const source = Array.isArray(items) ? items : [];
        for (let i = 0; i < source.length; i++)
            if (StoreLogic.itemKey(source[i]) === key)
                return i;
        return -1;
    }
    function providerItems(provider) {
        var out = [];
        var its = Store.items;
        for (var i = 0; i < its.length; i++) {
            var it = its[i];
            if (it.category !== "colorschemes")
                continue;
            var pv = (it.metadata && it.metadata.provider) ? it.metadata.provider : "Community";
            if (pv === provider)
                out.push(it);
        }
        return out;
    }

    function reconcileSelection(fallbackIndex) {
        selectedKey = StoreLogic.selectionKey(collection, selectedKey,
                                                fallbackIndex === undefined ? 0 : fallbackIndex);
    }

    function validRoute(route) {
        if (route === "discover" || route === "library")
            return true;
        if (route === "omarchy-plugins" && !app.nomarchyActive)
            return false;
        return Store.categories.some(category => category.id === route);
    }

    function currentFocusObject() {
        return app.Window.window ? app.Window.window.activeFocusItem : null;
    }

    function snapshotContext() {
        return {
            view: view,
            categoryID: categoryID,
            query: query,
            selectedKey: selectedKey,
            gridOffset: productGrid.contentY,
            focusObject: currentFocusObject()
        };
    }

    function restoreContext(context) {
        if (!context)
            return;
        view = context.view;
        categoryID = context.categoryID;
        query = context.query;
        selectedKey = context.selectedKey;
        gridOffset = context.gridOffset;
        Qt.callLater(function() {
            app.reconcileSelection(0);
            Qt.callLater(function() {
                productGrid.restoreOffset(context.gridOffset);
                app.gridOffset = productGrid.contentY;
                if (context.focusObject && context.focusObject.forceActiveFocus)
                    context.focusObject.forceActiveFocus();
                else
                    productGrid.forceActiveFocus();
            });
        });
    }

    function openRoute(route) {
        if (!validRoute(route)) {
            app.pendingRoute = route;
            return;
        }
        app.pendingRoute = "";
        app.providerFilter = "";
        app.pluginFilter = "";
        app.omarchySort = "popular";
        detailClear.stop();
        detailOpen = false;
        detailItem = null;
        detailContext = null;
        searchOpen = false;
        searchContext = null;
        query = "";
        previewItem = null;
        if (route === "discover" || route === "library") {
            view = route;
            categoryID = "";
        } else {
            view = "discover";
            categoryID = route;
        }
        reconcileSelection(0);
        Qt.callLater(function() { productGrid.forceActiveFocus(); });
    }

    function selectKey(key) {
        selectedKey = StoreLogic.selectionKey(collection, key, 0);
        previewItem = null;
    }

    function selectedCoverRect() {
        const rect = productGrid.cellRectFor(selectedKey);
        if (rect.width === 0)
            return Qt.rect(0, 0, 0, 0);
        const point = productGrid.mapToItem(productDetail, rect.x, rect.y);
        return Qt.rect(point.x, point.y, rect.width, rect.height);
    }

    function openSelectedDetail() {
        if (!selectedItem)
            return;
        detailClear.stop();
        detailContext = snapshotContext();
        detailOriginRect = selectedCoverRect();
        detailItem = selectedItem;
        detailOpen = true;
        previewItem = null;
        Qt.callLater(function() { productDetail.focusInitialAction(); });
    }

    function closeDetail() {
        if (!detailOpen)
            return;
        const context = detailContext;
        detailOpen = false;
        detailContext = null;
        restoreContext(context);
        if (reducedMotion)
            detailItem = null;
        else
            detailClear.restart();
    }

    function enterSearch() {
        if (searchOpen)
            return;
        searchContext = snapshotContext();
        searchOpen = true;
        previewItem = null;
        reconcileSelection(0);
    }

    function searchFor(value) {
        enterSearch();
        query = value;
        previewItem = null;
        reconcileSelection(0);
    }

    function exitSearch() {
        if (!searchOpen)
            return;
        const context = searchContext;
        searchOpen = false;
        searchContext = null;
        query = "";
        restoreContext(context);
    }

    function escapeLayer() {
        if (detailOpen && productDetail.lightboxOpen)
            productDetail.closeLightbox();
        else if (detailOpen)
            closeDetail();
        else if (searchOpen)
            exitSearch();
        else if (view !== "discover" || categoryID !== "")
            openRoute("discover");
    }

    function requestQuit() {
        // An in-flight install runs a backend transaction that is not safe to
        // interrupt: poll until the store is idle, then quit.
        if (Store.busyKey !== "") {
            if (!quitArm.running)
                quitArm.start();
            return;
        }
        quitArm.stop();
        // Stop our child processes and defer the quit past this event, so the QML
        // engine tears down with nothing in flight. Quitting straight out of the
        // window close handler races that teardown into a pure-virtual crash.
        Store.shutdown();
        Qt.callLater(Qt.quit);
    }

    onCollectionChanged: reconcileSelection(0)
    onSearchableItemsChanged: {
        reconcileSelection(0);
        if (detailItem)
            detailItem = itemForKey(StoreLogic.itemKey(detailItem), searchableItems) || detailItem;
    }
    Component.onCompleted: Qt.callLater(function() { productGrid.forceActiveFocus(); })

    Keys.onEscapePressed: event => {
        escapeLayer();
        event.accepted = true;
    }
    Keys.onPressed: event => {
        if (event.text === "/" && event.modifiers === Qt.NoModifier) {
            header.focusSearch();
        } else if (!detailOpen && (event.key === Qt.Key_Return || event.key === Qt.Key_Enter)) {
            openSelectedDetail();
        } else {
            return;
        }
        event.accepted = true;
    }

    Shortcut { sequence: "Ctrl+K"; onActivated: header.focusSearch() }
    Shortcut { sequence: "Ctrl+Q"; onActivated: app.requestQuit() }

    Timer { id: quitArm; interval: 400; repeat: true; onTriggered: app.requestQuit() }
    Timer {
        id: detailClear
        interval: Tokens.swap
        onTriggered: app.detailItem = null
    }
    Timer {
        id: nomarchyRefresh
        interval: 100
        onTriggered: {
            if (Store.loading) {
                restart();
                return;
            }
            const category = Store.category("omarchy-plugins");
            if (category && Number(category.count || 0) > 0)
                return;
            Store.refresh(true);
        }
    }

    FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config"))
              + "/ryoku/shell.json"
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        JsonAdapter {
            id: shellSettings
            property string barStyle: "qsbar"
        }
    }

    FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config"))
              + "/ryoku/performance.json"
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        JsonAdapter {
            id: performance
            property bool lowPowerMode: false
            property bool reduceMotion: false
        }
    }

    StoreHeader {
        id: header
        objectName: "ryostore-header"
        anchors { left: parent.left; top: parent.top; bottom: parent.bottom }
        width: implicitWidth
        view: app.view
        categoryID: app.categoryID
        categories: app.navigationCategories
        query: app.query
        libraryCount: app.libraryCount
        updateCount: app.updateCount
        updateAvailable: Store.updateAvailable
        offline: Store.offline
        refreshing: Store.refreshing
        searchActive: app.searchOpen
        resultCount: app.collection.length
        reducedMotion: app.reducedMotion
        onRouteRequested: (routeView, routeCategory) => app.openRoute(routeCategory || routeView)
        onRefreshRequested: Store.refresh(true)
        onQueryEdited: value => app.searchFor(value)
        onSearchActivated: app.enterSearch()
        onSearchEscaped: app.exitSearch()
    }

    Item {
        id: pageHead
        objectName: "ryostore-page-head"
        anchors { left: header.right; top: parent.top; right: parent.right }
        height: 116

        Column {
            anchors {
                left: parent.left; leftMargin: Tokens.s6
                right: headReadout.left; rightMargin: Tokens.s5
                verticalCenter: parent.verticalCenter
            }
            spacing: Tokens.s1

            Text {
                text: "力  " + app.pageGroup
                color: Tokens.inkDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
            }
            Text {
                width: parent.width
                text: app.pageTitle
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Tokens.fTitle
                font.weight: Font.Medium
                elide: Text.ElideRight
            }
            Text {
                width: Math.min(parent.width, 680)
                text: app.pageDescription
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                maximumLineCount: 2
                wrapMode: Text.Wrap
                elide: Text.ElideRight
            }
        }

        Column {
            id: headReadout
            anchors { right: parent.right; rightMargin: Tokens.s6; verticalCenter: parent.verticalCenter }
            spacing: Tokens.s1
            Text {
                anchors.right: parent.right
                text: String(app.collection.length).padStart(2, "0")
                color: Tokens.ink
                font.family: Tokens.mono
                font.pixelSize: Tokens.fValue
            }
            Text {
                anchors.right: parent.right
                text: Store.offline ? I18n.tr("OFFLINE CACHE") : I18n.tr("PIECES")
                color: Store.offline ? Tokens.alert : Tokens.inkMuted
                font.family: Tokens.mono
                font.pixelSize: Tokens.fTiny
                font.letterSpacing: Tokens.trackLabel
            }
        }

        Rectangle {
            anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
            height: Tokens.border
            color: Tokens.line
        }
    }
    ProviderTabs {
        id: providerTabs
        objectName: "ryostore-provider-tabs"
        anchors { left: header.right; top: pageHead.bottom; right: parent.right }
        readonly property bool shown: app.themesBrowse || app.decorBrowse || app.pluginsBrowse
                || app.omarchyPluginsBrowse
        height: shown ? implicitHeight : 0
        visible: shown
        providers: app.themesBrowse ? app.themeProviders
                   : (app.pluginsBrowse ? app.pluginTabs
                      : (app.omarchyPluginsBrowse ? app.omarchySortTabs : app.decorTabs))
        active: app.themesBrowse ? app.providerFilter
                : (app.pluginsBrowse ? app.pluginFilter
                   : (app.omarchyPluginsBrowse ? app.omarchySort : app.categoryID))
        // Themes and Plugins each browse one catalogue, so both offer an All
        // plate; Themes also offers the installed library, while Decor's plates
        // are whole catalogues so it offers neither.
        allLabel: app.themesBrowse || app.pluginsBrowse ? I18n.tr("ALL") : ""
        trailingLabel: app.themesBrowse ? I18n.tr("MY THEMES") : ""
        trailingKey: app.themesBrowse ? "__mine__" : ""
        installableCount: app.themesBrowse ? app.themeInstallable : 0
        busy: Store.busyKey !== ""
        reducedMotion: app.reducedMotion
        onPicked: filter => {
            if (app.decorBrowse) {
                app.openRoute(filter);
                Qt.callLater(function() { productGrid.forceActiveFocus(); });
                return;
            }
            if (app.pluginsBrowse) {
                app.pluginFilter = filter;
                app.reconcileSelection(0);
                Qt.callLater(function() { productGrid.forceActiveFocus(); });
                return;
            }
            if (app.omarchyPluginsBrowse) {
                app.omarchySort = filter;
                app.reconcileSelection(0);
                Qt.callLater(function() { productGrid.forceActiveFocus(); });
                return;
            }
            app.providerFilter = filter;
            app.reconcileSelection(0);
            Qt.callLater(function() { productGrid.forceActiveFocus(); });
        }
        onInstallAll: Store.installAll(app.providerItems(app.providerFilter))
    }

    ShowroomStage {
        id: stage
        objectName: "ryostore-stage"
        anchors { left: header.right; top: providerTabs.bottom; right: parent.right }
        height: app.showHero
                ? Math.min(300, Math.max(240, Math.round((actionBar.y - providerTabs.y) * 0.42))) : 0
        visible: app.showHero
        enabled: !app.detailOpen
        item: app.selectedItem
        previewItem: app.previewItem
        busyKey: Store.busyKey
        installStage: Store.installStage
        installErrorKey: Store.installErrorKey
        installError: Store.installError
        positionText: app.positionText
        offline: Store.offline
        reducedMotion: app.reducedMotion
        onInstallRequested: item => Store.install(item)
        onDetailsRequested: item => app.openSelectedDetail()
        onSettingsRequested: item => Store.openSettings(item)
        onRemoveRequested: item => Store.remove(item)
    }

    ProductGrid {
        id: productGrid
        objectName: "ryostore-grid"
        anchors {
            left: header.right; leftMargin: Tokens.s6
            top: stage.bottom
            right: parent.right; rightMargin: Tokens.s6
            bottom: actionBar.top
        }
        items: app.collection
        selectedKey: app.selectedKey
        reducedMotion: app.reducedMotion
        contentKey: app.contentMotionKey
        enabled: !app.detailOpen
        onPreviewRequested: item => app.previewItem = item
        onSelectionRequested: item => app.selectKey(StoreLogic.itemKey(item))
        onActivated: item => {
            app.selectKey(StoreLogic.itemKey(item));
            app.openSelectedDetail();
        }
    }
    Item {
        id: actionBar
        objectName: "ryostore-action-bar"
        anchors { left: header.right; right: parent.right; bottom: parent.bottom }
        height: 42

        Rectangle {
            anchors { left: parent.left; right: parent.right; top: parent.top }
            height: Tokens.border
            color: Tokens.line
        }
        Text {
            anchors { left: parent.left; leftMargin: Tokens.s6; verticalCenter: parent.verticalCenter }
            text: Store.busyKey !== ""
                    ? I18n.tr(Store.installStage)
                    : (app.view === "library" && app.updateCount > 0
                       ? I18n.tr("%1 UPDATES READY").arg(app.updateCount)
                       : I18n.tr("READY"))
            color: Store.busyKey !== "" || app.updateCount > 0 ? Tokens.ink : Tokens.inkMuted
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro
            font.letterSpacing: Tokens.trackLabel
        }
        Text {
            anchors { right: parent.right; rightMargin: Tokens.s6; verticalCenter: parent.verticalCenter }
            text: I18n.tr("ARROWS BROWSE  /  ENTER OPEN  /  ESC BACK")
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fTiny
            font.letterSpacing: Tokens.trackLabel
        }
    }

    // initial catalogue fetch: show progress, never the empty plate, so a slow
    // network never reads as "there is nothing here".
    Grid {
        id: loadingState
        anchors.fill: productGrid
        columns: productGrid.columns
        visible: app.catalogLoading
        z: 2

        Repeater {
            model: 6
            delegate: Rectangle {
                required property int index
                width: productGrid.cellW
                height: productGrid.cellH
                color: Tokens.paper
                border.width: Tokens.border
                border.color: Tokens.line

                Rectangle {
                    anchors { left: parent.left; right: parent.right; top: parent.top }
                    height: Math.round(parent.height * 0.67)
                    color: Tokens.paperLift
                    Rectangle {
                        width: parent.width * 0.24
                        height: parent.height
                        color: Tokens.tint5
                        x: app.reducedMotion || !app.Window.window || !app.Window.window.active
                                ? (parent.width - width) / 2 : -width
                        XAnimator on x {
                            from: -parent.width * 0.24
                            to: parent.width
                            duration: 1100
                            loops: Animation.Infinite
                            running: loadingState.visible && !app.reducedMotion
                                    && app.Window.window && app.Window.window.active
                        }
                    }
                }
                Rectangle {
                    x: Tokens.s3
                    y: Math.round(parent.height * 0.67) + Tokens.s3
                    width: parent.width * 0.52
                    height: Tokens.border * 4
                    color: Tokens.line
                }
                Rectangle {
                    x: Tokens.s3
                    y: Math.round(parent.height * 0.67) + Tokens.s5
                    width: parent.width * 0.72
                    height: Tokens.border * 3
                    color: Tokens.lineSoft
                }
            }
        }
    }

    // catalogue source failed with nothing cached to fall back on.
    Column {
        anchors.centerIn: productGrid
        spacing: Tokens.s3
        visible: app.catalogError
        z: 2

        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            text: I18n.tr("CATALOGUE UNAVAILABLE")
            color: Tokens.ink
            font.family: Tokens.mono
            font.pixelSize: Tokens.fSmall
            font.letterSpacing: Tokens.trackLabel
        }

        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            width: Math.min(productGrid.width - Tokens.s7 * 2, 420)
            text: Store.error
            visible: text !== ""
            horizontalAlignment: Text.AlignHCenter
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.Wrap
            maximumLineCount: 3
            elide: Text.ElideRight
        }

        Btn {
            anchors.horizontalCenter: parent.horizontalCenter
            text: I18n.tr("RETRY")
            armed: true
            onAct: Store.refresh(true)
            Accessible.role: Accessible.Button
            Accessible.name: text
            Accessible.onPressAction: Store.refresh(true)
        }
    }

    Column {
        anchors.centerIn: productGrid
        spacing: Tokens.s4
        visible: app.collection.length === 0 && !app.catalogLoading && !app.catalogError
        z: 2

        Empty {
            anchors.horizontalCenter: parent.horizontalCenter
            caption: app.view === "library"
                    ? I18n.tr("Installed pieces appear here, with updates called out in place.")
                    : (app.query !== ""
                       ? I18n.tr("No products match this search.")
                       : I18n.tr("Nothing has been published to this collection yet."))
        }

        Btn {
            anchors.horizontalCenter: parent.horizontalCenter
            text: I18n.tr("RETURN TO DISCOVER")
            visible: app.view === "library" || app.query !== ""
            armed: visible
            onAct: app.openRoute("discover")
            Accessible.role: Accessible.Button
            Accessible.name: text
            Accessible.onPressAction: app.openRoute("discover")
        }
    }

    ProductDetail {
        id: productDetail
        objectName: "ryostore-detail"
        anchors { left: header.right; top: parent.top; right: parent.right; bottom: parent.bottom }
        z: 20
        item: app.resolvedDetail
        open: app.detailOpen
        originRect: app.detailOriginRect
        busyKey: Store.busyKey
        installStage: Store.installStage
        installErrorKey: Store.installErrorKey
        installError: Store.installError
        reducedMotion: app.reducedMotion
        onCloseRequested: app.closeDetail()
        onInstallRequested: (item, dither, components) => Store.install(item, dither, components)
        onRetryRequested: (item, dither, components) => Store.retryInstall(item, dither, components)
        onSettingsRequested: item => Store.openSettings(item)
        onRemoveRequested: item => Store.remove(item)
        onEnabledRequested: (item, enabled) => Store.setOmarchyEnabled(item, enabled)
    }
}

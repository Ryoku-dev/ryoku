import QtQuick
import Quickshell
import Quickshell.Wayland
import stage
import stage.services
import stage.modules.common
import stage.modules.common.functions
import stage.modules.common.widgets
import stage.modules.ii.editMode
import shell.services as ShellServices

/**
 * One screen's worth of Edit Mode's chrome: a full-screen layer surface that is
 * transparent everywhere except the toolbar on it.
 *
 * Why not on the background surface: the desktop stays where it is - the
 * wallpaper and the widget canvas are already there - but those surfaces are
 * on the Background and Bottom layers, under the bar and the dock, which stay
 * in place at full size. Chrome drawn there would sit under the bar. So the
 * chrome takes a surface of its own on Overlay, and the desktop does not move.
 *
 * Three things a surface this size has to get right:
 *
 * Input. A screen-sized surface that accepts input everywhere makes the
 * desktop underneath unclickable - and the desktop underneath is the thing
 * being edited. The mask is the toolbar and nothing else.
 *
 * Blur. rules.lua's catch-all blurs every `quickshell:*` surface with a low
 * alpha threshold, under which a screen of transparent pixels asks the
 * compositor to blur the whole screen. The namespace is minted AND listed there
 * at `ignore_alpha = 1`, so only the toolbar's opaque body is blurred.
 *
 * Keyboard. None, deliberately: Escape and the arrows are answered by the
 * WidgetCanvas on the widgets surface, and a chrome surface taking OnDemand
 * focus would sit in front of it and swallow the keys. The one exception is
 * the catalogue's search field, which cannot be typed into without it: the
 * surface takes OnDemand focus for exactly as long as that field holds it, and
 * the field's own Escape empties it and then lets go.
 */
PanelWindow {
    id: root

    // Whether something is summoned over the desktop this chrome frames - a
    // special workspace, today. Under it the chrome drops to the desktop's own
    // layer, so the compositor blurs and dims both halves of the mode together
    // instead of painting the toolbar over the window.
    property bool underneath: false

    color: "transparent"
    WlrLayershell.namespace: "quickshell:editMode"
    WlrLayershell.layer: root.underneath ? WlrLayer.Bottom : WlrLayer.Overlay
    // None except while the catalogue's search field wants the keyboard: see
    // the Keyboard note above.
    //
    // Exclusive, and HELD exclusive, for as long as the field holds it.
    // OnDemand is not enough on its own: a surface at None is not focusable at
    // the instant of the click that reaches into it, so the first keystrokes
    // would go wherever the keyboard already was. And downgrading to OnDemand
    // afterwards - the two-step the sidebars map with
    // ([[layershell-keyboardfocus-steals-pointer]]) - only survives while the
    // cursor is over this surface: the downgrade makes Hyprland re-evaluate
    // pointer focus at the real cursor position, and the desktop's canvas,
    // which asks OnDemand throughout the mode, takes the keyboard straight
    // back. That is invisible for a click on the field and fatal for Ctrl+F,
    // where the cursor is wherever the user left it.
    //
    // Holding it means no click can take the keyboard away by itself, so the
    // field is released explicitly instead: by the catcher in the chrome for a
    // click on this surface, and by the canvas for one on the desktop.
    // The catalogue's search field, or a widget's options panel with a text
    // field in it: either wants the keyboard on this surface, and neither can
    // have it by asking the compositor on its own.
    readonly property bool searchFocused: chrome.drawerSearchFocused || root.menuWantsKeyboard
    readonly property bool menuWantsKeyboard: menuLoader.item ? (menuLoader.item.wantsKeyboard ?? false) : false
    // Published so the desktop's canvas stands down while the field types and
    // takes the keyboard back the moment it lets go.
    onSearchFocusedChanged: GlobalStates.editSearchFocused = root.searchFocused
    WlrLayershell.keyboardFocus: root.searchFocused ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
    exclusionMode: ExclusionMode.Ignore
    exclusiveZone: 0

    // All four edges and no margins, so this window's coordinate space is the
    // screen's. On a layer surface position IS margins, so a toolbar animating
    // into place through them would reconfigure the surface every frame; the
    // chrome moves inside the surface instead.
    anchors {
        top: true
        bottom: true
        left: true
        right: true
    }

    readonly property string screenName: root.screen ? root.screen.name : ""
    // The same pure function, on the same inputs, that the two desktop
    // surfaces build their transform out of - re-derived rather than published
    // across the window boundary, because every input is available here.
    readonly property var viewport: EditModeInsets.viewportFor(root.screenName, root.width, root.height)
    readonly property real progress: GlobalStates.editProgress
    readonly property real editShift: EditModeLogic.drawerTravel(root.viewport) * GlobalStates.editDrawerProgress
    readonly property var cardGeometry: EditModeLogic.cardRect(root.viewport, root.progress, root.width, root.height, root.editShift)
    readonly property var drawerGeometry: EditModeLogic.drawerRect(root.viewport, root.progress, GlobalStates.editDrawerProgress, root.width, root.height)
    readonly property var areaGeometry: EditModeLogic.areaRect(root.viewport, root.progress, root.width, root.height)

    // Whether the one widget menu belongs to this screen.
    readonly property bool menuOpenHere: GlobalStates.editWidgetMenuOpen && GlobalStates.editWidgetMenuScreenName === root.screenName
    readonly property bool barMenuOpenHere: GlobalStates.editBarMenuOpen && GlobalStates.editBarMenuScreenName === root.screenName

    readonly property bool barHoverHere: GlobalStates.editBarHoverShown && GlobalStates.editBarHoverScreenName === root.screenName

    // A point in the bar window's coordinates, in this surface's. The bar sits
    // flush with one screen edge, so the far edges translate by the window's
    // own size.
    function fromBarWindow(x, y, windowWidth, windowHeight) {
        const side = EditModeInsets.barSide;
        if (side === "bottom")
            return Qt.point(x, y + root.height - windowHeight);
        if (side === "right")
            return Qt.point(x + root.width - windowWidth, y);
        return Qt.point(x, y);
    }

    function barMenuPoint() {
        const from = root.fromBarWindow(GlobalStates.editBarMenuX, GlobalStates.editBarMenuY,
            GlobalStates.editBarMenuWindowWidth, GlobalStates.editBarMenuWindowHeight);
        let x = from.x;
        let y = from.y;
        const side = EditModeInsets.barSide;
        // Off the bar's body, so the card never covers the widgets it is about.
        const clear = EditModeInsets.barThickness + 4;
        if (side === "top")
            y = Math.max(y, clear);
        else if (side === "bottom")
            y = Math.min(y, root.height - clear) - 80;
        else if (side === "left")
            x = Math.max(x, clear);
        else
            x = Math.min(x, root.width - clear) - 200;
        return Qt.point(x, y);
    }

    // The toolbar, plus - while a menu is open - the whole screen: a click
    // anywhere that is not the menu dismisses it before it reaches the desktop.
    // The closer's region is zero-sized when there is no menu, so the desktop
    // gets every other click.
    mask: Region {
        item: chrome.toolbarItem
        Region {
            item: menuCloser
        }
        // A closed drawer is a zero-width item and contributes nothing.
        Region {
            item: chrome.drawerItem
        }
        // The guide's card, so its own buttons are clickable. Hidden, it is
        // still an item with a size, so the region follows its visibility.
        Region {
            item: chrome.guideItem
        }
    }

    // The drawer's reveal, handed to the surface that owns the desktop: a
    // widget dragged back into the drawer is removed, and the widget deciding
    // that is on another layer surface. Removed with the surface, so the map
    // always reads "the screens whose drawer exists".
    readonly property rect drawerReveal: chrome.drawer
    onDrawerRevealChanged: root.publishDrawerReveal(root.drawerReveal)
    Component.onCompleted: {
        root.publishDrawerReveal(root.drawerReveal);
        // The style history's baseline: what the wallpaper, palette and theme
        // were when the mode opened over this screen.
        root.styleSnapshot();
        // The mode has just opened over this screen: if the island owns the
        // bar's centre, take whatever is stored there out before the user can
        // see a catalogue that claims it is placed.
        root.evictBarCentre();
    }
    Component.onDestruction: {
        root.publishDrawerReveal(null);
        if (root.searchFocused)
            GlobalStates.editSearchFocused = false;
    }
    function publishDrawerReveal(reveal) {
        if (root.screenName === "")
            return;
        const published = Object.assign({}, GlobalStates.editDrawerReveals);
        if (reveal === null)
            delete published[root.screenName];
        else
            published[root.screenName] = { "x": reveal.x, "y": reveal.y, "width": reveal.width, "height": reveal.height };
        GlobalStates.editDrawerReveals = published;
    }

    // A desktop widget let go over the drawer leaves the desktop. Answered
    // here and not on the widget: every store the mode writes is written from
    // the chrome, and there is one chrome (decision D4), so one answer.
    Connections {
        target: GlobalStates
        function onEditWidgetDroppedOnDrawer(instanceId) {
            // By INSTANCE. It used to look the entry up and then remove by
            // kind, which took the first copy rather than the one that was
            // dropped - invisible while a widget could only be placed once,
            // and wrong the moment it could be placed twice.
            Config.removeWidgetInstance(instanceId);
        }
    }

    // A drawer row dropped on the card: the screen point becomes a canvas
    // point through the inverse of the desktop's transform, snapped to the
    // canvas's grid and kept inside it. A release back over the drawer, or
    // outside the card, is the gesture being abandoned.
    function addWidgetAt(widgetId, dropX, dropY) {
        if (EditModeLogic.pointInDrawerReveal(chrome.drawer, dropX, dropY))
            return;
        const card = root.cardGeometry;
        if (dropX < card.x || dropX > card.x + card.width || dropY < card.y || dropY > card.y + card.height)
            return;
        const p = EditModeLogic.canvasPointFromScreen(root.viewport, root.progress, root.editShift, dropX, dropY);
        const placed = EditModeLogic.dropPosition({
            "gridSize": Config.widgetProvider ? Config.widgetProvider.gridSize : 10,
            "canvasX": p.x,
            "canvasY": p.y,
            "screenWidth": root.width,
            "screenHeight": root.height
        });
        if (GlobalStates.editLockPreview) {
            root.placeOnLock(widgetId, placed.x, placed.y);
            return;
        }
        Config.addWidgetToDesktop(widgetId, placed.x, placed.y, root.screenName, "hide");
    }

    // The same drop on the Lockscreen tab: a new lock-only instance where it
    // was let go.
    //
    // This used to look for an existing copy and fork ITS lock placement
    // instead, because a widget could only be placed once and the add would
    // have returned early. With copies allowed, "which copy did you mean" has
    // no answer the drop could give, and the honest reading of dragging a
    // widget out of the catalogue is the same on both tabs: one more of it.
    // Forking a copy that already exists is still the drag ON the widget,
    // which writes the lock fork directly.
    function placeOnLock(widgetId, x, y) {
        Config.addWidgetToDesktop(widgetId, x, y, root.screenName, "lockOnly");
    }

    // A catalogue row's click: one more of this widget. On the Lockscreen tab
    // the new copy is lock-only, which is what the tab is for.
    //
    // It used to toggle - add if absent, remove if present, and cycle the lock
    // behaviour on the other tab - which is the only thing a catalogue CAN do
    // while a widget is a yes/no. Removal is now the widget's own affordance:
    // its menu, or dragging it back into the panel, both of which say which
    // copy they mean.
    function addWidgetInstance(widgetId) {
        Config.addWidgetToDesktop(widgetId, undefined, undefined, root.screenName,
            GlobalStates.editLockPreview ? "lockOnly" : "hide");
    }

    function addAppAt(appId, dropX, dropY) {
        if (EditModeLogic.pointInDrawerReveal(chrome.drawer, dropX, dropY))
            return;
        const card = root.cardGeometry;
        if (dropX < card.x || dropX > card.x + card.width || dropY < card.y || dropY > card.y + card.height)
            return;
        const p = EditModeLogic.canvasPointFromScreen(root.viewport, root.progress, root.editShift, dropX, dropY);
        if (GlobalStates.addAppToHomeScreenHandler)
            GlobalStates.addAppToHomeScreenHandler(appId, p.x, p.y);
    }

    function toggleAppOnHomeScreen(appId) {
        if (!GlobalStates.isAppOnHomeScreenHandler)
            return;
        if (GlobalStates.isAppOnHomeScreenHandler(appId)) {
            if (GlobalStates.removeAppFromHomeScreenHandler)
                GlobalStates.removeAppFromHomeScreenHandler(appId);
        } else {
            if (GlobalStates.addAppToHomeScreenHandler)
                GlobalStates.addAppToHomeScreenHandler(appId);
        }
    }

    function clearHomeScreenApps() {
        if (GlobalStates.clearHomeScreenAppsHandler)
            GlobalStates.clearHomeScreenAppsHandler();
    }

    function addAppPair(firstAppId, secondAppId, name) {
        if (GlobalStates.addAppPairToHomeScreenHandler)
            GlobalStates.addAppPairToHomeScreenHandler(firstAppId, secondAppId, name);
    }

    function addFolder(name, appsList) {
        if (GlobalStates.addFolderToHomeScreenHandler)
            GlobalStates.addFolderToHomeScreenHandler(name, appsList);
    }

    // ── The bar's centre while the Dynamic Island owns it ────────────────────
    // `bar.floatingNotch.centerInBar` makes BarLayout hand the centre section
    // an empty list: the island is drawn over that stretch of bar and anything
    // stored there renders nowhere. Nothing told Edit Mode, so the centre was
    // still a drop target and still counted in the catalogue - a widget put
    // there was written to the config, reported as placed, and invisible.
    //
    // Two halves. The controller refuses the centre as a target for as long as
    // the flag is on (BarEditController.centreBlocked); this is the other one -
    // whatever is ALREADY stored there is moved out, once, when the mode opens
    // over a bar in that state, and again if the flag is turned on while the
    // mode is up. They go to the FRONT of the right-hand list, which is the
    // place closest to where they were drawn, in their own order, as one
    // history entry - so Ctrl+Z puts the centre back exactly as it was for
    // anyone who turns the island off again.
    readonly property bool barCentreBlocked: ShellModePolicy.barCenterActive
    onBarCentreBlockedChanged: root.evictBarCentre()

    function evictBarCentre() {
        if (!root.barCentreBlocked || !GlobalStates.editMode)
            return;
        const layouts = Config.options.bar.layouts;
        if (!layouts)
            return;
        const centre = EditModeLogic.listCopy(layouts.center ?? []);
        if (centre.length === 0)
            return;
        const right = EditModeLogic.listCopy(layouts.right ?? []);
        const moved = centre
            .filter(entry => entry && entry.id)
            .map(entry => ({ "id": entry.id, "centered": false, "visible": entry.visible !== false }))
            // A component belongs to one list at a time, so an id the right
            // list somehow already holds is dropped rather than doubled.
            .filter(entry => !right.some(e => e && e.id === entry.id));
        const nextRight = moved.concat(right);
        Config.options.bar.layouts.center = [];
        Config.options.bar.layouts.right = nextRight;
        GlobalStates.editHistoryPush({
            "undo": () => {
                Config.options.bar.layouts.center = centre;
                Config.options.bar.layouts.right = right;
            },
            "redo": () => {
                Config.options.bar.layouts.center = [];
                Config.options.bar.layouts.right = nextRight;
            }
        });
    }

    // A component sent to a named list from its own page in the catalogue: the
    // way to place one without carrying it across the screen, and the way to
    // move a placed one between the three groups. One entry at a time and one
    // history entry for the pair, because a move touches two lists.
    //
    // The bar's layout is not history-aware on its own; the pair is recorded
    // here, around the one write.
    function placeBarComponent(componentId, bucket) {
        const layouts = Config.options.bar.layouts;
        if (!layouts || !(bucket in layouts))
            return;
        // While the island owns the centre that list is unreachable, so a
        // request for it lands on the right instead of nowhere.
        if (bucket === "center" && root.barCentreBlocked)
            bucket = "right";
        const buckets = ["left", "center", "right"];
        const before = {};
        const after = {};
        for (const name of buckets) {
            before[name] = EditModeLogic.listCopy(layouts[name] ?? []).map(e => Object.assign({}, e));
            // A component belongs to one list at a time, so it comes out of
            // wherever it was before it goes anywhere.
            after[name] = before[name].filter(e => !(e && e.id === componentId));
        }
        after[bucket] = after[bucket].concat([{ "id": componentId, "centered": false, "visible": true }]);
        // Nothing to record when it was already the last entry of that list.
        if (JSON.stringify(before) === JSON.stringify(after))
            return;
        for (const name of buckets)
            layouts[name] = after[name];
        GlobalStates.editHistoryPush({
            "undo": () => { for (const name of buckets) Config.options.bar.layouts[name] = before[name]; },
            "redo": () => { for (const name of buckets) Config.options.bar.layouts[name] = after[name]; }
        });
    }

    // A catalogue row for a component already on the bar: the click takes it
    // off, from wherever it sits. The badge on the bar does the same thing to
    // the same lists; this is the way back for a widget whose own badge is
    // hard to reach.
    function removeBarComponent(componentId) {
        const layouts = Config.options.bar.layouts;
        if (!layouts)
            return;
        const buckets = ["left", "center", "right"];
        const before = {};
        const after = {};
        const touched = [];
        for (const bucket of buckets) {
            const list = EditModeLogic.listCopy(layouts[bucket] ?? []);
            const next = list.filter(e => !(e && e.id === componentId));
            if (next.length === list.length)
                continue;
            before[bucket] = list;
            after[bucket] = next;
            touched.push(bucket);
        }
        if (touched.length === 0)
            return;
        for (const bucket of touched)
            layouts[bucket] = after[bucket];
        GlobalStates.editHistoryPush({
            "undo": () => { for (const bucket of touched) Config.options.bar.layouts[bucket] = before[bucket]; },
            "redo": () => { for (const bucket of touched) Config.options.bar.layouts[bucket] = after[bucket]; }
        });
    }

    // ── A catalogue row carried onto the bar ─────────────────────────────────
    // The drawer is drawn here; the bar is another layer surface entirely, so
    // the pointer is brought into the bar window's coordinates and the bar's
    // own controller answers with the same preview a reorder gets.
    function barController() {
        return GlobalStates.barEditControllerFor(root.screenName);
    }

    // The strip the bar occupies, with a little slack past it: a drop just
    // short of the bar is a drop on the bar.
    function overBarStrip(x, y) {
        const reach = EditModeInsets.barThickness + 16;
        const side = EditModeInsets.barSide;
        if (side === "top")
            return y <= reach;
        if (side === "bottom")
            return y >= root.height - reach;
        if (side === "left")
            return x <= reach;
        return x >= root.width - reach;
    }

    // The inverse of fromBarWindow: the bar sits flush with one screen edge,
    // so the far edges translate by that window's own size.
    function toBarWindow(controller, x, y) {
        const side = EditModeInsets.barSide;
        if (side === "bottom")
            return Qt.point(x, y - (root.height - controller.windowHeight));
        if (side === "right")
            return Qt.point(x - (root.width - controller.windowWidth), y);
        return Qt.point(x, y);
    }

    function barDragMoved(componentId, x, y) {
        const controller = root.barController();
        if (!controller)
            return;
        if (!root.overBarStrip(x, y)) {
            controller.externalDragEnd();
            return;
        }
        const point = root.toBarWindow(controller, x, y);
        controller.externalDragMoved(componentId, point.x, point.y);
    }

    function barDrop(componentId, x, y) {
        const controller = root.barController();
        if (!controller)
            return;
        if (!root.overBarStrip(x, y)) {
            controller.externalDragEnd();
            return;
        }
        const point = root.toBarWindow(controller, x, y);
        controller.externalDrop(componentId, point.x, point.y);
    }

    // ── Wallpaper history ────────────────────────────────────────────────────
    // Wallpaper services predate the editor's history, so the surface records
    // the three picture paths as they land. Palette changes now go through
    // MaterialThemeLoader, which owns their asynchronous undo boundary.
    readonly property var styleBackground: Config.options.background
    property string _styleWallpaper: ""
    property string _styleLockWallpaper: ""
    property string _styleLightWallpaper: ""

    function styleSnapshot() {
        root._styleWallpaper = String(root.styleBackground.wallpaperPath ?? "");
        root._styleLockWallpaper = String(root.styleBackground.lockscreenWallpaperPath ?? "");
        root._styleLightWallpaper = String(root.styleBackground.lightModeWallpaperPath ?? "");
    }


    function replayWallpaper(path) {
        if (path === "")
            return;
        Wallpapers.apply(path, Appearance.m3colors.darkmode);
    }

    function replayLockWallpaper(path) {
        if (path === "")
            return;
        Wallpapers.applyLockscreen(path, Appearance.m3colors.darkmode);
    }

    function replayLightWallpaper(path) {
        if (path === "")
            return;
        Wallpapers.applyLightModeWallpaper(path);
    }



    function recordStyleChange(before, after, replay) {
        if (before === after)
            return;
        GlobalStates.editHistoryPush({
            "undo": () => replay(before),
            "redo": () => replay(after)
        });
    }

    Connections {
        target: root.styleBackground
        function onWallpaperPathChanged() {
            const next = String(root.styleBackground.wallpaperPath ?? "");
            root.recordStyleChange(root._styleWallpaper, next, root.replayWallpaper);
            root._styleWallpaper = next;
        }
        function onLockscreenWallpaperPathChanged() {
            const next = String(root.styleBackground.lockscreenWallpaperPath ?? "");
            root.recordStyleChange(root._styleLockWallpaper, next, root.replayLockWallpaper);
            root._styleLockWallpaper = next;
        }
        function onLightModeWallpaperPathChanged() {
            const next = String(root.styleBackground.lightModeWallpaperPath ?? "");
            root.recordStyleChange(root._styleLightWallpaper, next, root.replayLightWallpaper);
            root._styleLightWallpaper = next;
        }
    }


    // ── Back to the defaults ─────────────────────────────────────────────────
    // A surface set back to what the shell ships with, as ONE history entry.
    // The defaults are the snapshot Config takes of its own declarations at
    // load, so a reset here and a fresh install agree. Written here, like
    // every other layout write, so the before/after pair is recorded where
    // the write is made.
    function defaultsFor(path) {
        let node = Config.defaultOptions;
        for (const key of path.split(".")) {
            if (node === null || node === undefined || typeof node !== "object")
                return undefined;
            node = node[key];
        }
        return node;
    }

    function resetSurface(what) {
        if (what === "widgets") {
            const ids = Config.activeWidgets.filter(e => e && e.id).map(e => e.id);
            if (ids.length === 0)
                return;
            GlobalStates.editHistoryBeginBatch();
            for (const id of ids)
                Config.removeWidgetInstance(id);
            GlobalStates.editHistoryEndBatch();
            return;
        }
        if (what === "bar") {
            const layouts = Config.options.bar.layouts;
            const defaults = root.defaultsFor("bar.layouts");
            if (!layouts || !defaults)
                return;
            const buckets = ["left", "center", "right"];
            const before = {};
            const after = {};
            for (const name of buckets) {
                before[name] = EditModeLogic.listCopy(layouts[name] ?? []).map(e => Object.assign({}, e));
                after[name] = EditModeLogic.listCopy(defaults[name] ?? []).map(e => Object.assign({}, e));
            }
            if (JSON.stringify(before) === JSON.stringify(after))
                return;
            for (const name of buckets)
                layouts[name] = after[name];
            GlobalStates.editHistoryPush({
                "undo": () => { for (const name of buckets) Config.options.bar.layouts[name] = before[name]; },
                "redo": () => { for (const name of buckets) Config.options.bar.layouts[name] = after[name]; }
            });
            return;
        }
        if (what === "dock") {
            const before = ShellServices.Dock.pinnedOrStarter().slice();
            const after = ShellServices.Dock.normalizedPins(ShellServices.Dock.starterPins());
            if (JSON.stringify(before) === JSON.stringify(after))
                return;
            ShellServices.Dock.setPinned(after);
            GlobalStates.editHistoryPush({
                "undo": () => ShellServices.Dock.setPinned(before),
                "redo": () => ShellServices.Dock.setPinned(after)
            });
            return;
        }
        if (what === "lockIslands") {
            const islands = Config.options.lock.islands;
            const defaults = root.defaultsFor("lock.islands");
            if (!islands || !defaults)
                return;
            const keys = ["main", "left", "right", "hidden"];
            const before = {};
            const after = {};
            for (const key of keys) {
                before[key] = EditModeLogic.listCopy(islands[key] ?? []);
                after[key] = EditModeLogic.listCopy(defaults[key] ?? []);
            }
            if (JSON.stringify(before) === JSON.stringify(after))
                return;
            for (const key of keys)
                islands[key] = after[key];
            GlobalStates.editHistoryPush({
                "undo": () => { for (const key of keys) Config.options.lock.islands[key] = before[key]; },
                "redo": () => { for (const key of keys) Config.options.lock.islands[key] = after[key]; }
            });
        }
    }


    Item {
        id: menuCloser
        width: (root.menuOpenHere || root.barMenuOpenHere) ? root.width : 0
        height: (root.menuOpenHere || root.barMenuOpenHere) ? root.height : 0
    }

    // The hovered widget's name, off the bar's body on whichever edge it sits.
    // Drawn here rather than in the bar because the toolbar covers the strip
    // just past the bar, and this has to sit on top of it.
    Loader {
        anchors.fill: parent
        active: root.barHoverHere
        z: 11
        // Wrapped in a filling Item: a Loader with a size resizes whatever it
        // loads to match, and a chip that has been stretched over the whole
        // screen is not obviously a chip.
        sourceComponent: Item {
            Rectangle {
                readonly property real clear: EditModeInsets.barThickness + 6
                readonly property string side: EditModeInsets.barSide
                readonly property point at: root.fromBarWindow(GlobalStates.editBarHoverX, GlobalStates.editBarHoverY,
                    GlobalStates.editBarHoverWindowWidth, GlobalStates.editBarHoverWindowHeight)

                width: hoverLabel.implicitWidth + 20
                height: 26
                radius: Appearance.rounding.full
                color: Appearance.colors.colSecondaryContainer

                x: {
                    const wanted = side === "left" ? clear
                        : side === "right" ? root.width - clear - width
                        : at.x - width / 2;
                    return Math.min(Math.max(wanted, 8), root.width - width - 8);
                }
                y: {
                    const wanted = side === "bottom" ? root.height - clear - height
                        : side === "top" ? clear
                        : at.y - height / 2;
                    return Math.min(Math.max(wanted, 8), root.height - height - 8);
                }

                StyledText {
                    id: hoverLabel
                    anchors.centerIn: parent
                    text: GlobalStates.editBarHoverName
                    font.pixelSize: Appearance.font.pixelSize.smaller
                    color: Appearance.colors.colOnSecondaryContainer
                }

                opacity: 0
                Component.onCompleted: opacity = 1
                Behavior on opacity {
                    enabled: !Appearance.reducedMotion
                    animation: Appearance.animation.elementMoveEnter.numberAnimation.createObject(this)
                }
            }
        }
    }

    Loader {
        anchors.fill: parent
        active: root.barMenuOpenHere
        z: 10
        sourceComponent: Item {
            MouseArea {
                anchors.fill: parent
                acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
                onPressed: GlobalStates.closeEditBarMenu()
            }
            EditBarMenu {
                id: barMenuCard
                readonly property point at: root.barMenuPoint()
                x: Math.min(Math.max(at.x, 8), root.width - width - 8)
                y: Math.min(Math.max(at.y, 8), root.height - height - 8)
                controller: GlobalStates.editBarMenuController
                bucket: GlobalStates.editBarMenuBucket
                index: GlobalStates.editBarMenuIndex
                centered: GlobalStates.editBarMenuCentered
                onDismissRequested: GlobalStates.closeEditBarMenu()
                transformOrigin: Item.TopLeft
                scale: 0.85
                opacity: 0
                Component.onCompleted: {
                    scale = 1;
                    opacity = 1;
                }
                Behavior on scale {
                    enabled: !Appearance.reducedMotion
                    animation: Appearance.animation.elementMoveEnter.numberAnimation.createObject(barMenuCard)
                }
                Behavior on opacity {
                    enabled: !Appearance.reducedMotion
                    animation: Appearance.animation.elementMoveEnter.numberAnimation.createObject(barMenuCard)
                }
            }
        }
    }

    Loader {
        id: menuLoader
        anchors.fill: parent
        active: root.menuOpenHere
        z: 10
        sourceComponent: Item {
            readonly property bool wantsKeyboard: menuCard.wantsKeyboard

            MouseArea {
                anchors.fill: parent
                acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
                onPressed: GlobalStates.closeEditWidgetMenu()
            }
            EditWidgetMenu {
                id: menuCard
                x: Math.min(Math.max(GlobalStates.editWidgetMenuX, 8), root.width - width - 8)
                y: Math.min(Math.max(GlobalStates.editWidgetMenuY, 8), root.height - height - 8)
                canvas: GlobalStates.editWidgetMenuCanvas
                instanceId: GlobalStates.editWidgetMenuInstanceId
                onDismissRequested: GlobalStates.closeEditWidgetMenu()
                // From the corner the pointer is at: the card belongs to a point.
                transformOrigin: Item.TopLeft
                scale: 0.85
                opacity: 0
                Component.onCompleted: {
                    scale = 1.0;
                    opacity = 1.0;
                }
                Behavior on scale {
                    enabled: !Appearance.reducedMotion
                    animation: Appearance.animation.elementMoveEnter.numberAnimation.createObject(menuCard)
                }
                Behavior on opacity {
                    enabled: !Appearance.reducedMotion
                    animation: Appearance.animation.elementMoveFast.numberAnimation.createObject(menuCard)
                }
            }
        }
    }

    EditModeChromeContent {
        id: chrome
        anchors.fill: parent
        card: Qt.rect(root.cardGeometry.x, root.cardGeometry.y, root.cardGeometry.width, root.cardGeometry.height)
        area: Qt.rect(root.areaGeometry.x, root.areaGeometry.y, root.areaGeometry.width, root.areaGeometry.height)
        // The second stand-down gate, the loader that creates this window
        // being the first. Either alone hides the chrome; both are kept so a
        // lost gate is not a lost chrome.
        opacity: Math.max(0, Math.min(1, root.progress))

        drawer: Qt.rect(root.drawerGeometry.x, root.drawerGeometry.y, root.drawerGeometry.width, root.drawerGeometry.height)
        drawerScreenName: root.screenName

        // Done is the mode's way out — except while a guide is hosting the
        // session, where the guide owns what "finished" means. The Welcome
        // moves to its next step, and closing the mode is its business.
        onDoneRequested: {
            if (GlobalStates.editGuideActive) {
                GlobalStates.editGuideDoneRequested();
                return;
            }
            GlobalStates.editMode = false;
        }
        onUndoRequested: GlobalStates.editUndo()
        onRedoRequested: GlobalStates.editRedo()
        onTabRequested: tab => {
            GlobalStates.editWidgetMenuOpen = false;
            GlobalStates.editTab = tab;
        }
        onDrawerLockLayoutResetRequested: Config.clearWidgetLockPositions(root.screenName)
        onDrawerToggleRequested: GlobalStates.editDrawerOpen = !GlobalStates.editDrawerOpen
        onDrawerPageRequested: (section, page) => {
            GlobalStates.editDrawerSection = section;
            GlobalStates.editDrawerPage = page;
            GlobalStates.editDrawerOpen = true;
        }
        onDrawerAddRequested: (widgetId, dropX, dropY) => root.addWidgetAt(widgetId, dropX, dropY)
        onDrawerAddWidgetRequested: widgetId => root.addWidgetInstance(widgetId)
        onDrawerBarPlaceRequested: (componentId, bucket) => root.placeBarComponent(componentId, bucket)
        onDrawerBarRemoveRequested: componentId => root.removeBarComponent(componentId)
        onDrawerBarDragMoved: (componentId, x, y) => root.barDragMoved(componentId, x, y)
        onDrawerBarDropRequested: (componentId, x, y) => root.barDrop(componentId, x, y)
        onDrawerBarDragCancelled: root.barController()?.externalDragEnd()
        onDrawerAddAppRequested: (appId, dropX, dropY) => root.addAppAt(appId, dropX, dropY)
        onDrawerToggleAppRequested: appId => root.toggleAppOnHomeScreen(appId)
        onDrawerAddAppPairRequested: (firstAppId, secondAppId, name) => root.addAppPair(firstAppId, secondAppId, name)
        onDrawerAddFolderRequested: (folderName, appsList) => root.addFolder(folderName, appsList)
        onDrawerClearHomeScreenAppsRequested: root.clearHomeScreenApps()
        onDrawerResetRequested: what => root.resetSurface(what)
        // A preference, not a layout edit: no history entry, same as the
        // Settings toggle that writes the same key.
        onSnapToggleRequested: Config.widgetProvider
            ? Config.widgetProvider.toggleSnap()
            : Config.options.background.widgets.enableSnap = !Config.options.background.widgets.enableSnap
    }
}

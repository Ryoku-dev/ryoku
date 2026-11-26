import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// The islands and the state they share; each island draws itself.
Item {
    id: root

    required property PickerState state
    readonly property LibraryView view: root.state ? root.state.view : null
    readonly property string collection: root.view ? root.view.collection : ""
    // Type, folder, sort, colour, favourites, shape and size describe wallpapers; rices
    // and themes carry none of them, so the second line drops those controls there.
    readonly property bool wallFacets: root.collection === "wallpapers" || root.collection === "workshop"

    // A cheap stamp so settings-driven bindings re-evaluate without one SettingValue per key.
    property int settingsRev: 0
    Connections {
        target: Settings
        function onChanged(key, value) {
            root.settingsRev++
            if (key === "filterBar.sticky") {
                root._initialized = false
                root.applyInitial()
            }
        }
        function onReadyChanged() { root.applyInitial() }
    }
    function val(key, def) {
        root.settingsRev
        var v = Settings.value(key)
        return (v === undefined || v === null) ? def : v
    }
    function show(key) { return root.val(key, true) !== false }

    readonly property CardField field: root.state ? root.state.field : null
    readonly property bool muted: root.val("wallpaperMute", true) === true
    readonly property bool audioActive: {
        var outs = Library.outputs || []
        for (var i = 0; i < outs.length; ++i) {
            var t = outs[i].current ? outs[i].current.type : ""
            if (t === "video" || t === "we")
                return true
        }
        return false
    }

    readonly property bool drawerSheet: root.state !== null && (root.state.sheet === "settings" || root.state.sheet === "browser")
    readonly property bool revealed: (root.state ? (root.state.filterBarShown || root.drawerSheet) : false)
        && !(root.field && root.field.flippedIndex >= 0)
    // The wall grid centres itself in the space below the islands while they show.
    readonly property bool obscured: root.state !== null && root.state.sheet !== "" && !root.state.sceneThrough && !root.drawerSheet
    // a see-through design sheet keeps the bar's room so the preview stays true, but not the bar itself
    readonly property bool behindSheet: root.state !== null && root.state.sheet !== "" && root.state.sceneThrough && !root.drawerSheet
    readonly property bool shown: root.revealed && !root.obscured && !root.behindSheet
    readonly property real s: Theme.scale
    readonly property real edge: 18 * Theme.scale
    readonly property real gap: 14 * Theme.scale
    readonly property real islandH: 54 * Theme.scale
    readonly property real islandGap: 30 * Theme.scale
    Binding {
        when: root.field !== null
        target: root.field
        property: "barReserve"
        value: root.revealed && !root.obscured
            ? Qt.size(root.totalW, root.edge + root.islandH + root.gap)
            : Qt.size(0, 0)
    }

    property bool rotationActive: false
    function refreshRotation() {
        Daemon.call("wall.rotation_status", {}, function (r, e) {
            if (r) root.rotationActive = r.active === true
        })
    }
    Connections {
        target: Daemon
        function onEvent(name, data) {
            if (name === "ryogami.wall.random_started" || name === "ryogami.wall.random_stopped")
                root.refreshRotation()
        }
        function onReconnected() { root.refreshRotation() }
    }
    // The daemon's randomRotate action flips the saved switch, so a rotation
    // started here resumes after a restart, and Ctrl+R runs the same action.
    function toggleRotation() {
        if (!root.rotationActive
                && root.val("general.randomIncludeStatic", true) !== true
                && root.val("general.randomIncludeVideo", true) !== true
                && root.val("general.randomIncludeWE", true) !== true) {
            if (root.state) root.state.toast(I18n.tr("Select at least one random source category in settings"), "error")
            return
        }
        Daemon.call("settings.action", { id: "randomRotate" }, function () { root.refreshRotation() })
    }

    function cycleShape() {
        if (!root.view) return
        var o = root.view.orientation
        root.view.orientation = (o === "" || o === undefined) ? "wide"
                              : (o === "wide") ? "tall" : ""
    }
    readonly property string shapeLabel: {
        var o = root.view ? root.view.orientation : ""
        return o === "wide" ? I18n.tr("Wide") : o === "tall" ? I18n.tr("Tall") : ""
    }

    property int resIndex: -1
    function resPresets() {
        var p = root.val("filterBar.resolutionPresets", [])
        // a list setting reaches QML as a Qt sequence, which Array.isArray rejects
        return p && typeof p === "object" && typeof p.length === "number" ? Array.prototype.slice.call(p) : []
    }
    function cycleResolution() {
        if (!root.view) return
        var presets = root.resPresets()
        if (presets.length === 0) return
        root.resIndex = root.resIndex + 1
        if (root.resIndex >= presets.length) {
            root.resIndex = -1
            root.view.minWidth = 0; root.view.minHeight = 0
            root.view.maxWidth = 0; root.view.maxHeight = 0
            return
        }
        var p = presets[root.resIndex]
        root.view.minWidth = Number(p.minWidth) || 0
        root.view.minHeight = Number(p.minHeight) || 0
        root.view.maxWidth = Number(p.maxWidth) || 0
        root.view.maxHeight = Number(p.maxHeight) || 0
        if (p.orientation === "wide") root.view.orientation = "wide"
        else if (p.orientation === "tall") root.view.orientation = "tall"
    }
    function pickResolution(i) {
        if (!root.view) return
        var presets = root.resPresets()
        root.resIndex = i >= 0 && i < presets.length ? i - 1 : presets.length
        root.cycleResolution()
    }
    function clearResolution() { root.pickResolution(-1) }

    readonly property int filtersInPlay: {
        var v = root.view
        if (!v) return 0
        return (v.typeFilter !== "" ? 1 : 0) + (v.folder !== "*" && v.folder !== undefined ? 1 : 0)
            + (v.hueFilter !== -1 ? 1 : 0) + (v.favouritesOnly ? 1 : 0)
            + (root.shapeLabel !== "" ? 1 : 0) + (root.resIndex >= 0 ? 1 : 0)
    }
    function clearFilters() {
        var v = root.view
        if (!v) return
        v.typeFilter = ""; v.folder = "*"; v.hueFilter = -1; v.favouritesOnly = false; v.orientation = ""
        root.clearResolution()
    }

    function resLabel() {
        var presets = root.resPresets()
        if (root.resIndex >= 0 && root.resIndex < presets.length && presets[root.resIndex].label)
            return presets[root.resIndex].label
        return ""
    }

    readonly property bool light: root.val("theme.mode", "dark") === "light"
    readonly property bool followWall: root.val("theme.policy", "wallpaper") === "wallpaper"

    readonly property bool sticky: root.val("filterBar.sticky", true) === true
    property bool _restoring: false
    function save(name, value) {
        if (root._restoring || !root.sticky) return
        Settings.set("filterBar.last." + name, value)
    }
    property bool _initialized: false
    function applyInitial() {
        if (!root.view || !Settings.ready || root._initialized) return
        root._initialized = true
        root._restoring = true
        var v = root.view
        if (root.sticky) {
            v.typeFilter = String(root.val("filterBar.last.type", ""))
            v.folder = String(root.val("filterBar.last.folder", "*"))
            v.hueFilter = Number(root.val("filterBar.last.color", -1))
            v.sort = String(root.val("filterBar.last.sort", "color"))
            v.favouritesOnly = root.val("filterBar.last.favourites", false) === true
        } else {
            v.folder = String(root.val("filterBar.defaultFolder", "*"))
        }
        root._restoring = false
        if (root.state)
            root.state.filtersReady = true
    }
    Component.onCompleted: { root.refreshRotation(); root.applyInitial() }
    onViewChanged: root.applyInitial()

    Connections {
        target: root.view
        ignoreUnknownSignals: true
        function onTypeFilterChanged() { root.save("type", root.view.typeFilter) }
        function onFolderChanged() { root.save("folder", root.view.folder) }
        function onHueFilterChanged() { root.save("color", root.view.hueFilter) }
        function onSortChanged() { root.save("sort", root.view.sort) }
        function onFavouritesOnlyChanged() { root.save("favourites", root.view.favouritesOnly) }
    }

    readonly property real leftW: leftIsland.implicitWidth + 16 * root.s
    readonly property real rightW: rightIsland.implicitWidth + 16 * root.s
    readonly property real midW: Math.max(300 * root.s, Math.min(700 * root.s,
        root.width - 48 * root.s - root.leftW - root.rightW - 2 * root.islandGap))
    readonly property real totalW: root.leftW + root.midW + root.rightW + 2 * root.islandGap
    readonly property real x0: Math.round((root.width - root.totalW) / 2)
    readonly property rect leftRect: Qt.rect(root.x0, root.edge, root.leftW, root.islandH)
    readonly property rect midRect: Qt.rect(root.x0 + root.leftW + root.islandGap, root.edge, root.midW, root.islandH)
    readonly property rect rightRect: Qt.rect(root.midRect.x + root.midW + root.islandGap, root.edge, root.rightW, root.islandH)

    property real reveal: root.shown ? 1 : 0
    Behavior on reveal {
        enabled: !Lithic.reduceMotion
        NumberAnimation { duration: Theme.slow; easing.type: root.shown ? Easing.OutBack : Easing.InCubic; easing.overshoot: 1.15 }
    }
    property real weld: root.shown && root.reveal > 0.9 ? 1 : 0
    Behavior on weld {
        enabled: !Lithic.reduceMotion
        NumberAnimation { duration: Theme.standard; easing.type: Easing.OutCubic }
    }
    readonly property real lift: (1 - root.reveal) * -(root.islandH + 40 * root.s)

    Binding { target: root.state; property: "chromeBottom"; value: root.edge + root.islandH; when: root.state !== null }
    Binding { target: root.state; property: "searchIsland"; value: root.midRect; when: root.state !== null }
    Binding { target: root.state; property: "chromeRight"; value: root.rightRect.x + root.rightRect.width; when: root.state !== null }
    Binding {
        target: root.state; property: "settingsAnchorX"; when: root.state !== null
        value: rightContent.x + rightIsland.settingsButton.x + rightIsland.settingsButton.width / 2
    }
    Binding {
        target: root.state; property: "downloadAnchorX"; when: root.state !== null
        value: rightContent.x + rightIsland.downloadButton.x + rightIsland.downloadButton.width / 2
    }

    readonly property bool walls: root.collection === "wallpapers" || root.collection === "workshop"
    readonly property bool sheetWanted: root.state !== null && root.state.filtersOpen && root.shown && root.walls
        && root.state.sheet === ""
    onWallsChanged: if (!root.walls && root.state) root.state.filtersOpen = false
    Connections {
        target: root.state
        function onSheetChanged() { if (root.state.sheet !== "") root.state.filtersOpen = false }
    }
    property real sheetProgress: root.sheetWanted ? 1 : 0
    Behavior on sheetProgress {
        enabled: !Lithic.reduceMotion
        NumberAnimation { duration: Theme.standard * 1.5; easing.type: Easing.OutCubic }
    }
    readonly property real sheetAnchorX: midContent.x + searchIsland.tuneCenterX
    readonly property real sheetW: Math.min(900 * root.s, root.width - 48 * root.s)
    readonly property rect sheetTarget: Qt.rect(
        Math.max(24 * root.s, Math.min(root.width - root.sheetW - 24 * root.s, root.sheetAnchorX - root.sheetW + 140 * root.s)),
        root.edge + root.islandH + root.gap, root.sheetW, filterSheetLoader.item ? filterSheetLoader.item.implicitHeight : 420 * root.s)
    readonly property rect sheetNow: {
        var p = root.sheetProgress, t = root.sheetTarget, bud = 56 * root.s
        var pw = Math.min(1, p * 1.3)
        function mix(a, b, k) { return a + (b - a) * k }
        return Qt.rect(mix(root.sheetAnchorX - bud / 2, t.x, pw), t.y, mix(bud, t.width, pw), mix(0, t.height, p))
    }

    // A click outside an open sheet only closes it; it never reaches the cards.
    MouseArea {
        anchors.fill: parent
        visible: root.sheetWanted
        onClicked: root.state.filtersOpen = false
        onWheel: (w) => w.accepted = true
    }
    Rectangle {
        anchors.fill: parent
        color: Lithic.scrim
        opacity: root.state && root.state.drawerKey !== "" ? (root.state.sceneThrough ? 0.12 : 0.6) * root.state.drawerProgress : 0
        Behavior on opacity { NumberAnimation { duration: Theme.standard } }
        visible: opacity > 0.01
        MouseArea {
            anchors.fill: parent
            onClicked: root.state.closeSheet()
            onWheel: (w) => w.accepted = true
        }
    }

    IslandLiquid {
        anchors.fill: parent
        state: root.state
        leftRect: root.leftRect
        midRect: root.midRect
        rightRect: root.rightRect
        reveal: root.reveal
        weld: root.weld
        readonly property bool searching: root.state !== null && root.state.searchProgress > root.sheetProgress
        sheetRect: searching ? root.state.searchPanelNow : root.sheetNow
        sheetAnchorX: searching ? root.midRect.x + 120 * root.s : root.sheetAnchorX
        sheetProgress: searching ? root.state.searchProgress : root.sheetProgress
    }

    // An empty click on the scene closes the picker, and hover there previews a palette.
    component Catch: MouseArea {
        property rect r
        x: r.x; y: r.y + root.lift; width: r.width; height: r.height
        hoverEnabled: true
        visible: root.reveal > 0.01
    }
    Catch { r: root.leftRect }
    Catch { r: root.midRect }
    Catch { r: root.rightRect }

    Item {
        id: leftContent
        x: root.leftRect.x + 8 * root.s
        y: root.leftRect.y + root.lift
        width: root.leftRect.width - 16 * root.s
        height: root.islandH
        opacity: Math.min(1, root.reveal * 1.4)
        visible: opacity > 0.01
        LibraryIsland {
            id: leftIsland
            anchors.verticalCenter: parent.verticalCenter
            state: root.state
        }
    }
    Item {
        id: midContent
        x: root.midRect.x
        y: root.midRect.y + root.lift
        width: root.midRect.width
        height: root.islandH
        opacity: Math.min(1, root.reveal * 1.4)
        visible: opacity > 0.01
        SearchIsland {
            id: searchIsland
            anchors.fill: parent
            state: root.state
            bar: root
        }
    }
    Item {
        id: rightContent
        x: root.rightRect.x + 8 * root.s
        y: root.rightRect.y + root.lift
        width: root.rightRect.width - 16 * root.s
        height: root.islandH
        opacity: Math.min(1, root.reveal * 1.4)
        visible: opacity > 0.01
        ActionIsland {
            id: rightIsland
            anchors.verticalCenter: parent.verticalCenter
            state: root.state
            bar: root
        }
    }

    Item {
        x: root.sheetNow.x
        y: root.sheetNow.y
        width: root.sheetNow.width
        height: root.sheetNow.height
        clip: true
        visible: root.sheetProgress > 0.001
        MouseArea { anchors.fill: parent; hoverEnabled: true; onWheel: (w) => w.accepted = true }
        Loader {
            id: filterSheetLoader
            x: root.sheetTarget.x - root.sheetNow.x
            width: root.sheetTarget.width
            active: root.sheetWanted || root.sheetProgress > 0.001
            opacity: Math.max(0, (root.sheetProgress - 0.5) * 2)
            sourceComponent: FilterSheet {
                state: root.state
                bar: root
            }
        }
    }
}

import QtQuick
import QtQuick.Effects
import Ryoku.Ui.Singletons

// The picker's masthead, set like the head of a printed page: the 力 seal, the four
// libraries named in Latin and Japanese, a search line and the count. Under a hairline,
// a second line carries only what the open library needs. Emphasis is inversion, colour
// appears only as data (the hue swatches) and in the seal.
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

    readonly property bool revealed: (root.state ? root.state.filterBarShown : false)
        && !(root.field && root.field.flippedIndex >= 0)
    // The wall grid centres itself in the space below the masthead while it shows.
    readonly property bool obscured: root.state !== null && root.state.sheet !== "" && !root.state.sceneThrough
    readonly property real edge: 22 * Theme.scale
    readonly property real gap: 14 * Theme.scale
    Binding {
        when: root.field !== null
        target: root.field
        property: "barReserve"
        value: root.revealed && !root.obscured
            ? Qt.size(slab.width, root.edge + slab.height + root.gap)
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
            if (name === "ryogami.wall.random_started") root.rotationActive = true
            else if (name === "ryogami.wall.random_stopped") root.rotationActive = false
        }
        function onReconnected() { root.refreshRotation() }
    }
    function toggleRotation() {
        if (root.rotationActive) { Daemon.call("wall.rotation_stop", {}, function () {}); return }
        var types = []
        if (root.val("general.randomIncludeStatic", true) === true) types.push("static")
        if (root.val("general.randomIncludeVideo", true) === true) types.push("video")
        if (root.val("general.randomIncludeWE", true) === true) types.push("we")
        if (types.length === 0) {
            if (root.state) root.state.toast(I18n.tr("Select at least one random source category in settings"), "error")
            return
        }
        Daemon.call("wall.random_start", {
            interval: Number(root.val("general.randomInterval", 300)),
            types: types,
            favourites_only: root.val("general.randomIncludeFavourites", false) === true
        }, function () {})
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
        return Array.isArray(p) ? p : []
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

    // The seal stamps in each time the picker opens: the one moment the masthead performs.
    Connections {
        target: root.state
        function onShownChanged() { if (root.state.shown) stamp.restart() }
    }

    // The second line fades across when the library changes, so its controls never pop.
    property string lineCollection: root.collection
    property real lineSwap: 1
    onCollectionChanged: swapAnim.restart()
    SequentialAnimation {
        id: swapAnim
        NumberAnimation { target: root; property: "lineSwap"; to: 0; duration: Theme.fast * 0.6; easing.type: Easing.InQuad }
        ScriptAction { script: root.lineCollection = root.collection }
        NumberAnimation { target: root; property: "lineSwap"; to: 1; duration: Theme.standard; easing.type: Easing.OutCubic }
    }

    Rectangle {
        id: slab
        width: Math.min(root.width - 96 * Theme.scale, 1320 * Theme.scale)
        height: head.height + 1 + line.height
        x: (root.width - width) * 0.5
        y: root.edge
        radius: Theme.radius
        color: Theme.withAlpha(Theme.surface, 0.95)
        border.width: 1
        border.color: Theme.withAlpha(Theme.outline, 0.5)
        enabled: root.revealed
        opacity: root.revealed ? 1 : 0
        visible: opacity > 0.01
        transform: Translate { y: (1 - slab.opacity) * -10 * Theme.scale }
        Behavior on opacity { NumberAnimation { duration: Theme.standard; easing.type: Theme.revealEasing } }
        // Clicks and hover in a gap of the masthead stay on it: an empty click on the scene
        // closes the picker, and hover reaching the cards below recoloured the whole desktop
        // through the hover palette preview as the pointer crossed the bar.
        MouseArea { anchors.fill: parent; hoverEnabled: true }

        // ── head: seal, libraries, search, count ─────────────────────────────
        Item {
            id: head
            width: parent.width
            height: 58 * Theme.scale

            // Ryoku's mark, the one on the README. On a light theme its bone strokes would
            // vanish, so it takes the ink colour there.
            Item {
                id: seal
                anchors.left: parent.left
                anchors.leftMargin: 16 * Theme.scale
                anchors.verticalCenter: parent.verticalCenter
                width: 30 * Theme.scale
                height: 26 * Theme.scale
                Image {
                    id: mark
                    anchors.fill: parent
                    source: Qt.resolvedUrl("brand/logo-mark.png")
                    fillMode: Image.PreserveAspectFit
                    sourceSize.height: 64
                    smooth: true
                    mipmap: true
                    visible: Theme.isDark
                }
                MultiEffect {
                    anchors.fill: mark
                    source: mark
                    visible: !Theme.isDark
                    colorization: 1
                    colorizationColor: Theme.surfaceText
                }
                ParallelAnimation {
                    id: stamp
                    NumberAnimation { target: seal; property: "scale"; from: 0.7; to: 1; duration: Theme.slow; easing.type: Easing.OutBack; easing.overshoot: 1.8 }
                    NumberAnimation { target: seal; property: "opacity"; from: 0; to: 1; duration: Theme.standard; easing.type: Easing.OutQuad }
                }
            }
            Rectangle {
                id: sealRule
                anchors.left: seal.right
                anchors.leftMargin: 14 * Theme.scale
                anchors.verticalCenter: parent.verticalCenter
                width: 1
                height: 30 * Theme.scale
                color: Theme.withAlpha(Theme.surfaceText, 0.18)
            }

            CollectionTabs {
                id: tabsBlock
                anchors.left: sealRule.right
                anchors.leftMargin: 10 * Theme.scale
                anchors.verticalCenter: parent.verticalCenter
                state: root.state
            }

            Row {
                id: headRight
                anchors.right: parent.right
                anchors.rightMargin: 12 * Theme.scale
                anchors.verticalCenter: parent.verticalCenter
                spacing: 4 * Theme.scale

                Item {
                    id: countMark
                    visible: root.view !== null
                    width: countCol.implicitWidth + 18 * Theme.scale
                    height: 44 * Theme.scale
                    property real shown: root.view ? root.view.count : 0
                    Behavior on shown { NumberAnimation { duration: Theme.slow; easing.type: Easing.OutCubic } }
                    Column {
                        id: countCol
                        anchors.centerIn: parent
                        spacing: -2 * Theme.scale
                        Text {
                            anchors.right: parent.right
                            text: String(Math.round(countMark.shown))
                            font.family: Theme.display
                            font.pixelSize: Theme.fs(24)
                            font.weight: Font.Normal
                            font.features: { "tnum": 1, "lnum": 1 }
                            color: Theme.surfaceText
                            renderType: Text.NativeRendering
                        }
                        Text {
                            anchors.right: parent.right
                            text: root.collection === "themes" ? I18n.tr("themes")
                                : root.collection === "rices" ? I18n.tr("rices")
                                : root.collection === "workshop" ? I18n.tr("scenes")
                                : I18n.tr("walls")
                            font.family: Theme.sans
                            font.pixelSize: Theme.fs(9)
                            font.letterSpacing: 1.6
                            font.capitalization: Font.AllUppercase
                            color: Theme.withAlpha(Theme.surfaceText, 0.5)
                            renderType: Text.NativeRendering
                        }
                    }
                }
                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    width: 1
                    height: 30 * Theme.scale
                    color: Theme.withAlpha(Theme.surfaceText, 0.18)
                }
                // Wallpaper colours: matugen derives the desktop palette from each wallpaper
                // you apply. Off holds the current colours; a theme tile picks a fixed one.
                BarButton {
                    anchors.verticalCenter: parent.verticalCenter
                    glyph: "\u{f020a}"
                    tooltip: root.followWall ? I18n.tr("Colours follow the wallpaper") : I18n.tr("Take colours from the wallpaper")
                    active: root.followWall
                    hpad: 9
                    onTriggered: Settings.set("theme.policy", root.followWall ? "off" : "wallpaper")
                }
                BarButton {
                    anchors.verticalCenter: parent.verticalCenter
                    visible: root.show("filterBar.show.theme")
                    glyph: root.light ? "\u{f0599}" : "\u{f0594}"
                    tooltip: root.light ? I18n.tr("Switch to dark") : I18n.tr("Switch to light")
                    hpad: 9
                    onTriggered: Settings.set("theme.mode", root.light ? "dark" : "light")
                }
                BarButton {
                    anchors.verticalCenter: parent.verticalCenter
                    glyph: "\u{f0493}"
                    tooltip: I18n.tr("Settings")
                    spinOnHover: true
                    hpad: 9
                    active: root.state && root.state.sheet === "settings"
                    onTriggered: if (root.state) root.state.openSheet("settings", undefined)
                }
            }

            SearchTrigger {
                visible: root.show("filterBar.show.tagcloud")
                anchors.right: headRight.left
                anchors.rightMargin: 18 * Theme.scale
                anchors.verticalCenter: parent.verticalCenter
                width: Math.max(Math.min(headRight.x - 18 * Theme.scale - (tabsBlock.x + tabsBlock.width + 32 * Theme.scale), 360 * Theme.scale), 120 * Theme.scale)
                state: root.state
            }
        }

        Rectangle {
            anchors.top: head.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.leftMargin: 1
            anchors.rightMargin: 1
            height: 1
            color: Theme.withAlpha(Theme.outline, 0.4)
        }

        // ── line: what the open library needs ────────────────────────────────
        Item {
            id: line
            y: head.height + 1
            width: parent.width
            height: 44 * Theme.scale

            Row {
                id: lineLeft
                anchors.left: parent.left
                anchors.leftMargin: 10 * Theme.scale
                anchors.verticalCenter: parent.verticalCenter
                spacing: 2 * Theme.scale
                opacity: root.lineSwap
                transform: Translate { x: (1 - root.lineSwap) * 10 * Theme.scale }

                readonly property bool walls: root.lineCollection === "wallpapers" || root.lineCollection === "workshop"

                TypeSegment {
                    visible: root.lineCollection === "wallpapers"
                    view: root.view
                    shown: root.show
                }
                BarRule { visible: root.lineCollection === "wallpapers" }
                FolderMenu {
                    id: folderMenu
                    visible: lineLeft.walls && root.show("filterBar.show.folder") && folderMenu.hasFolders
                    view: root.view
                }
                SortMenu {
                    id: sortMenu
                    visible: lineLeft.walls && sortMenu.shownCount > 0
                    view: root.view
                    shown: root.show
                }
                BarRule { visible: lineLeft.walls && root.show("filterBar.show.colors") }
                BarSwatches {
                    visible: lineLeft.walls && root.show("filterBar.show.colors")
                    view: root.view
                    compact: slab.width < 1300 * Theme.scale || root.audioActive || Tasks.bar.length > 0
                }
                BarRule { visible: lineLeft.walls }
                BarButton {
                    visible: lineLeft.walls && root.show("filterBar.show.favourites")
                    glyph: root.view && root.view.favouritesOnly ? "\u{f02d1}" : "\u{f02d5}"
                    tooltip: I18n.tr("Favourites only")
                    active: root.view && root.view.favouritesOnly
                    onTriggered: if (root.view) root.view.favouritesOnly = !root.view.favouritesOnly
                }
                BarButton {
                    visible: lineLeft.walls && root.show("filterBar.show.orient")
                    glyph: "\u{f065f}"
                    label: root.shapeLabel
                    tooltip: I18n.tr("Shape: any, wide, tall")
                    active: root.shapeLabel !== ""
                    onTriggered: root.cycleShape()
                }
                BarButton {
                    visible: lineLeft.walls && root.show("filterBar.show.resolution") && root.resPresets().length > 0
                    glyph: "\u{f0a24}"
                    label: root.resLabel()
                    tooltip: I18n.tr("Size")
                    active: root.resIndex >= 0
                    onTriggered: root.cycleResolution()
                }
                BarButton {
                    visible: lineLeft.walls && root.show("filterBar.show.random")
                    glyph: "\u{f049d}"
                    tooltip: root.rotationActive ? I18n.tr("Stop auto-rotate") : I18n.tr("Auto-rotate through random wallpapers")
                    active: root.rotationActive
                    onTriggered: root.toggleRotation()
                }

                BarButton {
                    visible: root.lineCollection === "themes"
                    glyph: "\u{f03d8}"
                    label: I18n.tr("Design a theme")
                    active: root.state && root.state.sheet === "themeDesigner"
                    onTriggered: if (root.state) root.state.openSheet("themeDesigner", ({}))
                }

                BarButton {
                    visible: root.lineCollection === "rices"
                    glyph: "\u{f0193}"
                    label: I18n.tr("Save look")
                    tooltip: I18n.tr("Save this desktop as a rice")
                    active: root.state && root.state.riceShare === "save"
                    onTriggered: if (root.state) root.state.openRiceShare("save", null)
                }
                BarButton {
                    visible: root.lineCollection === "rices"
                    glyph: "\u{f02fa}"
                    label: I18n.tr("Import")
                    tooltip: I18n.tr("Import a rice someone shared")
                    active: root.state && root.state.riceShare === "import"
                    onTriggered: if (root.state) root.state.openRiceShare("import", null)
                }
            }

            Row {
                id: lineRight
                anchors.right: parent.right
                anchors.rightMargin: 10 * Theme.scale
                anchors.verticalCenter: parent.verticalCenter
                spacing: 2 * Theme.scale

                Repeater {
                    model: Tasks.bar
                    delegate: TaskChip {
                        required property var modelData
                        task: modelData
                    }
                }
                BarRule { visible: Tasks.bar.length > 0 }

                // Shown while a display plays a video or scene, the only wallpapers with sound.
                BarButton {
                    visible: root.audioActive
                    glyph: root.muted ? "\u{f075f}" : "\u{f057e}"
                    tooltip: root.muted ? I18n.tr("Unmute wallpapers") : I18n.tr("Mute wallpapers")
                    onTriggered: Settings.set("wallpaperMute", !root.muted)
                }
                BarButton {
                    visible: root.audioActive
                    glyph: "\u{f062e}"
                    tooltip: I18n.tr("Audio mixer")
                    active: root.state && root.state.sheet === "audio"
                    onTriggered: if (root.state) root.state.openSheet("audio", undefined)
                }
                BarButton {
                    visible: root.show("filterBar.show.playlists")
                    glyph: "\u{f0cb8}"
                    tooltip: I18n.tr("Playlists")
                    active: root.state && root.state.sheet === "playlists"
                    onTriggered: if (root.state) root.state.openSheet("playlists", undefined)
                }
                DownloadMenu {
                    visible: root.show("filterBar.show.download")
                    state: root.state
                }
            }
        }
    }
}

import QtQuick
import Ryoku.Ui.Singletons

// The search surface: one focused panel that drops from the masthead search line. A text field
// takes focus at once, a Tags/Describe switch slides between two modes, the live count reads as
// data, and tags are quiet chips you include or exclude. Describe search says plainly when a
// model is missing and offers the one action that sets it up. Masthead language throughout:
// bone ink on dark paper, emphasis by inversion, hairline borders, colour only as data.
Item {
    id: root

    required property PickerState state
    readonly property LibraryView view: root.state ? root.state.view : null

    anchors.fill: parent

    readonly property bool shown: root.state ? root.state.searchOpen : false
    readonly property string mode: root.state ? (root.state.searchMode || "tags") : "tags"
    readonly property bool describe: root.mode === "describe"

    // available() treats an absent source as usable, so an old daemon that never reports keeps
    // Describe reachable rather than hiding it; the honest state below refines what it can do.
    readonly property bool lensAvailable: Settings.available("lens")

    property SettingValue svRows: SettingValue { key: "components.wallpaperSelector.tagCloudRows" }
    property SettingValue svWidth: SettingValue { key: "components.wallpaperSelector.tagCloudWidth" }
    property SettingValue svHeight: SettingValue { key: "components.wallpaperSelector.tagCloudHeight" }
    property SettingValue svOffX: SettingValue { key: "components.wallpaperSelector.tagCloudOffsetX" }
    property SettingValue svOffY: SettingValue { key: "components.wallpaperSelector.tagCloudOffsetY" }
    property SettingValue svDefaultMode: SettingValue { key: "tagging.defaultSearchMode" }

    function _num(sv, def) { var v = Number(sv.value); return (isNaN(v) || v <= 0) ? def : v }
    readonly property int cloudRows: Math.max(1, Math.min(3, root._num(svRows, 3)))

    readonly property real _padX: 16 * Theme.scale
    readonly property real _padY: 13 * Theme.scale

    // The masthead sits at the top of the screen; these mirror FilterBar's slab so the panel
    // hangs off the search line on the slab's right edge.
    readonly property real _mastheadTop: 22 * Theme.scale
    readonly property real _mastheadHeight: (58 + 1 + 44) * Theme.scale
    readonly property real _mastheadWidth: Math.min(root.width - 96 * Theme.scale, 1320 * Theme.scale)
    readonly property real _mastheadRight: (root.width + root._mastheadWidth) * 0.5

    readonly property real panelWidth: Math.max(360 * Theme.scale,
        Math.min(root._num(svWidth, 760) * Theme.scale, root._mastheadWidth))

    property real reveal: 0
    states: State { name: "open"; when: root.shown; PropertyChanges { target: root; reveal: 1 } }
    Behavior on reveal { NumberAnimation { duration: Theme.standard; easing.type: Theme.revealEasing } }
    visible: reveal > 0.01

    // ── tag histogram ───────────────────────────────────────────────────────
    // The cloud is the whole-collection histogram, rebuilt only when the library changes.
    property var tagCounts: ({})
    function _aggregate() {
        root.tagCounts = (root.view) ? Library.tagHistogram(root.view.collection) : ({})
    }
    property bool orderPopular: true
    readonly property var tagList: {
        var out = []
        for (var k in root.tagCounts) out.push({ tag: k, count: root.tagCounts[k] })
        if (root.orderPopular) out.sort(function (a, b) { return b.count - a.count || (a.tag < b.tag ? -1 : 1) })
        else out.sort(function (a, b) { return a.tag < b.tag ? -1 : (a.tag > b.tag ? 1 : 0) })
        return out.slice(0, 400)
    }
    function _firstTag(frag) {
        for (var i = 0; i < root.tagList.length; ++i) {
            var t = root.tagList[i].tag
            if (t.toLowerCase().indexOf(frag) === 0) return t
        }
        return ""
    }

    // ── tag selection ─────────────────────────────────────────────────────────
    property var selected: []
    property bool multi: true
    property bool matchAny: false
    // A tag is "include", "exclude" (stored as "-tag") or "".
    function _tagState(tag) {
        if (root.selected.indexOf(tag) >= 0) return "include"
        if (root.selected.indexOf("-" + tag) >= 0) return "exclude"
        return ""
    }
    function _removeForms(s, tag) {
        var i = s.indexOf(tag); if (i >= 0) s.splice(i, 1)
        var j = s.indexOf("-" + tag); if (j >= 0) s.splice(j, 1)
        return s
    }
    // Left-click cycles idle -> include; right-click cycles idle -> exclude. Single mode replaces.
    function _toggle(tag, exclude) {
        var token = exclude ? "-" + tag : tag
        var already = root.selected.indexOf(token) >= 0
        var s = root.multi ? root.selected.slice() : []
        if (root.multi) s = root._removeForms(s, tag)
        if (!already) s.push(token)
        root.selected = s
        root._applyTags()
    }
    function _clear() { root.selected = []; root._applyTags() }
    function _applyTags() {
        if (!root.view) return
        root.view.tagsMatchAny = root.matchAny
        root.view.tags = root.selected.slice()
    }

    // ── describe (semantic) search ─────────────────────────────────────────────
    // semanticState mirrors the daemon: ready | indexing | noModel | disabled | unavailable.
    property string semanticState: ""
    property int semIndexed: 0
    property int semTotal: 0
    property string semDetail: ""
    // queryState: "" idle | searching | ready | error, for the running describe query.
    property string queryState: ""
    property string queryError: ""
    property int semanticMs: 0
    property string describeText: ""
    property int semanticRequest: 0
    property bool applyingSemanticResults: false

    readonly property bool describeReady: root.semanticState === "ready"
        || (root.semanticState === "" && root.lensAvailable)
    readonly property bool describeIndexing: root.semanticState === "indexing"
    // A field is shown while search can run or is preparing; otherwise a setup notice takes its place.
    readonly property bool describeFieldShown: !root.describe || root.describeReady || root.describeIndexing

    // One filter after a short pause keeps the text caret ahead of model sorting and the card roll.
    Timer { id: queryDebounce; interval: 140; onTriggered: root._commitTextQuery() }
    function _commitTextQuery() {
        queryDebounce.stop()
        if (root.view && !root.describe)
            root.view.query = queryInput.text
    }

    Timer { id: describeDebounce; interval: 240; onTriggered: root._runSemantic() }
    function _runSemantic() {
        if (!root.view) return
        var q = root.describeText.trim()
        var request = ++root.semanticRequest
        if (q.length === 0) { root.view.keyOrder = []; root.queryState = ""; return }
        root.queryState = "searching"
        var t0 = Date.now()
        Daemon.call("semantic.query", { query: q }, function (r, e) {
            if (request !== root.semanticRequest || !root.describe || q !== root.describeText.trim())
                return
            if (e) { root.queryState = "error"; root.queryError = (e && e.message) ? e.message : ""; return }
            var res = (r && r.results) ? r.results : []
            var keys = []
            for (var i = 0; i < res.length; ++i) keys.push(res[i].key)
            root.applyingSemanticResults = true
            root.queryState = "ready"
            root.view.keyOrder = keys
            root.applyingSemanticResults = false
            root.semanticMs = Date.now() - t0
        })
    }
    function _applyStatus(r) {
        if (!r) return
        if (r.state !== undefined) root.semanticState = String(r.state || "")
        if (r.indexed !== undefined) root.semIndexed = Number(r.indexed) || 0
        if (r.total !== undefined) root.semTotal = Number(r.total) || 0
        if (r.detail !== undefined) root.semDetail = String(r.detail || "")
    }
    function _refreshStatus() {
        Daemon.call("semantic.status", {}, function (r, e) { if (!e) root._applyStatus(r) })
    }

    function _describeStatusText() {
        if (root.describeIndexing) return root.semTotal > 0 ? (root.semIndexed + " / " + root.semTotal) : I18n.tr("Preparing")
        if (root.queryState === "searching") return I18n.tr("Searching")
        if (root.queryState === "error") return root.queryError !== "" ? root.queryError : I18n.tr("Search failed")
        if (root.queryState === "ready") return root.semanticMs + " " + I18n.tr("ms")
        return ""
    }
    function _describeStatusColor() {
        if (root.queryState === "error") return Theme.tertiary
        if (root.describeIndexing || root.queryState === "searching") return Theme.withAlpha(Theme.surfaceText, 0.8)
        return Theme.withAlpha(Theme.surfaceText, 0.5)
    }

    function _setMode(m) {
        if (root.state) root.state.searchMode = m
    }
    function _close() { if (root.state) root.state.searchOpen = false }
    function _openModelSetup() {
        if (!root.state) return
        root._close()
        root.state.openSheet("settings", { tab: "picker", section: "filter", control: "semantic.models.import" })
    }
    function _enableSemantic() { Settings.set("semantic.enabled", true); root._refreshStatus() }

    // Field key handling routes to the active mode's target.
    function _fieldEnter() {
        if (root.describe) { describeDebounce.stop(); root._runSemantic(); return }
        queryDebounce.stop()
        var frag = queryInput.text.trim().toLowerCase()
        if (frag.length > 0) {
            var t = root._firstTag(frag)
            if (t !== "") {
                root._toggle(t, false)
                queryInput.text = ""
                if (root.view) root.view.query = ""
                return
            }
        }
        root._commitTextQuery()
        root._close()
    }
    function _fieldTab(event) {
        if (event.modifiers & Qt.ControlModifier) { event.accepted = false; return }
        event.accepted = true
        if (root.describe) return
        var frag = queryInput.text.trim().toLowerCase()
        if (frag.length === 0) return
        var t = root._firstTag(frag)
        if (t !== "") {
            queryDebounce.stop()
            queryInput.text = t
            queryInput.cursorPosition = t.length
            if (root.view) root.view.query = t
        }
    }

    onShownChanged: {
        if (!root.shown) return
        if (root.state && !root.state.searchMode)
            root.state.searchMode = (root.svDefaultMode.value || "tags")
        root._aggregate()
        root._refreshStatus()
        queryInput.text = root.describe ? root.describeText : (root.view ? root.view.query : "")
        Qt.callLater(function () { queryInput.forceActiveFocus(); queryInput.selectAll() })
    }
    onDescribeChanged: {
        queryDebounce.stop()
        describeDebounce.stop()
        ++root.semanticRequest
        queryInput.text = root.describe ? root.describeText : (root.view ? root.view.query : "")
        if (!root.view) return
        if (!root.describe) {
            // Leaving describe: drop the semantic ordering so tag and query filters take over.
            root.view.keyOrder = []
        } else if (root.describeText.trim().length > 0) {
            describeDebounce.restart()
        } else {
            root.view.keyOrder = []
        }
        if (root.shown) Qt.callLater(function () { queryInput.forceActiveFocus() })
    }

    Connections {
        target: Library
        function onChanged(collection) {
            if (root.view && collection === root.view.collection) root._aggregate()
        }
    }
    Connections {
        target: Daemon
        function onEvent(name, data) { if (name === "ryogami.semantic.status") root._applyStatus(data) }
        function onReconnected() { root._refreshStatus() }
    }
    Connections {
        target: root.view
        ignoreUnknownSignals: true
        function onCollectionChanged() { root._aggregate() }
        function onTagsChanged() { if (root.view.tags.length === 0 && root.selected.length > 0) root.selected = [] }
        function onQueryChanged() {
            if (!root.describe && root.view.query === "" && queryInput.text !== "") queryInput.text = ""
        }
        // The masthead clear empties keyOrder directly; an empty result from the active request does not.
        function onKeyOrderChanged() {
            if (!root.applyingSemanticResults && root.view.keyOrder.length === 0 && root.describeText !== "") {
                ++root.semanticRequest
                root.describeText = ""
                if (root.describe && queryInput.text !== "") queryInput.text = ""
                root.queryState = ""
            }
        }
    }
    Component.onCompleted: root._refreshStatus()

    // ── the panel ─────────────────────────────────────────────────────────────
    Rectangle {
        id: panel
        width: root.panelWidth
        height: Math.max(root._num(svHeight, 168) * Theme.scale, body.y + body.implicitHeight + root._padY)
        x: Math.max(8 * Theme.scale,
            Math.min(root._mastheadRight - width + Number(root.svOffX.value || 0), root.width - width - 8 * Theme.scale))
        y: root._mastheadTop + root._mastheadHeight + 8 * Theme.scale + Number(root.svOffY.value || 0)

        radius: Theme.radius
        color: Theme.withAlpha(Theme.surface, 0.97)
        border.width: 1
        border.color: Theme.withAlpha(Theme.outline, 0.5)

        opacity: root.reveal
        transformOrigin: Item.Top
        transform: [
            Translate { y: (1 - root.reveal) * -14 * Theme.scale },
            Scale { origin.x: panel.width; origin.y: 0; xScale: 0.98 + 0.02 * root.reveal; yScale: 0.94 + 0.06 * root.reveal }
        ]

        // Keeps clicks and hover on the panel: an empty click on the scene closes the picker, and
        // hover reaching the cards below recolours the desktop through the live palette preview.
        MouseArea { anchors.fill: parent; hoverEnabled: true; acceptedButtons: Qt.AllButtons }

        Column {
            id: body
            x: root._padX
            y: root._padY
            width: panel.width - 2 * root._padX
            spacing: 11 * Theme.scale

            // ── header: live count and the running describe status ────────────
            Item {
                width: parent.width
                height: 30 * Theme.scale

                Row {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 7 * Theme.scale

                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: String(root.view ? root.view.count : 0)
                        font.family: Theme.display
                        font.pixelSize: Theme.fs(22)
                        font.weight: Font.Normal
                        font.features: ({ "tnum": 1, "lnum": 1 })
                        color: Theme.surfaceText
                        renderType: Text.NativeRendering
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.verticalCenterOffset: 2 * Theme.scale
                        text: root.view && root.view.collection === "workshop" ? I18n.tr("scenes")
                            : root.view && root.view.collection === "themes" ? I18n.tr("themes")
                            : root.view && root.view.collection === "rices" ? I18n.tr("rices")
                            : I18n.tr("matches")
                        font.family: Theme.sans
                        font.pixelSize: Theme.fs(9.5)
                        font.letterSpacing: 1.4
                        font.capitalization: Font.AllUppercase
                        color: Theme.withAlpha(Theme.surfaceText, 0.5)
                        renderType: Text.NativeRendering
                    }
                }

                Text {
                    visible: root.describe && text !== ""
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: root._describeStatusText()
                    elide: Text.ElideRight
                    width: Math.min(implicitWidth, parent.width * 0.5)
                    horizontalAlignment: Text.AlignRight
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fs(10.5)
                    color: root._describeStatusColor()
                    renderType: Text.NativeRendering
                }
            }

            // ── mode switch and (tags) the match segment ──────────────────────
            Item {
                width: parent.width
                height: 32 * Theme.scale

                ModeSwitch {
                    id: modeSwitch
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    current: root.mode
                    onPicked: (m) => root._setMode(m)
                }

                MiniSeg {
                    visible: !root.describe
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    current: root.matchAny ? "any" : "all"
                    options: [
                        { v: "all", label: I18n.tr("Match all") },
                        { v: "any", label: I18n.tr("Match any") }
                    ]
                    onPicked: (v) => { root.matchAny = (v === "any"); root._applyTags() }
                }
            }

            // ── the field, shared by both modes ───────────────────────────────
            Rectangle {
                id: fieldBox
                visible: root.describeFieldShown
                width: parent.width
                height: 46 * Theme.scale
                radius: Theme.radius
                color: Theme.withAlpha(Theme.surfaceText, 0.05)
                border.width: 1
                border.color: queryInput.activeFocus
                    ? Theme.withAlpha(Theme.surfaceText, 0.55)
                    : Theme.withAlpha(Theme.outline, 0.4)
                Behavior on border.color { ColorAnimation { duration: Theme.fast } }

                Text {
                    id: fieldGlyph
                    anchors.left: parent.left
                    anchors.leftMargin: 13 * Theme.scale
                    anchors.verticalCenter: parent.verticalCenter
                    text: "\u{f0349}"
                    font.family: Theme.icon
                    font.pixelSize: Theme.fs(16)
                    color: queryInput.activeFocus ? Theme.surfaceText : Theme.withAlpha(Theme.surfaceText, 0.55)
                    rotation: queryInput.activeFocus ? -8 : 0
                    Behavior on rotation { NumberAnimation { duration: Theme.standard; easing.type: Easing.OutBack } }
                    Behavior on color { ColorAnimation { duration: Theme.fast } }
                    renderType: Text.NativeRendering
                }

                TextInput {
                    id: queryInput
                    anchors.left: fieldGlyph.right
                    anchors.leftMargin: 11 * Theme.scale
                    anchors.right: parent.right
                    anchors.rightMargin: 13 * Theme.scale
                    anchors.verticalCenter: parent.verticalCenter
                    enabled: !root.describe || root.describeReady
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fs(14)
                    color: Theme.surfaceText
                    selectionColor: Theme.withAlpha(Theme.surfaceText, 0.22)
                    selectedTextColor: Theme.surfaceText
                    selectByMouse: true
                    clip: true
                    renderType: Text.NativeRendering

                    onTextEdited: {
                        if (root.describe) {
                            ++root.semanticRequest
                            root.describeText = text
                            describeDebounce.restart()
                        } else if (root.view) {
                            queryDebounce.restart()
                        }
                    }
                    Keys.onEscapePressed: root._close()
                    Keys.onReturnPressed: root._fieldEnter()
                    Keys.onEnterPressed: root._fieldEnter()
                    Keys.onTabPressed: (event) => root._fieldTab(event)

                    Text {
                        anchors.fill: parent
                        verticalAlignment: Text.AlignVCenter
                        visible: queryInput.text.length === 0
                        text: root.describe ? I18n.tr("Describe the wallpaper you want")
                                            : I18n.tr("Search tags, names and folders")
                        font: queryInput.font
                        color: Theme.withAlpha(Theme.surfaceText, 0.4)
                        elide: Text.ElideRight
                        renderType: Text.NativeRendering
                    }
                }
            }

            // ── tags mode: hint, chips, selection summary ─────────────────────
            Text {
                visible: !root.describe
                width: parent.width
                text: I18n.tr("Tab completes · Enter adds the tag · right-click a chip to exclude it")
                font.family: Theme.sans
                font.pixelSize: Theme.fs(10)
                color: Theme.withAlpha(Theme.surfaceText, 0.5)
                elide: Text.ElideRight
                renderType: Text.NativeRendering
            }

            Flickable {
                visible: !root.describe
                width: parent.width
                height: root.cloudRows * 28 * Theme.scale + (root.cloudRows - 1) * 6 * Theme.scale
                contentHeight: cloud.implicitHeight
                clip: true
                boundsBehavior: Flickable.StopAtBounds

                Flow {
                    id: cloud
                    width: parent.width
                    spacing: 6 * Theme.scale

                    Repeater {
                        model: root.tagList
                        delegate: QuietChip {
                            required property var modelData
                            chipTag: modelData.tag
                            chipCount: modelData.count
                        }
                    }

                    Text {
                        visible: root.tagList.length === 0
                        text: I18n.tr("No tags yet")
                        font.family: Theme.sans
                        font.weight: Font.Medium
                        font.pixelSize: Theme.fs(13)
                        color: Theme.withAlpha(Theme.surfaceText, 0.6)
                        renderType: Text.NativeRendering
                    }
                }
            }

            Item {
                visible: !root.describe
                width: parent.width
                height: 26 * Theme.scale

                Row {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 8 * Theme.scale
                    visible: root.selected.length > 0
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.selected.length + " " + I18n.tr("selected")
                        font.family: Theme.sans
                        font.weight: Font.Medium
                        font.pixelSize: Theme.fs(10.5)
                        color: Theme.withAlpha(Theme.surfaceText, 0.7)
                        renderType: Text.NativeRendering
                    }
                    TinyButton { label: I18n.tr("Clear"); onTriggered: root._clear() }
                }

                MiniSeg {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    current: root.orderPopular ? "pop" : "az"
                    options: [
                        { v: "pop", label: I18n.tr("Popular") },
                        { v: "az", label: I18n.tr("A-Z") }
                    ]
                    onPicked: (v) => root.orderPopular = (v === "pop")
                }
            }

            // ── describe mode: ready hint or honest setup notice ──────────────
            Text {
                visible: root.describe && root.describeReady && !root.describeIndexing
                width: parent.width
                text: I18n.tr("Describe a wallpaper in words. The model runs on this machine and finds the closest matches.")
                wrapMode: Text.WordWrap
                font.family: Theme.sans
                font.pixelSize: Theme.fs(11)
                color: Theme.withAlpha(Theme.surfaceText, 0.55)
                renderType: Text.NativeRendering
            }

            Text {
                visible: root.describe && root.describeIndexing
                width: parent.width
                text: root.semTotal > 0
                    ? I18n.tr("Reading your wallpapers so you can describe them. %1 of %2 done.").arg(root.semIndexed).arg(root.semTotal)
                    : I18n.tr("Reading your wallpapers so you can describe them.")
                wrapMode: Text.WordWrap
                font.family: Theme.sans
                font.pixelSize: Theme.fs(11)
                color: Theme.withAlpha(Theme.surfaceText, 0.55)
                renderType: Text.NativeRendering
            }

            SetupNotice {
                visible: root.describe && !root.describeFieldShown
                width: parent.width
            }
        }
    }

    // ── the Tags / Describe switch, a bone plate that slides to the chosen word ──
    component ModeSwitch: Item {
        id: ms
        property string current: "tags"
        signal picked(string mode)

        readonly property var opts: [
            { v: "tags", label: I18n.tr("Tags") },
            { v: "describe", label: I18n.tr("Describe") }
        ]
        property real plateX: 0
        property real plateW: 0

        implicitHeight: 32 * Theme.scale
        implicitWidth: msRow.implicitWidth + 6 * Theme.scale

        Rectangle {
            anchors.fill: parent
            radius: Theme.radius
            color: Theme.withAlpha(Theme.surfaceText, 0.06)
            border.width: 1
            border.color: Theme.withAlpha(Theme.outline, 0.35)
        }
        Rectangle {
            x: ms.plateX
            width: ms.plateW
            anchors.verticalCenter: parent.verticalCenter
            height: parent.height - 6 * Theme.scale
            radius: Theme.radius - 1
            color: Theme.surfaceText
            Behavior on x { NumberAnimation { duration: Theme.standard; easing.type: Easing.OutBack; easing.overshoot: 1.1 } }
            Behavior on width { NumberAnimation { duration: Theme.standard; easing.type: Easing.OutCubic } }
        }
        Row {
            id: msRow
            x: 3 * Theme.scale
            height: parent.height
            Repeater {
                model: ms.opts
                delegate: Item {
                    id: opt
                    required property var modelData
                    readonly property bool active: ms.current === modelData.v
                    readonly property bool hovered: optMouse.containsMouse
                    width: cut.width + 30 * Theme.scale
                    height: msRow.height

                    TextMetrics {
                        id: cut
                        font.family: Theme.sans
                        font.weight: Font.DemiBold
                        font.pixelSize: Theme.fs(12.5)
                        text: opt.modelData.label
                    }
                    Binding { target: ms; property: "plateX"; value: msRow.x + opt.x; when: opt.active; restoreMode: Binding.RestoreNone }
                    Binding { target: ms; property: "plateW"; value: opt.width; when: opt.active; restoreMode: Binding.RestoreNone }

                    Text {
                        anchors.centerIn: parent
                        text: opt.modelData.label
                        font.family: Theme.sans
                        font.weight: opt.active ? Font.DemiBold : Font.Medium
                        font.pixelSize: Theme.fs(12.5)
                        color: opt.active ? Theme.surface
                             : opt.hovered ? Theme.surfaceText
                             : Theme.withAlpha(Theme.surfaceText, 0.6)
                        Behavior on color { ColorAnimation { duration: Theme.fast } }
                        renderType: Text.NativeRendering
                    }
                    MouseArea {
                        id: optMouse
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: ms.picked(opt.modelData.v)
                    }
                }
            }
        }
    }

    // ── a compact two-option pill: the chosen word inverts to a bone plate ──
    component MiniSeg: Row {
        id: seg
        property var options: []
        property string current: ""
        signal picked(string value)
        spacing: 3 * Theme.scale

        Repeater {
            model: seg.options
            delegate: Rectangle {
                id: cell
                required property var modelData
                readonly property bool active: seg.current === modelData.v
                readonly property bool hovered: cellMouse.containsMouse
                height: 24 * Theme.scale
                width: cellText.implicitWidth + 18 * Theme.scale
                radius: Theme.radius
                color: cell.active ? Theme.surfaceText
                     : cell.hovered ? Theme.withAlpha(Theme.surfaceText, 0.09)
                     : Theme.withAlpha(Theme.surfaceText, 0.045)
                border.width: 1
                border.color: cell.active ? Theme.surfaceText : Theme.withAlpha(Theme.outline, 0.35)
                Behavior on color { ColorAnimation { duration: Theme.fast } }
                Text {
                    id: cellText
                    anchors.centerIn: parent
                    text: cell.modelData.label
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fs(10)
                    color: cell.active ? Theme.surface : Theme.withAlpha(Theme.surfaceText, cell.hovered ? 0.9 : 0.65)
                    renderType: Text.NativeRendering
                }
                MouseArea {
                    id: cellMouse
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: seg.picked(cell.modelData.v)
                }
            }
        }
    }

    // ── one tag chip: quiet by default, a bone plate when included, an outline when excluded ──
    component QuietChip: Rectangle {
        id: qc
        property string chipTag: ""
        property int chipCount: 0
        readonly property string tagState: root._tagState(qc.chipTag)
        readonly property bool included: tagState === "include"
        readonly property bool excluded: tagState === "exclude"
        readonly property bool hovered: qcMouse.containsMouse

        height: 28 * Theme.scale
        implicitWidth: qcRow.implicitWidth + 20 * Theme.scale
        width: implicitWidth
        radius: Theme.radius
        color: qc.included ? Theme.surfaceText
             : qc.excluded ? Theme.withAlpha(Theme.tertiary, 0.14)
             : qc.hovered ? Theme.withAlpha(Theme.surfaceText, 0.09)
             : Theme.withAlpha(Theme.surfaceText, 0.045)
        border.width: 1
        border.color: qc.included ? Theme.surfaceText
                    : qc.excluded ? Theme.withAlpha(Theme.tertiary, 0.7)
                    : Theme.withAlpha(Theme.outline, 0.38)
        Behavior on color { ColorAnimation { duration: Theme.fast } }

        Row {
            id: qcRow
            anchors.centerIn: parent
            spacing: 5 * Theme.scale
            Text {
                visible: qc.excluded
                anchors.verticalCenter: parent.verticalCenter
                text: "\u2212"
                font.family: Theme.sans
                font.weight: Font.DemiBold
                font.pixelSize: Theme.fs(12)
                color: Theme.tertiary
                renderType: Text.NativeRendering
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: qc.chipTag
                font.family: Theme.sans
                font.weight: qc.included ? Font.DemiBold : Font.Medium
                font.pixelSize: Theme.fs(12)
                color: qc.included ? Theme.surface
                     : qc.excluded ? Theme.tertiary
                     : Theme.withAlpha(Theme.surfaceText, 0.82)
                renderType: Text.NativeRendering
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: qc.chipCount >= 1000 ? (Math.round(qc.chipCount / 100) / 10) + "k" : qc.chipCount
                font.family: Theme.display
                font.pixelSize: Theme.fs(11)
                font.features: ({ "tnum": 1, "lnum": 1 })
                color: qc.included ? Theme.withAlpha(Theme.surface, 0.7) : Theme.withAlpha(Theme.surfaceText, 0.5)
                renderType: Text.NativeRendering
            }
        }
        SequentialAnimation {
            id: qcPop
            NumberAnimation { target: qc; property: "scale"; to: 1.1; duration: Theme.fast * 0.4; easing.type: Easing.OutQuad }
            NumberAnimation { target: qc; property: "scale"; to: 1; duration: Theme.fast; easing.type: Easing.OutBack; easing.overshoot: 3 }
        }
        onIncludedChanged: if (qc.included) qcPop.restart()
        MouseArea {
            id: qcMouse
            anchors.fill: parent
            hoverEnabled: true
            acceptedButtons: Qt.LeftButton | Qt.RightButton
            cursorShape: Qt.PointingHandCursor
            onClicked: (mouse) => root._toggle(qc.chipTag, mouse.button === Qt.RightButton)
        }
    }

    // ── a small text button, inversion on hover ──
    component TinyButton: Rectangle {
        id: tb
        property string label: ""
        signal triggered()
        readonly property bool hovered: tbMouse.containsMouse
        height: 22 * Theme.scale
        implicitWidth: tbText.implicitWidth + 16 * Theme.scale
        width: implicitWidth
        radius: Theme.radius
        color: tb.hovered ? Theme.surfaceText : Theme.withAlpha(Theme.surfaceText, 0.06)
        border.width: 1
        border.color: tb.hovered ? Theme.surfaceText : Theme.withAlpha(Theme.outline, 0.35)
        Behavior on color { ColorAnimation { duration: Theme.fast } }
        Text {
            id: tbText
            anchors.centerIn: parent
            text: tb.label
            font.family: Theme.sans
            font.weight: Font.Medium
            font.pixelSize: Theme.fs(10)
            color: tb.hovered ? Theme.surface : Theme.withAlpha(Theme.surfaceText, 0.75)
            renderType: Text.NativeRendering
        }
        MouseArea {
            id: tbMouse
            anchors.fill: parent
            hoverEnabled: true
            cursorShape: Qt.PointingHandCursor
            onClicked: tb.triggered()
        }
    }

    // ── the honest describe-search setup notice ──
    component SetupNotice: Column {
        id: notice
        spacing: 8 * Theme.scale

        readonly property string title: root.semanticState === "noModel"
                ? I18n.tr("Describe search needs a vision model")
            : root.semanticState === "disabled"
                ? I18n.tr("Describe search is turned off")
                : I18n.tr("Describe search is not available here")
        readonly property string body: root.semanticState === "noModel"
                ? I18n.tr("Import a model pack once. Ryogami reads your wallpapers on this machine so you can search them by describing them. Tag search works without it.")
            : root.semanticState === "disabled"
                ? I18n.tr("Turn it on to search wallpapers by describing them. Tag search stays available either way.")
                : I18n.tr("The Lens component (skwd-lens and its runtime) is not installed on this system, so wallpapers cannot be read for description. Tag search works without it.")

        Text {
            width: parent.width
            text: notice.title
            wrapMode: Text.WordWrap
            font.family: Theme.sans
            font.weight: Font.DemiBold
            font.pixelSize: Theme.fs(13)
            color: Theme.surfaceText
            renderType: Text.NativeRendering
        }
        Text {
            width: parent.width
            text: notice.body
            wrapMode: Text.WordWrap
            font.family: Theme.sans
            font.pixelSize: Theme.fs(11)
            color: Theme.withAlpha(Theme.surfaceText, 0.6)
            renderType: Text.NativeRendering
        }
        SetupButton {
            visible: root.semanticState === "noModel" || root.semanticState === "disabled"
            glyph: root.semanticState === "noModel" ? "\u{f02fa}" : ""
            label: root.semanticState === "noModel" ? I18n.tr("Import a model") : I18n.tr("Turn on")
            onTriggered: root.semanticState === "noModel" ? root._openModelSetup() : root._enableSemantic()
        }
    }

    // ── the primary setup action: a bone plate that inverts, with a small pop ──
    component SetupButton: Rectangle {
        id: sb
        property string label: ""
        property string glyph: ""
        signal triggered()
        readonly property bool hovered: sbMouse.containsMouse
        height: 32 * Theme.scale
        implicitWidth: sbRow.implicitWidth + 26 * Theme.scale
        width: implicitWidth
        radius: Theme.radius
        color: sb.hovered ? Theme.withAlpha(Theme.surfaceText, 0.92) : Theme.surfaceText
        scale: sbMouse.pressed ? 0.95 : 1
        Behavior on scale { NumberAnimation { duration: Theme.fast; easing.type: Easing.OutBack; easing.overshoot: 2.2 } }
        Row {
            id: sbRow
            anchors.centerIn: parent
            spacing: 8 * Theme.scale
            Text {
                anchors.verticalCenter: parent.verticalCenter
                visible: sb.glyph.length > 0
                text: sb.glyph
                font.family: Theme.icon
                font.pixelSize: Theme.fs(14)
                color: Theme.surface
                renderType: Text.NativeRendering
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: sb.label
                font.family: Theme.sans
                font.weight: Font.DemiBold
                font.pixelSize: Theme.fs(12)
                color: Theme.surface
                renderType: Text.NativeRendering
            }
        }
        MouseArea {
            id: sbMouse
            anchors.fill: parent
            hoverEnabled: true
            cursorShape: Qt.PointingHandCursor
            onClicked: sb.triggered()
        }
    }
}

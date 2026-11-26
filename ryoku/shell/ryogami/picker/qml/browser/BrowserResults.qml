import QtQuick
import Lithic as Lc
import Ryoku.Ui.Singletons
import "KeyChord.js" as KeyChord

Item {
    id: results

    property var source: null
    property var settings: null
    property var sources: null
    property string provider: ""
    property string sourceLabel: ""
    property string query: ""
    property string searchPlaceholder: ""
    property string sortText: ""
    property bool searchable: true
    property int columns: 6
    property var palette: ({})
    property real reveal: 1
    property bool cancellable: false
    property string emptyText: ""
    property bool autoLoad: true
    property bool live: false

    property bool pendingApply: false
    property var applyItem: null
    property var applyingId: undefined

    signal previewRequested(int row)
    signal downloadRequested(int row)
    signal applyRequested(int row)
    signal cancelRequested(int row)
    signal queryEdited(string text)
    signal searchSubmitted(string text)

    readonly property alias field: cardField
    readonly property bool searchEditing: searchInput.editing

    property int active: 0
    property int queued: 0
    property var activeItem: null
    property var queuedItem: null



    function _isApplyKey(event) {
        var trigger = results.settings ? results.settings.value("keys.apply") : ""
        return KeyChord.signature(event) === KeyChord.canon(trigger || "enter")
    }

    function recount() {
        if (!results.source || !results.sources) return
        var a = 0, q = 0, ai = null, qi = null
        for (var i = 0; i < results.source.count; ++i) {
            var it = results.source.get(i)
            if (!it) continue
            if (results.sources.isQueued(it)) { q++; if (!qi) qi = it }
            else if (results.sources.isDownloading(it)) { a++; if (!ai) ai = it }
        }
        results.active = a
        results.queued = q
        results.activeItem = ai
        results.queuedItem = qi
    }

    function loadNext(reason) {
        var s = results.source
        if (!s || s.loading || s.page >= s.lastPage)
            return
        s.nextPage()
    }

    function _maybePage() {
        var s = results.source
        if (!results.autoLoad || !s || s.loading || s.page >= s.lastPage)
            return
        var end = cardField.visibleEnd
        var probe = (end !== undefined) ? end : cardField.currentIndex
        if (probe + Math.max(4, results.columns) >= s.count)
            results.loadNext("auto")
    }

    Connections {
        target: results.source
        ignoreUnknownSignals: true
        function onCountChanged() {
            results.recount()
            results._maybePage()
        }
        function onLoadingChanged() { results.recount() }
        function onDownloadChanged(row, tallyChanged) {
            if (tallyChanged) {
                results.recount()
            } else {
                var changed = results.source.get(row)
                if (changed && results.activeItem
                        && String(changed.id) === String(results.activeItem.id))
                    results.activeItem = changed
                if (changed && results.queuedItem
                        && String(changed.id) === String(results.queuedItem.id))
                    results.queuedItem = changed
            }
            if (actions.row === row)
                actions.refreshItem()
        }
    }


    Item {
        anchors.fill: parent
        clip: true

        Column {
            anchors.fill: parent
            spacing: 0

            Item {
                id: header
                width: parent.width
                height: 54 * Theme.scale

                Lc.CutRect {
                    id: searchBox
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width - (sortPlate.visible ? sortPlate.width + 10 * Theme.scale : 0)
                    height: 44 * Theme.scale
                    cut: Lc.LithicTokens.cut.small * Theme.scale
                    fill: Lc.Lithic.field
                    strokeWidth: searchInput.editing ? 2 : 1
                    strokeColor: searchInput.editing ? Lc.Lithic.sel : Lc.Lithic.alpha(Lc.Lithic.fg, 0.1)
                    Behavior on strokeColor { ColorAnimation { duration: Theme.fast } }

                    Lc.Glyph {
                        id: lens
                        x: 14 * Theme.scale
                        anchors.verticalCenter: parent.verticalCenter
                        name: "search"
                        size: 20
                        ink: searchInput.editing ? Lc.Lithic.fg : Lc.Lithic.fg3
                    }
                    TextField {
                        id: searchInput
                        visible: results.searchable
                        anchors.left: lens.right
                        anchors.right: parent.right
                        anchors.top: parent.top
                        anchors.bottom: parent.bottom
                        anchors.leftMargin: 4 * Theme.scale
                        variant: "ghost"
                        placeholder: results.searchPlaceholder
                        text: results.query
                        onEdited: function(t) { results.queryEdited(t) }
                        onCommitted: function(t) { results.searchSubmitted(t) }
                    }
                    Text {
                        visible: !results.searchable
                        anchors.left: lens.right
                        anchors.leftMargin: 12 * Theme.scale
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Picked fresh every day")
                        color: Lc.Lithic.fg3
                        font.family: Lc.Lithic.font.sans
                        font.pixelSize: Theme.fs(14)
                        renderType: Text.NativeRendering
                    }
                }

                Lc.CutRect {
                    id: sortPlate
                    visible: results.sortText.length > 0
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    width: visible ? sortRow.implicitWidth + 26 * Theme.scale : 0
                    height: 44 * Theme.scale
                    cut: Lc.LithicTokens.cut.small * Theme.scale
                    fill: Lc.Lithic.alpha(Lc.Lithic.fg, 0.05)
                    Row {
                        id: sortRow
                        anchors.centerIn: parent
                        spacing: 8 * Theme.scale
                        Lc.Glyph { name: "sort"; size: 16; ink: Lc.Lithic.fg3; anchors.verticalCenter: parent.verticalCenter }
                        Text {
                            id: sortLabel
                            anchors.verticalCenter: parent.verticalCenter
                            text: results.sortText
                            color: Lc.Lithic.fg2
                            font.family: Lc.Lithic.font.sans
                            font.pixelSize: Theme.fs(13)
                            font.weight: Font.Medium
                            renderType: Text.NativeRendering
                        }
                    }
                }
            }

            Item {
                id: gridArea
                width: parent.width
                height: parent.height - y - footer.height


                BrowserStatus {
                    id: strip
                    z: 4
                    anchors.top: parent.top
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.topMargin: 8 * Theme.scale
                    anchors.leftMargin: 8 * Theme.scale
                    anchors.rightMargin: 8 * Theme.scale
                    sources: results.sources
                    error: results.source ? results.source.error : ""
                    loading: results.source ? results.source.loading : false
                    sourceLabel: results.sourceLabel
                    count: results.source ? results.source.count : 0
                    pendingApply: results.pendingApply
                    applyItem: results.applyItem
                    active: results.active
                    queued: results.queued
                    activeItem: results.activeItem
                    queuedItem: results.queuedItem
                }
                CardField {
                    id: cardField
                    anchors.fill: parent
                    anchors.rightMargin: scrollRail.visible ? 14 * Theme.scale : 0
                    source: results.source
                    settings: results.settings
                    mode: "wall"
                    columns: results.columns
                    palette: results.palette
                    interactive: true
                    active: results.visible && (results.source ? results.source.count > 0 : false)
                    previewAllowed: active && !searchInput.activeFocus
                        && !(results.source && results.source.loading)
                    activateOnClick: true
                    visible: results.source ? results.source.count > 0 : false
                    Keys.priority: Keys.BeforeItem
                    Keys.onPressed: function(event) {
                        if (results._isApplyKey(event) && cardField.currentIndex >= 0) {
                            results.applyRequested(cardField.currentIndex)
                            event.accepted = true
                        }
                    }
                    onActivated: function(row) { results.previewRequested(row) }
                }

                BrowserScrollRail {
                    id: scrollRail
                    anchors.top: parent.top
                    anchors.bottom: parent.bottom
                    anchors.right: parent.right
                    anchors.topMargin: 10 * Theme.scale
                    anchors.bottomMargin: 10 * Theme.scale
                    anchors.rightMargin: 4 * Theme.scale
                    field: cardField
                }

                Item {
                    id: actionLayer
                    anchors.fill: cardField
                    clip: true
                    z: 2

                    BrowserCardBadges {
                        anchors.fill: parent
                        field: cardField
                        source: results.source
                        live: results.live
                        columns: results.columns
                    }

                    Item {
                        id: actionAnchor
                        property rect box: {
                            var row = actions.row
                            var scrollTick = cardField.scrollPosition
                            var widthTick = cardField.width
                            var heightTick = cardField.height
                            var layoutTick = cardField.currentRect
                            if (row < 0 || !cardField.visible)
                                return Qt.rect(0, 0, 0, 0)
                            var local = cardField.rectOf(row)
                            var origin = cardField.mapToItem(actionLayer, Qt.point(local.x, local.y))
                            return Qt.rect(origin.x, origin.y, local.width, local.height)
                        }
                        x: box.x
                        y: box.y
                        width: box.width
                        height: box.height
                        visible: actions.row >= 0 && width > 0 && height > 0

                        BrowserCardActions {
                            id: actions
                            anchors.fill: parent
                            property int row: -1
                            function refreshItem() {
                                item = (row >= 0 && results.source) ? results.source.get(row) : null
                            }
                            sources: results.sources
                            applying: results.applyingId !== undefined && !!item
                                && String(results.applyingId) === String(item.id)
                            reveal: results.reveal
                            cancellable: results.cancellable
                            onSave: if (row >= 0) results.downloadRequested(row)
                            onApply: if (row >= 0) results.applyRequested(row)
                            onCancel: if (row >= 0) results.cancelRequested(row)
                        }
                    }
                }

                Connections {
                    target: cardField
                    ignoreUnknownSignals: true
                    function onHoveredIndexChanged() {
                        if (cardField.hoveredIndex >= 0) {
                            actions.row = cardField.hoveredIndex
                            actions.refreshItem()
                        } else if (!actions.hovering) {
                            actions.row = -1
                            actions.item = null
                        }
                    }
                    function onCurrentIndexChanged() { results._maybePage() }
                    function onVisibleEndChanged() { results._maybePage() }
                }
                Connections {
                    target: actions
                    function onHoveringChanged() {
                        if (!actions.hovering && cardField.hoveredIndex < 0) {
                            actions.row = -1
                            actions.item = null
                        }
                    }
                }

                BrowserSpinner {
                    anchors.centerIn: parent
                    visible: results.source ? (results.source.loading && results.source.count === 0) : false
                    size: 72
                    color: Theme.surfaceText
                }
                Text {
                    anchors.centerIn: parent
                    width: parent.width - 40 * Theme.scale
                    horizontalAlignment: Text.AlignHCenter
                    wrapMode: Text.WordWrap
                    visible: results.source
                        ? (!results.source.loading && results.source.count === 0)
                        : true
                    text: (results.source && results.source.error && results.source.error.length > 0)
                        ? results.source.error : results.emptyText
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fontLabel
                    color: Theme.withAlpha(Theme.surfaceText, 0.45)
                    renderType: Text.NativeRendering
                }
            }

            Item {
                id: footer
                width: parent.width
                height: 34 * Theme.scale
                readonly property bool more: !!results.source && results.source.page < results.source.lastPage
                readonly property bool loading: !!results.source && results.source.loading && results.source.count > 0
                Row {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 8 * Theme.scale
                    BrowserSpinner {
                        visible: footer.loading
                        anchors.verticalCenter: parent.verticalCenter
                        size: 16
                        color: Lc.Lithic.fg2
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: footer.loading ? I18n.tr("Loading more")
                            : !results.source || results.source.count === 0 ? ""
                            : footer.more ? I18n.tr("%1 shown, more as you scroll").arg(results.source.count)
                            : I18n.tr("All %1 shown").arg(results.source.count)
                        color: Lc.Lithic.fg3
                        font.family: Lc.Lithic.font.sans
                        font.pixelSize: Theme.fs(12.5)
                        renderType: Text.NativeRendering
                    }
                }
                FolioAction {
                    visible: !results.autoLoad && footer.more
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    label: I18n.tr("Load more")
                    enabled: !footer.loading
                    onTriggered: results.loadNext("button")
                }
            }
        }
    }
}

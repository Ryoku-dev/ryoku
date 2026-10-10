import QtQuick
import Ryoku.Ui.Singletons

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
    property bool showApply: false
    property var palette: ({})
    property real reveal: 1
    property bool cancellable: false
    property string emptyText: ""
    property bool autoLoad: true

    property bool pendingApply: false
    property var applyItem: null
    property var applyingId: undefined
    property real appendScrollSnapshot: -1

    signal previewRequested(int row)
    signal downloadRequested(int row)
    signal applyRequested(int row)
    signal cancelRequested(int row)
    signal queryEdited(string text)
    signal searchSubmitted(string text)

    readonly property alias field: cardField
    readonly property alias searchEditing: searchInput.editing

    property int active: 0
    property int queued: 0
    property var activeItem: null
    property var queuedItem: null

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
        results.appendScrollSnapshot = cardField.scrollPosition
        console.info("RYOGAMI_BROWSER_SCROLL before_append reason=" + reason
                     + " page=" + s.page + " position=" + cardField.scrollPosition
                     + " extent=" + cardField.scrollExtent)
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
            if (results.appendScrollSnapshot >= 0) {
                Qt.callLater(function() {
                    console.info("RYOGAMI_BROWSER_SCROLL after_append page=" + results.source.page
                                 + " position=" + cardField.scrollPosition
                                 + " extent=" + cardField.scrollExtent
                                 + " preserved=" + (Math.abs(cardField.scrollPosition
                                                            - results.appendScrollSnapshot) < 1))
                    results.appendScrollSnapshot = -1
                })
            }
            results._maybePage()
        }
        function onLoadingChanged() { results.recount() }
        function onDataChanged(topLeft, bottomRight, roles) {
            results.recount()
            if (actions.row >= topLeft.row && actions.row <= bottomRight.row)
                actions.refreshItem()
        }
    }

    Rectangle {
        anchors.fill: parent
        color: Theme.withAlpha(Theme.background, 0.3)
        border.width: 1
        border.color: Theme.withAlpha(Theme.outline, 0.3)
        clip: true

        Column {
            anchors.fill: parent
            spacing: 0

            Rectangle {
                id: header
                width: parent.width
                height: 68 * Theme.scale
                color: Theme.withAlpha(Theme.surface, 0.92)

                Row {
                    anchors.fill: parent
                    anchors.leftMargin: 18 * Theme.scale
                    anchors.rightMargin: 18 * Theme.scale
                    spacing: 14 * Theme.scale

                    Column {
                        width: Math.min(190 * Theme.scale, parent.width * 0.24)
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 2 * Theme.scale
                        Text {
                            width: parent.width
                            text: I18n.tr(results.sourceLabel)
                            elide: Text.ElideRight
                            color: Theme.surfaceText
                            font.family: Theme.display
                            font.pixelSize: Theme.fontHead
                            renderType: Text.NativeRendering
                        }
                        Text {
                            text: results.source && results.source.count === 1
                                ? I18n.tr("1 result")
                                : I18n.tr("%1 results").arg(results.source ? results.source.count : 0)
                            color: Theme.withAlpha(Theme.surfaceText, 0.48)
                            font.family: Theme.sans
                            font.pixelSize: Theme.fontTiny
                            font.letterSpacing: 0.8
                            renderType: Text.NativeRendering
                        }
                    }

                    Rectangle {
                        anchors.verticalCenter: parent.verticalCenter
                        width: parent.width - x - sortPlate.width
                               - (sortPlate.visible ? 12 * Theme.scale : 0)
                        height: 36 * Theme.scale
                        radius: Theme.radius
                        color: searchInput.activeFocus
                            ? Theme.withAlpha(Theme.surfaceText, 0.08)
                            : Theme.withAlpha(Theme.surfaceText, 0.04)
                        border.width: 1
                        border.color: Theme.withAlpha(Theme.outline, searchInput.activeFocus ? 0.65 : 0.32)

                        TextField {
                            id: searchInput
                            visible: results.searchable
                            anchors.fill: parent
                            anchors.leftMargin: 2 * Theme.scale
                            anchors.rightMargin: 2 * Theme.scale
                            variant: "ghost"
                            glyph: "\uf002"
                            placeholder: results.searchPlaceholder
                            text: results.query
                            onEdited: function(t) { results.queryEdited(t) }
                            onCommitted: function(t) { results.searchSubmitted(t) }
                        }
                        Text {
                            visible: !results.searchable
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            anchors.leftMargin: 12 * Theme.scale
                            text: I18n.tr("Curated daily")
                            color: Theme.withAlpha(Theme.surfaceText, 0.56)
                            font.family: Theme.sans
                            font.pixelSize: Theme.fontBody
                            renderType: Text.NativeRendering
                        }
                    }

                    Rectangle {
                        id: sortPlate
                        visible: results.sortText.length > 0
                        anchors.verticalCenter: parent.verticalCenter
                        width: visible ? Math.min(138 * Theme.scale, sortLabel.implicitWidth + 24 * Theme.scale) : 0
                        height: 30 * Theme.scale
                        radius: Theme.radius
                        color: Theme.withAlpha(Theme.surfaceText, 0.05)
                        border.width: 1
                        border.color: Theme.withAlpha(Theme.outline, 0.32)
                        Text {
                            id: sortLabel
                            anchors.centerIn: parent
                            text: I18n.tr("Sort: %1").arg(results.sortText)
                            color: Theme.withAlpha(Theme.surfaceText, 0.68)
                            font.family: Theme.sans
                            font.pixelSize: Theme.fontSmall
                            font.weight: Font.Medium
                            renderType: Text.NativeRendering
                        }
                    }
                }

                Rectangle {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    height: 1
                    color: Theme.withAlpha(Theme.outline, 0.28)
                }
            }

            BrowserStatus {
                id: strip
                width: parent.width
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

            Item {
                id: gridArea
                width: parent.width
                height: parent.height - y - footer.height

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
                    activateOnClick: true
                    visible: results.source ? results.source.count > 0 : false
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
                            showApply: results.showApply
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

            Rectangle {
                id: footer
                width: parent.width
                height: 50 * Theme.scale
                color: Theme.withAlpha(Theme.surface, 0.94)

                Rectangle {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    height: 1
                    color: Theme.withAlpha(Theme.outline, 0.28)
                }

                Row {
                    anchors.centerIn: parent
                    spacing: 8 * Theme.scale

                    FolioAction {
                        label: I18n.tr("Previous")
                        enabled: cardField.scrollPosition > 1
                        onTriggered: cardField.scrollBy(-Math.max(120, gridArea.height * 0.8))
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Page %1 of %2").arg(results.source ? results.source.page : 1)
                            .arg(results.source ? Math.max(results.source.lastPage, 1) : 1)
                        color: Theme.withAlpha(Theme.surfaceText, 0.64)
                        font.family: Theme.sans
                        font.pixelSize: Theme.fontBody
                        font.weight: Font.Medium
                        renderType: Text.NativeRendering
                    }
                    FolioAction {
                        label: I18n.tr("Next")
                        enabled: !!results.source && !results.source.loading
                                 && results.source.page < results.source.lastPage
                        onTriggered: results.loadNext("next")
                    }
                    FolioAction {
                        label: results.source && results.source.loading
                            ? I18n.tr("Loading") : I18n.tr("Load more")
                        active: true
                        enabled: !!results.source && !results.source.loading
                                 && results.source.page < results.source.lastPage
                        onTriggered: results.loadNext("button")
                    }
                }
            }
        }
    }
}

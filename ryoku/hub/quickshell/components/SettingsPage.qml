import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../registry/registry.js" as Registry

// one settings page drawn from its schema plus the provider rows mapped to it
Item {
    id: page

    required property var hub
    property string pageId: ""
    property var schema: null
    property string stateLine: ""
    // any row of this page hidden because the window manager lacks it
    property bool someHidden: false
    // section id -> Component drawn above (body) or below (footer) its slab
    property var bodies: ({})
    property var footers: ({})

    // [{ id, title, missing, rows, more, hidden }] for the sections on screen
    property var layout: []
    property string layoutSignature: ""
    property string highlightId: ""
    property int currentSection: 0
    property bool indexHeld: false
    // after a landing the resting pointer shows no hover until it moves, so
    // the row it happens to rest on never reads as the found one
    property bool pointerQuiet: false
    property point quietAt: Qt.point(-1, -1)
    // row id -> delegate, kept in place so lookups never rebind
    property var rowItems: ({})
    property var sectionItems: ({})

    readonly property bool ready: Changes.desktopReady && Settings.loaded && ProviderSchema.ready
    readonly property bool sideIndex: page.layout.length >= 4
        && page.width >= LithicTokens.size.settingsExpandedMax + LithicTokens.size.sectionIndexWidth + LithicTokens.size.sectionIndexGap
    readonly property real columnWidth: page.sideIndex
        ? Math.min(page.width - LithicTokens.size.sectionIndexWidth - LithicTokens.size.sectionIndexGap, LithicTokens.size.settingsExpandedMax)
        : Math.min(page.width, page.hub.sizeClass === "expanded" ? LithicTokens.size.settingsExpandedMax : LithicTokens.size.settingsMediumMax)
    readonly property real groupX: page.sideIndex
        ? Math.max(0, (page.width - page.columnWidth - LithicTokens.size.sectionIndexWidth - LithicTokens.size.sectionIndexGap) / 2)
        : Math.max(0, (page.width - page.columnWidth) / 2)
    readonly property real columnX: page.sideIndex
        ? page.groupX + LithicTokens.size.sectionIndexWidth + LithicTokens.size.sectionIndexGap
        : page.groupX

    function whenMet(row) {
        if (!row.when)
            return true;
        var v = Changes.value(row.when.bind);
        for (var i = 0; i < row.when.is.length; i++)
            if (JSON.stringify(row.when.is[i]) === JSON.stringify(v))
                return true;
        return false;
    }

    function allRows() {
        if (!page.schema)
            return [];
        return page.schema.rows.concat(page.hub.providerRowsFor(page.pageId));
    }

    function rebuild() {
        if (!page.schema || !page.ready)
            return;
        var rows = page.allRows();
        var out = [], sig = [], anyHidden = false;
        for (var s = 0; s < page.schema.sections.length; s++) {
            var sec = page.schema.sections[s];
            var mine = rows.filter(r => r.section === sec.id);
            var avail = mine.filter(r => page.hub.rowAvailable(r));
            var shown = avail.filter(r => page.whenMet(r));
            var base = shown.filter(r => r.more !== true);
            var more = shown.filter(r => r.more === true);
            var gone = avail.length === 0 && mine.length > 0;
            if (avail.length < mine.length)
                anyHidden = true;
            // a section that lost every row says why, when it knows
            var bespoke = !!(page.bodies[sec.id] || page.footers[sec.id]);
            if (shown.length === 0 && !bespoke && !(gone && sec.missing))
                continue;
            out.push({ id: sec.id, title: sec.title, missing: gone ? (sec.missing || "") : "", rows: base, more: more });
            // a row whose words or options change under the same id is redrawn too
            var rowSig = r => r.id + "=" + r.label + "=" + (r.detail || "") + "=" + JSON.stringify(r.options || []);
            sig.push(sec.id + ":" + base.map(rowSig).join(",") + "|" + more.map(rowSig).join(",") + "|" + gone);
        }
        page.someHidden = anyHidden;
        var signature = sig.join(";");
        if (signature !== page.layoutSignature) {
            page.layoutSignature = signature;
            page.layout = out;
        }
    }

    onReadyChanged: page.rebuild()
    onSchemaChanged: page.rebuild()
    onBodiesChanged: page.rebuild()
    onFootersChanged: page.rebuild()
    Component.onCompleted: page.rebuild()
    Connections {
        target: Changes
        function onRevisionChanged() { page.rebuild(); }
    }
    Connections {
        target: Settings
        function onRevisionChanged() { page.rebuild(); }
        function onOverlayRevisionChanged() { page.rebuild(); }
    }
    Connections {
        target: ProviderSchema
        function onRevisionChanged() { page.rebuild(); }
    }

    function sectionOfRow(rowId) {
        for (var s = 0; s < page.layout.length; s++) {
            var sec = page.layout[s];
            if (sec.rows.some(r => r.id === rowId))
                return { section: sec, more: false };
            if (sec.more.some(r => r.id === rowId))
                return { section: sec, more: true };
        }
        return null;
    }

    // a row hidden by its condition leads to the row it depends on
    function landingRow(rowId) {
        if (page.sectionOfRow(rowId))
            return rowId;
        var rows = page.allRows();
        var target = rows.find(r => r.id === rowId);
        if (target && target.when) {
            var key = Changes.keyOf(target.when.bind);
            var parentRow = rows.find(r => r.bind && Changes.keyOf(r.bind) === key);
            if (parentRow && page.sectionOfRow(parentRow.id))
                return parentRow.id;
        }
        return "";
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (!page.ready || page.layout.length === 0) {
            pendingFocus.rowId = rowId;
            return;
        }
        var id = page.landingRow(rowId);
        if (id === "")
            return;
        var where = page.sectionOfRow(id);
        var secItem = page.sectionItems[where.section.id];
        if (where.more && secItem && !secItem.moreOpen) {
            secItem.openMore();
            landTimer.rowId = id;
            landTimer.restart();
            return;
        }
        page.land(id);
    }

    function land(id) {
        var item = page.rowItems[id];
        if (!item)
            return;
        var y = item.mapToItem(flick.contentItem, 0, 0).y;
        page.indexHeld = false;
        page.highlightId = "";
        scrollAnim.stop();
        scrollAnim.landing = id;
        page.pointerQuiet = true;
        page.quietAt = pointerWatch.point.scenePosition;
        page.scrollTo(y - flick.height / 3);
    }

    function scrollTo(y) {
        var target = Math.max(0, Math.min(y, flick.contentHeight - flick.height));
        scrollAnim.stop();
        scrollAnim.from = flick.contentY;
        scrollAnim.to = target;
        scrollAnim.start();
    }

    function chooseSection(index) {
        var sec = page.layout[index];
        var item = sec ? page.sectionItems[sec.id] : null;
        if (!item)
            return;
        page.currentSection = index;
        page.indexHeld = true;
        page.scrollTo(item.mapToItem(flick.contentItem, 0, 0).y);
    }

    // the index follows the reading position
    function followReading() {
        if (page.indexHeld || page.layout.length === 0)
            return;
        if (flick.contentY >= flick.contentHeight - flick.height - 1) {
            page.currentSection = page.layout.length - 1;
            return;
        }
        var current = 0;
        for (var i = 0; i < page.layout.length; i++) {
            var item = page.sectionItems[page.layout[i].id];
            if (!item)
                continue;
            var top = item.mapToItem(flick.contentItem, 0, 0).y;
            var visibleAbove = top + item.height - flick.contentY;
            if (top <= flick.contentY + flick.height / 3 && !(visibleAbove < item.height / 3 && i + 1 < page.layout.length))
                current = i;
        }
        page.currentSection = current;
    }

    QtObject {
        id: pendingFocus
        property string rowId: ""
    }
    onLayoutChanged: {
        if (pendingFocus.rowId !== "") {
            var id = pendingFocus.rowId;
            pendingFocus.rowId = "";
            Qt.callLater(page.focusRow, id);
        }
    }

    Timer {
        id: landTimer
        property string rowId: ""
        interval: LithicTokens.motion.unfold + 20
        onTriggered: page.land(landTimer.rowId)
    }

    Timer {
        id: highlightClear
        interval: Lithic.reduceMotion ? LithicTokens.motion.placardLife : LithicTokens.motion.highlight + 100
        onTriggered: page.highlightId = ""
    }

    NumberAnimation {
        id: scrollAnim
        // the row to spotlight once the scroll stops
        property string landing: ""
        target: flick
        property: "contentY"
        duration: Lithic.duration(LithicTokens.motion.navigate)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Lithic.curve
        onStopped: {
            if (scrollAnim.landing === "")
                return;
            page.highlightId = scrollAnim.landing;
            var landed = page.rowItems[scrollAnim.landing];
            if (landed && landed.settingRow.focusPolicy !== Qt.NoFocus)
                landed.settingRow.forceActiveFocus(Qt.OtherFocusReason);
            scrollAnim.landing = "";
            highlightClear.restart();
        }
    }

    Flickable {
        id: flick
        anchors.fill: parent
        contentWidth: width
        contentHeight: body.y + body.height + LithicTokens.size.settingsInsetVertical
        boundsBehavior: Flickable.StopAtBounds
        clip: true
        onContentYChanged: page.followReading()
        onMovementStarted: page.indexHeld = false

        Column {
            id: body
            x: page.columnX
            width: page.columnWidth
            spacing: 0

            Label {
                width: parent.width
                text: page.schema ? I18n.tr(page.schema.title) : ""
                typeRole: "name"
            }
            Item { width: 1; height: LithicTokens.space.s2 }
            Label {
                width: parent.width
                text: page.schema && page.schema.lead ? I18n.tr(page.schema.lead) : ""
                typeRole: "body"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Label {
                width: parent.width
                visible: page.stateLine.length > 0
                topPadding: LithicTokens.space.s1
                text: page.stateLine
                typeRole: "detail"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Label {
                width: parent.width
                visible: page.someHidden
                topPadding: LithicTokens.space.s1
                text: I18n.tr("Some options are hidden because this window manager does not support them.")
                typeRole: "detail"
                inkRole: "fg3"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Item { width: 1; height: LithicTokens.space.s5 }

            SectionIndex {
                visible: !page.sideIndex && page.layout.length >= 4
                width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
                sizeClass: "medium"
                sections: page.layout.map(s => I18n.tr(s.title))
                currentIndex: page.currentSection
                overlayParent: page.hub.overlayItem
                onActivated: index => page.chooseSection(index)
            }
            Item {
                visible: !page.sideIndex && page.layout.length >= 4
                width: 1
                height: LithicTokens.space.s5
            }

            Repeater {
                model: page.layout
                delegate: Section {
                    id: sec
                    required property var modelData
                    readonly property bool moreOpen: moreFold.open
                    function openMore() {
                        moreFold.open = true;
                    }
                    width: body.width
                    title: I18n.tr(modelData.title)
                    Component.onCompleted: page.sectionItems[modelData.id] = sec
                    Component.onDestruction: {
                        if (page.sectionItems[modelData.id] === sec)
                            delete page.sectionItems[modelData.id];
                    }

                    Column {
                        width: parent.width
                        spacing: LithicTokens.space.s2

                        Loader {
                            width: parent.width
                            active: !!page.bodies[sec.modelData.id]
                            visible: active
                            sourceComponent: page.bodies[sec.modelData.id] || null
                        }

                        Slab {
                            width: parent.width
                            visible: sec.modelData.rows.length + sec.modelData.more.length > 0

                            Column {
                                width: parent.width

                                Repeater {
                                    model: sec.modelData.rows
                                    delegate: RowItem { host: page }
                                }

                                Item {
                                    visible: sec.modelData.more.length > 0
                                    width: 1
                                    height: sec.modelData.rows.length > 0 ? LithicTokens.size.settingsRowGap : 0
                                }

                                Disclosure {
                                    id: moreFold
                                    visible: sec.modelData.more.length > 0
                                    width: parent.width
                                    text: I18n.tr("More options")

                                    Repeater {
                                        model: sec.modelData.more
                                        delegate: RowItem { host: page }
                                    }
                                }
                            }
                        }

                        Loader {
                            width: parent.width
                            active: !!page.footers[sec.modelData.id]
                            visible: active
                            sourceComponent: page.footers[sec.modelData.id] || null
                        }

                        Label {
                            width: parent.width
                            visible: sec.modelData.missing.length > 0
                            text: I18n.tr(sec.modelData.missing)
                            typeRole: "detail"
                            inkRole: "fg3"
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }
                    }
                }
            }

            Section {
                visible: related.count > 0
                width: body.width
                title: I18n.tr("Related")

                Flow {
                    width: parent.width
                    spacing: LithicTokens.space.s2

                    Repeater {
                        id: related
                        model: page.schema && page.schema.related ? page.schema.related.filter(id => page.hub.pageAvailable(id)) : []
                        delegate: Plaque {
                            required property var modelData
                            kind: "tonal"
                            small: true
                            glyph: "arrow_forward"
                            text: I18n.tr(Registry.page(modelData).title)
                            onClicked: page.hub.navigate(modelData, "")
                        }
                    }
                }
            }
        }
    }

    SectionIndex {
        visible: page.sideIndex
        x: page.groupX
        y: 0
        width: LithicTokens.size.sectionIndexWidth
        sizeClass: "expanded"
        sections: page.layout.map(s => I18n.tr(s.title))
        currentIndex: page.currentSection
        onActivated: index => page.chooseSection(index)
    }

    // scrolling under a resting pointer keeps its position; only a real move counts
    HoverHandler {
        id: pointerWatch
        onPointChanged: {
            if (!page.pointerQuiet)
                return;
            var p = pointerWatch.point.scenePosition;
            if (Math.abs(p.x - page.quietAt.x) + Math.abs(p.y - page.quietAt.y) > 2)
                page.pointerQuiet = false;
        }
    }

    ScrollRail {
        flickable: flick
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.right: parent.right
    }

    component RowItem: Column {
        id: item
        required property var modelData
        required property int index
        required property Item host
        readonly property alias settingRow: settingRow
        readonly property string error: Changes.error(item.modelData.bind)
        width: parent ? parent.width : 0
        spacing: LithicTokens.space.s1

        Component.onCompleted: item.host.rowItems[item.modelData.id] = item
        Component.onDestruction: {
            if (item.host.rowItems[item.modelData.id] === item)
                delete item.host.rowItems[item.modelData.id];
        }

        SettingRow {
            id: settingRow
            hoverEnabled: !item.host.pointerQuiet
            width: parent.width
            title: I18n.tr(item.modelData.label)
            detail: item.modelData.detail ? I18n.tr(item.modelData.detail) : ""
            changed: Changes.isChanged(item.modelData)
            first: item.index === 0
            highlighted: item.host.highlightId === item.modelData.id
            activatable: ["toggle", "handoff", "action"].indexOf(item.modelData.control) >= 0
            onActivated: rowControl.activate()
            onResetRequested: Changes.set(item.modelData.bind, Changes.defaultOf(item.modelData), item.modelData.changeLabel || item.modelData.label, item.host.pageId)

            RowControl {
                id: rowControl
                row: item.modelData
                pageId: item.host.pageId
                overlayParent: item.host.hub.overlayItem
            }
        }

        Row {
            visible: item.error.length > 0
            width: parent.width
            spacing: LithicTokens.space.s3

            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, parent.width - retry.width - LithicTokens.space.s3)
                text: item.error
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: retry
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: Changes.retry(item.modelData.bind)
            }
        }
    }
}

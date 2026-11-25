import QtQuick
import Quickshell
import Quickshell.Services.Mpris
import "../../shared/Singletons"
import "metrics.js" as MainMetrics
import "../../shared" as Shared
import "." as MainVariant
import Ryoku.Ui.Singletons

// The command palette body: a search row over the rest dashboard (empty query),
// the all-apps grid (Ctrl+A from rest), the action-mode tabs + list ("/" prefix),
// or the ranked result list. Ctrl+K opens the selected row's action panel. Grows
// and shrinks with its content on the Ryoku morph curve. Providers register with
// the dispatcher; this view only renders what the dispatcher returns.
Item {
    id: root

    property real s: 1
    property bool shown: false
    property string query: ""
    property bool allApps: false
    property bool help: false

    required property Item providerSet
    signal requestClose()

    property var results: []
    property var appEntries: []
    property int totalCount: 0
    property bool evaluationQueued: false
    readonly property bool resting: query.length === 0
    // grid and help are mutually exclusive body modes; exclude help here too so
    // no toggle path can leave both drawing over each other.
    readonly property bool gridMode: resting && allApps && !help

    // Sticky active player for the card. Proxy-free (Players.realPlayers drops
    // playerctld and dedupes). A currently-playing source always wins; otherwise
    // the last-shown player is held, so PAUSING the card never makes it jump to a
    // different source (the old "pause and YT takes over" bug). onActivePlayerChanged
    // records the shown player so a later pause has something to hold.
    property string primaryDbus: ""
    readonly property var activePlayer: {
        var list = Players.realPlayers();
        void Mpris.players.values;
        if (!list || list.length === 0)
            return null;
        for (var i = 0; i < list.length; i++)
            if (list[i].isPlaying)
                return list[i];
        if (root.primaryDbus.length > 0)
            for (var j = 0; j < list.length; j++)
                if (String(list[j].dbusName || "") === root.primaryDbus)
                    return list[j];
        for (var k = 0; k < list.length; k++)
            if (list[k].canControl && list[k].trackTitle)
                return list[k];
        return list[0];
    }
    onActivePlayerChanged: if (activePlayer) root.primaryDbus = String(activePlayer.dbusName || "");
    readonly property bool hasMedia: activePlayer !== null
    // route the current query once; everything keys off the matched prefix so the
    // mode chip, the tabs, and the find delegation stay consistent.
    readonly property var routed: Dispatcher.route(query)
    readonly property string modeLabel: {
        var p = routed.prefix;
        if (p === "/file") return I18n.tr("FILE");
        if (p === "/folder") return I18n.tr("FOLDER");
        if (p === "/image") return I18n.tr("IMAGE");
        if (p === "/video") return I18n.tr("VIDEO");
        if (p === "/") return I18n.tr("ACTIONS");
        if (p === ">") return I18n.tr("PACKAGE");
        if (p === "=") return I18n.tr("CALC");
        if (p === "?") return I18n.tr("WEB");
        if (p === "@") return I18n.tr("RADIO");
        if (askMode) return "HYAKKI";
        return "";
    }
    readonly property bool actionMode: routed.provider === "actions"
    // "?" prefix with a non-empty query and an available DDG answer: the
    // AnswerPanel takes the body above the Search fallback row.
    readonly property bool answerMode: routed.prefix === "?"
        && routed.query.length > 0
        && providerSet.web && providerSet.web.answer && providerSet.web.answer.available
    // tabs + hint row are browsing aids: show them only for a bare "/" (the
    // action catalog), not once the user types "/play"; then the results take
    // the space and nothing clips.
    readonly property bool actionBrowse: actionMode && routed.query.length === 0
    // "\" prefix: a one-shot quick ask to the Hyakki agent. No provider claims
    // it; the panel owns the whole body and the Enter key while active.
    readonly property bool askMode: query.length > 0 && query[0] === "\\"
    readonly property string askQuestion: askMode ? query.slice(1).replace(/^\s+/, "") : ""
    // an async provider is resolving the current query and nothing has come back
    // yet: show a spinner, not a premature "No matches".
    readonly property bool searching: shown && !resting && Dispatcher.busy && results.length === 0
    readonly property var selectedResult: root.gridMode
        ? root.appEntries[appGrid.selectedIndex]
        : root.results[list.selectedIndex]
    readonly property var selectedActions: {
        var actions = root.selectedResult && root.selectedResult.actions;
        return actions ? actions.slice(1) : [];
    }

    // Provider queries may start asynchronous work, so evaluate them outside
    // bindings and repaint from Dispatcher revisions instead of forming loops.
    function scheduleEvaluation() {
        if (root.evaluationQueued)
            return;
        root.evaluationQueued = true;
        Qt.callLater(root.evaluateResults);
    }

    function evaluateResults() {
        root.evaluationQueued = false;
        if (!root.shown) {
            root.results = [];
            root.appEntries = [];
            root.totalCount = 0;
            return;
        }
        root.results = Dispatcher.results(root.query, Metrics.maxResults);
        root.appEntries = Dispatcher.resultsFor("apps", "", "", 0);
        root.totalCount = root.appEntries.length;
    }

    // Machine-readable snapshot consumed by the stable selector.
    function stateDump() {
        var rs = [];
        var n = Math.min(results.length, 12);
        for (var i = 0; i < n; i++) {
            var r = results[i] || {};
            rs.push({
                title: String(r.title || ""),
                subtitle: String(r.subtitle || ""),
                type: String(r.type || ""),
                verb: r.actions && r.actions.length > 0
                    ? String(r.actions[0].name || "") : ""
            });
        }
        var acts = [];
        for (var j = 0; j < selectedActions.length; j++)
            acts.push(String(selectedActions[j].name || ""));
        return {
            query: query,
            resting: resting,
            gridMode: gridMode,
            help: help,
            askMode: askMode,
            answerMode: answerMode,
            actionMode: actionMode,
            modeLabel: modeLabel,
            searching: searching,
            busy: Dispatcher.busy,
            resultCount: results.length,
            totalCount: totalCount,
            busyIds: Object.keys(Dispatcher.busyProviders),
            selectedIndex: gridMode ? appGrid.selectedIndex
                : (!resting && !help && !askMode ? list.selectedIndex : -1),
            panelOpen: panel.open,
            selectedActions: acts,
            hasMedia: hasMedia,
            results: rs
        };
    }

    readonly property real cardW: MainMetrics.windowW * s
    readonly property int visibleRows: 8
    readonly property int visibleCount: Math.min(results.length, visibleRows)
    // each distinct type group draws an 18px section header, so the list must be
    // tall enough for the rows AND their headers, otherwise the last row clips
    // (the subtitle vanishes and the action verb sits wrong).
    readonly property int sectionCount: {
        var n = 0, prev = null;
        for (var i = 0; i < visibleCount; i++) {
            var t = results[i] ? (results[i].type || "") : "";
            if (i === 0 || t !== prev) n++;
            prev = t;
        }
        return n;
    }
    readonly property real listH: visibleCount * (MainMetrics.rowHeight + MainMetrics.gapRow) * s + sectionCount * MainMetrics.sectionH * s
    readonly property real gridH: 380 * s
    readonly property real tabsH: actionBrowse ? tabs.implicitHeight + 6 * s : 0
    readonly property real restH: rest.implicitHeight
        + (hasMedia ? nowPlaying.implicitHeight + MainMetrics.padRow * s : 0)
        + (hasMedia && mediaSources.sources.length > 0 ? mediaSources.implicitHeight + 6 * s : 0)
        + (radioParked ? radioAside.implicitHeight + 6 * s : 0)
    // the radio's between-states chip: a station the watcher parked, or a
    // tune-in still resolving (state on air, no player yet) - both need a
    // visible line or the radio reads as vanished/failed.
    readonly property bool radioParked: (Radio.aside !== null && !Radio.on) || Radio.tuning
    // Extra body slice for the instant-answer panel; padRow separates it from
    // the Search fallback row that stays underneath so Enter still targets it.
    readonly property real answerH: answerMode ? answerPanel.implicitHeight + MainMetrics.padRow * s : 0
    readonly property real bodyH: help ? helpPanel.implicitHeight
        : gridMode ? gridH
        : askMode ? askPanel.implicitHeight + MainMetrics.padRow * s
        : (resting ? restH
        : (answerH + (results.length > 0 ? listH
        : (searching ? loading.height : empty.height))))
    readonly property real contentH: tabsH + bodyH

    implicitWidth: cardW
    implicitHeight: search.height + divider.height + contentH + MainMetrics.padOuter * 2 * s
    // height as if at rest (no results/tabs). The window anchors the card top off
    // this so typing grows the body downward while the search row stays put.
    readonly property real restingHeight: search.height + divider.height + restH + MainMetrics.padOuter * 2 * s

    Behavior on implicitHeight {
        NumberAnimation { duration: Motion.standard; easing.type: Motion.easeMorph; easing.bezierCurve: Motion.morphCurve }
    }

    // open/close morph: the card inflates from its top edge (where the search row
    // sits) and fades, rather than popping in at full size. Synced to the window
    // timer in shell.qml so the close plays fully before the window drops.
    transformOrigin: Item.Top
    opacity: shown ? 1 : 0
    scale: shown ? 1 : 0.92
    Behavior on opacity { NumberAnimation { duration: Motion.open; easing.type: Easing.OutCubic } }
    Behavior on scale {
        NumberAnimation { duration: Motion.open; easing.type: Motion.easeMorph; easing.bezierCurve: Motion.morphCurve }
    }


    Connections {
        target: Dispatcher
        function onRevisionChanged() { root.scheduleEvaluation(); }
    }

    Component.onCompleted: root.scheduleEvaluation()


    Binding {
        target: providerSet.actions
        property: "activeCategory"
        value: tabs.activeCategory
        when: root.actionMode
    }

    onShownChanged: {
        if (shown) {
            root.query = "";
            root.allApps = false;
            root.help = false;
            search.clear();
            list.resetSelection();
            panel.open = false;
            // a category picked last session must not silently narrow "/"
            tabs.activeIndex = 0;
            Qt.callLater(search.focusField);
        }
        root.scheduleEvaluation();
    }
    onQueryChanged: {
        panel.open = false;
        if (query.length > 0) { root.allApps = false; root.help = false; }
        // leaving ask mode drops any in-flight or finished ask
        if (query.length === 0 || query[0] !== "\\") askPanel.reset();
        list.resetSelection();
        root.scheduleEvaluation();
    }

    MainVariant.Squircle {
        anchors.fill: parent
        radius: LauncherConfig.radius
        power: 4
        color: Theme.cardTop
        borderColor: Theme.border
        borderWidth: 1
    }

    // Absorb clicks on the card body so they never fall through to the shell's
    // click-out scrim (which hides the launcher). The interactive children
    // (search row, result rows, transport, source strips) are declared after
    // this and sit on top, so they still receive their own clicks first; this
    // only swallows clicks that land on empty card space.
    MouseArea {
        anchors.fill: parent
        onClicked: {}
    }

    MainVariant.SearchRow {
        id: search
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.topMargin: (MainMetrics.padOuter - 8) * root.s
        s: root.s
        resultCount: root.results.length
        totalCount: root.totalCount
        modeLabel: root.modeLabel
        gridActive: root.gridMode
        helpActive: root.help
        onTextChanged: root.query = text
        onMoved: (d) => { if (root.askMode && (askPanel.chips.length > 0 || askPanel.resumeMode)) askPanel.move(d); else if (panel.open) panel.move(d); else if (root.gridMode) appGrid.move(d * root.gridColumnsForMove); else list.move(d); }
        onAccepted: { if (root.askMode) { if (askPanel.resumeMode || askPanel.busy || askPanel.answerCurrent) askPanel.activate(); else askPanel.run(); } else if (panel.open) panel.run(); else if (root.gridMode) appGrid.activate(); else list.activate(); }
        onDismissed: { if (root.askMode && askPanel.busy) { askPanel.cancel(); root.requestClose(); } else if (root.askMode && askPanel.resumeMode) { askPanel.reset(); root.query = ""; search.clear(); } else if (panel.open) panel.open = false; else if (root.help) root.help = false; else if (root.allApps) root.allApps = false; else root.requestClose(); }
        onGridToggled: {
            if (root.gridMode) {
                root.allApps = false;
            } else {
                search.clear();
                root.allApps = true;
                root.help = false;
            }
        }
        onHelpToggled: {
            root.help = !root.help;
            if (root.help) { search.clear(); root.allApps = false; }
        }
        onKeyPressed: (e) => {
            if (e.key === Qt.Key_K && (e.modifiers & Qt.ControlModifier)) {
                if (root.selectedActions.length > 0)
                    panel.open = !panel.open;
                e.accepted = true;
            } else if (e.key === Qt.Key_A && (e.modifiers & Qt.ControlModifier) && root.resting) {
                root.allApps = !root.allApps;
                if (root.allApps) root.help = false;
                e.accepted = true;
            } else if (root.actionMode && e.key === Qt.Key_Tab) {
                // only cycle while the tab bar is visible; with a typed action
                // query it would invisibly narrow the results. Still accept so
                // Tab can't walk focus off the search field.
                if (root.actionBrowse) tabs.cycle(1);
                e.accepted = true;
            } else if (root.actionMode && e.key === Qt.Key_Backtab) {
                if (root.actionBrowse) tabs.cycle(-1);
                e.accepted = true;
            }
        }
    }

    readonly property int gridColumnsForMove: appGrid.columns

    Rectangle {
        id: divider
        anchors.top: search.bottom
        anchors.topMargin: 6 * root.s
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: MainMetrics.padOuter * root.s
        anchors.rightMargin: MainMetrics.padOuter * root.s
        height: 1
        color: Theme.hair
    }

    Shared.CategoryTabs {
        id: tabs
        visible: root.actionBrowse
        anchors.top: divider.bottom
        anchors.topMargin: 8 * root.s
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: MainMetrics.padOuter * root.s
        anchors.rightMargin: MainMetrics.padOuter * root.s
        s: root.s
    }

    MainVariant.RestDashboard {
        id: rest
        visible: root.resting && !root.allApps && !root.help
        anchors.top: divider.bottom
        anchors.topMargin: MainMetrics.padRow * root.s
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: MainMetrics.padOuter * root.s
        anchors.rightMargin: MainMetrics.padOuter * root.s
        s: root.s
    }

    Shared.HelpPanel {
        id: helpPanel
        visible: root.help
        anchors.top: divider.bottom
        anchors.topMargin: MainMetrics.padRow * root.s
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: MainMetrics.padOuter * root.s
        anchors.rightMargin: MainMetrics.padOuter * root.s
        s: root.s
    }

    Shared.AnswerPanel {
        id: answerPanel
        visible: root.answerMode
        anchors.top: divider.bottom
        anchors.topMargin: MainMetrics.padRow * root.s
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: MainMetrics.padOuter * root.s
        anchors.rightMargin: MainMetrics.padOuter * root.s
        s: root.s
        answer: providerSet.web ? providerSet.web.answer : ({ available: false })
    }

    Shared.AskPanel {
        id: askPanel
        visible: root.askMode
        anchors.top: divider.bottom
        anchors.topMargin: MainMetrics.padRow * root.s
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: MainMetrics.padOuter * root.s
        anchors.rightMargin: MainMetrics.padOuter * root.s
        s: root.s
        question: root.askQuestion
        onFinished: root.requestClose()
    }

    MainVariant.NowPlaying {
        id: nowPlaying
        visible: root.resting && !root.allApps && !root.help && root.hasMedia
        anchors.top: rest.bottom
        anchors.topMargin: visible ? MainMetrics.padRow * root.s : 0
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: MainMetrics.padOuter * root.s
        anchors.rightMargin: MainMetrics.padOuter * root.s
        // collapse when hidden so the strip/saved rows below chain flush against
        // the rest card instead of leaving the card's fixed height as a gap.
        height: visible ? implicitHeight : 0
        s: root.s
        player: root.activePlayer
    }

    // Slim strips for the other media players, extending under the card so both
    // sources read as one compact stack. Only when resting with media present.
    MainVariant.MediaSources {
        id: mediaSources
        visible: root.resting && !root.allApps && !root.help && root.hasMedia && sources.length > 0
        anchors.top: nowPlaying.bottom
        anchors.topMargin: visible ? 6 * root.s : 0
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: MainMetrics.padOuter * root.s
        anchors.rightMargin: MainMetrics.padOuter * root.s
        height: visible ? implicitHeight : 0
        s: root.s
        activePlayer: root.activePlayer
    }

    // The parked-radio chip, at the bottom of the media stack (or directly
    // under the rest card when nothing else plays): the radio the watcher set
    // aside stays one tap from returning instead of silently vanishing.
    MainVariant.RadioAside {
        id: radioAside
        visible: root.resting && !root.allApps && !root.help && root.radioParked
        anchors.top: mediaSources.bottom
        anchors.topMargin: visible ? 6 * root.s : 0
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: MainMetrics.padOuter * root.s
        anchors.rightMargin: MainMetrics.padOuter * root.s
        height: visible ? implicitHeight : 0
        s: root.s
    }

    MainVariant.ResultGrid {
        id: appGrid
        visible: root.gridMode
        anchors.top: divider.bottom
        anchors.topMargin: MainMetrics.padRow * root.s
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: MainMetrics.padOuter * root.s
        anchors.rightMargin: MainMetrics.padOuter * root.s
        height: root.gridH - MainMetrics.padRow * root.s
        s: root.s
        entries: root.appEntries
        onActivated: closeRequested => {
            if (closeRequested)
                root.requestClose();
        }
        onSelectionLost: panel.open = false
    }

    MainVariant.ResultList {
        id: list
        visible: !root.resting && !root.askMode && root.results.length > 0
        anchors.top: root.answerMode ? answerPanel.bottom : (root.actionBrowse ? tabs.bottom : divider.bottom)
        anchors.topMargin: root.answerMode ? MainMetrics.padRow * root.s : 6 * root.s
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: (MainMetrics.padOuter - MainMetrics.padRow) * root.s
        anchors.rightMargin: (MainMetrics.padOuter - MainMetrics.padRow) * root.s
        height: root.listH
        s: root.s
        results: root.results
        query: root.routed.query
        onActivated: closeRequested => {
            if (closeRequested)
                root.requestClose();
        }
        onSelectionLost: panel.open = false
    }

    Row {
        id: loading
        visible: root.searching && !root.askMode
        anchors.top: root.answerMode ? answerPanel.bottom : (root.actionBrowse ? tabs.bottom : divider.bottom)
        anchors.topMargin: MainMetrics.padRow * root.s
        anchors.horizontalCenter: parent.horizontalCenter
        height: 40 * root.s
        spacing: 8 * root.s
        Shared.Spinner {
            anchors.verticalCenter: parent.verticalCenter
            size: 15 * root.s
            color: Theme.verm
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("Searching\u2026")
            color: Theme.faint
            font.family: Theme.font
            font.pixelSize: 12 * root.s
        }
    }

    Text {
        id: empty
        visible: !root.resting && !root.searching && !root.askMode && root.results.length === 0
        anchors.top: root.answerMode ? answerPanel.bottom : (root.actionBrowse ? tabs.bottom : divider.bottom)
        anchors.topMargin: MainMetrics.padRow * root.s
        anchors.horizontalCenter: parent.horizontalCenter
        text: I18n.tr("No matches")
        color: Theme.faint
        font.family: Theme.font
        font.pixelSize: 12 * root.s
        height: 40 * root.s
    }

    MainVariant.ActionPanel {
        id: panel
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: MainMetrics.padOuter * root.s
        height: open ? Math.min(root.selectedActions.length, 5) * 31 * root.s + 28 * root.s : 0
        s: root.s
        actions: root.selectedActions
        onChosen: closeRequested => {
            panel.open = false;
            if (closeRequested)
                root.requestClose();
        }
    }
}

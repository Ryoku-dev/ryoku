pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Services.Mpris
import Ryoku.Ui
import Ryoku.Ui.Singletons as Ui
import "../../shared/Singletons"
import "../../shared/providers"
import "../../shared/lib/results.js" as Results
import "../../shared/lib/launcherstate.js" as LauncherState
import "." as HeroVariant

Item {
    id: root

    property real s: 1
    property bool shown: false
    property bool frostActive: false
    property alias query: hero.query
    required property Item providerSet

    property string lifecyclePhase: ""
    property int lifecycleGeneration: 0
    property real lifecycleOuterHeight: 0
    property real workAreaWidth: 1920
    property real workAreaHeight: 1080
    property real reportedSurfaceWidth: 0
    property real reportedSurfaceHeight: 0

    property string activeMode: "rest"
    property bool suppressQueryRouting: false
    property string selectedResultKey: ""
    property int selectedIndex: -1
    property int selectionFallbackIndex: 0
    property bool shelfOpen: false
    property string focusedActionId: ""
    property string shelfSignature: ""
    property string focusedWindowAddress: ""
    property string interactionRegion: "results"
    property var windowRail: null
    property var packedActionLayout: []
    property string actionError: ""
    property var freshResults: []
    property string freshResultsToken: ""
    property string initialSelectionToken: ""
    property var results: []
    property string displayRouteKey: ""
    property string displayQueryToken: ""
    property bool displayWasBusy: false
    property int evaluationGeneration: 0
    property bool evaluationQueued: false
    property int outerGeometryGeneration: 0
    property real lastOuterHeight: 0
    property real pendingOuterHeight: 0
    property bool lastBodyOpenForGeometry: false
    property bool lastShelfOpenForGeometry: false
    property int observedLifecycleGeneration: -1
    property bool snappingHiddenGeometry: false

    signal requestClose()
    signal outerHeightRequested(real height, bool growing, int duration)

    readonly property bool resting: activeMode === "rest" && query.length === 0
    readonly property bool gridMode: activeMode === "all"
    readonly property bool help: activeMode === "help"
    readonly property bool bodyOpen: activeMode !== "rest"
    readonly property var drawer: drawerLoader.item || emptyDrawer
    onBodyOpenChanged: {
        if (!bodyOpen && drawerLoader.active)
            drawerHold.restart();
        retainInputFocus();
    }

    QtObject {
        id: emptyDrawer
        readonly property real rawDesiredHeight: 0
        readonly property real height: 0
        readonly property real opacity: 0
        readonly property real actionShelfHeight: 0
        readonly property string activeCategory: ""
        readonly property string actionVisibleRange: ""
        readonly property real resultDeckOpacity: 0
        property bool suppressResultDeckAnimation: false
        function resetCategory() {}
        function snapAnimations() {}
        function hideResultDeckImmediately() {}
        function revealRank(rank) {}
        function cycleCategory(delta) {}
    }

    readonly property var activeRoute: {
        if (activeMode === "all")
            return LauncherState.routeForMode("all", query);
        if (activeMode === "image")
            return LauncherState.routeForMode("image", query);
        if (activeMode === "file")
            return LauncherState.routeForMode("file", query);
        if (activeMode === "recent")
            return LauncherState.routeForMode("recent", query);
        if (activeMode === "special")
            return Dispatcher.route(query);
        return { providerId: null, provider: null, prefix: "", query: query };
    }
    readonly property string routeProviderId: String(
        activeRoute.providerId || activeRoute.provider || "")
    readonly property string resultRouteKey: {
        if (!shown || !bodyOpen || help)
            return "";
        if (activeMode === "federated")
            return "federated";
        return activeMode + "|" + routeProviderId + "|"
            + String(activeRoute.prefix || "");
    }
    readonly property string resultQueryToken: resultRouteKey + "\u0000"
        + String(activeRoute.query || "")
    readonly property bool actionMode: activeMode === "special"
        && routeProviderId === "actions"
    readonly property bool actionBrowse: actionMode
        && String(activeRoute.query || "").length === 0
    readonly property bool answerMode: activeMode === "special"
        && activeRoute.prefix === "?"
        && String(activeRoute.query || "").length > 0
        && providerSet.web && providerSet.web.answer
        && providerSet.web.answer.available
    readonly property string modeLabel: {
        if (activeMode === "all") return "ALL";
        if (activeMode === "image") return "IMG";
        if (activeMode === "file") return "FILE";
        if (activeMode === "recent") return "REC";
        if (activeMode === "help") return "HELP";
        var prefix = String(activeRoute.prefix || "");
        if (prefix === "/file") return "FILE";
        if (prefix === "/folder") return "FOLDER";
        if (prefix === "/image") return "IMAGE";
        if (prefix === "/video") return "VIDEO";
        if (prefix === "/") return "ACTIONS";
        if (prefix === ">") return "PACKAGE";
        if (prefix === "=") return "CALC";
        if (prefix === "?") return "WEB";
        if (prefix === "@") return "RADIO";
        return "";
    }

    readonly property int totalCount: results.length
    readonly property var selectedResult: {
        for (var index = 0; index < results.length; index++) {
            if (results[index] && results[index].resultKey === selectedResultKey)
                return results[index];
        }
        return null;
    }
    readonly property var secondaryActions: selectedResult
        && selectedResult.secondaryActions ? selectedResult.secondaryActions : []
    readonly property string currentActionSignature: Results.actionSignature(selectedResult)
    readonly property var selectedActions: {
        var actions = [];
        if (!selectedResult)
            return actions;
        if (selectedResult.primaryAction)
            actions.push(selectedResult.primaryAction);
        for (var index = 0; index < secondaryActions.length; index++)
            actions.push(secondaryActions[index]);
        return actions;
    }

    readonly property bool busyForActiveProvider: {
        void Dispatcher.busyRevision;
        var busy = Dispatcher.busyProviders;
        if (!bodyOpen || help)
            return false;
        if (routeProviderId.length > 0)
            return Boolean(busy[routeProviderId]);
        if (activeMode !== "federated")
            return false;
        for (var providerId in busy) {
            var provider = Dispatcher.registry[providerId];
            if (provider && (provider.defaultProvider || provider.numericFallback))
                return true;
        }
        return false;
    }
    readonly property bool searching: shown && bodyOpen && busyForActiveProvider
    readonly property bool hasMedia: {
        void Mpris.players.values;
        return Players.realPlayers().length > 0;
    }

    readonly property string emptyText: {
        if (activeMode === "image")
            return query.length === 0 ? Ui.I18n.tr("TYPE TO SEARCH IMAGES") : Ui.I18n.tr("NO IMAGES");
        if (activeMode === "file")
            return query.length === 0 ? Ui.I18n.tr("TYPE TO SEARCH FILES") : Ui.I18n.tr("NO FILES");
        if (activeMode === "recent")
            return Ui.I18n.tr("NO RECENT FILES");
        if (activeMode === "all")
            return Ui.I18n.tr("NO APPLICATIONS");
        if (actionBrowse)
            return Ui.I18n.tr("NO ACTIONS");
        return Ui.I18n.tr("NO MATCHES");
    }

    readonly property var cardGeometry: Results.cardGeometry(
        workAreaWidth, workAreaHeight, s,
        bodyOpen ? drawer.rawDesiredHeight / Math.max(0.001, s) : 0)
    readonly property real desiredCardHeight: bodyOpen
        ? cardGeometry.height : 250 * s
    readonly property real currentSurfaceWidth: reportedSurfaceWidth > 0
        ? reportedSurfaceWidth : implicitWidth
    readonly property real currentSurfaceHeight: reportedSurfaceHeight > 0
        ? reportedSurfaceHeight : implicitHeight
    readonly property string currentPhase: lifecyclePhase.length > 0
        ? lifecyclePhase : (shown ? "open" : "closed")
    readonly property bool transitionAnimationsEnabled: shown
        && currentPhase !== "closed"
        && currentPhase !== "prelude"
    readonly property real heroPresentedHeight: hero.height
    readonly property real drawerPresentedHeight: drawer.height
    readonly property real drawerPresentedOpacity: drawer.opacity
    readonly property real shelfPresentedHeight: drawer.actionShelfHeight

    implicitWidth: cardGeometry.width
    implicitHeight: hero.height + drawer.height
    readonly property real restingHeight: 250 * s

    function focusInput() {
        hero.focusField();
    }

    function retainInputFocus() {
        if (shown)
            focusRepair.restart();
    }

    function syncInvocationGeometry() {
        var syncResult = LauncherState.synchronizeOuterGeometry({
            observedGeneration: observedLifecycleGeneration,
            lifecycleGeneration: lifecycleGeneration,
            lifecycleOuterHeight: lifecycleOuterHeight,
            outerGeometryGeneration: outerGeometryGeneration,
            bodyOpen: bodyOpen,
            shelfOpen: shelfOpen
        });
        if (!syncResult)
            return;
        shrinkOuter.stop();
        observedLifecycleGeneration = syncResult.observedGeneration;
        outerGeometryGeneration = syncResult.outerGeometryGeneration;
        lastOuterHeight = syncResult.lastOuterHeight;
        pendingOuterHeight = syncResult.pendingOuterHeight;
        lastBodyOpenForGeometry =
            syncResult.lastBodyOpenForGeometry;
        lastShelfOpenForGeometry =
            syncResult.lastShelfOpenForGeometry;
    }

    function prepareRest() {
        suppressQueryRouting = true;
        activeMode = "rest";
        query = "";
        suppressQueryRouting = false;
        closeShelf();
        drawer.resetCategory();
        actionError = "";
        resetSelection();
    }

    function snapHiddenGeometry() {
        snappingHiddenGeometry = true;
        hero.height = Qt.binding(function () {
            return (root.bodyOpen ? 126 : 250) * root.s;
        });
        drawer.snapAnimations();
        snappingHiddenGeometry = false;
    }

    function resetSelection() {
        interactionRegion = "results";
        selectedResultKey = "";
        selectedIndex = -1;
        selectionFallbackIndex = 0;
        Qt.callLater(reconcileResults);
    }

    function validateProviderSet() {
        var requiredProviders = ["actions", "apps", "recent", "web"];
        for (var index = 0; index < requiredProviders.length; index++) {
            var name = requiredProviders[index];
            if (!providerSet[name])
                throw new Error("Launcher providerSet is missing " + name);
        }
    }

    function evaluateCurrentResults() {
        if (!shown || resting || help)
            return [];
        if (activeMode === "federated")
            return Dispatcher.resultsFor(null, query, "", Metrics.maxResults);
        if (activeMode === "all")
            return Dispatcher.resultsFor("apps", query, "", Metrics.maxResults);
        if (activeMode === "image")
            return Dispatcher.resultsFor("find", query, "/image", Metrics.maxResults);
        if (activeMode === "file")
            return Dispatcher.resultsFor("find", query, "/file", Metrics.maxResults);
        if (activeMode === "recent")
            return Dispatcher.resultsFor("recent", query, "", Metrics.maxResults);
        if (activeMode === "special") {
            var routedQuery = Dispatcher.route(query);
            return Dispatcher.resultsFor(
                routedQuery.provider, routedQuery.query,
                routedQuery.prefix, Metrics.maxResults);
        }
        return [];
    }

    function scheduleEvaluation() {
        evaluationGeneration++;
        if (evaluationQueued)
            return;
        evaluationQueued = true;
        Qt.callLater(evaluateLatestResults);
    }

    function evaluateLatestResults() {
        evaluationQueued = false;
        var generation = evaluationGeneration;
        var token = resultQueryToken;
        var snapshot = Results.hideDuplicateWindowRows(
            evaluateCurrentResults());
        if (!LauncherState.acceptEvaluationSnapshot(
                generation, evaluationGeneration, token, resultQueryToken)) {
            if (!evaluationQueued) {
                evaluationQueued = true;
                Qt.callLater(evaluateLatestResults);
            }
            return;
        }
        freshResultsToken = token;
        freshResults = snapshot || [];
        scheduleResultPresentation();
    }

    function scheduleResultPresentation() {
        if (resultRouteKey.length === 0) {
            resultPresentation.stop();
            syncDisplayedResults();
            return;
        }
        resultPresentation.restart();
    }

    function clearDisplayedResultsForInput() {
        drawer.hideResultDeckImmediately();
        results = [];
        Qt.callLater(function () {
            drawer.suppressResultDeckAnimation = false;
        });
    }

    function syncDisplayedResults() {
        if (resultRouteKey.length === 0) {
            emptySettlement.stop();
            displayRouteKey = "";
            displayQueryToken = "";
            displayWasBusy = false;
            results = [];
            return;
        }
        if (displayRouteKey !== resultRouteKey) {
            emptySettlement.stop();
            displayRouteKey = resultRouteKey;
            displayQueryToken = resultQueryToken;
            displayWasBusy = false;
            results = [];
        }

        var tokenChanged = displayQueryToken !== resultQueryToken;
        displayQueryToken = resultQueryToken;
        var snapshotCurrent = LauncherState.snapshotMatchesToken(
            freshResultsToken, resultQueryToken);
        if (!snapshotCurrent) {
            if (busyForActiveProvider) {
                emptySettlement.stop();
                displayWasBusy = true;
            }
            return;
        }
        if (freshResults.length > 0) {
            emptySettlement.stop();
            if (initialSelectionToken === resultQueryToken) {
                selectedResultKey = "";
                selectedIndex = -1;
                selectionFallbackIndex = 0;
                initialSelectionToken = "";
            }
            results = freshResults;
            return;
        }
        if (busyForActiveProvider) {
            emptySettlement.stop();
            displayWasBusy = true;
            return;
        }
        if (results.length === 0) {
            results = [];
            return;
        }
        if (tokenChanged || !emptySettlement.running)
            emptySettlement.restart();
    }

    function reconcileResults() {
        var previousKey = selectedResultKey;
        var reconciled = Results.reconcileSelection(
            results, selectedResultKey, selectionFallbackIndex);
        selectedResultKey = reconciled.key;
        selectedIndex = reconciled.index;
        selectionFallbackIndex = Math.max(0, reconciled.index);
        if (shelfOpen && previousKey.length > 0 && previousKey !== reconciled.key)
            closeShelf();
        if (reconciled.index >= 0)
            Qt.callLater(function () { drawer.revealRank(reconciled.index); });
    }

    function selectResult(resultKey, rank) {
        if (resultKey === selectedResultKey)
            return;
        interactionRegion = "results";
        closeShelf();
        selectedResultKey = resultKey;
        selectedIndex = rank;
        selectionFallbackIndex = Math.max(0, rank);
        actionError = "";
        Qt.callLater(function () { drawer.revealRank(rank); });
    }

    function selectResultIndex(index) {
        if (results.length === 0)
            return;
        var bounded = Math.max(0, Math.min(results.length - 1, index));
        var row = results[bounded];
        if (!row)
            return;
        if (row.resultKey !== selectedResultKey)
            closeShelf();
        selectedIndex = bounded;
        selectionFallbackIndex = bounded;
        selectedResultKey = row.resultKey;
        actionError = "";
        drawer.revealRank(bounded);
    }

    function moveSelection(delta) {
        if (results.length === 0)
            return;
        interactionRegion = "results";
        var index = selectedIndex < 0 ? 0
            : Math.max(0, Math.min(results.length - 1, selectedIndex + delta));
        selectResultIndex(index);
    }

    function moveSelectionByKey(keyName) {
        if (results.length === 0)
            return;
        interactionRegion = "results";
        selectResultIndex(LauncherState.moveResultIndex(
            selectedIndex, results.length, keyName));
    }

    function selectMode(mode) {
        if (activeMode === mode) {
            hero.focusField();
            return;
        }
        var next = LauncherState.selectMode(mode, query, Dispatcher.prefixes);
        suppressQueryRouting = true;
        closeShelf();
        activeMode = next.mode;
        query = next.query;
        suppressQueryRouting = false;
        actionError = "";
        resetSelection();
        Qt.callLater(hero.focusField);
    }

    function returnToRest() {
        suppressQueryRouting = true;
        closeShelf();
        activeMode = "rest";
        query = "";
        suppressQueryRouting = false;
        actionError = "";
        resetSelection();
        Qt.callLater(hero.focusField);
    }

    function routeVisibleQuery() {
        if (suppressQueryRouting)
            return;
        if (shown)
            typedFocusSettler.restart();
        closeShelf();
        actionError = "";
        var next = LauncherState.resolveTypedPrefix(
            activeMode, query, Dispatcher.prefixes);
        activeMode = next.mode;
        resetSelection();
    }

    function closeShelf() {
        shelfOpen = false;
        focusedActionId = "";
    }

    function handleCompositionStarted() {
        closeShelf();
        hero.focusField();
    }

    function openShelf() {
        if (secondaryActions.length === 0)
            return;
        packedActionLayout = Results.packActions(secondaryActions);
        shelfSignature = currentActionSignature;
        focusedActionId = LauncherState.moveActionFocus(
            packedActionLayout, "", "Open");
        shelfOpen = focusedActionId.length > 0;
        hero.focusField();
    }

    function toggleShelf() {
        if (secondaryActions.length === 0)
            return;
        if (shelfOpen)
            closeShelf();
        else
            openShelf();
    }

    function moveActionFocus(keyName) {
        if (!shelfOpen)
            return;
        focusedActionId = LauncherState.moveActionFocus(
            packedActionLayout, focusedActionId, keyName);
    }

    function currentSecondaryAction(actionId) {
        for (var index = 0; index < secondaryActions.length; index++) {
            if (String(secondaryActions[index].id) === String(actionId))
                return secondaryActions[index];
        }
        return null;
    }

    function focusWindow(address) {
        var target = String(address || "");
        if (target.length === 0)
            return;
        focusedWindowAddress = target;
        if (providerSet.windows && providerSet.windows.focusWindow)
            providerSet.windows.focusWindow(target);
        requestClose();
    }

    function executeAction(action) {
        if (!action || action.enabled === false
                || typeof action.execute !== "function") {
            actionError = Ui.I18n.tr("ACTION UNAVAILABLE");
            return;
        }
        try {
            action.execute();
            actionError = "";
            if (action.closeOnExecute !== false)
                requestClose();
        } catch (error) {
            var message = String(error || Ui.I18n.tr("ACTION FAILED")).split("\n")[0];
            actionError = message.length > 72
                ? message.slice(0, 69) + "..." : message;
            hero.focusField();
        }
    }

    function activateCurrent() {
        if (LauncherState.activationTarget(interactionRegion,
                windowRail && windowRail.hasWindows) === "window"
                && windowRail && windowRail.hasWindows
                && windowRail.focusedAddress.length > 0) {
            focusWindow(windowRail.focusedAddress);
            return;
        }
        if (shelfOpen) {
            executeAction(currentSecondaryAction(focusedActionId));
            return;
        }
        if (selectedResult && selectedResult.primaryAction)
            executeAction(selectedResult.primaryAction);
    }

    function dismissStage() {
        var escaped = LauncherState.escape({
            mode: activeMode,
            query: query,
            shelfOpen: shelfOpen,
            actionFocusId: focusedActionId,
            preeditActive: hero.preeditText.length > 0,
            phase: currentPhase
        });
        if (escaped.cancelPreedit) {
            hero.cancelPreedit();
            return;
        }
        if (shelfOpen) {
            closeShelf();
            hero.focusField();
            return;
        }
        if (escaped.closeRequested) {
            requestClose();
            return;
        }
        if (bodyOpen) {
            returnToRest();
            return;
        }
    }

    function showHelp() {
        if (help) {
            returnToRest();
            return;
        }
        suppressQueryRouting = true;
        closeShelf();
        query = "";
        activeMode = "help";
        suppressQueryRouting = false;
        actionError = "";
        resetSelection();
        Qt.callLater(hero.focusField);
    }

    function handleInputKey(event) {
        var control = Boolean(event.modifiers & Qt.ControlModifier);
        if (control && event.key === Qt.Key_K) {
            toggleShelf();
            return;
        }
        if (control && event.key === Qt.Key_A && resting) {
            selectMode("all");
            return;
        }
        if (shelfOpen) {
            if (event.key === Qt.Key_Tab)
                moveActionFocus(event.modifiers & Qt.ShiftModifier ? "Shift+Tab" : "Tab");
            else if (event.key === Qt.Key_Backtab)
                moveActionFocus("Shift+Tab");
            else if (event.key === Qt.Key_Left)
                moveActionFocus("Left");
            else if (event.key === Qt.Key_Right)
                moveActionFocus("Right");
            else if (event.key === Qt.Key_Up)
                moveActionFocus("Up");
            else if (event.key === Qt.Key_Down)
                moveActionFocus("Down");
            return;
        }
        if ((event.key === Qt.Key_Tab || event.key === Qt.Key_Backtab)
                && windowRail && windowRail.hasWindows) {
            interactionRegion = LauncherState.toggleFocusRegion(
                interactionRegion, true);
            hero.focusField();
            return;
        }
        if (interactionRegion === "windows" && windowRail
                && windowRail.hasWindows) {
            if (event.key === Qt.Key_Left) {
                windowRail.focusRelative(-1);
                return;
            }
            if (event.key === Qt.Key_Right) {
                windowRail.focusRelative(1);
                return;
            }
        }
        if (event.key === Qt.Key_Left || event.key === Qt.Key_Right
                || event.key === Qt.Key_Up || event.key === Qt.Key_Down) {
            if (event.key === Qt.Key_Left)
                moveSelectionByKey("Left");
            else if (event.key === Qt.Key_Right)
                moveSelectionByKey("Right");
            else if (event.key === Qt.Key_Up)
                moveSelectionByKey("Up");
            else
                moveSelectionByKey("Down");
            return;
        }
        if (actionBrowse && event.key === Qt.Key_Tab)
            drawer.cycleCategory(event.modifiers & Qt.ShiftModifier ? -1 : 1);
        else if (actionBrowse && event.key === Qt.Key_Backtab)
            drawer.cycleCategory(-1);
    }

    function requestOuterGeometry() {
        if (!shown || observedLifecycleGeneration !== lifecycleGeneration)
            return;
        var target = Math.ceil(desiredCardHeight);
        var bodyChanged = bodyOpen !== lastBodyOpenForGeometry;
        var shelfChanged = shelfOpen !== lastShelfOpenForGeometry;
        lastBodyOpenForGeometry = bodyOpen;
        lastShelfOpenForGeometry = shelfOpen;
        var duration = bodyChanged
            ? Math.max(Motion.shutter, Motion.drawer)
            : (shelfChanged ? Motion.shelf : Motion.drawer);
        var decision = LauncherState.planOuterGeometry(
            lastOuterHeight, target, outerGeometryGeneration);
        outerGeometryGeneration = decision.generation;
        pendingOuterHeight = decision.pendingHeight;
        if (decision.cancelShrink)
            shrinkOuter.stop();
        if (decision.requestHeight > 0) {
            lastOuterHeight = decision.requestHeight;
            outerHeightRequested(
                decision.requestHeight, decision.growing, duration);
        } else if (decision.armShrink) {
            shrinkOuter.generation = decision.generation;
            shrinkOuter.duration = duration;
            shrinkOuter.restart();
        }
    }

    function stateDump() {
        var exportedResults = [];
        var count = Math.min(results.length, 12);
        for (var index = 0; index < count; index++) {
            var row = results[index] || {};
            exportedResults.push({
                resultKey: String(row.resultKey || ""),
                providerId: String(row.providerId || ""),
                title: String(row.title || ""),
                subtitle: String(row.subtitle || ""),
                type: String(row.type || ""),
                disabled: Boolean(row.disabled),
                secondaryActionCount: row.secondaryActions
                    ? row.secondaryActions.length : 0,
                verb: row.primaryAction
                    ? String(row.primaryAction.name || "") : String(row.verb || "")
            });
        }
        var exportedActions = [];
        for (var actionIndex = 0; actionIndex < selectedActions.length; actionIndex++)
            exportedActions.push(String(selectedActions[actionIndex].name || ""));
        var actionRowCellCounts = [];
        var actionRowFullWidth = [];
        for (var rowIndex = 0;
                rowIndex < packedActionLayout.length; rowIndex++) {
            var packedRow = packedActionLayout[rowIndex] || {};
            var rowActions = packedRow.actions || packedRow;
            actionRowCellCounts.push(
                Array.isArray(rowActions) ? rowActions.length : 0);
            actionRowFullWidth.push(Boolean(packedRow.fullWidth));
        }
        return {
            phase: currentPhase,
            activeMode: activeMode,
            query: query,
            selectedResultKey: selectedResultKey,
            secondaryActionCount: secondaryActions.length,
            focusedActionId: focusedActionId,
            shelfOpen: shelfOpen,
            surfaceWidth: Math.round(currentSurfaceWidth),
            surfaceHeight: Math.round(currentSurfaceHeight),
            heroHeight: Math.round(hero.height),
            drawerHeight: Math.round(drawer.height),
            drawerOpacity: Number(drawer.opacity),
            inputFocused: hero.inputFocused,
            windowRailActive: Boolean(windowRail && windowRail.hasWindows),
            windowRailCount: windowRail ? windowRail.windowCount : 0,
            windowRailAddresses: windowRail ? windowRail.windowAddresses : [],
            windowRailFocusSerial: windowRail ? windowRail.focusSerial : 0,
            windowRailFocusActive: interactionRegion === "windows",
            focusedWindowAddress: windowRail
                ? String(windowRail.focusedAddress || "") : focusedWindowAddress,
            shelfHeight: Math.round(drawer.actionShelfHeight),
            actionRowCellCounts: actionRowCellCounts,
            actionRowFullWidth: actionRowFullWidth,
            actionVisibleRange: String(drawer.actionVisibleRange || ""),
            palette: {
                lead: String(Theme.leadContainer),
                onLead: String(Theme.onLead),
                drawer: String(Theme.drawer),
                frame: String(Theme.frame)
            },
            resting: resting,
            gridMode: gridMode,
            help: help,
            answerMode: answerMode,
            actionMode: actionMode,
            modeLabel: modeLabel,
            searching: searching,
            busy: Dispatcher.busy,
            resultDeckOpacity: Number(drawer.resultDeckOpacity),
            resultCount: results.length,
            totalCount: totalCount,
            busyIds: Object.keys(Dispatcher.busyProviders),
            selectedIndex: selectedIndex,
            panelOpen: shelfOpen,
            selectedActions: exportedActions,
            hasMedia: hasMedia,
            results: exportedResults
        };
    }

    onQueryChanged: routeVisibleQuery()
    onResultRouteKeyChanged: {
        emptySettlement.stop();
        resultPresentation.stop();
        freshResultsToken = "";
        freshResults = [];
        displayRouteKey = resultRouteKey;
        displayQueryToken = resultQueryToken;
        displayWasBusy = false;
        results = [];
        scheduleEvaluation();
    }
    onResultQueryTokenChanged: {
        initialSelectionToken = resultQueryToken;
        resultPresentation.stop();
        emptySettlement.stop();
        freshResultsToken = "";
        freshResults = [];
        displayQueryToken = resultQueryToken;
        displayWasBusy = false;
        clearDisplayedResultsForInput();
        scheduleEvaluation();
    }
    onBusyForActiveProviderChanged: {
        if (busyForActiveProvider) {
            displayWasBusy = true;
            emptySettlement.stop();
        } else if (displayWasBusy) {
            displayWasBusy = false;
            emptySettlement.stop();
            scheduleEvaluation();
        } else {
            scheduleResultPresentation();
        }
    }
    onResultsChanged: reconcileResults()
    onCurrentActionSignatureChanged: {
        if (shelfOpen && shelfSignature !== currentActionSignature)
            closeShelf();
    }
    onDesiredCardHeightChanged: requestOuterGeometry()
    onLifecycleGenerationChanged: syncInvocationGeometry()
    onCurrentPhaseChanged: {
        if (shown && currentPhase !== "closing"
                && currentPhase !== "closed") {
            Qt.callLater(hero.focusField);
            phaseFocusSettler.restart();
        }
    }
    onReportedSurfaceHeightChanged: retainInputFocus()
    onShownChanged: {
        if (shown) {
            prepareRest();
            focusedWindowAddress = "";
            snapHiddenGeometry();
            syncInvocationGeometry();
            scheduleEvaluation();
            Qt.callLater(hero.focusField);
        } else {
            prepareRest();
            focusedWindowAddress = "";
            snapHiddenGeometry();
            scheduleEvaluation();
        }
    }

    Component.onCompleted: {
        validateProviderSet();
        syncInvocationGeometry();
        scheduleEvaluation();
    }

    Connections {
        target: Dispatcher
        function onRevisionChanged() { root.scheduleEvaluation(); }
    }

    Binding {
        target: providerSet.actions
        property: "activeCategory"
        value: drawer.activeCategory
        when: root.shown && root.actionMode
    }

    Timer {
        id: shrinkOuter
        property int generation: 0
        property int duration: Motion.drawer
        interval: Math.max(Motion.shutter, Motion.drawer, Motion.shelf)
        repeat: false
        onTriggered: {
            var desired = Math.ceil(root.desiredCardHeight);
            if (LauncherState.acceptOuterShrink(
                    generation, root.outerGeometryGeneration,
                    root.pendingOuterHeight, desired, root.lastOuterHeight)) {
                root.lastOuterHeight = root.pendingOuterHeight;
                root.outerHeightRequested(
                    root.pendingOuterHeight, false, duration);
            }
        }
    }

    Timer {
        id: resultPresentation
        interval: LauncherConfig.resultSettleMs
        repeat: false
        onTriggered: root.syncDisplayedResults()
    }

    Timer {
        id: emptySettlement
        interval: 240
        repeat: false
        onTriggered: {
            if (root.displayRouteKey === root.resultRouteKey
                    && root.displayQueryToken === root.resultQueryToken
                    && LauncherState.snapshotMatchesToken(
                        root.freshResultsToken, root.resultQueryToken)
                    && !root.busyForActiveProvider
                    && root.freshResults.length === 0)
                root.results = [];
        }
    }

    Timer {
        id: focusRepair
        interval: 16
        repeat: false
        onTriggered: {
            if (root.shown && !hero.inputFocused)
                hero.focusField();
        }
    }

    Timer {
        id: typedFocusSettler
        interval: 260
        repeat: false
        onTriggered: {
            if (root.shown && root.bodyOpen && !hero.inputFocused)
                hero.focusField();
        }
    }

    Timer {
        id: phaseFocusSettler
        interval: 140
        repeat: false
        onTriggered: {
            if (root.shown && root.currentPhase !== "closing"
                    && root.currentPhase !== "closed" && !hero.inputFocused)
                hero.focusField();
        }
    }

    HeroVariant.HeroShutter {
        id: hero
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: (root.bodyOpen ? 126 : 250) * root.s
        s: root.s
        shown: root.shown
        compressed: root.bodyOpen
        shelfOpen: root.shelfOpen
        windowRailActive: Boolean(windowRail && windowRail.hasWindows)
        windowRailFocused: root.interactionRegion === "windows"
        resultNavigationActive: root.bodyOpen && !root.shelfOpen
        activeMode: root.activeMode
        onMoved: delta => root.moveSelection(delta)
        onAccepted: root.activateCurrent()
        onDismissed: root.dismissStage()
        onCompositionStarted: root.handleCompositionStarted()
        onHelpRequested: root.showHelp()
        onModeRequested: mode => root.selectMode(mode)
        onKeyPressed: event => root.handleInputKey(event)
        onInputFocusedChanged: {
            if (!inputFocused)
                root.retainInputFocus();
        }

        Behavior on height {
            enabled: root.transitionAnimationsEnabled
                && !root.snappingHiddenGeometry
            NumberAnimation {
                id: heroHeightMotion
                duration: Motion.shutter
                easing.type: Motion.easeMorph
                easing.bezierCurve: Motion.morphCurve
            }
        }
    }

    Loader {
        id: drawerLoader
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: hero.bottom
        active: root.bodyOpen || drawerHold.running
        asynchronous: false

        sourceComponent: Component {
            HeroVariant.ResultDrawer {
                s: root.s
                open: root.shown && root.bodyOpen
                animationsEnabled: root.transitionAnimationsEnabled
                frostActive: root.frostActive
                maxHeight: root.cardGeometry.bodyHeight
                results: root.results
                selectedResult: root.selectedResult
                selectedResultKey: root.selectedResultKey
                shelfOpen: root.shelfOpen
                secondaryActions: root.secondaryActions
                packedLayout: root.packedActionLayout
                focusedActionId: root.focusedActionId
                helpMode: root.help
                answerMode: root.answerMode
                actionBrowse: root.actionBrowse
                searching: root.searching
                emptyText: root.emptyText
                errorText: root.actionError
                windowCount: windowRail ? windowRail.windowCount : 0
                windowFocusActive: root.interactionRegion === "windows"
                answer: providerSet.web ? providerSet.web.answer : ({ available: false })
                onLeadActivated: root.activateCurrent()
                onResultSelected: (resultKey, rank) => root.selectResult(resultKey, rank)
                onActionFocusRequested: actionId => root.focusedActionId = actionId
                onActionExecuteRequested: actionId => root.executeAction(
                    root.currentSecondaryAction(actionId))
            }
        }
    }

    Timer {
        id: drawerHold
        interval: 1000
    }

}

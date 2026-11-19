pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import Lithic
import Ryoku.Ui
import Ryoku.Ui.Singletons
import shell.services
import "../ask"

Scope {
    id: root
    required property var screen
    property var cards: []
    property var selection: null
    property var highlight: null
    property var inplace: null
    property var speaking: null
    property string handledInplaceId: ""
    property var inplaceContext: null
    property var inplaceWorker: null
    property bool inplaceWaiting: false
    property var replacement: null
    property string sheetCardId: ""
    property bool highlightSheet: false
    property string handledHighlightId: ""
    property bool probingPointer: false
    property bool highlightOpening: false
    property bool hidden: false
    property bool active: false
    property bool completed: false
    property var surfaces: []
    property var closingCards: ({})
    property int nextZ: 0
    property int revision: 0
    property string phase: "idle"
    property var selectContext: null
    property var frozenFrame: null
    property var frozenWindows: []
    property bool pendingCommit: false
    property var operation: null
    property string committedId: ""
    property rect committedRect: Qt.rect(0, 0, 0, 0)
    property string selectionError: ""
    readonly property real s: Math.max(0.5, Math.min(2, Number(scaleSetting.value) || 1))
    readonly property var clearances: SidebarState.railClearances(screen)
    readonly property rect usable: Qt.rect(clearances.left, clearances.top,
        Math.max(1, panel.width - clearances.left - clearances.right),
        Math.max(1, panel.height - clearances.top - clearances.bottom))
    readonly property real styleAlpha: Lithic.design === "material" ? 0.40 : Lithic.design === "adwaita" ? 0.50 : Lithic.design === "fluent" ? 0.45 : 0.58
    readonly property bool selecting: phase === "select"
    readonly property var modal: {
        revision;
        for (var i = surfaces.length - 1; i >= 0; --i)
            if (!surfaces[i].kept && !surfaces[i].closing) return surfaces[i];
        return null;
    }
    readonly property rect hole: selecting ? selectView.hole : modal ? modal.plateRect : Qt.rect(0, 0, 0, 0)
    readonly property bool dragging: { revision; return surfaces.some(function(group) { return group.panelItem && group.panelItem.dragging === true; }); }
    readonly property bool focused: { revision; return surfaces.some(function(group) { return group.panelTarget && group.panelTarget.activeFocus; }); }
    readonly property bool popupOpen: { revision; return surfaces.some(function(group) { return group.panelItem && group.panelItem.popupOpen === true; }); }
    property real frameReveal: selecting ? 1 : 0
    property real scrimReveal: selecting ? 1 : modal && modal.card.kind !== "text" && !hidden ? 0.55 : 0
    readonly property bool fullInput: selecting || pendingCommit || modal !== null || scrimReveal > 0.001 || dragging || popupOpen || probingPointer || inplaceWaiting
    readonly property bool windowShown: selecting || pendingCommit || frameReveal > 0.001 || scrimReveal > 0.001
        || (!hidden && (surfaces.length > 0 || probingPointer || bubble.presented || highlightOpening || inplaceWaiting || toast.presented))
    Behavior on frameReveal { enabled: !root.selecting; NumberAnimation { duration: Lithic.duration(150); easing.type: Easing.OutCubic } }
    Behavior on scrimReveal { NumberAnimation { duration: Lithic.duration(150); easing.type: Easing.OutCubic } }

    ShellSetting { id: scaleSetting; path: ["displays", "ui_scale", root.screen ? root.screen.name : ""]; fallback: 1 }
    RashinClient { id: rashin }
    TheiaCatalog { id: actionCatalog }

    function languageName(code) {
        if (code === "zh-Hans") code = "zh_CN";
        else if (code === "zh-Hant") code = "zh_TW";
        for (var i = 0; i < I18n.langs.length; ++i)
            if (String(I18n.langs[i].code) === String(code)) return String(I18n.langs[i].name);
        return String(code || "English");
    }
    function finishInplace() {
        const context = inplaceContext;
        inplaceContext = null;
        inplaceWaiting = false;
        if (inplaceWorker) { inplaceWorker.destroy(); inplaceWorker = null; }
        if (context) Bus.call("sight.inplaceDone", { id: String(context.id) });
    }
    function cancelInplace() {
        if (replacement && replacement.inplace) { replacement = null; replaceWait.stop(); }
        if (inplaceWorker) inplaceWorker.cancel();
        finishInplace();
        toast.dismiss();
    }
    function syncInplace() {
        if (!completed) return;
        if (!inplace || !actionCatalog.feature("inplace")) {
            if (inplaceContext) cancelInplace();
            return;
        }
        const id = String(inplace.id || "");
        if (!id || id === handledInplaceId) return;
        cancelInplace();
        handledInplaceId = id;
        inplaceContext = inplace;
        for (var i = 0; i < surfaces.length; ++i) surfaces[i].kept = true;
        revision++;
        probingPointer = false;
        bubble.dismissBubble(true);
        inplaceWaiting = true;
        pointerWait.restart();
        locatePointer();
    }
    function startInplace(point) {
        if (!inplaceContext || !inplaceWaiting) return;
        inplaceWaiting = false;
        const action = String(inplaceContext.action || "auto");
        toast.show(action === "auto" ? I18n.tr("Fixing or translating…") : action === "translate" ? I18n.tr("Translating…") : action === "proofread" ? I18n.tr("Fixing…") : I18n.tr("Rewriting…"), "working", false, point);
        inplaceWorker = transformComponent.createObject(root, { context: inplaceContext });
        canvas.forceActiveFocus();
    }
    function transformed(context, result, error) {
        if (!inplaceContext || String(inplaceContext.id) !== String(context.id)) return;
        const text = String(result && result.text || "");
        if (error || !text.trim()) {
            finishInplace();
            toast.show(error && error.message ? String(error.message) : I18n.tr("The writing model returned no text."), "error", false);
            return;
        }
        const action = String(context.action || "");
        const label = action === "auto" ? I18n.tr("Replaced") : action === "translate" ? I18n.tr("Translated") : action === "proofread" ? I18n.tr("Fixed") : I18n.tr("Rewritten");
        if (inplaceWorker) { inplaceWorker.destroy(); inplaceWorker = null; }
        queueReplace(text, label, true);
    }
    function queueReplace(text, label, fromInplace) {
        replacement = { text: text, label: label, inplace: fromInplace === true, context: fromInplace ? inplaceContext : null };
        replaceWait.restart();
    }
    function replaceCard(group, text) {
        if (!text.trim()) return;
        const target = group.panelTarget;
        const point = target ? Qt.point(target.x + group.offsetX + target.width / 2, target.y + group.offsetY) : Qt.point(usable.x + usable.width / 2, usable.y + usable.height / 2);
        toast.pointer = point;
        group.kept = true;
        closeCard(String(group.card.id));
        canvas.forceActiveFocus();
        queueReplace(text, I18n.tr("Replaced"), false);
    }
    function undoReplace() {
        toast.dismiss();
        canvas.forceActiveFocus();
        replacement = { undo: true };
        replaceWait.restart();
    }
    Timer {
        id: replaceWait
        interval: 150
        onTriggered: {
            const request = root.replacement;
            root.replacement = null;
            if (!request) return;
            Bus.call(request.undo ? "sight.undo" : "sight.replace", request.undo ? ({}) : {
                text: request.text, mode: actionCatalog.option("inplace.paste", true) === false ? "type" : "paste"
            }, function(ok, result, error) {
                if (request.inplace && root.inplaceContext !== request.context) return;
                if (request.inplace) root.finishInplace();
                const code = String(error || "");
                const reason = code === "no-wtype" ? I18n.tr("Install wtype to replace selected text.") : code === "no-text" ? I18n.tr("There is no text to replace.") : I18n.tr("The text could not be pasted here.");
                const copied = ok && result && result.copied === true;
                const label = copied ? I18n.tr("Copied. Terminals cannot change selected text, paste it where you need it.") : request.undo ? I18n.tr("Undone") : request.label;
                toast.show(ok ? label : reason, ok ? "done" : "error", ok && !request.undo && !copied);
            });
        }
    }
    Component {
        id: transformComponent
        Scope {
            id: worker
            required property var context
            property bool sent: false
            readonly property string requestId: "theia-" + String(context.id)
            function cancel() {
                if (!sent) return;
                const transport = rashin.ready ? rashin : transformClient;
                transport.call("writing.cancel", { request: requestId });
            }
            function run() {
                if (sent || !transformClient.ready) return;
                sent = true;
                var action = String(context.action || "auto");
                var options = {};
                if (action === "auto") { action = "translate"; options.fix = "on"; }
                if (action.indexOf("tone:") === 0) { options.tone = action.slice(5); action = "tone"; }
                if (action === "rewrite") {
                    options.variant = String(actionCatalog.option("rewrite.variant", "rephrase"));
                    options.audience = String(actionCatalog.option("rewrite.audience", "general"));
                }
                if (action === "translate") {
                    options.style = String(actionCatalog.option("translate.style", "natural"));
                    if (actionCatalog.option("translate.simpler", false) === true) options.simpler = "on";
                }
                if (actionCatalog.feature("voice") && actionCatalog.option("voice.use", true) !== false) options.voice = "on";
                transformClient.call("writing.transform", {
                    request: requestId, text: String(context.text || ""), action: action, options: options,
                    language: root.languageName(context.language || I18n.lang),
                    target: root.languageName(actionCatalog.option("target", "") || I18n.lang),
                    model: String(actionCatalog.option("model", "")), local: actionCatalog.option("localOnly", false) === true
                }, function(result, error) { root.transformed(worker.context, result, error); });
            }
            RashinClient {
                id: transformClient
                onBecameReady: worker.run()
                onStateChanged: {
                    if (state === "off" || state === "not-installed" || state === "error")
                        root.transformed(worker.context, null, { message: state === "off" ? I18n.tr("Turn on Rashin to change selected text.") : state === "not-installed" ? I18n.tr("Install Rashin to change selected text.") : transformClient.errorText || I18n.tr("Rashin could not be reached.") });
                }
            }
            Component.onCompleted: run()
        }
    }

    function syncHighlight() {
        if (!completed) return;
        if (!highlight) {
            probingPointer = false;
            if (!highlightOpening && bubble.shown) bubble.dismissBubble(true);
            return;
        }
        const id = String(highlight.id || "");
        if (!id || id === handledHighlightId) return;
        handledHighlightId = id;
        bubble.dismissBubble(true);
        probingPointer = !selecting && !modal && !hidden;
        if (probingPointer) { pointerWait.restart(); locatePointer(); }
    }
    // The overlay waits for the pointer to enter it, and a compositor may send
    // no enter to a surface it maps under a pointer that is not moving. Where it
    // can say where the pointer is, ask instead, so a still hand gets its bubble.
    function locatePointer() {
        if (!Wm.caps.pointerPosition) return;
        Wm.locatePointer(function(position) {
            if (!position) return;
            if (String(position.output) === String(root.screen.name)) root.foundPointer(position.x, position.y);
            else root.probingPointer = false;
        });
    }
    function foundPointer(x, y) {
        if (inplaceWaiting) { startInplace(Qt.point(x, y)); return; }
        if (!probingPointer || !highlight || selecting || modal || hidden) return;
        const context = highlight;
        probingPointer = false;
        bubble.showAt(context, Qt.point(x, y));
    }
    function dismissHighlight() {
        const id = bubble.context ? String(bubble.context.id || "") : "";
        probingPointer = false;
        bubble.dismissBubble(false);
        if (id) Bus.call("sight.highlightDismiss", { id: id });
    }
    function openHighlight(mode, sheet) {
        if (!bubble.shown || !bubble.context || highlightOpening) return;
        const id = String(bubble.context.id || "");
        if (!highlight || id !== String(highlight.id || "")) {
            bubble.dismissBubble(true);
            return;
        }
        highlightOpening = true;
        highlightSheet = sheet === true;
        bubble.dismissBubble(false);
        Bus.call("sight.highlightOpen", { id: id, mode: mode,
            x: bubble.pointer.x + Number(screen.x), y: bubble.pointer.y + Number(screen.y), output: screen.name }, function(ok, result) {
            root.highlightOpening = false;
            if (ok) {
                if (root.highlightSheet) root.sheetCardId = String(result.id || "");
                root.reconcile();
                root.openHighlightSheet();
            }
            root.highlightSheet = false;
        });
    }
    function openHighlightSheet() {
        if (!sheetCardId) return;
        for (var i = 0; i < surfaces.length; ++i) {
            if (String(surfaces[i].card.id) !== sheetCardId || !surfaces[i].panelItem) continue;
            surfaces[i].panelItem.sheetOpen = true;
            sheetCardId = "";
            return;
        }
    }
    Timer {
        id: pointerWait
        interval: 1500
        running: root.probingPointer || root.inplaceWaiting
        onTriggered: {
            root.probingPointer = false;
            if (root.inplaceWaiting) root.startInplace(Qt.point(root.usable.x + root.usable.width / 2, root.usable.y + root.usable.height / 2));
        }
    }

    function frameFor(context) {
        const frames = context && context.frames || [];
        for (var i = 0; i < frames.length; ++i)
            if (String(frames[i].output) === String(screen.name)) return frames[i];
        return null;
    }
    function snapshotWindows() {
        if (!Wm.caps.windowGeometry) return [];
        const workspaces = Wm.workspaces.filter(function(workspace) { return workspace.active && workspace.output === root.screen.name; });
        return Wm.windows.filter(function(window) {
            return window.output === root.screen.name && window.width > 0 && window.height > 0
                && (!workspaces.length || workspaces.some(function(workspace) { return String(workspace.id) === String(window.workspace) || workspace.name === window.workspace; }));
        }).sort(function(a, b) { return Number(b.focusOrder) - Number(a.focusOrder); });
    }
    function syncSelection() {
        if (!completed) return;
        const frame = frameFor(selection);
        if (frame) {
            if (selectContext && selectContext.id === selection.id) return;
            for (var i = 0; i < surfaces.length; ++i) surfaces[i].kept = true;
            selectContext = selection;
            frozenWindows = snapshotWindows();
            frozenFrame = frame;
            pendingCommit = false;
            committedId = "";
            selectionError = "";
            phase = "select";
            revision++;
        } else if (selecting && !pendingCommit) {
            phase = surfaces.length ? "answer" : "idle";
            selectContext = null;
        }
    }
    function commit(rect) {
        if (!selectContext || !frozenFrame || pendingCommit || rect.width < 8 || rect.height < 8) return;
        const selectId = String(selectContext.id);
        const token = { canceled: false };
        operation = token;
        committedRect = rect;
        pendingCommit = true;
        selectionError = "";
        Bus.call("sight.commit", { id: selectId, output: screen.name,
            x: rect.x + Number(frozenFrame.x), y: rect.y + Number(frozenFrame.y), width: rect.width, height: rect.height }, function(ok, result) {
            if (token.canceled) {
                if (ok) root.closeCard(String(result.id || ""));
                return;
            }
            if (!root.selectContext || String(root.selectContext.id) !== selectId) return;
            if (!ok) {
                root.pendingCommit = false;
                root.selectionError = I18n.tr("That area could not be captured. Try another box.");
                if (!root.frameFor(root.selection)) root.cancelSelection();
                return;
            }
            root.committedId = String(result.id || "");
            root.reconcile();
        });
    }
    function cancelSelection() {
        if (operation && pendingCommit) operation.canceled = true;
        if (selectContext) Bus.call("sight.cancel", { id: String(selectContext.id) });
        selectContext = null;
        pendingCommit = false;
        phase = surfaces.length ? "answer" : "idle";
    }
    function matchesCommit(card) {
        if (!pendingCommit) return false;
        if (committedId) return String(card.id) === committedId;
        if (card.kind === "text") return false;
        return card.plate === true && frozenFrame && Math.abs(Number(card.x) - Number(frozenFrame.x) - committedRect.x) < 1
            && Math.abs(Number(card.y) - Number(frozenFrame.y) - committedRect.y) < 1
            && Math.abs(Number(card.width) - committedRect.width) < 1 && Math.abs(Number(card.height) - committedRect.height) < 1;
    }
    function touch(group) { group.z = ++nextZ; revision++; }
    function keep(group) { group.kept = true; revision++; }
    function dismiss(group, notify) {
        if (!group || group.closing) return;
        if (speaking && String(speaking.card || "") === String(group.card.id)) Bus.call("sight.stopSpeaking", {});
        group.notifyClose = notify !== false;
        group.closing = true;
        group.finishTimer.restart();
        revision++;
    }
    function pin(group) {
        if (!group || group.card.kind === "text" || group.pinning) return;
        group.pinning = true;
        group.pinProcess.running = true;
    }
    function closeCard(id) {
        if (!id || closingCards[id]) return;
        const next = Object.assign({}, closingCards);
        next[id] = true;
        closingCards = next;
        Bus.call("sight.close", { id: id });
        for (var i = 0; i < surfaces.length; ++i)
            if (String(surfaces[i].card.id) === id) dismiss(surfaces[i], false);
    }
    function retire(group) {
        const regions = inputRegion.regions;
        for (var i = regions.length - 1; i >= 0; --i)
            if (regions[i] === group.plateInput || regions[i] === group.panelInput || regions[i] === group.overlayInput) regions.splice(i, 1);
        surfaces = surfaces.filter(function(item) { return item !== group; });
        if (group.notifyClose) closeCard(String(group.card.id));
        group.destroy();
        if (!surfaces.length && !selecting) phase = "idle";
        revision++;
    }
    function reconcile() {
        if (!completed) return;
        const next = [];
        const wanted = cards || [];
        for (var i = 0; i < wanted.length; ++i) {
            const data = wanted[i];
            if (closingCards[String(data.id)]) continue;
            var existing = null;
            for (var j = 0; j < surfaces.length; ++j)
                if (String(surfaces[j].card.id) === String(data.id)) { existing = surfaces[j]; break; }
            if (!existing) {
                const fromSelect = matchesCommit(data);
                if (!selecting || fromSelect) {
                    for (var k = 0; k < surfaces.length; ++k) surfaces[k].kept = true;
                }
                existing = groupComponent.createObject(canvas, { card: data, z: ++nextZ, kept: selecting && !fromSelect });
                if (existing) {
                    inputRegion.regions.push(existing.plateInput);
                    inputRegion.regions.push(existing.panelInput);
                    inputRegion.regions.push(existing.overlayInput);
                }
            }
            if (existing) {
                next.push(existing);
                if (matchesCommit(data)) {
                    existing.kept = false;
                    pendingCommit = false;
                    phase = "answer";
                    selectContext = null;
                }
            }
        }
        for (var n = 0; n < surfaces.length; ++n) {
            const old = surfaces[n];
            if (next.indexOf(old) < 0) { dismiss(old, false); next.push(old); }
        }
        surfaces = next;
        revision++;
        openHighlightSheet();
    }
    onSelectionChanged: syncSelection()
    onCardsChanged: reconcile()
    onHighlightChanged: syncHighlight()
    onInplaceChanged: syncInplace()
    onSelectingChanged: if (selecting) { if (inplaceContext) cancelInplace(); probingPointer = false; bubble.dismissBubble(true); }
    onModalChanged: if (modal) { probingPointer = false; bubble.dismissBubble(!highlightOpening); }
    onHiddenChanged: if (hidden) { if (inplaceContext) cancelInplace(); probingPointer = false; bubble.dismissBubble(true); toast.dismiss(); }
    Component.onCompleted: { completed = true; syncSelection(); reconcile(); syncHighlight(); syncInplace(); }
    Component.onDestruction: {
        if (inplaceWorker) inplaceWorker.cancel();
        if (inplaceContext) Bus.call("sight.inplaceDone", { id: String(inplaceContext.id) });
    }

    property Component answerPanel: Component { SightPanel {} }
    Component {
        id: groupComponent
        Item {
            id: group
            required property var card
            property bool kept: false
            property bool closing: false
            property bool notifyClose: false
            property bool pinning: false
            property bool pinned: false
            property string errorText: ""
            property real offsetX: 0
            property real offsetY: 0
            property var panelItem: null
            readonly property var panelTarget: panelItem ? panelItem.inputItem || panelItem : null
            readonly property rect plateRect: card.kind === "text" ? Qt.rect(Number(card.x || 0) - Number(root.screen.x), Number(card.y || 0) - Number(root.screen.y), 0, 0)
                : Qt.rect(Number(card.x) - Number(root.screen.x), Number(card.y) - Number(root.screen.y), Number(card.width), Number(card.height))
            readonly property var plateInput: plateRegion
            readonly property var panelInput: panelRegion
            readonly property var overlayInput: overlayRegion
            readonly property var finishTimer: finish
            readonly property var pinProcess: pinProc
            property real plateReveal: closing && !pinned ? 0 : 1
            property real plateScale: closing && !pinned ? 0.985 : 1
            visible: !root.hidden && !root.selecting
            width: panel.width; height: panel.height
            transform: Translate { x: group.offsetX; y: group.offsetY }
            Behavior on plateReveal { NumberAnimation { duration: Lithic.duration(group.pinned ? 0 : 120) } }
            Behavior on plateScale { NumberAnimation { duration: Lithic.duration(120); easing.type: Easing.InCubic } }
            Region {
                id: plateRegion
                x: group.plateRect.x + group.offsetX; y: group.plateRect.y + group.offsetY
                width: group.card.kind === "text" ? 0 : group.plateRect.width
                height: group.card.kind === "text" ? 0 : group.plateRect.height
                intersection: Intersection.Combine
            }
            Region {
                id: panelRegion
                x: (group.panelTarget ? group.panelTarget.x : 0) + group.offsetX
                y: (group.panelTarget ? group.panelTarget.y : 0) + group.offsetY
                width: group.panelTarget ? group.panelTarget.width : 0
                height: group.panelTarget ? group.panelTarget.height : 0
                intersection: Intersection.Combine
            }
            Region {
                id: overlayRegion
                x: group.plateRect.x + group.offsetX; y: group.plateRect.y + group.offsetY
                width: overlay.item && overlay.item.interactive && group.visible && !group.closing ? group.plateRect.width : 0
                height: overlay.item && overlay.item.interactive && group.visible && !group.closing ? group.plateRect.height : 0
                intersection: Intersection.Combine
            }
            SightPlate {
                id: plate
                rect: group.plateRect; source: group.card.kind === "text" ? "" : "file://" + group.card.image
                visible: group.card.kind !== "text"
                committed: true; s: root.s
                opacity: group.plateReveal; scale: group.plateScale
                MouseArea { anchors.fill: parent; onPressed: root.touch(group) }
            }
            Loader {
                id: overlay
                active: group.visible && !group.closing && group.panelItem !== null && group.panelItem.overlayLines.length > 0
                sourceComponent: Component {
                    TheiaOverlay { plateRect: group.plateRect; lines: group.panelItem ? group.panelItem.overlayLines : []; s: root.s }
                }
                opacity: group.plateReveal
            }
            Item {
                id: panelMotion
                anchors.fill: parent
                opacity: 1
            }
            Label {
                visible: group.errorText.length > 0
                x: Math.max(root.usable.x, group.plateRect.x); y: Math.max(root.usable.y, group.plateRect.y)
                text: group.errorText; typeRole: "detail"; inkRole: "jasper"
            }
            Connections {
                target: group.panelItem
                ignoreUnknownSignals: true
                function onDismiss() { root.dismiss(group, true); }
                function onKeep() { root.keep(group); }
                function onPin() { root.pin(group); }
                function onTouched() { root.touch(group); }
                function onReplaceRequested(text) { root.replaceCard(group, text); }
                function onNotNow() {
                    Bus.call("sight.quietNow", { app: String(group.card.app || "") });
                    root.dismiss(group, true);
                }
                function onDragMoved(dx, dy) {
                    if (!group.kept || !group.panelTarget || group.closing) return;
                    const target = group.panelTarget;
                    const left = Math.min(group.plateRect.x, target.x);
                    const top = Math.min(group.plateRect.y, target.y);
                    const right = Math.max(group.plateRect.x + group.plateRect.width, target.x + target.width);
                    const bottom = Math.max(group.plateRect.y + group.plateRect.height, target.y + target.height);
                    group.offsetX = Math.max(root.usable.x - left, Math.min(root.usable.x + root.usable.width - right, group.offsetX + dx));
                    group.offsetY = Math.max(root.usable.y - top, Math.min(root.usable.y + root.usable.height - bottom, group.offsetY + dy));
                }
                function onDraggingChanged() { root.revision++; }
                function onActiveFocusChanged() { root.revision++; }
            }
            Component.onCompleted: {
                panelItem = root.answerPanel.createObject(panelMotion, { card: card, plateRect: plateRect,
                    screen: root.screen, s: root.s, client: rashin, usable: root.usable });
                if (panelItem) {
                    panelItem.plateRect = Qt.binding(function() { return group.plateRect; });
                    panelItem.usable = Qt.binding(function() { return root.usable; });
                    panelItem.s = Qt.binding(function() { return root.s; });
                    panelItem.kept = Qt.binding(function() { return group.kept; });
                    panelItem.closing = Qt.binding(function() { return group.closing; });
                    panelItem.dragOffset = Qt.binding(function() { return Qt.point(group.offsetX, group.offsetY); });
                    panelItem.speakingId = Qt.binding(function() { return root.speaking ? String(root.speaking.id || "") : ""; });
                    root.openHighlightSheet();
                }
            }
            Timer { id: finish; interval: Lithic.duration(150); onTriggered: root.retire(group) }
            Process {
                id: pinProc
                environment: Spawn.env
                command: ["ryoku-cmd-pin", String(group.card.image || ""), "--at", Math.round(Number(group.card.x || 0) + group.offsetX) + "," + Math.round(Number(group.card.y || 0) + group.offsetY), "--output", String(root.screen.name)]
                onExited: code => {
                    group.pinning = false;
                    if (code === 0) { group.pinned = true; root.dismiss(group, true); }
                    else group.errorText = I18n.tr("The capture could not be pinned.");
                }
            }
        }
    }
    PanelWindow {
        id: panel
        screen: root.screen
        visible: root.windowShown
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        exclusiveZone: 0
        anchors { top: true; bottom: true; left: true; right: true }
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.namespace: "ryoku-sight"
        // release focus before replacing text in the source app
        WlrLayershell.keyboardFocus: root.selecting || (root.modal && !root.hidden) || root.inplaceWorker !== null ? WlrKeyboardFocus.Exclusive
            : root.probingPointer || bubble.presented || root.highlightOpening || root.inplaceWaiting ? WlrKeyboardFocus.None
            : root.focused || root.popupOpen || (toast.presented && toast.undoable) ? WlrKeyboardFocus.OnDemand : WlrKeyboardFocus.None
        mask: Region {
            id: inputRegion
            width: root.fullInput ? panel.width : 0
            height: root.fullInput ? panel.height : 0
            Region {
                x: bubble.x; y: bubble.y
                width: bubble.presented ? bubble.width : 0
                height: bubble.presented ? bubble.height : 0
                intersection: Intersection.Combine
            }
            Region {
                x: toast.x; y: toast.y
                width: toast.presented ? toast.width : 0
                height: toast.presented ? toast.height : 0
                intersection: Intersection.Combine
            }
        }
        FocusScope {
            id: canvas
            anchors.fill: parent
            readonly property real lithicScale: root.s
            Keys.onEscapePressed: {
                if (root.inplaceContext) root.cancelInplace();
                else if (root.selecting) root.cancelSelection();
                else if (root.modal) root.dismiss(root.modal, true);
            }
            Image {
                anchors.fill: parent
                source: root.frozenFrame ? "file://" + root.frozenFrame.path : ""
                sourceSize.width: root.frozenFrame ? Math.ceil(Number(root.frozenFrame.width) * Number(root.frozenFrame.scale || 1)) : 1
                sourceSize.height: root.frozenFrame ? Math.ceil(Number(root.frozenFrame.height) * Number(root.frozenFrame.scale || 1)) : 1
                fillMode: Image.Stretch; cache: false; asynchronous: false
                opacity: root.frameReveal
            }
            Item {
                anchors.fill: parent
                opacity: root.modal && root.modal.card.kind === "text" ? 0 : root.scrimReveal * root.styleAlpha
                Repeater {
                    model: [
                        Qt.rect(0, 0, panel.width, Math.max(0, root.hole.y)),
                        Qt.rect(0, root.hole.y, Math.max(0, root.hole.x), root.hole.height),
                        Qt.rect(root.hole.x + root.hole.width, root.hole.y, Math.max(0, panel.width - root.hole.x - root.hole.width), root.hole.height),
                        Qt.rect(0, root.hole.y + root.hole.height, panel.width, Math.max(0, panel.height - root.hole.y - root.hole.height))
                    ]
                    delegate: CutRect {
                        required property var modelData
                        x: modelData.x; y: modelData.y; width: modelData.width; height: modelData.height
                        fill: Qt.rgba(0, 0, 0, 1); cut: 0
                    }
                }
            }
            MouseArea {
                anchors.fill: parent
                enabled: !root.selecting && root.modal !== null
                acceptedButtons: Qt.AllButtons
                onPressed: root.dismiss(root.modal, true)
            }
            SightSelect {
                id: selectView
                anchors.fill: parent
                z: 1000000
                visible: root.selecting
                active: root.selecting
                frame: root.frozenFrame
                previous: root.selectContext ? root.selectContext.previous : null
                windows: root.frozenWindows
                busy: root.pendingCommit; errorText: root.selectionError
                usable: root.usable; s: root.s
                onCommitRequested: rect => root.commit(rect)
                onCancelRequested: root.cancelSelection()
            }
            MouseArea {
                id: pointerProbe
                anchors.fill: parent
                z: 1000001
                enabled: root.probingPointer || root.inplaceWaiting
                hoverEnabled: true
                acceptedButtons: Qt.NoButton
                onEntered: root.foundPointer(mouseX, mouseY)
                onPositionChanged: event => root.foundPointer(event.x, event.y)
            }
            SightBubble {
                id: bubble
                z: 1000002
                s: root.s
                catalog: actionCatalog
                bounds: Qt.rect(0, 0, panel.width, panel.height)
                onActionRequested: mode => root.openHighlight(mode)
                onMoreRequested: root.openHighlight("explain", true)
                onDismissRequested: root.dismissHighlight()
            }
            TheiaToast {
                id: toast
                z: 1000003
                s: root.s
                bounds: root.usable
                onCancelRequested: root.cancelInplace()
                onUndoRequested: root.undoReplace()
            }
        }
    }
}

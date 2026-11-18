pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui as RyokuUi
import Ryoku.Ui.Singletons
import "../ask"
import "placement.js" as Placement

FocusScope {
    id: root
    required property var card
    required property rect plateRect
    required property var screen
    required property real s
    required property RashinClient client
    required property rect usable
    property bool kept: false
    property point dragOffset: Qt.point(0, 0)
    property bool closing: false
    property bool placed: false
    property string mode: String(card.mode || (card.kind === "text" ? "translate" : "explain"))
    property string target: normalizeTarget(String(targetSetting.value || ""))
    readonly property string effectiveTarget: target || normalizeTarget(I18n.lang) || "en"
    property string detectedSource: card.kind === "text" ? String(card.language || "") : ""
    property string sourceOverride: ""
    readonly property string source: sourceOverride || detectedSource
    property bool translateAnyway: false
    readonly property bool alreadyInLanguage: mode === "translate" && ocrDone && !sourceOverride && detectedSource.length > 0
        && detectedSource === effectiveTarget && !translateAnyway
    property string ocrText: card.kind === "text" ? String(card.text || "") : ""
    property var codes: []
    property bool ocrDone: card.kind === "text"
    property bool ocrBusy: false
    property int ocrSerial: 0
    property bool keepBreaks: true
    property bool copied: false
    property bool looking: false
    property bool longWait: false
    property real originX: 0
    property real anchorY: 0
    property bool bottomAnchored: false
    property string side: "right"
    property real growthRoom: usable.height
    property real presentedHeight: 100 * s
    property real arrive: 0
    readonly property real lithicScale: s
    readonly property var session: conversation
    readonly property var inputItem: root
    readonly property var overlayHost: parent
    readonly property bool dragging: head.dragging
    readonly property bool popupOpen: more.popupOpen || language.popupOpen || provenance.popupOpen
    readonly property bool typing: ask.typing || textView.typing || popupOpen
    readonly property string ocrDisplay: keepBreaks ? ocrText : Placement.mergeLines(ocrText)
    readonly property string answerMode: mode === "text" ? "read" : mode
    readonly property string latest: String(conversation.answers[answerMode] || "")
    readonly property bool busy: conversation.busy || conversation.queued !== null || ocrBusy
    readonly property var planes: Lithic.planes(Lithic.design)
    readonly property color plane: Lithic.design === "lithic" ? planes.pane : Lithic.design === "fluent" ? Lithic.mix(planes.card, Lithic.sel, 0.025) : Lithic.stone4
    readonly property color stroke: Lithic.design === "material" ? "transparent" : Lithic.design === "adwaita" ? Lithic.alpha(Lithic.fg, Lithic.dark ? 0.10 : 0.16) : Lithic.design === "fluent" ? planes.cardStroke : Lithic.vein
    readonly property bool showProvenance: copied || conversation.model.length > 0
    readonly property bool errorShown: !alreadyInLanguage && (conversation.errorText.length > 0 || client.state === "off" || client.state === "not-installed" || client.state === "error")
    readonly property bool hasBody: errorShown || looking || (longWait && conversation.busy) || mode === "text" || alreadyInLanguage
        || (conversation.turns[mode] || []).length > 0 || (mode === "translate" && ocrDisplay.length > 0)
    readonly property real chromeHeight: 26 * s + head.height
        + (language.visible ? language.height + 13 * s : 0)
        + (ask.visible ? ask.height + 13 * s : 0)
        + (provenance.visible ? provenance.height + 13 * s : 0)
        + (hasBody ? 13 * s : 0)
    readonly property real maximumHeight: Math.max(1, Math.min(usable.height * 0.6, placed ? growthRoom + (bottomAnchored ? dragOffset.y : -dragOffset.y) : usable.height))
    readonly property real targetHeight: Math.min(maximumHeight, chromeHeight + body.implicitHeight)
    readonly property int enterDuration: Lithic.design === "material" ? 350 : Lithic.design === "fluent" ? 250 : 200
    readonly property var enterCurve: Lithic.design === "material" ? [0.42,1.67,0.21,0.9,1,1] : Lithic.design === "adwaita" ? [0.25,0.46,0.45,0.94,1,1] : Lithic.design === "fluent" ? [0,0,0,1,1,1] : [0,0,0.2,1,1,1]
    signal dismiss()
    signal keep()
    signal pin()
    signal touched()
    signal dragMoved(real dx, real dy)

    readonly property real preferredWidth: Math.max(400 * s, head.minimumWidth + 26 * s)
    width: usable.width > 26 * s ? Math.min(preferredWidth, usable.width - 26 * s) : preferredWidth
    height: presentedHeight
    x: originX
    y: bottomAnchored ? anchorY - height : anchorY
    opacity: Math.max(0, Math.min(1, arrive))
    scale: 0.96 + arrive * 0.04
    transformOrigin: side === "left" ? Item.TopRight : Item.TopLeft
    transform: Translate {
        x: (root.side === "right" ? -8 : root.side === "left" ? 8 : 0) * root.s * (1 - root.arrive)
        y: (root.side === "below" ? -8 : root.side === "above" ? 8 : 0) * root.s * (1 - root.arrive)
    }
    Behavior on arrive {
        NumberAnimation {
            duration: Lithic.duration(root.closing ? 150 : root.enterDuration)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: root.closing ? (Lithic.design === "fluent" ? [0.7,0,1,0.5,1,1] : [0.3,0,0.8,0.15,1,1]) : root.enterCurve
        }
    }
    RyokuUi.ShellSetting { id: targetSetting; path: "sight.target"; fallback: "" }
    RyokuUi.ShellSetting { id: localSetting; path: "sight.localOnly"; fallback: false }
    SightSession { id: conversation; client: root.client; card: root.card }
    Connections {
        target: targetSetting
        function onValueChanged() { root.target = root.normalizeTarget(String(targetSetting.value || "")); }
    }

    function languageName(code) {
        for (var i = 0; i < I18n.langs.length; ++i)
            if (String(I18n.langs[i].code) === code) return String(I18n.langs[i].name);
        return "";
    }
    function normalizeTarget(code) {
        if (code === "zh-Hans") code = "zh_CN";
        else if (code === "zh-Hant") code = "zh_TW";
        return languageName(code) ? code : "";
    }
    function resetTranslation() {
        if ((conversation.busy && conversation.turnMode === "translate") || (conversation.queued && conversation.queued.mode === "translate"))
            conversation.cancel();
        const turns = Object.assign({}, conversation.turns);
        delete turns.translate;
        conversation.turns = turns;
        const answers = Object.assign({}, conversation.answers);
        delete answers.translate;
        conversation.answers = answers;
        translateAnyway = false;
    }
    function chooseTarget(code) {
        target = code;
        ShellSettings.patch("sight.target", code);
        resetTranslation();
        if (mode === "translate") runMode();
    }
    function chooseSource(code) {
        sourceOverride = code;
        resetTranslation();
        if (mode === "translate") runMode();
    }

    function place() {
        if (placed || usable.width <= 26 * s || usable.height <= 26 * s) return;
        const area = Qt.rect(usable.x + 13 * s, usable.y + 13 * s, Math.max(1, usable.width - 26 * s), Math.max(1, usable.height - 26 * s));
        const at = Placement.lens(plateRect, width, targetHeight, area, 13 * s, maximumHeight);
        originX = at.x;
        anchorY = at.anchor;
        bottomAnchored = at.bottom;
        side = at.side;
        growthRoom = at.room;
        presentedHeight = Math.min(targetHeight, growthRoom);
        placed = true;
        Qt.callLater(function() {
            root.arrive = root.closing ? 0 : 1;
            root.runMode();
            if (root.mode === "ask") ask.focusField(); else root.forceActiveFocus();
        });
    }
    onUsableChanged: if (!placed) Qt.callLater(place)
    onWidthChanged: if (!placed) Qt.callLater(place)
    onTargetHeightChanged: if (!placed || Lithic.reduceMotion) presentedHeight = targetHeight
    onClosingChanged: if (closing) { closePopups(); arrive = 0; }
    Component.onCompleted: Qt.callLater(place)
    FrameAnimation {
        running: root.placed && root.visible && !root.closing && !Lithic.reduceMotion && Math.abs(root.presentedHeight - root.targetHeight) > 0.2
        onTriggered: {
            const next = root.presentedHeight + (root.targetHeight - root.presentedHeight) * (1 - Math.exp(-Math.min(0.1, frameTime) * 24));
            root.presentedHeight = Math.abs(root.targetHeight - next) <= 0.2 ? root.targetHeight : next;
        }
    }
    Connections { target: Lithic; function onReduceMotionChanged() { if (Lithic.reduceMotion) root.presentedHeight = root.targetHeight; } }

    function popupPlacement(anchor, width, height, host) {
        const point = anchor.mapToItem(overlayHost, 0, 0);
        const gap = 8 * s;
        const bounds = Qt.rect(usable.x - dragOffset.x, usable.y - dragOffset.y, usable.width, usable.height);
        const above = point.y + anchor.height + gap + height > bounds.y + bounds.height - gap;
        const x = Placement.clamp(point.x, bounds.x + gap, bounds.x + bounds.width - width - gap);
        const y = Placement.clamp(above ? point.y - height - gap : point.y + anchor.height + gap, bounds.y + gap, bounds.y + bounds.height - height - gap);
        const local = overlayHost.mapToItem(host, x, y);
        return { x: local.x, y: local.y, above: above };
    }
    function closePopups() { more.close(); language.close(); provenance.close(); }
    function preparePopup(which) {
        if (which !== "more") more.close();
        if (which !== "language") language.close();
        if (which !== "provenance") provenance.close();
    }
    function switchMode(next) {
        if (next === mode || closing) return;
        closePopups();
        mode = next;
        modeSwap.restart();
        runMode();
        if (next === "ask") Qt.callLater(ask.focusField);
        else Qt.callLater(function() { root.forceActiveFocus(); });
    }
    function loadOCR() {
        if (ocrDone || ocrBusy) return;
        ocrBusy = true;
        const serial = ++ocrSerial;
        const token = conversation.lease;
        Bus.call("sight.ocr", { id: String(card.id) }, function(ok, result) {
            if (token.closed || serial !== root.ocrSerial) return;
            root.ocrBusy = false;
            root.ocrDone = true;
            if (ok) {
                root.ocrText = String(result.text || "");
                root.codes = result.codes || [];
                root.detectedSource = String(result.language || "");
            }
            if (root.mode === "translate") root.runMode();
        });
    }
    function runMode() {
        if (mode === "text") { loadOCR(); return; }
        if (mode === "ask") return;
        if (conversation.turns[mode] && conversation.turns[mode].length > 0) return;
        if (mode === "translate" && !ocrDone) { loadOCR(); return; }
        if (mode === "translate" && alreadyInLanguage) return;
        conversation.request(mode, "", mode === "translate" || card.kind === "text" ? ocrDisplay : "", languageName(effectiveTarget), languageName(source));
    }
    function send(question) {
        if (conversation.busy || alreadyInLanguage || !question.trim()) return;
        conversation.request(mode, question, mode === "translate" || card.kind === "text" ? ocrDisplay : "", languageName(effectiveTarget), languageName(source));
    }
    function readWithRashin() {
        ++ocrSerial;
        ocrBusy = false;
        ocrDone = true;
        conversation.request("read", "", "", languageName(effectiveTarget), languageName(source));
    }
    function copyText(value) {
        Spawn.run(["sh", "-c", "printf '%s' \"$1\" | wl-copy", "_", String(value || "")]);
        copied = true;
        copiedTimer.restart();
    }
    function copyCurrent() { copyText(mode === "text" ? ocrDisplay : latest); }
    function copyImage() { if (card.kind !== "text") Bus.run("clip-copy image/png " + String(card.image)); }
    function openEditor() { if (card.kind !== "text") Spawn.run(["env", "RYOSHOT_OPEN=" + String(card.image), "ryoku-cmd-ryoshot"]); }
    function settings(provider) {
        closePopups();
        if (!provider) { Bus.run("hub open rashin/sight-enabled"); return; }
        client.call("ui.open", { tab: "settings" }, function(result, error) {
            if (error || result.delivered === false) Spawn.run(["rashin", "open", "--tab", "settings"]);
        });
    }
    function hasSelection() {
        const window = root.Window.window;
        const item = window ? window.activeFocusItem : null;
        return item && typeof item.selectedText === "string" && item.selectedText.length > 0;
    }
    function updateBusy() {
        if (!busy) { looking = false; longWait = false; lookingTimer.stop(); slowTimer.stop(); return; }
        if (latest.length > 0) { looking = false; lookingTimer.stop(); }
        else if (!looking && !lookingTimer.running) lookingTimer.restart();
        if (!longWait && !slowTimer.running) slowTimer.restart();
    }
    onBusyChanged: updateBusy()
    onLatestChanged: updateBusy()
    Timer { id: lookingTimer; interval: 300; onTriggered: root.looking = root.busy && root.latest.length === 0 }
    Timer { id: slowTimer; interval: 10000; onTriggered: root.longWait = root.busy }
    Timer { id: copiedTimer; interval: 1200; onTriggered: root.copied = false }
    Connections {
        target: conversation
        function onAnswerArrived() {
            if (conversation.turnMode === "read") root.ocrText = String(conversation.answers.read || "");
        }
    }
    Keys.priority: Keys.BeforeItem
    Keys.onPressed: event => {
        if (event.key === Qt.Key_Escape) {
            if (popupOpen) closePopups(); else if (conversation.busy) conversation.cancel(); else dismiss();
            event.accepted = true;
        } else if ((event.modifiers & Qt.ControlModifier) && event.key === Qt.Key_C) {
            if (event.modifiers & Qt.ShiftModifier) { copyImage(); event.accepted = true; }
            else if (!hasSelection()) { copyCurrent(); event.accepted = true; }
        } else if (event.key === Qt.Key_Tab && !popupOpen) {
            const modes = ["explain", "text", "translate", "ask"];
            switchMode(modes[(modes.indexOf(mode) + (event.modifiers & Qt.ShiftModifier ? 3 : 1)) % 4]);
            event.accepted = true;
        } else if (!typing && event.modifiers === Qt.NoModifier && event.key >= Qt.Key_1 && event.key <= Qt.Key_4) {
            switchMode(["explain", "text", "translate", "ask"][event.key - Qt.Key_1]);
            event.accepted = true;
        }
    }
    TapHandler { onPressedChanged: if (pressed) { root.touched(); root.forceActiveFocus(); } }
    CutRect { anchors.fill: parent; cut: 8; fill: root.plane; strokeColor: root.stroke; strokeWidth: Lithic.design === "material" ? 0 : 1 }
    Item {
        id: grain
        anchors.fill: parent
        visible: Lithic.design === "fluent"
        readonly property real radius: Lithic.cornerRadius(8 * root.s, width, height, "fluent")
        opacity: 0.55
        Image { x: grain.radius; y: 1; width: Math.max(0, grain.width - grain.radius * 2); height: Math.max(0, grain.radius - 1); source: "../notifications/designs/fluent-noise.png"; fillMode: Image.Tile; smooth: false }
        Image { x: 1; y: grain.radius; width: Math.max(0, grain.width - 2); height: Math.max(0, grain.height - grain.radius * 2); source: "../notifications/designs/fluent-noise.png"; fillMode: Image.Tile; smooth: false }
        Image { x: grain.radius; y: grain.height - grain.radius; width: Math.max(0, grain.width - grain.radius * 2); height: Math.max(0, grain.radius - 1); source: "../notifications/designs/fluent-noise.png"; fillMode: Image.Tile; smooth: false }
        CutRect { anchors.fill: parent; anchors.margins: 1; cut: 7; fill: "transparent"; strokeColor: Lithic.alpha(Lithic.fg, 0.04); strokeWidth: 1 }
    }
    Column {
        id: layout
        x: 13 * root.s; y: 13 * root.s
        width: root.width - 26 * root.s
        spacing: 13 * root.s
        SightPanelHead {
            id: head
            width: parent.width; height: 32 * root.s; s: root.s
            mode: root.mode; copied: root.copied; imageAvailable: root.card.kind !== "text"; kept: root.kept
            onModeRequested: mode => root.switchMode(mode)
            onCopyRequested: { root.closePopups(); root.copyCurrent(); }
            onPinRequested: { root.closePopups(); root.pin(); }
            onMoreRequested: { language.close(); provenance.close(); more.toggle(); }
            onCloseRequested: { root.closePopups(); root.dismiss(); }
            onDragMoved: (dx, dy) => root.dragMoved(dx, dy)
        }
        SightTranslateRow {
            id: language
            width: parent.width; height: 32 * root.s
            visible: root.mode === "translate"
            panel: root; s: root.s; target: root.target; source: root.source; sourceOverride: root.sourceOverride
            onTargetRequested: target => root.chooseTarget(target)
            onSourceRequested: source => root.chooseSource(source)
        }
        Item {
            id: answerSlot
            width: parent.width
            height: Math.max(0, Math.min(body.implicitHeight, root.height - root.chromeHeight))
            visible: root.hasBody
            opacity: 1
            transform: Translate { id: modeTranslate }
            Flickable {
                id: scroll
                anchors.fill: parent
                clip: true; contentWidth: width; contentHeight: body.implicitHeight
                boundsBehavior: Flickable.StopAtBounds
                Column {
                    id: body
                    width: scroll.width - (rail.overflowing ? 13 * root.s : 0)
                    spacing: 13 * root.s
                    Column {
                        width: parent.width
                        visible: root.errorShown
                        spacing: 8 * root.s
                        Label {
                            width: parent.width; typeRole: "detail"; inkRole: "jasper"; wrapMode: Text.Wrap; elide: Text.ElideNone
                            text: conversation.errorCode === "no_vision" ? I18n.tr("No model here can read pictures.")
                                : conversation.errorCode === "local_only" ? I18n.tr("Local models only is on, and no local model can read pictures.")
                                : root.client.state === "off" ? I18n.tr("Rashin is off.")
                                : root.client.state === "not-installed" ? I18n.tr("Rashin is not installed.")
                                : conversation.errorText || root.client.errorText
                        }
                        SightActionButton {
                            s: root.s
                            text: conversation.errorCode === "no_vision" ? I18n.tr("Add a provider")
                                : conversation.errorCode === "local_only" ? I18n.tr("Open Sight settings")
                                : root.client.state === "off" || root.client.state === "not-installed" ? I18n.tr("Open Rashin settings") : I18n.tr("Try again")
                            onClicked: {
                                if (conversation.errorCode === "no_vision") root.settings(true);
                                else if (conversation.errorCode === "local_only" || root.client.state === "off" || root.client.state === "not-installed") root.settings(false);
                                else { if (root.client.state === "error") root.client.connectSocket(); conversation.retry(); }
                            }
                        }
                    }
                    Column {
                        width: parent.width
                        visible: root.alreadyInLanguage
                        spacing: 8 * root.s
                        Label {
                            width: parent.width; typeRole: "detail"; inkRole: "fg3"; wrapMode: Text.Wrap; elide: Text.ElideNone
                            text: I18n.tr("This is already in {language}.").replace("{language}", I18n.tr(root.languageName(root.effectiveTarget)))
                        }
                        SightActionButton {
                            s: root.s; text: I18n.tr("Translate anyway")
                            onClicked: { root.translateAnyway = true; root.runMode(); }
                        }
                    }
                    SightThread { width: parent.width; visible: root.mode !== "text"; turns: conversation.turns[root.mode] || []; s: root.s; keyTarget: root }
                    SightTextView {
                        id: textView
                        width: parent.width; visible: root.mode === "text"
                        text: root.ocrDisplay; codes: root.codes; done: root.ocrDone; busy: conversation.busy; s: root.s; keyTarget: root
                        onEdited: text => root.ocrText = text
                        onCopyRequested: text => root.copyText(text)
                        onOpenRequested: url => Spawn.runApp(["xdg-open", url], { label: "browser" })
                        onReadRequested: root.readWithRashin()
                    }
                    Row {
                        visible: root.looking
                        spacing: 8 * root.s
                        Working { active: root.looking && root.visible; size: 14 * root.s; anchors.verticalCenter: parent.verticalCenter }
                        Label { typeRole: "detail"; inkRole: "fg3"; text: I18n.tr("Looking") }
                    }
                    SightActionButton { visible: root.longWait && conversation.busy; s: root.s; text: I18n.tr("Stop"); onClicked: conversation.cancel() }
                    Disclosure {
                        id: original
                        width: parent.width; visible: root.mode === "translate" && root.ocrDisplay.length > 0
                        text: I18n.tr("Original")
                        implicitHeight: originalBody.implicitHeight
                        contentItem: Column {
                            id: originalBody
                            width: original.width; spacing: 8 * root.s
                            SightActionButton { s: root.s; text: I18n.tr("Original"); glyph: original.open ? "expand_less" : "expand_more"; onClicked: original.open = !original.open }
                            Column {
                                width: parent.width; visible: original.open; spacing: 8 * root.s
                                TextEdit { width: parent.width; text: root.ocrDisplay; textFormat: TextEdit.PlainText; color: Lithic.fg2; font.family: Lithic.type.body.family; font.pixelSize: 14 * root.s; wrapMode: TextEdit.Wrap; readOnly: true; selectByMouse: true; selectionColor: Lithic.sel; selectedTextColor: Lithic.onSel; Keys.forwardTo: [root] }
                                SightActionButton { s: root.s; text: I18n.tr("Copy original"); onClicked: root.copyText(root.ocrDisplay) }
                            }
                        }
                    }
                }
            }
            ScrollRail { id: rail; anchors.right: parent.right; anchors.top: parent.top; anchors.bottom: parent.bottom; flickable: scroll }
        }
        SightAskField {
            id: ask
            width: parent.width; height: 40 * root.s; visible: root.mode !== "text" && !root.alreadyInLanguage
            s: root.s; busy: conversation.busy; keyTarget: root
            placeholder: root.mode === "ask" && !(conversation.turns.ask || []).length ? I18n.tr("Ask about this") : I18n.tr("Ask a follow-up")
            onSubmitted: text => root.send(text)
        }
        SightProvenance { id: provenance; width: parent.width; height: 16 * root.s; visible: root.showProvenance; panel: root; s: root.s; model: conversation.model; engine: conversation.engine; localModel: conversation.localModel; localOnly: localSetting.value === true; copied: root.copied }
    }
    SightMore { id: more; parent: head; panel: root; anchorItem: head.moreAnchor; s: root.s }
    ParallelAnimation {
        id: modeSwap
        NumberAnimation { target: answerSlot; property: "opacity"; from: 0; to: 1; duration: Lithic.duration(120) }
        NumberAnimation { target: modeTranslate; property: "y"; from: 4 * root.s; to: 0; duration: Lithic.duration(120); easing.type: Easing.OutCubic }
    }
}

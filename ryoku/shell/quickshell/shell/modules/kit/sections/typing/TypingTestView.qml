pragma ComponentBehavior: Bound
import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../../ask/look"
Item {
    id: test
    required property real s
    required property var store
    required property var languages
    property var soundPacks: null
    property bool active: false
    property bool personalBest: false
    property string lastPressed: ""
    readonly property bool finished: engine.isFinished
    readonly property alias testEngine: engine
    readonly property string targetText: engine.targetText
    signal requestFocus()
    function focusInput() { if (active) input.forceActiveFocus(); }
    function restart(reuseSeed) { engine.reset(reuseSeed); personalBest = false; input.text = ""; Qt.callLater(focusInput); }
    function cancel() { if (engine.isRunning) restart(false); input.focus = false; }
    function finishZen() { if (engine.mode === "zen" && engine.isRunning) engine.finish(); }
    TypingTestEngine {
        id: engine
        mode: test.store.mode; zenGuided: test.store.zenGuided; timeLimitSeconds: test.store.timeLimit; wordLimit: test.store.wordLimit
        punctuation: test.store.punctuation; numbers: test.store.numbers; finishOnLastWord: test.store.finishOnLastWord; languagePack: test.languages.currentPack
        onStarted: test.store.registerStart()
        onCharTyped: (character, correct) => { test.lastPressed = character; flash.restart(); if (audio.item) { audio.item.playKey(); if (!correct) audio.item.playError(); } }
        onFinished: { test.personalBest = test.store.record(resultPayload()); Qt.callLater(test.focusInput); }
        onResetRequested: input.text = ""
    }
    Loader { id: audio; active: test.active && test.store.soundsEnabled && test.soundPacks !== null; sourceComponent: Component { TypingSounds { store: test.store; packs: test.soundPacks } } }
    Timer { interval: 50; repeat: true; running: test.active && engine.isRunning; onTriggered: engine.tick() }
    Timer { id: flash; interval: 90; onTriggered: test.lastPressed = "" }
    TextInput {
        id: input
        objectName: "typing-input"
        width: 2; height: 2; opacity: 0
        enabled: test.active
        readOnly: engine.isFinished
        inputMethodHints: Qt.ImhNoPredictiveText | Qt.ImhNoAutoUppercase
        onTextChanged: engine.updateInput(text)
        Keys.onPressed: event => {
            if (event.key === Qt.Key_Tab && !(event.modifiers & Qt.ControlModifier)) { if (!engine.isRunning || test.store.quickRestart) test.restart(false); event.accepted = true; }
            else if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter) && engine.isFinished) { test.restart(false); event.accepted = true; }
            else if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter) && (event.modifiers & Qt.ControlModifier)) { test.finishZen(); event.accepted = true; }
            else if (engine.isFinished && event.key !== Qt.Key_Escape && !(event.modifiers & (Qt.ControlModifier | Qt.AltModifier | Qt.MetaModifier))) event.accepted = true;
        }
    }
    MouseArea { anchors.fill: parent; onClicked: test.requestFocus() }
    Item {
        id: stage
        anchors.centerIn: parent
        width: Math.min(parent.width - 64 * test.s, 860 * test.s)
        height: parent.height - 56 * test.s
        visible: !engine.isFinished
        TypingTestToolbar { id: toolbar; anchors.top: parent.top; width: parent.width; s: test.s; store: test.store; locked: engine.isRunning; onRefocus: test.requestFocus() }
        Row {
            id: metrics
            y: toolbar.height + 36 * test.s
            spacing: 42 * test.s
            Repeater {
                model: [
                    {label: I18n.tr("Words a minute"), value: test.store.showLiveWpm || !engine.isRunning ? String(Math.round(engine.wpm)) : ""},
                    {label: I18n.tr("Accuracy"), value: test.store.showLiveAccuracy || !engine.isRunning ? Math.round(engine.accuracy) + "%" : ""},
                    {label: engine.mode === "time" ? I18n.tr("Left") : I18n.tr("Words"), value: engine.mode === "time" ? Math.max(0, Math.ceil(engine.timeLimitSeconds - engine.elapsedSeconds)) + "s" : engine.completedWords() + "/" + (engine.mode === "words" ? engine.wordLimit : "∞")}
                ]
                delegate: Column { required property var modelData; spacing: 5 * test.s; Text { text: modelData.label; color: Lithic.fg3; font.family: Look.sans; font.pixelSize: 12 * test.s } Text { text: modelData.value; color: Lithic.fg; font.family: Look.figures; font.pixelSize: 34 * test.s } }
            }
        }
        TypingWordViewport { id: viewport; y: metrics.y + metrics.height + 34 * test.s; width: parent.width; height: Math.min(implicitHeight, Math.max(rowHeight, Math.floor(stage.height * 0.30 / rowHeight) * rowHeight)); s: test.s; engine: engine; store: test.store; visible: engine.hasTarget && engine.state !== "loading" }
        Text { y: viewport.y; width: parent.width; visible: !engine.hasTarget || engine.state === "loading"; text: engine.state === "loading" ? (test.languages.errorText || I18n.tr("Loading words")) : engine.inputText || I18n.tr("Free typing. Begin anywhere."); color: Lithic.fg2; font.family: Look.mono; font.pixelSize: test.store.fontSize * test.s; wrapMode: Text.Wrap }
        Column {
            id: recent
            y: viewport.y + viewport.height + 26 * test.s
            width: parent.width
            spacing: 10 * test.s
            Text { text: engine.isRunning ? I18n.tr("Test in progress") : I18n.tr("Type to begin"); color: Lithic.fg3; font.family: Look.sans; font.pixelSize: 12 * test.s }
            TypingGraph { width: parent.width; height: 66 * test.s; s: test.s; values: test.store.results.slice(0, 10).reverse().map(r => r.wpm) }
        }
        TypingKeyboardPreview { anchors.top: recent.bottom; anchors.topMargin: 10 * test.s; width: parent.width; height: Math.max(0, stage.height - y); s: test.s; store: test.store; nextChar: engine.nextExpectedChar; pressedChar: test.lastPressed; visible: test.store.keyboardEnabled && height > 90 * test.s; onRequestInputFocus: test.requestFocus() }
    }
    TypingResults { anchors.fill: parent; visible: engine.isFinished; s: test.s; engine: engine; personalBest: test.personalBest; onRestart: reuseSeed => test.restart(reuseSeed) }
    onActiveChanged: if (active) Qt.callLater(focusInput)
}

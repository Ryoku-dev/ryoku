import QtQuick

QtObject {
    id: store

    required property var kit
    property bool loading: true

    property string language: "english_1k"
    property string wordlistSize: "standard"
    property string mode: "time"
    property bool zenGuided: false
    property int timeLimit: 30
    property int wordLimit: 50
    property bool punctuation: false
    property bool numbers: false
    property bool showLiveWpm: false
    property bool showLiveAccuracy: false
    property bool smoothCaret: true
    property int fontSize: 26
    property int visibleLines: 3
    property string caretStyle: "line"
    property bool highlightCurrentWord: false
    property bool blindMode: false
    property bool quickRestart: false
    property bool finishOnLastWord: true
    property bool keyboardEnabled: true
    property string keyboardLayout: "qwerty"
    property bool highlightNextKey: true
    property bool fingerGuide: false
    property var fingerAssignments: []
    property bool soundsEnabled: false
    property string soundTheme: "click1"
    property string errorTheme: "error1"
    property int soundVolume: 55
    property bool errorSound: true
    property bool historyEnabled: true
    property int maxEntries: 100

    property var results: []
    property var personalBests: []
    property int testsStarted: 0
    property int testsCompleted: 0
    property real secondsTyping: 0
    property var activity: []

    function value(name, fallback) {
        return store.kit ? store.kit.get("typing." + name, fallback) : fallback;
    }

    function put(name, value) {
        if (!store.loading && store.kit)
            store.kit.set("typing." + name, value);
    }

    function load() {
        store.loading = true;
        if (!store.kit || store.kit.ready === false)
            return;
        store.language = String(value("language", "english_1k"));
        store.wordlistSize = String(value("wordlistSize", "standard"));
        store.mode = String(value("mode", "time"));
        store.zenGuided = Boolean(value("zenGuided", false));
        store.timeLimit = Number(value("time", 30));
        store.wordLimit = Number(value("words", 50));
        store.punctuation = Boolean(value("punctuation", false));
        store.numbers = Boolean(value("numbers", false));
        store.showLiveWpm = Boolean(value("showLiveWpm", false));
        store.showLiveAccuracy = Boolean(value("showLiveAccuracy", false));
        store.smoothCaret = Boolean(value("smoothCaret", true));
        store.fontSize = Number(value("fontSize", 26));
        store.visibleLines = Number(value("visibleLines", 3));
        store.caretStyle = String(value("caretStyle", "line"));
        store.highlightCurrentWord = Boolean(value("highlightCurrentWord", false));
        store.blindMode = Boolean(value("blindMode", false));
        store.quickRestart = Boolean(value("quickRestart", false));
        store.finishOnLastWord = Boolean(value("finishOnLastWord", true));
        store.keyboardEnabled = Boolean(value("keyboard.enabled", true));
        store.keyboardLayout = String(value("keyboard.layout", "qwerty"));
        store.highlightNextKey = Boolean(value("keyboard.highlightNextKey", true));
        store.fingerGuide = Boolean(value("keyboard.fingerGuide", false));
        store.fingerAssignments = Array.from(value("keyboard.fingerAssignments", []));
        store.soundsEnabled = Boolean(value("sounds.enabled", false));
        store.soundTheme = String(value("sounds.theme", "click1"));
        store.errorTheme = String(value("sounds.errorTheme", "error1"));
        store.soundVolume = Number(value("sounds.volume", 55));
        store.errorSound = Boolean(value("sounds.errorSound", true));
        store.historyEnabled = Boolean(value("history.enabled", true));
        store.maxEntries = Number(value("history.maxEntries", 100));
        store.results = Array.from(value("history.results", []));
        store.personalBests = Array.from(value("history.personalBests", []));
        store.testsStarted = Number(value("history.testsStarted", 0));
        store.testsCompleted = Number(value("history.testsCompleted", 0));
        store.secondsTyping = Number(value("history.secondsTyping", 0));
        store.activity = Array.from(value("history.activity", []));
        store.loading = false;
    }

    function dayKey(timestamp) {
        return Qt.formatDate(new Date(Number(timestamp)), "yyyy-MM-dd");
    }

    function keyOf(result) {
        return [result.mode, result.modeValue, result.language,
            result.punctuation ? "p" : "-", result.numbers ? "n" : "-"].join(":");
    }

    function bestFor(result) {
        const key = keyOf(result);
        return Array.from(store.personalBests).find(best => best.key === key) ?? null;
    }

    function bestOf(modeName, modeValue) {
        let best = null;
        for (const entry of Array.from(store.personalBests)) {
            if (entry.mode !== modeName || Number(entry.modeValue) !== Number(modeValue))
                continue;
            if (!best || Number(entry.wpm || 0) > Number(best.wpm || 0))
                best = entry;
        }
        return best;
    }

    function registerStart() {
        if (store.historyEnabled)
            store.testsStarted++;
    }

    function record(result) {
        if (!store.historyEnabled || Number(result.duration || 0) < 1)
            return false;
        store.testsCompleted++;
        store.secondsTyping += Number(result.duration || 0);
        const key = store.dayKey(result.timestamp);
        const cutoff = Date.now() - 372 * 86400000;
        const nextActivity = Array.from(store.activity)
            .map(entry => ({ d: String(entry.d), n: Number(entry.n || 0) }))
            .filter(entry => new Date(entry.d + "T00:00:00").getTime() >= cutoff);
        const existing = nextActivity.find(entry => entry.d === key);
        if (existing)
            existing.n = Number(existing.n || 0) + 1;
        else
            nextActivity.push({ d: key, n: 1 });
        store.activity = nextActivity;
        if (result.mode === "zen")
            return false;
        const oldBest = store.bestFor(result);
        const improved = !oldBest || Number(result.wpm) > Number(oldBest.wpm);
        store.results = [result].concat(Array.from(store.results))
            .slice(0, Math.max(1, Math.min(500, store.maxEntries)));
        if (improved) {
            const resultKey = store.keyOf(result);
            store.personalBests = Array.from(store.personalBests)
                .filter(best => best.key !== resultKey)
                .concat([Object.assign({ key: resultKey }, result)]);
        }
        return improved;
    }

    function clearHistory() {
        store.results = [];
        store.personalBests = [];
        store.testsStarted = 0;
        store.testsCompleted = 0;
        store.secondsTyping = 0;
        store.activity = [];
    }

    onLanguageChanged: put("language", language)
    onWordlistSizeChanged: put("wordlistSize", wordlistSize)
    onModeChanged: put("mode", mode)
    onZenGuidedChanged: put("zenGuided", zenGuided)
    onTimeLimitChanged: put("time", timeLimit)
    onWordLimitChanged: put("words", wordLimit)
    onPunctuationChanged: put("punctuation", punctuation)
    onNumbersChanged: put("numbers", numbers)
    onShowLiveWpmChanged: put("showLiveWpm", showLiveWpm)
    onShowLiveAccuracyChanged: put("showLiveAccuracy", showLiveAccuracy)
    onSmoothCaretChanged: put("smoothCaret", smoothCaret)
    onFontSizeChanged: put("fontSize", fontSize)
    onVisibleLinesChanged: put("visibleLines", visibleLines)
    onCaretStyleChanged: put("caretStyle", caretStyle)
    onHighlightCurrentWordChanged: put("highlightCurrentWord", highlightCurrentWord)
    onBlindModeChanged: put("blindMode", blindMode)
    onQuickRestartChanged: put("quickRestart", quickRestart)
    onFinishOnLastWordChanged: put("finishOnLastWord", finishOnLastWord)
    onKeyboardEnabledChanged: put("keyboard.enabled", keyboardEnabled)
    onKeyboardLayoutChanged: put("keyboard.layout", keyboardLayout)
    onHighlightNextKeyChanged: put("keyboard.highlightNextKey", highlightNextKey)
    onFingerGuideChanged: put("keyboard.fingerGuide", fingerGuide)
    onFingerAssignmentsChanged: put("keyboard.fingerAssignments", fingerAssignments)
    onSoundsEnabledChanged: put("sounds.enabled", soundsEnabled)
    onSoundThemeChanged: put("sounds.theme", soundTheme)
    onErrorThemeChanged: put("sounds.errorTheme", errorTheme)
    onSoundVolumeChanged: put("sounds.volume", soundVolume)
    onErrorSoundChanged: put("sounds.errorSound", errorSound)
    onHistoryEnabledChanged: put("history.enabled", historyEnabled)
    onMaxEntriesChanged: put("history.maxEntries", maxEntries)
    onResultsChanged: put("history.results", results)
    onPersonalBestsChanged: put("history.personalBests", personalBests)
    onTestsStartedChanged: put("history.testsStarted", testsStarted)
    onTestsCompletedChanged: put("history.testsCompleted", testsCompleted)
    onSecondsTypingChanged: put("history.secondsTyping", secondsTyping)
    onActivityChanged: put("history.activity", activity)

    property Connections memoryWatch: Connections {
        target: store.kit
        function onReadyChanged() { if (store.kit.ready) store.load(); }
    }
    onKitChanged: if (kit) Qt.callLater(load)
    Component.onCompleted: load()
}

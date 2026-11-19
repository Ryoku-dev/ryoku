pragma ComponentBehavior: Bound

import QtQuick

/**
 * Ephemeral, deterministic session state for one Typing Test panel instance.
 * It receives committed text from a real TextInput; no key events, I/O or
 * persistence occur in the hot input path.
 */
QtObject {
    id: root

    property string state: "loading" // loading, ready, running, finished
    property string mode: "time" // time, words, zen
    /** Zen against generated words instead of free typing. */
    property bool zenGuided: false
    property int timeLimitSeconds: 30
    property int wordLimit: 50
    property bool punctuation: false
    property bool numbers: false
    property bool finishOnLastWord: true
    property var languagePack: null

    property int seed: 1
    property int _randomState: 1
    property var targetWords: []
    property string targetText: ""
    // The target as code points, kept alongside the string so the per-key path
    // never re-splits a 1500-character target on every insertion.
    property var targetChars: []
    property string inputText: ""
    property string previousInputText: ""
    property int currentWordIndex: 0
    property int correctInputEvents: 0
    property int incorrectInputEvents: 0
    property real startedAt: 0
    property real finishedAt: 0
    property real elapsedSeconds: 0
    property var samples: []
    property int _lastSampleSecond: -1
    property int _lastSampleChars: 0

    readonly property bool isReady: root.state === "ready"
    readonly property bool isRunning: root.state === "running"
    readonly property bool isFinished: root.state === "finished"
    /**
     * Both of these are also the backing functions of the properties below,
     * because a handler for one of their inputs runs before the properties
     * have re-evaluated: reading `hasTarget` from `onZenGuidedChanged` gets
     * the value from before the change, and the reset it triggers then builds
     * the wrong target. Anything reacting to a parameter change calls the
     * function; everything else reads the property.
     */
    function targetRequired() {
        return root.mode !== "zen" || root.zenGuided;
    }
    /**
     * Whether the target has no end: the generator keeps topping it up as the
     * cursor approaches the tail, and nothing after the typed text counts as
     * missed because it was never asked for.
     */
    function targetIsEndless() {
        return root.mode === "time" || (root.mode === "zen" && root.zenGuided);
    }

    readonly property bool hasTarget: root.targetRequired()
    readonly property bool endlessTarget: root.targetIsEndless()
    readonly property int rawCharacterCount: root.codePointCount(root.inputText)
    readonly property int correctCharacterCount: root.hasTarget
        ? root.countCharacterBreakdown().correct
        : root.rawCharacterCount
    readonly property real rawWpm: root.calculateWpm(root.rawCharacterCount, root.elapsedSeconds)
    readonly property real wpm: root.calculateWpm(root.correctCharacterCount, root.elapsedSeconds)
    readonly property real accuracy: {
        const total = root.correctInputEvents + root.incorrectInputEvents;
        return total > 0 ? root.correctInputEvents / total * 100 : 100;
    }
    readonly property var characterBreakdown: root.countCharacterBreakdown()
    readonly property string currentWordInput: {
        const words = root.inputText.split(" ");
        return words.length > 0 ? words[words.length - 1] : "";
    }
    /** The glyph the keyboard preview should point at next. */
    readonly property string nextExpectedChar: {
        if (!root.hasTarget)
            return "";
        const index = root.codePointCount(root.inputText);
        return index < root.targetChars.length ? root.targetChars[index] : "";
    }
    /** How far a finite test has come, 0–1. Time mode uses the clock instead. */
    readonly property real progress: {
        if (root.mode === "time")
            return root.timeLimitSeconds > 0 ? Math.min(1, root.elapsedSeconds / root.timeLimitSeconds) : 0;
        if (root.mode === "words")
            return root.wordLimit > 0 ? Math.min(1, root.completedWords() / root.wordLimit) : 0;
        return 0;
    }

    signal resetRequested
    signal finished
    /** The first committed keystroke of a run. */
    signal started
    /**
     * One committed code point, carried with the signal rather than read back
     * from `inputText`: the text property is only assigned once the whole edit
     * has been accounted for, so a listener reading it here would be a
     * character behind.
     */
    signal charTyped(string character, bool correct)
    signal charDeleted

    onLanguagePackChanged: {
        if (languagePack?.words?.length > 0)
            root.reset(false);
        else if (root.targetRequired())
            root.state = "loading";
    }

    // Every structural parameter re-rolls the test the moment it changes, no
    // matter who changed it: the toolbar, the panel's own settings page, or
    // the Settings window with the panel already open. A running test is left
    // alone so a result stays the test the user actually took.
    function reapply() {
        if (!root.isRunning)
            root.reset(false);
    }

    onModeChanged: root.reapply()
    // Only zen reads it, so flipping it from another mode must not re-roll a
    // test the change cannot affect.
    onZenGuidedChanged: {
        if (root.mode === "zen")
            root.reapply();
    }
    onTimeLimitSecondsChanged: root.reapply()
    onWordLimitChanged: root.reapply()
    onPunctuationChanged: root.reapply()
    onNumbersChanged: root.reapply()

    function now() {
        return Date.now();
    }

    function codePoints(text) {
        return Array.from(String(text ?? ""));
    }

    function codePointCount(text) {
        return root.codePoints(text).length;
    }

    function normalizeSeed(value) {
        const unsigned = Number(value) >>> 0;
        return unsigned === 0 ? 1 : unsigned;
    }

    function nextRandom() {
        let x = root._randomState >>> 0;
        x ^= x << 13;
        x ^= x >>> 17;
        x ^= x << 5;
        root._randomState = x >>> 0;
        return root._randomState / 4294967296;
    }

    function randomIndex(length) {
        return Math.max(0, Math.min(length - 1, Math.floor(root.nextRandom() * length)));
    }

    function decorateWord(word, index) {
        let output = word;
        if (root.numbers && index > 0 && index % 13 === 0)
            output = String(10 + root.randomIndex(990));
        if (root.punctuation && index > 0 && index % 9 === 0) {
            const marks = [",", ".", ";", "!", "?"];
            output += marks[root.randomIndex(marks.length)];
        }
        return output;
    }

    function makeTarget(wordCount, startIndex) {
        const source = Array.from(root.languagePack?.words ?? []);
        if (source.length === 0)
            return [];
        const generated = [];
        let previous = "";
        for (let index = 0; index < wordCount; index++) {
            let word = source[root.randomIndex(source.length)];
            if (source.length > 1 && word === previous)
                word = source[(root.randomIndex(source.length - 1) + 1) % source.length];
            previous = word;
            generated.push(root.decorateWord(word, startIndex + index));
        }
        return generated;
    }

    function applyTarget(words) {
        root.targetWords = words;
        root.targetText = words.join(" ");
        root.targetChars = root.codePoints(root.targetText);
    }

    function reset(reuseSeed) {
        if (root.targetRequired() && !(root.languagePack?.words?.length > 0)) {
            root.state = "loading";
            return;
        }
        root.seed = reuseSeed ? root.seed : root.normalizeSeed(Math.floor(root.now()));
        root._randomState = root.seed;
        root.applyTarget(root.targetRequired()
            ? root.makeTarget(root.mode === "words" ? root.wordLimit : 220, 0) : []);
        root.inputText = "";
        root.previousInputText = "";
        root.currentWordIndex = 0;
        root.correctInputEvents = 0;
        root.incorrectInputEvents = 0;
        root.startedAt = 0;
        root.finishedAt = 0;
        root.elapsedSeconds = 0;
        root.samples = [];
        root._lastSampleSecond = -1;
        root._lastSampleChars = 0;
        root.state = "ready";
        root.resetRequested();
    }

    function start() {
        if (!root.isReady)
            return;
        root.startedAt = root.now();
        // Sampling begins at the one-second mark. A sample taken a few
        // milliseconds in reports a meaningless four-digit speed, and one
        // outlier is enough to flatten an entire result graph.
        root._lastSampleSecond = 0;
        root._lastSampleChars = 0;
        root.state = "running";
        root.started();
    }

    function updateInput(value) {
        if (root.isFinished)
            return;
        const next = String(value ?? "");
        if (next === root.previousInputText)
            return;
        if (root.isReady && root.codePointCount(next) > 0)
            root.start();
        if (!root.isReady && !root.isRunning)
            return;

        const before = root.codePoints(root.previousInputText);
        const after = root.codePoints(next);
        let prefix = 0;
        while (prefix < before.length && prefix < after.length && before[prefix] === after[prefix])
            prefix++;
        let suffix = 0;
        while (suffix < before.length - prefix && suffix < after.length - prefix
                && before[before.length - suffix - 1] === after[after.length - suffix - 1])
            suffix++;
        const inserted = after.slice(prefix, after.length - suffix);
        for (let offset = 0; offset < inserted.length; offset++) {
            const correct = !root.hasTarget || inserted[offset] === root.targetChars[prefix + offset];
            if (correct)
                root.correctInputEvents++;
            else
                root.incorrectInputEvents++;
            root.charTyped(inserted[offset], correct);
        }
        if (inserted.length === 0 && after.length < before.length)
            root.charDeleted();

        root.inputText = next;
        root.previousInputText = next;
        root.currentWordIndex = root.hasTarget ? Math.min(Math.max(0, root.targetWords.length - 1),
            Math.max(0, next.split(" ").length - 1)) : 0;
        root.tick();
        if (root.isFinished)
            return;
        root.extendTargetIfNeeded();
        if (root.mode === "words" && root.reachedWordLimit(next))
            root.finish();
    }

    /**
     * Monkeytype ends a words test the moment the final word is complete, so a
     * trailing space is never part of the score. A word committed with a space
     * still ends the test even when it was mistyped.
     */
    function reachedWordLimit(text) {
        if (root.completedWords() >= root.wordLimit)
            return true;
        if (!root.finishOnLastWord)
            return false;
        const words = String(text ?? "").split(" ");
        if (words.length !== root.wordLimit || root.targetWords.length < root.wordLimit)
            return false;
        return words[words.length - 1] === root.targetWords[root.wordLimit - 1];
    }

    function extendTargetIfNeeded() {
        if (!root.targetIsEndless() || root.currentWordIndex < root.targetWords.length - 30)
            return;
        root.applyTarget(root.targetWords.concat(root.makeTarget(100, root.targetWords.length)));
    }

    function completedWords() {
        if (!root.inputText)
            return 0;
        return Math.max(0, root.inputText.split(" ").length - 1);
    }

    function tick() {
        if (!root.isRunning)
            return;
        root.elapsedSeconds = Math.max(0, (root.now() - root.startedAt) / 1000);
        if (root.mode === "time" && root.elapsedSeconds >= root.timeLimitSeconds) {
            root.elapsedSeconds = root.timeLimitSeconds;
            root.finish();
            return;
        }
        const sampleSecond = Math.floor(root.elapsedSeconds);
        if (sampleSecond > root._lastSampleSecond) {
            const chars = root.rawCharacterCount;
            const seconds = Math.max(1, sampleSecond - root._lastSampleSecond);
            root._lastSampleSecond = sampleSecond;
            root.samples = root.samples.concat([{
                t: sampleSecond,
                wpm: root.wpm,
                raw: root.rawWpm,
                // Speed within this second alone is what consistency is measured
                // from, and what a result graph should plot.
                burst: (chars - root._lastSampleChars) / 5 / (seconds / 60),
                errors: root.incorrectInputEvents
            }]);
            root._lastSampleChars = chars;
        }
    }

    function finish() {
        if (root.isFinished)
            return;
        if (root.startedAt > 0) {
            root.finishedAt = root.now();
            if (root.mode !== "time")
                root.elapsedSeconds = Math.max(0, (root.finishedAt - root.startedAt) / 1000);
        }
        root.state = "finished";
        root.finished();
    }

    function calculateWpm(characters, seconds) {
        return seconds > 0 ? characters / 5 / (seconds / 60) : 0;
    }

    /**
     * How even the pace was, as a percentage: the coefficient of variation of
     * the per-second speed, inverted. A test too short to have two samples has
     * nothing to compare and reports 0.
     */
    function consistency() {
        const bursts = Array.from(root.samples ?? []).map(sample => sample.burst ?? 0);
        if (bursts.length < 2)
            return 0;
        const mean = bursts.reduce((total, value) => total + value, 0) / bursts.length;
        if (mean <= 0)
            return 0;
        const variance = bursts.reduce((total, value) => total + Math.pow(value - mean, 2), 0) / bursts.length;
        return Math.max(0, Math.min(100, 100 * (1 - Math.sqrt(variance) / mean)));
    }

    /** Compact, aggregate-only record for the local history. */
    function resultPayload() {
        const breakdown = root.characterBreakdown;
        return {
            timestamp: Math.round(root.finishedAt > 0 ? root.finishedAt : root.now()),
            mode: root.mode,
            modeValue: root.mode === "time" ? root.timeLimitSeconds : (root.mode === "words" ? root.wordLimit : 0),
            language: String(root.languagePack?.id ?? ""),
            punctuation: root.punctuation,
            numbers: root.numbers,
            wpm: Math.round(root.wpm * 10) / 10,
            raw: Math.round(root.rawWpm * 10) / 10,
            accuracy: Math.round(root.accuracy * 10) / 10,
            consistency: Math.round(root.consistency() * 10) / 10,
            duration: Math.round(root.elapsedSeconds * 10) / 10,
            chars: [breakdown.correct, breakdown.incorrect, breakdown.extra, breakdown.missed]
        };
    }

    function countCharacterBreakdown(): var {
        if (!root.hasTarget) {
            const raw = root.rawCharacterCount;
            return { correct: raw, incorrect: 0, extra: 0, missed: 0 };
        }

        const inputWords = root.inputText.split(" ");
        const targets = root.targetWords ?? [];
        let correct = 0;
        let incorrect = 0;
        let extra = 0;
        let missed = 0;

        const evaluatedWordCount = inputWords.length;
        for (let i = 0; i < evaluatedWordCount; i++) {
            if (i >= targets.length && !root.endlessTarget)
                break;
            const typed = root.codePoints(inputWords[i]);
            const target = i < targets.length ? root.codePoints(targets[i]) : [];
            const isLast = (i === evaluatedWordCount - 1);
            const hasSpace = (i < evaluatedWordCount - 1) || (isLast && root.inputText.endsWith(" "));

            if (isLast && typed.length === 0 && root.inputText.endsWith(" "))
                continue;

            const minLen = Math.min(typed.length, target.length);
            for (let c = 0; c < minLen; c++) {
                if (typed[c] === target[c])
                    correct++;
                else
                    incorrect++;
            }

            if (typed.length > target.length)
                extra += (typed.length - target.length);
            else if (typed.length < target.length)
                missed += (target.length - typed.length);

            if (hasSpace && i < targets.length) {
                if (inputWords[i] === targets[i])
                    correct++;
                else
                    incorrect++;
            }
        }

        if (root.mode === "words") {
            for (let i = evaluatedWordCount; i < Math.min(targets.length, root.wordLimit); i++) {
                missed += root.codePoints(targets[i]).length;
                if (i < Math.min(targets.length, root.wordLimit) - 1)
                    missed++;
            }
        }

        return {
            correct: correct,
            incorrect: incorrect,
            extra: extra,
            missed: missed
        };
    }
}

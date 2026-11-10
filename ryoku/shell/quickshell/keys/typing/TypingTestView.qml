pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "."

Item {
    id: test
    objectName: "typing-test"

    required property var store
    required property var languages
    required property var soundPacks
    property bool personalBest: false
    property string lastPressed: ""
    readonly property string targetText: testEngine.targetText
    signal requestSection(string section)

    function focusInput() {
        if (!testEngine.isFinished)
            input.forceActiveFocus();
    }

    function restart(reuseSeed) {
        testEngine.reset(reuseSeed);
        personalBest = false;
        input.text = "";
        Qt.callLater(focusInput);
    }

    function finishZen() {
        if (testEngine.mode === "zen" && testEngine.isRunning)
            testEngine.finish();
    }

    function cycleLanguage() {
        const available = Array.from(languages.catalogue).filter(entry =>
            !String(entry.id).endsWith("_extended"));
        if (available.length === 0)
            return;
        const current = available.findIndex(entry => entry.id === store.language);
        store.language = available[(current + 1 + available.length) % available.length].id;
    }

    TypingTestEngine {
        id: testEngine
        mode: test.store.mode
        zenGuided: test.store.zenGuided
        timeLimitSeconds: test.store.timeLimit
        wordLimit: test.store.wordLimit
        punctuation: test.store.punctuation
        numbers: test.store.numbers
        finishOnLastWord: test.store.finishOnLastWord
        languagePack: test.languages.currentPack
        onStarted: test.store.registerStart()
        onCharTyped: (character, correct) => {
            test.lastPressed = character;
            pressedFlash.restart();
            sounds.playKey();
            if (!correct)
                sounds.playError();
        }
        onFinished: {
            test.personalBest = test.store.record(resultPayload());
            input.focus = false;
        }
        onResetRequested: {
            if (input.text !== "") input.text = "";
        }
    }

    TypingSounds {
        id: sounds
        store: test.store
        packs: test.soundPacks
    }

    Timer {
        id: clock
        interval: 50
        repeat: true
        running: testEngine.isRunning
        onTriggered: testEngine.tick()
    }

    Timer {
        id: pressedFlash
        interval: 90
        onTriggered: test.lastPressed = ""
    }

    TextInput {
        id: input
        objectName: "typing-input"
        width: 2
        height: 2
        opacity: 0
        focus: true
        inputMethodHints: Qt.ImhNoPredictiveText | Qt.ImhNoAutoUppercase
        onTextChanged: testEngine.updateInput(text)
        Keys.onPressed: event => {
            if (event.key === Qt.Key_Tab && test.store.quickRestart && !event.isAutoRepeat) {
                test.restart(false);
                event.accepted = true;
            } else if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter)
                    && testEngine.isFinished && !event.isAutoRepeat) {
                test.restart(false);
                event.accepted = true;
            } else if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter)
                    && testEngine.mode === "zen" && testEngine.isRunning
                    && (event.modifiers & Qt.ControlModifier)) {
                test.finishZen();
                event.accepted = true;
            } else if (event.key === Qt.Key_Escape) {
                event.accepted = false;
            }
        }
    }

    ColumnLayout {
        id: testStage
        anchors.fill: parent
        anchors.margins: Tokens.s2
        spacing: Tokens.s3
        opacity: testEngine.isFinished ? 0 : 1
        visible: opacity > 0
        enabled: !testEngine.isFinished
        Behavior on opacity {
            enabled: !Tokens.reduceMotion
            NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
        }
        RowLayout {
            Layout.fillWidth: true
            spacing: Tokens.s3
            TypingTestToolbar {
                Layout.fillWidth: true
                store: test.store
                locked: testEngine.isRunning
                onRefocus: test.focusInput()
            }
            Btn {
                text: test.languages.currentPack
                    ? String(test.languages.currentPack.name || test.languages.currentLanguageId)
                    : I18n.tr("Loading language")
                compact: true
                armed: !testEngine.isRunning && test.languages.ready
                onAct: { test.cycleLanguage(); test.focusInput(); }
            }
        }

        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: 1
            color: Tokens.lineSoft
            Rectangle {
                width: parent.width * testEngine.progress
                height: parent.height
                color: Tokens.ink
                Behavior on width {
                    enabled: !Tokens.reduceMotion
                    NumberAnimation { duration: Tokens.snap; easing.type: Easing.OutCubic }
                }
            }
        }

        RowLayout {
            Layout.fillWidth: true
            spacing: Tokens.s3
            Text {
                text: testEngine.mode === "time" ? Math.max(0, Math.ceil(testEngine.timeLimitSeconds - testEngine.elapsedSeconds)) + "s"
                    : (testEngine.mode === "words" ? testEngine.completedWords() + "/" + testEngine.wordLimit
                    : Qt.formatTime(new Date(testEngine.elapsedSeconds * 1000), "mm:ss"))
                color: Tokens.ink
                font.family: Tokens.mono
                font.pixelSize: Tokens.fValue
                font.weight: Font.DemiBold
            }
            Item { Layout.fillWidth: true }
            Text {
                opacity: test.store.showLiveWpm && testEngine.isRunning ? 1 : 0
                visible: opacity > 0
                text: Math.round(testEngine.wpm) + " wpm"
                color: Tokens.inkDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fSmall
                Behavior on opacity {
                    enabled: !Tokens.reduceMotion
                    NumberAnimation { duration: Tokens.snap; easing.type: Easing.OutCubic }
                }
            }
            Text {
                opacity: test.store.showLiveAccuracy && testEngine.isRunning && testEngine.hasTarget ? 1 : 0
                visible: opacity > 0
                text: Math.round(testEngine.accuracy) + "%"
                color: Tokens.inkDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fSmall
                Behavior on opacity {
                    enabled: !Tokens.reduceMotion
                    NumberAnimation { duration: Tokens.snap; easing.type: Easing.OutCubic }
                }
            }
            Text {
                text: testEngine.isRunning
                    ? (testEngine.mode === "zen" ? I18n.tr("Ctrl+Enter to finish") : I18n.tr("Test in progress"))
                    : I18n.tr("Type to begin")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }
        }

        Item {
            Layout.fillWidth: true
            Layout.preferredHeight: Math.max(120, targetViewport.implicitHeight)
            Layout.maximumHeight: targetViewport.implicitHeight

            TypingWordViewport {
                id: targetViewport
                anchors.fill: parent
                visible: testEngine.hasTarget && testEngine.state !== "loading"
                engine: testEngine
                store: test.store
            }

            Text {
                anchors.fill: parent
                visible: !testEngine.hasTarget
                text: testEngine.inputText.length ? testEngine.inputText : I18n.tr("Free typing. Begin anywhere.")
                color: testEngine.inputText.length ? Tokens.ink : Tokens.inkMuted
                font.family: Tokens.mono
                font.pixelSize: test.store.fontSize
                font.weight: Font.Medium
                wrapMode: Text.Wrap
                verticalAlignment: Text.AlignVCenter
            }

            Column {
                anchors.centerIn: parent
                visible: testEngine.state === "loading"
                spacing: Tokens.s2
                Text { anchors.horizontalCenter: parent.horizontalCenter; text: I18n.tr("LOADING WORDS"); color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro; font.letterSpacing: Tokens.trackLabel }
                Text { anchors.horizontalCenter: parent.horizontalCenter; visible: test.languages.errorText !== ""; text: test.languages.errorText; color: Tokens.alert; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
            }
        }

        TypingKeyboardPreview {
            id: keyboard
            visible: test.store.keyboardEnabled && testEngine.state !== "loading"
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.minimumHeight: Math.min(implicitHeight, 120)
            store: test.store
            nextChar: testEngine.nextExpectedChar
            pressedChar: test.lastPressed
            onRequestInputFocus: test.focusInput()
        }

        RowLayout {
            Layout.fillWidth: true
            Item { Layout.fillWidth: true }
            Btn {
                text: testEngine.mode === "zen" && testEngine.isRunning ? I18n.tr("Finish") : I18n.tr("Restart")
                primary: testEngine.mode === "zen" && testEngine.isRunning
                onAct: {
                    if (testEngine.mode === "zen" && testEngine.isRunning) test.finishZen();
                    else test.restart(false);
                }
            }
        }
    }

    TypingResults {
        anchors.fill: parent
        opacity: testEngine.isFinished ? 1 : 0
        visible: opacity > 0
        enabled: testEngine.isFinished
        engine: testEngine
        personalBest: test.personalBest
        transform: Translate {
            y: testEngine.isFinished ? 0 : Tokens.s2
            Behavior on y {
                enabled: !Tokens.reduceMotion
                NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
            }
        }
        Behavior on opacity {
            enabled: !Tokens.reduceMotion
            NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
        }
        onRestart: sameSeed => test.restart(sameSeed)
    }

    Component.onCompleted: {
        languages.request(store.language);
        Qt.callLater(focusInput);
    }
}

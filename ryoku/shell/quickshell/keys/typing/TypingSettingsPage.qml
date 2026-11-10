pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "."

Item {
    id: settings

    required property var store
    required property var languages
    required property var soundPacks
    signal requestInputFocus()

    readonly property var baseLanguages: Array.from(languages.catalogue)
        .filter(entry => !String(entry.id).endsWith("_extended"))
    readonly property var languageIds: baseLanguages.map(entry => String(entry.id))
    readonly property var languageLabels: {
        const out = {};
        for (const entry of baseLanguages)
            out[entry.id] = entry.label || entry.id;
        return out;
    }
    readonly property var clickIds: Array.from(soundPacks.clickPacks).map(pack => String(pack.id))
    readonly property var clickLabels: {
        const out = {};
        for (const pack of soundPacks.clickPacks)
            out[pack.id] = pack.label || pack.id;
        return out;
    }
    readonly property var errorIds: Array.from(soundPacks.errorPacks).map(pack => String(pack.id))
    readonly property var errorLabels: {
        const out = {};
        for (const pack of soundPacks.errorPacks)
            out[pack.id] = pack.label || pack.id;
        return out;
    }

    Flickable {
        id: scroll
        anchors.fill: parent
        clip: true
        contentWidth: width
        contentHeight: cards.implicitHeight
        boundsBehavior: Flickable.StopAtBounds
        ScrollRail.vertical: ScrollRail { }
        WheelScroll { }

        GridLayout {
            id: cards
            width: scroll.width
            columns: width >= 940 ? 2 : 1
            columnSpacing: Tokens.s4
            rowSpacing: Tokens.s4

            SettingCard {
                Layout.fillWidth: true
                Layout.alignment: Qt.AlignTop
                title: I18n.tr("TEST")
                kana: "試験"
                collapsible: false

                SettingRow {
                    label: I18n.tr("Language")
                    desc: I18n.tr("Bundled word source; no network requests.")
                    block: true
                    Chips {
                        width: parent.width
                        options: settings.languageIds
                        labels: settings.languageLabels
                        current: settings.store.language
                        onChose: key => settings.store.language = key
                    }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Word list")
                    desc: I18n.tr("Standard is curated; extended adds broader vocabulary.")
                    controlWidth: 150
                    Seg {
                        width: 150
                        options: ["standard", "extended"]
                        current: settings.store.wordlistSize
                        onChose: key => settings.store.wordlistSize = key
                    }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Default mode")
                    controlWidth: 190
                    Seg {
                        width: 190
                        options: ["time", "words", "zen"]
                        current: settings.store.mode
                        onChose: key => settings.store.mode = key
                    }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Guided zen")
                    desc: I18n.tr("Keep generated words but remove the finish line.")
                    controlWidth: 54
                    Sw { on: settings.store.zenGuided; onToggled: value => settings.store.zenGuided = value }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Time target")
                    value: String(settings.store.timeLimit)
                    unit: "s"
                    controlWidth: 58
                    Step { value: settings.store.timeLimit; from: 15; to: 120; stepBy: 15; onModified: value => settings.store.timeLimit = value }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Word target")
                    value: String(settings.store.wordLimit)
                    controlWidth: 58
                    Step { value: settings.store.wordLimit; from: 10; to: 200; stepBy: 10; onModified: value => settings.store.wordLimit = value }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Punctuation")
                    controlWidth: 54
                    Sw { on: settings.store.punctuation; onToggled: value => settings.store.punctuation = value }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Numbers")
                    controlWidth: 54
                    Sw { on: settings.store.numbers; onToggled: value => settings.store.numbers = value }
                }
            }

            SettingCard {
                Layout.fillWidth: true
                Layout.alignment: Qt.AlignTop
                title: I18n.tr("TYPE")
                kana: "入力"
                collapsible: false

                SettingRow {
                    label: I18n.tr("Text size")
                    value: String(settings.store.fontSize)
                    unit: "px"
                    controlWidth: 58
                    Step { value: settings.store.fontSize; from: 18; to: 38; stepBy: 2; onModified: value => settings.store.fontSize = value }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Visible lines")
                    value: String(settings.store.visibleLines)
                    controlWidth: 58
                    Step { value: settings.store.visibleLines; from: 2; to: 6; onModified: value => settings.store.visibleLines = value }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Caret")
                    block: true
                    Seg {
                        width: parent.width
                        options: ["line", "block", "underline", "off"]
                        current: settings.store.caretStyle
                        onChose: key => settings.store.caretStyle = key
                    }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Smooth caret")
                    controlWidth: 54
                    Sw { on: settings.store.smoothCaret; onToggled: value => settings.store.smoothCaret = value }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Current word focus")
                    desc: I18n.tr("Dim words outside the active one.")
                    controlWidth: 54
                    Sw { on: settings.store.highlightCurrentWord; onToggled: value => settings.store.highlightCurrentWord = value }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Blind mode")
                    desc: I18n.tr("Do not reveal errors while typing.")
                    controlWidth: 54
                    Sw { on: settings.store.blindMode; onToggled: value => settings.store.blindMode = value }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Live WPM")
                    controlWidth: 54
                    Sw { on: settings.store.showLiveWpm; onToggled: value => settings.store.showLiveWpm = value }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Live accuracy")
                    controlWidth: 54
                    Sw { on: settings.store.showLiveAccuracy; onToggled: value => settings.store.showLiveAccuracy = value }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Quick restart")
                    desc: I18n.tr("Tab restarts the current test.")
                    controlWidth: 54
                    Sw { on: settings.store.quickRestart; onToggled: value => settings.store.quickRestart = value }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Finish on final word")
                    desc: I18n.tr("No trailing space is required.")
                    controlWidth: 54
                    Sw { on: settings.store.finishOnLastWord; onToggled: value => settings.store.finishOnLastWord = value }
                }
            }

            SettingCard {
                Layout.fillWidth: true
                Layout.alignment: Qt.AlignTop
                title: I18n.tr("KEYBOARD")
                kana: "指"
                collapsible: false

                SettingRow {
                    label: I18n.tr("Preview")
                    controlWidth: 54
                    Sw { on: settings.store.keyboardEnabled; onToggled: value => settings.store.keyboardEnabled = value }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Layout")
                    block: true
                    Chips {
                        width: parent.width
                        options: ["qwerty", "qwertz", "azerty", "dvorak", "colemak"]
                        current: settings.store.keyboardLayout
                        onChose: key => settings.store.keyboardLayout = key
                    }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Next key")
                    desc: I18n.tr("Mark the expected key as you type.")
                    controlWidth: 54
                    Sw { on: settings.store.highlightNextKey; onToggled: value => settings.store.highlightNextKey = value }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("Finger guide")
                    desc: I18n.tr("Shade each key by its recommended finger; click a key to adjust it.")
                    controlWidth: 54
                    Sw { on: settings.store.fingerGuide; onToggled: value => settings.store.fingerGuide = value }
                }
                SettingRow {
                    visible: settings.store.keyboardEnabled
                    divider: true
                    label: I18n.tr("Guide preview")
                    block: true
                    footH: keyboard.implicitHeight
                    TypingKeyboardPreview {
                        id: keyboard
                        width: parent.width
                        store: settings.store
                        nextChar: "f"
                        onRequestInputFocus: settings.requestInputFocus()
                    }
                }
            }

            SettingCard {
                Layout.fillWidth: true
                Layout.alignment: Qt.AlignTop
                title: I18n.tr("SOUND & DATA")
                kana: "記録"
                collapsible: false

                SettingRow {
                    label: I18n.tr("Key sounds")
                    desc: I18n.tr("Local Monkeytype samples; nothing is streamed.")
                    controlWidth: 54
                    Sw { on: settings.store.soundsEnabled; onToggled: value => settings.store.soundsEnabled = value }
                }
                SettingRow {
                    divider: true
                    enabled: settings.store.soundsEnabled
                    label: I18n.tr("Key sound")
                    block: true
                    Chips {
                        width: parent.width
                        options: settings.clickIds
                        labels: settings.clickLabels
                        current: settings.store.soundTheme
                        onChose: key => settings.store.soundTheme = key
                    }
                }
                SettingRow {
                    divider: true
                    enabled: settings.store.soundsEnabled
                    label: I18n.tr("Error sound")
                    controlWidth: 54
                    Sw { on: settings.store.errorSound; onToggled: value => settings.store.errorSound = value }
                }
                SettingRow {
                    divider: true
                    enabled: settings.store.soundsEnabled && settings.store.errorSound
                    label: I18n.tr("Error tone")
                    block: true
                    Chips {
                        width: parent.width
                        options: settings.errorIds
                        labels: settings.errorLabels
                        current: settings.store.errorTheme
                        onChose: key => settings.store.errorTheme = key
                    }
                }
                SettingRow {
                    divider: true
                    enabled: settings.store.soundsEnabled
                    label: I18n.tr("Volume")
                    value: String(settings.store.soundVolume)
                    unit: "%"
                    block: true
                    Slid { width: parent.width; value: settings.store.soundVolume; from: 0; to: 100; onModified: value => settings.store.soundVolume = value }
                }
                SettingRow {
                    divider: true
                    label: I18n.tr("History")
                    desc: I18n.tr("Store aggregate scores locally, never typed text.")
                    controlWidth: 54
                    Sw { on: settings.store.historyEnabled; onToggled: value => settings.store.historyEnabled = value }
                }
                SettingRow {
                    divider: true
                    enabled: settings.store.historyEnabled
                    label: I18n.tr("History limit")
                    value: String(settings.store.maxEntries)
                    controlWidth: 58
                    Step { value: settings.store.maxEntries; from: 10; to: 500; stepBy: 10; onModified: value => settings.store.maxEntries = value }
                }
            }
        }
    }
}

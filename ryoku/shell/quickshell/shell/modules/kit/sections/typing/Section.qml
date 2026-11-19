pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: root
    required property real s
    property bool active: false
    property string query: ""
    property var args: ({})
    property real maximumHeight: height
    property var memory: null
    property var source: null
    property string pageId: "test"
    property bool editing: true
    readonly property string title: I18n.tr("Typing test")
    readonly property string placeholder: pageId === "settings" ? (settings.editorOpen ? I18n.tr("Editing finger assignments") : I18n.tr("Filter settings")) : ownsText ? I18n.tr("Typing goes to the test") : pageId === "history" ? I18n.tr("Search typing history") : I18n.tr("Search every section with Ctrl+F")
    readonly property bool ownsText: pageId === "test" && editing || pageId === "settings" && settings.editorOpen
    readonly property var pages: [
        {id: "test", label: I18n.tr("Test")}, {id: "history", label: I18n.tr("History")},
        {id: "stats", label: I18n.tr("Stats")}, {id: "settings", label: I18n.tr("Settings")}
    ]
    readonly property string status: pageId === "settings" ? I18n.tr("Changes apply at once") : languages.currentPack ? I18n.tr("%1 · %2 words").arg(I18n.tr(languages.languageFor(languages.currentLanguageId)?.label || languages.currentPack.name)).arg(languages.currentPack.words.length) : languages.errorText
    readonly property var primaryAction: pageId === "test" ? {label: I18n.tr("Restart"), hint: "Tab"} : pageId === "settings" && !settings.editorOpen && settings.current ? {label: settings.current.id === "clear" ? (settings.confirmClear ? I18n.tr("Delete all saved tests") : I18n.tr("Clear history")) : settings.current.kind === "toggle" ? (settings.current.on ? I18n.tr("Turn off") : I18n.tr("Turn on")) : settings.current.kind === "step" ? I18n.tr("Next value") : I18n.tr("Change")} : null
    readonly property var secondaryAction: pageId === "test" && store.mode === "zen" ? {label: I18n.tr("Finish"), hint: "Ctrl+Enter"} : null
    readonly property var actions: [{id: "clear", label: I18n.tr("Clear typing history"), glyph: "delete", destructive: true}]
    signal hud(string text)
    signal alert(string text)
    signal queryRequested(string text)
    signal closeRequested()
    signal menuRequested()
    function activate() { if (pageId === "test") { editing = true; test.restart(false); } else if (pageId === "settings") settings.activate(); }
    function activateSecondary() { if (pageId === "test") test.finishZen(); }
    function takeFocus() { if (!active) return; if (pageId === "settings") settings.takeFocus(); else if (ownsText) test.focusInput(); }
    function handleKey(event) {
        if (pageId === "settings") {
            if (settings.editorOpen) return settings.handleKey(event);
            if (event.modifiers === Qt.NoModifier) {
                if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space) { settings.activate(); return true; }
                if (event.key === Qt.Key_Left || event.key === Qt.Key_Right) { settings.step(event.key === Qt.Key_Left ? -1 : 1); return true; }
            }
            return false;
        }
        const item = root.Window.window ? root.Window.window.activeFocusItem : null;
        if (!item) return false;
        const press = event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space;
        if (typeof item.step === "function" && (press || event.key === Qt.Key_Left || event.key === Qt.Key_Right)) {
            item.step(event.key === Qt.Key_Left ? -1 : 1); return true;
        }
        if (press && typeof item.clicked === "function") { item.clicked(); return true; }
        return false;
    }
    function move(delta) {
        if (pageId === "history") history.scroll(delta * 36 * s);
        else if (pageId === "stats") stats.contentY = Math.max(0, Math.min(stats.contentHeight - stats.height, stats.contentY + delta * 36 * s));
        else if (pageId === "settings") settings.move(delta);
    }
    function page(delta) { if (pageId === "settings") settings.page(delta); else move(delta * 10); }
    function jumpSection(delta) { if (pageId === "settings") settings.jumpSection(delta); }
    function handleEscape() {
        if (pageId === "settings" && settings.handleEscape()) return true;
        if (pageId !== "test") { pageId = "test"; editing = true; return true; }
        if (test.finished) { test.restart(false); return true; }
        if (editing) { test.cancel(); editing = false; return true; }
        return false;
    }
    function act(id) { if (id === "clear") { pageId = "settings"; queryRequested(""); Qt.callLater(settings.requestClear); } }
    onPageIdChanged: {
        if (!pages.some(p => p.id === pageId)) { pageId = "test"; return; }
        if (pageId !== "test") test.cancel();
        else editing = true;
        Qt.callLater(takeFocus);
    }
    onActiveChanged: { if (!active) test.cancel(); else Qt.callLater(takeFocus); }
    TypingStore { id: store; kit: root.memory }
    TypingLanguages { id: languages; store: store }
    Loader { id: packs; active: store.soundsEnabled; sourceComponent: Component { TypingSoundPacks {} } }
    TypingTestView {
        id: test
        anchors.fill: parent
        s: root.s
        active: root.active && root.pageId === "test" && root.editing
        visible: root.pageId === "test"
        store: store
        languages: languages
        soundPacks: packs.item
        onRequestFocus: { root.editing = true; root.takeFocus(); }
    }
    TypingHistoryPage { id: history; anchors.fill: parent; visible: root.pageId === "history"; s: root.s; store: store; query: root.query }
    TypingStatsPage { id: stats; anchors.fill: parent; visible: root.pageId === "stats"; s: root.s; store: store }
    TypingSettingsPage { id: settings; anchors.fill: parent; visible: root.pageId === "settings"; s: root.s; store: store; languages: languages; query: root.query }
    Component.onCompleted: Qt.callLater(takeFocus)
}

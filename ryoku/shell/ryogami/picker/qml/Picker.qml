import QtQuick
import Quickshell
import Quickshell.Wayland
import Ryoku.Ui.Singletons

PanelWindow {
    id: picker

    property bool shown: false
    readonly property bool active: picker.shown || picker._showPending
    readonly property bool busy: Settings.busy || riceShare.busy || riceWorkshop.busy
    property var _screen: null
    property bool _showPending: false
    property int _showRequest: 0

    screen: picker._screen
    visible: picker.shown
    color: "transparent"
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "ryogami-picker"
    WlrLayershell.keyboardFocus: picker.shown ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
    anchors { top: true; bottom: true; left: true; right: true }

    property SettingValue _monitorSetting: SettingValue { key: "monitor" }
    property SettingValue _backdrop: SettingValue { key: "general.selectorBackdropOpacity" }
    readonly property real _backdropOpacity: {
        var v = Number(picker._backdrop.value)
        return isNaN(v) ? 0 : Math.max(0, Math.min(1, v / 100))
    }

    Binding {
        target: Settings
        property: "viewportWidth"
        value: picker.width
        when: picker.width > 0
    }

    function show() {
        if (picker.shown || picker._showPending)
            return
        var request = ++picker._showRequest
        picker._showPending = true
        picker._latchScreen(function() {
            if (request !== picker._showRequest)
                return
            picker.shown = true
            picker._showPending = false
        })
    }
    function hide() {
        ++picker._showRequest
        picker._showPending = false
        picker.shown = false
    }
    function toggle() {
        if (picker.shown || picker._showPending)
            picker.hide()
        else
            picker.show()
    }
    function openSettings(tab) {
        picker.show()
        pickerState.openSheet("settings", tab ? { tab: tab } : ({}))
    }

    function _screenByName(name) {
        var ss = Quickshell.screens
        for (var i = 0; i < ss.length; ++i)
            if (ss[i].name === name)
                return ss[i]
        return null
    }
    function _latchScreen(done) {
        var name = picker._monitorSetting.value ? String(picker._monitorSetting.value) : ""
        if (name) {
            picker._screen = picker._screenByName(name)
            pickerState.monitor = name
            done()
            return
        }
        Daemon.call("wm.focusedOutput", {}, function(result, error) {
            var n = ""
            if (!error && result)
                n = (typeof result === "string") ? result : (result.name || result.output || "")
            picker._screen = n ? picker._screenByName(n) : null
            pickerState.monitor = n
            done()
        })
    }

    function _restoreFocus() {
        if (!picker.shown)
            return
        if (pickerState.sheet !== "" || pickerState.searchOpen || pickerState.riceShare !== "")
            return   // a text surface owns the keyboard
        cardField.forceActiveFocus()
    }

    Connections {
        target: pickerState
        function onSheetChanged() { picker._restoreFocus() }
        function onSearchOpenChanged() { picker._restoreFocus() }
        function onMultipickerOpenChanged() { picker._restoreFocus() }
        function onHelpOpenChanged() { picker._restoreFocus() }
        function onRiceWorkshopOpenChanged() { picker._restoreFocus() }
        function onRiceShareChanged() { picker._restoreFocus() }
        function onThemeBarOpenChanged() { picker._restoreFocus() }
    }

    PickerState {
        id: pickerState
        view: libraryView
        field: cardField
        shown: picker.shown
        onHideRequested: picker.hide()
        onToastRequested: (message, kind) => toastLayer.show(message, kind)
    }

    LibraryView {
        id: libraryView
        collection: pickerState.collection
    }

    FocusScope {
        id: content
        anchors.fill: parent
        focus: true
        Keys.onPressed: (event) => keymap.handleKey(event)

        Rectangle {
            anchors.fill: parent
            color: "black"
            opacity: picker._backdropOpacity
            visible: opacity > 0
        }

        CardField {
            id: cardField
            anchors.fill: parent
            focus: true
            source: pickerState.view
            settings: Settings
            mode: pickerState.mode
            active: picker.shown
            previewAllowed: picker.shown && pickerState.sheet === "" && !pickerState.searchOpen
                && !pickerState.multipickerOpen && !pickerState.helpOpen
                && !pickerState.themeBarOpen && !pickerState.riceWorkshopOpen
                && pickerState.riceShare === ""
            interactive: pickerState.sheet === "" && !pickerState.multipickerOpen
                && !pickerState.helpOpen && !pickerState.riceWorkshopOpen
                && pickerState.riceShare === ""
            palette: ({
                primary: Theme.primary,
                accent: Theme.primary,
                surface: Theme.surface,
                surfaceVariant: Theme.surfaceVariant,
                outline: Theme.outline,
                text: Theme.surfaceText,
                shadow: Theme.background
            })
            onActivated: (row) => keymap.mouseGesture(false, Qt.NoModifier, row)
            onContextRequested: (row, pos) => keymap.mouseGesture(true, Qt.NoModifier, row)
            onBackgroundClicked: () => pickerState.goBack()
        }

        WorkshopCardChrome { anchors.fill: parent; state: pickerState }
        CardChrome { anchors.fill: parent; state: pickerState }
        EmptyState { anchors.fill: parent; state: pickerState }

        FilterBar { anchors.fill: parent; state: pickerState }
        SearchPanel { anchors.fill: parent; state: pickerState }
        HelpOverlay { anchors.fill: parent; state: pickerState }
        ThemeBar { anchors.fill: parent; state: pickerState }
        RiceWorkshop { id: riceWorkshop; anchors.fill: parent; state: pickerState }
        RiceShare { id: riceShare; anchors.fill: parent; state: pickerState }

        CardBackPanel { anchors.fill: parent; state: pickerState }
        Multipicker { anchors.fill: parent; state: pickerState }
        SceneBackdrop { anchors.fill: parent; state: pickerState }
        SheetHost { anchors.fill: parent; state: pickerState }
        ToastLayer { id: toastLayer; anchors.fill: parent; state: pickerState }

        Keymap { id: keymap; state: pickerState }
    }
}

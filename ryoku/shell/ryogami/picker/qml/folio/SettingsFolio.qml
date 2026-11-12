import QtQuick
import Ryoku.Ui.Singletons
import "FolioConditions.js" as Cond
import "FolioActions.js" as Actions

FocusScope {
    id: folio

    required property PickerState state
    property var args
    property bool shown: false
    signal closeRequested()

    anchors.fill: parent
    focus: folio.shown
    onShownChanged: if (folio.shown) { folio._applyArgs(); folio.forceActiveFocus(); }
    // A request for another tab can arrive while the folio is already open.
    onArgsChanged: if (folio.shown) folio._applyArgs()

    // Escape leaves an open search before it closes the sheet.
    Keys.onEscapePressed: (e) => {
        if (folio.searchOpen) {
            folio.searchOpen = false
            folio.query = ""
            folio.forceActiveFocus()
        } else {
            folio.closeRequested()
        }
        e.accepted = true
    }
    Keys.onPressed: (e) => {
        if (e.key === Qt.Key_F && (e.modifiers & Qt.ControlModifier)) {
            folio.searchOpen = true
            e.accepted = true
        }
    }

    // Pure data, so every tab can live at once.
    WallpaperTab { id: tWallpaper }
    ColoursTab { id: tColours }
    LightingTab { id: tLighting }
    PickerTab { id: tPicker }
    SystemTab { id: tSystem }
    readonly property var tabs: [tWallpaper, tColours, tLighting, tPicker, tSystem]

    // tab keys from before the regroup, as [tab, section, control]; Settings and scripts still pass them
    readonly property var tabAliases: ({
        library: ["wallpaper", "library"],
        sources: ["wallpaper", "sources"],
        displays: ["wallpaper", "playback"],
        playback: ["wallpaper", "playback"],
        transitions: ["wallpaper", "transitions"],
        automation: ["wallpaper", "automation"],
        theme: ["colours", "source"],
        colors: ["colours", "source"],
        integrations: ["system", "integrations"],
        language: ["system", "integrations", "action.openTimeLanguage"],
        filter: ["picker", "filter"],
        position: ["picker", "position"],
        motion: ["picker", "motion"],
        performance: ["system", "performance"]
    })
    readonly property var controlAliases: ({
        "theme.mode@matugen": "theme.mode",
        "action.openThemeDesigner.fixed": "action.openThemeDesigner",
        "action.openThemeDesigner.wallpaper": "action.openThemeDesigner",
        "general.language": "action.openTimeLanguage"
    })
    readonly property var lightingController: lighting
    LightingController {
        id: lighting
        active: folio.shown && folio.activeTab && folio.activeTab.tabKey === "lighting"
    }

    property int activeTabIndex: 0
    property int activeSectionIndex: 0
    property bool searchOpen: false
    property string query: ""
    property string jumpControlId: ""

    readonly property var activeTab: folio.tabs[folio.activeTabIndex]
    readonly property var activeSection: (folio.activeTab && folio.activeTab.sections
        && folio.activeSectionIndex < folio.activeTab.sections.length)
        ? folio.activeTab.sections[folio.activeSectionIndex] : null
    readonly property bool isDesign: folio._sectionIsDesign(folio.activeSection)
    // The picker keeps the scene rendered behind the folio for design sections.
    readonly property bool sceneVisible: folio.isDesign
    Binding {
        target: folio.state
        property: "sceneThrough"
        value: folio.shown && folio.sceneVisible
    }

    function _sectionIsDesign(section) {
        if (!section) return false;
        var cs = section.controls || [];
        for (var i = 0; i < cs.length; i++)
            if (cs[i].perMode === true) return true;
        return false;
    }

    property int rev: 0
    readonly property var condCtx: Cond.makeCtx(Settings)
    Connections {
        target: Settings
        function onChanged(key, value) { folio.rev++; }
        function onSchemaChanged() { folio.rev++; }
    }

    FolioOptions { id: folioOptions }

    // tab, section and control each by key; a numeric section indexes a current tab, a control finds its own section
    function _applyArgs() {
        var a = folio.args;
        if (!a) return;
        var tab = a.tab === undefined || a.tab === null ? "" : a.tab;
        var section = a.section === undefined || a.section === null || a.section === "-" ? "" : String(a.section);
        var control = a.control === undefined || a.control === null ? "" : String(a.control);
        var t = folio._tabIndex(tab);
        var s = -1;
        var alias = typeof tab === "string" ? folio.tabAliases[tab] : undefined;
        if (t < 0 && alias) {
            t = folio._tabIndex(alias[0]);
            s = folio._sectionIndex(t, alias[1]);
            if (control.length === 0 && alias.length > 2) control = alias[2];
        }
        if (t >= 0 && section.length > 0) {
            var si = folio._sectionIndex(t, section);
            if (si < 0 && !alias && /^[0-9]+$/.test(section)) si = Number(section);
            if (si >= 0 && si < folio.tabs[t].sections.length) s = si;
            else if (control.length === 0) control = section;
        }
        if (folio.controlAliases[control] !== undefined) control = folio.controlAliases[control];
        if (control.length > 0) {
            var at = folio._locateControl(control, t);
            if (at) { t = at.tab; s = at.section; }
            else control = "";
        }
        if (t < 0) return;
        folio.activeTabIndex = t;
        folio.activeSectionIndex = s >= 0 ? s : 0;
        folio.jumpControlId = "";
        if (control.length > 0) Qt.callLater(function () { folio.jumpControlId = control; });
    }
    function _tabIndex(id) {
        if (typeof id === "number") return id >= 0 && id < folio.tabs.length ? id : -1;
        for (var i = 0; i < folio.tabs.length; i++)
            if (folio.tabs[i].tabKey === id) return i;
        return -1;
    }
    function _sectionIndex(t, key) {
        if (t < 0) return -1;
        var secs = folio.tabs[t].sections || [];
        for (var i = 0; i < secs.length; i++)
            if (secs[i].key === key) return i;
        return -1;
    }
    // the preferred tab first, then the rest, so a control moved by the regroup is still found
    function _locateControl(id, preferred) {
        var order = [];
        if (preferred >= 0) order.push(preferred);
        for (var i = 0; i < folio.tabs.length; i++)
            if (i !== preferred) order.push(i);
        for (var o = 0; o < order.length; o++) {
            var secs = folio.tabs[order[o]].sections || [];
            for (var s = 0; s < secs.length; s++) {
                var cs = secs[s].controls || [];
                for (var c = 0; c < cs.length; c++)
                    if (cs[c].id === id || cs[c].key === id) return { tab: order[o], section: s };
            }
        }
        return null;
    }

    function _selectTab(i) { folio.activeTabIndex = i; folio.activeSectionIndex = 0; folio.jumpControlId = ""; }
    function _selectSection(t, s) { folio.activeTabIndex = t; folio.activeSectionIndex = s; folio.jumpControlId = ""; }
    function _openResult(t, s, cid) {
        folio.activeTabIndex = t; folio.activeSectionIndex = s;
        folio.searchOpen = false; folio.query = "";
        // Reopening the same search result must expand its group again.
        folio.jumpControlId = "";
        Qt.callLater(function () { folio.jumpControlId = cid; });
    }

    property string capturingKey: ""
    property var _captureControl: null
    function captureKeybind(control) {
        folio._captureControl = control;
        folio.capturingKey = control.key ? control.key : "";
    }
    function _conflictFor(chord) {
        var tab = folio.activeTab;
        if (!tab || chord.length === 0) return "";
        var mineKey = folio._captureControl ? folio._captureControl.key : "";
        var secs = tab.sections || [];
        for (var s = 0; s < secs.length; s++) {
            var cs = secs[s].controls || [];
            for (var c = 0; c < cs.length; c++) {
                if (cs[c].kind !== "keybind" || cs[c].key === mineKey) continue;
                if (String(Settings.value(cs[c].key) || "") === chord)
                    return (folio._captureControl ? folio._captureControl.label : "")
                        + " " + I18n.tr("and") + " " + cs[c].label + " " + I18n.tr("both use") + " " + chord + ".";
            }
        }
        return "";
    }

    property bool processPickerOpen: false
    function chooseProcess() { folio.processPickerOpen = true; }
    function _addProcess(name) {
        if (!name || name.length === 0) return;
        var arr = Actions.asList(Settings.value("playback.processes"));
        if (arr.indexOf(name) < 0) { arr.push(name); Settings.set("playback.processes", arr); }
    }

    property real reveal: folio.shown ? 1 : 0
    Behavior on reveal { NumberAnimation { duration: Theme.standard; easing.type: Theme.revealEasing } }
    visible: folio.reveal > 0.001

    FolioSheet {
        id: sheet
        reveal: folio.reveal
        // the index carries the title, so an empty masthead would only be a band above the page
        showMasthead: false
        // a design section keeps the scene in view behind a surface plate that still carries the text
        pageOpacity: folio.isDesign ? 0.8 : 0.965
        scrimAlpha: folio.isDesign ? 0.2 : 0.68
        onDismissed: folio.closeRequested()

        FolioIndex {
            parent: sheet.indexArea
            anchors.fill: parent
            reveal: folio.reveal
            tabs: folio.tabs
            activeTabIndex: folio.activeTabIndex
            activeSectionIndex: folio.activeSectionIndex
            searchOpen: folio.searchOpen
            query: folio.query
            onSelectTab: (i) => folio._selectTab(i)
            onSelectSection: (t, s) => folio._selectSection(t, s)
            onOpenResult: (t, s, cid) => folio._openResult(t, s, cid)
            onSearchToggled: (open) => folio.searchOpen = open
            onQueryEdited: (t) => folio.query = t
        }

        Loader {
            parent: sheet.readingArea
            anchors.fill: parent
            sourceComponent: folio.isDesign ? studioComp : pageComp
        }
    }

    Component {
        id: pageComp
        FolioPage {
            tab: folio.activeTab
            sectionIndex: folio.activeSectionIndex
            state: folio.state
            options: folioOptions
            host: folio
            condCtx: folio.condCtx
            rev: folio.rev
            jumpControlId: folio.jumpControlId
            onCloseRequested: folio.closeRequested()
        }
    }
    Component {
        id: studioComp
        ShapeStudio {
            tab: folio.activeTab
            sectionIndex: folio.activeSectionIndex
            state: folio.state
            options: folioOptions
            host: folio
            condCtx: folio.condCtx
            rev: folio.rev
            jumpControlId: folio.jumpControlId
            onCloseRequested: folio.closeRequested()
        }
    }

    Loader {
        anchors.fill: parent
        active: folio.capturingKey.length > 0
        sourceComponent: KeybindCapture {
            actionLabel: folio._captureControl ? folio._captureControl.label : ""
            currentBinding: folio.capturingKey.length > 0 ? String(Settings.value(folio.capturingKey) || "") : ""
            conflictFor: folio._conflictFor
            Component.onCompleted: open()
            onSaved: (chord) => { Settings.set(folio.capturingKey, chord); folio.capturingKey = ""; }
            onCleared: { Settings.set(folio.capturingKey, ""); folio.capturingKey = ""; }
            onCancelled: folio.capturingKey = ""
        }
    }

    Loader {
        anchors.fill: parent
        active: folio.processPickerOpen
        sourceComponent: FocusScope {
            anchors.fill: parent
            focus: true
            Component.onCompleted: manual.forceActiveFocus()
            Keys.onEscapePressed: (e) => { folio.processPickerOpen = false; e.accepted = true; }

            Scrim { anchors.fill: parent; alpha: 0.5; onDismissed: folio.processPickerOpen = false }

            ChamferPanel {
                anchors.centerIn: parent
                width: Math.min(parent.width - 80 * Theme.scale, 420 * Theme.scale)
                height: pcol.implicitHeight + 40 * Theme.scale

                Column {
                    id: pcol
                    anchors.centerIn: parent
                    width: parent.width - 44 * Theme.scale
                    spacing: 12 * Theme.scale

                    Text {
                        width: parent.width
                        text: I18n.tr("Add a program to pause for")
                        font.family: Theme.display
                        font.pixelSize: Theme.fontTitle
                        color: Theme.surfaceText
                        wrapMode: Text.WordWrap
                        renderType: Text.NativeRendering
                    }
                    Text {
                        width: parent.width
                        text: I18n.tr("Enter the process name, for example mpv or steam.")
                        font.family: Theme.sans
                        font.weight: Font.Medium
                        font.pixelSize: Theme.fontBase
                        color: Theme.withAlpha(Theme.surfaceText, 0.56)
                        wrapMode: Text.WordWrap
                        renderType: Text.NativeRendering
                    }
                    TextField {
                        id: manual
                        width: parent.width
                        variant: "field"
                        placeholder: I18n.tr("Process name")
                        onCommitted: (t) => { folio._addProcess(t.trim()); folio.processPickerOpen = false; }
                    }
                    Row {
                        anchors.horizontalCenter: parent.horizontalCenter
                        spacing: 9 * Theme.scale
                        FolioAction {
                            label: I18n.tr("Add")
                            enabled: manual.text.trim().length > 0
                            onTriggered: { folio._addProcess(manual.text.trim()); folio.processPickerOpen = false; }
                        }
                        FolioAction {
                            label: I18n.tr("Cancel")
                            onTriggered: folio.processPickerOpen = false
                        }
                    }
                }
            }
        }
    }
}

pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "." as HubComponents
import "Singletons"
import "ReloadCoverModel.js" as ReloadCoverModel

// Every settings page, once. A page supplies its schema, its draft and its
// defaults; this draws the tabs its schema declares and hands the rows to a
// SettingsSheet. There is no per-page layout to write, which is the point:
// 30 pages that differ only in data should not be 30 files that differ in
// code.
//
// Surfaces that are not settings (a preview, a console, a drag-arrange) do not
// belong in a cell and are not forced into one: a page passes them in as
// `extras` and they sit full width inside the sheet's grid.
Item {
    id: page

    property var schema: []
    property var draft: null
    property var defaults: ({})
    property string title: ""
    property string eyebrow: I18n.tr("DESKTOP")
    property string blurb: ""
    property string query: ""
    property alias tab: sheet.tab
    // The card measure the grid below uses, so a page's own block can sit on the
    // same grid instead of spanning the window with an empty half.
    readonly property alias cardWidth: sheet.cardW
    property alias advanced: sheet.advanced
    // extras ride inside the sheet's scroll area, not pinned above it, so a page
    // with a tall extra block still scrolls as one surface.
    default property alias extras: sheet.lead
    property string externalReloadCoverError: ""
    property string importReloadCoverError: ""
    property var pendingReloadCoverRow: null
    property bool reloadCoverImportBusy: false
    property int reloadCoverImportGeneration: 0

    signal edited(string key, var value)
    signal pickRequested(var row)

    function invalidateReloadCoverImport() {
        reloadCoverImportGeneration++;
        pendingReloadCoverRow = null;
    }
    signal timezonePickRequested(var row)

    // a search jump forwards here; the sheet switches tab, scrolls, and flashes.
    function focusKey(k) { sheet.focusKey(k) }

    // A row the active compositor cannot back is dropped, not shown dead. Three
    // gates: caps (a behavioural capability), modelsKey (a store leaf nothing
    // would write), and the tiling illustration -- a bespoke keyless control the
    // key gate cannot see, which needs a tiled layout to mean anything. Empty
    // groups and tabs fall away on their own, both being derived from what
    // survives here.
    readonly property var capsSchema: (schema || []).filter(function (r) {
        if (!Settings.supports(r.caps)) return false;
        if (r.ctl === "layoutdemo" && !Settings.supports("tiledLayout")) return false;
        return Settings.modelsKey(r.key);
    })

    // A page may add a tab that holds no settings rows, for content that belongs
    // to the page rather than to a group of controls. It goes last so the page
    // still opens on its first real tab.
    property var tailTabs: []

    readonly property var tabs: {
        var t = [];
        for (var i = 0; i < capsSchema.length; i++) {
            var x = capsSchema[i].tab;
            if (x && t.indexOf(x) < 0) t.push(x);
        }
        for (var j = 0; j < page.tailTabs.length; j++)
            if (t.indexOf(page.tailTabs[j]) < 0) t.push(page.tailTabs[j]);
        return t;
    }

    Column {
        id: head
        anchors { left: parent.left; top: parent.top }
        anchors.leftMargin: sheet.sheetX
        width: sheet.sheetWidth
        // the register row sits off the title: a rule over a 32px
        // title needs more than the gap between two lines of body text
        spacing: Tokens.s3

        Item {
            width: parent.width
            // an eyebrow that only repeats the title is noise, not a register
            visible: I18n.tr(page.eyebrow).toLowerCase() !== page.title.toLowerCase()
            height: 18
            Row {
                id: ebrow
                spacing: Tokens.s2
                anchors.verticalCenter: parent.verticalCenter
                Rectangle { width: 16; height: 1; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
                Text {
                    text: "力"; color: Tokens.ink; font.family: Tokens.jp
                    font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter
                }
                Text {
                    text: I18n.tr(page.eyebrow); color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                    anchors.verticalCenter: parent.verticalCenter
                }
            }
            // the band runs to the page edge and closes with the sheet's marks:
            // a register cross and the /// cluster, per the reference poster.
            Rectangle {
                anchors { left: ebrow.right; right: parent.right; verticalCenter: parent.verticalCenter }
                anchors.leftMargin: Tokens.s3
                height: 1; color: Tokens.lineSoft
            }
        }
        Text {
            text: page.title
            color: Tokens.ink
            font.family: Tokens.display
            font.pixelSize: Tokens.fTitle
        }
        Text {
            visible: page.blurb !== ""
            width: Math.min(parent.width, 720)
            text: I18n.tr(page.blurb)
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
            wrapMode: Text.WordWrap
        }
        Item { width: 1; height: Tokens.s1 }

        // a page with one tab does not need a tab bar
        Tabs {
            visible: page.tabs.length > 1
            options: page.tabs
            current: sheet.tab
            onChose: (label) => sheet.tab = label
        }
    }

    SettingsSheet {
        id: sheet
        anchors {
            left: parent.left; right: parent.right
            top: head.bottom
            bottom: parent.bottom
            topMargin: Tokens.s5
        }
        schema: page.capsSchema
        draft: page.draft
        defaults: page.defaults
        query: page.query
        tab: page.tabs.length ? page.tabs[0] : ""
        onEdited: (k, v) => page.edited(k, v)
        onPickRequested: (r) => page.pickRequested(r)
        reloadCoverError: page.importReloadCoverError || page.externalReloadCoverError
        reloadCoverImportBusy: page.reloadCoverImportBusy
        onReloadCoverPickRequested: (r) => {
            if (page.reloadCoverImportBusy) return;
            page.importReloadCoverError = "";
            page.pendingReloadCoverRow = r;
            reloadCoverPick.open();
        }
        onReloadCoverDefaultRequested: (r) => {
            page.invalidateReloadCoverImport();
            page.importReloadCoverError = "";
            page.edited(r.key, ReloadCoverModel.empty());
        }
        onTimezonePickRequested: (r) => page.timezonePickRequested(r)
        onAppPickRequested: (r) => { page.pendingAppRow = r; appPick.visible = true; }
    }

    // The app control: SettingsSheet emits appPickRequested; the hub's
    // AppPicker overlay lists the installed apps and the chosen command lands
    // on the row's key like any edit.
    property var pendingAppRow: null

    HubComponents.AppPicker {
        id: appPick
        anchors.fill: parent
        visible: false
        z: 20
        title: page.pendingAppRow ? I18n.tr(String(page.pendingAppRow.label || "")) : ""
        onChosen: (cmd) => {
            if (page.pendingAppRow)
                page.edited(page.pendingAppRow.key, cmd);
            page.pendingAppRow = null;
            appPick.visible = false;
        }
        onDismissed: {
            page.pendingAppRow = null;
            appPick.visible = false;
        }
    }


    PickFile {
        id: reloadCoverPick
        title: I18n.tr("Add reload-cover asset")
        emptyText: I18n.tr("No supported media or folders here")
        fileFilters: ReloadCoverModel.filters()
        onPicked: (p) => {
            reloadCoverPick.active = false;
            if (page.reloadCoverImportBusy || !page.pendingReloadCoverRow) return;
            page.reloadCoverImportGeneration++;
            reloadCoverImport.requestGeneration = page.reloadCoverImportGeneration;
            reloadCoverImport.command = ["ryoku-hub", "reload-cover", "import", "" + p];
            page.reloadCoverImportBusy = true;
            reloadCoverImport.running = true;
        }
        onCanceled: {
            reloadCoverPick.active = false;
            if (!page.reloadCoverImportBusy)
                page.pendingReloadCoverRow = null;
        }
    }

    Process {
        id: reloadCoverImport
        property int requestGeneration: 0
        running: false
        stdout: StdioCollector { id: reloadCoverImportOut }
        stderr: StdioCollector { id: reloadCoverImportErr }
        onExited: function(code) {
            var isCurrent = requestGeneration === page.reloadCoverImportGeneration;
            page.reloadCoverImportBusy = false;
            if (!isCurrent) return;

            var row = page.pendingReloadCoverRow;
            if (code === 0) {
                try {
                    var descriptor = ReloadCoverModel.normalize(JSON.parse(reloadCoverImportOut.text.trim()));
                    if (descriptor.path === "")
                        throw new Error("missing managed path");
                    if (row)
                        page.edited(row.key, descriptor);
                    page.importReloadCoverError = "";
                } catch (e) {
                    page.importReloadCoverError = I18n.tr("Couldn't import this asset. Choose a supported local file under 64 MiB.");
                }
            } else {
                page.importReloadCoverError = reloadCoverImportErr.text.trim()
                    || I18n.tr("Couldn't import this asset. Choose a supported local file under 64 MiB.");
            }
            page.pendingReloadCoverRow = null;
        }
    }
}

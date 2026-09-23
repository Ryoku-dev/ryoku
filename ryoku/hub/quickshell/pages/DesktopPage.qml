import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import ".."
import "../schema/DesktopPage.js" as DesktopSchema
import "../Singletons"

// Desktop: what sits on the desktop itself. The brand mark, the weather source
// the widgets and bar read, the widget board, and the audio visualiser.
// Rendered through the shared SchemaPage; the ledger and action bar belong to
// the shell.
Item {
    id: pg
    property var hub

    readonly property string pTitle: I18n.tr("Desktop")
    readonly property string pEyebrow: I18n.tr("DESKTOP")
    readonly property string pBlurb: I18n.tr("The brand mark, the pickers, and the audio visualiser.")
    function focusKey(k) { sp.focusKey(k) }

    // ── Pickers ───────────────────────────────────────────────────────────────
    // The theme/wallpaper/media picker layout lives at qsbar.pickerStyle, a nested
    // leaf under the bar's own map. Read and write it through the daemon settings
    // seam so a write patches only that key and never rebuilds the whole qsbar
    // object, which would clobber the bar layout and widgets the panel writes; it
    // applies live like every daemon-backed key.
    readonly property var pickerOptions: ["tanzaku", "hearthstone", "carousel"]
    readonly property string pickerStyle: {
        Settings.revision;
        const v = Settings.get("qsbar.pickerStyle");
        return (v === undefined || v === null || v === "") ? "tanzaku" : v;
    }
    function setPickerStyle(k) {
        if (k) Settings.patch("qsbar.pickerStyle", k);
    }

    // ── Clipboard storage ─────────────────────────────────────────────────────
    // What the history occupies, and how to get the space back. The numbers come
    // from the daemon through the Hub backend: it owns the entries and their
    // files, so the size shown here is the size a prune actually frees, and the
    // prune is the same operation the panel's clear runs.
    property var clipStats: null
    property bool clipBusy: false
    property string clipError: ""

    function humanBytes(n) {
        if (n < 1024)
            return n + " B";
        if (n < 1048576)
            return (n / 1024).toFixed(1) + " KB";
        if (n < 1073741824)
            return (n / 1048576).toFixed(1) + " MB";
        return (n / 1073741824).toFixed(2) + " GB";
    }

    readonly property string storageSize: {
        if (pg.clipError !== "")
            return "";
        return pg.clipStats ? pg.humanBytes(pg.clipStats.bytes) : "";
    }
    readonly property string storageDesc: {
        if (pg.clipError !== "")
            return pg.clipError;
        if (!pg.clipStats)
            return I18n.tr("Measuring what the history holds…");
        return I18n.tr("%1 items · text %2 · images %3")
            .arg(pg.clipStats.items)
            .arg(pg.humanBytes(pg.clipStats.textBytes))
            .arg(pg.humanBytes(pg.clipStats.imageBytes));
    }
    readonly property string weeklyDesc: {
        var when = "";
        if (pg.clipStats && pg.clipStats.lastPrune > 0) {
            var days = Math.floor((Date.now() / 1000 - pg.clipStats.lastPrune) / 86400);
            when = days <= 0
                ? I18n.tr(" Last run today.")
                : I18n.tr(" Last run %1 day(s) ago.").arg(days);
        } else {
            when = I18n.tr(" Has not run yet.");
        }
        return I18n.tr("Drops unstarred items once a week, without asking.") + when;
    }
    readonly property bool pruneWeekly: {
        Settings.revision;
        return Settings.get("clipboard.pruneWeekly") === true;
    }
    function setPruneWeekly(v) { Settings.patch("clipboard.pruneWeekly", v); }

    function loadStorage() {
        if (pg.clipBusy)
            return;
        pg.clipBusy = true;
        clipStatsProc.running = true;
    }
    function pruneHistory() {
        if (pg.clipBusy)
            return;
        pg.clipBusy = true;
        clipPruneProc.running = true;
    }

    Component.onCompleted: pg.loadStorage()

    // The numbers belong to the Clipboard tab; measuring them is cheap but there
    // is no reason to run it for a page the user is not looking at.
    Connections {
        target: sp
        function onTabChanged() { if (sp.tab === "Clipboard") pg.loadStorage(); }
    }

    Process {
        id: clipStatsProc
        command: ["ryoku-hub", "clipboard", "stats"]
        stdout: StdioCollector { id: clipStatsOut }
        stderr: StdioCollector { id: clipStatsErr }
        onExited: (code) => {
            pg.clipBusy = false;
            if (code !== 0) {
                pg.clipError = clipStatsErr.text.trim()
                    || I18n.tr("Couldn't read the clipboard size: the shell daemon is not answering.");
                return;
            }
            try {
                pg.clipStats = JSON.parse(clipStatsOut.text.trim());
                pg.clipError = "";
            } catch (e) {
                pg.clipError = I18n.tr("Couldn't read the clipboard size.");
            }
        }
    }

    // The prune prints the report it just refreshed, so the row updates from one
    // command instead of two racing ones.
    Process {
        id: clipPruneProc
        command: ["ryoku-hub", "clipboard", "prune"]
        stdout: StdioCollector { id: clipPruneOut }
        stderr: StdioCollector { id: clipPruneErr }
        onExited: (code) => {
            pg.clipBusy = false;
            if (code !== 0) {
                pg.clipError = clipPruneErr.text.trim()
                    || I18n.tr("Couldn't prune the clipboard history.");
                return;
            }
            try {
                pg.clipStats = JSON.parse(clipPruneOut.text.trim());
                pg.clipError = "";
            } catch (e) {
                pg.clipError = I18n.tr("Couldn't read the clipboard size.");
            }
        }
    }

    SchemaPage {
        id: sp
        anchors.fill: parent
        schema: DesktopSchema.rows
        draft: pg.hub ? pg.hub.draft : null
        defaults: pg.hub ? pg.hub.committed : ({})
        advanced: pg.hub ? pg.hub.advanced : false
        title: pg.pTitle
        eyebrow: pg.pEyebrow
        blurb: pg.pBlurb
        query: pg.hub ? pg.hub.query : ""
        externalReloadCoverError: pg.hub ? pg.hub.reloadCoverCleanupError : ""
        onEdited: (k, v) => { if (pg.hub) pg.hub.edit(k, v); }
        onPickRequested: (r) => { if (pg.hub) pg.hub.openPick(r); }

        // A live, self-contained preview of the desktop visualiser. It rides the
        // shared `extras` slot, so it sits full width between the tabs and the
        // STYLE/SPECTRUM/MOTION cards -- shown only on the Visualizer subtab and
        // folded flat (height 0) elsewhere, so the General tab is untouched.
        VizPreview {
            hub: pg.hub
            anchors.left: parent.left
            anchors.right: parent.right
            visible: sp.tab === "Visualizer"
        }

        // ── PICKERS: how the theme, wallpaper and media pickers are laid out.
        // Folded flat off the General subtab so the shared extras slot leaves no
        // gap on Visualizer, the way the visualiser preview folds off General.
        SettingCard {
            id: pickersCard
            // one card on the page's own measure: a lone control stretched across
            // the window leaves its right half empty
            anchors.left: parent.left
            width: sp.cardWidth
            title: I18n.tr("PICKERS")
            // the shared extras slot is a Column, which sizes itself from its
            // VISIBLE children: a height binding here would be the trap of
            // measuring the card by its own implicitHeight, and is not needed
            visible: sp.tab === "General"

            SettingRow {
                anchors.left: parent.left
                anchors.right: parent.right
                block: true
                label: I18n.tr("Picker style")
                desc: I18n.tr("How the theme, wallpaper and media pickers are laid out.")
                source: "shell.json"
                Seg {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    options: pg.pickerOptions
                    current: pg.pickerStyle
                    onChose: key => pg.setPickerStyle(key)
                }
            }
        }

        // ── CLIPBOARD STORAGE: what the history holds and how to take the space
        // back. The size is measured by the daemon (the component that owns the
        // entries and their files), so the number and the prune cannot disagree.
        SettingCard {
            id: storageCard
            anchors.left: parent.left
            width: sp.cardWidth
            title: I18n.tr("STORAGE")
            visible: sp.tab === "Clipboard"

            SettingRow {
                anchors.left: parent.left
                anchors.right: parent.right
                label: I18n.tr("History size")
                value: pg.storageSize
                desc: pg.storageDesc
                source: "clipboard history"
            }
            SettingRow {
                anchors.left: parent.left
                anchors.right: parent.right
                divider: true
                label: I18n.tr("Prune history")
                desc: I18n.tr("Drops every item you have not starred. Favorites are never pruned.")
                controlWidth: 92
                Btn {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("PRUNE")
                    armed: !pg.clipBusy
                    onAct: pg.pruneHistory()
                }
            }
            SettingRow {
                anchors.left: parent.left
                anchors.right: parent.right
                divider: true
                label: I18n.tr("Prune weekly")
                desc: pg.weeklyDesc
                source: "shell.json"
                controlWidth: 54
                Sw {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    on: pg.pruneWeekly
                    onToggled: (v) => pg.setPruneWeekly(v)
                }
            }
        }
    }

    Connections {
        target: pg.hub
        function onInvalidatePageWork() {
            sp.invalidateReloadCoverImport();
        }
    }
}

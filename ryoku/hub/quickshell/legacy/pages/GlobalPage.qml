import QtQuick
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import ".."
import "../schema/GlobalPage.js" as GlobalSchema
import "users" as Users

// Global: the cross-cutting preferences that are not tied to one surface:
// language and regional formats, people, fonts, app icons, window treatment
// and desktop sounds. The full account and sound blocks sit on their own tabs;
// schema rows keep using the shared settings sheet.
Item {
    id: pg
    property var hub

    readonly property string pTitle: I18n.tr("Global")
    readonly property string pEyebrow: I18n.tr("GLOBAL")
    readonly property string pBlurb: I18n.tr("Language, regional formats, people, fonts, icons, window blur and sounds.")
    readonly property string soundsTab: "Sounds"
    readonly property string usersTab: I18n.tr("Users")
    function focusKey(k) { sp.focusKey(k) }
    function openUsers(profileId) {
        sp.tab = pg.usersTab;
        if (profileId)
            usersPanel.openProfile(profileId);
    }
    function resetDefaults() {
        if (!pg.hub)
            return;
        for (var i = 0; i < pg.schema.length; i++) {
            var key = pg.schema[i].key;
            if (key && pg.hub.defs[key] !== undefined)
                pg.hub.edit(key, JSON.parse(JSON.stringify(pg.hub.defs[key])));
        }
    }

    // Installed font families, read live so both font drawers offer exactly what
    // this machine can render. Injected into the font rows' options so the shared
    // "pick" control shows the searchable font list.
    property var fontList: []
    // the live system time zone, injected into the timezone row and refreshed
    // after an apply.
    property string currentTimezone: ""
    readonly property var schema: {
        var out = [];
        for (var i = 0; i < GlobalSchema.rows.length; i++) {
            var r = GlobalSchema.rows[i];
            if (r.key === "fontFamily" || r.key === "terminalFontFamily" || r.key === "timezone") {
                var c = {};
                for (var k in r)
                    c[k] = r[k];
                if (r.key === "fontFamily" || r.key === "terminalFontFamily") c.opts = pg.fontList;
                else c.tzCurrent = pg.currentTimezone;
                out.push(c);
            } else {
                out.push(r);
            }
        }
        return out;
    }

    Process {
        id: fonts
        command: ["bash", "-c", "fc-list : family | cut -d, -f1 | sort -u"]
        stdout: StdioCollector {
            id: fontsOut
            onStreamFinished: {
                var t = ("" + fontsOut.text).trim();
                pg.fontList = t.length > 0 ? t.split("\n").filter(function (x) { return x.length > 0; }) : [];
            }
        }
    }

    function readTz() { tzRead.running = false; tzRead.running = true; }
    function applyTimezone(z) {
        if (!z) return;
        tzApply.command = ["ryoku-host", "time", "set-zone", z];
        tzApply.running = false;
        tzApply.running = true;
        tzMap.close();
    }
    // After the zone lands, refresh the shown value and restart the shell: a
    // long-running process caches its zone (glibc and the QML engine never
    // re-read /etc/localtime), so the bar, lockscreen and dashboard clocks only
    // pick up the change on a fresh process. The Hub runs as its own process, so
    // this does not close Settings.
    function applyDone() {
        pg.readTz();
        shellRestart.running = false;
        shellRestart.running = true;
    }
    Process {
        id: tzRead
        command: ["ryoku-host", "time", "zone"]
        stdout: StdioCollector { onStreamFinished: pg.currentTimezone = ("" + text).trim() }
    }
    Process {
        id: shellRestart
        command: ["ryoku-host", "svc", "restart", "ryoku-shell"]
        onExited: function(code) {
            if (code === 4)
                Bus.run("reload");
        }
    }
    Process {
        id: tzApply
        stdout: StdioCollector { onStreamFinished: pg.applyDone() }
        stderr: StdioCollector { }
    }
    Component.onCompleted: { fonts.running = true; pg.readTz(); }

    SchemaPage {
        id: sp
        anchors.fill: parent
        schema: pg.schema
        draft: pg.hub ? pg.hub.draft : null
        defaults: pg.hub ? pg.hub.committed : ({})
        advanced: pg.hub ? pg.hub.advanced : false
        title: pg.pTitle
        eyebrow: pg.pEyebrow
        blurb: pg.pBlurb
        query: pg.hub ? pg.hub.query : ""
        onEdited: (k, v) => { if (pg.hub) pg.hub.edit(k, v); }
        onPickRequested: (r) => { if (pg.hub) pg.hub.openPick(r); }
        onTimezonePickRequested: (r) => tzMap.open()
        // Sounds and user accounts are full blocks rather than schema rows, so
        // each gets a tab of its own.
        tailTabs: [pg.soundsTab, pg.usersTab]

        IconThemeSettings {
            id: iconThemes
            width: sp.cardWidth
            visible: sp.tab === "Region & fonts"
            onPickRequested: {
                iconPickerLayer.visible = true;
                iconPicker.open();
            }
        }

        SoundsSettings {
            width: parent.width
            visible: sp.tab === pg.soundsTab
        }

        Users.UsersTab {
            id: usersPanel
            width: parent.width
            visible: sp.tab === pg.usersTab
        }
    }

    Item {
        id: iconPickerLayer
        anchors.fill: parent
        visible: false
        z: 800

        Rectangle {
            anchors.fill: parent
            color: "black"
            opacity: 0.55
            TapHandler { onTapped: iconPickerLayer.visible = false }
        }
        Picker {
            id: iconPicker
            anchors.centerIn: parent
            title: I18n.tr("ICON THEME")
            options: iconThemes.options
            labels: iconThemes.labels
            current: iconThemes.current
            onChose: (theme) => {
                iconThemes.apply(theme);
                iconPickerLayer.visible = false;
            }
            onDismissed: iconPickerLayer.visible = false
        }
    }

    TimezoneMap {
        id: tzMap
        anchors.fill: parent
        currentZone: pg.currentTimezone
        onApplied: (z) => pg.applyTimezone(z)
        onCanceled: tzMap.close()
    }
}

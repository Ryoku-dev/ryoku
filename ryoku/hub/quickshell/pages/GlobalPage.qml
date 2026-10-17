import QtQuick
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import ".."
import "../schema/GlobalPage.js" as GlobalSchema

// Global: the cross-cutting preferences that are not tied to one surface -- the
// interface language, the regional formats used for dates and numbers, the
// machine location, the system font, and the icon theme. Rendered through the
// shared SchemaPage; the ledger and action bar belong to the shell.
Item {
    id: pg
    property var hub

    readonly property string pTitle: I18n.tr("Global")
    readonly property string pEyebrow: I18n.tr("GLOBAL")
    readonly property string pBlurb: I18n.tr("Language, regional formats, location, the system font and the icon theme.")
    function focusKey(k) { sp.focusKey(k) }

    // Installed font families, read live so the System font drawer offers exactly
    // what this machine can render. Injected into the fontFamily row's options,
    // so the shared "pick" control shows the searchable font list.
    property var fontList: []
    // Installed icon themes, enumerated by the backend (the same search paths the
    // cursor picker uses, cursor-only sets excluded); injected into the iconTheme
    // row's options the same way fontList is.
    property var iconList: []
    // the live system time zone, read via timedatectl and injected into the
    // timezone row (like fontList above); refreshed after an apply.
    property string currentTimezone: ""
    readonly property var schema: {
        var out = [];
        for (var i = 0; i < GlobalSchema.rows.length; i++) {
            var r = GlobalSchema.rows[i];
            if (r.key === "fontFamily" || r.key === "timezone" || r.key === "iconTheme") {
                var c = {};
                for (var k in r)
                    c[k] = r[k];
                if (r.key === "fontFamily") c.opts = pg.fontList;
                else if (r.key === "iconTheme") c.opts = pg.iconList;
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

    Process {
        id: iconEnum
        command: ["ryoku-hub", "desktop", "icons"]
        stdout: StdioCollector {
            id: iconEnumOut
            onStreamFinished: {
                try {
                    var l = JSON.parse(iconEnumOut.text);
                    pg.iconList = Array.isArray(l) ? l : [];
                } catch (e) {
                    pg.iconList = [];
                }
            }
        }
    }

    function readTz() { tzRead.running = false; tzRead.running = true; }
    function applyTimezone(z) {
        if (!z) return;
        tzApply.command = ["timedatectl", "set-timezone", z];
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
        command: ["timedatectl", "show", "-p", "Timezone", "--value"]
        stdout: StdioCollector { onStreamFinished: pg.currentTimezone = ("" + text).trim() }
    }
    Process {
        id: shellRestart
        command: ["systemctl", "--user", "restart", "ryoku-shell.service"]
    }
    // set-timezone talks to systemd-timedated over D-Bus; the shipped polkit
    // rule authorises it for the active user, so no password prompt.
    Process {
        id: tzApply
        stdout: StdioCollector { onStreamFinished: pg.applyDone() }
        stderr: StdioCollector { }
    }
    Component.onCompleted: { fonts.running = true; iconEnum.running = true; pg.readTz(); }

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
    }

    TimezoneMap {
        id: tzMap
        anchors.fill: parent
        currentZone: pg.currentTimezone
        onApplied: (z) => pg.applyTimezone(z)
        onCanceled: tzMap.close()
    }
}

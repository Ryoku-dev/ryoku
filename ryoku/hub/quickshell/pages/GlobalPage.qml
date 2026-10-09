import QtQuick
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import ".."
import "../schema/GlobalPage.js" as GlobalSchema

// Global: the cross-cutting preferences that are not tied to one surface -- the
// interface language, the regional formats used for dates and numbers, the
// machine location, and the system font. Rendered through the shared SchemaPage;
// the ledger and action bar belong to the shell.
Item {
    id: pg
    property var hub

    readonly property string pTitle: I18n.tr("Global")
    readonly property string pEyebrow: I18n.tr("GLOBAL")
    readonly property string pBlurb: I18n.tr("Language, regional formats, location and the system font.")
    function focusKey(k) { sp.focusKey(k) }

    // Installed font families, read live so the System font drawer offers exactly
    // what this machine can render. Injected into the fontFamily row's options,
    // so the shared "pick" control shows the searchable font list.
    property var fontList: []
    // the live system time zone, injected into the timezone row and refreshed
    // after an apply.
    property string currentTimezone: ""
    readonly property var schema: {
        var out = [];
        for (var i = 0; i < GlobalSchema.rows.length; i++) {
            var r = GlobalSchema.rows[i];
            if (r.key === "fontFamily" || r.key === "timezone") {
                var c = {};
                for (var k in r)
                    c[k] = r[k];
                if (r.key === "fontFamily") c.opts = pg.fontList;
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
        tzApply.command = ["sh", "-c",
            "if [ -d /run/systemd/system ]; then exec timedatectl set-timezone \"$1\"; fi; zone=$(readlink -f -- \"/usr/share/zoneinfo/$1\") || exit 2; case \"$zone\" in /usr/share/zoneinfo/*) ;; *) exit 2;; esac; [ -f \"$zone\" ] || exit 2; exec pkexec ln -sfn -- \"$zone\" /etc/localtime",
            "ryoku-timezone", z];
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
        command: ["sh", "-c",
            "if [ -d /run/systemd/system ]; then exec timedatectl show -p Timezone --value; fi; target=$(readlink -f /etc/localtime) || exit; case \"$target\" in /usr/share/zoneinfo/*) printf '%s\\n' \"${target#/usr/share/zoneinfo/}\";; esac"]
        stdout: StdioCollector { onStreamFinished: pg.currentTimezone = ("" + text).trim() }
    }
    Process {
        id: shellRestart
        command: ["sh", "-c",
            "if [ -d /run/systemd/system ]; then exec systemctl --user restart ryoku-shell.service; fi; service=\"$HOME/.config/service/ryoku-shell\"; if [ -d \"$service/supervise\" ]; then exec sv restart \"$service\"; fi; exec ryoku-shell reload"]
    }
    // timedatectl is authorised by the shipped polkit rule. Void uses pkexec
    // because it has no timedated service.
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
    }

    TimezoneMap {
        id: tzMap
        anchors.fill: parent
        currentZone: pg.currentTimezone
        onApplied: (z) => pg.applyTimezone(z)
        onCanceled: tzMap.close()
    }
}

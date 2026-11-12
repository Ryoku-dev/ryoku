import QtQuick
import Ryoku.Ui.Singletons
import "../../Singletons"

// the icon theme as a Changes store (the backend applies it to every toolkit),
// plus the font, icon and pointer catalogues, read once per session (again on Try again)
QtObject {
    id: store

    // every read and write gives up after this long, so nothing waits forever
    readonly property int deadline: 10000

    property var iconTheme: undefined
    property bool applying: false
    // [{ value, label }] in display order
    property var iconThemes: []
    property var pointerThemes: []
    property var fonts: []
    property var monoFonts: []
    // why a catalogue could not be read, in plain words; "" while it reads or once it has
    property string iconsFailure: ""
    property string pointersFailure: ""
    property string fontsFailure: ""

    function read(key) {
        return key === "iconTheme" ? store.iconTheme : undefined;
    }

    function write(key, v, done) {
        if (key !== "iconTheme") {
            done(false, "");
            return;
        }
        var before = store.iconTheme;
        store.iconTheme = String(v);
        store.applying = true;
        Changes.touch();
        Changes.run(["ryoku-hub", "desktop", "icon-theme", "set", String(v)], function (ok, out, err, code) {
            store.applying = false;
            // the control shows what really applied, even when the re-read cannot run
            if (!ok) {
                store.iconTheme = before;
                Changes.touch();
            }
            done(ok, ok ? "" : code === -1 ? err : I18n.tr("The icon theme did not apply. Check that it is still installed, then try again."));
            store.readCurrent();
        }, { timeout: store.deadline });
    }

    // a re-read that cannot run (missing program, timeout) keeps the theme last known,
    // so a failed apply never blanks the field
    function readCurrent() {
        Changes.run(["ryoku-hub", "desktop", "icon-theme"], function (ok, out) {
            var theme;
            try {
                theme = ok ? String(JSON.parse(out).theme || "") : undefined;
            } catch (e) {
                theme = undefined;
            }
            if (theme === undefined && store.iconTheme !== undefined)
                return;
            store.iconTheme = theme === undefined ? "" : theme;
            Changes.touch();
        }, { timeout: store.deadline });
    }

    // a catalogue that could not be read says why: the run's own words for a missing
    // program or a timeout, else what to do about it
    function failure(code, err, words) {
        return code === -1 ? err : words;
    }

    function readIcons() {
        store.iconsFailure = "";
        Changes.run(["ryoku-hub", "desktop", "icons"], function (ok, out, err, code) {
            try {
                if (!ok)
                    throw new Error(err);
                var rows = JSON.parse(out);
                store.iconThemes = (Array.isArray(rows) ? rows : [])
                    .filter(r => r && r.code)
                    .map(r => ({ value: String(r.code), label: String(r.name || r.code) }));
            } catch (e) {
                store.iconsFailure = store.failure(ok ? 1 : code, err, I18n.tr("The installed icon themes could not be read. Run ryoku doctor in a terminal, then try again."));
            }
        }, { timeout: store.deadline });
    }

    function readPointers() {
        store.pointersFailure = "";
        Changes.run(["ryoku-hub", "desktop", "cursors"], function (ok, out, err, code) {
            try {
                if (!ok)
                    throw new Error(err);
                var names = JSON.parse(out);
                store.pointerThemes = (Array.isArray(names) ? names : [])
                    .map(n => ({ value: String(n), label: String(n).replace(/[-_]+/g, " ") }));
            } catch (e) {
                store.pointersFailure = store.failure(ok ? 1 : code, err, I18n.tr("The installed pointer styles could not be read. Run ryoku doctor in a terminal, then try again."));
            }
        }, { timeout: store.deadline });
    }

    function readFonts() {
        store.fontsFailure = "";
        Changes.run(["fc-list", "--format", "%{family[0]}\t%{spacing}\n"], function (ok, out, err, code) {
            if (ok)
                store.setFonts(out);
            else
                store.fontsFailure = store.failure(code, err, I18n.tr("The installed fonts could not be read. Run ryoku doctor in a terminal, then try again."));
        }, { timeout: store.deadline });
    }

    function family(line) {
        return String(line || "").trim();
    }

    // symbol and emoji faces are not text
    function textFace(name) {
        return name.length > 0 && !/emoji|symbols|signwriting/i.test(name);
    }

    function setFonts(text) {
        var all = {}, mono = {};
        String(text || "").split("\n").forEach(function (line) {
            var tab = line.indexOf("\t");
            var name = store.family(tab >= 0 ? line.slice(0, tab) : line);
            if (!store.textFace(name))
                return;
            all[name] = true;
            var spacing = tab >= 0 ? Number(line.slice(tab + 1)) : 0;
            if (spacing >= 90)
                mono[name] = true;
        });
        var byName = (a, b) => a.toLowerCase() < b.toLowerCase() ? -1 : a.toLowerCase() > b.toLowerCase() ? 1 : 0;
        store.fonts = Object.keys(all).sort(byName).map(n => ({ value: n, label: n }));
        store.monoFonts = Object.keys(mono).sort(byName).map(n => ({ value: n, label: n }));
    }

    Component.onCompleted: {
        Changes.registerStore("fonts-icons", store);
        store.readCurrent();
        store.readIcons();
        store.readPointers();
        store.readFonts();
    }
}

pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

// The one translation lookup, shared by every Ryoku surface (Hub, shell, apps,
// the wallpaper picker, the greeter). Source-string-as-key:
// `I18n.tr("Power limit")` returns the current language's string, or the English
// key itself when there is no translation, so the UI is never broken and
// translating is purely additive. English is the source of truth in code; the
// per-language files are generated (see ryoku/i18n/sync.py), so a developer only
// ever writes English.
//
// The language is a global shell setting (shell.json "language"), set from the
// Hub's Global page and watched here, so changing it retranslates every open
// surface live -- no relogin. The same catalog serves the Go installers and the
// installer's shell (ryoku/i18n/i18n.go, installation/backend/lib/i18n.sh), so
// there is one set of strings for the whole system, at /usr/share/ryoku/i18n
// (a dev checkout points RYOKU_I18N_DIR at ryoku/i18n/catalog). Brand kana
// (力, 描画, seals) are never wrapped, so they stay put.
Singleton {
    id: i18n

    // the language table, langs.json, shipped next to the catalogs. It is the
    // same file sync.py, the Hub's picker and the Go runtime read, so a new
    // language is added in exactly one place.
    property var langs: []
    // shell.json stores a human-readable choice ("Polski"); this maps any of
    // the table's spellings -- code, English name, native name -- to a file code.
    readonly property var names: {
        var m = { "Auto": "auto" };
        for (var i = 0; i < i18n.langs.length; i++) {
            var l = i18n.langs[i];
            m[l.code] = l.code;
            m[l.name] = l.code;
            m[l.native] = l.code;
        }
        return m;
    }

    property string configLang: "auto"     // raw value from shell.json (name or code)
    readonly property string lang: {
        var sel = i18n.names[i18n.configLang] || i18n.configLang;   // name -> code, else raw
        if (sel && sel !== "auto")
            return sel;
        var n = Qt.locale().name;           // es_ES, pt_BR, pt_PT, fr_FR, en_US, ...
        if (i18n.names[n])
            return i18n.names[n];           // an exact regional catalog (pt_BR)
        var base = n.split("_")[0];
        return i18n.names[base] || "en";
    }

    // text direction. Arabic, Hebrew and Persian read right to left, so a
    // surface roots its layout on this and Qt mirrors anchors, rows and text
    // alignment for everything below it.
    readonly property bool rtl: {
        for (var i = 0; i < i18n.langs.length; i++)
            if (i18n.langs[i].code === i18n.lang)
                return i18n.langs[i].dir === "rtl";
        return false;
    }
    readonly property int dir: i18n.rtl ? Qt.RightToLeft : Qt.LeftToRight

    // What a language picker shows. "Auto" first, then every shipped language
    // under its own name, so someone who cannot read the current UI language
    // still recognises theirs. The stored value is the code, and the shell's
    // older human-name values ("Español") keep resolving through `names`.
    readonly property var pickerOptions: {
        var out = ["auto"];
        for (var i = 0; i < i18n.langs.length; i++)
            out.push(i18n.langs[i].code);
        return out;
    }
    readonly property var pickerLabels: {
        var m = { "auto": "Auto (system locale)" };
        for (var i = 0; i < i18n.langs.length; i++) {
            var l = i18n.langs[i];
            m[l.code] = l.native === l.name ? l.native : l.native + "  " + l.name;
        }
        return m;
    }
    // The regional-format choices, from the same table: every locale a shipped
    // language implies, plus the two English variants people actually pick.
    readonly property var localeOptions: {
        var out = ["", "en_US", "en_GB"];
        for (var i = 0; i < i18n.langs.length; i++) {
            var code = i18n.langs[i].locale.split(".")[0];
            if (code && out.indexOf(code) < 0)
                out.push(code);
        }
        return out;
    }

    property var map: ({})       // shipped translations
    property var genMap: ({})    // user/AI-generated, layered on top

    // Where the shipped catalog lives, most specific first: an explicit
    // override (tests, a CI run), a dev checkout's `ryoku/i18n/install.sh`
    // drop, then the packaged location every runtime shares. langs.json sits in
    // the same directory, so probing for it settles the catalog too: a failed
    // load steps to the next candidate rather than needing a stat call QML
    // cannot make.
    readonly property var catalogDirs: [
        Quickshell.env("RYOKU_I18N_DIR") || "",
        (Quickshell.env("XDG_DATA_HOME") || (Quickshell.env("HOME") + "/.local/share")) + "/ryoku/i18n",
        "/usr/share/ryoku/i18n"
    ].filter((d) => d !== "")
    property int dirIndex: 0
    readonly property string catalogDir: i18n.catalogDirs[Math.min(i18n.dirIndex, i18n.catalogDirs.length - 1)]
    function _trPath(l) {
        return i18n.catalogDir + "/" + l + ".json";
    }
    // user/AI-generated translations live in the config dir, so a language the
    // shell did not ship (or a better LLM pass) can be dropped in and layered.
    function _genPath(l) {
        return (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/i18n/" + l + ".json";
    }

    // the global config; language lives under "language", watched for live switch.
    FileView {
        id: cfg
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/shell.json"
        blockLoading: true
        watchChanges: true
        atomicWrites: true
        printErrors: false
        onFileChanged: { reload(); i18n._loadCfg(); }
        onLoadFailed: i18n.configLang = "auto"
    }

    // the language table. Loaded before anything else needs it (blockLoading),
    // so `lang` resolves against the real list on the first evaluation.
    FileView {
        id: table
        path: i18n.catalogDir + "/langs.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onLoaded: i18n._loadLangs()
        onFileChanged: { reload(); i18n._loadLangs(); }
        // not here: try the next candidate directory, and only give up (empty
        // table, English UI) once the list is exhausted.
        onLoadFailed: {
            // The path rebind after stepping does not reliably re-emit
            // loaded/loadFailed, so the next candidate has to force its own
            // read once the binding settles. Without this the step never
            // happens on a packaged install (no user drop), /usr/share/ryoku/
            // i18n is never reached, and the picker lists only "Auto" (#241).
            if (i18n.dirIndex < i18n.catalogDirs.length - 1) {
                i18n.dirIndex = i18n.dirIndex + 1;
                Qt.callLater(i18n._loadLangs);
            } else
                i18n.langs = [];
        }
    }

    // the active language's string map; path rebinds when `lang` changes.
    FileView {
        id: tf
        path: i18n._trPath(i18n.lang)
        blockLoading: true
        watchChanges: true
        printErrors: false
        onLoaded: i18n._loadMap()
        onFileChanged: { reload(); i18n._loadMap(); }
        onLoadFailed: i18n.map = ({})      // no file (e.g. English) -> keys are the strings
    }

    // the user/AI-generated overlay for the active language; empty when absent.
    FileView {
        id: gen
        path: i18n._genPath(i18n.lang)
        blockLoading: true
        watchChanges: true
        printErrors: false
        onLoaded: i18n._loadGen()
        onFileChanged: { reload(); i18n._loadGen(); }
        onLoadFailed: i18n.genMap = ({})
    }

    Component.onCompleted: { i18n._loadLangs(); i18n._loadCfg(); }

    function _loadLangs() {
        try {
            var t = table.text();
            i18n.langs = t ? ((JSON.parse(t) || {}).languages || []) : [];
        } catch (e) {
            i18n.langs = [];
        }
    }
    function _loadCfg() {
        try {
            i18n.configLang = (JSON.parse(cfg.text()) || {}).language || "auto";
        } catch (e) {
            i18n.configLang = "auto";
        }
    }
    function _loadMap() {
        try {
            var t = tf.text();
            i18n.map = t ? (JSON.parse(t) || {}) : ({});
        } catch (e) {
            i18n.map = ({});
        }
    }
    function _loadGen() {
        try {
            var t = gen.text();
            i18n.genMap = t ? (JSON.parse(t) || {}) : ({});
        } catch (e) {
            i18n.genMap = ({});
        }
    }

    // the lookup. English key in, translated (or the key) out. A user/AI-generated
    // string wins over the shipped one, which wins over the English key.
    function tr(s) {
        if (s === undefined || s === null || s === "")
            return s;
        var k = "" + s;
        var g = i18n.genMap[k];
        if (g !== undefined && g !== "")
            return g;
        var v = i18n.map[k];
        return v === undefined ? s : v;
    }
}

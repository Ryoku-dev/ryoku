pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Fastfetch (DESIGN.md section 8, DESKTOP). The branded terminal readout, edited
// end to end. config.jsonc stays the source of truth (the head's EDIT CONFIG
// opens it raw); this reads it via `ryoku-hub fastfetch get`, edits a model,
// live-previews the readout via `ryoku-hub fastfetch preview <json>` (parsing its
// truecolor SGR into coloured runs), and writes back with `fastfetch save <json>`
// on Save. The emblem takes an image (an SVG rasterized on import), an ASCII art
// file, a built-in, or none; info rows toggle, reorder, and rename.
//
// Full-bleed: this page owns the whole content region, so it draws its own head,
// its inline live preview, its controls, and -- because the shell hides its
// global action bar -- its own Save/Revert bar. The backend is fastfetch's own
// config, not the shared settings store; nothing writes to disk until Save.
// Monochrome throughout (every value reads from Tokens) with one exception: the
// preview renders the real readout in its own colours, because that panel is a
// specimen of what the terminal will actually show (section 1, colour is data).
Item {
    id: pg

    // The shell injects the real hub; a bare probe object may be handed in
    // instead. Fastfetch owns its own backend, so hub is only the fullBleed flag.
    property var hub
    readonly property bool fullBleed: true

    property var model: ({ "logo": { "kind": "none", "source": "", "width": 28, "height": 14, "padding": 3, "paddingRight": 5, "paddingTop": 5 }, "accent": "226;52;42", "palette": "fixed", "rows": [] })
    property var committed: ({})
    property bool ready: false
    property int rev: 0
    // which overlay is open: the "add a row" catalogue.
    property bool addOpen: false
    property var installedStoreStyles: []
    property bool storeStyleOpen: false
    property bool storeStyleRemoveOpen: false
    property string storeStyleError: ""

    function storeStyleLabels() {
        return pg.installedStoreStyles.map(function (item) { return item.name; });
    }
    function applyStoreStyle(label) {
        for (var i = 0; i < pg.installedStoreStyles.length; i++) {
            var item = pg.installedStoreStyles[i];
            if (item.name === label) {
                pg.storeStyleError = "";
                applyStoreStyleProc.command = ["ryostore", "internal", "apply-fastfetch", item.id];
                applyStoreStyleProc.running = true;
                return;
            }
        }
    }
    // `ryostore remove` takes the category and id positionally, not as a flag.
    function removeStoreStyle(label) {
        for (var i = 0; i < pg.installedStoreStyles.length; i++) {
            var item = pg.installedStoreStyles[i];
            if (item.name === label) {
                pg.storeStyleError = "";
                removeStoreStyleProc.command = ["ryostore", "remove", "fastfetch", item.id];
                removeStoreStyleProc.running = true;
                return;
            }
        }
    }

    // A store-installed emblem is one flat PNG in ~/Pictures/ryoemblems, so its
    // id is the file's stem and removing it is the store's own remove. The
    // repoint happens after the file is gone, not before: a commit racing the
    // delete let the reload read the dangling source, and a missing source
    // normalises to the builtin logo, which is not what anyone asked for.
    property string repointAfterRemove: ""
    function removeInstalledEmblem(path) {
        var file = String(path).split("/").pop();
        var id = file.replace(/\.png$/i, "");
        if (id.length === 0)
            return;
        pg.storeStyleError = "";
        pg.repointAfterRemove =
            String(pg.model.logo.source).indexOf("/ryoemblems/" + file) >= 0
                ? pg.brandArt[0].path : "";
        removeEmblemProc.command = ["ryostore", "remove", "fastfetch-emblems", id];
        removeEmblemProc.running = true;
    }
    function resetToDefault() { resetProc.running = true; }

    readonly property bool dirty: {
        void pg.rev;
        return pg.ready && JSON.stringify(pg.model) !== JSON.stringify(pg.committed);
    }

    // fastfetch's own config path; the head's EDIT CONFIG opens it raw, the same
    // file the Hub writes on Save so hand-edits and the GUI share one source.
    readonly property string configPath: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/fastfetch/config.jsonc"
    function browseFastfetch() {
        Quickshell.execDetached(["ryostore", "open", "fastfetch"]);
    }

    // info modules the Add menu offers, beyond the brand lines already present.
    readonly property var catalog: [
        { "key": "cpu", "label": "CPU" }, { "key": "gpu", "label": "GPU" },
        { "key": "memory", "label": "Memory" }, { "key": "disk", "label": "Disk" },
        { "key": "kernel", "label": "Kernel" }, { "key": "os", "label": "OS" },
        { "key": "host", "label": "Host" }, { "key": "uptime", "label": "Uptime" },
        { "key": "packages", "label": "Packages" }, { "key": "shell", "label": "Shell" },
        { "key": "terminal", "label": "Terminal" }, { "key": "wm", "label": "WM" },
        { "key": "de", "label": "Desktop" }, { "key": "battery", "label": "Battery" },
        { "key": "localip", "label": "Local IP" }, { "key": "board", "label": "Board" }
    ]
    readonly property var addOptions: {
        var out = [{ "key": "__header", "label": "Section header" }, { "key": "__break", "label": "Spacer" }, { "key": "__colors", "label": "Colour swatches" }];
        for (var i = 0; i < pg.catalog.length; i++)
            out.push({ "key": pg.catalog[i].key, "label": pg.catalog[i].label, "hint": "module" });
        return out;
    }
    function addLabels() { return pg.addOptions.map(function (o) { return o.label; }); }
    function addFromLabel(label) {
        for (var i = 0; i < pg.addOptions.length; i++)
            if (pg.addOptions[i].label === label) {
                pg.addFromMenu(pg.addOptions[i].key);
                return;
            }
    }

    // ---- load ---------------------------------------------------------------
    Process {
        id: getProc
        command: ["ryoku-hub", "fastfetch", "get"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    pg.model = JSON.parse(this.text);
                    pg.committed = JSON.parse(this.text);
                    if (!pg.model.rows)
                        pg.model.rows = [];
                    pg.ready = true;
                    pg.rev++;
                    pg.queuePreview();
                } catch (e) {
                    console.log("fastfetch get parse failed: " + e);
                }
            }
        }
    }

    Process {
        id: storeStylesProc
        command: ["ryostore", "catalog", "--category", "fastfetch"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var catalogue = JSON.parse(this.text);
                    pg.installedStoreStyles = catalogue.items.filter(function (item) { return item.installed; });
                } catch (e) {
                    pg.installedStoreStyles = [];
                }
            }
        }
    }
    Process {
        id: applyStoreStyleProc
        stderr: StdioCollector { id: applyStoreStyleError }
        onExited: function (code) {
            if (code !== 0) {
                pg.storeStyleError = applyStoreStyleError.text.trim() || I18n.tr("Couldn't apply the Store style.");
                return;
            }
            getProc.running = true;
            storeStylesProc.running = true;
        }
    }
    // removal only deletes the store copy; the applied config.jsonc is inlined on
    // apply, so it stays intact. Refresh both, like the apply path.
    Process {
        id: removeStoreStyleProc
        stderr: StdioCollector { id: removeStoreStyleError }
        onExited: function (code) {
            if (code !== 0) {
                pg.storeStyleError = removeStoreStyleError.text.trim() || I18n.tr("Couldn't remove the Store style.");
                return;
            }
            getProc.running = true;
            storeStylesProc.running = true;
        }
    }
    Process {
        id: removeEmblemProc
        stderr: StdioCollector { id: removeEmblemError }
        onExited: function (code) {
            if (code !== 0) {
                pg.storeStyleError = removeEmblemError.text.trim() || I18n.tr("Couldn't remove the emblem.");
                pg.repointAfterRemove = "";
                return;
            }
            emblemScan.running = true;
            if (pg.repointAfterRemove.length > 0) {
                // pickArt commits, and the commit's own reload replaces getProc here
                pg.pickArt(pg.repointAfterRemove);
                pg.repointAfterRemove = "";
                return;
            }
            getProc.running = true;
        }
    }
    Process {
        id: resetProc
        command: ["ryoku-hub", "fastfetch", "reset"]
        onExited: function (code) { if (code === 0) getProc.running = true; }
    }
    // ---- edit helpers (reassign model so bindings + dirty re-evaluate) -------
    function clone() { return JSON.parse(JSON.stringify(pg.model)); }
    function commitModel(m) { pg.model = m; pg.rev++; pg.queuePreview(); }
    function setLogo(k, v) { var m = pg.clone(); m.logo[k] = v; pg.commitModel(m); }
    function setAccent(v) { var m = pg.clone(); m.accent = v; pg.commitModel(m); }
    function setPalette(v) { var m = pg.clone(); m.palette = v; pg.commitModel(m); }
    function setRow(i, k, v) { var m = pg.clone(); m.rows[i][k] = v; pg.commitModel(m); }
    function moveRow(i, d) {
        var j = i + d;
        if (j < 0 || j >= pg.model.rows.length)
            return;
        var m = pg.clone();
        var t = m.rows[i]; m.rows[i] = m.rows[j]; m.rows[j] = t;
        pg.commitModel(m);
    }
    function removeRow(i) { var m = pg.clone(); m.rows.splice(i, 1); pg.commitModel(m); }
    function addFromMenu(key) {
        var m = pg.clone();
        if (key === "__header")
            m.rows.push({ "kind": "header", "enabled": true, "text": "SECTION", "label": "Section header" });
        else if (key === "__break")
            m.rows.push({ "kind": "break", "enabled": true, "label": "Spacer" });
        else if (key === "__colors")
            m.rows.push({ "kind": "colors", "enabled": true, "label": "Colour swatches", "raw": { "type": "colors", "symbol": "circle" } });
        else {
            var lbl = key.toUpperCase();
            for (var i = 0; i < pg.catalog.length; i++)
                if (pg.catalog[i].key === key) lbl = pg.catalog[i].label.toUpperCase();
            m.rows.push({ "kind": "module", "enabled": true, "module": key, "key": lbl, "label": lbl });
        }
        pg.commitModel(m);
    }

    function accentHex() {
        var p = String(pg.model.accent || "226;52;42").split(";");
        function h(n) { return ("0" + (parseInt(n) || 0).toString(16)).slice(-2); }
        return "#" + h(p[0]) + h(p[1]) + h(p[2]);
    }
    function hexToTriple(hex) {
        var c = String(hex);
        if (c.charAt(0) === "#") c = c.slice(1);
        return parseInt(c.substr(0, 2), 16) + ";" + parseInt(c.substr(2, 2), 16) + ";" + parseInt(c.substr(4, 2), 16);
    }
    function rowEditable(kind) { return kind === "tagline" || kind === "header" || kind === "module"; }
    function rowPlaceholder(kind) { return kind === "module" ? I18n.tr("LABEL") : I18n.tr("text"); }

    // an image logo source may carry a leading ~; expand it so the real emblem
    // renders as its own specimen instead of a broken tile.
    function logoUrl(src) {
        if (!src || src.length === 0)
            return "";
        var s = String(src);
        if (s.indexOf("file:") === 0)
            return s;
        if (s.charAt(0) === "~")
            s = Quickshell.env("HOME") + s.slice(1);
        return "file://" + s;
    }

    // ready-made emblems for the picker: the Ryoku brand marks, the shipped
    // ryodecors (our decor), and whatever the store has installed into
    // ryoemblems. Picking one sets the source directly (no copy); "Your image"
    // imports a file since it can live anywhere. Paths keep a leading ~ so the
    // config stays portable.
    property var installedArt: []
    readonly property var brandArt: [
        { "name": "RYOKU", "path": "~/.local/share/ryoku/assets/brand/fastfetch-emblem.png" },
        { "name": "MARK", "path": "~/.local/share/ryoku/assets/brand/logo-mark.png" },
        { "name": "LAOCOON", "path": "~/Pictures/ryodecors/laocoon.png" },
        { "name": "DAVID", "path": "~/Pictures/ryodecors/david.png" },
        { "name": "AURELIUS", "path": "~/Pictures/ryodecors/aurelius.png" },
        { "name": "HAWK", "path": "~/Pictures/ryodecors/hawk.png" },
        { "name": "KATANA", "path": "~/Pictures/ryodecors/katana.png" },
        { "name": "MOON", "path": "~/Pictures/ryodecors/moon.png" },
        { "name": "NEEDLE", "path": "~/Pictures/ryodecors/needle.png" },
        { "name": "MIC", "path": "~/Pictures/ryodecors/mic.png" },
        { "name": "CAMERA", "path": "~/Pictures/ryodecors/camera.png" },
        { "name": "RASHIN", "path": "~/Pictures/ryodecors/rashin-hero.png" }
    ]
    readonly property var readyArt: pg.brandArt.concat(pg.installedArt)

    // Emblems installed from the store land in ~/Pictures/ryoemblems as one flat
    // PNG each, so listing that folder is the whole discovery step.
    Process {
        id: emblemScan
        running: true
        command: ["sh", "-c", "ls -1 \"$HOME/Pictures/ryoemblems\" 2>/dev/null | grep -i '\\.png$' || true"]
        stdout: StdioCollector {
            onStreamFinished: {
                var out = [];
                var lines = String(this.text || "").split("\n");
                for (var i = 0; i < lines.length; i++) {
                    var file = lines[i].trim();
                    if (file.length === 0)
                        continue;
                    var stem = file.replace(/\.png$/i, "");
                    out.push({ "name": stem.replace(/-/g, " ").toUpperCase(),
                               "path": "~/Pictures/ryoemblems/" + file });
                }
                pg.installedArt = out;
            }
        }
    }
    // pick a ready emblem: set the source straight to its stable path.
    function pickArt(path) {
        var m = pg.clone();
        m.logo.source = path;
        m.logo.kind = "image";
        m.logo.dither = false; // a freshly picked emblem is the original, not a bake
        pg.commitModel(m);
    }
    // a dropped file arrives as a file:// URL; import expects a plain path.
    function stripFile(u) {
        var s = String(u);
        if (s.indexOf("file://") === 0)
            s = s.slice(7);
        return decodeURIComponent(s);
    }

    // mono cell metrics for the readout: the emblem is measured in these same
    // cells, so its size relative to the text is a true specimen of the terminal.
    FontMetrics { id: monoFM; font.family: Tokens.mono; font.pixelSize: 12 }
    readonly property real cellW: monoFM.averageCharacterWidth
    readonly property real cellH: monoFM.height
    // the emblem's other two paddings. They are not exposed as controls, but the
    // preview has to honour them or it stops being a specimen: fastfetch keeps
    // padding.right cells between emblem and readout, and drops the emblem
    // padding.top rows without moving the text.
    readonly property int padRight: pg.model.logo.paddingRight === undefined ? 5 : pg.model.logo.paddingRight
    readonly property int padTop: pg.model.logo.paddingTop === undefined ? 5 : pg.model.logo.paddingTop

    // a hidden probe reads the chosen art's natural aspect so Auto fit can size
    // the cell box to the image and it renders undistorted.
    Image {
        id: emblemProbe
        visible: false
        asynchronous: true
        source: (pg.model.logo.kind === "image" && pg.model.logo.source.length > 0) ? pg.logoUrl(pg.model.logo.source) : ""
    }
    readonly property real emblemAspect: (emblemProbe.status === Image.Ready && emblemProbe.implicitHeight > 0) ? (emblemProbe.implicitWidth / emblemProbe.implicitHeight) : 1

    // Auto fit: hold the width, set the height so the cell box matches the art's
    // aspect (undistorted), using the terminal's ~1:2 cell ratio from the mono
    // font. Square art lands near width/2, the fastfetch default shape.
    function autoFit() {
        if (pg.cellH <= 0 || pg.emblemAspect <= 0)
            return;
        var w = pg.model.logo.width > 0 ? pg.model.logo.width : 28;
        var h = Math.round(w * (pg.cellW / pg.cellH) / pg.emblemAspect);
        h = Math.max(1, Math.min(60, h));
        var m = pg.clone();
        m.logo.width = w;
        m.logo.height = h;
        pg.commitModel(m);
    }

    // ---- live preview (throttled) -------------------------------------------
    property var previewLines: []
    property bool previewPending: false
    Timer {
        id: throttle
        interval: 90
        onTriggered: {
            if (pg.previewPending) {
                pg.previewPending = false;
                pg.previewNow();
                throttle.restart();
            }
        }
    }
    function queuePreview() {
        if (throttle.running)
            pg.previewPending = true;
        else {
            pg.previewNow();
            throttle.start();
        }
    }
    function previewNow() {
        previewProc.command = ["ryoku-hub", "fastfetch", "preview", JSON.stringify(pg.model)];
        previewProc.running = true;
    }
    Process {
        id: previewProc
        stdout: StdioCollector { onStreamFinished: pg.previewLines = pg.parseAnsi(this.text) }
    }

    // fastfetch SGR (truecolor) -> lines of {text,color,bold} runs. mono font in
    // the view keeps column alignment; only 38;2;r;g;b, bold (1) and reset (0)
    // appear in the readout. Reset returns to bone ink -- the readout's base text
    // colour on our black paper -- while the coloured runs keep their own hue, so
    // the panel reads as a true specimen of the terminal output.
    function parseAnsi(s) {
        var lines = [];
        var raw = String(s || "").split("\n");
        for (var li = 0; li < raw.length; li++) {
            var line = raw[li], runs = [], color = Tokens.ink, bold = false, buf = "", i = 0;
            while (i < line.length) {
                if (line.charCodeAt(i) === 27 && line.charAt(i + 1) === "[") {
                    if (buf.length) { runs.push({ "text": buf, "color": color, "bold": bold }); buf = ""; }
                    var j = line.indexOf("m", i);
                    if (j < 0) break;
                    var codes = line.substring(i + 2, j).split(";");
                    for (var k = 0; k < codes.length; k++) {
                        if (codes[k] === "0" || codes[k] === "") { color = Tokens.ink; bold = false; }
                        else if (codes[k] === "1") bold = true;
                        else if (codes[k] === "38" && codes[k + 1] === "2") {
                            color = Qt.rgba((parseInt(codes[k + 2]) || 0) / 255, (parseInt(codes[k + 3]) || 0) / 255, (parseInt(codes[k + 4]) || 0) / 255, 1);
                            k += 4;
                        }
                    }
                    i = j + 1;
                } else { buf += line.charAt(i); i++; }
            }
            if (buf.length) runs.push({ "text": buf, "color": color, "bold": bold });
            lines.push(runs);
        }
        return lines;
    }

    // ---- save / revert ------------------------------------------------------
    Process {
        id: saveProc
        property var pending: ({})
        stderr: StdioCollector { id: saveError }
        onExited: function (code) {
            if (code !== 0) {
                pg.storeStyleError = saveError.text.trim() || I18n.tr("Couldn't save the Fastfetch settings.");
                return;
            }
            pg.committed = JSON.parse(JSON.stringify(saveProc.pending));
            pg.rev++;
        }
    }
    function save() {
        if (saveProc.running)
            return;
        throttle.stop();
        pg.previewPending = false;
        pg.storeStyleError = "";
        saveProc.pending = pg.clone();
        saveProc.command = ["ryoku-hub", "fastfetch", "save", JSON.stringify(saveProc.pending)];
        saveProc.running = true;
    }
    function revert() {
        throttle.stop();
        pg.previewPending = false;
        pg.model = JSON.parse(JSON.stringify(pg.committed));
        pg.rev++;
        pg.queuePreview();
    }
    Component.onDestruction: {
        // leaving with unsaved edits: nothing to undo on the live system (config
        // is only written on Save), so just stop pending previews.
        throttle.stop();
    }

    // ---- logo import --------------------------------------------------------
    Process {
        id: importProc
        property string pendingKind: "image"
        stdout: StdioCollector {
            onStreamFinished: {
                var p = String(this.text).trim();
                if (p.length) {
                    var m = pg.clone();
                    m.logo.source = p;
                    m.logo.kind = importProc.pendingKind;
                    m.logo.dither = false; // a freshly imported emblem is the original
                    pg.commitModel(m);
                }
            }
        }
    }
    function importLogo(path, kind) {
        importProc.pendingKind = kind;
        importProc.command = ["ryoku-hub", "fastfetch", "import-logo", path];
        importProc.running = true;
    }
    function openLogoFile() {
        var s = pg.model.logo.source;
        if (!s || s.length === 0)
            return;
        Spawn.run(["kitty", "-e", "nvim", s]);
    }
    function previewInTerminal() { Spawn.run(["kitty", "-e", "sh", "-c", "ryoku-fastfetch; read -n1"]); }
    function openConfig() { Spawn.run(["kitty", "-e", "nvim", "-O", pg.configPath]); }

    // ---- emblem dither + imported-emblem removal ----------------------------
    // the 1-bit bake toggles through the backend so it can read/write the sibling
    // file; the returned source (baked, or the restored original) flows back into
    // the model like an import. Reuses storeStyleError as the page's error line.
    Process {
        id: ditherProc
        property bool pendingOn: false
        stderr: StdioCollector { id: ditherError }
        stdout: StdioCollector {
            onStreamFinished: {
                var p = String(this.text).trim();
                if (p.length) {
                    var m = pg.clone();
                    m.logo.source = p;
                    m.logo.dither = ditherProc.pendingOn;
                    pg.commitModel(m);
                }
            }
        }
        onExited: function (code) {
            if (code !== 0)
                pg.storeStyleError = ditherError.text.trim() || I18n.tr("Couldn't change the emblem dither.");
        }
    }
    function setDither(v) {
        pg.storeStyleError = "";
        ditherProc.pendingOn = v;
        ditherProc.command = ["ryoku-hub", "fastfetch", "dither-logo", v ? "on" : "off", pg.model.logo.source];
        ditherProc.running = true;
    }
    // an imported emblem is the ryoku-logo.* the backend wrote into the fastfetch
    // dir; only those (and their bake) can be removed here.
    function isImportedEmblem(src) { return /\/fastfetch\/ryoku-logo\./.test(String(src || "")); }
    Process {
        id: removeLogoProc
        stderr: StdioCollector { id: removeLogoError }
        onExited: function (code) {
            if (code !== 0) {
                pg.storeStyleError = removeLogoError.text.trim() || I18n.tr("Couldn't remove the imported emblem.");
                return;
            }
            getProc.running = true; // backend repointed config.jsonc; reload to match
        }
    }
    function removeLogo() {
        pg.storeStyleError = "";
        removeLogoProc.command = ["ryoku-hub", "fastfetch", "remove-logo"];
        removeLogoProc.running = true;
    }

    PickFile {
        id: imageDlg
        title: I18n.tr("Choose a logo image")
        fileFilters: ["*.svg", "*.png", "*.jpg", "*.jpeg", "*.webp"]
        onPicked: (p) => { pg.importLogo(("" + p).replace("file://", ""), "image"); imageDlg.active = false; }
        onCanceled: imageDlg.active = false
    }
    PickFile {
        id: asciiDlg
        title: I18n.tr("Choose an ASCII art file")
        fileFilters: ["*.txt", "*.ascii", "*.art"]
        onPicked: (p) => { pg.importLogo(("" + p).replace("file://", ""), "ascii"); asciiDlg.active = false; }
        onCanceled: asciiDlg.active = false
    }

    // ── reusable pieces ─────────────────────────────────────────────────────

    // a named group head: 4px ink dot + tracked caps + a lineSoft leader.
    component SectionHead: Item {
        property string label: ""
        width: parent ? parent.width : 0
        height: 20
        Row {
            id: shl
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2
            Rectangle { width: 4; height: 4; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
            Text {
                text: I18n.tr(parent.parent.label)
                color: Tokens.ink; font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Rectangle {
            anchors.left: shl.right; anchors.leftMargin: Tokens.s3
            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
            height: 1; color: Tokens.lineSoft
        }
    }

    // a bounded-integer control row: a tracked caps label on the left, the value
    // numeral + unit + a Step (section 6) on the right.
    component NumRow: Item {
        id: nr
        property string label: ""
        property string unit: ""
        property int from: 0
        property int to: 100
        property int value: 0
        signal modified(int v)
        width: parent ? parent.width : 0
        height: 30
        Text {
            anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr(nr.label)
            color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
            font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
            font.capitalization: Font.AllUppercase
        }
        Row {
            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: nr.value
                color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fRow
                font.weight: Font.Light; font.features: ({ "tnum": 1 })
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                visible: nr.unit.length > 0
                text: nr.unit
                color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                font.capitalization: Font.AllUppercase
            }
            Step {
                anchors.verticalCenter: parent.verticalCenter
                from: nr.from; to: nr.to; value: nr.value
                onModified: (v) => nr.modified(v)
            }
        }
    }

    // the row's move / enable / remove tool: a 26px hairline glyph button, the
    // square-utility idiom in monochrome. Recreated here (it was inline in the
    // old page) because the enable tool needs its glyph colour to track state.
    component RowTool: Rectangle {
        id: rt
        property string glyph: ""
        property color tint: Tokens.inkDim
        signal tapped()
        width: 26
        height: 26
        radius: Tokens.radius
        color: rh.hovered ? Tokens.tint10 : "transparent"
        border.width: Tokens.border
        border.color: rh.hovered ? Tokens.lineStrong : Tokens.line
        Behavior on color { ColorAnimation { duration: Tokens.snap } }
        Text {
            anchors.centerIn: parent
            text: rt.glyph
            color: rh.hovered ? Tokens.ink : rt.tint
            font.family: Tokens.ui
            font.pixelSize: 12 // glyph, sized to the button, no matching token
        }
        HoverHandler { id: rh; cursorShape: Qt.PointingHandCursor }
        TapHandler { onTapped: rt.tapped() }
    }

    // ── head: eyebrow, Fraunces title with EDIT CONFIG, subtitle ─────────────
    Column {
        id: head
        anchors { left: parent.left; right: parent.right; top: parent.top }
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s6
        // the register row sits off the title: a rule over a 32px
        // title needs more than the gap between two lines of body text
        spacing: Tokens.s3

        Row {
            // the register row holds a fixed box, so the rule and the seal keep
            // their distance from the title on every page
            height: Tokens.s5
            spacing: Tokens.s2
            Rectangle {
                width: 16; height: 1; color: Tokens.ink
                anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: "力"; color: Tokens.ink; font.family: Tokens.jp
                font.pixelSize: Tokens.fMicro; anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: I18n.tr("TOOLS"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: Tokens.fTiny; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }

        Item {
            id: titleRow
            width: parent.width
            height: title.implicitHeight

            Text {
                id: title
                anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Fastfetch"); color: Tokens.ink
                font.family: Tokens.display; font.pixelSize: Tokens.fTitle
            }
            Row {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: Tokens.s2
                Btn {
                    text: I18n.tr("BROWSE RYOSTORE")
                    onAct: pg.browseFastfetch()
                }
                Btn {
                    id: editBtn
                    text: I18n.tr("EDIT CONFIG")
                    onAct: pg.openConfig()

                    HoverHandler { id: editHov; cursorShape: Qt.PointingHandCursor }
                    ToolTip {
                        id: editTip
                        visible: editHov.hovered
                        delay: 300
                        text: I18n.tr("Opens the fastfetch readout config (config.jsonc) directly. Yours; the Hub writes your changes here too.")
                        contentItem: Text {
                            width: 260
                            text: editTip.text
                            color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }
                        background: Rectangle {
                            color: Tokens.paperLift; radius: Tokens.radius
                            border.width: Tokens.border; border.color: Tokens.lineStrong
                        }
                    }
                }
            }
        }

        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("The terminal readout: emblem, rows and tagline, previewed live.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
        // a Flow, not a Row: three verbs plus the count overflow the page at this
        // width and the last button was cut off the right edge instead of wrapping
        Flow {
            width: parent.width
            spacing: Tokens.s3
            Text {
                visible: pg.installedStoreStyles.length > 0
                height: Tokens.ctlH
                verticalAlignment: Text.AlignVCenter
                text: I18n.tr("STORE LIBRARY \u00b7 %1").arg(pg.installedStoreStyles.length)
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fTiny
                font.weight: Font.Medium
                font.letterSpacing: Tokens.trackMark
            }
            Btn {
                visible: pg.installedStoreStyles.length > 0
                text: I18n.tr("APPLY INSTALLED STYLE")
                onAct: pg.storeStyleOpen = true
            }
            Btn {
                visible: pg.installedStoreStyles.length > 0
                text: I18n.tr("REMOVE INSTALLED STYLE")
                onAct: pg.storeStyleRemoveOpen = true
            }
            Btn {
                text: I18n.tr("RESET TO DEFAULT")
                onAct: pg.resetToDefault()
            }
        }
        Text {
            visible: pg.storeStyleError !== ""
            text: pg.storeStyleError
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }
    }

    // ── body: live preview (left) + controls (right) ─────────────────────────
    Item {
        id: body
        anchors { left: parent.left; right: parent.right; top: head.bottom; bottom: bar.top }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s5; anchors.bottomMargin: Tokens.s5

        // ── the pinned readout preview: emblem left, dossier right, drawn in
        // the terminal's own colours (a specimen, section 1). ──
        Rectangle {
            id: previewCard
            anchors.left: parent.left; anchors.top: parent.top; anchors.bottom: parent.bottom
            width: Math.round((body.width - Tokens.s6) * 0.46)
            radius: Tokens.radius
            color: "transparent"
            border.width: Tokens.border
            border.color: Tokens.line
            clip: true

            Text {
                id: pvLabel
                anchors.left: parent.left; anchors.top: parent.top
                anchors.leftMargin: Tokens.s4; anchors.topMargin: Tokens.s4
                text: I18n.tr("READOUT PREVIEW")
                color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                font.letterSpacing: Tokens.trackMark
                font.capitalization: Font.AllUppercase
            }

            Flickable {
                id: pvFlick
                anchors.left: parent.left; anchors.right: parent.right
                anchors.top: pvLabel.bottom; anchors.bottom: pvRule.top
                anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s3
                anchors.topMargin: Tokens.s3; anchors.bottomMargin: Tokens.s3
                contentWidth: width
                contentHeight: pvScale.height
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                WheelScroll { }

                // the whole readout scales to fit the card, so the emblem's size
                // relative to the text is a true specimen: adjusting Width /
                // Height / Pad resizes it in the same cells the terminal uses.
                Item {
                    id: pvScale
                    width: pvFlick.width
                    height: pvRow.implicitHeight * pvScale.factor
                    readonly property real factor: pvRow.implicitWidth > 0 ? Math.min(1, pvFlick.width / pvRow.implicitWidth) : 1

                    Row {
                        id: pvRow
                        transformOrigin: Item.TopLeft
                        scale: pvScale.factor
                        // fastfetch puts padding.right cells between the emblem
                        // and the first column of the readout.
                        spacing: pg.model.logo.kind === "none" ? 0 : Math.round(pg.padRight * pg.cellW)

                        // emblem: a Width x Height cell box with the art fit
                        // inside, Pad cells in from the left and padding.top rows
                        // down -- fastfetch drops the emblem by padding.top and
                        // leaves the readout at the top, so the preview must too.
                        Item {
                            id: emblemCol
                            readonly property real boxW: Math.max(1, pg.model.logo.width) * pg.cellW
                            readonly property real boxH: Math.max(1, pg.model.logo.height) * pg.cellH
                            readonly property real padL: pg.model.logo.padding * pg.cellW
                            readonly property real padT: pg.padTop * pg.cellH
                            width: pg.model.logo.kind === "none" ? 0 : (padL + boxW)
                            height: Math.max(readout.implicitHeight, padT + boxH)
                            Image {
                                x: emblemCol.padL; y: emblemCol.padT
                                width: emblemCol.boxW; height: emblemCol.boxH
                                visible: pg.model.logo.kind === "image" && pg.model.logo.source.length > 0
                                source: (pg.model.logo.kind === "image" && pg.model.logo.source.length > 0) ? pg.logoUrl(pg.model.logo.source) : ""
                                fillMode: Image.PreserveAspectFit
                                sourceSize.width: Math.round(emblemCol.boxW * 2) || 168
                                sourceSize.height: Math.round(emblemCol.boxH * 2) || 168
                                smooth: true
                                asynchronous: true
                            }
                            Rectangle {
                                x: emblemCol.padL; y: emblemCol.padT
                                width: emblemCol.boxW; height: emblemCol.boxH
                                radius: Tokens.radius
                                visible: pg.model.logo.kind === "ascii" || pg.model.logo.kind === "builtin"
                                color: "transparent"
                                border.width: Tokens.border
                                border.color: Tokens.line
                                Text {
                                    anchors.centerIn: parent
                                    text: pg.model.logo.kind === "ascii" ? I18n.tr("TXT") : "\uf303"
                                    color: Tokens.inkFaint
                                    font.family: Tokens.mono
                                    font.pixelSize: pg.model.logo.kind === "builtin" ? 30 : 12
                                }
                            }
                        }

                        Column {
                            id: readout
                            spacing: 0
                            Repeater {
                                model: pg.previewLines
                                delegate: Row {
                                    id: lineRow
                                    required property var modelData
                                    spacing: 0
                                    Repeater {
                                        model: lineRow.modelData
                                        delegate: Text {
                                            required property var modelData
                                            text: modelData.text
                                            color: modelData.color
                                            font.family: Tokens.mono
                                            font.pixelSize: 12
                                            font.weight: modelData.bold ? Font.Bold : Font.Normal
                                            textFormat: Text.PlainText
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }

            Rectangle {
                id: pvRule
                anchors.left: parent.left; anchors.right: parent.right
                anchors.bottom: pvBar.top
                anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4
                height: 1; color: Tokens.lineSoft
            }

            Item {
                id: pvBar
                anchors.left: parent.left; anchors.right: parent.right
                anchors.bottom: parent.bottom
                height: 48
                Btn {
                    anchors.left: parent.left; anchors.leftMargin: Tokens.s4
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("PREVIEW IN TERMINAL")
                    onAct: pg.previewInTerminal()
                }
                Text {
                    anchors.right: parent.right; anchors.rightMargin: Tokens.s4
                    anchors.verticalCenter: parent.verticalCenter
                    visible: pg.model.logo.kind === "image"
                    text: I18n.tr("kitty shows the real logo")
                    color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                }
            }
        }

        // ── controls: emblem, accent, info rows ──
        Flickable {
            id: ctrlFlick
            anchors.left: previewCard.right; anchors.leftMargin: Tokens.s6
            anchors.right: parent.right; anchors.top: parent.top; anchors.bottom: parent.bottom
            contentWidth: width
            contentHeight: ctrlCol.height
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }

            Column {
                id: ctrlCol
                width: ctrlFlick.width - Tokens.s3 // reserve a lane for the scroll rail
                spacing: Tokens.s5

                // ── EMBLEM ──
                Column {
                    width: parent.width
                    spacing: Tokens.s3

                    SectionHead { label: I18n.tr("EMBLEM") }

                    Seg {
                        options: ["Image", "ASCII", "Built-in", "None"]
                        current: {
                            var k = pg.model.logo.kind;
                            return k === "image" ? "Image" : (k === "ascii" ? "ASCII" : (k === "builtin" ? "Built-in" : "None"));
                        }
                        onChose: (label) => {
                            var k = label === "Image" ? "image" : (label === "ASCII" ? "ascii" : (label === "Built-in" ? "builtin" : "none"));
                            pg.setLogo("kind", k);
                        }
                    }

                    // your art: a gallery of ready emblems (the Ryoku brand marks
                    // and our ryodecors) plus a browse/drop tile for your own
                    // image. Click a tile to set it; drop a file to import it.
                    DropArea {
                        width: parent.width
                        height: artFlow.height
                        visible: pg.model.logo.kind === "image"
                        onDropped: (drop) => {
                            if (drop.hasUrls && drop.urls.length > 0)
                                pg.importLogo(pg.stripFile(drop.urls[0]), "image");
                        }
                        Flow {
                            id: artFlow
                            width: parent.width
                            spacing: Tokens.s2

                            Rectangle {
                                id: ownTile
                                width: 80; height: 80
                                radius: Tokens.radius
                                color: ownHov.hovered ? Tokens.tint10 : Tokens.tint5
                                border.width: Tokens.border
                                border.color: ownHov.hovered ? Tokens.ink : Tokens.lineStrong
                                Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                Column {
                                    anchors.centerIn: parent
                                    spacing: Tokens.s1
                                    Text {
                                        anchors.horizontalCenter: parent.horizontalCenter
                                        text: "+"; color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 22
                                    }
                                    Text {
                                        anchors.horizontalCenter: parent.horizontalCenter
                                        text: I18n.tr("YOUR IMAGE"); color: Tokens.inkMuted
                                        font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                                        font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                                    }
                                    Text {
                                        anchors.horizontalCenter: parent.horizontalCenter
                                        text: I18n.tr("browse or drop"); color: Tokens.inkFaint
                                        font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                                    }
                                }
                                HoverHandler { id: ownHov; cursorShape: Qt.PointingHandCursor }
                                TapHandler { onTapped: imageDlg.open() }
                            }

                            Repeater {
                                model: pg.readyArt
                                delegate: Rectangle {
                                    id: artTile
                                    required property var modelData
                                    readonly property bool on: pg.model.logo.kind === "image" && pg.model.logo.source === artTile.modelData.path
                                    width: 80; height: 80
                                    radius: Tokens.radius
                                    color: artTile.on ? Tokens.tint10 : (artHov.hovered ? Tokens.tint5 : "transparent")
                                    border.width: Tokens.border
                                    border.color: artTile.on ? Tokens.ink : (artHov.hovered ? Tokens.lineStrong : Tokens.line)
                                    Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
                                    Image {
                                        anchors.top: parent.top; anchors.horizontalCenter: parent.horizontalCenter
                                        anchors.topMargin: Tokens.s2
                                        width: 46; height: 46
                                        source: pg.logoUrl(artTile.modelData.path)
                                        fillMode: Image.PreserveAspectFit
                                        sourceSize.width: 92; sourceSize.height: 92
                                        asynchronous: true; smooth: true
                                    }
                                    Text {
                                        anchors.bottom: parent.bottom; anchors.horizontalCenter: parent.horizontalCenter
                                        anchors.bottomMargin: Tokens.s2
                                        text: artTile.modelData.name
                                        color: artTile.on ? Tokens.ink : Tokens.inkFaint
                                        font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                                        font.letterSpacing: 1
                                    }
                                    // only a store emblem can be removed: the
                                    // brand marks and the shipped decors are ours
                                    readonly property bool removable:
                                        String(artTile.modelData.path).indexOf("/ryoemblems/") >= 0
                                    Rectangle {
                                        id: dropMark
                                        visible: artTile.removable && (artHov.hovered || dropHov.hovered)
                                        anchors.top: parent.top
                                        anchors.right: parent.right
                                        width: Tokens.ctlH
                                        height: Tokens.ctlH
                                        radius: Tokens.radius
                                        color: dropHov.hovered ? Tokens.tint10 : "transparent"
                                        Text {
                                            anchors.centerIn: parent
                                            text: "\u00d7"
                                            color: dropHov.hovered ? Tokens.ink : Tokens.inkFaint
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fBody
                                        }
                                        HoverHandler { id: dropHov; cursorShape: Qt.PointingHandCursor }
                                        TapHandler { onTapped: pg.removeInstalledEmblem(artTile.modelData.path) }
                                    }
                                    HoverHandler { id: artHov; cursorShape: Qt.PointingHandCursor }
                                    TapHandler { onTapped: pg.pickArt(artTile.modelData.path) }
                                }
                            }
                        }
                    }

                    // ASCII art is a text file, not gallery art: keep a chooser.
                    Item {
                        width: parent.width; height: 30
                        visible: pg.model.logo.kind === "ascii"
                        Btn {
                            anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("CHOOSE .TXT"); onAct: asciiDlg.open()
                        }
                    }

                    // the chosen file, mono file-truth, with an open action.
                    Item {
                        width: parent.width; height: 26
                        visible: (pg.model.logo.kind === "image" || pg.model.logo.kind === "ascii") && pg.model.logo.source.length > 0
                        Btn {
                            id: openFileBtn
                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("OPEN FILE"); onAct: pg.openLogoFile()
                        }
                        Text {
                            anchors.left: parent.left; anchors.right: openFileBtn.left; anchors.rightMargin: Tokens.s3
                            anchors.verticalCenter: parent.verticalCenter
                            elide: Text.ElideMiddle
                            text: pg.model.logo.source
                            color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: Tokens.fSmall
                        }
                    }

                    // remove an imported emblem (ryoku-logo.*) from the fastfetch
                    // dir; the readout falls back to the shipped emblem.
                    Item {
                        width: parent.width; height: 26
                        visible: pg.model.logo.kind === "image" && pg.isImportedEmblem(pg.model.logo.source)
                        Btn {
                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("REMOVE IMPORTED EMBLEM"); onAct: pg.removeLogo()
                        }
                    }

                    // size: how the art occupies the terminal's character grid.
                    Column {
                        width: parent.width
                        spacing: Tokens.s2
                        visible: pg.model.logo.kind === "image" || pg.model.logo.kind === "ascii"
                        // Auto fit sizes the cell box to the art so it never distorts.
                        Item {
                            width: parent.width; height: 26
                            visible: pg.model.logo.kind === "image"
                            Text {
                                anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("SIZE"); color: Tokens.inkMuted; font.family: Tokens.ui
                                font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                                font.letterSpacing: Tokens.trackLabel; font.capitalization: Font.AllUppercase
                            }
                            Btn {
                                anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("AUTO FIT"); onAct: pg.autoFit()
                            }
                        }
                        NumRow {
                            label: I18n.tr("Width"); unit: "col"; from: 0; to: 80
                            value: pg.model.logo.width
                            onModified: (v) => pg.setLogo("width", v)
                        }
                        NumRow {
                            label: I18n.tr("Height"); unit: "col"; from: 0; to: 60
                            value: pg.model.logo.height
                            onModified: (v) => pg.setLogo("height", v)
                        }
                        NumRow {
                            label: I18n.tr("Pad"); unit: "col"; from: 0; to: 20
                            value: pg.model.logo.padding
                            onModified: (v) => pg.setLogo("padding", v)
                        }
                    }

                    // 1-bit bake: bone stipple on transparent (ordered Bayer 4x4),
                    // the ryodecor look. Off restores the original source.
                    Item {
                        width: parent.width; height: 26
                        visible: pg.model.logo.kind === "image" && pg.model.logo.source.length > 0
                        Text {
                            anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("DITHER"); color: Tokens.inkMuted; font.family: Tokens.ui
                            font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                            font.letterSpacing: Tokens.trackLabel; font.capitalization: Font.AllUppercase
                        }
                        Sw {
                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                            on: pg.model.logo.dither === true
                            onToggled: (v) => pg.setDither(v)
                        }
                    }

                    Text {
                        width: Math.min(parent.width, 560)
                        wrapMode: Text.WordWrap
                        visible: pg.model.logo.kind === "image"
                        text: I18n.tr("Pick a ready emblem, or drop your own (PNG, JPG, WEBP; an SVG is rasterized to PNG on import). Square art reads best; it draws with the kitty graphics protocol in kitty, chafa elsewhere. Size is in terminal character cells.")
                        color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    }
                }

                // ── ACCENT ──
                Column {
                    width: parent.width
                    spacing: Tokens.s3

                    SectionHead { label: I18n.tr("ACCENT") }

                    Seg {
                        options: [I18n.tr("Fixed"), I18n.tr("Follow wallpaper")]
                        current: pg.model.palette === "wallpaper" ? I18n.tr("Follow wallpaper") : I18n.tr("Fixed")
                        onChose: (label) => pg.setPalette(label === I18n.tr("Follow wallpaper") ? "wallpaper" : "fixed")
                    }

                    Text {
                        width: Math.min(parent.width, 560)
                        visible: pg.model.palette === "wallpaper"
                        wrapMode: Text.WordWrap
                        text: I18n.tr("The live Material palette colours the title, labels, sections, rules and percentage gauges. Your fixed accent stays saved underneath.")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                    }

                    // the readout accent is a colour the user is choosing, so its
                    // swatch is a specimen (section 1): it is allowed to be itself.
                    Item {
                        enabled: pg.model.palette !== "wallpaper"
                        opacity: enabled ? 1 : 0.38
                        width: parent.width
                        height: 30
                        Text {
                            anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("READOUT ACCENT")
                            color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                            font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                        }
                        Row {
                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s2
                            Rectangle {
                                anchors.verticalCenter: parent.verticalCenter
                                width: 30; height: 24
                                radius: Tokens.radius
                                color: pg.accentHex() // specimen: the accent is its own colour
                                border.width: Tokens.border; border.color: Tokens.line
                            }
                            Field {
                                id: hexField
                                anchors.verticalCenter: parent.verticalCenter
                                width: 110
                                tabular: true // a hex code is file-truth
                                placeholder: "#rrggbb"
                                text: pg.accentHex()
                                onCommitted: (v) => {
                                    var t = v.charAt(0) === "#" ? v : "#" + v;
                                    if (/^#[0-9A-Fa-f]{6}$/.test(t))
                                        pg.setAccent(pg.hexToTriple(t));
                                }
                            }
                        }
                    }
                }

                // ── INFO ──
                Column {
                    width: parent.width
                    spacing: Tokens.s3

                    // section head with an add affordance, the Environment idiom.
                    Item {
                        width: parent.width
                        height: 26
                        Row {
                            id: infoLabel
                            anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s2
                            Rectangle { width: 4; height: 4; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
                            Text {
                                text: I18n.tr("INFO"); color: Tokens.ink; font.family: Tokens.ui
                                font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                                font.letterSpacing: Tokens.trackMark
                                anchors.verticalCenter: parent.verticalCenter
                            }
                        }
                        IconBtn {
                            id: addBtn
                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                            glyph: "+"
                            onAct: pg.addOpen = true
                        }
                        Rectangle {
                            anchors.left: infoLabel.right; anchors.right: addBtn.left
                            anchors.leftMargin: Tokens.s3; anchors.rightMargin: Tokens.s3
                            anchors.verticalCenter: parent.verticalCenter
                            height: 1; color: Tokens.lineSoft
                        }
                    }

                    Repeater {
                        model: pg.model.rows
                        delegate: Rectangle {
                            id: rowItem
                            required property var modelData
                            required property int index
                            width: parent.width
                            height: Tokens.rowH
                            radius: Tokens.radius
                            color: rowHov.hovered ? Tokens.tint5 : "transparent"
                            opacity: rowItem.modelData.enabled ? 1 : 0.5
                            Behavior on color { ColorAnimation { duration: Tokens.snap } }
                            HoverHandler { id: rowHov }

                            Row {
                                id: moveTools
                                anchors.left: parent.left; anchors.leftMargin: Tokens.s2
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s1
                                RowTool { glyph: "\u2191"; onTapped: pg.moveRow(rowItem.index, -1) }
                                RowTool { glyph: "\u2193"; onTapped: pg.moveRow(rowItem.index, 1) }
                            }

                            Text {
                                id: rowLabel
                                anchors.left: moveTools.right; anchors.leftMargin: Tokens.s3
                                anchors.verticalCenter: parent.verticalCenter
                                width: 108
                                elide: Text.ElideRight
                                text: I18n.tr(rowItem.modelData.label || rowItem.modelData.kind)
                                color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            }

                            // inline editor for a tagline/header text or a module key.
                            Field {
                                id: ed
                                anchors.left: rowLabel.right; anchors.leftMargin: Tokens.s3
                                anchors.right: infoTools.left; anchors.rightMargin: Tokens.s3
                                anchors.verticalCenter: parent.verticalCenter
                                visible: pg.rowEditable(rowItem.modelData.kind)
                                placeholder: pg.rowPlaceholder(rowItem.modelData.kind)
                                text: rowItem.modelData.kind === "module" ? (rowItem.modelData.key || "") : (rowItem.modelData.text || "")
                                onCommitted: (v) => {
                                    var f = rowItem.modelData.kind === "module" ? "key" : "text";
                                    if (v !== (rowItem.modelData[f] || ""))
                                        pg.setRow(rowItem.index, f, v);
                                }
                            }

                            Row {
                                id: infoTools
                                anchors.right: parent.right; anchors.rightMargin: Tokens.s2
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s1
                                RowTool {
                                    glyph: rowItem.modelData.enabled ? "\u25cf" : "\u25cb"
                                    tint: rowItem.modelData.enabled ? Tokens.ink : Tokens.inkFaint
                                    onTapped: pg.setRow(rowItem.index, "enabled", !rowItem.modelData.enabled)
                                }
                                RowTool { glyph: "\u2715"; onTapped: pg.removeRow(rowItem.index) }
                            }
                        }
                    }
                }
            }
        }
    }

    // ── action bar: status + Revert / Save, the only path to disk ────────────
    Rectangle {
        id: bar
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        height: 60
        color: "transparent"
        // hairline lid, like the shell's action bar (section 8).
        Rectangle {
            anchors { left: parent.left; right: parent.right; top: parent.top }
            height: 1; color: Tokens.line
        }

        Row {
            anchors.left: parent.left; anchors.leftMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            // filled ink while dirty (heartbeat, the one perpetual animation), a
            // hairline outline when clean.
            Rectangle {
                id: dot
                anchors.verticalCenter: parent.verticalCenter
                width: 6; height: 6; radius: 3
                antialiasing: false
                color: pg.dirty ? Tokens.ink : "transparent"
                border.width: pg.dirty ? 0 : Tokens.border
                border.color: Tokens.inkFaint
                SequentialAnimation on opacity {
                    running: pg.dirty
                    loops: Animation.Infinite
                    NumberAnimation { to: 0.3; duration: 600; easing.type: Easing.InOutSine }
                    NumberAnimation { to: 1.0; duration: 600; easing.type: Easing.InOutSine }
                    onStopped: dot.opacity = 1
                }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: pg.dirty ? I18n.tr("Unsaved changes") : I18n.tr("Saved \u00b7 config.jsonc")
                color: pg.dirty ? Tokens.ink : Tokens.inkMuted
                font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.Medium
            }
        }

        Row {
            anchors.right: parent.right; anchors.rightMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3
            Btn { text: I18n.tr("REVERT"); armed: pg.dirty; onAct: pg.revert() }
            // the armed primary inverts to bone while dirty; Save writes config.jsonc.
            Btn { text: I18n.tr("SAVE"); primary: true; armed: pg.dirty; onAct: pg.save() }
        }
    }

    // ── the "add a row" catalogue overlay (Picker) ──
    MouseArea {
        id: addScrim
        anchors.fill: parent
        visible: pg.addOpen
        z: 100
        // a bare click-catcher: dismiss on an outside click. No fill --
        // translucency is banned on app surfaces (section 6).
        onClicked: pg.addOpen = false
        onVisibleChanged: if (visible) addPicker.open()

        Picker {
            id: addPicker
            anchors.centerIn: parent
            title: I18n.tr("Add a row")
            options: pg.addLabels()
            current: ""
            onChose: (label) => { pg.addFromLabel(label); pg.addOpen = false; }
            onDismissed: pg.addOpen = false

            // absorb clicks inside the card so the scrim does not treat a
            // header/padding tap as an outside dismiss.
            MouseArea { anchors.fill: parent; z: -1 }
        }
    }

    MouseArea {
        anchors.fill: parent
        visible: pg.storeStyleOpen
        z: 100
        onClicked: pg.storeStyleOpen = false
        onVisibleChanged: if (visible) storeStylePicker.open()

        Picker {
            id: storeStylePicker
            anchors.centerIn: parent
            title: I18n.tr("Apply a Store style")
            options: pg.storeStyleLabels()
            current: ""
            onChose: (label) => {
                pg.applyStoreStyle(label);
                pg.storeStyleOpen = false;
            }
            onDismissed: pg.storeStyleOpen = false

            MouseArea { anchors.fill: parent; z: -1 }
        }
    }

    MouseArea {
        anchors.fill: parent
        visible: pg.storeStyleRemoveOpen
        z: 100
        onClicked: pg.storeStyleRemoveOpen = false
        onVisibleChanged: if (visible) storeStyleRemovePicker.open()

        Picker {
            id: storeStyleRemovePicker
            anchors.centerIn: parent
            title: I18n.tr("Remove a Store style")
            options: pg.storeStyleLabels()
            current: ""
            onChose: (label) => {
                pg.removeStoreStyle(label);
                pg.storeStyleRemoveOpen = false;
            }
            onDismissed: pg.storeStyleRemoveOpen = false

            MouseArea { anchors.fill: parent; z: -1 }
        }
    }
}

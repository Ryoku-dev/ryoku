pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"
import "schema/DesktopPage.js" as DesktopSchema
import "schema/BarStudioPage.js" as BarStudioSchema
import "schema/WindowSettings.js" as WindowSettingsSchema
import "schema/PluginsPage.js" as PluginsSchema
import "schema/InputPage.js" as InputSchema
import "schema/KeybindsPage.js" as KeybindsSchema
import "schema/DisplaysPage.js" as DisplaysSchema
import "schema/GpuPage.js" as GpuSchema
import "schema/RecordingPage.js" as RecordingSchema
import "schema/DictationPage.js" as DictationSchema
import "schema/LauncherPage.js" as LauncherSchema
import "schema/FastfetchPage.js" as FastfetchSchema
import "schema/WidgetsPage.js" as WidgetsSchema
import "schema/LockscreenPage.js" as LockscreenSchema
import "schema/AnimationsPage.js" as AnimationsSchema
import "schema/AddonsPage.js" as AddonsSchema
import "schema/WindowRulesPage.js" as WindowRulesSchema
import "schema/AppOverridesPage.js" as AppOverridesSchema
import "schema/LayerRulesPage.js" as LayerRulesSchema
import "schema/SessionPage.js" as SessionSchema
import "schema/PerformancePage.js" as PerformanceSchema
import "schema/UpdatesPage.js" as UpdatesSchema
import "ReloadCoverModel.js" as ReloadCoverModel
import Ryoku.FrameBars

// Ryoku Settings, assembled. The rail owns navigation and global search; the
// head, cells and tabs are drawn once by SchemaPage from a page's schema; the
// pinned side is the feedback loop (live preview, the dirty-state plate, the
// pending-write diff); the action bar is the one surface whose absence eats an
// edit, so it lives in the shell and no page can lose it.
//
// Nothing writes to disk except Save. Values live in `draft`; `committed` is
// what the JsonAdapters hold from disk; the diff is draft against committed,
// rendered in each file's own JSON syntax. Factory values live in `defs` and
// only RESET reaches for them.
Rectangle {
    id: hub

    implicitWidth: 1400
    implicitHeight: 880
    color: Tokens.paper
    focus: true

    // ── which page ───────────────────────────────────────────────────────
    property string section: "windowmanager"
    // remember the last section so a reopen lands where you left, not the default.
    // Read once at startup by `sectionGet` below; written here on every change.
    onSectionChanged: Quickshell.execDetached(["ryoku-hub", "config", "set", "section", hub.section])
    // A Hub that remembered the retired `windows` section, or a deep link that
    // still names it, lands on the compositor page that now holds those rows
    // rather than a blank pane.
    function canonicalSection(s) {
        // sections that were folded into another page: an old deep link (the
        // Store's "open in settings", a keybind, a script) still lands right.
        if (s === "windows") return "windowmanager";
        if (s === "cursor") return "input";
        if (s === "autostart" || s === "environment") return "session";
        return s;
    }
    // An explicit jump (the nav IPC, i.e. the Store's "open in settings") must
    // win over the remembered section. `sectionGet` is a Process, so on a cold
    // start its stdout lands AFTER the IPC has already set the page and the
    // restore silently drags the user back to wherever they were last -- the
    // handoff looked like it did nothing. Deep links come through here and latch.
    function navigate(target) {
        target = hub.canonicalSection(target);
        if (!target || hub.pageFile(target) === "" || !hub.sectionAvailable(target))
            return;
        hub.navigated = true;
        hub.section = target;
    }
    property bool navigated: false
    property string query: ""

    // progressive disclosure: one global Advanced switch (in the rail) reveals the
    // deep knobs across every schema page. persisted like `section`, restored at
    // startup by `advancedGet` below.
    property bool advanced: false
    onAdvancedChanged: Quickshell.execDetached(["ryoku-hub", "config", "set", "advanced", hub.advanced ? "1" : "0"])

    // The full catalogue. `wired` marks the pages whose content and
    // persistence are ported; the rest render an honest porting plate rather
    // than a settings page that cannot save. `needs` is what the active window
    // manager must offer for a page to mean anything: a behavioural capability
    // (`cap`), a provider being present at all (`provider`), or the page holding
    // at least one row the provider actually backs (`rows`, tested against the
    // effective set of Hub rows plus the provider's own). A page with no `needs`
    // is compositor-neutral and always shows. The rail lists every section it
    // can back, grouped by task, so a control is never shown that nothing would
    // write. The rail scrolls; the "Advanced" switch at its foot only reveals the
    // deep per-page knobs inside a schema page, never whole sections, so a page
    // stays calm until asked. Compositor-owned pages sit together under one group
    // so a compositor's settings live in one place, not sprinkled through the list.
    readonly property var groups: [
        { name: "OVERVIEW", items: [ { key: "profile", name: "Profile" }, { key: "global", name: "General" }, { key: "updates", name: "Updates" } ] },
        { name: "DEVICES", items: [
            { key: "displays", name: "Displays", needs: { cap: "monitorConfig" } }, { key: "connections", name: "Connections" },
            { key: "input", name: "Input" }, { key: "gpu", name: "Graphics & Power" } ] },
        { name: "LOOK", items: [
            { key: "animations", name: "Animations" }, { key: "lockscreen", name: "Lockscreen" } ] },
        { name: "COMPOSITOR", items: [
            { key: "windowmanager", name: "Window Manager", needs: { provider: true } }, { key: "plugins", name: "Plugins", needs: { cap: "plugins" } },
            { key: "layerrules", name: "Layer Rules", adv: true, needs: { rows: true } } ] },
        { name: "DESKTOP", items: [
            { key: "bar-studio", name: "Bar Studio", wired: true }, { key: "desktop", name: "Desktop", wired: true },
            { key: "widgets", name: "Widgets" }, { key: "launcher", name: "App Launcher" } ] },
        { name: "KEYS & APPS", items: [
            { key: "keybinds", name: "Keybinds" }, { key: "appoverrides", name: "App Overrides", adv: true },
            { key: "windowrules", name: "Window Rules", adv: true } ] },
        { name: "SYSTEM", items: [
            { key: "performance", name: "Performance" }, { key: "session", name: "Session" },
            { key: "recording", name: "Recording" }, { key: "dictation", name: "Dictation" }, { key: "fastfetch", name: "Fastfetch", adv: true },
            { key: "import", name: "Import config", adv: true, wired: true } ] },
        { name: "EXTEND", items: [
            { key: "addons", name: "Add-ons" }, { key: "rashin", name: "Rashin" } ] },
        { name: "", items: [ { key: "credits", name: "Credits" } ] }
    ]

    // Each section's terse kanji, paired with its Latin name in the rail. Latin
    // names the thing; the kanji is its seal. The two scripts sitting together
    // is the texture, and every gloss is the real word, never decoration:
    // 外観 = appearance, 接続 = connections, 演算 = compute (Machine), and so on.
    readonly property var jpName: ({
        "profile": "横顔", "displays": "画面", "input": "入力", "keybinds": "操作",
        "connections": "接続", "gpu": "演算", "recording": "録画", "dictation": "音声",
        "plugins": "補", "bar-studio": "帯", "desktop": "卓上", "launcher": "起動", "fastfetch": "情報",
        "widgets": "部品", "lockscreen": "施錠", "animations": "動き",
        "addons": "拡張", "windowrules": "規則", "appoverrides": "上書", "layerrules": "階層",
        "session": "起動", "performance": "性能", "rashin": "羅針",
        "updates": "更新", "credits": "謝辞", "global": "全般", "import": "取込", "windowmanager": "合成"
    })

    // Extra search vocabulary per section: the words a user actually types that
    // the labels never use. This is what lets the search reach a page with no
    // schema rows (Connections, Rashin) and cover synonyms the copy avoids
    // (transparency->opacity, startup->autostart, screensaver->lockscreen).
    readonly property var sectionKeywords: ({
        "global": "language locale region formats format date time number currency text size font scale location city weather timezone international i18n l10n",
        "profile": "dashboard status overview telemetry hostname cpu gpu memory uptime specs",
        "displays": "monitor screen resolution refresh scale rotation arrange mirror hidpi dual second external multiple",
        "connections": "wifi wi-fi wireless bluetooth network hotspot tether internet ethernet pair pairing device",
        "input": "keyboard mouse touchpad pointer trackpad sensitivity scroll layout dvorak remap capslock repeat gesture",
        "keybinds": "shortcuts hotkeys binds keys browser terminal editor files launch super",
        "gpu": "graphics nvidia amd vram passthrough vfio rendering hybrid performance cpu governor epp frequency thermal battery charge ceiling aspm profile",
        "recording": "screen record capture video screencast screenshot fps codec framerate",
        "dictation": "voice typing speech transcribe whisper microphone stt",
        "windowmanager": "compositor window manager wm wayland switch change swap session provider window windows rounding corners softness gaps border borders thickness colour tiling layout opacity transparency transparent dim blur shadow float snap resize animation spread offset",
        "plugins": "plugin plugins hyprland compositor hyprpm title bar titlebar hyprbars glass hyprglass image border imgborders cursor motion dynamic cursors focus flash hyprfocus key sound sounds keyboard keysounds typing click clicky thock creamy cherry mx topre mechvibes switch version abi mismatch rebuild build update add git repository install",
        "bar-studio": "bar frame rails zones widgets menus surfaces style catalogue layout framebars sidebar dock dockapps pinned pin magnify autohide auto-hide media chip peek labels edge taskbar",
        "desktop": "desktop visualizer visualiser spectrum brand logo mark name widget board wallpaper",
        "launcher": "launcher spotlight command palette greeting weather home",
        "fastfetch": "fetch neofetch terminal system info logo ascii emblem readout",
        "widgets": "desktop widget clock calendar weather face overlay wallpaper",
        "lockscreen": "lock screensaver signin greeter skin theme login",
        "animations": "animation animations motion transition bezier curve speed feel wobbly disable enable toggle",
        "addons": "installed plugin addon extension manage enable remove update widget bundle extras store marketplace browse",
        "windowrules": "window rule float pin size place opacity class title override",
        "appoverrides": "app override per-app opacity blur corner class inherit opaque transparent",
        "layerrules": "layer rule namespace blur dim bar notification surface",
        "session": "session login startup autostart launch run command boot environment variable env var export",
        "performance": "performance battery power saving save lowpower potato lag cpu gpu ram memory idle freeze reduce motion fps",
        "rashin": "rashin agent ai assistant hermes vault memory skills chat code llm needle",
        "updates": "update upgrade version channel commit behind check origin",
        "import": "import bring migrate dotfiles config existing kitty fish fastfetch drop folder git backup undo restore adopt",
        "credits": "credits thanks acknowledgement gratitude contributor"
    })

    // ── global search ────────────────────────────────────────────────────
    // The rail search matches page titles AND every option (label, hint, key)
    // inside every schema page, so "noise" finds the Blur noise control from
    // anywhere. Ranking is fuzzy: exact word > substring > subsequence.
    // section -> its schema rows, the one source for both global search and the
    // compositor-driving classification, so the two cannot drift apart.
    readonly property var sectionRows: ({
        "bar-studio": BarStudioSchema.rows, "desktop": DesktopSchema.rows, "windowmanager": WindowSettingsSchema.rows, "plugins": PluginsSchema.rows,
        "input": InputSchema.rows, "keybinds": KeybindsSchema.rows,
        "displays": DisplaysSchema.rows, "gpu": GpuSchema.rows,
        "recording": RecordingSchema.rows, "dictation": DictationSchema.rows,
        "launcher": LauncherSchema.rows, "fastfetch": FastfetchSchema.rows,
        "widgets": WidgetsSchema.rows, "lockscreen": LockscreenSchema.rows,
        "animations": AnimationsSchema.rows, "addons": AddonsSchema.rows,
        "windowrules": WindowRulesSchema.rows, "appoverrides": AppOverridesSchema.rows,
        "layerrules": LayerRulesSchema.rows, "session": SessionSchema.rows,
        "performance": PerformanceSchema.rows,
        "updates": UpdatesSchema.rows
    })
    readonly property var searchIndex: {
        var srcs = hub.sectionRows;
        // a real, navigable setting vs a doc-only "surface" row (an action button
        // or a dynamic-title note) whose engineering copy must never surface.
        var isSetting = function (r) {
            if (!r || !r.label) return false;
            if (r.ctl === "action" || r.ctl === "layoutdemo") return false;
            if (/^\s*\(/.test(r.label) || /\((action bar|quick action)\)/.test(r.label)) return false;
            return true;
        };
        // a result breadcrumb shows the group; the schemas hide engineering notes
        // in it (parentheticals, <dynamic> tokens), so drop those from display.
        var cleanGroup = function (g) {
            if (!g) return "";
            if (g.indexOf("(") >= 0 || g.indexOf("<") >= 0) return "";
            return g;
        };
        var nameOf = {}, out = [];
        for (var gi = 0; gi < groups.length; gi++)
            for (var ii = 0; ii < groups[gi].items.length; ii++) {
                var it = groups[gi].items[ii];
                nameOf[it.key] = it.name;
                // a page the active provider cannot back is hidden in the rail, so
                // the deep-link path through a search hit must be closed here too.
                if (!hub.needsMet(it)) continue;
                var pkw = sectionKeywords[it.key] || "";
                // the window-manager page answers to the running compositor's own
                // name, read live from the provider rather than a hardcoded list, so
                // a user reaches it by typing the name of the desktop they run.
                if (it.key === "windowmanager" && Settings.provider)
                    pkw += " " + Settings.provider;
                out.push({ section: it.key, sectionName: it.name, group: "", tab: "", label: it.name, desc: "", kw: pkw, key: "", isPage: true });
            }
        // Updates left the rail for the top-right corner button, so it has no
        // page row here; its one setting still surfaces in search, named right.
        nameOf["updates"] = "Updates";
        for (var k in srcs) {
            // rows on a filtered page never surface either, or the gate is cosmetic.
            if (!hub.sectionAvailable(k)) continue;
            var rows = srcs[k] || [];
            for (var ri = 0; ri < rows.length; ri++) {
                var r = rows[ri];
                if (!isSetting(r)) continue;
                if (r.caps && !Settings.supports(r.caps)) continue;
                if (!Settings.modelsKey(r.key)) continue;
                // a setting also matches its option values (h264, dwindle, dark,
                // fahrenheit): index the lowercase ones (skips DisplaysPage's
                // capitalised doc placeholders) so an enum value finds its row.
                var optkw = r.opts ? r.opts.filter(function (o) { return typeof o === "string" && /^[a-z0-9][a-z0-9 ._/-]*$/.test(o); }).join(" ") : "";
                out.push({ section: k, sectionName: nameOf[k] || k, group: cleanGroup(r.group), tab: r.tab || "", label: r.label, desc: r.desc || "", kw: optkw, key: r.key || "", isPage: false });
            }
        }
        // The active provider's own settings are real rows too: fold them in so
        // search still reaches a migrated setting (a tiling knob, a plugin toggle,
        // a bezier curve) at the page it now lives on. Each carries its own `page`
        // tag; a row on a filtered page is skipped, the same as the Hub's own rows.
        var prows = ProviderSchema.rows || [];
        for (var pi = 0; pi < prows.length; pi++) {
            var pr = prows[pi];
            if (!isSetting(pr)) continue;
            var psec = pr.page || "windowmanager";
            if (!hub.sectionAvailable(psec)) continue;
            if (pr.caps && !Settings.supports(pr.caps)) continue;
            if (!Settings.modelsKey(pr.key)) continue;
            var poptkw = pr.opts ? pr.opts.filter(function (o) { return typeof o === "string" && /^[a-z0-9][a-z0-9 ._/-]*$/.test(o); }).join(" ") : "";
            out.push({ section: psec, sectionName: nameOf[psec] || psec, group: cleanGroup(pr.group), tab: pr.tab || "", label: pr.label, desc: pr.desc || "", kw: poptkw, key: pr.key || "", isPage: false });
        }
        return out;
    }
    // Intent map: what a user types -> the vocabulary the index actually uses.
    // This is the "semantic" layer without a model: a curated synonym table for
    // a bounded settings vocabulary matches intent far more cheaply (and
    // predictably) than a local embedding runtime would. A query word expands to
    // itself plus these, and any one of them satisfying a term counts as a hit.
    readonly property var synonyms: ({
        "transparency": "opacity", "transparent": "opacity", "seethrough": "opacity transparency", "opacity": "transparency",
        "darkmode": "dark scheme theme", "lightmode": "light scheme theme", "theme": "appearance scheme palette",
        "wallpaper": "appearance background", "color": "colour appearance palette", "colour": "color appearance palette", "accent": "appearance colour",
        "hotkey": "keybind shortcut", "hotkeys": "keybinds shortcuts", "shortcut": "keybind", "shortcuts": "keybinds",
        "wifi": "connections wireless network", "internet": "connections network", "ethernet": "connections network", "bluetooth": "connections", "hotspot": "connections",
        "brightness": "backlight", "backlight": "brightness", "nightlight": "night comfort backlight", "warmth": "night comfort", "bluelight": "night comfort",
        "volume": "audio sound", "sound": "audio", "font": "typeface appearance", "typeface": "font appearance",
        "screenshot": "recording capture", "screencast": "recording capture", "screensaver": "lockscreen lock", "lock": "lockscreen",
        "startup": "session", "boot": "session", "battery": "performance power", "powersaving": "performance power", "potato": "performance", "lag": "performance",
        "gap": "gaps spacing", "spacing": "gaps", "glass": "blur liquid", "liquid": "glass blur",
        "titlebar": "title bar", "titlebars": "title bar", "plugin": "plugins addon", "plugins": "plugin addon",
        "monitor": "displays screen", "monitors": "displays screen", "resolution": "displays screen", "hidpi": "displays scale", "refresh": "displays",
        "mouse": "input pointer", "pointer": "input", "keyboard": "input", "touchpad": "input trackpad", "trackpad": "input touchpad",
        "visualizer": "desktop spectrum", "visualiser": "desktop spectrum", "clock": "widgets desktop", "notifications": "layerrules",
        "update": "updates upgrade", "upgrade": "updates", "blur": "windows glass", "rounding": "windows corners", "corners": "windows rounding",
        "animation": "animations motion", "motion": "animations", "gpu": "graphics", "graphics": "gpu",
        "voice": "dictation", "speech": "dictation voice", "microphone": "dictation", "mic": "dictation"
    })
    // a query word expands to itself plus its synonyms, so a term is satisfied by
    // any one of them (and never dilutes multi-word coverage).
    function expandQuery(q) {
        var words = q.split(/\s+/), groups = [];
        for (var i = 0; i < words.length; i++) {
            var w = words[i];
            if (!w) continue;
            var g = [w], syn = synonyms[w];
            if (syn) { var ps = syn.split(/\s+/); for (var j = 0; j < ps.length; j++) if (ps[j]) g.push(ps[j]); }
            groups.push(g);
        }
        return groups;
    }
    // bounded Levenshtein: returns the distance if <= max, else -1, with an
    // early length-diff reject and a per-row floor so it stays cheap in the
    // per-keystroke loop.
    function editDistance(a, b, max) {
        var la = a.length, lb = b.length, j;
        if (Math.abs(la - lb) > max) return -1;
        var prev = [], cur = [];
        for (j = 0; j <= lb; j++) prev[j] = j;
        for (var i = 1; i <= la; i++) {
            cur[0] = i;
            var rowMin = i;
            for (j = 1; j <= lb; j++) {
                var cost = a.charAt(i - 1) === b.charAt(j - 1) ? 0 : 1;
                var v = Math.min(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + cost);
                cur[j] = v;
                if (v < rowMin) rowMin = v;
            }
            if (rowMin > max) return -1;
            var tmp = prev; prev = cur; cur = tmp;
        }
        return prev[lb] <= max ? prev[lb] : -1;
    }
    function wordScore(w, text) {
        var tws = text.split(/[^a-z0-9]+/);
        var best = 0;
        for (var i = 0; i < tws.length; i++) {
            var tw = tws[i];
            if (tw === "") continue;
            var idx = tw.indexOf(w);
            if (idx === 0) { if (best < 1200) best = 1200; continue; }
            if (idx > 0) { var ss = 1000 - Math.min(idx, 50); if (ss > best) best = ss; continue; }
            if (w.length <= tw.length) {
                var ti = 0, sc = 0, streak = 0, ok = true;
                for (var ci = 0; ci < w.length; ci++) {
                    var f = tw.indexOf(w.charAt(ci), ti);
                    if (f < 0) { ok = false; break; }
                    streak = (f === ti) ? streak + 1 : 0;
                    sc += 2 + streak * 3;
                    ti = f + 1;
                }
                if (ok && sc > best) best = sc;
            }
        }
        // typo tolerance: a near-miss still matches, weaker than a clean hit, so
        // "trasparency" or "keybnd" still reach their setting. gated on nothing
        // cleaner matching and bounded, so it stays cheap per keystroke.
        if (best < 600 && w.length >= 4) {
            for (var t = 0; t < tws.length; t++) {
                var tw2 = tws[t];
                if (tw2.length < 3) continue;
                var d = editDistance(w, tw2, w.length >= 7 ? 2 : 1);
                if (d >= 0) { var es = 640 - d * 140; if (es > best) best = es; }
            }
        }
        return best;
    }
    // Tolerant multi-word scoring over expanded groups: each term scores its best
    // synonym (a synonym counts a touch less than the typed word), summed and
    // scaled by how much of the query landed, so one out-of-vocab word never
    // blanks the rail and a synonym or typo still finds the page.
    function searchScore(groups, text) {
        var total = 0, matched = 0, n = 0;
        for (var gi = 0; gi < groups.length; gi++) {
            n++;
            var g = groups[gi], best = 0;
            for (var wi = 0; wi < g.length; wi++) {
                var s = wordScore(g[wi], text);
                if (wi > 0) s *= 0.85;
                if (s > best) best = s;
            }
            if (best > 0) { total += best; matched++; }
        }
        if (matched === 0 || n === 0) return 0;
        return total * (matched / n);
    }
    readonly property var searchResults: {
        var q = query.toLowerCase().trim();
        if (q === "") return [];
        var groups = expandQuery(q);
        var scored = [];
        for (var i = 0; i < searchIndex.length; i++) {
            var e = searchIndex[i];
            var full = (e.label + " " + e.desc + " " + e.sectionName + " " + e.group + " " + e.tab + " " + e.kw).toLowerCase();
            var s = searchScore(groups, full);
            if (s <= 0) continue;
            s += 2 * searchScore(groups, e.label.toLowerCase());
            var kwHit = searchScore(groups, (e.sectionName + " " + e.tab + " " + e.kw).toLowerCase());
            s += kwHit;
            if (e.isPage) s += 300 + 3 * kwHit;
            scored.push({ e: e, s: s });
        }
        scored.sort(function (a, b) { return b.s - a.s; });
        var out = [];
        for (var j = 0; j < Math.min(scored.length, 60); j++) out.push(scored[j].e);
        return out;
    }

    // Layout class per section, kept as sets so the chrome is derived from the
    // section and never from the mid-load item: that is what stops the rail,
    // side column and bar reflowing (flickering) during an async page swap.
    // `framed` pages keep the rail + bottom action bar; `ledger` pages also get
    // the right write-ledger column. Everything else is full-bleed.
    readonly property var framedSet: ({
        "bar-studio": true, "desktop": true, "plugins": true, "input": true, "animations": true, "global": true, "windowmanager": true,
        "windowrules": true, "appoverrides": true, "layerrules": true,
        "session": true
    })
    // A section drives the compositor when any of its rows targets the neutral
    // window-manager store (src desktop.json), derived from the schema so it
    // tracks the rows instead of a hand-kept list. A shell section gets the red
    // rail marker, reading apart from the compositor ones at a glance.
    readonly property var compositorSections: {
        var m = {};
        for (var k in hub.sectionRows) {
            var rows = hub.sectionRows[k] || [];
            for (var i = 0; i < rows.length; i++)
                if (rows[i].src === "desktop.json") { m[k] = true; break; }
        }
        return m;
    }
    function isShellSection(key) { return !hub.compositorSections[key]; }

    readonly property var pageMeta: ({})
    function metaFor(s) {
        return hub.pageMeta[s] || { title: hub.nameFor(s), eyebrow: hub.groupFor(s), blurb: "" };
    }
    function nameFor(s) {
        for (var g = 0; g < groups.length; g++)
            for (var i = 0; i < groups[g].items.length; i++)
                if (groups[g].items[i].key === s) return groups[g].items[i].name;
        return s;
    }
    function groupFor(s) {
        for (var g = 0; g < groups.length; g++)
            for (var i = 0; i < groups[g].items.length; i++)
                if (groups[g].items[i].key === s) return groups[g].name || "SETTINGS";
        return "SETTINGS";
    }
    function isWired(s) {
        for (var g = 0; g < groups.length; g++)
            for (var i = 0; i < groups[g].items.length; i++)
                if (groups[g].items[i].key === s) return groups[g].items[i].wired === true;
        return false;
    }
    // What a catalogue item's `needs` demands of the active window manager: a
    // behavioural capability (`cap`), a provider being present at all (`provider`),
    // or the page holding at least one row the provider actually backs (`rows`).
    // No `needs` -> the page is compositor-neutral and always available. This is
    // the page-level twin of SchemaPage's per-row gate, so a page is never shown
    // when nothing on it can be written; the rail, router, deep link and search
    // all run it, so a filtered page is unreachable, not merely hidden.
    function needsMet(item) {
        var n = item ? item.needs : undefined;
        if (!n) return true;
        if (n.cap !== undefined && !Settings.supports(n.cap)) return false;
        if (n.provider === true && Settings.provider === "") return false;
        if (n.rows === true && !hub.hasModeledRow(item.key)) return false;
        return true;
    }
    function itemFor(s) {
        for (var g = 0; g < groups.length; g++)
            for (var i = 0; i < groups[g].items.length; i++)
                if (groups[g].items[i].key === s) return groups[g].items[i];
        return null;
    }
    function sectionAvailable(s) { var it = hub.itemFor(s); return it ? hub.needsMet(it) : true; }
    // Whether the active provider backs at least one setting on this page, tested
    // against the effective row set under the same supports + modelsKey gate the
    // rows themselves pass. A keyless row (a header or an action) is not a setting
    // the provider models, so it never keeps an otherwise-empty page alive.
    function hasModeledRow(s) {
        var rows = hub.effectiveRows(s);
        for (var i = 0; i < rows.length; i++) {
            var r = rows[i];
            if (!r || !r.key) continue;
            if (r.caps && !Settings.supports(r.caps)) continue;
            if (Settings.modelsKey(r.key)) return true;
        }
        return false;
    }
    // The rows that back a page: the Hub's static schema for it plus the active
    // provider's own schema rows for the same page, so a row that has migrated
    // into the provider still counts. One source, so the gate and the page never
    // read a different row set.
    function effectiveRows(s) {
        var base = hub.sectionRows[s] || [];
        var prov = ProviderSchema.rowsFor(s);
        return (prov && prov.length) ? base.concat(prov) : base;
    }
    function pageFile(s) {
        var map = { "plugins": "PluginsPage", "profile": "ProfilePage", "bar-studio": "BarStudioPage", "desktop": "DesktopPage", "session": "SessionPage", "layerrules": "LayerRulesPage", "windowrules": "WindowRulesPage", "appoverrides": "AppOverridesPage", "animations": "AnimationsPage", "input": "InputPage", "keybinds": "KeybindsPage", "dictation": "DictationPage", "displays": "DisplaysPage", "connections": "ConnectionsPage", "gpu": "GpuPage", "updates": "UpdatesPage", "rashin": "RashinPage", "recording": "RecordingPage", "performance": "PerformancePage", "launcher": "LauncherPage", "lockscreen": "LockscreenPage", "fastfetch": "FastfetchPage", "addons": "AddonsPage", "widgets": "WidgetsPage", "credits": "CreditsPage" };
        map.global = "GlobalPage";
        map["import"] = "ImportPage";
        map.windowmanager = "WindowManagerPage";
        return map[s] ? Qt.resolvedUrl("pages/" + map[s] + ".qml") : "";
    }
    function openPick(r) { picker.openFor(r); }

    // Compositor actions and the live window list for the pages, over the daemon
    // wm seam so no page forks a compositor CLI.
    readonly property var wmWindows: Settings.windows
    readonly property var wmConfigFiles: Settings.configFiles
    function wmAct(action, args) { Settings.send("wm.act", { action: action, args: args || [] }); }

    // ── the store ─────────────────────────────────────────────────────────
    // draft is the live full map; committed mirrors disk; defs are factory.
    property var draft: ({})
    property var committed: ({})
    property bool pristine: true

    readonly property var defs: ({
        "frameRadius": 9, "roundness": 10, "frameBorder": 59, "frameEnabled": true,
        "frameSmoothing": 8, "frameOpacity": 1, "shadowStrength": 0.63, "shadowSize": 12,
        "frameThickness": 2, "frameCorner": 8,
        "surfaceColor": "#0f1115", "osdRadius": 28, "osdOpacity": 1,
        "fontFamily": "Space Grotesk", "fontSize": 11, "fontScale": 1.3,
        "frameBars": FrameBars.defaultConfig(),
        "frameBars.menus.quick-settings.anchor": "left",
        "frameBars.menus.quick-settings.expansion": "always",
        "frameBars.menus.quick-settings.minWidth": 410,
        "weatherLocation": "", "weatherUnit": "auto", "formatLocale": "",
        "enabled": true, "bars": 64, "thickness": 0.58, "bloom": 0.6,
        "reflection": 0.1, "idleWave": true, "style": "bars", "shape": "rounded",
        "mirror": false, "segments": 10, "fps": 30,
        "adaptive": true, "smoothing": 0.5, "gain": 1.0, "peaks": false,
        "spin": 0, "x": 0, "y": 0.58, "w": 1, "h": 0.42, "grow": "up", "angle": 0, "tiltX": 0, "tiltY": 0,
        "markText": "力", "markImage": "", "markTint": true, "name": "Ryoku",
        "reloadCover": ReloadCoverModel.empty(),
        "language": "Auto", "barStyle": "sumi", "obi": {}, "nacre": NacreConfig.defaultConfig(), "qsbar": {}, "dock": {},
        "clipboard.widthPercent": 65, "clipboard.heightPercent": 42, "clipboard.bottomPercent": 0,
        "clipboard.panelRadius": 18, "clipboard.paneRadius": 12, "clipboard.cardRadius": 9,
        "clipboard.pruneWeekly": false
    })

    // key -> source file, derived from the schema so it cannot drift.
    readonly property var srcOf: {
        var m = {};
        var rows = DesktopSchema.rows;
        for (var i = 0; i < rows.length; i++) {
            var r = rows[i];
            if (r.src && r.src !== "none") m[r.key] = r.src;
        }
        m.frameBars = "shell";
        return m;
    }
    function adapterFor(src) { return src === "viz" ? vizA : brandA; }
    function fileFor(src) { return src === "viz" ? "visualizer" : (src === "brand" ? "brand" : "shell"); }

    // The full config files backing the current page: the source of truth the GUI
    // writes and the user can hand-edit in place (every value present, not a sparse
    // overlay). Plus the shipped base they layer over. Opened from the FILES
    // masthead chip; the overlay model lives in docs/updates.md.
    function settingsFiles() {
        var s = hub.section;
        if (hub.pageFile(s) === "") return [];
        if (s === "import") return [];
        var ce = hub.cfgDir;
        var home = Quickshell.env("HOME") || "";
        var out = [];
        if (hub.compositorSections[s]) {
            out.push({ role: "yours", label: "Your config", path: ce + "/desktop.json",
                note: "Every window-manager setting the pages control, with your values, in one full file you edit in place. The GUI writes this same file and reads your hand-edits back on open." });
            var cf = Settings.configFiles || [];
            for (var ci = 0; ci < cf.length; ci++)
                out.push({ role: "advanced", label: cf[ci].split("/").pop(), path: home + "/.config/" + cf[ci],
                    note: "Config the compositor loads for anything the GUI does not expose. Provider-owned, and updates never touch it." });
        } else {
            out.push({ role: "yours", label: "Your shell config", path: ce + "/shell.json",
                note: "Every shell setting, with your values, in one full file you edit in place. The GUI writes it and the shell retunes live when you save a hand-edit." });
            if (s === "desktop") {
                out.push({ role: "yours", label: "Widget visuals", path: ce + "/visualizer.json",
                    note: "Visualiser and desktop-widget visuals, a full file you edit in place." });
                out.push({ role: "yours", label: "Brand mark", path: ce + "/brand.json",
                    note: "The brand mark and hero, a full file you edit in place." });
            }
            out.push({ role: "base", label: "Shipped defaults", noOpen: true,
                note: "Ship in the shell package and refresh each update; your config above overrides them." });
        }
        return out;
    }
    function tildePath(p) {
        var home = Quickshell.env("HOME") || "";
        return (home && p.indexOf(home) === 0) ? "~" + p.substring(home.length) : p;
    }
    // Ensure a user file exists (seeding a header when given), then open it via
    // xdg-open (mimeapps route text/json/lua to ryoku-nvim); a base dir opens in
    // the file manager.
    function openSettingsFile(e) {
        if (!e || e.noOpen || !e.path)
            return;
        var dir = e.path.substring(0, e.path.lastIndexOf("/"));
        var sh = "mkdir -p \"" + dir + "\"; ";
        if (e.seed)
            sh += "[ -e \"" + e.path + "\" ] || printf \"" + e.seed + "\" > \"" + e.path + "\"; ";
        sh += "xdg-open \"" + e.path + "\"";
        Spawn.run(["sh", "-c", sh]);
    }

    function snapshot() {
        var s = {};
        for (var k in defs) {
            var src = srcOf[k] || "shell";
            if (src === "shell") { var v = Settings.get(k); s[k] = v === undefined ? defs[k] : v; }
            else s[k] = adapterFor(src)[k];
        }
        return s;
    }
    function rebase() {
        hub.committed = snapshot();
        if (hub.pristine) hub.draft = JSON.parse(JSON.stringify(hub.committed));
        // the first real daemon frame is the saved state Bar Studio's live
        // edits are measured against (and walked back to on an unsaved close)
        if (!hub.liveBaseline && Settings.ready) hub.captureLiveBaseline();
    }
    function val(k) { var v = draft[k]; return v === undefined ? committed[k] : v; }
    function edit(k, v) {
        hub.pristine = false;
        var d = {}; for (var x in draft) d[x] = draft[x];
        d[k] = v; hub.draft = d;
    }
    readonly property int dirty: {
        var n = 0;
        for (var k in defs) {
            if (draft[k] === undefined || committed[k] === undefined) continue;
            if (JSON.stringify(draft[k]) !== JSON.stringify(committed[k])) n++;
        }
        return n + hub.hyprChanges().length + (hub.pageDirty ? 1 : 0) + hub.liveChanges.length;
    }

    // ── Bar Studio live-apply ────────────────────────────────────────────
    // The frame keys and frameBars are native settings-daemon keys, so a Bar
    // Studio edit applies to the RUNNING desktop as it is staged: stageLive
    // stages the draft like any edit and rides a coalesced settings.patch to
    // the daemon, which repaints the pill live. The daemon's re-push rebases
    // committed onto the patched value, so the unsaved distance is kept here
    // against liveBaseline: the state at open, re-snapshotted on every Save.
    // Quit and Revert walk the desktop back to that baseline through the same
    // channel, so an unsaved close leaves no residue.
    readonly property var liveKeys: ["frameBars", "frameEnabled", "frameOpacity", "frameThickness", "frameCorner", "fontFamily", "fontSize", "barStyle", "obi", "nacre", "qsbar", "dock", "clipboard.widthPercent", "clipboard.heightPercent", "clipboard.bottomPercent", "clipboard.panelRadius", "clipboard.paneRadius", "clipboard.cardRadius"]
    property var liveBaseline: null
    property var livePending: ({})
    function captureLiveBaseline() {
        var b = {};
        for (var i = 0; i < hub.liveKeys.length; i++) {
            var k = hub.liveKeys[i];
            var v = hub.val(k);
            b[k] = JSON.parse(JSON.stringify(v === undefined ? hub.defs[k] : v));
        }
        hub.liveBaseline = b;
    }
    function stageLive(k, v) {
        hub.edit(k, v);
        var p = {};
        for (var x in hub.livePending) p[x] = true;
        p[k] = true;
        hub.livePending = p;
        liveFlush.restart();
    }
    Timer {
        id: liveFlush
        interval: 130
        onTriggered: {
            for (var k in hub.livePending) Settings.patch(k, hub.val(k));
            hub.livePending = ({});
        }
    }
    // the live-applied keys that have drifted from the saved baseline
    readonly property var liveChanges: {
        var out = [];
        if (!hub.liveBaseline) return out;
        for (var i = 0; i < hub.liveKeys.length; i++) {
            var k = hub.liveKeys[i];
            var v = hub.val(k);
            if (v === undefined) continue;
            if (JSON.stringify(v) !== JSON.stringify(hub.liveBaseline[k])) out.push(k);
        }
        return out;
    }
    // put the saved state back on the desktop; true if anything was patched
    function restoreLiveUnsaved() {
        liveFlush.stop();
        hub.livePending = ({});
        if (!hub.liveBaseline) return false;
        var changed = hub.liveChanges;
        if (!changed.length) return false;
        var d = {};
        for (var x in hub.draft) d[x] = hub.draft[x];
        for (var i = 0; i < changed.length; i++) {
            var k = changed[i];
            Settings.patch(k, hub.liveBaseline[k]);
            d[k] = JSON.parse(JSON.stringify(hub.liveBaseline[k]));
        }
        hub.draft = d;
        return true;
    }
    function save() {
        var files = {};
        for (var k in defs) {
            if (draft[k] === undefined || committed[k] === undefined) continue;
            if (JSON.stringify(draft[k]) === JSON.stringify(committed[k])) continue;
            var src = srcOf[k] || "shell";
            if (src === "shell") Settings.patch(k, draft[k]);
            else adapterFor(src)[k] = draft[k];
            files[src] = true;
        }
        if (files.viz) vizFV.writeAdapter();
        if (files.brand) {
            brandFV.writeAdapter();
            hub.requestReloadCoverPrune(hub.draft.reloadCover);
        }
        hub.committed = JSON.parse(JSON.stringify(hub.draft));
        // language rides the normal Save: the file it just wrote (shell.json) is
        // watched by every Ryoku surface's shared I18n, so the whole desktop
        // retranslates. Set it here too so this window switches deterministically.
        if (files.shell) I18n.configLang = hub.committed.language || "Auto";
        if (hub.hyprChanges().length) {
            hyprSave.command = ["ryoku-hub", "desktop", "save", JSON.stringify(hub.hyprDraft)];
            hyprSave.running = true;
            hub.hyprCommitted = JSON.parse(JSON.stringify(hub.hyprDraft));
        }
        hub.captureLiveBaseline();
        hub.savePage();
    }
    function revert() {
        hub.invalidatePageWork();
        hub.draft = JSON.parse(JSON.stringify(hub.committed));
        hub.restoreLiveUnsaved();
        hub.hyprDraft = JSON.parse(JSON.stringify(hub.hyprCommitted));
        if (hub.wmLoaded) { hyprRestore.command = ["ryoku-hub", "desktop", "restore"]; hyprRestore.running = true; }
        hub.revertPage();
        hub.requestReloadCoverPrune(hub.committed.reloadCover);
    }
    function resetDefaults() {
        hub.invalidatePageWork();
        hub.pristine = false;
        hub.draft = JSON.parse(JSON.stringify(hub.defs));
        if (Object.keys(hub.hyprDefaults).length) hub.hyprDraft = JSON.parse(JSON.stringify(hub.hyprDefaults));
    }

    property string reloadCoverCleanupError: ""
    property string reloadCoverPruneWanted: ""
    function requestReloadCoverPrune(value) {
        reloadCoverPruneWanted = ReloadCoverModel.path(value);
        if (!reloadCoverPrune.running)
            reloadCoverPruneKick.restart();
    }
    Timer {
        id: reloadCoverPruneKick
        interval: 20
        onTriggered: {
            if (reloadCoverPrune.running)
                return;
            var command = ["ryoku-hub", "reload-cover", "prune"];
            if (hub.reloadCoverPruneWanted !== "")
                command.push(hub.reloadCoverPruneWanted);
            reloadCoverPrune.keep = hub.reloadCoverPruneWanted;
            reloadCoverPrune.command = command;
            reloadCoverPrune.running = true;
        }
    }
    Process {
        id: reloadCoverPrune
        property string keep: ""
        stderr: StdioCollector { id: reloadCoverPruneStderr }
        onExited: function(code) {
            if (code !== 0)
                hub.reloadCoverCleanupError = reloadCoverPruneStderr.text.trim() || I18n.tr("Couldn't clean old reload-cover assets.");
            else
                hub.reloadCoverCleanupError = "";
            if (keep !== hub.reloadCoverPruneWanted)
                reloadCoverPruneKick.restart();
        }
    }

    // the diff, grouped by file, in each file's own JSON syntax.
    readonly property var diff: {
        var by = { shell: [], viz: [], brand: [] };
        for (var k in defs) {
            if (draft[k] === undefined || committed[k] === undefined) continue;
            if (JSON.stringify(draft[k]) === JSON.stringify(committed[k])) continue;
            var src = srcOf[k] || "shell";
            by[src].push({ key: k, was: JSON.stringify(committed[k]), now: JSON.stringify(draft[k]) });
        }
        var out = [];
        var order = ["shell", "viz", "brand"];
        for (var i = 0; i < order.length; i++)
            if (by[order[i]].length)
                out.push({ file: hub.fileFor(order[i]) + ".json", changes: by[order[i]] });
        var hc = hub.hyprChanges();
        if (hc.length) out.push({ file: "settings.lua", changes: hc });
        return out;
    }

    Component.onCompleted: rebase()
    Process {
        id: sectionGet
        command: ["ryoku-hub", "config", "get", "section"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                if (hub.navigated)
                    return;
                var s = hub.canonicalSection(this.text.trim());
                if (s && hub.pageFile(s) !== "" && hub.sectionAvailable(s)) hub.section = s;
            }
        }
    }
    Process {
        id: advancedGet
        command: ["ryoku-hub", "config", "get", "advanced"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: { if (this.text.trim() === "1") hub.advanced = true; }
        }
    }

    property string cfgDir: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku"

    // shell.json is owned by the ryoku-shell settings daemon; the Hub reads it
    // through the `settings` topic and writes it only through settings.patch
    // (Singletons/Settings.qml), never opening the file itself. A fresh daemon
    // frame rebases committed onto the pushed values, the way the FileView
    // reload did before the daemon owned the schema.
    Connections {
        target: Settings
        function onRevisionChanged() { hub.rebase(); }
    }
    FileView {
        id: vizFV
        path: hub.cfgDir + "/visualizer.json"
        watchChanges: true
        onFileChanged: reload()
        onLoaded: hub.rebase()
        JsonAdapter {
            id: vizA
            property bool enabled: true
            property real bars: 64
            property real thickness: 0.58
            property real bloom: 0.6
            property real reflection: 0.1
            property bool idleWave: true
            property string style: "bars"
            property string shape: "rounded"
            property string color: ""
            property string color2: ""
            property bool gradient: false
            property bool mirror: false
            property real segments: 10
            property real fps: 30
            property bool adaptive: true
            property real smoothing: 0.5
            property real gain: 1.0
            property bool peaks: false
            property real spin: 0
            property real x: 0
            property real y: 0.58
            property real w: 1
            property real h: 0.42
            property string grow: "up"
        property real angle: 0
        property real tiltX: 0
        property real tiltY: 0
        // Preserved so a hub save never drops the desktop's extra visualisers or
        // which one it is editing; the hub itself tunes the primary (flat keys).
        property var extras: []
        property int active: 0
        }
    }
    FileView {
        id: brandFV
        path: hub.cfgDir + "/brand.json"
        watchChanges: true
        onFileChanged: reload()
        onLoaded: {
            brandA.reloadCover = ReloadCoverModel.normalize(brandA.reloadCover);
            hub.rebase();
            hub.requestReloadCoverPrune(brandA.reloadCover);
        }
        JsonAdapter {
            id: brandA
            property string markText: "力"
            property string markImage: ""
            property bool markTint: true
            property string name: "Ryoku"
            property var reloadCover: ReloadCoverModel.empty()
        }
    }

    // ── hypr backend (the Lua pages) ─────────────────────────────────────
    // Every Lua page persists through one nested object via ryoku-hub, not a
    // JsonAdapter. Pages read and write dotted paths (appearance.gapsIn); the
    // change shows up in the same dirty/diff/save the JSON pages use, grouped
    // under settings.lua. Nothing is written until Save calls `hypr save`.
    property var hyprCommitted: ({})
    property var hyprDraft: ({})
    property var hyprDefaults: ({})
    property bool wmLoaded: false

    // A bespoke page (e.g. Appearance > Theme) that owns its own edits can route
    // them through the shared action bar: it raises pageDirty while it holds
    // staged changes (lighting Save), and Save/Revert emit these so the page
    // applies or drops them in lockstep with everything else.
    property bool pageDirty: false
    signal savePage()
    signal revertPage()
    signal invalidatePageWork()

    Process {
        id: hyprGet
        command: ["ryoku-hub", "desktop", "get"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var o = JSON.parse(this.text);
                    hub.hyprCommitted = o;
                    if (hub.pristine || !hub.wmLoaded) hub.hyprDraft = JSON.parse(JSON.stringify(o));
                    hub.wmLoaded = true;
                } catch (e) { console.log("hub: hypr get parse failed: " + e); }
            }
        }
    }
    Process {
        id: hyprDefaultsGet
        command: ["ryoku-hub", "desktop", "defaults"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: { try { hub.hyprDefaults = JSON.parse(this.text); } catch (e) {} }
        }
    }
    Process { id: hyprSave }

    // live preview: the shell owns it, not the pages. A hypr edit applies to the
    // running compositor (throttled) so "previewing live" is honest; revert and
    // an unsaved quit restore the compositor to what is on disk.
    property bool quitting: false
    onHyprDraftChanged: if (hub.wmLoaded) hyprPreviewThrottle.restart()
    Timer {
        id: hyprPreviewThrottle
        interval: 140
        onTriggered: {
            hyprPreview.command = ["ryoku-hub", "desktop", "preview", JSON.stringify(hub.hyprDraft)];
            hyprPreview.running = true;
        }
    }
    Process { id: hyprPreview }
    Process { id: hyprRestore; onRunningChanged: if (!running && hub.quitting) Qt.quit() }
    function requestQuit() {
        if (hub.quitting) return;
        if (brandFV.loaded) {
            var cleanup = ["ryoku-hub", "reload-cover", "prune"];
            var keep = ReloadCoverModel.path(hub.committed.reloadCover);
            if (keep !== "")
                cleanup.push(keep);
            Spawn.run(cleanup);
        }
        // Bar Studio edits are already live on the desktop: an unsaved quit
        // puts the saved state back through the same channel before the Hub
        // goes, whichever way it was closed.
        var restored = hub.restoreLiveUnsaved();
        if (hub.wmLoaded && hub.hyprChanges().length) {
            hub.quitting = true;
            hyprRestore.command = ["ryoku-hub", "desktop", "restore"];
            hyprRestore.running = true;
        } else if (restored) {
            // hold the door one beat so the control socket flushes the patches
            hub.quitting = true;
            quitFlush.restart();
        } else {
            Qt.quit();
        }
    }
    Timer { id: quitFlush; interval: 120; onTriggered: Qt.quit() }

    function hyprVal(path) { return hub.pathGet(hub.hyprDraft, path); }
    function hyprCommittedVal(path) { return hub.pathGet(hub.hyprCommitted, path); }
    function hyprEdit(path, v) {
        hub.pristine = false;
        var d = JSON.parse(JSON.stringify(hub.hyprDraft));
        hub.pathSet(d, path, v);
        hub.hyprDraft = d;
    }
    // forget a subtree the backend already removed from the store (a plugin the
    // Plugins page dropped): out of the draft so the next Save does not put it
    // back, and out of committed so nothing reads as an unsaved change.
    function hyprDrop(path) {
        var drop = function (root) {
            var d = JSON.parse(JSON.stringify(root));
            var parts = path.split("."), cur = d;
            for (var i = 0; i < parts.length - 1; i++) {
                cur = cur[parts[i]];
                if (typeof cur !== "object" || cur === null) return root;
            }
            delete cur[parts[parts.length - 1]];
            return d;
        };
        hub.hyprDraft = drop(hub.hyprDraft);
        hub.hyprCommitted = drop(hub.hyprCommitted);
    }
    function pathGet(obj, path) {
        var parts = path.split("."), cur = obj;
        for (var i = 0; i < parts.length; i++) { if (cur === undefined || cur === null) return undefined; cur = cur[parts[i]]; }
        return cur;
    }
    function pathSet(obj, path, v) {
        var parts = path.split("."), cur = obj;
        for (var i = 0; i < parts.length - 1; i++) {
            if (typeof cur[parts[i]] !== "object" || cur[parts[i]] === null) cur[parts[i]] = {};
            cur = cur[parts[i]];
        }
        cur[parts[parts.length - 1]] = v;
    }
    function hyprChanges() {
        if (!hub.wmLoaded) return [];
        var out = [];
        hub.walkHypr("", hub.hyprCommitted, hub.hyprDraft, out);
        return out;
    }
    function walkHypr(prefix, a, b, out) {
        var seen = {}, k;
        for (k in (b || {})) seen[k] = true;
        for (k in (a || {})) seen[k] = true;
        for (k in seen) {
            var pa = a ? a[k] : undefined, pb = b ? b[k] : undefined;
            var p = prefix ? prefix + "." + k : k;
            var oa = pa && typeof pa === "object" && !Array.isArray(pa);
            var ob = pb && typeof pb === "object" && !Array.isArray(pb);
            if (oa && ob) hub.walkHypr(p, pa, pb, out);
            // a key on only one side (added or removed) is a change too, not only
            // a modified value, so first-time sets of omitempty maps (apps,
            // keybindRebinds) mark the store dirty and light Save.
            else if (JSON.stringify(pa) !== JSON.stringify(pb)) out.push({ key: p, was: pa === undefined ? "(unset)" : JSON.stringify(pa), now: pb === undefined ? "(unset)" : JSON.stringify(pb) });
        }
    }

    Keys.onEscapePressed: {
        if (diffPop.open) diffPop.open = false;
        else if (hub.query !== "") { hub.query = ""; searchField.clear(); }
        else hub.requestQuit();
    }
    Keys.onPressed: (e) => {
        if (e.key === Qt.Key_K && (e.modifiers & Qt.ControlModifier)) {
            searchField.grabFocus();
            e.accepted = true;
        }
    }

    // ── rail ────────────────────────────────────────────────────────────
    Item {
        id: rail
        anchors { left: parent.left; top: parent.top; bottom: bar.top }
        width: Tokens.railW
        Rectangle { anchors.right: parent.right; width: 1; height: parent.height; color: Tokens.line }

        Column {
            id: railHead
            anchors { left: parent.left; right: parent.right; top: parent.top }
            anchors.margins: Tokens.s5
            spacing: Tokens.s4

            // the masthead as a poster plate: framed, register-ticked, with the
            // seal and a /// mark. The reference sheet's title block, scaled down.
            Rectangle {
                width: parent.width
                height: 64
                color: "transparent"
                radius: Tokens.radius
                border.width: Tokens.border
                border.color: Tokens.line
                Row {
                    anchors { left: parent.left; verticalCenter: parent.verticalCenter; leftMargin: Tokens.s4 }
                    spacing: Tokens.s3
                    Text { text: "力"; color: Tokens.ink; font.family: Tokens.jp; font.pixelSize: 22 }
                    Column {
                        spacing: 1
                        anchors.verticalCenter: parent.verticalCenter
                        Text {
                            text: I18n.tr("RYOKU ARCH"); color: Tokens.ink; font.family: Tokens.ui
                            font.pixelSize: 14; font.weight: Font.Medium; font.letterSpacing: 2.4
                        }
                        Text {
                            text: I18n.tr("SETTINGS"); color: Tokens.inkMuted
                            font.family: Tokens.mono; font.pixelSize: 10; font.letterSpacing: 1.4
                        }
                    }
                }
            }
            Field {
                id: searchField
                width: parent.width
                toolbar: true
                placeholder: I18n.tr("Search settings…")
                onEdited: (t) => hub.query = t
                onAccepted: hub.activateSearch(0)
            }
        }

        // The one global switch at the foot: Advanced reveals the deep per-page
        // knobs inside a schema page (the rail always lists every section). It
        // persists and restores at startup by `advancedGet` / the settings
        // daemon.
        Item {
            id: advToggle
            anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
            anchors.leftMargin: Tokens.s5; anchors.rightMargin: Tokens.s5
            anchors.bottomMargin: Tokens.s4
            height: Tokens.ctlH
            Rectangle {
                anchors { left: parent.left; right: parent.right; top: parent.top; topMargin: -Tokens.s3 }
                height: 1; color: Tokens.lineSoft
            }
            Text {
                anchors { left: parent.left; right: advSw.left; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                elide: Text.ElideRight
                text: I18n.tr("Advanced")
                color: hub.advanced ? Tokens.ink : Tokens.inkMuted
                font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
            }
            Sw {
                id: advSw
                anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                on: hub.advanced
                onToggled: (v) => hub.advanced = v
            }
        }

        Flickable {
            id: navFlick
            anchors { left: parent.left; right: parent.right; top: railHead.bottom; bottom: advToggle.top }
            anchors.margins: Tokens.s5
            anchors.topMargin: Tokens.s4
            contentHeight: nav.height
            clip: true
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }

            // keep the active section on screen: jumping there by search, IPC or
            // a config-restored section must never leave the selection clipped at
            // the rail edge. Only scrolls when the item is actually off-screen,
            // and animates the scroll, so an in-view click never jumps the rail.
            function reveal(it) {
                var y = it.mapToItem(nav, 0, 0).y;
                if (y >= contentY && y + it.height <= contentY + height)
                    return;
                var max = Math.max(0, nav.height - height);
                railScroll.to = Math.max(0, Math.min(y - height / 2, max));
                railScroll.restart();
            }
            NumberAnimation {
                id: railScroll
                target: navFlick; property: "contentY"
                duration: Tokens.move; easing.type: Tokens.ease
            }

            Column {
                id: nav
                width: navFlick.width - 12
                spacing: 0

                Repeater {
                    model: hub.groups
                    Column {
                        id: grp
                        required property var modelData
                        required property int index
                        width: nav.width
                        spacing: 0

                        // a breath between groups, so the rail reads as clusters
                        // rather than one long list of rows
                        Item { width: 1; height: grp.index === 0 ? 0 : Tokens.s2 }

                        // which group holds the open section: its header lifts up
                        // the ink ramp (faint -> dim) as a quiet "you are here",
                        // monochrome, never a colour, so the bone-plate item stays
                        // the one emphasis.
                        readonly property bool activeGroup: grp.modelData.items.some(function (i) { return i.key === hub.section; })

                        Item {
                            // the header shows only while the group has a visible
                            // item: a section the active provider cannot back, or a
                            // power-user (adv) section with Advanced off, drops out,
                            // and a group whose every item drops out folds its header
                            // away instead of leaving a bare label. The open section
                            // always counts, so you never lose your place.
                            readonly property bool anyShown: grp.modelData.items.some(function (i) {
                                return hub.needsMet(i) && (!i.adv || hub.advanced || hub.section === i.key);
                            })
                            width: parent.width
                            height: !anyShown ? 0 : (grp.modelData.name === "" ? Tokens.s4 : 34)
                            visible: anyShown
                            Row {
                                visible: grp.modelData.name !== ""
                                anchors.verticalCenter: parent.verticalCenter
                                anchors.bottomMargin: 2
                                spacing: Tokens.s2
                                // the poster's plate numerals: each group carries
                                // its index, 01..05, in tabular mono.
                                Text {
                                    text: (grp.index + 1 < 10 ? "0" : "") + (grp.index + 1)
                                    color: grp.activeGroup ? Tokens.inkDim : Tokens.inkFaint
                                    font.family: Tokens.mono; font.pixelSize: 9
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                                Text {
                                    text: I18n.tr(grp.modelData.name); color: grp.activeGroup ? Tokens.inkDim : Tokens.inkFaint
                                    font.family: Tokens.ui; font.pixelSize: 9
                                    font.weight: Font.Medium; font.letterSpacing: 2
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                                Rectangle {
                                    width: Math.max(0, nav.width - 130); height: 1; color: Tokens.lineSoft
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                                Rectangle {
                                    width: 1; height: 5; color: grp.activeGroup ? Tokens.lineStrong : Tokens.line
                                    anchors.verticalCenter: parent.verticalCenter
                                    anchors.verticalCenterOffset: -2
                                }
                            }
                        }

                        Repeater {
                            model: grp.modelData.items
                            Item {
                                id: navItem
                                required property var modelData
                                // Advanced off hides the power-user (adv) sections
                                // from the rail while search still reaches them; a
                                // section the active provider cannot back is hidden
                                // outright and unreachable. The open section stays put
                                // so turning Advanced off never strands you on a page
                                // the rail no longer lists.
                                readonly property bool shown: hub.needsMet(modelData)
                                    && (!modelData.adv || hub.advanced || hub.section === modelData.key)
                                width: nav.width
                                height: shown ? 36 : 0
                                visible: shown
                                readonly property bool sel: hub.section === modelData.key
                                onSelChanged: if (sel) navFlick.reveal(navItem)
                                Component.onCompleted: if (sel) navFlick.reveal(navItem)

                                Rectangle {
                                    anchors.fill: parent
                                    anchors.topMargin: 1; anchors.bottomMargin: 1
                                    radius: Tokens.radius
                                    color: navItem.sel ? Tokens.bone : (nh.hovered ? Tokens.tint10 : "transparent")
                                    Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                }
                                // selection is typography, never a coloured bar:
                                // the live section takes the sheet's // lead. On
                                // the right, every item carries its kanji seal,
                                // Latin and Japanese sitting side by side.
                                Row {
                                    x: Tokens.s3
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: Tokens.s2
                                    Text {
                                        id: navLead
                                        visible: navItem.sel
                                        text: "//"
                                        color: Tokens.inkOnBoneDim
                                        font.family: Tokens.mono; font.pixelSize: 11
                                        anchors.verticalCenter: parent.verticalCenter
                                    }
                                    Text {
                                        id: navLatin
                                        text: I18n.tr(navItem.modelData.name)
                                        color: navItem.sel ? Tokens.inkOnBone : Tokens.inkDim
                                        font.family: Tokens.ui; font.pixelSize: 14
                                        anchors.verticalCenter: parent.verticalCenter
                                        elide: Text.ElideRight
                                        width: Math.max(0, navKana.x - Tokens.s2 - Tokens.s3
                                            - (navItem.sel ? navLead.width + Tokens.s2 : 0))
                                        Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                    }
                                }
                                Text {
                                    id: navKana
                                    anchors { right: parent.right; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                                    text: hub.jpName[navItem.modelData.key] || ""
                                    color: navItem.sel ? Tokens.inkOnBoneDim : Tokens.inkFaint
                                    font.family: Tokens.jp; font.pixelSize: 12
                                    Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                }
                                HoverHandler { id: nh; cursorShape: Qt.PointingHandCursor }
                                TapHandler { onTapped: { hub.query = ""; searchField.clear(); hub.section = navItem.modelData.key } }
                            }
                        }
                    }
                }
            }
        }
    }

    // ── the page ──────────────────────────────────────────────────────────
    Item {
        id: pageArea
        // full/framed and the ledger are derived from the section via the sets
        // above, never the loaded item, so the chrome holds still through an
        // async page swap (only the page content fades). A porting page (no
        // file) stays framed too.
        readonly property bool full: hub.pageFile(hub.section) !== "" && !hub.framedSet[hub.section]
        anchors.top: parent.top
        anchors.bottom: pageArea.full ? parent.bottom : bar.top
        anchors.topMargin: pageArea.full ? 0 : Tokens.s5
        anchors.bottomMargin: pageArea.full ? 0 : Tokens.s3
        // Framed pages fill the window beside the rail: the Hub opens
        // page-wide, and a page that refuses the width it was given wastes it.
        // Nothing needs a global cap -- a page that reads better on a shorter
        // measure (a paragraph, a list, a release note) caps its own blocks, and
        // a grid page caps itself through its cards. Placed by `x` alone (an
        // anchor here would silently win and pin the page to the rail).
        width: parent.width - rail.width - (pageArea.full ? 0 : 2 * Tokens.s6)
        x: rail.width + (pageArea.full
            ? 0
            : Math.max(Tokens.s6, Math.round((parent.width - rail.width - width) / 2)))

        // Two loaders crossfade the page: the incoming page loads async into the
        // hidden loader, then fades in as the visible one fades out, so the
        // content never blanks to bare paper mid-swap (that blank was the
        // "flicker on section change"). The new page always loads into the
        // non-front loader, so the visible page is never disturbed even on rapid
        // switches, and a stale load from a superseded switch never reveals.
        Item {
            id: pageHost
            anchors.fill: parent
            readonly property string src: hub.pageFile(hub.section)
            property Item front: lb
            onSrcChanged: pageHost.swap()
            Component.onCompleted: pageHost.swap()
            function swap() {
                var incoming = pageHost.front === la ? lb : la;
                if (incoming.source == pageHost.src && incoming.status === Loader.Ready)
                    pageHost.reveal(incoming);
                else
                    incoming.source = pageHost.src;
            }
            function reveal(l) {
                if (l.source != pageHost.src)
                    return;
                pageHost.front = l;
                la.opacity = la === l ? 1 : 0; la.z = la === l ? 1 : 0;
                lb.opacity = lb === l ? 1 : 0; lb.z = lb === l ? 1 : 0;
            }
            Loader {
                id: la
                anchors.fill: parent
                asynchronous: true
                opacity: 1
                // hidden once fully faded, so the parked page stops taking hover
                // (a stale tooltip was leaking through the overlay layer).
                visible: opacity > 0.01
                onLoaded: { if (item) item.hub = hub; pageHost.reveal(la); }
                Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
            }
            Loader {
                id: lb
                anchors.fill: parent
                asynchronous: true
                opacity: 0
                visible: opacity > 0.01
                onLoaded: { if (item) item.hub = hub; pageHost.reveal(lb); }
                Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
            }
        }

        // honest interim: a page whose content is not ported yet says so,
        // rather than showing settings it cannot persist.
        Column {
            visible: hub.pageFile(hub.section) === ""
            anchors.top: parent.top; anchors.left: parent.left
            spacing: Tokens.s2
            Row {
                spacing: Tokens.s2
                Rectangle { width: 16; height: 1; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
                Text { text: "力"; color: Tokens.ink; font.family: Tokens.jp; font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter }
                Text {
                    text: I18n.tr(hub.groupFor(hub.section)); color: Tokens.inkMuted
                    font.family: Tokens.ui; font.pixelSize: 9; font.weight: Font.Medium
                    font.letterSpacing: Tokens.trackMark; anchors.verticalCenter: parent.verticalCenter
                }
            }
            Text { text: I18n.tr(hub.nameFor(hub.section)); color: Tokens.ink; font.family: Tokens.display; font.pixelSize: Tokens.fTitle }
            Item { width: 1; height: Tokens.s4 }
            Text {
                text: I18n.tr("PORTING IN PROGRESS"); color: Tokens.inkDim; font.family: Tokens.ui
                font.pixelSize: 11; font.weight: Font.Medium; font.letterSpacing: 2
            }
            Text {
                width: 520
                text: I18n.tr("This page is being rebuilt into the monochrome instrument. Its settings and surfaces are wired page by page; the Shell page is the proven pattern.")
                color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 13; wrapMode: Text.WordWrap
            }
        }
    }


    // ── updates: a corner button, not a rail row ────────────────────────────
    // Updates left the rail for a button in the top-right corner: an English
    // UPDATES chip that opens the Updates page and wears a red dot
    // (`Tokens.alert`, a fixed attention red) when the channel sits behind
    // origin. It rides the top strip beside every page's head: same right inset
    // as the page's content (S6, so it lines up with the last card rather than
    // floating inside it) and a box tall enough to read as a control next to the
    // head instead of a scrap of chrome above it. The `Updates` singleton
    // self-checks on load and on cadence, so the dot is live.
    Item {
        id: updatesBtn
        anchors { top: parent.top; right: parent.right }
        anchors.topMargin: Tokens.s4; anchors.rightMargin: Tokens.s6
        z: 60
        width: ubLabel.implicitWidth + Tokens.s4 * 2
        height: 30
        readonly property bool here: hub.section === "updates"
        Rectangle {
            anchors.fill: parent
            radius: Tokens.radius
            color: updatesBtn.here ? Tokens.bone : (ubh.hovered ? Tokens.paperLift : Tokens.paper)
            border.width: Tokens.border
            border.color: (updatesBtn.here || ubh.hovered) ? Tokens.lineStrong : Tokens.line
            antialiasing: false
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
            Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
        }
        Text {
            id: ubLabel
            anchors.centerIn: parent
            text: I18n.tr("UPDATES")
            color: updatesBtn.here ? Tokens.inkOnBone : Tokens.inkDim
            font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
            font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
        }
        // the one red accent: an update is waiting on the channel.
        Rectangle {
            visible: Updates.available
            width: 8; height: 8; radius: 4
            color: Tokens.alert
            border.width: 1; border.color: Tokens.paper
            antialiasing: true
            anchors { right: parent.right; top: parent.top; rightMargin: -2; topMargin: -2 }
        }
        HoverHandler { id: ubh; cursorShape: Qt.PointingHandCursor }
        TapHandler { onTapped: hub.section = "updates" }
    }

    // ── settings files: a masthead chip beside UPDATES ───────────────────────
    // Opens a popover naming the files that back the current page: the user-owned
    // layer (kept on update, edits win) and the shipped base it overrides.
    Item {
        id: filesBtn
        anchors { top: parent.top; right: updatesBtn.left }
        anchors.topMargin: Tokens.s4; anchors.rightMargin: Tokens.s3
        z: 60
        visible: hub.settingsFiles().length > 0
        width: fbLabel.implicitWidth + Tokens.s4 * 2
        height: 30
        Rectangle {
            anchors.fill: parent
            radius: Tokens.radius
            color: filesPop.open ? Tokens.bone : (fbh.hovered ? Tokens.paperLift : Tokens.paper)
            border.width: Tokens.border
            border.color: (filesPop.open || fbh.hovered) ? Tokens.lineStrong : Tokens.line
            antialiasing: false
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
            Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
        }
        Text {
            id: fbLabel
            anchors.centerIn: parent
            text: I18n.tr("FILES")
            color: filesPop.open ? Tokens.inkOnBone : Tokens.inkDim
            font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
            font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
        }
        HoverHandler { id: fbh; cursorShape: Qt.PointingHandCursor }
        TapHandler { onTapped: filesPop.open = !filesPop.open }
    }

    // ── action bar ─────────────────────────────────────────────────────────
    ActionBar {
        id: bar
        visible: !pageArea.full
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        dirty: hub.dirty
        onSaved: hub.save()
        onReverted: hub.revert()
        onReset: hub.resetDefaults()
        onDiffRequested: diffPop.toggle()
    }

    // ── catalogue overlay (font pick) ──────────────────────────────────────
    Item {
        id: picker
        anchors.fill: parent
        visible: pickState.row !== null
        z: 900

        QtObject { id: pickState; property var row: null }

        function openFor(r) { pickState.row = r; pick.open(); }
        function close() { pickState.row = null; }

        Rectangle {
            anchors.fill: parent
            color: "black"
            opacity: 0.55
            TapHandler { onTapped: picker.close() }
        }
        Picker {
            id: pick
            anchors.centerIn: parent
            title: pickState.row ? I18n.tr(pickState.row.label) : ""
            // a `set` row reads its list from the singleton that owns the
            // table (languages, locales) instead of the schema's own opts.
            options: pickState.row
                ? (pickState.row.set === "languages" ? I18n.pickerOptions
                    : pickState.row.set === "locales" ? I18n.localeOptions
                    : (pickState.row.opts || []))
                : []
            labels: pickState.row && pickState.row.set === "languages" ? I18n.pickerLabels : ({})
            current: pickState.row ? String(hub.val(pickState.row.key)) : ""
            onChose: (k) => { if (pickState.row) hub.edit(pickState.row.key, k); picker.close(); }
            onDismissed: picker.close()
        }
    }

    // ── search: results overlay + jump-to-setting ────────────────────────────
    // The rail search opens a results card under the field (not a cramped rail
    // list). Picking a result navigates to its page and, for a real setting,
    // scrolls it into view and flashes it -- the jump-to-setting the old search
    // never did. Enter takes the top hit; Escape (the field clears) dismisses.
    property string pendingFocusKey: ""
    function activateSearch(i) {
        var r = hub.searchResults[i];
        if (!r) return;
        // a filtered page never enters searchResults, but guard the jump anyway so
        // the search path can never reach one even if the index changes.
        if (!hub.sectionAvailable(r.section)) return;
        hub.section = r.section;
        hub.pendingFocusKey = (r.isPage || !r.key) ? "" : r.key;
        hub.query = "";
        searchField.clear();
        if (hub.pendingFocusKey !== "") { focusTimer.tries = 0; focusTimer.restart(); }
    }
    // bold the matched run in a result label (StyledText).
    function hlLabel(text) {
        function esc(s) { return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;"); }
        var q = hub.query.trim();
        if (!q) return esc(text);
        var lc = String(text).toLowerCase(), i = lc.indexOf(q.toLowerCase());
        if (i < 0) return esc(text);
        return esc(text.substring(0, i)) + "<u>" + esc(text.substring(i, i + q.length)) + "</u>" + esc(text.substring(i + q.length));
    }
    Timer {
        id: focusTimer
        interval: 90; repeat: false
        property int tries: 0
        onTriggered: {
            var it = pageHost.front ? pageHost.front.item : null;
            if (it && typeof it.focusKey === "function") { it.focusKey(hub.pendingFocusKey); hub.pendingFocusKey = ""; focusTimer.tries = 0; return; }
            if (focusTimer.tries < 15) { focusTimer.tries++; focusTimer.restart(); } else focusTimer.tries = 0;
        }
    }

    Item {
        id: searchPop
        z: 850
        visible: hub.query !== "" && hub.searchResults.length > 0
        x: rail.x + railHead.x + searchField.x
        y: rail.y + railHead.y + searchField.y + searchField.height + Tokens.s2
        width: Math.min(480, hub.width - x - Tokens.s5)
        height: Math.min(440, list.contentHeight + head.height + Tokens.s3 * 2 + Tokens.s1)

        Rectangle {
            anchors.fill: parent
            color: Tokens.paperLift
            radius: Tokens.radius
            border.width: Tokens.border
            border.color: Tokens.lineStrong
        }
        // sink so a click on the card's chrome never falls through to the rail.
        MouseArea { anchors.fill: parent; hoverEnabled: true }
        Row {
            id: head
            anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s3 }
            height: 14
            Text {
                text: "// RESULTS_"; color: Tokens.inkMuted
                font.family: Tokens.mono; font.pixelSize: 9; font.letterSpacing: 1.2
                anchors.verticalCenter: parent.verticalCenter
            }
            Item { width: Math.max(0, parent.width - 170); height: 1 }
            Text {
                text: hub.searchResults.length + (hub.searchResults.length === 1 ? I18n.tr(" HIT") : I18n.tr(" HITS"))
                color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 9
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        ListView {
            id: list
            anchors { left: parent.left; right: parent.right; top: head.bottom; bottom: parent.bottom }
            anchors.margins: Tokens.s2; anchors.topMargin: Tokens.s1
            clip: true
            model: hub.searchResults
            spacing: 1
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }
            delegate: Rectangle {
                id: rr
                required property var modelData
                required property int index
                width: ListView.view.width
                height: 46
                radius: Tokens.radius
                color: rrh.containsMouse ? Tokens.tint10 : "transparent"
                Behavior on color { ColorAnimation { duration: Tokens.snap } }
                Column {
                    anchors { left: parent.left; right: kseal.left; verticalCenter: parent.verticalCenter }
                    anchors.leftMargin: Tokens.s3; anchors.rightMargin: Tokens.s2
                    spacing: 1
                    Text {
                        width: parent.width
                        text: hub.hlLabel(I18n.tr(rr.modelData.label))
                        textFormat: Text.StyledText
                        color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 13
                        elide: Text.ElideRight
                    }
                    Text {
                        width: parent.width
                        text: rr.modelData.isPage ? I18n.tr("Page") : (rr.modelData.sectionName + (rr.modelData.group ? "  \u203a  " + rr.modelData.group : ""))
                        color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 9
                        elide: Text.ElideRight
                    }
                }
                Text {
                    id: kseal
                    anchors { right: parent.right; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                    text: hub.jpName[rr.modelData.section] || ""
                    color: Tokens.inkFaint; font.family: Tokens.jp; font.pixelSize: 12
                }
                MouseArea {
                    id: rrh
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: hub.activateSearch(rr.index)
                }
            }
        }
    }

    // ── the pending diff, on demand ──────────────────────────────────────────
    // The write ledger left the rail for a popover the action bar opens, so the
    // page keeps the full width and the diff is one click away when it matters.
    Item {
        id: diffPop
        anchors.fill: parent
        z: 870
        property bool open: false
        visible: diffPop.open && hub.diff.length > 0
        function toggle() { diffPop.open = !diffPop.open }

        Rectangle {
            anchors.fill: parent
            color: "transparent"
            TapHandler { onTapped: diffPop.open = false }
        }
        Rectangle {
            id: diffCard
            width: 480
            anchors { left: parent.left; leftMargin: Tokens.s5; bottom: parent.bottom; bottomMargin: (bar.visible ? bar.height : 0) + Tokens.s2 }
            height: Math.min(440, dcol.height + dhead.height + Tokens.s3 * 3)
            color: Tokens.paperLift
            radius: Tokens.radius
            border.width: Tokens.border
            border.color: Tokens.lineStrong
            TapHandler {}
            Row {
                id: dhead
                anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s3 }
                height: 14
                Text { text: "// PENDING WRITE_"; color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: 9; font.letterSpacing: 1.2; anchors.verticalCenter: parent.verticalCenter }
                Item { width: Math.max(0, parent.width - 200); height: 1 }
                Text { text: I18n.tr("DIFF"); color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 9; anchors.verticalCenter: parent.verticalCenter }
            }
            Flickable {
                id: dflick
                anchors { left: parent.left; right: parent.right; top: dhead.bottom; bottom: parent.bottom }
                anchors.margins: Tokens.s3; anchors.topMargin: Tokens.s2
                contentHeight: dcol.height
                clip: true
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                WheelScroll { }
                Column {
                    id: dcol
                    width: dflick.width - 12
                    spacing: Tokens.s3
                    Repeater {
                        model: hub.diff
                        Column {
                            id: fg
                            required property var modelData
                            width: dcol.width
                            spacing: Tokens.s1
                            Row {
                                spacing: Tokens.s2
                                Rectangle { width: 3; height: 3; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
                                Text { text: fg.modelData.file; color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 10; anchors.verticalCenter: parent.verticalCenter }
                                Text { text: "\u00b7 " + fg.modelData.changes.length; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 10; anchors.verticalCenter: parent.verticalCenter }
                            }
                            Repeater {
                                model: fg.modelData.changes
                                Column {
                                    required property var modelData
                                    width: fg.width
                                    topPadding: 2
                                    Text { text: modelData.key + ":"; color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 12 }
                                    Row {
                                        spacing: Tokens.s2
                                        Text { text: modelData.was; color: Tokens.inkFaint; font.strikeout: true; font.family: Tokens.mono; font.pixelSize: 12 }
                                        Text { text: "\u2192"; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 12 }
                                        Text { text: modelData.now; color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 12 }
                                    }
                                    Rectangle { width: parent.width; height: 1; color: Tokens.lineSoft }
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    // ── settings files popover ───────────────────────────────────────────────
    Item {
        id: filesPop
        anchors.fill: parent
        z: 880
        property bool open: false
        visible: filesPop.open && hub.settingsFiles().length > 0
        Rectangle {
            anchors.fill: parent
            color: "transparent"
            TapHandler { onTapped: filesPop.open = false }
        }
        Rectangle {
            width: Math.min(460, filesPop.width - Tokens.s4 * 2)
            anchors { right: parent.right; rightMargin: Tokens.s4; top: parent.top; topMargin: Tokens.s2 + 24 + Tokens.s2 }
            height: Math.min(520, fcol.height + fhead.height + Tokens.s3 * 3)
            color: Tokens.paperLift
            radius: Tokens.radius
            border.width: Tokens.border
            border.color: Tokens.lineStrong
            TapHandler {}
            Row {
                id: fhead
                anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s3 }
                height: 14
                Text { text: I18n.tr("SETTINGS FILES"); color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: 9; font.letterSpacing: 1.2; anchors.verticalCenter: parent.verticalCenter }
                Item { width: Math.max(0, parent.width - 240); height: 1 }
                Text { text: I18n.tr(hub.nameFor(hub.section)); color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 9; anchors.verticalCenter: parent.verticalCenter }
            }
            Flickable {
                anchors { left: parent.left; right: parent.right; top: fhead.bottom; bottom: parent.bottom }
                anchors.margins: Tokens.s3; anchors.topMargin: Tokens.s2
                contentHeight: fcol.height
                clip: true
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                WheelScroll { }
                Column {
                    id: fcol
                    width: parent.width - 12
                    spacing: Tokens.s2
                    Text {
                        width: fcol.width
                        text: I18n.tr("Each page's settings live in one full file below: the complete config you edit in place (the GUI writes the very same file). The shipped base underneath refreshes on update; your file wins. Run `ryoku reset <path>` to drop an override.")
                        color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11; wrapMode: Text.WordWrap
                    }
                    Repeater {
                        model: hub.settingsFiles()
                        Rectangle {
                            required property var modelData
                            width: fcol.width
                            height: fentry.height + Tokens.s3 * 2
                            color: rowH.hovered && !modelData.noOpen ? Tokens.paper : "transparent"
                            radius: Tokens.radius
                            border.width: Tokens.border
                            border.color: Tokens.line
                            HoverHandler { id: rowH; cursorShape: modelData.noOpen ? Qt.ArrowCursor : Qt.PointingHandCursor }
                            TapHandler { enabled: !modelData.noOpen; onTapped: { hub.openSettingsFile(modelData); filesPop.open = false; } }
                            Column {
                                id: fentry
                                anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter; margins: Tokens.s3 }
                                spacing: 3
                                Row {
                                    spacing: Tokens.s2
                                    Rectangle {
                                        width: 6; height: 6; radius: 3; anchors.verticalCenter: parent.verticalCenter
                                        color: modelData.role === "yours" ? Tokens.ink : Tokens.inkFaint
                                    }
                                    Text { text: I18n.tr(modelData.label); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 13; font.weight: Font.Medium; anchors.verticalCenter: parent.verticalCenter }
                                    Text { visible: modelData.role === "yours"; text: I18n.tr("EDIT HERE"); color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 8; font.letterSpacing: 1; anchors.verticalCenter: parent.verticalCenter }
                                    Text { visible: modelData.role === "advanced"; text: I18n.tr("ADVANCED"); color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 8; font.letterSpacing: 1; anchors.verticalCenter: parent.verticalCenter }
                                    Text { visible: modelData.role === "base"; text: I18n.tr("SHIPPED"); color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 8; font.letterSpacing: 1; anchors.verticalCenter: parent.verticalCenter }
                                }
                                Text { visible: !!modelData.path; text: hub.tildePath(modelData.path || ""); color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 11; width: fentry.width; elide: Text.ElideMiddle }
                                Text { text: modelData.note; color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11; width: fentry.width; wrapMode: Text.WordWrap }
                            }
                        }
                    }
                }
            }
        }
    }

}

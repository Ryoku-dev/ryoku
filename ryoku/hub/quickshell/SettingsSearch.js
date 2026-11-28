.pragma library

// Per-page settings search: the fuzzy, synonym-expanded, typo-tolerant ranking
// the Hub's global rail search uses (Hub.qml `synonyms`, `wordScore`,
// `searchScore`), applied to the rows a settings page draws. It replaces the
// plain indexOf the sheet used, which only matched a phrase typed exactly as
// the copy had it:
//
//  - synonyms, so "transparency", "glass" or "darkmode" reach the row that
//    words never spelled that way
//  - typo tolerance, so "trasparency" or "keybnd" still land
//  - diacritic folding, so "energia" finds a row that reads "Energía"
//  - a substring fallback, so a script the ASCII tokeniser cannot read
//    (Japanese, Thai, Arabic, Hindi) keeps the literal match indexOf gave
//  - ranking, so the row the words name comes first instead of schema order
//
// The rows arrive untranslated: the sheet translates a row at draw time
// (I18n.tr(r.label)), so the text searched here is the English source. Every
// addition therefore widens what the old matcher could find; nothing it found
// is lost.

// lowercases and folds the Latin diacritics Ryoku's catalogs use, so
// "Energía" and "energia" are one word to the ASCII tokeniser below and a
// Spanish, Portuguese, French, German or Polish query still meets its row.
// Unicode normalization is used when the engine carries it and backed by an
// explicit table otherwise, because the QML JS engine is not guaranteed to
// implement String.prototype.normalize. Scripts this table does not cover
// (Arabic harakat, Thai, CJK) keep their marks: the substring fallback in
// searchRows still matches them, which is exactly the old behaviour.
var LATIN_FLAT = {
    "\u00e0": "a", "\u00e1": "a", "\u00e2": "a", "\u00e3": "a", "\u00e4": "a", "\u00e5": "a",
    "\u00e7": "c", "\u0107": "c", "\u010d": "c", "\u0105": "a", "\u0103": "a", "\u00e8": "e",
    "\u00e9": "e", "\u00ea": "e", "\u00eb": "e", "\u0113": "e", "\u0119": "e", "\u00ec": "i",
    "\u00ed": "i", "\u00ee": "i", "\u00ef": "i", "\u0129": "i", "\u012b": "i", "\u0131": "i",
    "\u0142": "l", "\u00f1": "n", "\u0144": "n", "\u0148": "n", "\u00f2": "o", "\u00f3": "o",
    "\u00f4": "o", "\u00f5": "o", "\u00f6": "o", "\u0151": "o", "\u0153": "o", "\u00f9": "u",
    "\u00fa": "u", "\u00fb": "u", "\u00fc": "u", "\u016f": "u", "\u0171": "u", "\u00fd": "y",
    "\u00ff": "y", "\u0159": "r", "\u0155": "r", "\u015b": "s", "\u0161": "s", "\u017e": "z",
    "\u017a": "z", "\u017c": "z", "\u00df": "ss", "\u013e": "l", "\u015f": "s", "\u011f": "g",
    "\u0123": "g", "\u0163": "t", "\u00fe": "th"
};
var COMBINING = /[\u0300-\u036f]/g;
function fold(s) {
    if (s === undefined || s === null)
        return "";
    var t = ("" + s).toLowerCase();
    try {
        t = t.normalize("NFD").replace(COMBINING, "");
    } catch (e) {
        // no normalization: the table below still flattens the letters it knows.
    }
    // always applied, because NFD does not decompose the standalone letters
    // (l with stroke, d with stroke, thorn), which several catalogs use.
    var out = "", c;
    for (var i = 0; i < t.length; i++) {
        c = t.charAt(i);
        out += (LATIN_FLAT[c] !== undefined) ? LATIN_FLAT[c] : c;
    }
    return out;
}

// the synonym map, kept in step with Hub.qml's: a typed word expands to itself
// plus these, and any one of them satisfying a term counts as a hit.
var synonyms = {
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
    "titlebar": "titlebars decoration csd", "titlebars": "titlebar decoration csd", "plugin": "plugins addon", "plugins": "plugin addon",
    "monitor": "displays screen", "monitors": "displays screen", "resolution": "displays screen", "hidpi": "displays scale", "refresh": "displays",
    "mouse": "input pointer", "pointer": "input", "keyboard": "input", "touchpad": "input trackpad", "trackpad": "input touchpad",
    "notifications": "layerrules",
    "update": "updates upgrade", "upgrade": "updates", "blur": "windows glass", "rounding": "windows corners", "corners": "windows rounding",
    "animation": "animations motion", "motion": "animations", "gpu": "graphics", "graphics": "gpu",
    "voice": "dictation", "speech": "dictation voice", "microphone": "dictation", "mic": "dictation"
};

// the words a term may be answered by: the typed word, its synonyms, and --
// when the typed word is a near-miss of a known term -- that term's synonyms
// too, so "trasparency" reaches the row "transparency" names.
var TERMS = {};
function termGroup(w) {
    var g = [w], key;
    if (TERMS[w] !== undefined) return TERMS[w];
    key = synonyms[w];
    if (key === undefined && w.length >= 4) {
        for (var k in synonyms) {
            if (editDistance(w, k, w.length >= 7 ? 2 : 1) >= 0) { key = synonyms[k]; break; }
        }
    }
    if (key !== undefined) {
        var ps = ("" + key).split(/\s+/);
        for (var j = 0; j < ps.length; j++) if (ps[j] && g.indexOf(ps[j]) < 0) g.push(ps[j]);
    }
    TERMS[w] = g;
    return g;
}

// a query word expands to itself plus its synonyms, so a term is satisfied by
// any one of them and never dilutes multi-word coverage.
function expandQuery(q) {
    var words = q.split(/\s+/), groups = [];
    for (var i = 0; i < words.length; i++) {
        var w = words[i];
        if (!w) continue;
        groups.push(termGroup(w));
    }
    return groups;
}

// bounded Levenshtein: the distance if <= max, else -1, with an early
// length-diff reject and a per-row floor so it stays cheap per keystroke.
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

// one word against a row's tokens: exact word start > inside word >
// subsequence, and (when `loose` is on) a near-miss, weaker than a clean hit.
// The tokens are split once per row and cached, not once per call: a search
// scores a word against every token of every row, and re-splitting the same
// 200-character haystack thirteen thousand times per keystroke cost more than
// the matching itself.
//
// The subsequence and the typo steps are separable because a page must not
// offer them before the exact reading has been tried: "blue" is one edit from
// "blur" and also fits inside "blurEnabled" as a scatter, so a page that
// answered either in the first pass walked a user looking for a colour into
// the blur settings. Hub.qml's copy has no such switch because the rail caps
// and ranks its results; the sheet shows everything it is given.
function wordScore(w, tws, loose) {
    var best = 0;
    for (var i = 0; i < tws.length; i++) {
        var tw = tws[i];
        if (tw === "") continue;
        var idx = tw.indexOf(w);
        if (idx === 0) { if (best < 1200) best = 1200; continue; }
        if (idx > 0) { var ss = 1000 - Math.min(idx, 50); if (ss > best) best = ss; continue; }
        if (loose && w.length <= tw.length) {
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
    // the typo scan is the expensive step, and it only ever adds a hit the
    // scatter above has not already made: skipped once a word has matched.
    if (loose && best === 0 && w.length >= 4) {
        for (var t = 0; t < tws.length; t++) {
            var tw2 = tws[t];
            if (tw2.length < 3) continue;
            var d = editDistance(w, tw2, w.length >= 7 ? 2 : 1);
            if (d >= 0) { var es = 640 - d * 140; if (es > best) best = es; }
        }
    }
    return best;
}

// multi-word scoring over the expanded groups: each term scores its best
// synonym (a synonym counts a touch less than the typed word), summed and
// scaled by how much of the query landed, so one out-of-vocabulary word never
// blanks the list and a synonym or a typo still finds the row. `loose` is the
// pass's setting: see wordScore.
function searchScore(groups, tws, loose) {
    var total = 0, matched = 0, n = 0;
    for (var gi = 0; gi < groups.length; gi++) {
        n++;
        var g = groups[gi], best = 0;
        for (var wi = 0; wi < g.length; wi++) {
            var s = wordScore(g[wi], tws, loose);
            if (wi > 0) s *= 0.85;
            if (s > best) best = s;
        }
        if (best > 0) { total += best; matched++; }
    }
    if (matched === 0 || n === 0) return 0;
    return total * (matched / n);
}

// The row's searchable words, folded once and remembered per row object.
//
// The sheet re-runs a search on every keystroke over the same stable row
// objects, and folding a label, a description, a dotted key, the keywords and
// the option values of 200+ rows is the whole cost of a search -- measured at
// eight times the cost of the matching itself on the largest page. A row that
// is rebuilt (a page that injects a live option list) simply misses once.
var FOLDED = (typeof WeakMap !== "undefined") ? new WeakMap() : null;
function buildHaystack(r) {
    var own = fold(r.label) + " " + fold(r.desc) + " " + fold(r.key);
    var optkw = (r.keywords || "") + " ";
    if (r.opts) {
        for (var i = 0; i < r.opts.length; i++) {
            var o = r.opts[i];
            if (typeof o === "string" && /^[a-z0-9][a-z0-9 ._/-]*$/.test(o))
                optkw += o + " ";
        }
    }
    var hay = own + " " + fold(optkw);
    return { hay: hay, label: fold(r.label), toks: hay.split(/[^a-z0-9]+/) };
}
function cachedRow(r) {
    if (!FOLDED) return buildHaystack(r);
    var got = FOLDED.get(r);
    if (got === undefined) {
        got = buildHaystack(r);
        FOLDED.set(r, got);
    }
    return got;
}
function rowLabel(r) {
    return cachedRow(r).label;
}

// the words that name the page, not the row: the group and the tab. They are
// matched only as the whole typed phrase, because letting a single common word
// ("window", "blur", "layout") satisfy them is how a one-word query comes back
// showing half the page. A typed phrase still reaches them ("layer rules").
function groupHaystack(r) {
    // the group and the tab change only when the schema does; folding them per
    // keystroke is the same cost the row cache removes above
    var got = cachedRow(r);
    if (got.gtoks === undefined)
        got.gtoks = (fold(r.group) + " " + fold(r.tab)).split(/[^a-z0-9]+/);
    return got.gtoks;
}

// how many of the query's terms a haystack satisfies, by word, prefix or typo.
function termsHit(groups, tws, loose) {
    var n = 0;
    for (var gi = 0; gi < groups.length; gi++) {
        for (var wi = 0; wi < groups[gi].length; wi++) {
            if (wordScore(groups[gi][wi], tws, loose) > 0) { n++; break; }
        }
    }
    return n;
}

// Search the rows and return the matches, best first.
//
// rows: the rows the page draws, untranslated (the sheet translates at draw
// time, so the text here is the English source).
// query: the typed text; an empty or whitespace-only query returns every row,
// because the caller shows its unfiltered view for that and never asks.
//
// Three passes, each tried only when the one before it found nothing, because
// a page has no room to show half of itself and the loosest reading is the
// noisiest:
//
//  1. the typed words themselves, diacritics folded, matched exactly. A row
//     must satisfy a term to be a hit at all, and the words of a multi-word
//     query are preferred complete: rows answering every word come back, and
//     only if none do the partial ones follow.
//  2. the same words with their typos and synonyms -- so "trasparency" and
//     "darkmode" still land, while "blue" never walks a colour-hunter into
//     the blur settings just because it is one edit from "blur".
//  3. the literal substring and the row's group or tab as a whole phrase, for
//     a script the tokeniser cannot read (Japanese, Thai, Arabic, Hindi) and
//     for a query that names the section rather than a setting.
function searchRows(rows, query) {
    var q = fold(query).trim();
    if (q === "") return rows;
    var groups = expandQuery(q);
    var terms = groups.length;
    // every pass scores the same rows against the same haystack, so fold each
    // row's words once rather than three times over
    var hays = [], i;
    for (i = 0; i < rows.length; i++) hays.push(cachedRow(rows[i]));

    function scoreRow(idx, g, loose) {
        var r = rows[idx], toks = hays[idx].toks, hay = hays[idx].hay;
        var s = 0, hit = false, hitAll = true;
        for (var gi = 0; gi < g.length; gi++) {
            var best = 0;
            for (var wi = 0; wi < g[gi].length; wi++) {
                var t;
                // the exact pass can only match a word the row contains as one
                // run of characters, so one native substring search settles the
                // row without walking its tokens -- the common case while the
                // query is still being typed, and most rows do not contain it
                if (!loose && hay.indexOf(g[gi][wi]) < 0) {
                    t = 0;
                } else {
                    t = wordScore(g[gi][wi], toks, loose);
                }
                if (wi > 0) t *= 0.85;
                if (t > best) best = t;
            }
            if (best > 0) { hit = true; s += best; } else { hitAll = false; }
        }
        if (!hit) return null;
        // a hit on the label itself outranks one buried in the description
        s += 2 * searchScore(g, fold(r.label).split(/[^a-z0-9]+/), loose);
        return { row: r, score: s, all: hitAll };
    }
    // the literal substring the sheet always had, and the row's group or tab
    // read as one phrase. Both run below every token hit, so they only fill in
    // what the ranker missed; one character is enough, because a single kanji
    // a user types mid-composition is a query too.
    function fallbackScore(idx) {
        var r = rows[idx], hay = hays[idx].hay, s = hay.indexOf(q) >= 0 ? 260 : 0;
        if (s <= 0 && termsHit(groups, groupHaystack(r), true) === terms) s = 240;
        return s > 0 ? s + 2 * searchScore(groups, rowLabel(r), true) : 0;
    }

    for (var pass = 0; pass < 3; pass++) {
        var g = (pass === 1) ? groups : groups.map(function (one) { return [one[0]]; });
        var scored = [], partial = [];
        if (pass < 2) {
            var loose = (pass === 1);
            for (i = 0; i < rows.length; i++) {
                var m = scoreRow(i, g, loose);
                if (!m) continue;
                if (m.all || terms === 1) scored.push(m); else partial.push(m);
            }
            if (!scored.length) scored = partial;
        } else {
            for (i = 0; i < rows.length; i++) {
                var s2 = fallbackScore(i);
                if (s2 > 0) scored.push({ row: rows[i], score: s2 });
            }
        }
        if (scored.length) {
            scored.sort(function (a, b) { return b.score - a.score; });
            var out = [];
            for (var k = 0; k < scored.length; k++) out.push(scored[k].row);
            return out;
        }
    }
    return [];
}
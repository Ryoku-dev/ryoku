.pragma library

// The one search engine: the fuzzy, synonym-expanded, typo-tolerant ranking
// the Hub's rail and every settings page share. It replaces the plain indexOf
// they used, which only matched a phrase typed exactly as the copy had it:
//
//  - synonyms, so "transparency", "glass" or "darkmode" reach the row that
//    words never spelled that way
//  - typo tolerance, so "trasparency" or "keybnd" still land
//  - diacritic folding, so "energia" finds a row that reads "Energía"
//  - both the language a row is authored in and the language it is drawn in,
//    so a query in either reaches it
//  - a substring fallback, so a script the ASCII tokeniser cannot read
//    (Japanese, Thai, Arabic, Hindi) keeps the literal match indexOf gave
//  - ranking, so the row the words name comes first instead of schema order
//  - task language (intentKeys), so a phrase that describes a goal reaches the
//    row that owns it even when no haystack carries its words

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
// two scratch rows, reused across every call: the typo scan runs thousands of
// times per keystroke and a fresh array pair each time cost more than the match
var DIST_A = [], DIST_B = [];
function editDistance(a, b, max) {
    var la = a.length, lb = b.length, j;
    if (Math.abs(la - lb) > max) return -1;
    var prev = DIST_A, cur = DIST_B;
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
// the blur settings. `searchRows` runs the loose reading in a second pass for
// that reason, and `wordScore`'s `loose` flag is what separates the two.
function wordScore(w, tws, loose) {
    var best = 0;
    for (var i = 0; i < tws.length; i++) {
        var tw = tws[i];
        if (tw === "") continue;
        var idx = tw.indexOf(w);
        // a two-letter word is a prefix of half the vocabulary, not a match:
        // "al" is not the "al" of "align". It still matches as a whole token,
        // so a real two-letter word (4k, tv, go) keeps working.
        if (idx === 0) { if (best < 1200 && (w.length > 2 || tw === w)) best = 1200; continue; }
        // the same for a word carried inside another: "al" is not the "al" of
        // "label", and a filler word riding in a query must not bring 80 rows
        if (idx > 0) {
            var ss = (w.length > 2) ? 1000 - Math.min(idx, 50) : 0;
            if (ss > best) best = ss;
            continue;
        }
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
function searchScoreParts(groups, tws, loose) {
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
    return { score: (matched === 0 || n === 0) ? 0 : total * (matched / n), matched: matched, n: n };
}
function searchScore(groups, tws, loose) {
    return searchScoreParts(groups, tws, loose).score;
}

// The row's searchable words, folded once and remembered per row object.
//
// The sheet re-runs a search on every keystroke over the same stable row
// objects, and folding a label, a description, a dotted key, the keywords and
// the option values of 200+ rows is the whole cost of a search -- measured at
// eight times the cost of the matching itself on the largest page. A row that
// is rebuilt (a page that injects a live option list) simply misses once.
//
// A row is searched in the language it is authored in AND the language it is
// drawn in: the sheet hands over English source rows and translates them at
// draw time, while a bespoke page may hand over rows that are already
// translated. Keeping both in the haystack is what makes a query in either
// language reach the row.
var FOLDED = (typeof WeakMap !== "undefined") ? new WeakMap() : null;
function translated(tr, s) {
    return (typeof tr === "function") ? tr(s) : s;
}
function buildHaystack(r, tr) {
    var labelTr = translated(tr, r.label);
    var groupTr = translated(tr, r.group);
    var tabTr = translated(tr, r.tab);
    var src = fold(r.label) + " " + fold(r.desc) + " " + fold(r.key);
    var dst = fold(labelTr) + " " + fold(translated(tr, r.desc)) + " " + fold(translated(tr, r.eg || ""));
    var optkw = (r.keywords || "") + " " + (translated(tr, r.keywords || "") !== (r.keywords || "")
        ? translated(tr, r.keywords || "") + " " : "");
    if (r.opts) {
        for (var i = 0; i < r.opts.length; i++) {
            var o = r.opts[i];
            if (typeof o !== "string") continue;
            if (/^[a-z0-9][a-z0-9 ._/-]*$/.test(o)) optkw += o + " ";
            var to = translated(tr, o);
            if (to !== o) optkw += to + " ";
        }
    }
    var hay = src + " " + dst + " " + fold(optkw);
    var labelHay = fold(r.label) + (labelTr !== r.label ? " " + fold(labelTr) : "");
    var groupHay = fold(r.group) + (groupTr !== r.group ? " " + fold(groupTr) : "")
        + " " + fold(r.tab) + (tabTr !== r.tab ? " " + fold(tabTr) : "");
    return {
        hay: hay,
        label: labelHay,
        toks: hay.split(/[^a-z0-9]+/),
        labelToks: labelHay.split(/[^a-z0-9]+/),
        gtoks: groupHay.split(/[^a-z0-9]+/)
    };
}
function cachedRow(r, tr, lang) {
    if (!FOLDED) return buildHaystack(r, tr);
    var got = FOLDED.get(r);
    if (got === undefined || got.lang !== lang) {
        got = buildHaystack(r, tr);
        got.lang = lang;
        FOLDED.set(r, got);
    }
    return got;
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
// rows: the rows the page draws. A page may hand over English source rows (the
// sheet translates them at draw time) or rows whose text is already translated;
// both are searched, so a query in either language reaches the row.
// query: the typed text; an empty or whitespace-only query returns every row,
// because the caller shows its unfiltered view for that and never asks.
// tr: the translation lookup, when the caller has one (I18n.tr).
// lang: the active language code, so the per-row cache is rebuilt when it
// changes.
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
function searchRows(rows, query, tr, lang, intents, fillers, langs) {
    var q = fold(query).trim();
    if (q === "") return rows;
    var groups = expandQuery(q);
    var terms = groups.length;
    // task language, when the caller passes the table: the rows the phrase
    // means come first, so describing a goal lands on the row that owns it
    // instead of wherever its own words happened to rank. A bonus, not a class
    // of its own: an intent may reorder rows that already matched, never lift
    // one over a clean label hit (measured: a hard first/second split turned a
    // "shadow" query into "turn shadows on" ahead of "Shadows off")
    var want = intentKeys(query, intents, fillers, lang, langs);
    var rank = function (a, b) {
        var ai = (want && a.row && want.indexOf(a.row.key) >= 0) ? 600 : 0;
        var bi = (want && b.row && want.indexOf(b.row.key) >= 0) ? 600 : 0;
        return (b.score + bi) - (a.score + ai);
    };
    // every pass scores the same rows against the same haystack, so fold each
    // row's words once rather than three times over
    var hays = [], i;
    for (i = 0; i < rows.length; i++) hays.push(cachedRow(rows[i], tr, lang));

    function scoreRow(idx, g, loose) {
        var r = rows[idx], toks = hays[idx].toks, hay = hays[idx].hay;
        var s = 0, hit = false, hitAll = true, answered = 0;
        for (var gi = 0; gi < g.length; gi++) {
            var best = 0;
            for (var wi = 0; wi < g[gi].length; wi++) {
                var t;
                // the exact pass can only match a word the row contains as one
                // run of characters, so one native substring search settles the
                // row without walking its tokens -- the common case while the
                // query is still being typed, and most rows do not contain it
                // an exact word settles a row with one native search: in the
                // exact pass that is every word, and in the loose pass it is
                // every synonym -- a synonym is spelled out, only the typed word
                // may be a near-miss, and it is the group's first entry
                if ((!loose || wi > 0) && hay.indexOf(g[gi][wi]) < 0) {
                    t = 0;
                } else {
                    t = wordScore(g[gi][wi], toks, loose);
                }
                if (wi > 0) t *= 0.85;
                if (t > best) best = t;
            }
            if (best > 0) { hit = true; s += best; answered++; } else { hitAll = false; }
        }
        if (!hit) return null;
        // a row that answered one word of three is a quarter of an answer, not
        // a full one: without the scaling a partial match scored the same as a
        // complete one, and a two-word query put a row naming a single word
        // above the row naming both. The label bonus is scaled with it, so it
        // still outranks a match buried in the description.
        s *= answered / g.length;
        s += 2 * searchScore(g, hays[idx].labelToks, loose);
        return { row: r, score: s, all: hitAll };
    }
    // the literal substring the sheet always had, and the row's group or tab
    // read as one phrase. Both run below every token hit, so they only fill in
    // what the ranker missed; one character is enough, because a single kanji
    // a user types mid-composition is a query too.
    function fallbackScore(idx) {
        var r = rows[idx], hay = hays[idx].hay, s = hay.indexOf(q) >= 0 ? 260 : 0;
        if (s <= 0 && termsHit(groups, hays[idx].gtoks, true) === terms) s = 240;
        return s > 0 ? s + 2 * searchScore(groups, hays[idx].labelToks, true) : 0;
    }

    // seed the rows a task phrase names, so a query whose words no haystack
    // carries (a Spanish phrase against English rows) still reaches them: a
    // bonus can only reorder a candidate the pass already produced. The seeded
    // score sits above a weak partial match and below a clean label hit, so an
    // intent fills what the search missed and never outranks a row that
    // answered directly.
    function seedIntent(scored) {
        if (!want) return scored;
        for (var i = 0; i < rows.length; i++) {
            if (want.indexOf(rows[i].key) < 0) continue;
            var seen = false;
            for (var j = 0; j < scored.length; j++) if (scored[j].row === rows[i]) { seen = true; break; }
            if (!seen) scored.push({ row: rows[i], score: 1400, all: true });
            // a row the pass already reached on a partial match is lifted to the
            // seeded score, so the hand-written mapping outranks the coincidence
            else scored[j].score = Math.max(scored[j].score, 1400);
        }
        return scored;
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
            // nothing answered every word: the partial rows follow, but only the
            // ones that answered one strongly. A word scattered inside a token
            // scores single digits, and 59 such rows read as a page that matched
            // nothing rather than as help. A single-word query keeps every
            // match, because a scatter is all an abbreviation has.
            if (!scored.length) {
                for (i = 0; i < partial.length; i++)
                    if (terms === 1 || partial[i].score >= 100) scored.push(partial[i]);
            }
            // an intent fills what this pass could not reach, never hides what it
            // did: a clean label hit still outranks the seeded row
            seedIntent(scored);
        } else {
            for (i = 0; i < rows.length; i++) {
                var s2 = fallbackScore(i);
                if (s2 > 0) scored.push({ row: rows[i], score: s2 });
            }
            seedIntent(scored);
        }
        if (scored.length) {
            scored.sort(rank);
            var out = [];
            for (var k = 0; k < scored.length; k++) out.push(scored[k].row);
            return out;
        }
    }
    return [];
}
// Rank the objects a surface draws, without pretending they are settings.
//
// A list that owns its own drawing (an app picker, a plugin grid) filters with
// a plain substring over name and id, so a typo or a word in the wrong order
// finds nothing. This hands the engine the same row shape the settings pages
// give it and hands the caller its own objects back, ranked. `toRow` names the
// searchable fields; the empty query keeps the caller's order (already sorted
// for reading), because a search that arrives with nothing typed must not
// reshuffle a list nobody asked about.
function searchItems(items, query, toRow, tr, lang) {
    if (!items || !items.length) return items || [];
    var q = fold(query).trim();
    if (q === "") return items;
    var rows = [], byKey = {}, i;
    for (i = 0; i < items.length; i++) {
        var r = toRow(items[i]);
        // the row key is how the item is found again after ranking, so an item
        // without one falls back to its place in the list rather than being
        // dropped from the results silently
        r.key = r.key === undefined || r.key === null || r.key === "" ? "#" + i : "" + r.key;
        rows.push(r);
        byKey[r.key] = items[i];
    }
    var ranked = searchRows(rows, q, tr, lang);
    var out = [];
    for (i = 0; i < ranked.length; i++) {
        var it = byKey[ranked[i].key];
        if (it !== undefined) out.push(it);
    }
    return out;
}

// ── task language ─────────────────────────────────────────────────────────
// The keys a phrase means, so describing a goal ("cursor grande") reaches the
// row that owns it even when none of those words sit in its label. Every word
// of the query that is not filler must be a word the intent spells, which keeps
// a stray word from derailing a phrase, and a one-character query never
// matches: a single kanji mid-composition is a query too, and no intent speaks
// it.
//
// intents: [{ keys: [...], words: "..." }] from schema/SearchIntents.js.
// fillers: that table's words that carry no intent (el, muy, too).
// lang: the language the user is drawing ("en", "es", ...).
// curated: the languages the table is written in ("en es"). An intent is a
// hand-written promise about how a phrase reads, so outside those languages it
// must not fire: a loanword ("layout", "size") typed in a German or French Hub
// would otherwise get an English or Spanish task promoted over the row the
// user's own labels already ranked first.
//
// Returns null when nothing matched, so callers keep one code path.
function intentKeys(query, intents, fillers, lang, curated) {
    if (!intents || !intents.length) return null;
    if (curated && curated !== "" && ("" + curated).split(/\s+/).indexOf("" + lang) < 0)
        return null;
    var q = fold(query).trim();
    if (q === "") return null;
    var pad = fillers ? " " + fold(fillers) + " " : "";
    var words = q.split(/\s+/);
    var task = [];
    for (var i = 0; i < words.length; i++) {
        if (words[i].length < 2) return null;
        if (pad !== "" && pad.indexOf(" " + words[i] + " ") >= 0) continue;
        // word boundaries, not substrings: "se" is a filler, not the "se" of
        // "separacion", and a short word that leaks into another's middle would
        // make a phrase look spelled when nothing spells it
        var spelled = false;
        for (var n = 0; n < intents.length && !spelled; n++)
            if ((" " + fold(intents[n].words || "") + " ").indexOf(" " + words[i] + " ") >= 0) spelled = true;
        // a word no intent spells is a stray one: the phrase is not this task
        if (!spelled) return null;
        task.push(words[i]);
    }
    // a query of nothing but filler carries no task, and must not set every
    // intent off at once
    if (!task.length) return null;
    var out = null;
    for (var it = 0; it < intents.length; it++) {
        var have = " " + fold(intents[it].words || "") + " ";
        var ok = true;
        for (var w = 0; w < task.length && ok; w++)
            if (have.indexOf(" " + task[w] + " ") < 0) ok = false;
        if (!ok) continue;
        var keys = intents[it].keys || [];
        if (!out) out = [];
        for (var k = 0; k < keys.length; k++) if (out.indexOf(keys[k]) < 0) out.push(keys[k]);
    }
    return out;
}

// ── highlighting ─────────────────────────────────────────────────────────
// Underline the run a query names inside a label, in the label's own spelling.
// The match is made on folded text (so "energia" underlines "Energía") and the
// index is mapped back to the original string, so the markup never slices a
// character in half. Used by the Hub's result list.
function escapeHtml(s) {
    return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}
function foldMap(s) {
    var text = "", map = [], f, i, j;
    for (i = 0; i < s.length; i++) {
        f = fold(s.charAt(i));
        for (j = 0; j < f.length; j++) {
            text += f.charAt(j);
            map.push(i);
        }
    }
    return { text: text, map: map };
}
function textHasQuery(text, query) {
    var q = fold(query).trim();
    if (q === "") return false;
    var h = fold(text);
    if (h.indexOf(q) >= 0) return true;
    // word boundaries: a two-letter query word is inside half the vocabulary,
    // and the caller only wants to know whether the drawn label carries it
    var padded = " " + h + " ";
    var words = q.split(/\s+/);
    for (var i = 0; i < words.length; i++)
        if (words[i].length >= 2 && padded.indexOf(" " + words[i] + " ") >= 0) return true;
    return false;
}
function highlightHtml(text, query) {
    var src = "" + text;
    var q = fold(query).trim();
    if (q === "") return escapeHtml(src);
    var fm = foldMap(src);
    var ranges = [], at, from;
    // the whole typed phrase first, so a two-word query reads as one run
    at = fm.text.indexOf(q);
    if (at >= 0) {
        ranges.push([fm.map[at], fm.map[at + q.length - 1] + 1]);
    } else {
        var words = q.split(/\s+/);
        for (var w = 0; w < words.length; w++) {
            if (words[w].length < 2) continue;
            from = 0;
            while ((at = fm.text.indexOf(words[w], from)) >= 0) {
                ranges.push([fm.map[at], fm.map[at + words[w].length - 1] + 1]);
                from = at + words[w].length;
            }
        }
    }
    if (!ranges.length) return escapeHtml(src);
    ranges.sort(function (a, b) { return a[0] - b[0]; });
    var merged = [];
    for (var r = 0; r < ranges.length; r++) {
        var last = merged[merged.length - 1];
        if (last && ranges[r][0] <= last[1]) last[1] = Math.max(last[1], ranges[r][1]);
        else merged.push([ranges[r][0], ranges[r][1]]);
    }
    var out = "", pos = 0;
    for (var m = 0; m < merged.length; m++) {
        out += escapeHtml(src.slice(pos, merged[m][0]));
        out += "<u>" + escapeHtml(src.slice(merged[m][0], merged[m][1])) + "</u>";
        pos = merged[m][1];
    }
    return out + escapeHtml(src.slice(pos));
}

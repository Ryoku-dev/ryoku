// A control matches when every query term appears in its haystack.

// A keyword present in a haystack pulls in its synonyms.
var ALIASES = [
    ["fps", "frame rate frames per second"],
    ["battery", "laptop power wattage energy saver"],
    ["gpu", "graphics vram hardware acceleration"],
    ["cpu", "processor software rendering"],
    ["wallpaper engine", "we workshop"],
    ["monitor", "display output screen"],
    ["transition", "animation motion effect"],
    ["path", "folder directory location"],
    ["rgb", "lighting led openrgb aura qmk keyboard mouse palette"]
];

function normalize(value) {
    if (value === undefined || value === null)
        return "";
    var s = String(value);
    var out = "";
    var prevLower = false;
    for (var i = 0; i < s.length; i++) {
        var ch = s.charAt(i);
        var isUpper = ch >= "A" && ch <= "Z";
        var isAlnum = /[0-9A-Za-z]/.test(ch) || ch.charCodeAt(0) > 127;
        if (isUpper && prevLower)
            out += " ";
        if (isAlnum) {
            out += ch.toLowerCase();
            prevLower = !isUpper;
        } else {
            if (out.length > 0 && out.charAt(out.length - 1) !== " ")
                out += " ";
            prevLower = false;
        }
    }
    return out.replace(/\s+/g, " ").trim();
}

function _addAliases(haystack) {
    var h = haystack;
    for (var i = 0; i < ALIASES.length; i++) {
        if (h.indexOf(ALIASES[i][0]) >= 0)
            h += " " + ALIASES[i][1];
    }
    return h;
}

function search(query, tabs, limit) {
    var q = normalize(query);
    if (q.length === 0)
        return [];
    var terms = q.split(" ");
    var results = [];

    for (var t = 0; t < tabs.length; t++) {
        var tab = tabs[t];
        var tabLabel = normalize(tab.title);
        var sections = tab.sections || [];
        for (var s = 0; s < sections.length; s++) {
            var section = sections[s];
            var sectionTitle = normalize(section.title);
            var controls = section.controls || [];
            for (var c = 0; c < controls.length; c++) {
                var ctl = controls[c];
                if (ctl.kind === "segment")
                    continue;
                // Rows whose label is a live %N template name themselves for search.
                var shown = ctl.searchTitle || ctl.label || "";
                var title = normalize(shown);
                var terms2 = (ctl.search || []).join(" ");
                var haystack = title + " " + sectionTitle + " " + tabLabel + " "
                    + normalize(ctl.help) + " " + normalize(terms2) + " " + normalize(ctl.kind);
                haystack = _addAliases(haystack);

                var ok = true;
                for (var k = 0; k < terms.length; k++) {
                    if (haystack.indexOf(terms[k]) < 0) { ok = false; break; }
                }
                if (!ok)
                    continue;

                var score;
                if (title === q) score = 0;
                else if (title.indexOf(q) === 0) score = 1;
                else if (title.indexOf(q) >= 0) score = 2;
                else if (sectionTitle.indexOf(q) >= 0) score = 3;
                else if (tabLabel.indexOf(q) >= 0) score = 4;
                else score = 5;

                results.push({
                    tabIndex: t,
                    tabKey: tab.tabKey,
                    tabLabel: tab.title,
                    sectionIndex: s,
                    sectionTitle: section.title,
                    controlId: ctl.id,
                    title: shown.length > 0 ? shown : (ctl.id || ""),
                    score: score
                });
            }
        }
    }

    results.sort(function (a, b) {
        if (a.score !== b.score) return a.score - b.score;
        if (a.title !== b.title) return a.title < b.title ? -1 : 1;
        return a.tabLabel < b.tabLabel ? -1 : (a.tabLabel > b.tabLabel ? 1 : 0);
    });
    return results.slice(0, limit > 0 ? limit : 6);
}

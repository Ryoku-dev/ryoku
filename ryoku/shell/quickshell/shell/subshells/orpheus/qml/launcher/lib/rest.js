// The launcher's rest state (an empty query): pinned apps in their order, then
// the most used, then for a fresh user a spread across categories by priority,
// never a plain A to Z.

// Priority order. System and network land after tool with everything else.
var CATEGORY_ORDER = [
    ["browser", ["WebBrowser"]],
    ["terminal", ["TerminalEmulator"]],
    ["files", ["FileManager"]],
    ["media", ["AudioVideo", "Audio", "Video", "Player", "Music", "Recorder"]],
    ["office", ["Office", "WordProcessor", "Spreadsheet", "Presentation"]],
    ["dev", ["Development", "IDE", "TextEditor"]],
    ["graphics", ["Graphics", "Photography", "2DGraphics", "3DGraphics", "RasterGraphics", "VectorGraphics"]],
    ["chat", ["InstantMessaging", "Chat", "IRCClient", "VideoConference"]],
    ["game", ["Game"]],
    ["tool", ["Utility"]],
    ["system", ["Settings", "System", "Monitor"]],
    ["network", ["Network"]]
];
var PRIORITY_COUNT = 10;
var REST_LIMIT = 9;

function categoryList(categories) {
    if (Array.isArray(categories))
        return categories.map(String);
    if (categories && typeof categories.length === "number" && typeof categories !== "string") {
        var out = [];
        for (var i = 0; i < categories.length; i++)
            out.push(String(categories[i]));
        return out;
    }
    return String(categories || "").split(/[;,]/).filter(function (c) { return c.length > 0; });
}

function categoryIndex(categories) {
    var list = categoryList(categories);
    for (var i = 0; i < CATEGORY_ORDER.length; i++) {
        var names = CATEGORY_ORDER[i][1];
        for (var j = 0; j < names.length; j++)
            if (list.indexOf(names[j]) >= 0)
                return i;
    }
    return CATEGORY_ORDER.length;
}

// "browser" .. "network", or "app" when no known category applies.
function categoryKey(categories) {
    var index = categoryIndex(categories);
    return index < CATEGORY_ORDER.length ? CATEGORY_ORDER[index][0] : "app";
}

// 0 is the most wanted, every category past tool shares the last rank.
function categoryRank(categories) {
    return Math.min(categoryIndex(categories), PRIORITY_COUNT);
}

function appKey(row) {
    return String(row && (row.appId || row.id) || "");
}

function byTitle(a, b) {
    var x = String(a.title || "").toLowerCase();
    var y = String(b.title || "").toLowerCase();
    return x < y ? -1 : x > y ? 1 : 0;
}

function countOf(usage, id) {
    var count = usage ? usage[id] : 0;
    return typeof count === "number" && count > 0 ? count : 0;
}

// rows: app rows ({ appId|id, title }), categoriesOf(row) gives its desktop
// categories. Returns { rows, section } with section "pinned", "mostUsed" or "".
function restOrder(rows, pinned, usage, categoriesOf, limit) {
    var source = Array.isArray(rows) ? rows : [];
    var cap = limit > 0 ? limit : REST_LIMIT;
    var byId = {};
    for (var i = 0; i < source.length; i++) {
        var key = appKey(source[i]);
        if (key.length > 0 && !byId["$" + key])
            byId["$" + key] = source[i];
    }

    var taken = {};
    var out = [];
    function take(row) {
        var k = "$" + appKey(row);
        if (taken[k] || out.length >= cap)
            return false;
        taken[k] = true;
        out.push(row);
        return true;
    }

    var pinnedCount = 0;
    var pins = Array.isArray(pinned) ? pinned : [];
    for (var p = 0; p < pins.length; p++) {
        var pinnedRow = byId["$" + String(pins[p])];
        if (pinnedRow && take(pinnedRow))
            pinnedCount++;
    }

    var used = source.filter(function (row) {
        return countOf(usage, appKey(row)) > 0 && !taken["$" + appKey(row)];
    });
    used.sort(function (a, b) {
        var diff = countOf(usage, appKey(b)) - countOf(usage, appKey(a));
        return diff !== 0 ? diff : byTitle(a, b);
    });
    var usedCount = 0;
    for (var u = 0; u < used.length; u++)
        if (take(used[u]))
            usedCount++;

    if (out.length < cap) {
        var groups = [];
        for (var g = 0; g <= PRIORITY_COUNT; g++)
            groups.push([]);
        for (var r = 0; r < source.length; r++) {
            var row = source[r];
            if (taken["$" + appKey(row)])
                continue;
            groups[categoryRank(categoriesOf ? categoriesOf(row) : [])].push(row);
        }
        for (var s = 0; s < groups.length; s++)
            groups[s].sort(byTitle);
        // one app per category in priority order, then a second round, so a
        // fresh user sees a browser, a terminal, files and so on first
        var progressed = true;
        while (out.length < cap && progressed) {
            progressed = false;
            for (var c = 0; c < groups.length && out.length < cap; c++) {
                while (groups[c].length > 0) {
                    if (take(groups[c].shift())) {
                        progressed = true;
                        break;
                    }
                }
            }
        }
    }

    var section = pinnedCount > 0 ? "pinned" : usedCount > 0 ? "mostUsed" : "";
    return { rows: out, section: section };
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = {
        REST_LIMIT: REST_LIMIT,
        categoryKey: categoryKey,
        categoryRank: categoryRank,
        categoryList: categoryList,
        restOrder: restOrder
    };
}

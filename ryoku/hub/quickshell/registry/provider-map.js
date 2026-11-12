.pragma library

// provider rows tag an old page, tab and group; this places each in a new page
// and section. "page|tab|group" wins over "page|tab|*", then "page|*|group"
var table = {
    "animations||ANIMATIONS": ["animations", "per-animation"],
    "animations||CURVES": ["animations", "curves"],
    "animations||(action bar)": ["animations", "style"],
    "animations|Per-animation|*": ["animations", "per-animation"],
    "animations|Window style|*": ["animations", "open-close"],

    "layerrules|*|*": ["app-rules", "shell-layers"],

    "plugins|Cursor motion|*": ["mouse-touchpad", "pointer"],
    "plugins|Extra borders|*": ["window-style", "borders"],
    "plugins|Focus flash|*": ["animations", "focus-flash"],
    "plugins|Glass|*": ["window-style", "blur"],
    "plugins|Image borders|*": ["window-style", "borders"],
    "plugins|Key sounds|*": ["sounds", "key-sounds"],
    "plugins|Title bars|*": ["window-style", "title-bars"],
    "plugins|Touch gestures|*": ["mouse-touchpad", "gestures"],

    "windowmanager|Display|HDR": ["displays", "each-display"],
    "windowmanager|Layout|TILING": ["window-behaviour", "tiling"],
    "windowmanager|Layout|DWINDLE": ["window-behaviour", "tiling"],
    "windowmanager|Layout|MASTER": ["window-behaviour", "tiling"],
    "windowmanager|Layout|COLUMN FOCUS": ["window-behaviour", "focus"],
    "windowmanager|Layout|DRAG AND DROP": ["window-behaviour", "moving"],
    "windowmanager|Layout|FRAME": ["window-style", "borders"],
    "windowmanager|Layout|ROOM FOR BARS AND DOCKS": ["window-behaviour", "room"],
    "windowmanager|Layout|TABBED COLUMNS": ["window-behaviour", "tabbed"],
    "windowmanager|Layout|URGENT WINDOWS": ["window-behaviour", "urgent"],
    "windowmanager|Layout|WINDOW HEIGHT": ["window-behaviour", "new-windows"],
    "windowmanager|Layout|WINDOW WIDTH": ["window-behaviour", "new-windows"],
    "windowmanager|Layout|NEW WINDOWS": ["window-behaviour", "new-windows"],
    "windowmanager|Layout|WORKSPACE BACKGROUND": ["window-behaviour", "workspaces"],
    "windowmanager|Layout|WORKSPACES": ["window-behaviour", "workspaces"],
    "windowmanager|Layout|WORKSPACE GRID": ["window-behaviour", "workspaces"],
    "windowmanager|Layout|SETTINGS WINDOW": null,

    "windowmanager|Behaviour|DRAG GESTURES": ["mouse-touchpad", "gestures"],
    "windowmanager|Behaviour|KEY SOUNDS": ["sounds", "key-sounds"],
    "windowmanager|Behaviour|MODIFIER KEY": ["shortcuts", "modifier"],
    "windowmanager|Behaviour|MOTION": ["animations", "speed"],
    "windowmanager|Behaviour|OVERVIEW": ["window-behaviour", "overview"],
    "windowmanager|Behaviour|WINDOW SWITCHER": ["window-behaviour", "overview"],
    "windowmanager|Behaviour|POINTER": ["mouse-touchpad", "pointer"],
    "windowmanager|Behaviour|PRIVACY": ["screen-capture", "privacy"],
    "windowmanager|Behaviour|SCREENSHOTS": ["screen-capture", "screenshots"],
    "windowmanager|Behaviour|SESSION": ["power", "power-key"],
    "windowmanager|Behaviour|STARTUP": ["startup-apps", "startup"],
    "windowmanager|Behaviour|WORKSPACES": ["window-behaviour", "workspaces"],
    "windowmanager|Behaviour|CLICK TO FOCUS": ["window-behaviour", "focus"],
    "windowmanager|Behaviour|COMPATIBILITY": ["window-behaviour", "more"],
    "windowmanager|Behaviour|DESKTOP ZOOM": ["window-behaviour", "zoom"],

    "windowmanager|Focus deck|*": ["window-style", "behind"],

    "windowmanager|Look|BLUR": ["window-style", "blur"],
    "windowmanager|Look|TITLE BARS": ["window-style", "title-bars"],
    "windowmanager|Look|WINDOW BUTTONS": ["window-style", "title-bars"],
    "windowmanager|Look|DECORATION RULES": ["window-style", "title-bars"],

    "windowmanager|Motion|WOBBLY WINDOWS": ["animations", "wobbly"]
};

function placeOf(row) {
    var page = row.page || "windowmanager", tab = row.tab || "", group = row.group || "";
    var keys = [page + "|" + tab + "|" + group, page + "|" + tab + "|*", page + "|*|" + group, page + "|*|*"];
    for (var i = 0; i < keys.length; i++)
        if (table.hasOwnProperty(keys[i]))
            return { known: true, place: table[keys[i]] };
    return { known: false, place: null };
}

// desktop store keys drop their "desktop." prefix, as the schema files write them
function storeKey(key) {
    return String(key || "").replace(/^desktop\./, "");
}

function whenOf(when) {
    if (!when)
        return null;
    var keys = Object.keys(when);
    if (keys.length === 0)
        return null;
    return { bind: { store: "desktop", key: storeKey(keys[0]) }, is: when[keys[0]] };
}

function optionsOf(row) {
    var opts = row.opts || [];
    var labels = row.optLabels || {};
    var out = [];
    for (var i = 0; i < opts.length; i++) {
        var v = opts[i];
        var label = labels[v] !== undefined ? labels[v] : String(v);
        out.push({ value: v, label: label.charAt(0).toUpperCase() + label.slice(1).replace(/_/g, " ") });
    }
    return out;
}

function controlOf(row) {
    switch (row.ctl) {
    case "sw": return "toggle";
    case "slid": return "slider";
    case "step":
        var whole = Math.floor(row.lo) === row.lo && Math.floor(row.hi) === row.hi;
        return whole ? "stepper" : "slider";
    case "seg":
    case "chips":
        return (row.opts || []).length <= 4 ? "segmented" : "picker";
    case "pick": return "picker";
    case "text": return "text";
    case "color": return "color";
    case "action": return "action";
    default: return "";
    }
}

// one provider row as a schema row
function normalize(row, place) {
    var control = controlOf(row);
    var kw = String(row.keywords || "").split(/\s+/).filter(function (w) { return w.length > 0; });
    var out = {
        id: String(row.key || row.label).replace(/^wm\.[^.]+\./, "").replace(/[^A-Za-z0-9]+/g, "-").toLowerCase(),
        section: place[1],
        label: row.label,
        detail: row.desc || "",
        keywords: kw,
        control: control,
        bind: { store: "desktop", key: storeKey(row.key) },
        more: row.adv === true,
        provider: true
    };
    if (row.caps)
        out.needs = { cap: row.caps };
    if (row.lo !== undefined) out.min = row.lo;
    if (row.hi !== undefined) out.max = row.hi;
    if (row.unit && row.unit !== "%") out.unit = row.unit;
    if (row.pct) out.pct = true;
    if (control === "slider")
        out.step = row.pct ? 0.01 : Math.max(0.001, (row.hi - row.lo) / 100);
    if (row.opts) out.options = optionsOf(row);
    if (row.eg) out.placeholder = row.eg;
    var w = whenOf(row.when);
    if (w) out.when = w;
    return out;
}

// { pages: { pageId: [rows] }, unknown: [rows the table does not place] }
function mapRows(rows) {
    var pages = {}, unknown = [];
    for (var i = 0; i < (rows || []).length; i++) {
        var r = rows[i];
        var p = placeOf(r);
        if (!p.known) {
            unknown.push(r);
            continue;
        }
        if (!p.place)
            continue;
        var n = normalize(r, p.place);
        if (!n.control)
            continue;
        (pages[p.place[0]] = pages[p.place[0]] || []).push(n);
    }
    return { pages: pages, unknown: unknown };
}

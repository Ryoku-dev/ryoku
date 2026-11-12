.pragma library

// plain names for the provider's animation groups; others are sentence-cased
var groupNames = {
    "WINDOW": "Timing",
    "VIEW MOVEMENT": "Scrolling the view",
    "WINDOW MOVEMENT": "Window move",
    "CONFIG NOTIFICATION": "Settings reloaded notice",
    "SCREENSHOT UI": "Screenshot tool",
    "OVERVIEW ZOOM": "Overview",
    "SNAP": "Snapping into place"
};

function sentence(s) {
    s = String(s || "").trim().toLowerCase();
    return s.length ? s.charAt(0).toUpperCase() + s.slice(1) : "";
}

function groupName(group) {
    return groupNames[group] || sentence(group);
}

function slug(s) {
    return String(s || "").toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
}

// one animation's settings, keys in name order: the order a value takes once QML has held it
function leafShape(leaf, it) {
    it = it || {};
    return { bezier: String(it.bezier || ""), enabled: it.enabled !== false, leaf: leaf,
             speed: Number(it.speed) || 1, style: String(it.style || "") };
}

// the animation tree a provider exposes for editing: its saved-curves list row
function curveBase(rawRows) {
    for (var i = 0; i < rawRows.length; i++) {
        var r = rawRows[i];
        if (r.ctl === "list" && /\.curves$/.test(String(r.key || "")))
            return String(r.key).replace(/\.curves$/, "");
    }
    return "";
}

// "Window open · speed" -> "Window open"; group and root marks drop
function leafName(rawRows, leaf) {
    var tag = "[leaf=" + leaf + "]";
    for (var i = 0; i < rawRows.length; i++) {
        var r = rawRows[i];
        if (String(r.key || "").indexOf(tag) >= 0) {
            var name = String(r.label || "").split(" \u00b7 ")[0];
            return name.replace(/\s*\((group|root)\)\s*$/, "").trim();
        }
    }
    return leaf;
}

var styleNames = {
    "": "Default",
    "slide": "Slide",
    "slidevert": "Slide vertically",
    "fade": "Fade",
    "slidefade": "Slide and fade",
    "slidefadevert": "Slide and fade vertically",
    "popin": "Pop in",
    "gnomed": "Unfold"
};

function styleLabel(v) {
    v = String(v || "");
    if (styleNames[v] !== undefined)
        return styleNames[v];
    var base = v.split(" ")[0];
    return styleNames[base] !== undefined ? styleNames[base] + " " + v.slice(base.length).trim() : v;
}

// [{ value, label }] for a leaf's style, from the provider row; the current value stays choosable
function styleOptions(rawRows, leaf, current) {
    var tag = "[leaf=" + leaf + "].style";
    var opts = [];
    for (var i = 0; i < rawRows.length; i++) {
        var r = rawRows[i];
        if (String(r.key || "").indexOf(tag) >= 0) {
            opts = (r.opts || []).map(function (o) { return { value: String(o), label: styleLabel(o) }; });
            break;
        }
    }
    if (opts.length > 0 && current !== undefined && current !== null
            && !opts.some(function (o) { return o.value === String(current); }))
        opts.push({ value: String(current), label: styleLabel(current) });
    return opts;
}

// one value as a row shows it: an option's label, or the number and its unit
function shown(row, v, tr) {
    if (v === undefined || v === null || v === "")
        return "";
    var opts = row.options || [];
    for (var i = 0; i < opts.length; i++)
        if (JSON.stringify(opts[i].value) === JSON.stringify(v))
            return tr(opts[i].label);
    if (typeof v === "number") {
        var text = Number.isInteger(v) ? String(v) : String(Math.round(v * 100) / 100);
        if (row.pct)
            return Math.round(v * 100) + "%";
        return row.unit ? text + " " + row.unit : tr(row.label) + " " + text;
    }
    if (typeof v === "boolean")
        return tr(row.label) + " " + (v ? tr("on") : tr("off"));
    return "";
}

// "Spring, Damping 1, Stiffness 800" for the rows of one group
function groupSummary(rows, valueOf, met, tr) {
    var parts = [];
    for (var i = 0; i < rows.length; i++) {
        var r = rows[i];
        if (r.more === true || r.control === "text" || !met(r))
            continue;
        var t = shown(r, valueOf(r.bind), tr);
        if (t)
            parts.push(t);
    }
    return parts.join(", ");
}

// curves

var feels = [
    { name: "Linear", c: [0.0, 0.0, 1.0, 1.0] },
    { name: "Gentle", c: [0.45, 0.0, 0.25, 1.0] },
    { name: "Smooth", c: [0.25, 0.1, 0.25, 1.0] },
    { name: "Snappy", c: [0.3, 0.0, 0.1, 1.0] },
    { name: "Bouncy", c: [0.34, 1.5, 0.64, 1.0] }
];

// a curve's name as a person reads it: "ryokuBloom" -> "Bloom", "easeOutQuint" -> "Ease out quint",
// "custom1" -> "Custom 1"; the stored name stays what the window manager knows
function curveLabel(name) {
    var s = String(name || "").replace(/^ryoku(?=[A-Z])/, "");
    s = s.replace(/([a-z])([A-Z])/g, "$1 $2").replace(/([A-Za-z])([0-9])/g, "$1 $2");
    return sentence(s);
}

function names(defaults, draft) {
    var seen = {}, out = [];
    (defaults || []).concat(draft || []).forEach(function (c) {
        if (c && c.name && !seen[c.name]) {
            seen[c.name] = true;
            out.push(c.name);
        }
    });
    return out;
}

function find(list, name) {
    for (var i = 0; i < (list || []).length; i++)
        if (list[i] && list[i].name === name)
            return list[i];
    return null;
}

function curveOf(defaults, draft, name) {
    var c = find(draft, name) || find(defaults, name);
    return c ? { name: name, x0: Number(c.x0), y0: Number(c.y0), x1: Number(c.x1), y1: Number(c.y1) }
             : { name: name, x0: 0.25, y0: 0.1, x1: 0.25, y1: 1 };
}

function close(a, b) {
    return Math.abs(a - b) < 0.005;
}

function sameShape(a, b) {
    return close(a.x0, b.x0) && close(a.y0, b.y0) && close(a.x1, b.x1) && close(a.y1, b.y1);
}

function feelMatches(curve, c) {
    return Math.abs(curve.x0 - c[0]) < 0.03 && Math.abs(curve.y0 - c[1]) < 0.03
        && Math.abs(curve.x1 - c[2]) < 0.03 && Math.abs(curve.y1 - c[3]) < 0.03;
}

function round3(v) {
    return Math.round(v * 1000) / 1000;
}

function upsert(draft, curve) {
    var next = { name: curve.name, x0: round3(curve.x0), y0: round3(curve.y0), x1: round3(curve.x1), y1: round3(curve.y1) };
    var arr = (draft || []).slice(), found = false;
    for (var i = 0; i < arr.length; i++)
        if (arr[i] && arr[i].name === next.name) {
            arr[i] = next;
            found = true;
        }
    if (!found)
        arr.push(next);
    return arr;
}

function without(draft, name) {
    return (draft || []).filter(function (c) { return c && c.name !== name; });
}

function freshName(existing) {
    var name = "custom", n = 1;
    while (existing.indexOf(name) >= 0)
        name = "custom" + n++;
    return name;
}

.pragma library

// the layout and variant strings hold one or two layouts by position ("fr,us",
// "azerty,"); the option string holds every option, one family per control
var capsIds = ["caps:escape", "ctrl:nocaps", "caps:swapescape", "caps:none"];
var composeIds = ["compose:ralt", "compose:menu"];
// grp:caps_toggle belongs to the family so it never leaks into the free text,
// though the switch control does not offer it
var switchIds = ["grp:alt_shift_toggle", "grp:win_space_toggle", "grp:caps_toggle"];
var swapId = "altwin:swap_alt_win";

function parts(s) {
    return String(s || "").split(",");
}

function primary(s) {
    return parts(s)[0].trim();
}

function secondary(s) {
    var p = parts(s);
    return p.length > 1 ? p[1].trim() : "";
}

function tokens(options) {
    return parts(options).map(t => t.trim()).filter(t => t.length > 0);
}

// the first token of a family wins
function pick(options, ids) {
    var t = tokens(options);
    for (var i = 0; i < t.length; i++)
        if (ids.indexOf(t[i]) >= 0)
            return t[i];
    return "";
}

function known() {
    return capsIds.concat(composeIds).concat(switchIds).concat([swapId]);
}

function extra(options) {
    var k = known();
    return tokens(options).filter(t => k.indexOf(t) < 0).join(",");
}

// the option string with one family replaced; "" drops it
function withFamily(options, ids, value) {
    var out = tokens(options).filter(t => ids.indexOf(t) < 0);
    if (value)
        out.push(value);
    return out.join(",");
}

// the option string with the free text replaced and every family kept
function withExtra(options, text) {
    var k = known();
    var keep = tokens(options).filter(t => k.indexOf(t) >= 0);
    return keep.concat(tokens(text)).join(",");
}

function layoutString(first, second) {
    return second ? first + "," + second : first;
}

// the variant belongs to the first layout; a second layout keeps its slot empty
function variantString(variant, second) {
    return second && variant ? variant + "," : String(variant || "");
}

// xkb names a variant after its language ("French (AZERTY)"); inside the
// chosen layout only the part in brackets says anything
function variantLabel(name) {
    var m = String(name || "").match(/^[^(]+\((.+)\)$/);
    if (!m)
        return String(name || "");
    var q = m[1].trim();
    return q.charAt(0).toUpperCase() + q.slice(1);
}

function nameOf(list, code) {
    for (var i = 0; i < (list || []).length; i++)
        if (list[i].code === code)
            return list[i].name;
    return code;
}

// entries whose name or code holds every word of the query
function filter(list, query) {
    var words = String(query || "").toLowerCase().split(/\s+/).filter(w => w.length > 0);
    if (words.length === 0)
        return list;
    return list.filter(function (e) {
        var hay = (String(e.name) + " " + String(e.code)).toLowerCase();
        return words.every(w => hay.indexOf(w) >= 0);
    });
}

// Kit's surface routes. The shell hears "cheatsheet" (Super+K: open or close),
// "cheatsheet#<section>" and "cheatsheet#<section>/<page>" (deep links: open
// there, never close). A page id may itself hold ":" and "/".

var BASE = "cheatsheet";
var SECTIONS = ["downloads", "keys", "commands", "phone", "devtools", "typing"];

// parse reads a surface id. kit is false for another surface's id; toggle is
// true only for the bare id. An unknown section opens Kit where it was.
function parse(id) {
    var value = String(id || "");
    if (value === "ask#downloads")
        return { kit: true, toggle: false, section: "downloads", page: "queue" };
    var split = value.indexOf("#");
    var base = split >= 0 ? value.slice(0, split) : value;
    if (base !== BASE)
        return { kit: false, toggle: false, section: "", page: "" };
    if (split < 0)
        return { kit: true, toggle: true, section: "", page: "" };
    var rest = value.slice(split + 1);
    var slash = rest.indexOf("/");
    var section = slash >= 0 ? rest.slice(0, slash) : rest;
    var page = slash >= 0 ? rest.slice(slash + 1) : "";
    if (SECTIONS.indexOf(section) < 0)
        return { kit: true, toggle: false, section: "", page: "" };
    return { kit: true, toggle: false, section: section, page: page };
}


if (typeof module !== "undefined" && module.exports)
    module.exports = { BASE: BASE, SECTIONS: SECTIONS, parse: parse };

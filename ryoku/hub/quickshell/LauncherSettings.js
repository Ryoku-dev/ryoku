.pragma library

// What Settings -> App Launcher may offer for a Store launcher or one a
// subshell or bar style contributes. A launcher says so in a `settings.json`
// beside its entry (Main.qml):
//
//   { "capabilities": ["overlay", "hiddenApps"],
//     "sections": [{ "id": "internet", "label": "Internet" }] }
//
// `capabilities` uses the names catalog.json gives the shipped launchers, plus
// the ones in KNOWN below. `sections` lists the groups the launcher files apps
// under, for the "sections" capability. Anything else in the file is ignored,
// and a missing or malformed file offers nothing.

var KNOWN = ["background", "overlay", "hiddenApps", "grid", "sections", "pinnedApps"];
var MAX_CHARS = 65536;
var MAX_SECTIONS = 64;
var MAX_LABEL = 80;

// The settings.json beside a launcher's entry file, given as an absolute path;
// "" when there is no such file to look for.
function besideEntry(entryPath) {
    var entry = String(entryPath || "");
    var slash = entry.lastIndexOf("/");
    if (entry.charAt(0) !== "/" || slash <= 0 || slash === entry.length - 1)
        return "";
    return entry.slice(0, slash) + "/settings.json";
}

function parse(text) {
    var empty = { capabilities: [], sections: [] };
    var source = String(text || "");
    if (source.length === 0 || source.length > MAX_CHARS)
        return empty;
    var raw;
    try {
        raw = JSON.parse(source);
    } catch (e) {
        return empty;
    }
    if (!raw || typeof raw !== "object")
        return empty;
    var capabilities = [];
    var asked = Array.isArray(raw.capabilities) ? raw.capabilities : [];
    for (var i = 0; i < asked.length; i++)
        if (KNOWN.indexOf(asked[i]) >= 0 && capabilities.indexOf(asked[i]) < 0)
            capabilities.push(asked[i]);
    var sections = [];
    var seen = {};
    var rows = Array.isArray(raw.sections) ? raw.sections : [];
    for (var j = 0; j < rows.length && sections.length < MAX_SECTIONS; j++) {
        var row = rows[j];
        if (!row || typeof row.id !== "string" || typeof row.label !== "string")
            continue;
        var label = row.label.trim();
        if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(row.id)
                || label.length === 0 || label.length > MAX_LABEL
                || /[\u0000-\u001f\u007f]/.test(label) || seen[row.id])
            continue;
        seen[row.id] = true;
        sections.push({ id: row.id, label: label });
    }
    // The sections card needs rows to order; without them it would be empty.
    if (sections.length === 0)
        capabilities = capabilities.filter(function (name) { return name !== "sections"; });
    return { capabilities: capabilities, sections: sections };
}

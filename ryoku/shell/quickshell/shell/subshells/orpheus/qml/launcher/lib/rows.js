// How a result row reads in the track list: its fallback symbol, its kind,
// its index label, and whether an open window or a player belongs to its app.

function text(value) {
    return String(value == null ? "" : value);
}

// "app" | "window" | "calc" | "web" | "file" | "action" | "media" | "ask" | "other"
function kindKey(row) {
    var provider = text(row && row.providerId);
    switch (provider) {
    case "apps": return "app";
    case "windows": return "window";
    case "calc": return "calc";
    case "web": return "web";
    case "find":
    case "recent": return "file";
    case "actions": return "action";
    case "mpris": return "media";
    case "ask": return "ask";
    default: return "other";
    }
}

// A Material symbol for a row that carries no icon of its own.
function glyphFor(row) {
    var type = text(row && row.type);
    switch (kindKey(row)) {
    case "app": return "apps";
    case "window": return "select_window";
    case "calc": return "calculate";
    case "web": return "public";
    case "action": return "bolt";
    case "media": return "music_note";
    case "ask": return "auto_awesome";
    case "file":
        if (type === "Folder") return "folder";
        if (type === "Image") return "image";
        if (type === "Video") return "movie";
        if (type === "Recent") return "history";
        return "description";
    default: return "radio_button_unchecked";
    }
}

// The folder a path sits in, home shortened to ~.
function parentFolder(path, home) {
    var value = text(path).replace(/^file:\/\//, "").replace(/\/+$/, "");
    var cut = value.lastIndexOf("/");
    var folder = cut > 0 ? value.slice(0, cut) : (cut === 0 ? "/" : "");
    var h = text(home).replace(/\/+$/, "");
    if (h.length > 0 && (folder === h || folder.indexOf(h + "/") === 0))
        folder = "~" + folder.slice(h.length);
    return folder;
}

// Rows 1 to 9 carry the Alt+N index; Lithic sets it as two print digits.
function indexLabel(index, lithic) {
    var n = Number(index) + 1;
    if (!(n >= 1 && n <= 9))
        return "";
    return lithic ? "0" + n : String(n);
}

function countLabel(count, lithic) {
    var n = Math.max(0, Math.floor(Number(count) || 0));
    if (!lithic)
        return String(n);
    var digits = String(n);
    while (digits.length < 3)
        digits = "0" + digits;
    return digits;
}

function appName(value) {
    return text(value).toLowerCase().replace(/\.desktop$/, "");
}

function tail(value) {
    var cut = value.lastIndexOf(".");
    return cut >= 0 ? value.slice(cut + 1) : value;
}

// Whether a window app id or an MPRIS desktop entry names this desktop entry
// (its id or StartupWMClass), tolerating reverse-DNS ids on one side only.
function sameApp(entryId, startupClass, other) {
    var target = appName(other);
    if (target.length === 0)
        return false;
    var names = [appName(entryId), appName(startupClass)].filter(function (n) { return n.length > 0; });
    for (var i = 0; i < names.length; i++) {
        var name = names[i];
        if (name === target)
            return true;
        var nameDotted = name.indexOf(".") >= 0;
        var targetDotted = target.indexOf(".") >= 0;
        if (nameDotted !== targetDotted && tail(name) === tail(target))
            return true;
    }
    return false;
}

// What a row shows, so a refresh that changes nothing visible keeps the
// delegates instead of rebuilding them (and reloading every icon).
function signature(rows) {
    var source = Array.isArray(rows) ? rows : [];
    var parts = [];
    for (var i = 0; i < source.length; i++) {
        var row = source[i] || {};
        var actions = [row.primaryAction].concat(Array.isArray(row.secondaryActions) ? row.secondaryActions : []);
        var names = actions.map(function (action) {
            return action ? text(action.id) + "=" + text(action.name) + (action.enabled === false ? "!" : "") : "";
        });
        parts.push([text(row.resultKey), text(row.title), text(row.subtitle), text(row.icon),
            text(row.type), text(row.preview), row.disabled === true ? "1" : "0",
            names.join(",")].join("\u001f"));
    }
    return parts.join("\u001e");
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = {
        kindKey: kindKey,
        glyphFor: glyphFor,
        parentFolder: parentFolder,
        indexLabel: indexLabel,
        countLabel: countLabel,
        sameApp: sameApp,
        signature: signature
    };
}

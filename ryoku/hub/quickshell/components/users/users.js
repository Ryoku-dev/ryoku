.pragma library
.import "../../schema/users.js" as Schema

// a plain array from json data, which a model or property may hand back as a sequence
function list(v) {
    return v !== null && v !== undefined && typeof v === "object" && v.length !== undefined ? Array.prototype.slice.call(v) : [];
}

// what is wrong with a new user name by ryoku-host's rule: "empty", "pattern" or ""
function nameProblem(name) {
    if (name === "")
        return "empty";
    if (!/^[a-z_][a-z0-9_-]{0,31}$/.test(name))
        return "pattern";
    return "";
}

function gibToBytes(text) {
    var gib = Number(String(text || "").trim().replace(",", "."));
    if (!isFinite(gib) || gib <= 0)
        return 0;
    return Math.round(gib * 1073741824);
}

function bytesToGib(bytes) {
    var n = Number(bytes || 0) / 1073741824;
    return n > 0 ? String(Math.round(n * 100) / 100) : "";
}

// binary units, one decimal: "1.2 GB"
function size(bytes) {
    var n = Number(bytes || 0);
    if (n < 1024)
        return Math.round(n) + " B";
    var units = ["KB", "MB", "GB", "TB"];
    var i = -1;
    do {
        n /= 1024;
        i++;
    } while (n >= 1024 && i < units.length - 1);
    return (n >= 10 ? Math.round(n) : Math.round(n * 10) / 10) + " " + units[i];
}

function permissionTitle(def, i18n) {
    var known = Schema.permission(String(def.id || ""));
    return known ? i18n.tr(known.label) : String(def.label || def.id || "");
}

function permissionDetail(def, i18n) {
    var known = Schema.permission(String(def.id || ""));
    return known ? i18n.tr(known.detail) : "";
}

// ryoku-host's plan words (actions and kinds of system change) in plain words; unknown ones pass through
var planWords = {
    "enable": "Turned on", "disable": "Turned off", "keep": "Kept", "remove": "Removed", "delete": "Deleted",
    "pacman": "Package manager", "pacman-hook": "Package hook", "capability": "Tool permission",
    "group": "Group", "wordlists": "Wordlists", "hosts": "Hosts file", "polkit": "Administrator rule", "file": "File"
};

function planWord(v, i18n) {
    var key = String(v || "");
    return planWords[key] ? i18n.tr(planWords[key]) : key;
}

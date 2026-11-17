// Pure helpers for the Ryoku apps shelf, the Apps search over gpk and store
// sources. QML owns processes and copy; the job events are read by Ryoku.Ui's
// PackageJob (lib/jobevents.js). This file only shapes data, so node can test it.

var systemManagers = ["pacman", "dnf", "xbps"];
var searchManagers = ["flatpak", "pacman", "aur", "dnf", "xbps"];

function sourceKind(source) {
    var s = String(source || "");
    if (s === "flatpak")
        return "flatpak";
    if (s === "aur")
        return "aur";
    if (systemManagers.indexOf(s) !== -1)
        return "system";
    return "";
}

// the chip on a result: where the app comes from, in the words a user knows
function sourceChip(source) {
    var s = String(source || "");
    if (s === "flatpak")
        return "FLATHUB";
    if (s === "aur")
        return "AUR";
    return s.toUpperCase();
}

function formatBytes(value) {
    var n = Number(value || 0);
    if (!(n > 0))
        return "";
    if (n >= 1073741824)
        return (n / 1073741824).toFixed(1) + " GB";
    if (n >= 1048576)
        return (n / 1048576).toFixed(n >= 104857600 ? 0 : 1) + " MB";
    if (n >= 1024)
        return Math.round(n / 1024) + " KB";
    return Math.round(n) + " B";
}

// `ryoku apps list --json`: the shelf shows the apps a user can add or take
// away; the built-in store is where they already are, so it stays off it.
function parseRyokuApps(text) {
    var doc = JSON.parse(String(text || ""));
    if (!doc || !Array.isArray(doc.apps))
        throw new Error("apps are missing");
    return doc.apps.filter(function(app) {
        return app && String(app.id || "") !== "" && app.builtin !== true;
    }).map(function(app) {
        return {
            id: String(app.id),
            name: String(app.name || app.id),
            summary: String(app.summary || ""),
            installed: app.installed === true,
            version: String(app.version || ""),
            latest: String(app.latest || ""),
            update: app.installed === true && app.update === true,
            sizeBytes: Number(app.size_bytes || 0),
            removable: app.removable !== false,
            desktopId: String(app.desktop_id || ""),
            lane: String(app.lane || ""),
            channel: String(app.channel || "stable")
        };
    });
}

// the actions a Ryoku app card offers, primary first
function ryokuActions(app) {
    if (!app)
        return [];
    if (!app.installed)
        return ["install"];
    var out = [];
    if (app.update)
        out.push("update");
    if (app.desktopId !== "")
        out.push("open");
    if (app.removable)
        out.push("remove");
    return out;
}

// the host helper stops when its stdin says so: pkexec's child runs as root,
// so a signal from Ryostore could never reach it
function ryokuCommand(app, action) {
    return ["pkexec", "ryoku-host", "apps", "--stdin-cancel", "ryoku", String(action), String(app.id)];
}

// a pacman epoch ("1:2.0.0-1") is bookkeeping, not part of the version a person reads
function displayVersion(version) {
    return String(version || "").replace(/^[0-9]+:/, "");
}

// gpk 0.7.0 brought the progress events installs are read from and the
// required_by a removal checks; an older gpk still searches
var gpkEventsVersion = [0, 7, 0];

// "gpk v0.6.8" (--version) or "v0.7.0-dev" (a JSON envelope) as [0, 6, 8]; a
// pre-release counts as its release, since it was built for it
function gpkVersion(text) {
    var match = String(text || "").match(/v?([0-9]+)\.([0-9]+)\.([0-9]+)/);
    return match ? [Number(match[1]), Number(match[2]), Number(match[3])] : null;
}

// false only for a gpk known to be older than 0.7.0; an unknown one is let
// through and its job reports what went wrong
function gpkCanInstall(text) {
    var v = gpkVersion(text);
    if (!v)
        return true;
    for (var i = 0; i < 3; i++)
        if (v[i] !== gpkEventsVersion[i])
            return v[i] > gpkEventsVersion[i];
    return true;
}

function searchVersion(text) {
    var doc = JSON.parse(String(text || ""));
    return doc && typeof doc.gpk_version === "string" ? doc.gpk_version : "";
}

// `gpk search --json`: one row per installable result; managers outside the
// Apps search (pip, npm, ...) never reach it, and Flatpak rows install by
// their application id when gpk reports one.
function parseSearch(text) {
    var doc = JSON.parse(String(text || ""));
    var data = doc && Array.isArray(doc.data) ? doc.data : null;
    if (!data)
        throw new Error("search data is missing");
    var seen = {};
    var rows = [];
    data.forEach(function(pkg) {
        var kind = sourceKind(pkg && pkg.source);
        var name = String(pkg && pkg.name || "");
        if (kind === "" || name === "")
            return;
        var installName = String(pkg.id || pkg.app_id || name);
        var key = String(pkg.source) + ":" + installName;
        if (seen[key])
            return;
        seen[key] = true;
        rows.push({
            key: key,
            name: name,
            installName: installName,
            summary: String(pkg.description || ""),
            version: String(pkg.version || ""),
            source: String(pkg.source),
            kind: kind,
            repository: String(pkg.repository || ""),
            installed: pkg.installed === true,
            sizeBytes: Number(pkg.size_bytes || 0)
        });
    });
    return rows;
}

function searchCommand(query) {
    return ["gpk", "search", "--json", "--quiet", "--limit", "60",
            "--manager", searchManagers.join(","), String(query)];
}

// the command a result's action runs: Flatpak as the user, system packages
// through the polkit-guarded host helper, the AUR never here (a terminal)
function packageCommand(row, action) {
    if (!row)
        return null;
    if (row.kind === "flatpak")
        return ["gpk", String(action), "--manager", "flatpak", "--yes", "--events", row.installName];
    if (row.kind === "system")
        return ["pkexec", "ryoku-host", "apps", "--stdin-cancel", String(action), "--manager", row.source, row.installName];
    return null;
}

// the plan gpk would run, read before a removal is confirmed (nothing runs)
function removePlanCommand(row) {
    if (!row || (row.kind !== "system" && row.kind !== "flatpak"))
        return null;
    return ["gpk", "remove", "--manager", row.source, "--json", row.installName];
}

// the installed packages that still need what a removal plan takes away
function requiredBy(text) {
    var doc = JSON.parse(String(text || ""));
    var steps = doc && doc.data && Array.isArray(doc.data.steps) ? doc.data.steps : [];
    var seen = {};
    var out = [];
    steps.forEach(function(step) {
        (Array.isArray(step && step.required_by) ? step.required_by : []).forEach(function(name) {
            var n = String(name || "");
            if (n !== "" && !seen[n]) {
                seen[n] = true;
                out.push(n);
            }
        });
    });
    return out;
}

// the line the terminal runs for an AUR package
function aurLine(row, action) {
    var verb = action === "remove" ? "remove" : "install";
    return "gpk " + verb + " --manager aur " + shellQuote(row.installName);
}

function shellQuote(value) {
    return "'" + String(value).replace(/'/g, "'\\''") + "'";
}
// `ryostore sources list --json`, split for the two lists
function parseSources(text) {
    var doc = JSON.parse(String(text || ""));
    if (!doc || !Array.isArray(doc.sources))
        throw new Error("sources are missing");
    var official = [];
    var mine = [];
    doc.sources.forEach(function(src) {
        if (!src || String(src.id || "") === "")
            return;
        var row = {
            id: String(src.id),
            name: String(src.name || src.id),
            url: String(src.url || ""),
            official: src.official === true,
            enabled: src.enabled !== false,
            status: String(src.status || "ok"),
            error: String(src.error || ""),
            note: String(src.note || ""),
            description: String(src.description || ""),
            homepage: String(src.homepage || ""),
            items: Number(src.items || 0),
            installed: Number(src.installed || 0),
            categories: Array.isArray(src.categories) ? src.categories.slice() : []
        };
        (row.official ? official : mine).push(row);
    });
    return { official: official, mine: mine };
}

// a first check before the backend's own: what it accepts, nothing more
function sourceUrlProblem(url) {
    var text = String(url || "").trim();
    if (text === "")
        return "empty";
    if (/^https:\/\/[^\s/]+/i.test(text))
        return "";
    if (/^http:\/\/(127\.0\.0\.1|localhost|\[::1\])(:[0-9]+)?(\/|$)/i.test(text))
        return "";
    if (/^file:\/\/\/\S/.test(text))
        return "";
    return "scheme";
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { sourceKind, sourceChip, formatBytes, parseRyokuApps, ryokuActions, ryokuCommand, displayVersion, gpkVersion, gpkCanInstall, searchVersion, parseSearch, searchCommand, packageCommand, removePlanCommand, requiredBy, aurLine, shellQuote, parseSources, sourceUrlProblem };

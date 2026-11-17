.pragma library

// Installed apps: what `gpk apps --json` and `ryoku apps list --json` print, read into the
// page's words. Pure, so apps.test.mjs holds the sizes, groups, filters and commands to it.

var MB = 1024 * 1024;
var GB = 1024 * MB;

// the size groups, largest first; an app whose size is not known goes last
var groups = [
    { id: "over-1gb", label: "Over 1 GB" },
    { id: "100mb-1gb", label: "100 MB to 1 GB" },
    { id: "10-100mb", label: "10 to 100 MB" },
    { id: "under-10mb", label: "Under 10 MB" },
    { id: "unknown", label: "Unknown" }
];

// where an app came from, in the order the filters list them
var kinds = [
    { id: "system", label: "System packages" },
    { id: "aur", label: "AUR" },
    { id: "flatpak", label: "Flatpak" },
    { id: "account", label: "On this account" }
];

// the system package managers ryoku-host removes with, as root
var systemManagers = ["pacman", "dnf", "xbps"];

function rowsOf(text) {
    var o = JSON.parse(String(text || "").trim() || "null");
    var data = o && typeof o === "object" && "data" in o ? o.data : o;
    if (data && !Array.isArray(data) && Array.isArray(data.apps))
        data = data.apps;
    if (!Array.isArray(data))
        throw new Error("not a list of apps");
    return data;
}

function bytesOf(v) {
    var n = Number(v);
    return isFinite(n) && n > 0 ? Math.round(n) : 0;
}

// a desktop id as the launcher keeps it: the file's name without ".desktop"
function desktopId(id) {
    return String(id || "").replace(/\.desktop$/, "");
}

// the gpk that lists apps (`gpk apps`, `--events`, sizes) is 0.7.0
var gpkNeeded = "0.7.0";

// the version `gpk --version` names ("gpk v0.6.8", "gpk 0.7.0"), or "" for a build without one
function gpkVersion(text) {
    var m = /(\d+)\.(\d+)\.(\d+)/.exec(String(text || ""));
    return m ? m[1] + "." + m[2] + "." + m[3] : "";
}

// false only for a gpk that names a version older than gpkNeeded; a development build lists
function gpkListsApps(text) {
    var v = gpkVersion(text);
    if (v === "")
        return true;
    var have = v.split(".").map(Number), need = gpkNeeded.split(".").map(Number);
    for (var i = 0; i < 3; i++)
        if (have[i] !== need[i])
            return have[i] > need[i];
    return true;
}

// every app that shows in app search, one per desktop entry; throws on output that is not gpk's.
// An older gpk takes `apps` for a search and prints packages with no desktop entry: that throws
// "old gpk", so it never reads as an empty list
function parseApps(text) {
    var out = [];
    var seen = {};
    var rows = rowsOf(text);
    if (rows.length > 0 && !rows.some(r => r && typeof r === "object" && "desktop_id" in r))
        throw new Error("old gpk");
    rows.forEach(r => {
        if (!r || typeof r !== "object" || r.no_display === true)
            return;
        var id = desktopId(r.desktop_id);
        if (id === "" || seen[id])
            return;
        seen[id] = true;
        out.push({
            id: id,
            name: String(r.name || id),
            summary: String(r.generic_name || r.comment || ""),
            package: String(r.package || ""),
            source: String(r.source || ""),
            version: String(r.version || ""),
            bytes: bytesOf(r.size_bytes),
            location: String(r.location || ""),
            terminal: r.terminal === true,
            file: String(r.desktop_file || "")
        });
    });
    return out;
}

// Ryoku's own apps, in the catalogue's order; throws on output that is not `ryoku apps list`
function parseRyoku(text) {
    var o = JSON.parse(String(text || "").trim() || "null");
    if (!o || !Array.isArray(o.apps))
        throw new Error("not a list of Ryoku apps");
    return o.apps.filter(a => a && a.id).map(a => ({
        id: String(a.id),
        name: String(a.name || a.id),
        summary: String(a.summary || ""),
        installed: a.installed === true,
        version: String(a.version || ""),
        latest: String(a.latest || ""),
        update: a.update === true,
        bytes: bytesOf(a.size_bytes),
        removable: a.removable === true && a.builtin !== true,
        builtin: a.builtin === true,
        desktopId: desktopId(a.desktop_id),
        channel: String(a.channel || "stable")
    }));
}

// a package manager's app is that manager's wherever its entry lies; only what no manager
// installed (an AppImage, a web app) in your own folders is "on this account"
function kindOf(app) {
    if (app.source === "flatpak" || app.location === "flatpak")
        return "flatpak";
    if (app.source === "aur")
        return "aur";
    if (app.source === "local" && app.location === "user")
        return "account";
    return "system";
}

// the desktop itself owns some entries (Settings, RyoStore); removing them would remove Ryoku
function partOfRyoku(app) {
    return !!app && app.package === "ryoku-desktop";
}

function groupOf(bytes) {
    var b = bytesOf(bytes);
    if (b === 0)
        return "unknown";
    if (b >= GB)
        return "over-1gb";
    if (b >= 100 * MB)
        return "100mb-1gb";
    if (b >= 10 * MB)
        return "10-100mb";
    return "under-10mb";
}

function groupLabel(id) {
    var g = groups.find(x => x.id === id);
    return g ? g.label : "";
}

function kindLabel(id) {
    var k = kinds.find(x => x.id === id);
    return k ? k.label : "";
}

// "1.4 GB", "312 MB", "4.2 MB", "380 KB"; tr translates the unit patterns
function humanSize(bytes, tr) {
    var t = tr || (s => s);
    var b = bytesOf(bytes);
    if (b >= GB)
        return t("%1 GB").replace("%1", (b / GB).toFixed(1));
    if (b >= 10 * MB)
        return t("%1 MB").replace("%1", Math.round(b / MB));
    if (b >= MB)
        return t("%1 MB").replace("%1", (b / MB).toFixed(1));
    return t("%1 KB").replace("%1", Math.max(1, Math.round(b / 1024)));
}

function byName(a, b) {
    var x = a.name.toLowerCase(), y = b.name.toLowerCase();
    return x < y ? -1 : x > y ? 1 : (a.id < b.id ? -1 : a.id > b.id ? 1 : 0);
}

// "size": largest first, unknown sizes last; "name": A to Z
function sorted(apps, by) {
    var out = apps.slice();
    out.sort(by === "name" ? byName : (a, b) => (b.bytes - a.bytes) || byName(a, b));
    return out;
}

// apps in the filter ("all" or a kind) whose name, summary or package holds the query
function filtered(apps, kind, query) {
    var q = String(query || "").trim().toLowerCase();
    return apps.filter(a => (kind === "all" || !kind || kindOf(a) === kind)
        && (q === "" || (a.name + " " + a.summary + " " + a.package + " " + a.id).toLowerCase().indexOf(q) >= 0));
}

// [{ id, label, apps }] in size order, skipping empty groups
function grouped(apps, by) {
    return groups.map(g => ({ id: g.id, label: g.label, apps: sorted(apps.filter(a => groupOf(a.bytes) === g.id), by) }))
        .filter(g => g.apps.length > 0);
}

// how many apps each kind has, so a filter with none is not offered
function kindCounts(apps) {
    var out = {};
    kinds.forEach(k => out[k.id] = 0);
    apps.forEach(a => out[kindOf(a)]++);
    return out;
}

// one entry per package: several apps of one package take its space once
function packages(apps) {
    var seen = {};
    var out = [];
    apps.forEach(a => {
        var key = a.package !== "" ? a.source + ":" + a.package : "app:" + a.id;
        if (seen[key])
            return;
        seen[key] = true;
        out.push({ kind: kindOf(a), bytes: a.bytes, group: groupOf(a.bytes) });
    });
    return out;
}

// the space overview: totals by where apps came from and by size group
function space(apps) {
    var pk = packages(apps);
    var total = 0;
    var bySource = kinds.map(k => ({ id: k.id, label: k.label, bytes: 0, count: 0 }));
    var byGroup = groups.map(g => ({ id: g.id, label: g.label, bytes: 0, count: 0 }));
    pk.forEach(p => {
        total += p.bytes;
        var s = bySource.find(x => x.id === p.kind);
        s.bytes += p.bytes;
        s.count++;
        var g = byGroup.find(x => x.id === p.group);
        g.bytes += p.bytes;
        g.count++;
    });
    return {
        total: total,
        apps: apps.length,
        packages: pk.length,
        unknown: pk.filter(p => p.bytes === 0).length,
        bySource: bySource.filter(s => s.count > 0),
        byGroup: byGroup.filter(g => g.count > 0)
    };
}

// how a removal runs: { kind: "run", argv } in Settings, { kind: "terminal", line } for the
// AUR (its helper asks in a terminal), or null for an app no package manager owns and for
// Ryoku's own
function removeCommand(app) {
    if (!app || app.package === "" || partOfRyoku(app))
        return null;
    if (app.source === "flatpak")
        return { kind: "run", argv: ["gpk", "remove", "-m", "flatpak", "--yes", "--events", app.package] };
    if (app.source === "local")
        return { kind: "run", argv: ["gpk", "remove", "-m", "local", "--yes", "--events", app.package] };
    if (app.source === "aur")
        return { kind: "terminal", line: "gpk remove -m aur " + shq(app.package) };
    if (systemManagers.indexOf(app.source) >= 0)
        return { kind: "run", argv: ["pkexec", "ryoku-host", "apps", "--stdin-cancel", "remove", "--manager", app.source, app.package] };
    return null;
}

function shq(s) {
    return "'" + String(s).replace(/'/g, "'\\''") + "'";
}

// the removal's plan, read as you before asking: gpk says which installed packages need it
function planCommand(app) {
    var how = removeCommand(app);
    return how === null ? null : ["gpk", "remove", "-m", app.source, "--json", app.package];
}

// the packages that need the one being removed, from `gpk remove --json`; [] when none or
// when the plan cannot be read (the removal then says what went wrong itself)
function requiredBy(text) {
    var out = [];
    try {
        var o = JSON.parse(String(text || "").trim() || "null");
        var steps = o && o.data && Array.isArray(o.data.steps) ? o.data.steps : [];
        steps.forEach(s => (Array.isArray(s.required_by) ? s.required_by : []).forEach(n => {
            if (typeof n === "string" && n !== "" && out.indexOf(n) < 0)
                out.push(n);
        }));
    } catch (e) {
    }
    return out;
}

// what a removal takes with it: the other apps of the same package and the space it frees
function removal(app, apps) {
    var same = app.package === "" ? [app]
        : apps.filter(a => a.source === app.source && a.package === app.package);
    return { names: same.map(a => a.name), others: same.filter(a => a.id !== app.id).map(a => a.name), bytes: app.bytes };
}

function ryokuCommand(verb, id) {
    return ["pkexec", "ryoku-host", "apps", "--stdin-cancel", "ryoku", verb, id];
}

// Ryoku's own apps are listed apart, so the list leaves out the entries they open
function withoutRyoku(apps, ryoku) {
    var ids = {};
    ryoku.forEach(r => {
        if (r.desktopId !== "")
            ids[r.desktopId] = true;
    });
    return apps.filter(a => !ids[a.id]);
}

// the launcher's hidden list with one app shown or hidden
function withHidden(hidden, id, hide) {
    var list = (Array.isArray(hidden) ? hidden : []).filter(x => x !== id);
    if (hide)
        list.push(id);
    return list;
}

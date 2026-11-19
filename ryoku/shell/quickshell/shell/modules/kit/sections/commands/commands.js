function pageId(value) {
    const parts = String(value || "ryoku").split(":");
    const id = parts[0] === "group" || parts[0] === "library" ? parts[1] : parts[0];
    return ["ryoku", "linux", "saved"].indexOf(id) >= 0 ? id : "ryoku";
}
function placeholders(command) {
    const result = [];
    const pattern = /<([^<>\s][^<>]*?)>/g;
    let match;
    while ((match = pattern.exec(command)))
        if (result.indexOf(match[1]) < 0) result.push(match[1]);
    return result;
}
function shellQuote(value) { return "'" + String(value).replace(/'/g, "'\\''") + "'"; }
function fill(command, values) {
    let result = "", quote = "";
    for (let i = 0; i < command.length; i++) {
        const char = command[i];
        if (char === "\\" && quote !== "'" && i + 1 < command.length) {
            result += char + command[++i];
            continue;
        }
        if ((char === "'" || char === "\"") && (!quote || quote === char)) {
            quote = quote ? "" : char;
            result += char;
            continue;
        }
        const match = char === "<" ? /^<([^<>\s][^<>]*?)>/.exec(command.slice(i)) : null;
        if (!match) { result += char; continue; }
        const value = String(values[match[1]] || "");
        result += quote === "'" ? value.replace(/'/g, "'\\''")
            : quote === "\"" ? value.replace(/[\\\"$`]/g, "\\$&") : shellQuote(value);
        i += match[0].length - 1;
    }
    return result;
}
function runnable(entry) {
    return !!entry && !entry.danger && !entry.onlyOn && !/[\[\]|…]/.test(entry.command)
        && entry.command.indexOf("...") < 0 && !/[\n\r]/.test(entry.command);
}
function matches(entry, query, tr, group) {
    const text = [entry.title, tr(entry.title), entry.description, tr(entry.description), entry.command,
        group, tr(group), entry.tool].concat(entry.tags || []).join(" ").toLocaleLowerCase();
    return String(query || "").trim().toLocaleLowerCase().split(/\s+/).every(word => text.indexOf(word) >= 0);
}
function list(entries, libraries, favourites, page, query, tr) {
    const labels = { "favourites": tr("Favourites"), "my-commands": tr("My commands") };
    const groups = [];
    for (const library of libraries)
        for (const group of library.groups) { labels[group.id] = group.label; groups.push(group.id); }
    groups.push("favourites", "my-commands");
    const buckets = {};
    for (const entry of entries) {
        const group = page === "saved" && entry.source !== "user" ? "favourites" : entry.group;
        if (page === "saved" ? entry.source !== "user" && favourites.indexOf(entry.id) < 0 : entry.library !== page) continue;
        if (!matches(entry, query, tr, labels[group] || group)) continue;
        if (!buckets[group]) buckets[group] = [];
        buckets[group].push(entry);
    }
    const rows = [];
    for (const group of groups) for (const entry of buckets[group] || [])
        rows.push({ entry: entry, group: tr(labels[group] || group) });
    return rows;
}
function terminalLine(command, message) {
    return "bash -c " + shellQuote(command) + "; printf '\\n%s\\n' " + shellQuote(message) + "; read -r _";
}

.pragma library

// Reads the shells index ryostore writes (store/shells.json): every shipped
// subshell and installed store subshell or bar style, with its settings and
// contributions. The Hub only reads it; ryostore is its one writer.

var MAX_CHARS = 4194304;
var FALLBACK = "orpheus";

function empty() {
    return { schema: 1, active: "", fallback: FALLBACK, items: [] };
}

function parse(text) {
    var source = String(text || "");
    if (source.length === 0 || source.length > MAX_CHARS)
        return empty();
    var raw;
    try {
        raw = JSON.parse(source);
    } catch (e) {
        return empty();
    }
    if (!raw || raw.schema !== 1 || !Array.isArray(raw.items))
        return empty();
    raw.items = raw.items.filter(function (item) {
        return item && typeof item.id === "string" && item.id !== ""
            && (item.category === "subshells" || item.category === "barstyles");
    });
    if (typeof raw.fallback !== "string" || raw.fallback === "")
        raw.fallback = FALLBACK;
    return raw;
}

function item(index, id) {
    var items = index && Array.isArray(index.items) ? index.items : [];
    var wanted = String(id || "");
    for (var i = 0; i < items.length; i++)
        if (items[i].id === wanted)
            return items[i];
    return null;
}

function items(index, categories) {
    var all = index && Array.isArray(index.items) ? index.items : [];
    var wanted = Array.isArray(categories) ? categories : [];
    return all.filter(function (entry) {
        return wanted.indexOf(entry.category) !== -1;
    });
}

// Shipped paths are relative to the shell config root; store paths are
// absolute inside the unpacked item, or empty while it is packed.
function fileUrl(record, path, shellRoot) {
    var value = String(path || "");
    if (!record || value === "" || record.prepared !== true)
        return "";
    if (record.source === "shipped")
        return "file://" + String(shellRoot || "") + "/" + value;
    return value.charAt(0) === "/" ? "file://" + value : "";
}

function contributions(index, kind) {
    var out = [];
    var all = index && Array.isArray(index.items) ? index.items : [];
    for (var i = 0; i < all.length; i++) {
        var owner = all[i];
        var provided = owner.provides && Array.isArray(owner.provides[kind])
            ? owner.provides[kind] : [];
        for (var j = 0; j < provided.length; j++) {
            var contribution = provided[j];
            if (!contribution || typeof contribution.id !== "string" || contribution.id === "")
                continue;
            out.push({
                id: contribution.id,
                name: contribution.name || contribution.id,
                description: contribution.description || "",
                entry: contribution.entry || "",
                preview: contribution.preview || "",
                widget: contribution.widget || "",
                options: contribution.options || "",
                roster: Array.isArray(contribution.roster) ? contribution.roster : [],
                prepared: owner.prepared === true,
                itemId: owner.id,
                itemName: owner.name || owner.id,
                itemSource: owner.source || "store",
                record: owner
            });
        }
    }
    return out;
}

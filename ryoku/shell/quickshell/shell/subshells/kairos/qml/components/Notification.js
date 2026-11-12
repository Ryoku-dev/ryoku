.pragma library

function actions(n) {
    var all = (n && n.actions) || [], out = [];
    for (var i = 0; i < all.length; i++)
        if (all[i] && all[i].identifier !== "default" && (all[i].text || "").length > 0)
            out.push(all[i]);
    return out.slice(0, 3);
}

function defaultAction(n) {
    var all = (n && n.actions) || [];
    for (var i = 0; i < all.length; i++)
        if (all[i] && all[i].identifier === "default")
            return all[i];
    return null;
}

// icons is the Icons singleton, a library script can't see it on its own
function source(value, icons) {
    var raw = String(value || "");
    if (raw.length === 0)
        return "";
    if (raw.indexOf("image://icon/") === 0)
        raw = decodeURIComponent(raw.substring(13));
    if (raw.charAt(0) === "/")
        return "file://" + raw;
    if (raw.indexOf(":") > 0)
        return raw;
    return icons.path(raw, true);
}

function plainBody(n) {
    return String((n && n.body) || "").replace(/<[^>]*>/g, "");
}

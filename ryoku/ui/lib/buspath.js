// Settings paths for the daemon bus. A path is "bar.style" or ["bar", "style"];
// a delta frame names what it changed as key arrays. Plain module (no
// `.pragma library`) so node can test it, like its siblings in ryoku/ui/lib.

// A key list that crossed a QML `var` signal parameter arrives as a sequence
// (array-like, but Array.isArray is false), so anything with a length that is
// not a string counts as a key list.
function split(path) {
    if (path !== null && typeof path === "object" && typeof path.length === "number") {
        var out = [];
        for (var i = 0; i < path.length; i++)
            out.push(String(path[i]));
        return out;
    }
    var s = path === undefined || path === null ? "" : String(path);
    return s.length ? s.split(".") : [];
}

function join(path) {
    return split(path).join(".");
}

// The value at path, or fallback when any step is missing or not an object.
function get(root, path, fallback) {
    var keys = split(path);
    var cur = root;
    for (var i = 0; i < keys.length; i++) {
        if (cur === null || typeof cur !== "object" || !(keys[i] in cur))
            return fallback;
        cur = cur[keys[i]];
    }
    return cur === undefined ? fallback : cur;
}

// Whether a change at any of changed (null = everything) can move the value
// at path: the change is at the path, inside it, or replaces an ancestor.
function touches(path, changed) {
    if (changed === null || changed === undefined)
        return true;
    var keys = split(path);
    for (var i = 0; i < changed.length; i++) {
        var c = split(changed[i]);
        var n = Math.min(c.length, keys.length);
        var j = 0;
        while (j < n && c[j] === keys[j])
            j++;
        if (j === n)
            return true;
    }
    return false;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { split, join, get, touches };

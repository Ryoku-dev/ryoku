.pragma library

// Which output a window-manager setting applies to. With the link on, every
// setting goes to the global scope and only the first output's tab stays live;
// with it off, each output reads and writes its own scope.

function showLink(outputs) {
    return (outputs || []).length >= 2;
}

function activeOutputs(outputs, linked) {
    var list = outputs || [];
    return linked ? list.slice(0, 1) : list.slice();
}

function resolveOutput(outputs, selected) {
    var list = outputs || [];
    if (list.length === 0)
        return "";
    return list.indexOf(selected) >= 0 ? selected : list[0];
}

function writeScope(outputs, linked, selected) {
    if (linked)
        return "";
    return resolveOutput(outputs, selected);
}

function readScope(outputs, linked, selected) {
    return writeScope(outputs, linked, selected);
}

function toggleLink(linked) {
    return !linked;
}

// Settings whose value can differ per output. Everything else stays global
// even while unlinked, so the link only changes what the Hub shows as scoped.
var PER_OUTPUT = { "desktop.appearance.layout": true };

function scopedKey(output, key) {
    var rest = key.indexOf("desktop.") === 0 ? key.slice("desktop.".length) : key;
    return "desktop.outputs." + output + "." + rest;
}

function editKey(linked, output, key) {
    if (linked || !output || !PER_OUTPUT[key])
        return key;
    return scopedKey(output, key);
}

function readValue(draft, linked, output, key) {
    var k = editKey(linked, output, key);
    if (k !== key && draft[k] !== undefined)
        return draft[k];
    return draft[key];
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { showLink, activeOutputs, resolveOutput, writeScope, readScope, toggleLink, editKey, readValue };

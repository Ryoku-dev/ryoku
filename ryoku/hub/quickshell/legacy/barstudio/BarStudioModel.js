// Bar Studio's frame-bar edits, as pure functions. Every function clones the
// whole config and returns a fresh root, so an edit can never drop a subtree it
// did not touch (menus, surfaces, dock) -- the source-side half of the
// subtree-preservation invariant the daemon also enforces
// (ryoku/shell/ipc/settings.go). The rebuilt Bar Studio edits only the
// essentials: the four rails (on/off, reveal, thickness), the widgets in each
// rail's three zones (add, remove, reorder), and the frame style. The bounded
// menus and preserved surfaces keep their persisted values untouched, carried
// through every clone.

function copy(value) {
    return JSON.parse(JSON.stringify(value));
}

function zones(edge) {
    return edge === "top" || edge === "bottom" ? ["start", "center", "end"] : ["top", "center", "bottom"];
}

function axisOf(edge) {
    return edge === "top" || edge === "bottom" ? "horizontal" : "vertical";
}

// Every widget currently placed anywhere on one rail, so the add flow can offer
// only widgets not already on that rail (no rail ever holds one twice).
function railWidgets(config, edge) {
    const rail = config.rails && config.rails[edge];
    if (!rail) return [];
    const out = [];
    for (const zone of zones(edge)) if (Array.isArray(rail[zone])) for (const id of rail[zone]) out.push(id);
    return out;
}

// Add a catalogued widget to a zone. Rejected (clean no-op clone) if the id is
// not catalogued, does not fit the rail's axis, or is already on the rail.
// Adding a widget also turns the rail ON: the bottom and right rails ship off,
// so dropping a widget onto one used to land on a hidden rail and read as fully
// broken. Enabling on every add keeps "I added a widget, so show it" always
// true; a rail is hidden again with its own switch, after the fact.
function addZoneItem(config, edge, zone, id, catalog) {
    const next = copy(config);
    if (!next.rails || !next.rails[edge] || !zones(edge).includes(zone)) return next;
    const entry = catalog.entry(id);
    const list = next.rails[edge][zone];
    if (!entry || !entry.axes.includes(axisOf(edge)) || !Array.isArray(list)) return next;
    if (railWidgets(next, edge).includes(id)) return next;
    next.rails[edge].enabled = true;
    list.push(id);
    return next;
}

// Move a widget within its zone (reorder). index and targetIndex are positions
// in the same zone; out-of-range indices are a clean no-op.
function reorderZoneItem(config, edge, zone, index, targetIndex) {
    const next = copy(config);
    if (!next.rails || !next.rails[edge] || !zones(edge).includes(zone)) return next;
    const list = next.rails[edge][zone];
    if (!Array.isArray(list) || !Number.isInteger(index) || index < 0 || index >= list.length) return next;
    const bounded = Math.max(0, Math.min(Number.isInteger(targetIndex) ? targetIndex : index, list.length - 1));
    const [item] = list.splice(index, 1);
    list.splice(bounded, 0, item);
    return next;
}

function removeZoneItem(config, edge, zone, index) {
    const next = copy(config);
    if (!next.rails || !next.rails[edge] || !zones(edge).includes(zone)) return next;
    const list = next.rails[edge][zone];
    if (Array.isArray(list) && Number.isInteger(index) && index >= 0 && index < list.length) list.splice(index, 1);
    return next;
}

function setRail(config, edge, changes) {
    const next = copy(config);
    const rail = next.rails && next.rails[edge];
    if (!rail || !changes || typeof changes !== "object") return next;
    if (typeof changes.enabled === "boolean") rail.enabled = changes.enabled;
    if (typeof changes.reveal === "boolean") rail.reveal = changes.reveal;
    if (Number.isFinite(changes.size)) rail.size = Math.round(changes.size);
    return next;
}

if (typeof module !== "undefined" && module.exports) module.exports = { zones, axisOf, railWidgets, addZoneItem, reorderZoneItem, removeZoneItem, setRail };

function clone(value) { return JSON.parse(JSON.stringify(value)); }
function isObject(value) { return value !== null && typeof value === "object" && !Array.isArray(value); }
function horizontal(edge) { return edge === "top" || edge === "bottom"; }
function zones(edge) { return horizontal(edge) ? ["start", "center", "end"] : ["top", "center", "bottom"]; }
function sizeFor(value, edge, fallback) {
    if (typeof value !== "number" || !isFinite(value)) return fallback;
    const min = horizontal(edge) ? 16 : 24;
    const max = horizontal(edge) ? 96 : 112;
    return Math.max(min, Math.min(max, Math.round(value)));
}
function idsFor(value, edge, catalog) {
    if (!Array.isArray(value)) return [];
    const axis = horizontal(edge) ? "horizontal" : "vertical";
    const seen = {};
    const ids = [];
    for (const id of value) {
        const item = catalog.entry(id);
        if (typeof id === "string" && item && item.axes.includes(axis) && !seen[id]) {
            seen[id] = true;
            ids.push(id);
        }
    }
    return ids;
}

function menuWidgetsFor(value, catalog, depth) {
    if (!Array.isArray(value) || depth > 3) return [];
    const out = [];
    for (const item of value) {
        const id = typeof item === "string" ? item : (isObject(item) ? item.id : "");
        const entry = catalog.widget(id);
        if (!entry) continue;
        if (entry.nested) {
            if (!isObject(item)) continue;
            out.push({ id: id, widgets: menuWidgetsFor(item.widgets, catalog, depth + 1) });
        } else if (typeof item === "string") {
            out.push(item);
        }
    }
    return out;
}


function defaultConfig(menuCatalog) {
    return {
        version: 1,
        style: "slate-frame",
        rails: {
            top: { enabled: false, size: 32, reveal: true, start: [], center: [], end: [] },
            left: { enabled: true, size: 48, reveal: true, top: ["quick-settings", "workspaces"], center: ["dock"], bottom: ["recording", "tray", "audio-input", "audio-output", "bluetooth", "network", "clock", "battery"] },
            bottom: { enabled: false, size: 32, reveal: true, start: [], center: [], end: [] },
            right: { enabled: false, size: 48, reveal: true, top: [], center: [], bottom: [] }
        },
        menus: {
            wallpaper: { anchor: "bottom", minWidth: 1400, expansion: "always", widgets: ["theme", "wallpaper"] },
            theme: { anchor: "right", minWidth: 320, expansion: "never", widgets: ["theme"] },
            weather: { anchor: "right", minWidth: 320, expansion: "never", widgets: ["weather"] },
        },
        surfaces: {},
        dock: { pinned: [] }
    };
}

// Side menus support the four reference vertical expansion modes; corner and
// edge menus ignore it. "never" is kept as an accepted legacy value (top-pinned,
// same as "always").
const menuExpansions = ["always", "never", "both", "up", "down"];

function normalize(raw, barCatalog, menuCatalog) {
    const base = defaultConfig(menuCatalog);
    const source = isObject(raw) ? raw : {};
    const output = defaultConfig(menuCatalog);
    output.style = source.style === "slate-frame" || source.style === "ryoku-frame" ? source.style : base.style;
    for (const edge of ["top", "left", "bottom", "right"]) {
        const rail = isObject(source.rails) && isObject(source.rails[edge]) ? source.rails[edge] : {};
        output.rails[edge].enabled = typeof rail.enabled === "boolean" ? rail.enabled : base.rails[edge].enabled;
        output.rails[edge].reveal = typeof rail.reveal === "boolean" ? rail.reveal : base.rails[edge].reveal;
        output.rails[edge].size = sizeFor(rail.size, edge, base.rails[edge].size);
        for (const zone of zones(edge)) output.rails[edge][zone] = Array.isArray(rail[zone]) ? idsFor(rail[zone], edge, barCatalog) : clone(base.rails[edge][zone]);
    }
    for (const id of Object.keys(base.menus)) {
        const value = isObject(source.menus) && isObject(source.menus[id]) ? source.menus[id] : {};
        const fallback = base.menus[id];
        output.menus[id] = {
            anchor: menuCatalog.anchors().includes(value.anchor) ? value.anchor : fallback.anchor,
            minWidth: typeof value.minWidth === "number" && isFinite(value.minWidth) ? Math.max(1, Math.round(value.minWidth)) : fallback.minWidth,
            expansion: menuExpansions.includes(value.expansion) ? value.expansion : fallback.expansion,
            widgets: Array.isArray(value.widgets) ? menuWidgetsFor(value.widgets, menuCatalog, 0) : clone(fallback.widgets)
        };
    }
    output.dock.pinned = isObject(source.dock) && Array.isArray(source.dock.pinned) ? source.dock.pinned.filter(id => typeof id === "string") : [];
    return output;
}

function addWidget(config, edge, zone, id, barCatalog) {
    const output = clone(config);
    if (!output.rails || !output.rails[edge] || !zones(edge).includes(zone)) return output;
    const item = barCatalog.entry(id);
    const axis = horizontal(edge) ? "horizontal" : "vertical";
    if (!item || !item.axes.includes(axis)) return output;
    if (output.rails[edge][zone].includes(id)) return output;
    output.rails[edge][zone].push(id);
    return output;
}

function moveWidget(config, fromEdge, fromZone, index, toEdge, toZone, targetIndex, barCatalog) {
    const output = clone(config);
    if (!output.rails || !output.rails[fromEdge] || !output.rails[toEdge] || !zones(fromEdge).includes(fromZone) || !zones(toEdge).includes(toZone)) return output;
    const source = output.rails[fromEdge][fromZone];
    if (!Array.isArray(source) || index < 0 || index >= source.length) return output;
    const id = source[index];
    const item = barCatalog.entry(id);
    const axis = horizontal(toEdge) ? "horizontal" : "vertical";
    if (!item || !item.axes.includes(axis)) return output;
    const target = output.rails[toEdge][toZone];
    if (target !== source && target.includes(id)) return output;
    source.splice(index, 1);
    const position = Math.max(0, Math.min(typeof targetIndex === "number" ? targetIndex : target.length, target.length));
    target.splice(position, 0, id);
    return output;
}

function removeWidget(config, edge, zone, index) {
    const output = clone(config);
    if (!output.rails || !output.rails[edge] || !zones(edge).includes(zone)) return output;
    const items = output.rails[edge][zone];
    if (Array.isArray(items) && index >= 0 && index < items.length) items.splice(index, 1);
    return output;
}

function setMenu(config, id, value, menuCatalog) {
    const output = clone(config);
    const fallback = defaultConfig(menuCatalog).menus[id];
    if (!fallback || !menuCatalog.menu(id)) return output;
    const source = isObject(value) ? value : {};
    output.menus[id] = {
        anchor: menuCatalog.anchors().includes(source.anchor) ? source.anchor : fallback.anchor,
        minWidth: typeof source.minWidth === "number" && isFinite(source.minWidth) ? Math.max(1, Math.round(source.minWidth)) : fallback.minWidth,
        expansion: menuExpansions.includes(source.expansion) ? source.expansion : fallback.expansion,
        widgets: Array.isArray(source.widgets) ? source.widgets.filter(widget => menuCatalog.widget(widget)) : clone(fallback.widgets)
    };
    return output;
}

function setSurface(config, id, value, menuCatalog) {
    const output = clone(config);
    const fallback = defaultConfig(menuCatalog).surfaces[id];
    if (!fallback || !menuCatalog.surface(id)) return output;
    const source = isObject(value) ? value : {};
    output.surfaces[id] = {
        anchor: menuCatalog.anchors().includes(source.anchor) ? source.anchor : fallback.anchor,
        minWidth: typeof source.minWidth === "number" && isFinite(source.minWidth) ? Math.max(1, Math.round(source.minWidth)) : fallback.minWidth,
        panes: Array.isArray(source.panes) ? source.panes.filter(pane => typeof pane === "string" && fallback.panes.includes(pane)) : clone(fallback.panes)
    };
    return output;
}

if (typeof module !== "undefined" && module.exports) module.exports = { defaultConfig, normalize, addWidget, moveWidget, removeWidget, setMenu, setSurface };

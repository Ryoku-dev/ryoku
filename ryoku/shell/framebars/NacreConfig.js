const islands = ["left", "center", "right"];
const workspaceStyles = ["dots", "numbers", "kanji"];
const brandClicks = ["launcher", "quicksettings"];
const catalog = [
    { id: "brand", label: "Brand", file: "Brand.qml" },
    { id: "media", label: "Media", file: "Media.qml" },
    { id: "activeWindow", label: "Active window", file: "ActiveWindow.qml" },
    { id: "clock", label: "Clock", file: "Clock.qml" },
    { id: "workspaces", label: "Workspaces", file: "Workspaces.qml" },
    { id: "resources", label: "Resources", file: "Resources.qml" },
    { id: "connectivity", label: "Connections", file: "Connectivity.qml" },
    { id: "audio", label: "Audio", file: "Audio.qml" },
    { id: "battery", label: "Battery", file: "Battery.qml" },
    { id: "notifications", label: "Notifications", file: "Notifications.qml" },
    { id: "tray", label: "Tray", file: "Tray.qml" },
    { id: "weather", label: "Weather", file: "Weather.qml" },
    { id: "utils", label: "Recording", file: "Utils.qml" }
];
const widgets = catalog.map(item => item.id);
const ranges = {
    height: [32, 56],
    opacity: [0.45, 1],
    padding: [6, 24],
    spacing: [2, 18],
    islandGap: [6, 32],
    frameSize: [2, 24],
    frameRoundness: [0, 32],
    edgeMelt: [1, 32],
    islandScale: [0.65, 1.25],
    osdScale: [0.65, 1.25]
};

function clone(value) {
    return JSON.parse(JSON.stringify(value));
}

function object(value) {
    return value !== null && typeof value === "object" && !Array.isArray(value);
}

function defaultConfig() {
    return {
        islands: {
            left: ["brand", "media", "activeWindow"],
            center: ["clock", "workspaces", "resources"],
            right: ["connectivity", "audio", "battery", "notifications", "tray"]
        },
        height: 40,
        opacity: 0.82,
        padding: 12,
        spacing: 8,
        islandGap: 14,
        frameSize: 9,
        frameRoundness: 9,
        edgeMelt: 8,
        islandScale: 1,
        osdScale: 1,
        frame: true,
        occupiedWorkspaces: true,
        workspaceStyle: "dots",
        brandClick: "launcher"
    };
}

function number(value, key, fallback) {
    if (typeof value !== "number" || !isFinite(value))
        return fallback;
    const range = ranges[key];
    const clamped = Math.max(range[0], Math.min(range[1], value));
    return ["opacity", "islandScale", "osdScale"].includes(key)
        ? Math.round(clamped * 100) / 100 : Math.round(clamped);
}

function normalize(raw) {
    const source = object(raw) ? raw : {};
    const base = defaultConfig();
    const output = defaultConfig();
    const seen = {};

    for (const island of islands) {
        const supplied = object(source.islands) && Array.isArray(source.islands[island]);
        const values = supplied ? source.islands[island] : base.islands[island];
        output.islands[island] = [];
        for (const id of values) {
            if (typeof id === "string" && widgets.includes(id) && !seen[id]) {
                seen[id] = true;
                output.islands[island].push(id);
            }
        }
    }

    for (const key of Object.keys(ranges))
        output[key] = number(source[key], key, base[key]);
    output.frame = typeof source.frame === "boolean" ? source.frame : base.frame;
    output.occupiedWorkspaces = typeof source.occupiedWorkspaces === "boolean"
        ? source.occupiedWorkspaces : base.occupiedWorkspaces;
    output.workspaceStyle = workspaceStyles.includes(source.workspaceStyle)
        ? source.workspaceStyle : base.workspaceStyle;
    output.brandClick = brandClicks.includes(source.brandClick)
        ? source.brandClick : base.brandClick;
    return output;
}

function locate(config, widgetId) {
    for (const island of islands) {
        const index = config.islands[island].indexOf(widgetId);
        if (index >= 0)
            return { island, index };
    }
    return null;
}

function move(config, widgetId, sourceIsland, targetIsland, targetIndex) {
    const output = normalize(config);
    if (!widgets.includes(widgetId) || !islands.includes(targetIsland))
        return output;

    const current = locate(output, widgetId);
    if ((sourceIsland === "" && current) || (sourceIsland !== "" && (!current || current.island !== sourceIsland)))
        return output;

    let position = typeof targetIndex === "number" && isFinite(targetIndex)
        ? Math.round(targetIndex) : output.islands[targetIsland].length;
    if (current) {
        output.islands[current.island].splice(current.index, 1);
        if (current.island === targetIsland && current.index < position)
            position--;
    }
    const target = output.islands[targetIsland];
    target.splice(Math.max(0, Math.min(position, target.length)), 0, widgetId);
    return output;
}

function remove(config, widgetId) {
    const output = normalize(config);
    const current = locate(output, widgetId);
    if (current)
        output.islands[current.island].splice(current.index, 1);
    return output;
}

function setValue(config, key, value) {
    const output = normalize(config);
    if (ranges[key])
        output[key] = value;
    else if (key === "occupiedWorkspaces" || key === "frame" || key === "workspaceStyle"
             || key === "brandClick")
        output[key] = value;
    else
        return output;
    return normalize(output);
}

function widgetIds() {
    return widgets.slice();
}

function entry(id) {
    for (const item of catalog)
        if (item.id === id)
            return clone(item);
    return null;
}

function unused(config) {
    const output = normalize(config);
    return widgets.filter(id => locate(output, id) === null);
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { defaultConfig, normalize, move, remove, setValue, widgetIds, entry, unused };

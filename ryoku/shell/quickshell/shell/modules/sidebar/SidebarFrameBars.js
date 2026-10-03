.pragma library

const leftIds = ["system", "notifications", "weather", "media", "capture", "stage"];
const rightIds = ["usage", "tools", "chat"];
const builtInIds = leftIds.concat(rightIds);

function defaultConfig() {
    return {
        width: 380,
        motion: "standard",
        depth: true,
        push: true,
        wallpaperSlide: 1.15,
        left: { enabled: true, cards: leftIds.slice() },
        right: { enabled: true, cards: rightIds.slice() }
    };
}

function finiteNumber(value, fallback) {
    var number = Number(value);
    return isFinite(number) ? number : fallback;
}

function clamp(value, low, high) {
    return Math.max(low, Math.min(high, value));
}

function cardList(value, side, fallback) {
    if (!Array.isArray(value))
        return fallback.slice();
    var allowed = side === "left" ? leftIds : rightIds;
    var result = [];
    for (var i = 0; i < value.length; ++i) {
        var id = typeof value[i] === "string" ? value[i] : "";
        var isPlugin = builtInIds.indexOf(id) < 0 && /^[a-z0-9][a-z0-9-]*$/.test(id);
        if ((allowed.indexOf(id) < 0 && !isPlugin) || result.indexOf(id) >= 0)
            continue;
        result.push(id);
    }
    return result;
}

function sideConfig(raw, side, fallback) {
    var value = raw && typeof raw === "object" ? raw : {};
    return {
        enabled: typeof value.enabled === "boolean" ? value.enabled : fallback.enabled,
        cards: cardList(value.cards, side, fallback.cards)
    };
}

function normalize(raw) {
    var defaults = defaultConfig();
    var value = raw && typeof raw === "object" ? raw : {};
    var motion = ["quick", "standard", "calm"].indexOf(value.motion) >= 0
        ? value.motion : defaults.motion;
    return {
        width: Math.round(clamp(finiteNumber(value.width, defaults.width), 280, 560)),
        motion: motion,
        depth: typeof value.depth === "boolean" ? value.depth : defaults.depth,
        push: typeof value.push === "boolean" ? value.push : defaults.push,
        wallpaperSlide: clamp(finiteNumber(value.wallpaperSlide, defaults.wallpaperSlide), 1.0, 1.4),
        left: sideConfig(value.left, "left", defaults.left),
        right: sideConfig(value.right, "right", defaults.right)
    };
}

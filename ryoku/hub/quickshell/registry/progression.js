.pragma library

// Local-only Ryoku achievements, collection and profile canvas.
// Entitlements here are personal customisation, never trusted online currency.
var achievements = [
    { id: "ryoku-first-steps", category: "Ryoku", title: "First Steps", detail: "Open your achievement collection", reward: 30, glyph: "flag" },
    { id: "ryoku-pathfinder", category: "Ryoku", title: "Pathfinder", detail: "Visit five different Ryoku settings pages", reward: 80, glyph: "explore" },
    { id: "ryoku-voyager", category: "Ryoku", title: "Voyager", detail: "Visit fifteen different Ryoku settings pages", reward: 150, glyph: "public" },
    { id: "ryoku-versatile", category: "Ryoku", title: "Versatile", detail: "Explore four different Ryoku settings areas", reward: 100, glyph: "auto_awesome" },
    { id: "profile-curator", category: "Profile", title: "Curator", detail: "Feature an earned badge on your profile", reward: 70, glyph: "workspace_premium" },
    { id: "profile-sticker", category: "Profile", title: "Sticker Artist", detail: "Place a collectible sticker on your profile", reward: 90, glyph: "draw" },
    { id: "profile-background", category: "Profile", title: "New Canvas", detail: "Equip a collectible profile backdrop", reward: 80, glyph: "texture" },
    { id: "profile-type", category: "Profile", title: "Typographer", detail: "Change the typography of your profile identity", reward: 80, glyph: "text_fields" },
    { id: "welcome", category: "Card studio", title: "First Contact", detail: "Open the card studio", reward: 50, glyph: "style" },
    { id: "explorer", category: "Card studio", title: "Under the Hood", detail: "Inspect your hardware card's rarity", reward: 80, glyph: "memory" },
    { id: "identity", category: "Card studio", title: "Signature", detail: "Personalise your card's words", reward: 100, glyph: "edit_note" },
    { id: "portrait", category: "Card studio", title: "A New Face", detail: "Choose your own card artwork", reward: 100, glyph: "portrait" },
    { id: "layout", category: "Card studio", title: "Art Director", detail: "Customise your card layout", reward: 120, glyph: "view_quilt" },
    { id: "showcase", category: "Card studio", title: "Showtime", detail: "Successfully copy or save your card", reward: 150, glyph: "present_to_all" }
];

var groups = [
    { id: "Ryoku", title: "Ryoku", subtitle: "Explore the world you call home" },
    { id: "Profile", title: "Profile", subtitle: "Make your identity your own" },
    { id: "Card studio", title: "Card Studio", subtitle: "Craft the machine that represents you" }
];

var cosmetics = [
    { id: "midnight", name: "Midnight", type: "palette", price: 150,
      palette: { ink: "#f2f1fc", plate: "#141426", accent: "#a4a3ef", rim: "#7776c4" } },
    { id: "ember", name: "Ember", type: "palette", price: 200,
      palette: { ink: "#ffe6d7", plate: "#281813", accent: "#ffad72", rim: "#d77c58" } },
    { id: "jade", name: "Jade", type: "palette", price: 250,
      palette: { ink: "#e0fff1", plate: "#10251e", accent: "#7de0b8", rim: "#49b58c" } },
    { id: "sticker-orbit", name: "Orbit Seal", type: "sticker", price: 65, glyph: "planet", tint: "#b9c9fa" },
    { id: "sticker-flame", name: "Little Flame", type: "sticker", price: 75, glyph: "local_fire_department", tint: "#ffac83" },
    { id: "sticker-bloom", name: "Night Bloom", type: "sticker", price: 60, glyph: "filter_vintage", tint: "#d9b0ef" },
    { id: "sticker-star", name: "Starfall", type: "sticker", price: 45, glyph: "star", tint: "#f6d18b" },
    { id: "bg-graphite", name: "Graphite Grid", type: "background", price: 60 },
    { id: "bg-blueprint", name: "Blueprint", type: "background", price: 70 },
    { id: "bg-ember", name: "Afterglow", type: "background", price: 80 },
    { id: "font-serif", name: "Engraved", type: "font", price: 50, family: "Liberation Serif" },
    { id: "font-mono", name: "Terminal", type: "font", price: 60, family: "JetBrainsMono Nerd Font" },
    { id: "font-clean", name: "Technical", type: "font", price: 65, family: "Inter Display" }
];

function achievement(id) { return achievements.find(function(a) { return a.id === id; }); }
function cosmetic(id) { return cosmetics.find(function(a) { return a.id === id; }); }
function stamp(id) {
    var a = achievement(id);
    if (a)
        return { id: id, name: a.title, glyph: a.glyph, tint: "#e9c78c", type: "badge" };
    var c = cosmetic(id);
    return c && c.type === "sticker" ? c : null;
}
function clamp(n, min, max, fallback) {
    n = Number(n);
    return isFinite(n) ? Math.min(max, Math.max(min, n)) : fallback;
}
function validId(id) { return typeof id === "string" && /^[a-z][a-z0-9-]{0,49}$/.test(id); }

function clean(value) {
    var source = value && typeof value === "object" ? value : {};
    var earned = {}, claimed = {}, owned = {}, visited = {};
    var old = !(Number(source.version) >= 3);
    achievements.forEach(function(a) {
        if (source.earned && source.earned[a.id] === true)
            earned[a.id] = true;
        if (earned[a.id] && (old || (source.claimed && source.claimed[a.id] === true)))
            claimed[a.id] = true;
    });
    cosmetics.forEach(function(c) {
        if (source.owned && source.owned[c.id] === true)
            owned[c.id] = true;
    });
    if (source.visited && typeof source.visited === "object")
        Object.keys(source.visited).slice(0, 150).forEach(function(key) {
            if (validId(key) && validId(source.visited[key]))
                visited[key] = source.visited[key];
        });
    var balance = 0;
    achievements.forEach(function(a) { if (claimed[a.id]) balance += a.reward; });
    cosmetics.forEach(function(c) { if (owned[c.id]) balance -= c.price; });

    var active = owned[source.active] && cosmetic(source.active).type === "palette" ? source.active : "";
    var featured = claimed[source.featured] ? source.featured : "";
    var rawCanvas = source.canvas && typeof source.canvas === "object" ? source.canvas : {};
    var background = owned[rawCanvas.background] && cosmetic(rawCanvas.background).type === "background"
        ? rawCanvas.background : "";
    var rawFonts = rawCanvas.fonts && typeof rawCanvas.fonts === "object" ? rawCanvas.fonts : {};
    var fonts = {};
    ["name", "hostname", "tagline"].forEach(function(role) {
        var id = rawFonts[role];
        fonts[role] = owned[id] && cosmetic(id).type === "font" ? id : "";
    });
    var stickers = [];
    var used = {};
    if (Array.isArray(rawCanvas.stickers)) {
        rawCanvas.stickers.slice(0, 24).forEach(function(s) {
            if (!s || !validId(s.item) || !stamp(s.item) || used[s.slot])
                return;
            var a = achievement(s.item), c = cosmetic(s.item);
            if (a ? !claimed[a.id] : !owned[c.id])
                return;
            var slot = String(s.slot || "");
            if (!/^st[0-9]{1,8}$/.test(slot))
                return;
            used[slot] = true;
            stickers.push({ slot: slot, item: s.item, x: clamp(s.x, 0, 0.94, 0.5),
                            y: clamp(s.y, 0, 0.94, 0.5), size: clamp(s.size, 48, 150, 82),
                            rotation: clamp(s.rotation, -45, 45, 0) });
        });
    }
    var image = typeof rawCanvas.image === "string" &&
        /^file:\/\/\/[^?#]{1,1024}\.(png|jpe?g|webp|gif|bmp)$/i.test(rawCanvas.image)
        ? rawCanvas.image : "";
    return {
        version: 3, earned: earned, claimed: claimed, owned: owned, visited: visited,
        featured: featured, active: active, balance: Math.max(0, balance),
        canvas: { background: background, image: image, fonts: fonts, stickers: stickers,
                  serial: Math.floor(clamp(rawCanvas.serial, 0, 99999999, 0)) }
    };
}

function unlock(state, id) {
    var next = clean(state);
    if (!achievement(id) || next.earned[id])
        return { state: next, changed: false };
    next.earned[id] = true;
    return { state: clean(next), changed: true };
}

function claim(state, id) {
    var next = clean(state);
    if (!next.earned[id] || next.claimed[id])
        return { state: next, changed: false };
    next.claimed[id] = true;
    return { state: clean(next), changed: true };
}

function purchase(state, id) {
    var next = clean(state), item = cosmetic(id);
    if (!item || next.owned[id] || next.balance < item.price)
        return { state: next, changed: false };
    next.owned[id] = true;
    return { state: clean(next), changed: true };
}

function equip(state, id) {
    var next = clean(state);
    if (id !== "" && (!next.owned[id] || cosmetic(id).type !== "palette"))
        return { state: next, changed: false };
    if (next.active === id)
        return { state: next, changed: false };
    next.active = id;
    return { state: clean(next), changed: true };
}

function visit(state, page, category) {
    var next = clean(state);
    if (!validId(page) || !validId(category) || next.visited[page])
        return { state: next, changed: false };
    next.visited[page] = category;
    var count = Object.keys(next.visited).length, areas = {};
    Object.keys(next.visited).forEach(function(k) { areas[next.visited[k]] = true; });
    if (count >= 5) next.earned["ryoku-pathfinder"] = true;
    if (count >= 15) next.earned["ryoku-voyager"] = true;
    if (Object.keys(areas).length >= 4) next.earned["ryoku-versatile"] = true;
    return { state: clean(next), changed: true };
}

function feature(state, id) {
    var next = clean(state);
    if (id !== "" && !next.claimed[id])
        return { state: next, changed: false };
    if (next.featured === id)
        return { state: next, changed: false };
    next.featured = id;
    if (id) next.earned["profile-curator"] = true;
    return { state: clean(next), changed: true };
}

function background(state, id) {
    var next = clean(state);
    if (id !== "" && (!next.owned[id] || cosmetic(id).type !== "background"))
        return { state: next, changed: false };
    if (next.canvas.background === id)
        return { state: next, changed: false };
    next.canvas.background = id;
    next.canvas.image = "";
    if (id) next.earned["profile-background"] = true;
    return { state: clean(next), changed: true };
}

function customImage(state, url) {
    var next = clean(state);
    if (url !== "" && (typeof url !== "string" ||
        !/^file:\/\/\/[^?#]{1,1024}\.(png|jpe?g|webp|gif|bmp)$/i.test(url)))
        return { state: next, changed: false };
    if (next.canvas.image === url && (url === "" || next.canvas.background === ""))
        return { state: next, changed: false };
    next.canvas.image = url;
    next.canvas.background = "";
    if (url) next.earned["profile-background"] = true;
    return { state: clean(next), changed: true };
}

function font(state, role, id) {
    var next = clean(state);
    if (["name", "hostname", "tagline"].indexOf(role) < 0 ||
        (id !== "" && (!next.owned[id] || cosmetic(id).type !== "font")))
        return { state: next, changed: false };
    if (next.canvas.fonts[role] === id)
        return { state: next, changed: false };
    next.canvas.fonts[role] = id;
    if (id) next.earned["profile-type"] = true;
    return { state: clean(next), changed: true };
}

function addSticker(state, id) {
    var next = clean(state);
    if (next.canvas.stickers.length >= 24 || !stamp(id))
        return { state: next, changed: false };
    var a = achievement(id), c = cosmetic(id);
    if (a ? !next.claimed[a.id] : !next.owned[c.id])
        return { state: next, changed: false };
    var serial = next.canvas.serial;
    do { serial++; } while (next.canvas.stickers.some(function(s) { return s.slot === "st" + serial; }));
    if (serial > 99999999)
        return { state: next, changed: false };
    next.canvas.serial = serial;
    next.canvas.stickers.push({ slot: "st" + serial, item: id, x: 0.56, y: 0.65, size: 82, rotation: 0 });
    next.earned["profile-sticker"] = true;
    return { state: clean(next), changed: true };
}

function moveSticker(state, slot, x, y, size, rotation) {
    var next = clean(state);
    var s = next.canvas.stickers.find(function(item) { return item.slot === slot; });
    if (!s)
        return { state: next, changed: false };
    var nx = clamp(x, 0, 0.94, s.x), ny = clamp(y, 0, 0.94, s.y);
    var ns = clamp(size, 48, 150, s.size), nr = clamp(rotation, -45, 45, s.rotation);
    if (s.x === nx && s.y === ny && s.size === ns && s.rotation === nr)
        return { state: next, changed: false };
    s.x = nx; s.y = ny; s.size = ns; s.rotation = nr;
    return { state: clean(next), changed: true };
}

function removeSticker(state, slot) {
    var next = clean(state);
    var count = next.canvas.stickers.length;
    next.canvas.stickers = next.canvas.stickers.filter(function(s) { return s.slot !== slot; });
    return { state: clean(next), changed: count !== next.canvas.stickers.length };
}

function palette(state) {
    var next = clean(state);
    var item = cosmetic(next.active);
    return item && item.type === "palette" ? item.palette : null;
}

function availableStickers(state) {
    var next = clean(state), result = [];
    achievements.forEach(function(a) { if (next.claimed[a.id]) result.push(stamp(a.id)); });
    cosmetics.forEach(function(c) { if (c.type === "sticker" && next.owned[c.id]) result.push(c); });
    return result;
}

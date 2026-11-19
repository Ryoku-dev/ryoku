.pragma library

// Local-only Ryoku achievements, collection and profile canvas.
// Entitlements here are personal customisation, never trusted online currency.
var achievements = [
    { id: "ryoku-first-steps", category: "Ryoku", title: "First Steps", detail: "Open your achievement collection", reward: 30, glyph: "flag", rarity: "common", sigil: "I" },
    { id: "ryoku-pathfinder", category: "Ryoku", title: "Pathfinder", detail: "Visit five different Ryoku settings pages", reward: 80, glyph: "explore", rarity: "uncommon", sigil: "V" },
    { id: "ryoku-voyager", category: "Ryoku", title: "Voyager", detail: "Visit fifteen different Ryoku settings pages", reward: 150, glyph: "public", rarity: "rare", sigil: "XV" },
    { id: "ryoku-versatile", category: "Ryoku", title: "Versatile", detail: "Explore four different Ryoku settings areas", reward: 100, glyph: "auto_awesome", rarity: "rare", sigil: "IV" },
    { id: "profile-curator", category: "Profile", title: "Curator", detail: "Feature an earned badge on your profile", reward: 70, glyph: "workspace_premium", rarity: "common", sigil: "C" },
    { id: "profile-sticker", category: "Profile", title: "Sticker Artist", detail: "Place a collectible sticker on your profile", reward: 90, glyph: "draw", rarity: "common", sigil: "S" },
    { id: "profile-background", category: "Profile", title: "New Canvas", detail: "Equip a collectible profile backdrop", reward: 80, glyph: "texture", rarity: "uncommon", sigil: "B" },
    { id: "profile-type", category: "Profile", title: "Typographer", detail: "Change the typography of your profile identity", reward: 80, glyph: "text_fields", rarity: "uncommon", sigil: "T" },
    { id: "welcome", category: "Card studio", title: "First Contact", detail: "Open the card studio", reward: 50, glyph: "style", rarity: "common", sigil: "I" },
    { id: "explorer", category: "Card studio", title: "Under the Hood", detail: "Inspect your hardware card's rarity", reward: 80, glyph: "memory", rarity: "uncommon", sigil: "II" },
    { id: "identity", category: "Card studio", title: "Signature", detail: "Personalise your card's words", reward: 100, glyph: "edit_note", rarity: "uncommon", sigil: "III" },
    { id: "portrait", category: "Card studio", title: "A New Face", detail: "Choose your own card artwork", reward: 100, glyph: "portrait", rarity: "rare", sigil: "IV" },
    { id: "layout", category: "Card studio", title: "Art Director", detail: "Customise your card layout", reward: 120, glyph: "view_quilt", rarity: "rare", sigil: "V" },
    { id: "showcase", category: "Card studio", title: "Showtime", detail: "Successfully copy or save your card", reward: 150, glyph: "present_to_all", rarity: "epic", sigil: "VI" },
    { id: "ryoku-tenfold", category: "Ryoku", title: "Tenfold", detail: "Visit ten different settings pages", reward: 90, glyph: "route", rarity: "uncommon", sigil: "X" },
    { id: "ryoku-cartographer", category: "Ryoku", title: "Cartographer", detail: "Map twenty-five settings pages", reward: 170, glyph: "map", rarity: "epic", sigil: "XXV" },
    { id: "ryoku-omniscope", category: "Ryoku", title: "Omniscope", detail: "Explore thirty different settings pages", reward: 190, glyph: "all_inclusive", rarity: "legendary", sigil: "XXX" },
    { id: "profile-mosaic", category: "Profile", title: "Mosaic", detail: "Display five stickers at once", reward: 140, glyph: "dashboard_customize", rarity: "rare", sigil: "V" },
    { id: "profile-gallery", category: "Profile", title: "Gallery", detail: "Display ten stickers at once", reward: 160, glyph: "collections", rarity: "epic", sigil: "X" },
    { id: "profile-sanctuary", category: "Profile", title: "The Sanctuary", detail: "Fill twenty-four sticker placements", reward: 210, glyph: "temple_buddhist", rarity: "mythic", sigil: "XXIV" },
    { id: "profile-self", category: "Profile", title: "Own Reflection", detail: "Use a personal background image", reward: 100, glyph: "image", rarity: "rare", sigil: "OWN" },
    { id: "profile-identity", category: "Profile", title: "Type Ritual", detail: "Style your name hostname and tagline", reward: 130, glyph: "format_size", rarity: "epic", sigil: "III" },
    { id: "profile-custom", category: "Profile", title: "Signature Plate", detail: "Equip a backdrop font and featured badge", reward: 180, glyph: "fingerprint", rarity: "legendary", sigil: "力" },
    { id: "card-spectrum", category: "Card studio", title: "Chromatic", detail: "Equip a collectible card palette", reward: 100, glyph: "palette", rarity: "rare", sigil: "VII" },
    { id: "card-complete", category: "Card studio", title: "Full Deck", detail: "Complete every original card milestone", reward: 140, glyph: "playing_cards", rarity: "legendary", sigil: "VIII" },
    { id: "card-tapestry", category: "Card studio", title: "Card Archivist", detail: "Claim all six original card seals", reward: 170, glyph: "collections_bookmark", rarity: "mythic", sigil: "IX" },
    { id: "collection-firstclaim", category: "Collection", title: "Seal Keeper", detail: "Claim your first achievement badge", reward: 45, glyph: "verified", rarity: "common", sigil: "I" },
    { id: "collection-fifth", category: "Collection", title: "Fivefold Archive", detail: "Claim five achievement badges", reward: 100, glyph: "shield", rarity: "uncommon", sigil: "V" },
    { id: "collection-tenth", category: "Collection", title: "Ten Seals", detail: "Claim ten achievement badges", reward: 140, glyph: "token", rarity: "rare", sigil: "X" },
    { id: "collection-twentieth", category: "Collection", title: "Twenty Seals", detail: "Claim twenty achievement badges", reward: 180, glyph: "album", rarity: "epic", sigil: "XX" },
    { id: "collection-firstbuy", category: "Collection", title: "First Relic", detail: "Buy one cosmetic", reward: 45, glyph: "local_mall", rarity: "common", sigil: "I" },
    { id: "collection-trio", category: "Collection", title: "Relic Collector", detail: "Own three cosmetics", reward: 90, glyph: "inventory_2", rarity: "uncommon", sigil: "III" },
    { id: "collection-stickers", category: "Collection", title: "Wax and Ink", detail: "Collect all decorative shop stickers", reward: 130, glyph: "auto_awesome_mosaic", rarity: "epic", sigil: "S" },
    { id: "collection-backgrounds", category: "Collection", title: "World Painter", detail: "Collect every shop backdrop", reward: 130, glyph: "landscape", rarity: "epic", sigil: "B" },
    { id: "collection-fonts", category: "Collection", title: "The Letterpress", detail: "Collect every shop font", reward: 130, glyph: "article", rarity: "epic", sigil: "T" },
    { id: "collection-everything", category: "Collection", title: "Complete Collection", detail: "Own every shop cosmetic", reward: 260, glyph: "diamond", rarity: "mythic", sigil: "∞" },
    { id: "master-archive", category: "Mastery", title: "Living Archive", detail: "Complete every unclassified achievement", reward: 230, glyph: "account_balance", rarity: "mythic", sigil: "力" },
    { id: "secret-signal", category: "Classified", title: "Signal in the Stone", detail: "Follow the archive hidden signal", reward: 210, glyph: "sensors", rarity: "holographic", sigil: "?", secret: true },
    { id: "secret-sevenfold", category: "Classified", title: "Sevenfold", detail: "Display seven unique earned badge seals", reward: 220, glyph: "deployed_code", rarity: "holographic", sigil: "VII", secret: true },
    { id: "secret-monolith", category: "Classified", title: "Monolith Protocol", detail: "Unite graphite field terminal name and legendary seal", reward: 250, glyph: "hexagon", rarity: "holographic", sigil: "NULL", secret: true },
    { id: "secret-totality", category: "Classified", title: "The Final Seal", detail: "Complete every other achievement in Ryoku", reward: 420, glyph: "all_inclusive", rarity: "holographic", sigil: "力", secret: true }
];

var groups = [
    { id: "Ryoku", title: "Ryoku", subtitle: "Explore the world you call home" },
    { id: "Profile", title: "Profile", subtitle: "Make your identity your own" },
    { id: "Card studio", title: "Card Studio", subtitle: "Craft the machine that represents you" },
    { id: "Collection", title: "Collection", subtitle: "Acquire, classify, preserve" },
    { id: "Mastery", title: "Mastery", subtitle: "For those who complete the archive" },
    { id: "Classified", title: "Classified", subtitle: "Nothing here is given away" }
];

var cosmetics = [
    { id: "midnight", name: "Midnight", type: "palette", price: 150,
      palette: { ink: "#f2f1fc", plate: "#141426", accent: "#a4a3ef", rim: "#7776c4" } },
    { id: "ember", name: "Ember", type: "palette", price: 200,
      palette: { ink: "#ffe6d7", plate: "#281813", accent: "#ffad72", rim: "#d77c58" } },
    { id: "jade", name: "Jade", type: "palette", price: 250,
      palette: { ink: "#e0fff1", plate: "#10251e", accent: "#7de0b8", rim: "#49b58c" } },
    { id: "sticker-orbit", name: "Orbit Seal", type: "sticker", price: 65, art: "orbit", glyph: "planet", tint: "#b9c9fa" },
    { id: "sticker-flame", name: "Little Flame", type: "sticker", price: 75, art: "flame", glyph: "local_fire_department", tint: "#ffac83" },
    { id: "sticker-bloom", name: "Night Bloom", type: "sticker", price: 60, art: "bloom", glyph: "filter_vintage", tint: "#d9b0ef" },
    { id: "sticker-star", name: "Starfall", type: "sticker", price: 45, art: "star", glyph: "star", tint: "#f6d18b" },
    { id: "bg-graphite", name: "Graphite Grid", type: "background", price: 60 },
    { id: "bg-blueprint", name: "Blueprint", type: "background", price: 70 },
    { id: "bg-ember", name: "Afterglow", type: "background", price: 80 },
    { id: "font-serif", name: "Engraved", type: "font", price: 50, family: "Liberation Serif" },
    { id: "font-mono", name: "Terminal", type: "font", price: 60, family: "JetBrainsMono Nerd Font" },
    { id: "font-clean", name: "Technical", type: "font", price: 65, family: "Inter Display" },
    { id: "sticker-origins", name: "Origin Mark", type: "sticker", price: 140, art: "origins", glyph: "token", tint: "#ef8d69", rarity: "legendary", sigil: "力" },
    { id: "sticker-sutra", name: "Sutra Fragment", type: "sticker", price: 105, art: "sutra", glyph: "deployed_code", tint: "#b3b7c5", rarity: "epic", sigil: "道" },
    { id: "sticker-ember", name: "Ember Crest", type: "sticker", price: 140, art: "ember", glyph: "whatshot", tint: "#e97452", rarity: "mythic", sigil: "焔" },
    { id: "sticker-archive", name: "Archive Clearance", type: "sticker", price: 120, art: "archive", glyph: "verified_user", tint: "#b8ab90", rarity: "rare", sigil: "III" },
    { id: "sticker-moonseal", name: "Moon Archive", type: "sticker", price: 115, art: "moonseal", glyph: "brightness_2", tint: "#b3bbdd", rarity: "rare", sigil: "月" },
    { id: "sticker-fossil", name: "Stone Witness", type: "sticker", price: 120, art: "fossil", glyph: "museum", tint: "#d8c8b0", rarity: "epic", sigil: "石" },
    { id: "bg-archive", name: "Archive Paper", type: "background", price: 175 },
    { id: "bg-obsidian", name: "Obsidian Mesh", type: "background", price: 180 },
    { id: "bg-sutra", name: "Sutra Diagram", type: "background", price: 195 },
    { id: "bg-signal", name: "Signal Void", type: "background", price: 200 },
    { id: "font-ritual", name: "Ritual Sans", type: "font", price: 130, family: "Inter Display" },
    { id: "font-archive", name: "Archive Mono", type: "font", price: 135, family: "FiraCode Nerd Font" },
    { id: "font-stone", name: "Stone Serif", type: "font", price: 150, family: "Noto Serif" }
];

function achievement(id) { return achievements.find(function(a) { return a.id === id; }); }
function cosmetic(id) { return cosmetics.find(function(a) { return a.id === id; }); }
function stamp(id) {
    var a = achievement(id);
    if (a)
        return { id: id, name: a.title, glyph: a.glyph, sigil: a.sigil, rarity: a.rarity, tint: rarityColour(a.rarity), type: "badge" };
    var c = cosmetic(id);
    return c && c.type === "sticker" ? Object.assign({ rarity: c.rarity || "common", sigil: c.sigil || "力" }, c) : null;
}
var rarityColours = {
    common: "#9499a6",
    uncommon: "#86bca6",
    rare: "#65b9e1",
    epic: "#c38be6",
    legendary: "#f1bb6e",
    mythic: "#f17854",
    holographic: "#c5a8f6"
};
function rarityColour(name) { return rarityColours[name] || rarityColours.common; }
function visibleAchievements(state, category) {
    var next = clean(state);
    return achievements.filter(function(a) {
        return (!category || a.category === category) && (!a.secret || next.earned[a.id]);
    });
}
function secretCount(state) {
    var next = clean(state);
    return achievements.filter(function(a) { return a.secret && next.earned[a.id]; }).length;
}
function entitled(state, id) {
    var next = clean(state);
    return next.claimed[id] === true || next.owned[id] === true;
}
function sealProgress(state, id) {
    var next = clean(state);
    var a = achievement(id);
    if (!a) return "locked";
    return next.claimed[id] ? "claimed" : next.earned[id] ? "claimable" : "locked";
}
function evaluate(next) {
    var count = Object.keys(next.visited).length;
    if (count >= 10) next.earned["ryoku-tenfold"] = true;
    if (count >= 25) next.earned["ryoku-cartographer"] = true;
    if (count >= 30) next.earned["ryoku-omniscope"] = true;

    var placed = next.canvas.stickers;
    if (placed.length >= 5) next.earned["profile-mosaic"] = true;
    if (placed.length >= 10) next.earned["profile-gallery"] = true;
    if (placed.length >= 24) next.earned["profile-sanctuary"] = true;
    if (next.canvas.image) next.earned["profile-self"] = true;
    if (["name","hostname","tagline"].every(function(role) { return !!next.canvas.fonts[role]; }))
        next.earned["profile-identity"] = true;
    if ((next.canvas.background || next.canvas.image) &&
        Object.keys(next.canvas.fonts).some(function(role) { return !!next.canvas.fonts[role]; }) &&
        next.featured) next.earned["profile-custom"] = true;

    var original = ["welcome","explorer","identity","portrait","layout","showcase"];
    if (original.every(function(id) { return next.earned[id]; })) next.earned["card-complete"] = true;
    if (original.every(function(id) { return next.claimed[id]; })) next.earned["card-tapestry"] = true;
    if (next.active) next.earned["card-spectrum"] = true;

    var claimed = Object.keys(next.claimed).length;
    if (claimed >= 1) next.earned["collection-firstclaim"] = true;
    if (claimed >= 5) next.earned["collection-fifth"] = true;
    if (claimed >= 10) next.earned["collection-tenth"] = true;
    if (claimed >= 20) next.earned["collection-twentieth"] = true;

    var owned = Object.keys(next.owned).length;
    if (owned >= 1) next.earned["collection-firstbuy"] = true;
    if (owned >= 3) next.earned["collection-trio"] = true;
    ["sticker","background","font"].forEach(function(type) {
        var relevant = cosmetics.filter(function(c) { return c.type === type; });
        var goals = { sticker:"collection-stickers", background:"collection-backgrounds", font:"collection-fonts" };
        if (relevant.length && relevant.every(function(c) { return next.owned[c.id]; }))
            next.earned[goals[type]] = true;
    });
    if (cosmetics.every(function(c) { return next.owned[c.id]; }))
        next.earned["collection-everything"] = true;

    var seals = {};
    placed.forEach(function(p) {
        var a = achievement(p.item);
        if (a && next.claimed[a.id]) seals[a.id] = true;
    });
    if (Object.keys(seals).length >= 7) next.earned["secret-sevenfold"] = true;
    if (next.canvas.background === "bg-graphite" && next.canvas.fonts.name === "font-mono" &&
        next.featured && achievement(next.featured).rarity === "legendary")
        next.earned["secret-monolith"] = true;

    if (achievements.filter(function(a) { return !a.secret && a.id !== "master-archive"; })
        .every(function(a) { return next.earned[a.id]; }))
        next.earned["master-archive"] = true;
    if (achievements.filter(function(a) { return a.id !== "secret-totality"; })
        .every(function(a) { return next.earned[a.id]; }))
        next.earned["secret-totality"] = true;
    return next;
}
function signal(state) {
    var next = clean(state);
    if (next.earned["secret-signal"]) return { state:next, changed:false };
    var previous = Number(next.secretSignal) || 0;
    next.secretSignal = Math.min(7, previous + 1);
    if (next.secretSignal >= 7) next.earned["secret-signal"] = true;
    return { state:clean(next), changed:true };
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
                            y: clamp(s.y, 0, 0.94, 0.5), size: clamp(s.size, 36, 300, 82),
                            rotation: clamp(s.rotation, -45, 45, 0) });
        });
    }
    var image = typeof rawCanvas.image === "string" &&
        /^file:\/\/\/[^?#]{1,1024}\.(png|jpe?g|webp|gif|bmp)$/i.test(rawCanvas.image)
        ? rawCanvas.image : "";
    var secretSignal = Math.floor(clamp(source.secretSignal, 0, 7, 0));
    var result = {
        version: 3, earned: earned, claimed: claimed, owned: owned, visited: visited,
        secretSignal: secretSignal,
        featured: featured, active: active, balance: Math.max(0, balance),
        canvas: { background: background, image: image, fonts: fonts, stickers: stickers,
                  serial: Math.floor(clamp(rawCanvas.serial, 0, 99999999, 0)) }
    };
    return evaluate(result);
}

function unlock(state, id) {
    var next = clean(state);
    var target = achievement(id);
    if (!target || target.secret || target.id === "master-archive" || next.earned[id])
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
    var ns = clamp(size, 36, 300, s.size), nr = clamp(rotation, -45, 45, s.rotation);
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

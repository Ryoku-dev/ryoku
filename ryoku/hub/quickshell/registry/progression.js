.pragma library

// Local-only Ryoku achievements, collection and profile canvas.
// Entitlements here are personal customisation, never trusted online currency.
var achievements = [
    { id: "ryoku-first-steps", category: "Ryoku", title: "First Steps", detail: "Open your achievement collection", reward: 30, glyph: "flag", rarity: "common", sigil: "I" },
    { id: "ryoku-pathfinder", category: "Ryoku", title: "Pathfinder", detail: "Visit twelve different Ryoku settings pages", reward: 80, glyph: "explore", rarity: "uncommon", sigil: "V" },
    { id: "ryoku-voyager", category: "Ryoku", title: "Voyager", detail: "Visit thirty different Ryoku settings pages", reward: 150, glyph: "public", rarity: "rare", sigil: "XV" },
    { id: "ryoku-versatile", category: "Ryoku", title: "Versatile", detail: "Explore six distinct Settings areas", reward: 100, glyph: "auto_awesome", rarity: "rare", sigil: "IV" },
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
    { id: "ryoku-tenfold", category: "Ryoku", title: "Tenfold", detail: "Visit twenty-four different Settings pages", reward: 90, glyph: "route", rarity: "uncommon", sigil: "X" },
    { id: "ryoku-cartographer", category: "Ryoku", title: "Cartographer", detail: "Map forty different Settings pages", reward: 170, glyph: "map", rarity: "epic", sigil: "XXV" },
    { id: "ryoku-omniscope", category: "Ryoku", title: "Omniscope", detail: "Explore forty-two different Settings pages", reward: 190, glyph: "all_inclusive", rarity: "legendary", sigil: "XXX" },
    { id: "profile-mosaic", category: "Profile", title: "Mosaic", detail: "Display five unique stickers at once", reward: 140, glyph: "dashboard_customize", rarity: "rare", sigil: "V" },
    { id: "profile-gallery", category: "Profile", title: "Gallery", detail: "Display ten unique stickers at once", reward: 160, glyph: "collections", rarity: "epic", sigil: "X" },
    { id: "profile-sanctuary", category: "Profile", title: "The Sanctuary", detail: "Display twenty-four different sticker designs at once", reward: 210, glyph: "temple_buddhist", rarity: "mythic", sigil: "XXIV" },
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
    { id: "master-archive", category: "Mastery", title: "First Archive", detail: "Complete the original unclassified archive", reward: 230, glyph: "account_balance", rarity: "mythic", sigil: "力" },
    { id: "secret-signal", category: "Classified", title: "Signal in the Stone", detail: "Follow the archive hidden signal", reward: 210, glyph: "sensors", rarity: "holographic", sigil: "?", secret: true },
    { id: "secret-sevenfold", category: "Classified", title: "Sevenfold", detail: "Display seven unique earned badge seals", reward: 220, glyph: "deployed_code", rarity: "holographic", sigil: "VII", secret: true },
    { id: "secret-monolith", category: "Classified", title: "Monolith Protocol", detail: "Unite graphite field terminal name and legendary seal", reward: 250, glyph: "hexagon", rarity: "holographic", sigil: "NULL", secret: true },
    { id: "secret-totality", category: "Classified", title: "The First Seal", detail: "Finish all four category mastery journeys", reward: 420, glyph: "all_inclusive", rarity: "holographic", sigil: "力", secret: true },
    { id: "ryoku-steadfast", category: "Ryoku", title: "First Week", detail: "Visit Ryoku on seven different days", reward: 190, glyph: "today", rarity: "rare", sigil: "VII" },
    { id: "ryoku-two-weeks", category: "Ryoku", title: "Habit Formed", detail: "Return on fourteen separate days", reward: 1100, glyph: "event_repeat", rarity: "epic", sigil: "XIV" },
    { id: "ryoku-thirty", category: "Ryoku", title: "A Month in the Archive", detail: "Open Ryoku on thirty different days", reward: 2600, glyph: "calendar_month", rarity: "mythic", sigil: "XXX" },
    { id: "ryoku-mapmaker", category: "Ryoku", title: "Behind Every Door", detail: "Explore forty different Settings pages", reward: 220, glyph: "map", rarity: "legendary", sigil: "XL" },
    { id: "profile-variety", category: "Profile", title: "Variety Show", detail: "Display five different stickers at once", reward: 150, glyph: "interests", rarity: "rare", sigil: "V" },
    { id: "profile-curation", category: "Profile", title: "Curated Exhibit", detail: "Display twelve unique sticker designs at once", reward: 310, glyph: "museum", rarity: "legendary", sigil: "XII" },
    { id: "profile-week", category: "Profile", title: "Seven Versions of Me", detail: "Decorate your profile on seven different days", reward: 750, glyph: "brush", rarity: "mythic", sigil: "VII" },
    { id: "profile-wardrobe", category: "Profile", title: "Changing Rooms", detail: "Use five different collectible backgrounds", reward: 240, glyph: "wallpaper", rarity: "epic", sigil: "V" },
    { id: "card-collector", category: "Card studio", title: "Dress the Machine", detail: "Own every card palette and complete the original card goals", reward: 320, glyph: "style", rarity: "legendary", sigil: "力" },
    { id: "card-exhibit", category: "Card studio", title: "Living Portrait", detail: "Own a palette, a custom portrait and a themed backdrop", reward: 250, glyph: "collections", rarity: "epic", sigil: "XI" },
    { id: "collection-thirty", category: "Collection", title: "Thirty in the Vault", detail: "Claim thirty distinct achievement badges", reward: 250, glyph: "bookmarks", rarity: "epic", sigil: "XXX" },
    { id: "collection-forty", category: "Collection", title: "An Inconvenient Hobby", detail: "Claim forty achievement badges", reward: 380, glyph: "inventory", rarity: "legendary", sigil: "XL" },
    { id: "collection-tenth-buy", category: "Collection", title: "Retail Therapy", detail: "Buy ten different collectables", reward: 190, glyph: "shopping_bag", rarity: "rare", sigil: "X" },
    { id: "collection-twentieth-buy", category: "Collection", title: "The Hoarder", detail: "Buy twenty different collectables", reward: 340, glyph: "inventory_2", rarity: "mythic", sigil: "XX" },
    { id: "collection-patron", category: "Collection", title: "Regular Patron", detail: "Visit the Points Shop on seven different days", reward: 650, glyph: "storefront", rarity: "epic", sigil: "VII" },
    { id: "master-ryoku", category: "Mastery", title: "Master of Ryoku", detail: "Complete the entire Ryoku achievement collection", reward: 280, glyph: "verified", rarity: "legendary", sigil: "力" },
    { id: "master-profile", category: "Mastery", title: "Master of Identity", detail: "Complete every Profile achievement", reward: 280, glyph: "verified", rarity: "legendary", sigil: "身" },
    { id: "master-card", category: "Mastery", title: "Master of the Card", detail: "Complete every Card Studio achievement", reward: 280, glyph: "verified", rarity: "legendary", sigil: "札" },
    { id: "master-collection", category: "Mastery", title: "Master Collector", detail: "Complete every Collection achievement", reward: 280, glyph: "verified", rarity: "legendary", sigil: "蔵" },
    { id: "secret-keepsake", category: "Classified", title: "Human After All", detail: "Find the hidden human side of Ryoku", reward: 340, glyph: "favorite", rarity: "holographic", sigil: "心", secret: true },
    { id: "secret-empty", category: "Classified", title: "Did You Delete the Whole Thing?", detail: "Take down a display of at least twelve stickers", reward: 240, glyph: "delete_sweep", rarity: "holographic", sigil: "∅", secret: true },
    { id: "secret-twelve", category: "Classified", title: "The Twelve", detail: "Display twelve unique earned badges at once", reward: 320, glyph: "all_out", rarity: "holographic", sigil: "XII", secret: true },
    { id: "secret-atelier", category: "Classified", title: "Gallery After Dark", detail: "Display five different Ryoku art relics on your profile", reward: 320, glyph: "museum", rarity: "holographic", sigil: "石", secret: true },
    { id: "secret-moon", category: "Classified", title: "Moon Ritual", detail: "Place the moon artwork over the Signal Void backdrop", reward: 250, glyph: "brightness_2", rarity: "holographic", sigil: "月", secret: true },
    { id: "secret-scribe", category: "Classified", title: "Ink Collector", detail: "Collect every typography option and set all three text roles", reward: 280, glyph: "draw", rarity: "holographic", sigil: "文", secret: true },
    { id: "secret-timekeeper", category: "Classified", title: "The Long Game", detail: "Keep returning to Ryoku for fourteen different days", reward: 260, glyph: "hourglass_top", rarity: "holographic", sigil: "時", secret: true },
    { id: "secret-many-faces", category: "Classified", title: "Identity Crisis", detail: "Use five different featured badges over time", reward: 290, glyph: "face", rarity: "holographic", sigil: "五", secret: true },
    { id: "secret-nightshift", category: "Classified", title: "The Midnight Compiler", detail: "Visit Settings on three different evenings", reward: 220, glyph: "nights_stay", rarity: "holographic", sigil: "夜", secret: true },
    { id: "secret-rabbit", category: "Classified", title: "Are You Still Here?", detail: "Browse forty settings pages and decorate on seven days", reward: 300, glyph: "pest_control_rodent", rarity: "holographic", sigil: "兎", secret: true },
    { id: "master-hyperion", category: "Mastery", title: "Living Archive", detail: "Master every category in the new archive", reward: 650, glyph: "workspace_premium", rarity: "holographic", sigil: "道" },
    { id: "secret-absolute", category: "Classified", title: "The Final Seal", detail: "Complete every achievement, including the classified archive", reward: 800, glyph: "all_inclusive", rarity: "holographic", sigil: "力", secret: true }
];

// The original alpha archive is a permanent historical cohort.
var originalAchievements = ["ryoku-first-steps", "ryoku-pathfinder", "ryoku-voyager", "ryoku-versatile", "profile-curator", "profile-sticker", "profile-background", "profile-type", "welcome", "explorer", "identity", "portrait", "layout", "showcase", "ryoku-tenfold", "ryoku-cartographer", "ryoku-omniscope", "profile-mosaic", "profile-gallery", "profile-sanctuary", "profile-self", "profile-identity", "profile-custom", "card-spectrum", "card-complete", "card-tapestry", "collection-firstclaim", "collection-fifth", "collection-tenth", "collection-twentieth", "collection-firstbuy", "collection-trio", "collection-stickers", "collection-backgrounds", "collection-fonts", "collection-everything", "secret-signal", "secret-sevenfold", "secret-monolith"];

var groups = [
    { id: "Ryoku", title: "Ryoku", subtitle: "Explore the world you call home" },
    { id: "Profile", title: "Profile", subtitle: "Make your identity your own" },
    { id: "Card studio", title: "Card Studio", subtitle: "Craft the machine that represents you" },
    { id: "Collection", title: "Collection", subtitle: "Acquire, classify, preserve" },
    { id: "Mastery", title: "Mastery", subtitle: "For those who complete the archive" },
    { id: "Classified", title: "Classified", subtitle: "Nothing here is given away" }
];

var legacyPrices = {"midnight": 150, "ember": 200, "jade": 250, "sticker-orbit": 65, "sticker-flame": 75, "sticker-bloom": 60, "sticker-star": 45, "bg-graphite": 60, "bg-blueprint": 70, "bg-ember": 80, "font-serif": 50, "font-mono": 60, "font-clean": 65, "sticker-origins": 140, "sticker-sutra": 105, "sticker-ember": 140, "sticker-archive": 120, "sticker-moonseal": 115, "sticker-fossil": 120, "bg-archive": 175, "bg-obsidian": 180, "bg-sutra": 195, "bg-signal": 200, "font-ritual": 130, "font-archive": 135, "font-stone": 150};

var cosmetics = [
    { id: "midnight", name: "Midnight", type: "palette", price: 322,
      palette: { ink: "#f2f1fc", plate: "#141426", accent: "#a4a3ef", rim: "#7776c4" } },
    { id: "ember", name: "Ember", type: "palette", price: 430,
      palette: { ink: "#ffe6d7", plate: "#281813", accent: "#ffad72", rim: "#d77c58" } },
    { id: "jade", name: "Jade", type: "palette", price: 538,
      palette: { ink: "#e0fff1", plate: "#10251e", accent: "#7de0b8", rim: "#49b58c" } },
    { id: "sticker-orbit", name: "Astral Archive", type: "sticker", price: 140, art: "ryodecors/moon.png", glyph: "planet", tint: "#b9c9fa" },
    { id: "sticker-flame", name: "Ryoku Needle", type: "sticker", price: 161, art: "ryodecors/needle.png", glyph: "local_fire_department", tint: "#ffac83" },
    { id: "sticker-bloom", name: "Laocoön", type: "sticker", price: 129, art: "ryodecors/laocoon.png", glyph: "filter_vintage", tint: "#d9b0ef" },
    { id: "sticker-star", name: "Hawk Study", type: "sticker", price: 97, art: "ryodecors/hawk.png", glyph: "star", tint: "#f6d18b" },
    { id: "bg-graphite", name: "Aurelius / Graphite", type: "background", price: 156, art: "ryodecors/aurelius.png" },
    { id: "bg-blueprint", name: "Ryoku Needle", type: "background", price: 182, art: "ryodecors/needle.png" },
    { id: "bg-ember", name: "Afterglow / Blade", type: "background", price: 208, art: "ryodecors/katana.png" },
    { id: "font-serif", name: "Engraved", type: "font", price: 108, family: "Liberation Serif" },
    { id: "font-mono", name: "Terminal", type: "font", price: 129, family: "JetBrainsMono Nerd Font" },
    { id: "font-clean", name: "Technical", type: "font", price: 140, family: "Inter Display" },
    { id: "sticker-origins", name: "Carved David", type: "sticker", price: 301, art: "ryodecors/david.png", glyph: "token", tint: "#ef8d69", rarity: "legendary", sigil: "力" },
    { id: "sticker-sutra", name: "Marcus Aurelius", type: "sticker", price: 226, art: "ryodecors/aurelius.png", glyph: "deployed_code", tint: "#b3b7c5", rarity: "epic", sigil: "道" },
    { id: "sticker-ember", name: "The Katana", type: "sticker", price: 301, art: "ryodecors/katana.png", glyph: "whatshot", tint: "#e97452", rarity: "mythic", sigil: "焔" },
    { id: "sticker-archive", name: "Camera Obscura", type: "sticker", price: 258, art: "ryodecors/camera.png", glyph: "verified_user", tint: "#b8ab90", rarity: "rare", sigil: "III" },
    { id: "sticker-moonseal", name: "Microphone", type: "sticker", price: 247, art: "ryodecors/mic.png", glyph: "brightness_2", tint: "#b3bbdd", rarity: "rare", sigil: "月" },
    { id: "sticker-fossil", name: "Rashin Hero", type: "sticker", price: 258, art: "ryodecors/rashin-hero.png", glyph: "museum", tint: "#d8c8b0", rarity: "epic", sigil: "石" },
    { id: "bg-archive", name: "Laocoön Archive", type: "background", price: 455, art: "ryodecors/laocoon.png" },
    { id: "bg-obsidian", name: "Lunar Obsidian", type: "background", price: 468, art: "ryodecors/moon.png" },
    { id: "bg-sutra", name: "Stone & Sutra", type: "background", price: 507, art: "ryodecors/david.png" },
    { id: "bg-signal", name: "Signal / Rashin", type: "background", price: 520, art: "ryodecors/rashin-hero.png" },
    { id: "font-ritual", name: "Ritual Sans", type: "font", price: 280, family: "Inter Display" },
    { id: "font-archive", name: "Archive Mono", type: "font", price: 290, family: "FiraCode Nerd Font" },
    { id: "font-stone", name: "Stone Serif", type: "font", price: 322, family: "Noto Serif" },
    { id: "bg-atelier", name: "The Atelier", type: "background", price: 550, art: "ryodecors/laocoon.png" },
    { id: "bg-antiquity", name: "Ancient Archive", type: "background", price: 600, art: "ryodecors/aurelius.png" },
    { id: "bg-memento", name: "Memento Mori", type: "background", price: 700, art: "ryodecors/david.png" },
    { id: "bg-astral", name: "Astral Cartography", type: "background", price: 740, art: "ryodecors/moon.png" },
    { id: "bg-storm", name: "Rashin Afterimage", type: "background", price: 820, art: "ryodecors/rashin-hero.png" },
    { id: "bg-metal", name: "Temple of Steel", type: "background", price: 780, art: "ryodecors/katana.png" }
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
function progress(state, id) {
    var next = clean(state);
    if (!achievement(id) || next.earned[id]) return "";
    var visited = Object.keys(next.visited).length;
    var areas = {};
    Object.keys(next.visited).forEach(function(key) { areas[next.visited[key]] = true; });
    var placed = {};
    next.canvas.stickers.forEach(function(sticker) { placed[sticker.item] = true; });
    var claimed = Object.keys(next.claimed).length;
    var owned = Object.keys(next.owned).length;
    var total = {
        "ryoku-pathfinder": [visited, 12], "ryoku-tenfold": [visited, 24],
        "ryoku-voyager": [visited, 30], "ryoku-cartographer": [visited, 40],
        "ryoku-mapmaker": [visited, 40], "ryoku-omniscope": [visited, 42],
        "ryoku-versatile": [Object.keys(areas).length, 6],
        "ryoku-steadfast": [Object.keys(next.days).length, 7],
        "ryoku-two-weeks": [Object.keys(next.days).length, 14],
        "ryoku-thirty": [Object.keys(next.days).length, 30],
        "profile-mosaic": [Object.keys(placed).length, 5],
        "profile-gallery": [Object.keys(placed).length, 10],
        "profile-sanctuary": [Object.keys(placed).length, 24],
        "profile-variety": [Object.keys(placed).length, 5],
        "profile-curation": [Object.keys(placed).length, 12],
        "profile-week": [Object.keys(next.customDays).length, 7],
        "profile-wardrobe": [Object.keys(next.backgroundHistory).length, 5],
        "collection-fifth": [claimed, 5], "collection-tenth": [claimed, 10],
        "collection-twentieth": [claimed, 20], "collection-thirty": [claimed, 30],
        "collection-forty": [claimed, 40], "collection-trio": [owned, 3],
        "collection-tenth-buy": [owned, 10], "collection-twentieth-buy": [owned, 20],
        "collection-patron": [Object.keys(next.shopDays).length, 7]
    };
    var measure = total[id];
    if (measure) return Math.min(measure[0], measure[1]) + " / " + measure[1];
    var master = { "master-ryoku": "Ryoku", "master-profile": "Profile",
                   "master-card": "Card studio", "master-collection": "Collection" };
    if (master[id]) {
        var list = achievements.filter(function(a) { return a.category === master[id]; });
        return list.filter(function(a) { return next.earned[a.id]; }).length + " / " + list.length;
    }
    return "";
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
    var days = Object.keys(next.days).length;
    var customDays = Object.keys(next.customDays).length;
    var claimed = Object.keys(next.claimed).length;
    var owned = Object.keys(next.owned).length;
    var placed = next.canvas.stickers;
    var distinct = {};
    placed.forEach(function(p) { distinct[p.item] = true; });
    var uniqueCount = Object.keys(distinct).length;

    if (count >= 24) next.earned["ryoku-tenfold"] = true;
    if (count >= 40) next.earned["ryoku-cartographer"] = true;
    if (count >= 42) next.earned["ryoku-omniscope"] = true;
    if (count >= 40) next.earned["ryoku-mapmaker"] = true;
    if (days >= 7) next.earned["ryoku-steadfast"] = true;
    if (days >= 14) next.earned["ryoku-two-weeks"] = true;
    if (days >= 30) next.earned["ryoku-thirty"] = true;

    if (uniqueCount >= 5) next.earned["profile-mosaic"] = true;
    if (uniqueCount >= 10) next.earned["profile-gallery"] = true;
    if (uniqueCount >= 24) next.earned["profile-sanctuary"] = true;
    if (uniqueCount >= 5) next.earned["profile-variety"] = true;
    if (uniqueCount >= 12) next.earned["profile-curation"] = true;
    if (customDays >= 7) next.earned["profile-week"] = true;
    if (Object.keys(next.backgroundHistory).length >= 5) next.earned["profile-wardrobe"] = true;
    if (next.canvas.image) next.earned["profile-self"] = true;
    if (["name", "hostname", "tagline"].every(function(role) { return !!next.canvas.fonts[role]; }))
        next.earned["profile-identity"] = true;
    if ((next.canvas.background || next.canvas.image) &&
        Object.keys(next.canvas.fonts).some(function(role) { return !!next.canvas.fonts[role]; }) &&
        next.featured)
        next.earned["profile-custom"] = true;

    var original = ["welcome", "explorer", "identity", "portrait", "layout", "showcase"];
    if (original.every(function(id) { return next.earned[id]; })) next.earned["card-complete"] = true;
    if (original.every(function(id) { return next.claimed[id]; })) next.earned["card-tapestry"] = true;
    if (next.active) next.earned["card-spectrum"] = true;
    if (original.every(function(id) { return next.earned[id]; }) &&
        cosmetics.filter(function(c) { return c.type === "palette"; })
            .every(function(c) { return next.owned[c.id]; }))
        next.earned["card-collector"] = true;
    if (next.active && next.earned.portrait && (next.canvas.background || next.canvas.image))
        next.earned["card-exhibit"] = true;

    if (claimed >= 1) next.earned["collection-firstclaim"] = true;
    if (claimed >= 5) next.earned["collection-fifth"] = true;
    if (claimed >= 10) next.earned["collection-tenth"] = true;
    if (claimed >= 20) next.earned["collection-twentieth"] = true;
    if (claimed >= 30) next.earned["collection-thirty"] = true;
    if (claimed >= 40) next.earned["collection-forty"] = true;
    if (owned >= 1) next.earned["collection-firstbuy"] = true;
    if (owned >= 3) next.earned["collection-trio"] = true;
    if (owned >= 10) next.earned["collection-tenth-buy"] = true;
    if (owned >= 20) next.earned["collection-twentieth-buy"] = true;
    if (Object.keys(next.shopDays).length >= 7) next.earned["collection-patron"] = true;

    ["sticker", "background", "font"].forEach(function(type) {
        var relevant = cosmetics.filter(function(c) { return c.type === type; });
        var goals = { sticker: "collection-stickers", background: "collection-backgrounds", font: "collection-fonts" };
        if (relevant.length && relevant.every(function(c) { return next.owned[c.id]; }))
            next.earned[goals[type]] = true;
    });
    if (cosmetics.every(function(c) { return next.owned[c.id]; }))
        next.earned["collection-everything"] = true;

    var seals = {}, gallery = {};
    placed.forEach(function(p) {
        var a = achievement(p.item);
        if (a && next.claimed[a.id]) seals[a.id] = true;
        var c = cosmetic(p.item);
        if (c && c.type === "sticker" && next.owned[c.id]) gallery[c.id] = true;
    });
    if (Object.keys(seals).length >= 7) next.earned["secret-sevenfold"] = true;
    if (next.canvas.background === "bg-graphite" && next.canvas.fonts.name === "font-mono" &&
        next.featured && achievement(next.featured).rarity === "legendary")
        next.earned["secret-monolith"] = true;
    if (Object.keys(next.signalDays).length >= 7) next.earned["secret-signal"] = true;
    if (next.keepsakeSeen) next.earned["secret-keepsake"] = true;
    if (next.maxStickers >= 12 && !placed.length) next.earned["secret-empty"] = true;
    if (Object.keys(seals).length >= 12) next.earned["secret-twelve"] = true;
    if (Object.keys(gallery).length >= 5) next.earned["secret-atelier"] = true;
    if (next.canvas.background === "bg-signal" && distinct["sticker-orbit"])
        next.earned["secret-moon"] = true;
    if (cosmetics.filter(function(c) { return c.type === "font"; })
        .every(function(c) { return next.owned[c.id]; }) &&
        ["name", "hostname", "tagline"].every(function(role) { return !!next.canvas.fonts[role]; }))
        next.earned["secret-scribe"] = true;
    if (days >= 14) next.earned["secret-timekeeper"] = true;
    if (Object.keys(next.featuredHistory).length >= 5)
        next.earned["secret-many-faces"] = true;
    if (Object.keys(next.eveningDays).length >= 3)
        next.earned["secret-nightshift"] = true;
    if (count >= 40 && customDays >= 7)
        next.earned["secret-rabbit"] = true;

    ["Ryoku", "Profile", "Card studio", "Collection"].forEach(function(category) {
        var masters = { "Ryoku": "master-ryoku", "Profile": "master-profile",
                        "Card studio": "master-card", "Collection": "master-collection" };
        if (achievements.filter(function(a) { return a.category === category; })
            .every(function(a) { return next.earned[a.id]; }))
            next.earned[masters[category]] = true;
    });
    // Original archive achievements remain independently achievable; old
    // collectors keep their exact old badges instead of losing past progress.
    if (originalAchievements.filter(function(id) { return !achievement(id).secret; })
        .every(function(id) { return next.earned[id]; }))
        next.earned["master-archive"] = true;
    if (originalAchievements.every(function(id) { return next.earned[id]; }) &&
        next.earned["master-archive"])
        next.earned["secret-totality"] = true;

    if (["master-ryoku", "master-profile", "master-card", "master-collection"]
        .every(function(id) { return next.earned[id]; }))
        next.earned["master-hyperion"] = true;
    if (achievements.filter(function(a) { return a.id !== "secret-absolute"; })
        .every(function(a) { return next.earned[a.id]; }))
        next.earned["secret-absolute"] = true;
    return next;
}

function signal(state, at) {
    var next = clean(state);
    if (next.earned["secret-signal"]) return { state: next, changed: false };
    if (!noteDay(next, "signalDays", at))
        return { state: next, changed: false };
    next.secretSignal = Math.min(7, Object.keys(next.signalDays).length);
    return { state: clean(next), changed: true };
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
    var paid = {}, balance = 0;
    cosmetics.forEach(function(c) {
        if (!owned[c.id]) return;
        var prior = source.paid && Number(source.paid[c.id]);
        var legacy = Number(source.version) >= 4 ? c.price :
            (legacyPrices[c.id] === undefined ? c.price : legacyPrices[c.id]);
        paid[c.id] = isFinite(prior) && prior >= 0 && Number(source.version) >= 4 ?
            Math.min(c.price, Math.round(prior)) : legacy;
    });
    achievements.forEach(function(a) { if (claimed[a.id]) balance += a.reward; });
    Object.keys(paid).forEach(function(id) { balance -= paid[id]; });

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
    var dates = {};
    ["days", "customDays", "shopDays", "eveningDays", "signalDays"].forEach(function(kind) {
        dates[kind] = {};
        var raw = source[kind] && typeof source[kind] === "object" ? source[kind] : {};
        Object.keys(raw).slice(0, 366).forEach(function(day) {
            if (/^20[0-9]{2}-(0[1-9]|1[0-2])-([012][0-9]|3[01])$/.test(day) &&
                raw[day] === true) dates[kind][day] = true;
        });
    });
    var featuredHistory = {}, backgroundHistory = {};
    [ ["featuredHistory", featuredHistory], ["backgroundHistory", backgroundHistory] ].forEach(function(pair) {
        var raw = source[pair[0]] && typeof source[pair[0]] === "object" ? source[pair[0]] : {};
        Object.keys(raw).slice(0, 100).forEach(function(id) {
            if (validId(id) && raw[id] === true) pair[1][id] = true;
        });
    });
    var result = {
        version: 4, earned: earned, claimed: claimed, owned: owned, visited: visited,
        paid: paid, secretSignal: secretSignal, days: dates.days, customDays: dates.customDays,
        shopDays: dates.shopDays, eveningDays: dates.eveningDays, signalDays: dates.signalDays,
        featuredHistory: featuredHistory, backgroundHistory: backgroundHistory,
        maxStickers: Math.floor(clamp(source.maxStickers, 0, 24, 0)),
        keepsakeSeen: source.keepsakeSeen === true,
        featured: featured, active: active, balance: Math.max(0, balance),
        canvas: { background: background, image: image, fonts: fonts, stickers: stickers,
                  serial: Math.floor(clamp(rawCanvas.serial, 0, 99999999, 0)) }
    };
    return evaluate(result);
}

function unlock(state, id) {
    var next = clean(state);
    var target = achievement(id);
    if (!target || target.secret || target.category === "Mastery" ||
        !["ryoku-first-steps", "welcome", "explorer", "identity", "portrait", "layout", "showcase"].includes(id) ||
        next.earned[id])
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
    next.paid[id] = item.price;
    return { state: clean(next), changed: true };
}

function equip(state, id) {
    var next = clean(state);
    if (id !== "" && (!next.owned[id] || cosmetic(id).type !== "palette"))
        return { state: next, changed: false };
    if (next.active === id)
        return { state: next, changed: false };
    next.active = id;
    touchProfile(next);
    return { state: clean(next), changed: true };
}

function dayKey(value) {
    var date = value instanceof Date ? value : value ? new Date(value) : new Date();
    if (!isFinite(date.getTime())) return "";
    return date.getFullYear() + "-" + String(date.getMonth() + 1).padStart(2, "0") +
        "-" + String(date.getDate()).padStart(2, "0");
}
function noteDay(next, kind, at) {
    var key = dayKey(at);
    if (!key || next[kind][key]) return false;
    next[kind][key] = true;
    return true;
}
function visit(state, page, category, at) {
    var next = clean(state);
    if (!validId(page) || !validId(category)) return { state: next, changed: false };
    var changed = noteDay(next, "days", at);
    var when = at ? new Date(at) : new Date();
    if (isFinite(when.getTime()) && when.getHours() >= 20 && when.getHours() <= 23)
        changed = noteDay(next, "eveningDays", at) || changed;
    if (page === "points-shop")
        changed = noteDay(next, "shopDays", at) || changed;
    if (!next.visited[page]) {
        next.visited[page] = category;
        changed = true;
    }
    var count = Object.keys(next.visited).length, areas = {};
    Object.keys(next.visited).forEach(function(k) { areas[next.visited[k]] = true; });
    if (count >= 12) next.earned["ryoku-pathfinder"] = true;
    if (count >= 30) next.earned["ryoku-voyager"] = true;
    if (Object.keys(areas).length >= 6) next.earned["ryoku-versatile"] = true;
    return { state: clean(next), changed: changed };
}
function startDay(state, at) {
    var next = clean(state);
    var changed = noteDay(next, "days", at);
    return { state: changed ? clean(next) : next, changed: changed };
}
function keepsake(state) {
    var next = clean(state);
    if (next.keepsakeSeen) return { state: next, changed: false };
    next.keepsakeSeen = true;
    return { state: clean(next), changed: true };
}
function touchProfile(next, at) {
    noteDay(next, "customDays", at);
}

function feature(state, id) {
    var next = clean(state);
    if (id !== "" && !next.claimed[id])
        return { state: next, changed: false };
    if (next.featured === id)
        return { state: next, changed: false };
    next.featured = id;
    if (id) {
        next.earned["profile-curator"] = true;
        next.featuredHistory[id] = true;
    }
    touchProfile(next);
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
    if (id) {
        next.earned["profile-background"] = true;
        next.backgroundHistory[id] = true;
    }
    touchProfile(next);
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
    if (url) {
        next.earned["profile-background"] = true;
        next.backgroundHistory["personal"] = true;
    }
    touchProfile(next);
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
    touchProfile(next);
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
    next.maxStickers = Math.max(next.maxStickers, next.canvas.stickers.length);
    touchProfile(next);
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
    touchProfile(next);
    return { state: clean(next), changed: true };
}

function removeSticker(state, slot) {
    var next = clean(state);
    var count = next.canvas.stickers.length;
    next.canvas.stickers = next.canvas.stickers.filter(function(s) { return s.slot !== slot; });
    if (count !== next.canvas.stickers.length) touchProfile(next);
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

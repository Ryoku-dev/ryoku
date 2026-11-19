.pragma library

// Local Ryoku-wide progression. Badges describe verified user actions, not hardware tier.
var achievements = [
    { id: "ryoku-first-steps", category: "Ryoku", title: "First Steps", detail: "Visit your Ryoku achievements", reward: 30 },
    { id: "ryoku-pathfinder", category: "Ryoku", title: "Pathfinder", detail: "Explore five different Ryoku settings pages", reward: 80 },
    { id: "ryoku-voyager", category: "Ryoku", title: "Voyager", detail: "Explore fifteen different Ryoku settings pages", reward: 150 },
    { id: "ryoku-versatile", category: "Ryoku", title: "Versatile", detail: "Explore four different areas of Ryoku", reward: 100 },
    { id: "welcome", category: "Card studio", title: "First Contact", detail: "Open your card studio", reward: 50 },
    { id: "explorer", category: "Card studio", title: "Under the Hood", detail: "Inspect your card's rarity", reward: 80 },
    { id: "identity", category: "Card studio", title: "Signature", detail: "Personalise your card's words", reward: 100 },
    { id: "portrait", category: "Card studio", title: "A New Face", detail: "Choose your own card artwork", reward: 100 },
    { id: "layout", category: "Card studio", title: "Art Director", detail: "Customise your card layout", reward: 120 },
    { id: "showcase", category: "Card studio", title: "Showtime", detail: "Successfully copy or save your card", reward: 150 }
];

var cosmetics = [
    { id: "midnight", name: "Midnight", price: 150,
      palette: { ink: "#f2f1fc", plate: "#141426", accent: "#a4a3ef", rim: "#7776c4" } },
    { id: "ember", name: "Ember", price: 200,
      palette: { ink: "#ffe6d7", plate: "#281813", accent: "#ffad72", rim: "#d77c58" } },
    { id: "jade", name: "Jade", price: 250,
      palette: { ink: "#e0fff1", plate: "#10251e", accent: "#7de0b8", rim: "#49b58c" } }
];

function clean(value) {
    var source = value && typeof value === "object" ? value : {};
    var earned = {};
    var owned = {};
    for (var i = 0; i < achievements.length; i++) {
        var id = achievements[i].id;
        if (source.earned && source.earned[id] === true)
            earned[id] = true;
    }
    for (var j = 0; j < cosmetics.length; j++) {
        var key = cosmetics[j].id;
        if (source.owned && source.owned[key] === true)
            owned[key] = true;
    }
    var balance = 0;
    for (var e = 0; e < achievements.length; e++)
        if (earned[achievements[e].id])
            balance += achievements[e].reward;
    for (var c = 0; c < cosmetics.length; c++)
        if (owned[cosmetics[c].id])
            balance -= cosmetics[c].price;
    var active = owned[source.active] ? source.active : "";
    var visited = {};
    if (source.visited && typeof source.visited === "object")
        Object.keys(source.visited).slice(0, 150).forEach(function(key) {
            if (/^[a-z][a-z0-9-]{0,49}$/.test(key) && typeof source.visited[key] === "string"
                    && /^[a-z][a-z0-9-]{0,49}$/.test(source.visited[key]))
                visited[key] = source.visited[key];
        });
    var featured = earned[source.featured] ? source.featured : "";
    return { version: 2, earned: earned, owned: owned, active: active,
             featured: featured, visited: visited, balance: Math.max(0, balance) };
}

function unlock(state, id) {
    var next = clean(state);
    if (next.earned[id] || !achievements.some(function(a) { return a.id === id; }))
        return { state: next, changed: false };
    next.earned[id] = true;
    return { state: clean(next), changed: true };
}

function purchase(state, id) {
    var next = clean(state);
    var item = cosmetics.find(function(c) { return c.id === id; });
    if (!item || next.owned[id] || next.balance < item.price)
        return { state: next, changed: false };
    next.owned[id] = true;
    return { state: clean(next), changed: true };
}

function equip(state, id) {
    var next = clean(state);
    if (id !== "" && !next.owned[id])
        return { state: next, changed: false };
    if (next.active === id)
        return { state: next, changed: false };
    next.active = id;
    return { state: next, changed: true };
}

function visit(state, page, category) {
    var next = clean(state);
    if (!/^[a-z][a-z0-9-]{0,49}$/.test(String(page)) ||
        !/^[a-z][a-z0-9-]{0,49}$/.test(String(category)) || next.visited[page])
        return { state: next, changed: false };
    next.visited[page] = category;
    var count = Object.keys(next.visited).length;
    var categories = {};
    Object.keys(next.visited).forEach(function(key) { categories[next.visited[key]] = true; });
    if (count >= 5)
        next.earned["ryoku-pathfinder"] = true;
    if (count >= 15)
        next.earned["ryoku-voyager"] = true;
    if (Object.keys(categories).length >= 4)
        next.earned["ryoku-versatile"] = true;
    return { state: clean(next), changed: true };
}

function feature(state, id) {
    var next = clean(state);
    if (id !== "" && !next.earned[id])
        return { state: next, changed: false };
    if (next.featured === id)
        return { state: next, changed: false };
    next.featured = id;
    return { state: clean(next), changed: true };
}

function palette(state) {
    var s = clean(state);
    var item = cosmetics.find(function(c) { return c.id === s.active; });
    return item ? item.palette : null;
}

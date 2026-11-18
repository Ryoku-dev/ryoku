.pragma library

// Offline cosmetics only. This is user-owned progress, not an online currency.
var achievements = [
    { id: "welcome", title: "First Contact", detail: "Open your card studio", reward: 50 },
    { id: "explorer", title: "Under the Hood", detail: "Inspect your card's rarity", reward: 80 },
    { id: "identity", title: "Signature", detail: "Personalise your card's words", reward: 100 },
    { id: "portrait", title: "A New Face", detail: "Choose your own card artwork", reward: 100 },
    { id: "layout", title: "Art Director", detail: "Customise your card layout", reward: 120 },
    { id: "showcase", title: "Showtime", detail: "Successfully copy or save your card", reward: 150 }
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
    return { version: 1, earned: earned, owned: owned, active: active,
             balance: balance >= 0 ? balance : 0 };
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

function palette(state) {
    var s = clean(state);
    var item = cosmetics.find(function(c) { return c.id === s.active; });
    return item ? item.palette : null;
}

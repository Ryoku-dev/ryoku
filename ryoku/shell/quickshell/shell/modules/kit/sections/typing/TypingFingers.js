// Physical finger assignments, independent of key labels and firmware layers.
// Signed finger numbers: left < 0, right > 0; thumb=1 ... little=5.
// 6 means either thumb, 0 means unassigned. No IO in this module.
function classicFinger(column) {
    return [-5, -4, -3, -2, -2, 2, 2, 3, 4, 5][column] || 5;
}

function classicBoard(rows, label) {
    const keys = [], entries = [], fingers = [];
    const width = Math.max.apply(null, rows.map(row => row.length));
    rows.forEach((row, r) => row.forEach((char, col) => {
        keys.push({row: r, col: col, x: (width - row.length) / 2 + col, y: r, w: 1, h: 1, r: 0, rx: 0, ry: 0});
        entries.push({label: char, char: char});
        fingers.push(classicFinger(col));
    }));
    keys.push({row: rows.length, col: 0, x: (width - 7) / 2, y: rows.length, w: 7, h: 1, r: 0, rx: 0, ry: 0});
    entries.push({label: label, char: " "});
    fingers.push(6);
    return {keys: keys, entries: entries, fingers: fingers, width: width, height: rows.length + 1};
}

function center(key) {
    const angle = (key.r || 0) * Math.PI / 180;
    const x = key.x + key.w / 2, y = key.y + key.h / 2;
    const rx = key.rx || 0, ry = key.ry || 0;
    return {x: rx + (x - rx) * Math.cos(angle) - (y - ry) * Math.sin(angle),
        y: ry + (x - rx) * Math.sin(angle) + (y - ry) * Math.cos(angle)};
}

function nearest(items, x) {
    return items.reduce((best, item) => !best || Math.abs(item.x - x) < Math.abs(best.x - x) ? item : best, null);
}

function infer(keys) {
    const points = keys.map((key, index) => Object.assign({index: index, key: key}, center(key)));
    const result = keys.map(() => 0);
    const usable = points.filter(p => !p.key.encoder);
    if (!usable.length) return result;

    // ANSI/ISO/ABNT: the 1.75u leading home-row cap is a physical anchor,
    // even when Caps Lock or every letter has been remapped in Vial.
    for (const lead of usable.filter(p => Math.abs(p.key.w - 1.75) < 0.05 && !p.key.r)) {
        const row = usable.filter(p => Math.abs(p.key.y - lead.key.y) < 0.05
            && p.key.x > lead.key.x && p.key.w <= 1.05 && !p.key.r).sort((a, b) => a.x - b.x);
        if (row.length < 10) continue;
        // Stop before a separated navigation cluster / numpad.
        let count = 1;
        while (count < row.length && row[count].x - row[count - 1].x < 1.4) count++;
        if (count < 10) continue;
        const home = row.slice(0, Math.min(count, 11)).map((p, i) => ({x: p.x, finger: classicFinger(i)}));
        for (const p of usable) {
            if (p.x > home[home.length - 1].x + 2 || p.key.y < lead.key.y - 2.1) continue;
            result[p.index] = p.key.w >= 3 ? 6 : nearest(home, p.x).finger;
        }
        const numbers = usable.filter(p => Math.abs(p.key.y - (lead.key.y - 2)) < 0.05
            && p.key.w <= 1.05 && p.x < home[home.length - 1].x + 1).sort((a, b) => a.x - b.x);
        if (numbers.length >= 12) numbers.forEach((p, i) => {
            result[p.index] = [-5, -5, -4, -3, -2, -2, 2, 2, 3, 4, 5, 5, 5][i] || 5;
        });
        return result;
    }

    // Columnar / split boards: repeated columns identify the typing field.
    // Work in each KLE rotation cluster's local coordinates so a whole rotated
    // half retains its columns. Matrix rows/cols are electrical, not anatomical.
    const clusters = {};
    for (const p of usable) {
        if (p.key.w > 1.25 || p.key.h > 1.25) continue;
        const id = [p.key.r || 0, p.key.rx || 0, p.key.ry || 0].join(":");
        if (!clusters[id]) clusters[id] = [];
        let column = clusters[id].find(c => Math.abs(c.localX - (p.key.x + p.key.w / 2)) < 0.15);
        if (!column) {
            column = {localX: p.key.x + p.key.w / 2, points: []};
            clusters[id].push(column);
        }
        column.points.push(p);
    }
    const columns = [];
    for (const id of Object.keys(clusters)) {
        for (const column of clusters[id]) {
            if (column.points.length < 3) continue;
            const sorted = column.points.slice().sort((a, b) => a.key.y - b.key.y);
            const middle = sorted[Math.floor((sorted.length - 1) / 2)];
            columns.push({x: middle.x, y: middle.y, points: sorted, finger: 0});
        }
    }
    columns.sort((a, b) => a.x - b.x);
    if (columns.length < 8) return result; // Macro pads need explicit assignments.
    // Prefer a physical split near the middle, excluding outer accessory keys.
    let split = Math.floor(columns.length / 2), largest = 1.4;
    for (let i = 4; i <= columns.length - 4; i++) {
        const gap = columns[i].x - columns[i - 1].x;
        if (gap > largest) { largest = gap; split = i; }
    }
    const sides = [columns.slice(0, split), columns.slice(split)];
    sides.forEach((side, hand) => {
        // Five alpha columns per hand; any outer columns belong to the pinky.
        side.forEach((column, i) => {
            const fromInside = hand === 0 ? side.length - 1 - i : i;
            column.finger = (hand === 0 ? -1 : 1) * [2, 2, 3, 4, 5][Math.min(4, fromInside)];
            column.points.forEach(p => result[p.index] = column.finger);
        });
    });
    const midX = (columns[split - 1].x + columns[split].x) / 2;
    for (const p of usable) {
        if (result[p.index]) continue;
        const hand = p.x < midX ? 0 : 1;
        const column = nearest(sides[hand], p.x);
        const bottom = Math.max.apply(null, column.points.map(k => k.y + k.key.h / 2));
        // A separate lower cluster is thumb territory, regardless of whether
        // its caps say Space, Enter, a layer action, or a letter.
        result[p.index] = p.y > bottom + 0.15 ? (hand === 0 ? -1 : 1) : column.finger;
    }
    return result;
}

function targets(entries, character) {
    if (!character) return [];
    const wanted = character.toLowerCase();
    // Match composed characters exactly. Guessing an accent's base letter or
    // a macro's output would teach a keystroke the firmware may never send.
    return entries.reduce((out, entry, index) => {
        if (entry && entry.char && entry.char.toLowerCase() === wanted) out.push(index);
        return out;
    }, []);
}

function keyId(boardId, key) {
    return encodeURIComponent(boardId) + "/" + key.row + "/" + key.col;
}

function validFinger(value) {
    return Number.isInteger(value) && value >= -5 && value <= 6;
}

function assignments(automatic, keys, boardId, overrides) {
    const saved = {};
    for (const item of overrides) {
        const split = item.lastIndexOf("=");
        const value = Number(item.slice(split + 1));
        if (split > 0 && validFinger(value)) saved[item.slice(0, split)] = value;
    }
    return keys.map((key, i) => {
        const id = keyId(boardId, key);
        return Object.prototype.hasOwnProperty.call(saved, id) ? saved[id] : (automatic[i] || 0);
    });
}

function saveAssignment(overrides, boardId, key, finger) {
    const prefix = keyId(boardId, key) + "=";
    const out = Array.from(overrides).filter(item => !item.startsWith(prefix));
    if (validFinger(finger)) out.push(prefix + finger);
    return out;
}

function resetBoard(overrides, boardId) {
    const prefix = encodeURIComponent(boardId) + "/";
    return Array.from(overrides).filter(item => !item.startsWith(prefix));
}

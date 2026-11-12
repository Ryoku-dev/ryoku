// pure display-model helpers for the Displays page, tested by model.test.mjs

// "2560x1600@59.951" -> { key, w, h, rate }, or null
function parseMode(s) {
    var m = /^(\d+)x(\d+)@([\d.]+)/.exec(String(s || ""));
    if (!m)
        return null;
    var w = parseInt(m[1], 10), h = parseInt(m[2], 10);
    return { key: w + "x" + h + "@" + m[3], w: w, h: h, rate: parseFloat(m[3]) };
}

// the advertised modes, unique, largest and fastest first
function modeList(mon) {
    var seen = {}, out = [];
    var arr = mon && mon.modes ? mon.modes : [];
    for (var i = 0; i < arr.length; i++) {
        var p = parseMode(arr[i]);
        if (!p || seen[p.key])
            continue;
        seen[p.key] = true;
        out.push(p);
    }
    out.sort(function (a, b) { return (b.w * b.h - a.w * a.h) || (b.w - a.w) || (b.rate - a.rate); });
    return out;
}

function pickCurrentMode(mon) {
    var arr = modeList(mon), i;
    for (i = 0; i < arr.length; i++)
        if (arr[i].w === mon.width && arr[i].h === mon.height && Math.round(arr[i].rate) === Math.round(mon.refresh))
            return arr[i].key;
    for (i = 0; i < arr.length; i++)
        if (arr[i].w === mon.width && arr[i].h === mon.height)
            return arr[i].key;
    return mon.width + "x" + mon.height + "@" + mon.refresh;
}

// one editable display from the backend's output row
function clone(mon) {
    return {
        name: mon.name, modes: mon.modes || [],
        make: mon.make || "", model: mon.model || "",
        width: mon.width, height: mon.height, refresh: mon.refresh,
        physicalWidth: mon.physicalWidth || 0,
        mode: mon.mode || pickCurrentMode(mon),
        scale: mon.scale, x: mon.x, y: mon.y, transform: mon.transform || 0,
        vrr: mon.vrr === true,
        mirror: mon.mirror && mon.mirror !== "none" ? mon.mirror : "",
        colorMode: mon.colorMode || "srgb", sdrBrightness: mon.sdrBrightness || 1.0,
        disabled: mon.disabled === true
    };
}

// what the backend applies for one display
function spec(m) {
    return {
        name: m.name, enabled: !m.disabled, mode: m.mode,
        scale: m.scale, x: m.x, y: m.y,
        transform: m.transform, vrr: !!m.vrr,
        mirror: m.mirror, colorMode: m.colorMode, sdrBrightness: m.sdrBrightness
    };
}

// a mode either is advertised or was typed in by hand
function isCustom(m) {
    var opts = modeList(m);
    for (var i = 0; i < opts.length; i++)
        if (opts[i].key === m.mode)
            return false;
    return true;
}

// distinct resolutions; a typed one the panel does not list joins as custom
function sizes(m) {
    var seen = {}, out = [], arr = modeList(m);
    for (var i = 0; i < arr.length; i++) {
        var k = arr[i].w + "x" + arr[i].h;
        if (seen[k])
            continue;
        seen[k] = true;
        out.push({ key: k, w: arr[i].w, h: arr[i].h, custom: false });
    }
    var cur = m.width + "x" + m.height;
    if (!seen[cur])
        out.unshift({ key: cur, w: m.width, h: m.height, custom: true });
    return out;
}

// the refresh rates offered at the display's current resolution
function rates(m) {
    var out = [], arr = modeList(m), has = false;
    for (var i = 0; i < arr.length; i++) {
        if (arr[i].w !== m.width || arr[i].h !== m.height)
            continue;
        out.push({ key: arr[i].key, rate: arr[i].rate, custom: false });
        if (arr[i].key === m.mode)
            has = true;
    }
    if (!has) {
        var p = parseMode(m.mode);
        if (p)
            out.unshift({ key: p.key, rate: p.rate, custom: true });
    }
    return out;
}

// whole hertz unless two rates would read the same
function rateText(rate, all) {
    var r = Math.round(rate);
    var clash = (all || []).filter(function (o) { return Math.round(o.rate) === r; }).length > 1;
    return clash ? (Math.round(rate * 100) / 100) + " Hz" : r + " Hz";
}

// the best mode for a resolution: the current rate when it exists there, else the fastest
function modeForSize(m, w, h) {
    var arr = modeList(m), best = null;
    for (var i = 0; i < arr.length; i++) {
        if (arr[i].w !== w || arr[i].h !== h)
            continue;
        if (Math.round(arr[i].rate) === Math.round(m.refresh))
            return arr[i].key;
        if (!best || arr[i].rate > best.rate)
            best = arr[i];
    }
    return best ? best.key : "";
}

// scales that divide the mode into whole logical pixels: k/120, at least 1x,
// never shrinking the desktop below 640 by 360
function ladder(w, h, tf) {
    var rotated = (tf & 1) === 1;
    var lw = rotated ? h : w, lh = rotated ? w : h;
    var out = [];
    for (var k = 30; k <= 720; k++)
        if ((w * 120) % k === 0 && (h * 120) % k === 0) {
            var s = Math.round(k / 120 * 10000) / 10000;
            if (s >= 1 && s <= 3 && (lw / s) >= 640 && (lh / s) >= 360)
                out.push(s);
        }
    return out.length ? out : [1];
}

function scaleLadder(m) {
    return ladder(m.width, m.height, m.transform);
}

function nearestIdx(list, v) {
    var best = 0;
    for (var i = 1; i < list.length; i++)
        if (Math.abs(list[i] - v) < Math.abs(list[best] - v))
            best = i;
    return best;
}

function snapScale(m, s) {
    var l = scaleLadder(m);
    return l[nearestIdx(l, s)];
}

function footW(m) {
    var lw = Math.round(m.width / m.scale), lh = Math.round(m.height / m.scale);
    return (m.transform & 1) ? lh : lw;
}

function footH(m) {
    var lw = Math.round(m.width / m.scale), lh = Math.round(m.height / m.scale);
    return (m.transform & 1) ? lw : lh;
}

// a scale from the panel's pixel density, snapped to the mode's ladder
function dpiScale(m) {
    var mm = m.physicalWidth || 0, px = m.width || 0, target = 1.0;
    if (mm > 0 && px > 0) {
        var dpi = px * 25.4 / mm;
        if (dpi < 40 || dpi > 700) target = 1.0;
        else if (dpi <= 120) target = 1.0;
        else if (dpi <= 150) target = 1.25;
        else if (dpi <= 230) target = 1.5;
        else if (dpi <= 290) target = 1.75;
        else target = 2.0;
    }
    return snapScale(m, target);
}

// rotation keeps a flipped panel flipped
function rotation(t) { return (t || 0) & 3; }
function withRotation(t, r) { return ((t || 0) & 4) | (r & 3); }

// brightness of everyday content while HDR is on
var sdrLadder = [1.0, 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9, 2.0,
    2.25, 2.5, 2.75, 3.0, 3.5, 4.0, 4.5, 5.0, 5.5, 6.0];

// minutes from midnight as the clock reads them
function clockText(mins, h24) {
    mins = ((Math.round(mins) % 1440) + 1440) % 1440;
    var h = Math.floor(mins / 60), m = mins % 60;
    var mm = (m < 10 ? "0" : "") + m;
    if (h24)
        return (h < 10 ? "0" : "") + h + ":" + mm;
    var h12 = h % 12 === 0 ? 12 : h % 12;
    return h12 + ":" + mm + (h >= 12 ? " PM" : " AM");
}

// "9", "9:30", "9:30 pm", "21:30" -> minutes, or -1
function parseClock(text) {
    var m = String(text).trim().toLowerCase().match(/^(\d{1,2})(?::(\d{1,2}))?\s*(?:([ap])\.?m?\.?)?$/);
    if (!m)
        return -1;
    var h = parseInt(m[1], 10), mm = m[2] === undefined ? 0 : parseInt(m[2], 10);
    if (mm > 59)
        return -1;
    if (m[3] !== undefined) {
        if (h < 1 || h > 12)
            return -1;
        h = h % 12 + (m[3] === "p" ? 12 : 0);
    } else if (h > 23) {
        return -1;
    }
    return h * 60 + mm;
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { parseMode, modeList, pickCurrentMode, clone, spec, isCustom, sizes, rates, rateText,
        modeForSize, ladder, scaleLadder, nearestIdx, snapScale, footW, footH, dpiScale, rotation,
        withRotation, sdrLadder, clockText, parseClock };
}

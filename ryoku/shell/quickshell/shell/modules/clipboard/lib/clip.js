// words for people stay in QML, this only reads, groups and searches entries

var LINK = /^(?:https?|ftp):\/\/[^\s/$.?#][^\s]*$/i;
var WWW = /^www\.[^\s.\/]+\.[^\s]+$/i;
var HEX = /^#(?:[0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})$/i;
var RGB = /^rgba?\(\s*(\d{1,3})\s*[, ]\s*(\d{1,3})\s*[, ]\s*(\d{1,3})\s*(?:[,\/]\s*([\d.]+)(%?)\s*)?\)$/i;
var HSL = /^hsla?\(\s*([\d.]+)(?:deg)?\s*[, ]\s*([\d.]+)%\s*[, ]\s*([\d.]+)%\s*(?:[,\/]\s*([\d.]+)(%?)\s*)?\)$/i;
var PATH = /^(?:file:\/\/)?(?:\/|~\/)\S/;
var CODE_LINE = /^\s*(?:(?:if|for|while|switch|catch)\s*\(|(?:def|class|fn|func|function|impl|struct|enum|interface)\s+[\w$]+|(?:import|from|use|package|#include)\s+[\w.\/<"'*{@]|#!\/|(?:const|let|var|val)\s+[\w$]+\s*[:=]|return\b.*;\s*$|\/\/\s)/;
var COMMANDS = [
    "sudo", "doas", "git", "gh", "npm", "npx", "pnpm", "yarn", "bun", "deno", "node", "cargo", "rustup",
    "go", "make", "cmake", "meson", "ninja", "python", "python3", "pip", "pipx", "uv", "docker", "podman",
    "kubectl", "systemctl", "journalctl", "loginctl", "pacman", "paru", "yay", "makepkg", "dnf", "rpm",
    "xbps-install", "xbps-query", "flatpak", "ryoku", "ryoku-shell", "ssh", "scp",
    "rsync", "curl", "wget", "cd", "ls", "cat", "rm", "mv", "cp", "mkdir", "chmod", "chown", "ln", "grep",
    "rg", "find", "fd", "sed", "awk", "tar", "unzip", "ffmpeg", "yt-dlp", "nvim", "vim", "export",
    "source", "echo", "kill", "pkill", "wl-copy", "wl-paste", "qs", "env", "bash", "sh", "fish", "zsh"
];
// words that are also plain English need a flag, a path or a pipe to read as a command
var EVERYDAY = ["go", "make", "find", "source", "echo", "export", "kill", "cat", "env", "uv", "cd", "ln", "fd", "sh"];
var SHELLISH = /\s-{1,2}[\w-]|[\/~]|\s(?:\||&&|>>?|2>)\s|=|\$\(/;

function lines(text) {
    var all = String(text || "").replace(/\r\n?/g, "\n").split("\n");
    while (all.length > 0 && all[all.length - 1].trim().length === 0)
        all.pop();
    return all;
}

function filled(text) {
    return lines(text).map(function(line) { return line.trim(); }).filter(function(line) { return line.length > 0; });
}

function firstLine(text) {
    var all = filled(text);
    return all.length > 0 ? all[0].replace(/\s+/g, " ") : "";
}

// the daemon cuts a preview to 200 characters of a payload it counts in bytes
function utf8Length(text) {
    var s = String(text || "");
    var n = 0;
    for (var i = 0; i < s.length; i++) {
        var c = s.charCodeAt(i);
        if (c < 0x80) {
            n += 1;
        } else if (c < 0x800) {
            n += 2;
        } else if (c >= 0xd800 && c <= 0xdbff && i + 1 < s.length) {
            n += 4;
            i++;
        } else {
            n += 3;
        }
    }
    return n;
}

function clamp255(v) {
    return Math.max(0, Math.min(255, Math.round(v)));
}

function hex2(v) {
    var s = clamp255(v).toString(16);
    return s.length < 2 ? "0" + s : s;
}

function hslToRgb(h, s, l) {
    h = ((h % 360) + 360) % 360 / 360;
    s = Math.max(0, Math.min(1, s));
    l = Math.max(0, Math.min(1, l));
    if (s === 0)
        return [l * 255, l * 255, l * 255];
    var q = l < 0.5 ? l * (1 + s) : l + s - l * s;
    var p = 2 * l - q;
    function tone(t) {
        if (t < 0)
            t += 1;
        if (t > 1)
            t -= 1;
        if (t < 1 / 6)
            return p + (q - p) * 6 * t;
        if (t < 1 / 2)
            return q;
        if (t < 2 / 3)
            return p + (q - p) * (2 / 3 - t) * 6;
        return p;
    }
    return [tone(h + 1 / 3) * 255, tone(h) * 255, tone(h - 1 / 3) * 255];
}

function rgbToHsl(r, g, b) {
    r /= 255;
    g /= 255;
    b /= 255;
    var max = Math.max(r, g, b), min = Math.min(r, g, b);
    var l = (max + min) / 2;
    if (max === min)
        return { h: 0, s: 0, l: Math.round(l * 100) };
    var d = max - min;
    var s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
    var h = max === r ? (g - b) / d + (g < b ? 6 : 0) : max === g ? (b - r) / d + 2 : (r - g) / d + 4;
    return { h: Math.round(h * 60) % 360, s: Math.round(s * 100), l: Math.round(l * 100) };
}

function alphaOf(value, percent) {
    if (value === undefined || value === "")
        return 1;
    var a = Number(value);
    return Math.max(0, Math.min(1, percent === "%" ? a / 100 : a));
}

// a short form needs a letter or one repeated digit, so an issue number like #712 stays text
function colour(text) {
    var t = String(text || "").trim();
    var r, g, b, a = 1, m;
    if (HEX.test(t)) {
        var h = t.slice(1);
        if (h.length <= 4 && !/[a-f]/i.test(h) && !/^(\d)\1+$/.test(h))
            return null;
        if (h.length <= 4)
            h = h.split("").map(function(c) { return c + c; }).join("");
        r = parseInt(h.slice(0, 2), 16);
        g = parseInt(h.slice(2, 4), 16);
        b = parseInt(h.slice(4, 6), 16);
        if (h.length === 8)
            a = parseInt(h.slice(6, 8), 16) / 255;
    } else if ((m = RGB.exec(t))) {
        if (Number(m[1]) > 255 || Number(m[2]) > 255 || Number(m[3]) > 255)
            return null;
        r = Number(m[1]);
        g = Number(m[2]);
        b = Number(m[3]);
        a = alphaOf(m[4], m[5]);
    } else if ((m = HSL.exec(t))) {
        var rgb = hslToRgb(Number(m[1]), Number(m[2]) / 100, Number(m[3]) / 100);
        r = clamp255(rgb[0]);
        g = clamp255(rgb[1]);
        b = clamp255(rgb[2]);
        a = alphaOf(m[4], m[5]);
    } else {
        return null;
    }
    var hsl = rgbToHsl(r, g, b);
    return {
        value: t,
        hex: "#" + hex2(r) + hex2(g) + hex2(b) + (a < 1 ? hex2(a * 255) : ""),
        r: r, g: g, b: b, a: Math.round(a * 100) / 100,
        h: hsl.h, s: hsl.s, l: hsl.l
    };
}

function link(text) {
    var t = String(text || "").trim();
    if (t.indexOf("\n") >= 0 || !(LINK.test(t) || WWW.test(t)))
        return null;
    var url = WWW.test(t) ? "https://" + t : t;
    var m = /^[a-z]+:\/\/([^\/?#]+)(.*)$/i.exec(url);
    if (!m)
        return null;
    var host = m[1].replace(/^www\./i, "").toLowerCase();
    var rest = m[2].replace(/\/+$/, "");
    return { url: url, host: host, path: rest, shown: host + rest };
}

function baseName(path) {
    var p = String(path || "").replace(/\/+$/, "");
    var at = p.lastIndexOf("/");
    return at >= 0 ? p.slice(at + 1) : p;
}

function dirName(path) {
    var p = String(path || "").replace(/\/+$/, "");
    var at = p.lastIndexOf("/");
    return at > 0 ? p.slice(0, at) : at === 0 ? "/" : "";
}

function decodeUri(path) {
    var p = String(path).replace(/^file:\/\//, "");
    try {
        return decodeURIComponent(p);
    } catch (e) {
        return p;
    }
}

// a lone plain path with a space in it reads as a sentence about a path, not a file
function files(text) {
    var all = filled(text);
    if (all.length === 0)
        return null;
    for (var i = 0; i < all.length; i++) {
        if (!PATH.test(all[i]))
            return null;
    }
    if (all.length === 1 && /\s/.test(all[0]) && !/^file:\/\//.test(all[0]))
        return null;
    var paths = all.map(decodeUri);
    return { paths: paths, names: paths.map(baseName), dir: dirName(paths[0]) };
}

function command(text) {
    var all = filled(text);
    if (all.length === 0 || all.length > 12)
        return false;
    for (var i = 0; i < all.length; i++) {
        if (/^\$\s+\S/.test(all[i]))
            continue;
        var parts = all[i].split(/\s+/);
        var word = parts[0];
        if (/^\.{1,2}\//.test(word))
            continue;
        if (COMMANDS.indexOf(word) < 0 || parts.length < 2)
            return false;
        if (EVERYDAY.indexOf(word) >= 0 && !SHELLISH.test(all[i]))
            return false;
    }
    return true;
}

function code(text) {
    var all = lines(text);
    if (all.length < 2)
        return false;
    var indented = 0, braced = 0, worded = 0;
    for (var i = 0; i < all.length; i++) {
        var line = all[i];
        if (/^(?:\t| {2,})\S/.test(line))
            indented++;
        if (/[{};]\s*$/.test(line) || /^\s*[}\])]/.test(line))
            braced++;
        if (CODE_LINE.test(line))
            worded++;
    }
    return (worded >= 1 && (indented >= 1 || braced >= 1)) || worded >= 2 || braced >= 2;
}

function read(entry) {
    var e = entry || {};
    var kind = String(e.kind || "");
    var size = Number(e.size) || 0;
    var mime = String(e.mime || "");
    if (kind === "image")
        return { kind: "image", title: "", size: size, w: Number(e.w) || 0, h: Number(e.h) || 0, mime: mime, mono: false };
    if (kind !== "text")
        return { kind: "binary", title: mime, size: size, mime: mime, mono: false };
    var text = String(e.preview || "");
    var out = {
        kind: "text", text: text, title: firstLine(text), size: size, mime: mime, mono: false,
        lines: lines(text).length, chars: text.length, cut: size > utf8Length(text)
    };
    var l = link(text);
    if (l) {
        out.kind = "link";
        out.link = l;
        out.title = l.shown;
        return out;
    }
    var c = colour(text);
    if (c) {
        out.kind = "colour";
        out.colour = c;
        out.title = c.value;
        return out;
    }
    var f = files(text);
    if (f) {
        out.kind = "files";
        out.files = f;
        out.title = f.names[0];
        return out;
    }
    if (command(text)) {
        out.kind = "command";
        out.mono = true;
    } else if (code(text)) {
        out.kind = "code";
        out.mono = true;
    }
    return out;
}

// kind words search also matches, none for plain text, whose mime would match "text" everywhere
function keywords(entry, info) {
    var mime = String(entry && entry.mime || "");
    if (info.kind === "image")
        return [mime, "image", "picture", "screenshot"];
    if (info.kind === "binary")
        return [mime];
    if (info.kind === "link")
        return ["link", "url", info.link.host];
    if (info.kind === "colour")
        return ["colour", "color", info.colour.hex];
    if (info.kind === "files")
        return ["file", "files", "folder", info.files.dir];
    if (info.kind === "command")
        return ["command", "terminal"];
    if (info.kind === "code")
        return ["code"];
    return [];
}

// Qt's regex engine has no \p{L}, so past ASCII a letter is anything outside the punctuation, symbol, space and surrogate blocks
var WORD_CLASS = "0-9A-Za-z\\u00c0-\\u00d6\\u00d8-\\u00f6\\u00f8-\\u1fff\\u2070-\\u209f\\u20d0-\\u218f\\u2c00-\\u2dff"
    + "\\u2e80-\\u2fff\\u3040-\\ud7ff\\ue000-\\ufe2f\\ufe70-\\ufeff\\uff10-\\uff19\\uff21-\\uff3a\\uff41-\\uff5a\\uff66-\\uffef";
var WORD_RUN = new RegExp("[" + WORD_CLASS + "]+", "g");
var WORD_CHAR = new RegExp("^[" + WORD_CLASS + "]$");
// a run that needs a closer look for words inside it: camelCase, or anything past ASCII
var INNER = /[a-z][A-Z]|[^\x00-\x7f]/;

function wordChar(c) {
    return WORD_CHAR.test(c);
}

// Thai, Lao, Myanmar, Khmer, kana and Han write words without spaces, so any character can start one
function unspaced(code) {
    return (code >= 0x0e00 && code <= 0x0eff) || (code >= 0x1000 && code <= 0x109f) || (code >= 0x1780 && code <= 0x17ff)
        || (code >= 0x3040 && code <= 0x30ff) || (code >= 0x3400 && code <= 0x4dbf) || (code >= 0x4e00 && code <= 0x9fff)
        || (code >= 0xf900 && code <= 0xfaff);
}

function lowered(raw) {
    var low = raw.toLowerCase();
    if (low.length === raw.length)
        return low;
    var out = "";
    for (var i = 0; i < raw.length; i++) {
        var c = raw.charAt(i).toLowerCase();
        out += c.length === 1 ? c : raw.charAt(i);
    }
    return out;
}

// camelCase parts and each character of a script written without spaces start words too, so typing a part finds it
function field(text) {
    var raw = String(text || "");
    var starts = [];
    WORD_RUN.lastIndex = 0;
    for (var m = WORD_RUN.exec(raw); m !== null; m = WORD_RUN.exec(raw)) {
        var at = m.index;
        var run = m[0];
        starts.push(at);
        if (!INNER.test(run))
            continue;
        for (var i = 1; i < run.length; i++) {
            var c = run.charAt(i);
            var p = run.charAt(i - 1);
            if (unspaced(c.charCodeAt(0)) || (p !== p.toUpperCase() && c !== c.toLowerCase()))
                starts.push(at + i);
        }
    }
    return { raw: raw, low: lowered(raw), starts: starts };
}

function wordEnd(f, at) {
    var i = at;
    while (i < f.raw.length && wordChar(f.raw.charAt(i)))
        i++;
    return i;
}

// title is what the row shows, body the rest of what was copied, a picture's body the words read in it
function prepare(entry, info, words) {
    var body = info.kind === "image" ? String(words || "")
        : info.kind === "files" ? info.files.names.join("\n") + "\n" + info.files.dir
        : info.kind === "binary" ? String(info.mime || "")
        : String(info.text || "");
    return {
        title: field(info.kind === "image" || info.kind === "binary" ? "" : info.title),
        keys: field(keywords(entry, info).join(" ")),
        body: field(body)
    };
}

function terms(query) {
    var out = [];
    var parts = String(query || "").toLowerCase().split(/\s+/);
    for (var i = 0; i < parts.length; i++) {
        if (parts[i].length > 0 && out.indexOf(parts[i]) < 0)
            out.push(parts[i]);
    }
    return out;
}

function startHit(f, term) {
    for (var i = 0; i < f.starts.length; i++) {
        if (f.low.startsWith(term, f.starts[i]))
            return f.starts[i];
    }
    return -1;
}

// one edit apart: a letter added, dropped, changed, or two neighbours swapped
function oneEdit(a, b) {
    if (a === b)
        return true;
    var la = a.length, lb = b.length;
    if (Math.abs(la - lb) > 1)
        return false;
    var i = 0;
    while (i < la && i < lb && a.charAt(i) === b.charAt(i))
        i++;
    if (la > lb)
        return a.slice(i + 1) === b.slice(i);
    if (la < lb)
        return a.slice(i) === b.slice(i + 1);
    if (a.slice(i + 1) === b.slice(i + 1))
        return true;
    return a.charAt(i) === b.charAt(i + 1) && a.charAt(i + 1) === b.charAt(i) && a.slice(i + 2) === b.slice(i + 2);
}

// a slip of the finger, or of the reader, against the start of a word beginning with the same letter
function typoHits(f, term, every) {
    var out = [];
    var first = term.charAt(0);
    for (var i = 0; i < f.starts.length; i++) {
        var s = f.starts[i];
        if (f.low.charAt(s) !== first)
            continue;
        var room = wordEnd(f, s) - s;
        for (var k = term.length - 1; k <= term.length + 1; k++) {
            if (k >= 1 && k <= room && oneEdit(term, f.low.substr(s, k))) {
                out.push({ at: s, len: k });
                if (!every)
                    return out;
                break;
            }
        }
    }
    return out;
}

function typoHit(f, term) {
    var hits = typoHits(f, term, false);
    return hits.length > 0 ? hits[0] : null;
}

// the letters in order, the first at a word start, for short titles only
function scatterHit(f, term) {
    if (f.raw.length > 40 || term.length < 2)
        return null;
    for (var i = 0; i < f.starts.length; i++) {
        if (f.low.charAt(f.starts[i]) !== term.charAt(0))
            continue;
        var spans = [{ start: f.starts[i], len: 1 }];
        var at = f.starts[i] + 1;
        for (var j = 1; j < term.length && spans !== null; j++) {
            var next = f.low.indexOf(term.charAt(j), at);
            if (next < 0) {
                spans = null;
            } else {
                spans.push({ start: next, len: 1 });
                at = next + 1;
            }
        }
        if (spans !== null)
            return spans;
    }
    return null;
}

// tier 0 starts a title or kind word, 1 starts a word further in or sits inside one, 2 is one edit off, 3 scattered
function hit(ix, term) {
    var at = startHit(ix.title, term);
    if (at >= 0)
        return { where: "title", tier: 0, spans: [{ start: at, len: term.length }] };
    if (startHit(ix.keys, term) >= 0)
        return { where: "keys", tier: 0, spans: [] };
    if (term.length >= 3 && (at = ix.title.low.indexOf(term)) >= 0)
        return { where: "title", tier: 1, spans: [{ start: at, len: term.length }] };
    if ((at = startHit(ix.body, term)) >= 0 || (term.length >= 3 && (at = ix.body.low.indexOf(term)) >= 0))
        return { where: "body", tier: 1, spans: [{ start: at, len: term.length }] };
    if (term.length >= 5) {
        var t = typoHit(ix.title, term);
        if (t)
            return { where: "title", tier: 2, spans: [{ start: t.at, len: t.len }] };
        if (typoHit(ix.keys, term))
            return { where: "keys", tier: 2, spans: [] };
        if ((t = typoHit(ix.body, term)))
            return { where: "body", tier: 2, spans: [{ start: t.at, len: t.len }] };
    }
    var scattered = scatterHit(ix.title, term);
    return scattered ? { where: "title", tier: 3, spans: scattered } : null;
}

// a line around the first word found, with every word found in it marked once
function excerpt(f, first, list) {
    var raw = f.raw;
    var lineStart = raw.lastIndexOf("\n", first.start - 1) + 1;
    var lineEnd = raw.indexOf("\n", first.start);
    if (lineEnd < 0)
        lineEnd = raw.length;
    var from = Math.max(lineStart, first.start - 28);
    var to = Math.min(lineEnd, first.start + first.len + 60);
    for (var n = 0; n < 12 && from > lineStart && wordChar(raw.charAt(from - 1)); n++)
        from--;
    for (var m = 0; m < 12 && to < lineEnd && wordChar(raw.charAt(to)); m++)
        to++;
    var lead = from > lineStart ? "\u2026" : "";
    var spans = [];
    var mark = function(start, len) {
        for (var k = 0; k < spans.length; k++) {
            if (spans[k].start === start && spans[k].len === len)
                return;
        }
        spans.push({ start: start, len: len });
    };
    if (first.start >= from && first.start + first.len <= to)
        mark(lead.length + first.start - from, first.len);
    var low = f.low.slice(from, to);
    for (var i = 0; i < list.length; i++) {
        var at = low.indexOf(list[i]);
        if (at >= 0)
            mark(lead.length + at, list[i].length);
    }
    return { text: lead + raw.slice(from, to) + (to < lineEnd ? "\u2026" : ""), spans: spans, line: lineStart };
}

// a line holding more of the words typed, so the excerpt shows them together
function densest(f, list, first) {
    var best = first;
    var most = 1;
    for (var from = 0; from <= f.low.length;) {
        var end = f.low.indexOf("\n", from);
        if (end < 0)
            end = f.low.length;
        var line = f.low.slice(from, end);
        var count = 0;
        var at = -1;
        for (var i = 0; i < list.length; i++) {
            var p = line.indexOf(list[i]);
            if (p >= 0) {
                count++;
                if (at < 0 || p < at)
                    at = p;
            }
        }
        if (count > most) {
            most = count;
            var word = list.filter(function(t) { return line.indexOf(t) === at; })[0];
            best = { start: from + at, len: word.length };
        }
        from = end + 1;
    }
    return best;
}

// a title outranks the body, and a match the row cannot show, in the body or far along a long title, comes with an excerpt
function match(ix, list) {
    if (!ix || list.length === 0)
        return null;
    var score = 0;
    var title = [];
    var inBody = null;
    var late = null;
    for (var i = 0; i < list.length; i++) {
        var h = hit(ix, list[i]);
        if (!h)
            return null;
        score += h.tier + (h.where === "body" ? 0.5 : 0);
        if (h.where === "title") {
            title = title.concat(h.spans);
            if (late === null && h.spans[0].start >= 40)
                late = h.spans[0];
        } else if (h.where === "body" && inBody === null) {
            inBody = h.spans[0];
        }
    }
    if (list.length > 1 && ix.title.low.indexOf(list.join(" ")) >= 0)
        score = Math.max(0, score - 1);
    if (inBody && list.length > 1)
        inBody = densest(ix.body, list, inBody);
    var cut = inBody ? excerpt(ix.body, inBody, list) : late ? excerpt(ix.title, late, list) : null;
    return { score: score, title: title, excerpt: cut, inBody: inBody !== null };
}

// short words only where a word starts, or every letter would light up, and a long one one edit off when it is nowhere as typed
function marks(text, list) {
    var f = field(text);
    var out = [];
    for (var i = 0; i < list.length; i++) {
        var t = list[i];
        var before = out.length;
        if (t.length >= 3) {
            for (var at = f.low.indexOf(t); at >= 0; at = f.low.indexOf(t, at + t.length))
                out.push({ start: at, len: t.length });
        } else {
            for (var k = 0; k < f.starts.length; k++) {
                if (f.low.startsWith(t, f.starts[k]))
                    out.push({ start: f.starts[k], len: t.length });
            }
        }
        if (out.length === before && t.length >= 5) {
            var slips = typoHits(f, t, true);
            for (var j = 0; j < slips.length; j++)
                out.push({ start: slips[j].at, len: slips[j].len });
        }
    }
    return out;
}

function escaped(text) {
    return String(text).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

// rich text with the spans underlined, "" when there is nothing to mark
function markup(text, spans) {
    var t = String(text || "");
    var list = (spans || []).filter(function(s) { return s.len > 0 && s.start >= 0 && s.start < t.length; })
        .sort(function(a, b) { return a.start - b.start; });
    if (list.length === 0)
        return "";
    var out = "";
    var at = 0;
    for (var i = 0; i < list.length; i++) {
        var start = Math.max(at, list[i].start);
        var end = Math.min(t.length, list[i].start + list[i].len);
        if (end <= start)
            continue;
        out += escaped(t.slice(at, start)) + "<u>" + escaped(t.slice(start, end)) + "</u>";
        at = end;
    }
    return out + escaped(t.slice(at));
}

function ago(at, now) {
    var t = Number(at) || 0;
    if (t <= 0)
        return null;
    var d = Math.max(0, now - t);
    if (d < 45000)
        return { unit: "now", n: 0 };
    if (d < 3600000)
        return { unit: "min", n: Math.max(1, Math.round(d / 60000)) };
    if (d < 86400000)
        return { unit: "hour", n: Math.floor(d / 3600000) };
    var days = Math.floor(d / 86400000);
    return days < 7 ? { unit: "day", n: days } : { unit: "date", n: 0 };
}

function midnight(now) {
    var d = new Date(now);
    d.setHours(0, 0, 0, 0);
    return d.getTime();
}

function day(at, now) {
    var t = Number(at) || 0;
    if (t <= 0)
        return "";
    var today = midnight(now);
    if (t >= today)
        return "today";
    return t >= midnight(today - 1) ? "yesterday" : "earlier";
}

// an entry with no time joins the group before it, the daemon's order being newest first
function groups(entries, now) {
    var order = ["today", "yesterday", "earlier"];
    var byDay = { today: [], yesterday: [], earlier: [] };
    var list = entries || [];
    var known = false;
    var last = "earlier";
    for (var i = 0; i < list.length; i++) {
        var d = day(list[i] && list[i].at, now);
        if (d.length > 0) {
            known = true;
            last = d;
        }
        byDay[d.length > 0 ? d : last].push(list[i]);
    }
    if (!known)
        return list.length > 0 ? [{ id: "all", rows: list.slice() }] : [];
    var out = [];
    for (var k = 0; k < order.length; k++) {
        if (byDay[order[k]].length > 0)
            out.push({ id: order[k], rows: byDay[order[k]] });
    }
    return out;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = {
        lines: lines, firstLine: firstLine, utf8Length: utf8Length, colour: colour, link: link,
        files: files, command: command, code: code, read: read, keywords: keywords,
        prepare: prepare, terms: terms, match: match, markup: markup, marks: marks, oneEdit: oneEdit,
        ago: ago, day: day, groups: groups, baseName: baseName, dirName: dirName
    };

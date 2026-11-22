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

// words the search matches besides the text itself, so "link" or "png" finds them
function keywords(entry, info) {
    var words = [String(entry && entry.mime || ""), info.kind];
    if (info.kind === "image")
        words.push("image", "picture", "screenshot");
    else if (info.kind === "link")
        words.push("link", "url", info.link.host);
    else if (info.kind === "colour")
        words.push("colour", "color", info.colour.hex);
    else if (info.kind === "files")
        words.push("file", "files", "folder", info.files.dir);
    else if (info.kind === "command")
        words.push("command", "terminal");
    else if (info.kind === "code")
        words.push("code");
    return words;
}

function searchable(entry, info) {
    var name = info.kind === "image" || info.kind === "binary" ? String(entry && entry.mime || "")
        : info.kind === "files" ? info.files.names.join(" ")
        : String(info.text || "").replace(/\s+/g, " ").trim();
    return { name: name, keywords: keywords(entry, info) };
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
        searchable: searchable, ago: ago, day: day, groups: groups, baseName: baseName, dirName: dirName
    };

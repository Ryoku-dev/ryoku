// The Downloads view's logic: which section a transfer belongs to, what Enter
// does to it, the piece map's numbers, a pasted link's choices and the steps a
// setting moves in. Labels arrive translated from QML.

var MOVING = ["resolving", "active", "merging", "seeding"];
var WAITING = ["queued", "paused"];

// a seeding torrent has landed: it sits with what is finished, still sending
function bucket(state) {
    if (state === "seeding")
        return "finished";
    if (MOVING.indexOf(state) >= 0)
        return "active";
    return WAITING.indexOf(state) >= 0 ? "waiting" : "finished";
}

// tone names the mineral a state wears: "" moving, "paused", "done", "error",
// and "idle" for what has not started or was stopped.
function tone(state) {
    if (state === "paused")
        return "paused";
    if (state === "error")
        return "error";
    if (state === "done" || state === "seeding")
        return "done";
    return state === "queued" || state === "stopped" ? "idle" : "";
}

function stamp(text) {
    var value = Date.parse(String(text || ""));
    return isNaN(value) ? 0 : value;
}

// a file is known by its name on disk, a video or a post by its title until it has one
function nameOf(task) {
    return String(task.name || task.title || task.url || "");
}

// glyph: the Material Symbol a transfer leads with, by what it is.
function glyph(task) {
    if (task.state === "error")
        return "error";
    if (task.kind === "torrent")
        return "hub";
    var name = String(task.name || task.title || "").toLowerCase();
    var mime = String(task.mime || "");
    if (task.mode === "audio" || mime.indexOf("audio/") === 0 || /\.(mp3|m4a|opus|ogg|flac|wav)$/.test(name))
        return "music_note";
    if (task.kind === "media" || mime.indexOf("video/") === 0 || /\.(mp4|mkv|webm|mov|avi)$/.test(name))
        return "movie";
    if (mime.indexOf("image/") === 0 || /\.(png|jpe?g|gif|webp|avif|svg)$/.test(name))
        return "image";
    if (/\.(iso|img)$/.test(name))
        return "album";
    if (/\.(zip|rar|7z|xz|gz|bz2|zst|tar|tgz)$/.test(name))
        return "folder_zip";
    return "draft";
}

// matches: every typed word appears somewhere in the transfer's name, site,
// link or state.
function matches(task, query) {
    var words = String(query || "").toLowerCase().split(/\s+/).filter(function(word) { return word.length > 0; });
    if (words.length === 0)
        return true;
    var hay = [task.title, task.name, task.url, task.source ? task.source.name : "", task.state].join(" ").toLowerCase();
    for (var i = 0; i < words.length; i++) {
        if (hay.indexOf(words[i]) < 0)
            return false;
    }
    return true;
}

// group sorts the daemon's list into what is moving, what waits and what is
// over (newest first), keeping the daemon's order inside the first two.
function group(tasks, query) {
    var out = { active: [], waiting: [], finished: [] };
    var list = tasks || [];
    for (var i = 0; i < list.length; i++) {
        if (matches(list[i], query))
            out[bucket(String(list[i].state || ""))].push(list[i]);
    }
    out.finished.sort(function(a, b) {
        return stamp(b.finished || b.created) - stamp(a.finished || a.created);
    });
    return out;
}

// primary: what Enter does to a transfer.
function primary(task) {
    var state = String(task ? task.state || "" : "");
    if (state === "done")
        return "open";
    if (state === "error")
        return "retry";
    if (state === "paused" || state === "stopped")
        return "resume";
    return state.length > 0 ? "pause" : "";
}

// can: which of the view's actions apply to a transfer in its state.
function can(task) {
    var state = String(task ? task.state || "" : "");
    var over = state === "done" || state === "error" || state === "stopped";
    return {
        pause: MOVING.indexOf(state) >= 0 || state === "queued",
        resume: state === "paused" || state === "stopped",
        retry: state === "error",
        open: state === "done",
        reveal: state.length > 0,
        cancel: state.length > 0 && !over && state !== "seeding",
        remove: over || state === "seeding",
        removeFile: state === "done",
        link: String(task ? task.url || "" : "").length > 0
    };
}

// blend: the fills a map shows part-way (t) between two sets of 64.
function blend(from, to, t) {
    var out = [];
    for (var i = 0; i < 64; i++) {
        var a = Number(from[i] || 0);
        out.push(a + (Number(to[i] || 0) - a) * t);
    }
    return out;
}

function same(a, b) {
    for (var i = 0; i < 64; i++) {
        if (Math.abs(Number(a[i] || 0) - Number(b[i] || 0)) > 0.0005)
            return false;
    }
    return true;
}

// trace turns up to `slots` samples (oldest first) into points across 0..1 with
// the newest at the right edge, and the largest of them.
function trace(samples, slots) {
    var list = samples || [];
    var n = Math.max(2, slots || 60);
    var from = Math.max(0, list.length - n);
    var points = [];
    var peak = 0;
    for (var i = from; i < list.length; i++) {
        var value = Math.max(0, Number(list[i]) || 0);
        peak = Math.max(peak, value);
        points.push({ x: (n - (list.length - i)) / (n - 1), y: value });
    }
    return { points: points, peak: peak };
}

// ceiling rounds a rate up to a scale a reader can name: 1, 2 or 5 times a
// power of two-to-the-ten unit step.
function ceiling(peak) {
    var value = Math.max(1024, Number(peak) || 0);
    var unit = 1;
    while (value / unit >= 1024)
        unit *= 1024;
    var steps = [1, 2, 5, 10, 20, 50, 100, 200, 500, 1000];
    for (var i = 0; i < steps.length; i++) {
        if (steps[i] * unit >= value)
            return steps[i] * unit;
    }
    return 1024 * unit;
}

// streams: a link's pictures, one per height (the first the engine lists for
// it), tallest first whatever order the site gave them in, and its first
// sound-only stream.
function streams(resolved) {
    var formats = (resolved && resolved.formats) || [];
    var videos = [];
    var audio = null;
    var seen = {};
    for (var i = 0; i < formats.length; i++) {
        var f = formats[i];
        if (f.kind === "audio") {
            if (!audio)
                audio = f;
        } else if (!seen[f.label]) {
            seen[f.label] = true;
            videos.push({ format: f, tall: parseInt(f.label, 10) || Number(f.height) || 0, order: videos.length });
        }
    }
    videos.sort(function(a, b) { return b.tall - a.tall || a.order - b.order; });
    return { videos: videos.map(function(entry) { return entry.format; }), audio: audio };
}

// chips: the choices a resolved link offers. The first lets the daemon pick by
// the reader's settings (limit: the most picture height they allow, 0 for any),
// then one per picture height (its best stream), then sound alone.
// Each {id, kind:"best"|"video"|"audio", label, detail, format, mode, size}.
function chips(resolved, limit) {
    var formats = (resolved && resolved.formats) || [];
    var read = streams(resolved);
    var videos = read.videos;
    var audio = read.audio;
    if (videos.length === 0 && !audio)
        return [];
    var sound = audio ? Math.max(0, Number(audio.size) || 0) : 0;
    var out = [];
    if (videos.length > 0) {
        var chosen = null;
        for (var d = 0; d < formats.length; d++) {
            if (resolved.defaultFormat && formats[d].id === resolved.defaultFormat && formats[d].kind !== "audio")
                chosen = formats[d];
        }
        for (var c = 0; c < videos.length && !chosen; c++) {
            if (!(limit > 0) || !(parseInt(videos[c].label, 10) > limit))
                chosen = videos[c];
        }
        if (!chosen)
            chosen = videos[videos.length - 1];
        out.push({ id: "best", kind: "best", label: "", detail: String(chosen.label || ""), format: "", mode: "auto", size: sized(chosen.size, sound) });
        for (var v = 0; v < videos.length && v < 3; v++)
            out.push({ id: "v:" + videos[v].id, kind: "video", label: String(videos[v].label || ""), detail: "", format: String(videos[v].id), mode: "auto", size: sized(videos[v].size, sound) });
    }
    if (audio)
        out.push({ id: "audio", kind: "audio", label: "", detail: String(audio.ext || ""), format: "", mode: "audio", size: sound > 0 ? sound : -1 });
    return out;
}

// The batch card's four choices for all links, and what each one means for one
// resolved link. batchChoices: the distinct outcomes a link offers, each with
// the batch choices it answers to ({id, kind, label, detail, format, mode, size,
// answers:[...]}); empty for a plain file or a torrent. A link whose best
// picture is 720p answers "best", "1080" and "720" with that one stream; a link
// with sound only answers every choice with it.
var BATCH = ["best", "1080", "720", "audio"];

function batchChoices(resolved, limit) {
    var read = streams(resolved);
    var videos = read.videos;
    var audio = read.audio;
    if (videos.length === 0 && !audio)
        return [];
    var sound = audio ? Math.max(0, Number(audio.size) || 0) : 0;
    var soundOnly = { id: "audio", kind: "audio", label: "", detail: String(audio ? audio.ext || "" : ""), format: "", mode: "audio", size: sound > 0 ? sound : -1, answers: ["audio"] };
    if (videos.length === 0) {
        soundOnly.answers = BATCH.slice();
        return [soundOnly];
    }
    function under(height) {
        for (var v = 0; v < videos.length; v++) {
            if (!(height > 0) || !(parseInt(videos[v].label, 10) > height))
                return videos[v];
        }
        return videos[videos.length - 1];
    }
    var best = under(limit);
    var out = [{ id: "best", kind: "best", label: "", detail: String(best.label || ""), format: "", mode: "auto", size: sized(best.size, sound), answers: ["best"], stream: String(best.id) }];
    var heights = [1080, 720];
    for (var h = 0; h < heights.length; h++) {
        var pick = under(heights[h]);
        var held = null;
        for (var o = 0; o < out.length; o++) {
            if (out[o].stream === String(pick.id))
                held = out[o];
        }
        if (held)
            held.answers.push(String(heights[h]));
        else
            out.push({ id: String(heights[h]), kind: "video", label: String(pick.label || ""), detail: "", format: String(pick.id), mode: "auto", size: sized(pick.size, sound), answers: [String(heights[h])], stream: String(pick.id) });
    }
    out.push(soundOnly);
    return out;
}

// batchPick: the index of the choice a link takes for one of BATCH.
function batchPick(choices, wanted) {
    for (var i = 0; i < choices.length; i++) {
        if (choices[i].answers.indexOf(wanted) >= 0)
            return i;
    }
    return 0;
}

// batchNext: which waiting rows to read now, keeping at most `limit` in flight.
function batchNext(states, limit) {
    var flying = 0;
    var out = [];
    for (var i = 0; i < states.length; i++) {
        if (states[i] === "resolving")
            flying++;
    }
    for (var w = 0; w < states.length && flying + out.length < limit; w++) {
        if (states[w] === "waiting")
            out.push(w);
    }
    return out;
}

// batchItem: one row as an `addMany` item. choice is the row's own outcome (null
// for a file, a torrent, or a link not read yet, which then only takes "audio");
// only names the one entry of a list to fetch, -1 for all of it; title is the
// name already read, so a task added paused is not called by its link.
function batchItem(url, choice, wanted, only, title) {
    var item = { url: String(url) };
    if (choice) {
        if (choice.mode !== "auto")
            item.mode = choice.mode;
        if (choice.format.length > 0)
            item.format = choice.format;
    } else if (wanted === "audio") {
        item.mode = "audio";
    }
    if (only >= 0)
        item.files = [only];
    if (String(title || "").trim().length > 0)
        item.title = String(title).trim();
    return item;
}

// named: which entry of its list a link itself names (a video opened from a
// playlist), ignoring the parameters that only say where it was opened from;
// -1 when the link is the list.
function named(url, items) {
    var bare = function(text) { return String(text || "").replace(/[?&](list|index|pp|start_radio|t)=[^&#]*/g, "").replace(/[?&]$/, ""); };
    var list = items || [];
    for (var i = 0; i < list.length; i++) {
        if (bare(list[i].url) === bare(url))
            return i;
    }
    return -1;
}

function sized(picture, sound) {
    var p = Number(picture) || 0;
    return p > 0 ? p + sound : -1;
}

// a checklist's ticked indices, kept in order.
function everything(n) {
    var out = [];
    for (var i = 0; i < n; i++)
        out.push(i);
    return out;
}

function toggle(selected, index) {
    var list = selected.slice();
    var at = list.indexOf(index);
    if (at >= 0)
        list.splice(at, 1);
    else
        list.push(index);
    list.sort(function(a, b) { return a - b; });
    return list;
}

function selectedSize(files, selected) {
    var total = 0;
    var known = false;
    for (var i = 0; i < files.length; i++) {
        if (selected.indexOf(i) < 0)
            continue;
        var size = Number(files[i].size) || 0;
        if (size > 0) {
            total += size;
            known = true;
        }
    }
    return known ? total : -1;
}

// the steps a rate limit moves in, KiB/s; 0 is off.
var LIMITS = [0, 128, 256, 512, 1024, 2048, 5120, 10240, 20480, 51200, 102400];

// step moves a value to its neighbour in an ordered list, staying inside it; a
// value between two steps goes to the nearer one in that direction.
function step(list, value, delta) {
    var at = list.indexOf(value);
    if (at < 0) {
        at = 0;
        for (var i = 0; i < list.length; i++) {
            if (Number(list[i]) <= Number(value))
                at = i;
        }
        if (delta < 0 && Number(list[at]) < Number(value))
            at++;
    }
    return list[Math.max(0, Math.min(list.length - 1, at + (delta < 0 ? -1 : 1)))];
}

function clamp(value, low, high) {
    return Math.max(low, Math.min(high, value));
}

// number reads a typed figure; null when it is not one or falls outside.
function number(text, low, high, whole) {
    var raw = String(text || "").trim().replace(",", ".");
    if (!/^\d+(\.\d+)?$/.test(raw))
        return null;
    var value = Number(raw);
    if (whole && Math.floor(value) !== value)
        return null;
    return value < low || value > high ? null : value;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = {
        bucket: bucket, tone: tone, stamp: stamp, nameOf: nameOf, glyph: glyph, matches: matches, group: group,
        primary: primary, can: can, blend: blend, same: same, trace: trace, ceiling: ceiling,
        chips: chips, BATCH: BATCH, batchChoices: batchChoices, batchPick: batchPick, batchNext: batchNext,
        batchItem: batchItem, named: named, everything: everything, toggle: toggle, selectedSize: selectedSize,
        LIMITS: LIMITS, step: step, clamp: clamp, number: number
    };

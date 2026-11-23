import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const D = require("./downloads.js");

function equal(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        console.error(label, "\nexpected", expected, "\nactual", actual);
        process.exitCode = 1;
    }
}

const tasks = [
    { id: "a", title: "Sprite Fright", state: "active", source: { name: "YouTube" }, url: "https://youtu.be/x" },
    { id: "b", name: "debian.iso", state: "queued", url: "https://cdimage.debian.org/debian.iso" },
    { id: "c", name: "old.zip", state: "done", finished: "2026-11-18T10:00:00Z" },
    { id: "d", name: "blender.tar.xz", state: "paused" },
    { id: "e", name: "post", state: "error", finished: "2026-11-18T11:00:00Z" },
    { id: "f", name: "arch.iso", state: "seeding", finished: "2026-11-18T12:30:00Z" },
    { id: "g", name: "new.zip", state: "done", finished: "2026-11-18T12:00:00Z" }
];
const ids = list => list.map(task => task.id);
equal(ids(D.lanes(tasks, "ISO", "all").waiting), ["b"], "a filter matches the name without case");
equal(ids(D.lanes(tasks, "youtube sprite", "all").active), ["a"], "every word must match, in any field");
equal(ids(D.lanes(tasks, "paused", "all").waiting), ["d"], "a state can be typed");
equal(D.lanes(tasks, "nothing-here", "all"), { active: [], waiting: [], failed: [], done: [] }, "a filter can empty the list");

const mixed = tasks.concat([
    { id: "h", name: "cut.mkv", state: "stopped", finished: "2026-11-18T13:00:00Z" },
    { id: "i", name: "reading", state: "resolving" },
    { id: "j", name: "glue.mp4", state: "merging" }
]);
equal(["resolving", "active", "merging", "queued", "paused", "seeding", "done", "error", "stopped"].map(D.lane),
    ["active", "active", "active", "waiting", "waiting", "done", "done", "failed", "failed"], "a stopped download sits with what failed, a seeding one with what is done");
equal(D.counts(mixed), { all: 10, active: 3, waiting: 2, done: 3, failed: 2 }, "each filter counts its own, all counts everything");
equal(D.counts([]), { all: 0, active: 0, waiting: 0, done: 0, failed: 0 }, "an empty queue counts nothing");
const all = D.lanes(mixed, "", "all");
equal([ids(all.active), ids(all.waiting), ids(all.failed), ids(all.done)], [["a", "i", "j"], ["b", "d"], ["h", "e"], ["f", "g", "c"]],
    "four groups: moving and waiting in the daemon's order, failed and done newest first");
const failedOnly = D.lanes(mixed, "", "failed");
equal([failedOnly.active.length, failedOnly.waiting.length, ids(failedOnly.failed), failedOnly.done.length], [0, 0, ["h", "e"], 0], "a filter keeps one group");
equal(ids(D.lanes(mixed, "zip", "done").done), ["g", "c"], "typed words narrow a filter");
equal(ids(D.lanes(mixed, "zip", "unknown").done), ["g", "c"], "an unknown filter shows everything");
equal(D.idsIn(mixed, ["error", "stopped"]), ["e", "h"], "a group's ids, for acting on it at once");

equal(D.pace([]), { average: 0, peak: 0 }, "no samples, no pace");
equal(D.pace([0, 0, 100, 300, 200]), { average: 200, peak: 300 }, "the quiet before the start is not averaged");
equal(D.pace([100, 0, 200]), { average: 100, peak: 200 }, "a stall after the start is");
equal(D.pace([0, 0, 0]), { average: 0, peak: 0 }, "a minute of nothing");

equal(["active", "queued", "merging", "seeding", "paused", "stopped", "error", "done", ""].map(state => D.primary({ state })),
    ["pause", "pause", "pause", "pause", "resume", "resume", "retry", "open", ""], "Enter follows the state");
equal(["resolving", "paused", "done", "error", "queued", "stopped"].map(D.tone), ["", "paused", "done", "error", "idle", "idle"], "a state wears one mineral");
const can = D.can({ state: "done", url: "https://x" });
equal([can.open, can.remove, can.removeFile, can.cancel, can.pause, can.link], [true, true, true, false, false, true], "a finished file can be opened and removed, not cancelled");
equal([D.can({ state: "seeding" }).cancel, D.can({ state: "seeding" }).remove], [false, true], "a seeding torrent is whole: it is removed, not cancelled");
equal([D.can({ state: "active" }).cancel, D.can({ state: "active" }).remove, D.can({ state: "error" }).retry], [true, false, true], "a moving transfer is cancelled, a failed one retried");

equal([{ state: "error" }, { kind: "torrent" }, { kind: "media", mode: "audio" }, { kind: "media" }, { name: "debian.ISO" }, { name: "x.tar.xz" }, { name: "notes.txt" }].map(D.glyph),
    ["error", "hub", "music_note", "movie", "album", "folder_zip", "draft"], "a transfer leads with what it is");

const empty = Array(64).fill(0);
const full = Array(64).fill(1);
equal(D.blend(empty, full, 0.25)[10], 0.25, "a blend is linear");
equal([D.same(empty, empty), D.same(empty, full)], [true, false], "equal maps are recognised");

const short = D.trace([100, 300, 200], 60);
equal([short.points.length, short.points[2].x, short.peak], [3, 1, 300], "a short trace ends at the right edge");
equal(Math.round(short.points[0].x * 59), 57, "and starts as many samples back as it has");
equal(D.trace(Array.from({ length: 80 }, (_, i) => i), 60).points.length, 60, "a long trace keeps the newest samples");
equal([D.ceiling(0), D.ceiling(900 * 1024), D.ceiling(41 * 1048576), D.ceiling(1048576)], [1024, 1000 * 1024, 50 * 1048576, 1048576], "a scale rounds up to a round rate");

const resolved = {
    defaultFormat: "",
    formats: [
        { id: "271", kind: "video", label: "1080p", size: 1000, ext: "webm" },
        { id: "137", kind: "video", label: "1080p", size: 800, ext: "mp4" },
        { id: "247", kind: "video", label: "720p", size: 500, ext: "webm" },
        { id: "251", kind: "audio", label: "137 kbps", size: 90, ext: "webm" },
        { id: "140", kind: "audio", label: "129 kbps", size: 80, ext: "m4a" }
    ]
};
const chips = D.chips(resolved);
equal(chips.map(chip => chip.id), ["best", "v:271", "v:247", "audio"], "one chip per height, led by the daemon's pick and closed by sound alone");
equal(chips.map(chip => chip.size), [1090, 1090, 590, 90], "a picture's size includes its sound");
equal([chips[0].format, chips[1].format, chips[3].mode], ["", "271", "audio"], "only an explicit height names a format");
equal([D.chips(resolved, 720)[0].detail, D.chips(resolved, 720)[0].size, D.chips(resolved, 0)[0].detail, D.chips(resolved, 144)[0].detail], ["720p", 590, "1080p", "720p"],
    "the daemon's pick is sized by the most the settings allow");
equal(D.chips({ formats: [] }), [], "a plain file has no choices");
equal(D.chips({ formats: [{ id: "a", kind: "audio", label: "128 kbps", size: 0, ext: "mp3" }] }).map(chip => [chip.id, chip.size]), [["audio", -1]], "an unknown size stays unknown");

equal(D.toggle(D.everything(3), 1), [0, 2], "unticking one of all leaves the rest");
equal(D.toggle([0, 2], 1), [0, 1, 2], "ticking keeps the order");
equal(D.selectedSize([{ size: 10 }, { size: 20 }, { size: 0 }], [0, 2]), 10, "a selection sums what it knows");
equal(D.selectedSize([{ size: 0 }], [0]), -1, "nothing known is unknown");

equal([D.step(D.LIMITS, 0, 1), D.step(D.LIMITS, 0, -1), D.step(D.LIMITS, 102400, 1), D.step(D.LIMITS, 300, 1), D.step(D.LIMITS, 300, -1)],
    [128, 0, 102400, 512, 256], "a limit steps through its list and stops at the ends");
equal([D.loosen(0, 1), D.loosen(0, -1), D.loosen(102400, 1), D.loosen(128, -1), D.loosen(300, 1), D.loosen(300, -1), D.loosen(512, 1)],
    [0, 102400, 0, 128, 512, 256, 1024], "one download's limit eases up to none and tightens from none to the top step");
equal([D.loosen(50, -1), D.loosen(50, 1)], [50, 128], "lowering a limit under the smallest step leaves it, raising takes the smallest step");
equal([D.number("6881", 0, 65535, true), D.number("1,5", 0, 100, false), D.number("70000", 0, 65535, true), D.number("1.5", 0, 10, true), D.number("abc", 0, 9, true)],
    [6881, 1.5, null, null, null], "a typed figure is checked against its range");

const tall = { formats: [
    { id: "313", kind: "video", label: "2160p", size: 4000 },
    { id: "271", kind: "video", label: "1440p", size: 2000 },
    { id: "248", kind: "video", label: "1080p", size: 1000 },
    { id: "247", kind: "video", label: "720p", size: 500 },
    { id: "251", kind: "audio", label: "137 kbps", size: 90, ext: "webm" }
] };
const tallChoices = D.batchChoices(tall, 0);
equal(tallChoices.map(choice => [choice.id, choice.format, choice.answers.join("+")]),
    [["best", "", "best"], ["1080", "248", "1080"], ["720", "247", "720"], ["audio", "", "audio"]], "a tall video offers each batch choice its own stream");
equal(tallChoices.map(choice => choice.size), [4090, 1090, 590, 90], "each with its size and its sound");
const low = D.batchChoices(resolved, 0);
equal(low.map(choice => [choice.id, choice.answers.join("+")]), [["best", "best+1080"], ["720", "720"], ["audio", "audio"]], "a 1080p video answers Best and 1080p with one stream");
equal(D.BATCH.map(wanted => low[D.batchPick(low, wanted)].id), ["best", "best", "720", "audio"], "a batch choice lands on the link's nearest outcome");
equal(D.batchChoices(resolved, 720).map(choice => [choice.id, choice.detail || choice.label, choice.answers.join("+")]),
    [["best", "720p", "best+720"], ["1080", "1080p", "1080"], ["audio", "webm", "audio"]], "the settings' limit decides what Best is");
const song = D.batchChoices({ formats: [{ id: "a", kind: "audio", label: "128 kbps", size: 70, ext: "mp3" }] }, 0);
equal([song.length, song[0].mode, D.batchPick(song, "1080")], [1, "audio", 0], "a link with sound only takes Audio whatever was chosen");
equal(D.batchChoices({ formats: [] }, 0), [], "a file or a torrent has no choice to take");

equal(D.batchNext(["ready", "resolving", "waiting", "waiting", "waiting", "waiting", "failed"], 4), [2, 3, 4], "rows are read four at a time");
equal(D.batchNext(["resolving", "resolving", "resolving", "resolving", "waiting"], 4), [], "a full flight starts nothing");
equal(D.batchItem("https://a/b", tallChoices[1], "1080", -1), { url: "https://a/b", format: "248" }, "a picked height names its stream");
equal(D.batchItem("https://a/b", tallChoices[3], "audio", 0), { url: "https://a/b", mode: "audio", files: [0] }, "sound alone is a mode, and one entry of a list is named by its place");
equal(D.batchItem("https://a/b", tallChoices[0], "best", -1), { url: "https://a/b" }, "Best leaves the pick to the daemon");
equal([D.batchItem("magnet:?x", null, "720", -1), D.batchItem("https://a/c", null, "audio", -1)], [{ url: "magnet:?x" }, { url: "https://a/c", mode: "audio" }], "a link with no choices is added as it is");
equal([D.batchItem("https://a/b", null, "best", -1, " Spring "), D.batchItem("https://a/b", null, "best", -1, "")], [{ url: "https://a/b", title: "Spring" }, { url: "https://a/b" }], "a name already read travels with the link");
const entries = [{ url: "https://www.youtube.com/watch?v=aaa" }, { url: "https://www.youtube.com/watch?v=bbb" }];
equal([D.named("https://www.youtube.com/watch?v=bbb&list=PL1&index=2", entries), D.named("https://www.youtube.com/playlist?list=PL1", entries), D.named("https://www.youtube.com/watch?v=zzz&list=PL1", entries)],
    [1, -1, -1], "a video opened from a playlist names its own entry; the playlist names none");

const rising = { defaultFormat: "mp4-2", formats: [
    { id: "mp4-0", kind: "video", label: "320p", height: 320, size: 0 },
    { id: "mp4-1", kind: "video", label: "540p", height: 540, size: 0 },
    { id: "mp4-2", kind: "video", label: "720p", height: 720, size: 0 }
] };
equal(D.batchChoices(rising, 0).map(choice => [choice.id, choice.detail || choice.label, choice.answers.join("+")]),
    [["best", "720p", "best+1080+720"], ["audio", "", "audio"]], "a site that lists its smallest picture first still gives its tallest as Best");
equal(D.chips(rising, 0).map(chip => [chip.id, chip.detail || chip.label]), [["best", "720p"], ["v:mp4-2", "720p"], ["v:mp4-1", "540p"], ["v:mp4-0", "320p"]], "and its chips run tallest first");

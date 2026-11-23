import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const C = require("./clip.js");

function equal(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        console.error(label, "\nexpected", expected, "\nactual", actual);
        process.exitCode = 1;
    }
}

const text = (preview, size) => ({ kind: "text", mime: "text/plain;charset=utf-8", preview, size: size === undefined ? Buffer.byteLength(preview) : size });
const kind = preview => C.read(text(preview)).kind;

// links
equal(C.read(text("https://github.com/Ryoku-dev/ryoku/pull/12/")).link, { url: "https://github.com/Ryoku-dev/ryoku/pull/12/", host: "github.com", path: "/Ryoku-dev/ryoku/pull/12", shown: "github.com/Ryoku-dev/ryoku/pull/12" }, "a link reads as host and path");
equal(C.read(text("www.example.org/a")).link.url, "https://www.example.org/a", "a bare www address gains its scheme");
equal(kind("see https://example.org for more"), "text", "a sentence holding a link stays text");
equal(kind("https://a.example\nhttps://b.example"), "text", "two links on two lines stay text");

// colours
equal(C.colour("#FF8800").hex, "#ff8800", "a six-digit hex");
equal(C.colour("#f80").hex, "#ff8800", "a short hex with a letter");
equal(C.colour("#000").hex, "#000000", "a short hex of one repeated digit");
equal(kind("#712"), "text", "an issue number stays text");
equal(C.colour("#ff880080").a, 0.5, "an eight-digit hex keeps its alpha");
equal([C.colour("rgb(255, 136, 0)").hex, C.colour("rgba(255 136 0 / 50%)").a], ["#ff8800", 0.5], "rgb and rgba, comma or space");
equal(C.colour("rgb(300, 0, 0)"), null, "a channel past 255 is not a colour");
equal(C.colour("hsl(32, 100%, 50%)").hex, "#ff8800", "hsl converts");
equal([C.colour("#ff8800").h, C.colour("#ff8800").s, C.colour("#ff8800").l], [32, 100, 50], "hsl figures for the preview");

// commands, and English that only starts like one
equal(kind("sudo pacman -Syu"), "command", "a command");
equal(kind("git status"), "command", "a two-word command");
equal(kind("$ ls -la"), "command", "a prompted line");
equal(kind("./build.sh --release"), "command", "a relative script");
equal(kind("go to the store first"), "text", "everyday words need a flag, path or pipe");
equal(kind("find the file you need"), "text", "find in a sentence is text");
equal(kind("find . -name '*.qml'"), "command", "find with a flag is a command");
equal(kind("git"), "text", "a lone word is text");
equal(kind("cd ~/Work && make -j8"), "command", "a chain");
equal(C.read(text("npm run build")).mono, true, "commands are set in mono");

// code, and prose that starts like it
equal(kind("function f() {\n  return 1;\n}"), "code", "braced and indented code");
equal(kind("import os\nimport sys"), "code", "two import lines");
equal(kind("if you want it\nfor real, say so"), "text", "lowercase prose is not code");
equal(kind("Dear team,\n  thanks for the help.\nBest"), "text", "an indented letter is not code");

// files
equal(C.read(text("/home/nero/Pictures/a.png\n/home/nero/Pictures/b.png")).files, { paths: ["/home/nero/Pictures/a.png", "/home/nero/Pictures/b.png"], names: ["a.png", "b.png"], dir: "/home/nero/Pictures" }, "file manager paths");
equal(C.read(text("file:///home/nero/My%20Docs/report.pdf")).files.names, ["report.pdf"], "a file URI decodes");
equal(C.read(text("file:///home/nero/My%20Docs/report.pdf")).files.dir, "/home/nero/My Docs", "and its folder");
equal(kind("/etc/fstab is wrong"), "text", "a sentence about a path is text");
equal(kind("~/Work/hyperion"), "files", "a home path");

// text figures and cut previews
const long = "x".repeat(200);
equal(C.read(text(long, 4300)).cut, true, "a preview shorter than its payload is cut");
equal(C.read(text("héllo")).cut, false, "multibyte text counts in bytes, not cut");
equal(C.read(text("one\ntwo\n\n")).lines, 2, "trailing blank lines do not count");
equal(C.read(text("  \n  hello   world  \nnext")).title, "hello world", "the title is the first filled line, spaced once");
equal(C.read({ kind: "image", mime: "image/png", size: 2048, w: 1920, h: 1080 }), { kind: "image", title: "", size: 2048, w: 1920, h: 1080, mime: "image/png", mono: false }, "an image keeps its pixel size");
equal(C.read({ kind: "binary", mime: "application/pdf", size: 9 }).kind, "binary", "anything else is binary");

// search: what each kind is found by
const picture = { id: 1, kind: "image", mime: "image/png" };
equal(C.keywords(picture, C.read(picture)), ["image/png", "image", "picture", "screenshot"], "a picture by its kind and type");
equal(C.keywords(text("https://ryoku.dev"), C.read(text("https://ryoku.dev"))), ["link", "url", "ryoku.dev"], "a link by its host");
equal(C.keywords(text("hello there"), C.read(text("hello there"))), [], "plain text by nothing but itself");

// search: every word typed must be found, a title's word starts first, then the newest
const item = (e, words) => ({ e, ix: C.prepare(e, C.read(e), words) });
const find = (query, items) => items.map((x, at) => ({ id: x.e.id, at, m: C.match(x.ix, C.terms(query)) }))
    .filter(x => x.m).sort((a, b) => a.m.score - b.m.score || a.at - b.at).map(x => x.id);
const shot = item({ id: 1, kind: "image", mime: "image/png" }, "Invoice #4471 Total 82.40\nDue Friday 14:00");
const note = item({ id: 2, ...text("Friday plans: invoice the studio") });
const other = item({ id: 3, ...text("lunch with the team on monday") });
const grain = item({ id: 4, kind: "image", mime: "image/jpeg" }, "");
const all = [shot, note, other, grain];
equal(find("friday", all), [2, 1], "a word in a picture finds it, after a title that starts with it");
equal(find("due friday", all), [1], "every word must be found");
equal(find("4471", all), [1], "numbers in a picture");
equal(find("fdy", [shot, grain]), [], "letters scattered through a picture's words find nothing");
equal(find("invocie", all), [2, 1], "one swapped pair of letters still finds the word");
equal(find("fryday", all), [2, 1], "so does one wrong letter");
equal(find("dux", all), [], "short words need to be spelled right");
equal(find("image", all), [1, 4], "pictures by their kind");
equal(find("jpeg", all), [4], "and by their type");
equal(find("plain", [other]), [], "plain text is not found by its type");
equal(find("udio", all), [2], "three letters inside a word");
equal(find("ud", all), [], "two letters only at the start of one");

const short = item({ id: 5, ...text("git commit --amend") });
const wordy = item({ id: 6, ...text("a very long sentence that goes past forty characters with git and commit in it") });
equal(find("gca", [short, wordy]), [5], "scattered letters, only in a short title");
equal(C.match(short.ix, C.terms("gca")).score, 3, "and below every other kind of match");

const camel = item({ id: 7, ...text("ClipPalette.qml") });
equal(C.match(camel.ix, C.terms("pal")).title, [{ start: 4, len: 3 }], "camelCase parts start words");
const station = item({ id: 8, kind: "image", mime: "image/png" }, "東京駅 cafe");
equal(find("駅", [station]), [8], "one CJK character is a word");
const scripts = item({ id: 11, kind: "image", mime: "image/png" }, "مرحبا بالعالم\nनमस्ते दुनिया\nสวัสดีครับ\n“quoted”\u2013dash…");
equal([find("بالع", [scripts]), find("दुनि", [scripts]), find("ครับ", [scripts])], [[11], [11], [11]], "words start in scripts without case, and anywhere in Thai");
equal(C.match(scripts.ix, C.terms("quoted")).score, 1.5, "a word after a curly quote starts a word");

// search: the part of a picture or text that matched, marked, for the row and the preview
equal(C.match(shot.ix, C.terms("friday")).excerpt, { text: "Due Friday 14:00", spans: [{ start: 4, len: 6 }], line: 26 }, "the matched line of the picture, and where it starts");
const essay = item({ id: 9, ...text("We met at the old harbour before sunrise, then walked along the coast to the lighthouse where the keeper showed us the lens and told us about the storms that shaped the coast for a hundred years") });
const cut = C.match(essay.ix, C.terms("keeper")).excerpt;
equal([cut.text.startsWith("\u2026"), cut.text.endsWith("\u2026"), cut.text.slice(cut.spans[0].start, cut.spans[0].start + 6)], [true, true, "keeper"], "a long line is cut around the match");
const screen = item({ id: 10, kind: "image", mime: "image/png" }, "HOME ORPHEUS\nContinue in Style\nAPPEARANCE WINDOW STYLE ORPHEUS 13:04");
equal(C.match(screen.ix, C.terms("orpheus style")).excerpt.text, "APPEARANCE WINDOW STYLE ORPHEUS 13:04", "the line holding the most of the words typed");
equal(C.match(note.ix, C.terms("friday")).excerpt, null, "a title match needs no excerpt");
equal(C.markup("a<b>&c", [{ start: 2, len: 1 }]), "a&lt;<u>b</u>&gt;&amp;c", "marks escape the rest");
equal(C.marks("Due Friday, friday at 4", ["friday", "4", "du"]), [{ start: 4, len: 6 }, { start: 12, len: 6 }, { start: 22, len: 1 }, { start: 0, len: 2 }], "every place a word shows up, short words at word starts only");
equal(C.marks("Appearance and more Appearance", ["apperance"]), [{ start: 0, len: 10 }, { start: 20, len: 10 }], "a misspelt word marks every word it found");
equal(C.markup("abcdef", [{ start: 1, len: 3 }, { start: 2, len: 3 }]), "a<u>bcd</u><u>e</u>f", "overlapping marks do not repeat letters");
equal(C.markup("abc", []), "", "nothing to mark");

equal([C.oneEdit("invoice", "invoce"), C.oneEdit("invoice", "invocie"), C.oneEdit("invoice", "invoise"), C.oneEdit("invoice", "invoicee")], [true, true, true, true], "one edit of each kind");
equal(C.oneEdit("invoice", "involve"), false, "two edits are too many");

// time
const now = new Date(2026, 10, 22, 15, 0, 0).getTime();
equal(C.ago(0, now), null, "no time, no age");
equal(C.ago(now - 10000, now), { unit: "now", n: 0 }, "just now");
equal(C.ago(now - 5 * 60000, now), { unit: "min", n: 5 }, "minutes");
equal(C.ago(now - 3 * 3600000 - 100, now), { unit: "hour", n: 3 }, "hours");
equal(C.ago(now - 2 * 86400000, now), { unit: "day", n: 2 }, "days");
equal(C.ago(now - 9 * 86400000, now), { unit: "date", n: 0 }, "a week on, a date");
equal(C.day(new Date(2026, 10, 22, 0, 0, 1).getTime(), now), "today", "after midnight is today");
equal(C.day(new Date(2026, 10, 21, 23, 59).getTime(), now), "yesterday", "before it, yesterday");
equal(C.day(new Date(2026, 10, 20, 12, 0).getTime(), now), "earlier", "then earlier");

// groups
const at = (d, h) => new Date(2026, 10, d, h, 0).getTime();
const list = [{ id: 1, at: at(22, 14) }, { id: 2, at: at(22, 9) }, { id: 3 }, { id: 4, at: at(21, 20) }, { id: 5, at: at(19, 8) }, { id: 6 }];
equal(C.groups(list, now).map(g => [g.id, g.rows.map(r => r.id)]), [["today", [1, 2, 3]], ["yesterday", [4]], ["earlier", [5, 6]]], "entries without a time join the group before them");
equal(C.groups([{ id: 1 }, { id: 2 }], now), [{ id: "all", rows: [{ id: 1 }, { id: 2 }] }], "no times, one untitled group");
equal(C.groups([], now), [], "nothing, no groups");

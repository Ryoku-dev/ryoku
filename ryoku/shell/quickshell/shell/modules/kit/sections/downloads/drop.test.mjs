import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const Drop = require("./drop.js");
const Root = require("../../../ask/lib/root.js");

function equal(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        console.error(label, "\nexpected", expected, "\nactual", actual);
        process.exitCode = 1;
    }
}

const magnet = "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=arch";
equal(Drop.read(["https://example.org/a.iso"], "", Root.detectLink), { links: ["https://example.org/a.iso"], files: [], skipped: 0 }, "a dragged link");
equal(Drop.read(["file:///home/me/Downloads/Big%20Buck.torrent"], "", Root.detectLink), { links: [], files: ["/home/me/Downloads/Big Buck.torrent"], skipped: 0 }, "a local torrent file, its name decoded");
equal(Drop.read(["file://laptop/home/me/x.torrent", "file:///home/me/notes.txt"], "", Root.detectLink), { links: [], files: ["/home/me/x.torrent"], skipped: 1 }, "a host before the path is dropped, a file that is no torrent is skipped");
equal(Drop.read([], "https://a.org/1.zip\n" + magnet + "\n# comment\nhttps://a.org/1.zip", Root.detectLink),
    { links: ["https://a.org/1.zip", magnet], files: [], skipped: 0 }, "plain text: one link per word, comments and repeats left out");
equal(Drop.read([], "just some words", Root.detectLink), { links: [], files: [], skipped: 0 }, "words hold no link");
equal(Drop.read(["file:///home/me/x.torrent"], "file:///home/me/x.torrent\nhttps://b.org/x " + magnet, Root.detectLink),
    { links: ["https://b.org/x", magnet], files: ["/home/me/x.torrent"], skipped: 0 }, "the uri list and the text are read together, each entry once");
equal(Drop.read(["file:///home/me/Big%20Buck.torrent"], "/home/me/Big Buck.torrent", Root.detectLink),
    { links: [], files: ["/home/me/Big Buck.torrent"], skipped: 0 }, "a bare path with a space stays one path, never a fake link");
equal(Drop.read(["file:///home/me/My%20Notes.txt"], "/home/me/My Notes.txt", Root.detectLink),
    { links: [], files: [], skipped: 1 }, "a spaced file that is no torrent is not supported, never a fake link");
equal(Drop.read([], "/home/me/My Notes.txt", Root.detectLink),
    { links: [], files: [], skipped: 1 }, "a spaced bare path alone is not supported either");

if (!process.exitCode)
    console.log("download drop: passed");

import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const R = require("./root.js");

function equal(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        console.error(label, "\nexpected", expected, "\nactual", actual);
        process.exitCode = 1;
    }
}

equal(R.prefixRoute("\\why is it slow"), { view: "ask", rest: "why is it slow", external: false }, "a backslash opens Ask with the rest");
equal(R.prefixRoute("? arch wiki"), { view: "web", rest: "arch wiki", external: false }, "a question mark opens Web and drops the gap");
equal(R.prefixRoute("/"), { view: "downloads", rest: "", external: true }, "a slash hands off to Kit Downloads");
equal(R.prefixRoute("/ kernel"), { view: "downloads", rest: "kernel", external: true }, "the Downloads handoff preserves its filter");
equal(R.prefixRoute("plain"), null, "plain text has no prefix");
equal(R.prefixRoute(""), null, "an empty line has no prefix");

equal(R.detectLink("https://www.Example.com/a/b.zip?x=1"), { kind: "url", url: "https://www.Example.com/a/b.zip?x=1", host: "example.com", name: "" }, "a web address keeps its text and loses www in the host");
equal(R.detectLink("example.com/file.iso"), { kind: "url", url: "https://example.com/file.iso", host: "example.com", name: "" }, "a bare domain becomes https");
equal(R.detectLink("https://user:pw@host.tld:8080/x").host, "host.tld", "credentials and port are not the host");
equal(R.detectLink("https://distro.org/x.torrent").kind, "torrent", "a .torrent address is a torrent");
equal(R.detectLink("magnet:?xt=urn:btih:c12fe1c06bba254a9dc9f519b335aa7c1367a88a&dn=Big+Buck%20Bunny"), { kind: "magnet", url: "magnet:?xt=urn:btih:c12fe1c06bba254a9dc9f519b335aa7c1367a88a&dn=Big+Buck%20Bunny", host: "", name: "Big Buck Bunny" }, "a magnet carries its display name");
equal(R.detectLink("magnet:?dn=nothing"), null, "a magnet without a hash is not a link");
equal(R.detectLink("how do I install example.com"), null, "a sentence is not a link");
equal(R.detectLink("1.2.3"), null, "a version number is not a domain");
equal(R.detectLink("readme.md"), { kind: "url", url: "https://readme.md", host: "readme.md", name: "" }, "a two-part name with a letter TLD reads as a domain");
equal(R.detectLink("file"), null, "one word is not a link");
equal(R.detectLink("javascript:alert(1)"), null, "only web, ftp and magnet schemes are links");

equal(["2+2", "12 * (3 - 1)", "15% of 80", "5 km to mi", "sqrt 16", "100 usd in eur", "7 - 2"].map(R.looksLikeMath), [true, true, true, true, true, true, true], "sums and conversions reach the calculator");
equal(["hello", "42", "3.14", "ubuntu 24.04", "example.com/2+2", "install python3"].map(R.looksLikeMath), [false, false, false, false, false, false], "words, bare numbers and links do not");

const commands = [
    { id: "ask", title: "Ask Rashin", keywords: ["question", "ai"] },
    { id: "web", title: "Search the web", keywords: ["internet", "find"] },
    { id: "downloads", title: "Downloads", keywords: ["torrent", "queue"] },
    { id: "rashin", title: "Open Rashin", keywords: ["app"] }
];
equal(R.rank(commands, "").map(c => c.id), ["ask", "web", "downloads", "rashin"], "an empty query keeps the given order");
equal(R.rank(commands, "down").map(c => c.id), ["downloads"], "a title prefix matches");
equal(R.rank(commands, "rashin").map(c => c.id), ["ask", "rashin"], "a word start matches, earlier command first on a tie");
equal(R.rank(commands, "open rashin").map(c => c.id), ["rashin"], "a whole title wins");
equal(R.rank(commands, "torrent").map(c => c.id), ["downloads"], "a keyword matches");
equal(R.rank(commands, "web").map(c => c.id), ["web"], "a word inside the title matches");
equal(R.rank(commands, "zzz"), [], "nothing matches nonsense");
equal(R.matchScore("dl", "Downloads") > 0, true, "letters in order match loosely");
equal(R.matchScore("d", "Open Rashin"), 0, "one loose letter is not a match");

// the home list shows what is moving, then news: what settled and was not seen ten minutes ago
const now = Date.parse("2026-11-22T12:00:00Z");
const tasks = [
    { id: "a", state: "done", finished: "2026-11-22T10:00:00Z" },
    { id: "b", state: "active", started: "2026-11-22T09:00:00Z" },
    { id: "c", state: "error", finished: "2026-11-22T11:00:00Z" },
    { id: "d", state: "queued", created: "2026-11-22T08:00:00Z" },
    { id: "e", state: "done", finished: "2026-11-22T11:30:00Z", seen: "2026-11-22T11:55:00Z" },
    { id: "f", state: "seeding" },
    { id: "g", state: "done", finished: "2026-11-22T11:40:00Z", seen: "2026-11-22T11:45:00Z" },
    { id: "h", state: "error", finished: "2026-11-20T09:00:00Z" },
    { id: "i", state: "paused", started: "2026-11-01T09:00:00Z" },
    { id: "j", state: "stopped", finished: "2026-11-22T11:50:00Z" }
];
equal(R.pickDownloads(tasks, now).map(t => t.id), ["b", "d", "f", "j", "e", "c"], "moving first in the daemon's order, then the three newest pieces of news");
equal(R.pickDownloads(tasks, now, 9).map(t => t.id), ["b", "d", "f", "j", "e", "c", "a", "i"], "seen over ten minutes ago, or settled over a day ago and never seen, is not news; an unseen pause is");
equal(R.pickDownloads(tasks, now, 0).map(t => t.id), ["b", "d", "f"], "the news limit can be zero");
equal(R.pickDownloads(null, now), [], "no task list is an empty one");
equal(R.unseen(tasks, now), ["a", "c", "i", "j"], "every piece of news not yet marked is reported, shown or not");
const marked = tasks.map(t => R.unseen(tasks, now).indexOf(t.id) >= 0 ? Object.assign({}, t, { seen: new Date(now).toISOString() }) : t);
equal(R.pickDownloads(marked, now, 9).map(t => t.id), ["b", "d", "f", "j", "e", "c", "a", "i"], "marking the news brings nothing stale back");
const gone = Object.assign({}, tasks[0], { seen: R.dismissed(now).toISOString() });
equal(R.pickDownloads([gone], now).length, 0, "a dismissed row leaves at once");
const ahead = Object.assign({}, tasks[0], { seen: "2026-11-22T14:00:00Z" });
equal(R.unseen([ahead], now), ["a"], "a seen stamp from a clock that ran ahead is marked again");

const sessions = [
    { id: "s1", turns: 2, lastMessageAt: "2026-11-22T11:00:00Z", updated: "2026-11-22T11:00:00Z" },
    { id: "s2", turns: 0, updated: "2026-11-22T11:30:00Z" },
    { id: "s3", turns: 1, archived: true, lastMessageAt: "2026-11-22T10:00:00Z" },
    { id: "s4", turns: 1, temporary: true, lastMessageAt: "2026-11-22T10:00:00Z" },
    { id: "s5", turns: 3, updated: "2026-11-21T13:00:00Z" },
    { id: "s6", turns: 1, lastMessageAt: "2026-11-19T10:00:00Z", updated: "2026-11-22T11:59:00Z" }
];
const day = 24 * 60 * 60 * 1000;
equal(R.pickAsks(sessions, now, day).map(s => s.id), ["s1", "s5"], "home asks: a turn in the last day, by its last message, never archived, temporary or empty");
equal(R.pickAsks(sessions, now, 0).map(s => s.id), ["s1", "s5", "s6"], "without a window every real conversation counts");
equal(R.pickAsks(sessions, now, day, "s1").map(s => s.id), ["s5"], "an ask waiting to be deleted is already gone");

equal(R.quote("  a   long\nline  "), "a long line", "quoting flattens white space");
equal(R.quote("abcdefghij", 6), "abcde\u2026", "a long quote is cut with an ellipsis");

const labels = { downloads: "Downloads", asks: "Recent asks", goTo: "Go to", results: "Commands", link: "Link", calc: "Calculator", fallbacks: "Use text with" };
const empty = R.sections({ query: "", commands, downloads: [{ id: "task:b" }], asks: [{ id: 1 }, { id: 2 }, { id: 3 }, { id: 4 }], rashin: "ready", labels });
equal(empty.map(s => [s.id, s.rows.length]), [["downloads", 1], ["asks", 3], ["goto", 4]], "an empty line shows downloads, three recent asks and the commands");
equal(R.sections({ query: "", commands, downloads: [], asks: [{ id: 1 }], rashin: "off", labels }).map(s => s.id), ["goto"], "without Rashin or downloads only the commands show");

const typed = R.sections({ query: "down", commands, labels, rashin: "ready" });
equal(typed.map(s => s.id), ["results", "fallbacks"], "typing filters commands and closes with the fallbacks");
equal(typed[1].rows.map(r => [r.target, r.text]), [["ask", "down"], ["web", "down"]], "the fallbacks carry the text");

const linked = R.sections({ query: "https://example.com/a.zip", commands, link: R.detectLink("https://example.com/a.zip"), labels });
equal([linked[0].id, linked[0].title, linked[0].rows[0].kind], ["top", "Link", "link"], "a link leads the list");
equal(linked[linked.length - 1].id, "fallbacks", "a link still closes with the fallbacks");

const sum = R.sections({ query: "2+2", commands, calc: { expr: "2+2", result: "4" }, labels });
equal([sum[0].title, sum[0].rows[0].kind, sum[0].rows[0].calc.result], ["Calculator", "calc", "4"], "a settled sum leads the list");
equal(R.sections({ query: "2+3", commands, calc: { expr: "2+2", result: "4" }, labels })[0].id, "fallbacks", "a result for older text is not shown");

const paste = "https://example.com/a.zip?utm_source=x\nhttps://www.youtube.com/watch?v=abc&si=1\nmagnet:?xt=urn:btih:c12fe1c06bba254a9dc9f519b335aa7c1367a88a&dn=Arch\nwww.kernel.org/pub/linux.tar.xz";
equal(R.countLinks(paste), 4, "four pasted lines are four link starts");
equal(R.countLinks("https://a.example/x https://b.example/y"), 2, "links on one line count apiece");
equal(R.countLinks("https://a.example/x,https://b.example/y"), 2, "a comma between links does not glue them");
equal(R.countLinks("https://www.example.com/a"), 1, "www inside an address is not a second link");
equal(R.countLinks("see www.example.com and (www.kernel.org)"), 2, "bare www hosts count");
equal(R.countLinks("https://l.example/r?u=https%3A%2F%2Fb.example"), 1, "an encoded address inside a link is not a start");
equal(R.countLinks("FTP://files.example/a sftp://box.example/b"), 2, "ftp and sftp count, whatever the case");
equal(R.countLinks("magnet:?xt=urn:btih:c12fe1c06bba254a9dc9f519b335aa7c1367a88a magnet:?xt=urn:btih:d12fe1c06bba254a9dc9f519b335aa7c1367a88b"), 2, "magnets count");
equal(R.countLinks("how do I mount https:// shares"), 0, "a scheme with nothing behind it is not a link");
equal(R.countLinks("awww.example.com, a.www.example.com"), 0, "www inside a word or a host is not a start");
equal(R.countLinks("what is 2 + 2"), 0, "a sentence holds no links");
equal(R.countLinks(""), 0, "an empty line holds no links");
equal(R.countLinks(null), 0, "nothing holds no links");
equal(R.looksLikeMath("https://a.example/1 https://b.example/2"), false, "a paste of links is not a sum");
equal(R.detectLink(paste), null, "several links are not one link");

equal(R.asLink({ url: "https://www.Example.com/a.zip", kind: "url", host: "www.example.com" }), { kind: "url", url: "https://www.Example.com/a.zip", host: "example.com", name: "" }, "an engine link takes the shape of a detected one");
equal(R.asLink({ url: "magnet:?xt=urn:btih:c12fe1c06bba254a9dc9f519b335aa7c1367a88a&dn=Arch+Linux", kind: "magnet" }).name, "Arch Linux", "an engine magnet keeps its name");

const read = R.batch({
    links: [
        { raw: "https://example.com/a.zip?utm_source=x", url: "https://example.com/a.zip", kind: "url", host: "example.com", notes: ["tracking"], duplicate: false },
        { raw: "https://youtu.be/abc?si=1", url: "https://www.youtube.com/watch?v=abc", kind: "url", host: "www.youtube.com", notes: ["tracking", "short"], duplicate: false },
        { raw: "https://example.com/a.zip?utm_medium=y", url: "https://example.com/a.zip", kind: "url", host: "example.com", notes: ["tracking-removed"], duplicate: true },
        { raw: "example.com/b.zip", url: "https://example.com/b.zip", kind: "url", host: "example.com", notes: [], duplicate: false }
    ],
    total: 4, cleaned: 3, duplicates: 1
});
equal([read.count, read.cleaned, read.duplicates], [3, 2, 1], "a batch counts what is left and what was adjusted, a cleaned duplicate only as a duplicate");
equal(read.urls, ["https://example.com/a.zip", "https://www.youtube.com/watch?v=abc", "https://example.com/b.zip"], "a batch keeps the cleaned links in order, duplicates dropped");
equal(read.hosts, ["example.com", "youtube.com"], "a batch names each site once");
const bare = R.batch({ links: [{ url: "https://a.example/x", kind: "url", notes: ["tracking"] }, { url: "https://a.example/x", kind: "url", duplicate: true }, { url: "", kind: "url" }] });
equal([bare.count, bare.cleaned, bare.duplicates], [1, 1, 1], "a batch counts from the links, not the engine's totals");
equal(R.batch(null), { links: [], urls: [], count: 0, cleaned: 0, duplicates: 0, hosts: [] }, "no answer is an empty batch");

const manyLabels = Object.assign({ links: "Links" }, labels);
const several = R.sections({ query: paste, commands, many: { state: "ready", batch: read }, link: R.detectLink("https://example.com/a.zip"), labels: manyLabels });
equal([several[0].id, several[0].title, several[0].rows.map(r => r.kind)], ["top", "Links", ["many"]], "several links lead the list in place of the single link");
equal(several[several.length - 1].rows.map(r => r.target), ["ask", "web"], "several links still close with the fallbacks");
equal(R.sections({ query: paste, commands, many: null, labels: manyLabels })[0].id, "fallbacks", "a paste the engine found nothing in falls to the fallbacks");

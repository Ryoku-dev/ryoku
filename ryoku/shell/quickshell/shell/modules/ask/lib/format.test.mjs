import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const F = require("./format.js");

function equal(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        console.error(label, "\nexpected", expected, "\nactual", actual);
        process.exitCode = 1;
    }
}

equal([0, 512, 1024, 1536, 10 * 1024, 150 * 1024, 1048576, 3.5 * 1073741824, 2 * 1099511627776].map(F.bytes),
    ["0 B", "512 B", "1.00 KiB", "1.50 KiB", "10.0 KiB", "150 KiB", "1.00 MiB", "3.50 GiB", "2.00 TiB"], "sizes keep three figures");
equal([-1, NaN, undefined, "x"].map(F.bytes), ["", "", "", ""], "an unknown size is empty");
equal(F.bytePair(2.05 * 1073741824, 3.2 * 1073741824), ["2.05", "3.20 GiB"], "landed figures share their unit when both sizes use it");
equal(F.bytePair(1.25 * 1048576, 8 * 1048576), ["1.25", "8.00 MiB"], "small landed figures also share one unit");
equal(F.bytePair(512 * 1024, 8 * 1048576), ["512 KiB", "8.00 MiB"], "different size units both remain visible");
equal(F.speed(2 * 1048576), "2.00 MiB/s", "a rate is a size per second");
equal(F.speed(-1), "", "an unknown rate is empty");

equal(F.eta(42), { unit: "s", a: 42, b: 0 }, "under a minute is seconds");
equal(F.eta(125), { unit: "m", a: 2, b: 5 }, "short minutes keep their seconds");
equal(F.eta(1500), { unit: "m", a: 25, b: 0 }, "long minutes drop the seconds");
equal(F.eta(3 * 3600 + 5 * 60), { unit: "h", a: 3, b: 5 }, "hours keep minutes");
equal(F.eta(2 * 86400 + 3 * 3600), { unit: "d", a: 2, b: 3 }, "days keep hours");
equal([F.eta(-1), F.eta(Infinity), F.eta("x")], [null, null, null], "an unknown time left is null");

equal([F.fraction(50, 200), F.fraction(300, 200), F.fraction(5, -1), F.fraction(5, 0)], [0.25, 1, -1, -1], "a fraction is clamped and unknown without a size");
equal([F.percent(0, 100), F.percent(999, 1000), F.percent(1000, 1000), F.percent(1, -1)], ["0%", "99%", "100%", ""], "a percent only reads 100 when complete");

const map = F.pieces(Buffer.from(Uint8Array.from({ length: 64 }, (_, i) => i < 32 ? 255 : i === 32 ? 128 : 0)).toString("base64"));
equal(map.length, 64, "a piece map has 64 buckets");
equal([map[0], map[31], Math.round(map[32] * 100), map[33], map[63]], [1, 1, 50, 0, 0], "buckets decode to fills");
equal(F.pieces("").length, 64, "a missing piece map is 64 empty buckets");
equal(F.pieces("////").slice(0, 4), [1, 1, 1, 0], "a short map is padded");

const now = Date.parse("2026-11-18T12:00:00Z");
equal(F.ago(now - 20000, now), { unit: "now", n: 0 }, "seconds ago is now");
equal(F.ago(now - 5 * 60000, now), { unit: "m", n: 5 }, "minutes");
equal(F.ago(now - 3 * 3600000, now), { unit: "h", n: 3 }, "hours");
equal(F.ago(now - 3 * 86400000, now), { unit: "d", n: 3 }, "days");
equal(F.ago(now + 60000, now), { unit: "now", n: 0 }, "a clock ahead of us is now");
equal(F.ago(NaN, now), null, "no timestamp is null");

equal([F.duration(59), F.duration(754), F.duration(3725), F.duration(0)], ["0:59", "12:34", "1:02:05", ""], "a running time reads as a clock");
equal([F.count(999), F.count(1200), F.count(34567), F.count(1100000), F.count(25000000), F.count(1000)], ["999", "1.2k", "35k", "1.1M", "25M", "1k"], "tallies shorten");

const sentences = { network: "Couldn't reach the site.", private: "This is private or needs a sign-in.", unavailable: "This isn't available any more.", unsupported: "There's nothing here to download.", rate_limited: "The site is limiting requests. Try again in a minute.", invalid: "That doesn't look like a link.", other: "Something went wrong. Try again." };
equal(["network", "private", "unavailable", "unsupported", "rate_limited", "invalid", "no_space", "browser_check", "checksum", "busy", "internal", "not_found", ""].map(code => F.reasonKey({ code })),
    ["network", "private", "unavailable", "unsupported", "rate_limited", "invalid", "no_space", "browser_check", "checksum", "other", "other", "other", "other"], "each code a reader can act on has its own reason; the rest are one");
equal(F.reason({ code: "network", message: 'network: Get "https://x.invalid/a": dial tcp: lookup x.invalid: no such host' }, sentences), "Couldn't reach the site.", "a row never shows the engine's own words");
equal([F.reason({ code: "busy", message: "busy: try later" }, sentences), F.reason({ code: "internal" }, sentences), F.reason(null, sentences)],
    ["Something went wrong. Try again.", "Something went wrong. Try again.", "Something went wrong. Try again."], "busy, internal and no error at all share one sentence");
equal(F.reason({ code: "engine", message: "The download engine is not installed." }, sentences), "The download engine is not installed.", "the bar's own failure is already a sentence");
equal(F.reason({ code: "private" }, {}), "", "no sentences, no reason");
equal(F.detail({ code: "network", message: 'network: Get "https://x.invalid/a": no such host' }), 'Get "https://x.invalid/a": no such host', "the detail is the engine's words without its code");
equal([F.detail({ code: "private", message: "Only followers can see this" }), F.detail({ code: "internal", message: "" }), F.detail(null), F.detail({ code: "engine", message: "The download engine did not start." })],
    ["Only followers can see this", "", "", ""], "a plain message stays, nothing and the bar's own failure have no detail");

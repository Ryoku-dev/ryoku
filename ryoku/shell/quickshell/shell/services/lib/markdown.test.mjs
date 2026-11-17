import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { prepare, safeUrl } = require("./markdown.js");

test("images never reach the renderer, their alt text stays", () => {
    assert.equal(prepare("see ![the logo](http://tracker.example/p.png) here"), "see the logo here");
    assert.equal(prepare("![](https://x.example/a.png)"), "image");
    assert.equal(prepare("![chart][c]\n\n[c]: http://x.example/c.png").split("\n")[0], "chart");
    assert.equal(prepare("[![badge](http://x.example/b.png)](https://example.com)"), "[badge](https://example.com)");
});

test("soft line breaks become hard ones, paragraphs stay", () => {
    assert.equal(prepare("one\ntwo\n\nthree"), "one  \ntwo\n\nthree");
});

test("fenced code is left as written", () => {
    const code = "```sh\nfoo ![x](y)\nbar\n```";
    assert.equal(prepare("before\n" + code + "\nafter"), "before  \n" + code + "\nafter");
});

test("only web and mail links open", () => {
    assert.equal(safeUrl("https://ryoku.dev/docs"), "https://ryoku.dev/docs");
    assert.equal(safeUrl("HTTP://example.com"), "HTTP://example.com");
    assert.equal(safeUrl("mailto:a@example.com"), "mailto:a@example.com");
    for (const bad of ["file:///etc/passwd", "javascript:alert(1)", "--version", "smb://host/share", "ftp://x", "", "https://a b"])
        assert.equal(safeUrl(bad), "");
});

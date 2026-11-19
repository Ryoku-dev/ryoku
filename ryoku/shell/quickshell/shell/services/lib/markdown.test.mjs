import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { render, parse, bind, safeUrl } = require("./markdown.js");

const style = { px: 14, gap: 8, itemGap: 3, lineHeight: 135, mono: "Martian Mono", monoPx: 12, codeBg: "#222222", codePad: 9, link: "#cc6633", dim: "#999999" };
const NBSP = "\u00a0";

test("images never reach the renderer, their alt text stays", () => {
    assert.match(render("see ![the logo](http://tracker.example/p.png) here", style), />see the logo here<\/p>$/);
    assert.match(render("![](https://x.example/a.png)", style), />image<\/p>$/);
    assert.match(render("![chart][c]\n\n[c]: http://x.example/c.png", style), />chart<\/p>$/);
    assert.doesNotMatch(render("![chart][c]\n\n[c]: http://x.example/c.png", style), /x\.example/);
    const badge = render("[![badge](http://x.example/b.png)](https://example.com)", style);
    assert.match(badge, /<a href="https:\/\/example\.com"[^>]*>badge<\/a>/);
    assert.doesNotMatch(badge, /<img|b\.png/);
});

test("markup in an answer is text, never markup", () => {
    const html = render('<img src="http://x.example/p.png"> & <b>bold</b>', style);
    assert.doesNotMatch(html, /<img|<b>bold/);
    assert.match(html, /&lt;img src=&quot;http:\/\/x\.example\/p\.png&quot;&gt; &amp;/);
});

test("blocks are set a half line apart, the first one flush", () => {
    const html = render("one\n\ntwo\n\n- a\n- b\n\n```\ncode\n```", style);
    assert.equal((html.match(/margin-top:8px/g) || []).length, 3);
    assert.match(html, /^<p style="margin-top:0px; margin-bottom:0px; line-height:135%;">one<\/p>/);
});

test("a line break in the answer is a line break on screen, a blank line a new paragraph", () => {
    assert.match(render("one\ntwo\n\nthree", style), />one<br\/>two<\/p><p [^>]*>three<\/p>$/);
});

test("list items get a little air, nested lists indent a level", () => {
    const html = render("Lead:\n- first\n- second\n  - inner\n- third", style);
    assert.deepEqual(parse("Lead:\n- first\n- second\n  - inner\n- third").map(b => b.t), ["p", "list"]);
    assert.match(html, /<ul style="margin-top:8px; margin-bottom:0px; margin-left:0px; -qt-list-indent:1;"><li style="margin-top:0px;/);
    assert.match(html, /<li style="margin-top:3px; [^>]*>second<\/li><ul style="margin-top:0px; [^>]*-qt-list-indent:2;"><li style="margin-top:2px;[^>]*>inner<\/li><\/ul><li style="margin-top:3px;[^>]*>third<\/li><\/ul>$/);
});

test("numbered lists keep their numbers and their continuation lines", () => {
    const html = render("3. third\n   more of it\n4. fourth", style);
    assert.match(html, /<ol start="3" /);
    assert.match(html, />third<br\/>more of\u00a0it<\/li>/);
});

test("fenced code is left as written, on a tinted padded run", () => {
    const html = render("before\n```sh\nfoo ![x](y) **b**\n  bar <tag>\n```\nafter", style);
    assert.match(html, /<table width="100%" cellspacing="0" cellpadding="9" style="margin-top:8px; margin-bottom:0px;" bgcolor="#222222">/);
    assert.match(html, /white-space:pre-wrap; font-family:'Martian Mono'; font-size:12px;[^>]*>foo !\[x\]\(y\) \*\*b\*\*\n  bar &lt;tag&gt;<\/p>/);
    assert.match(html, />after<\/p>$/);
});

test("a fence still open while the answer streams is already code", () => {
    const html = render("Run:\n```sh\nryoku reload", style);
    assert.match(html, /white-space:pre-wrap[^>]*>ryoku reload<\/p><\/td><\/tr><\/table>$/);
});

test("inline code, emphasis and headings", () => {
    const html = render("## Fix\nUse `ryoku reload` and **wait** a *little* ~~bit~~.", style);
    assert.match(html, /<p style="margin-top:0px; margin-bottom:0px; font-size:16px; font-weight:600;">Fix<\/p>/);
    assert.match(html, /<span style="font-family:'Martian Mono'; font-size:12px; background-color:#222222;">ryoku reload<\/span>/);
    assert.match(render("because **`qs`** has *the `x` flag*", style), /because <b><span [^>]*>qs<\/span><\/b> has <i>the <span [^>]*>x<\/span> flag<\/i>/);
    assert.match(render("keep `**as is** <b>`", style), /<span [^>]*>\*\*as is\*\* &lt;b&gt;<\/span>/);
    assert.match(html, /<b>wait<\/b> a\u00a0<i>little<\/i> <s>bit<\/s>\./);
    assert.match(render("snake_case_name stays", style), />snake_case_name stays<\/p>$/);
});

test("quotes and tables", () => {
    assert.match(render("> said\n> twice", style), /margin-left:15px; color:#999999;[^>]*>said<br\/>twice<\/p>$/);
    const html = render("| Mount | Free |\n|---|---|\n| /home | 212 GiB |", style);
    assert.match(html, /<td style="font-weight:600; color:#999999;">Mount<\/td>/);
    assert.match(html, /<td>\/home<\/td><td>212\u00a0GiB<\/td>/);
});

test("only web and mail links open", () => {
    assert.equal(safeUrl("https://ryoku.dev/docs"), "https://ryoku.dev/docs");
    assert.equal(safeUrl("HTTP://example.com"), "HTTP://example.com");
    assert.equal(safeUrl("mailto:a@example.com"), "mailto:a@example.com");
    for (const bad of ["file:///etc/passwd", "javascript:alert(1)", "--version", "smb://host/share", "ftp://x", "", "https://a b"])
        assert.equal(safeUrl(bad), "");
    assert.match(render("[docs](https://ryoku.dev/docs?a=1&b=2)", style), /<a href="https:\/\/ryoku\.dev\/docs\?a=1&amp;b=2" style="color:#cc6633;">docs<\/a>/);
    const script = render("[run](javascript:alert(1)) it", style);
    assert.doesNotMatch(script, /<a |href/);
    assert.match(script, />run\)\u00a0it<\/p>$/);
    assert.doesNotMatch(render("[run](file:///etc/passwd)", style), /<a /);
    assert.match(render("see https://ryoku.dev/a.", style), /see <a href="https:\/\/ryoku\.dev\/a"[^>]*>https:\/\/ryoku\.dev\/a<\/a>\./);
});

test("a figure keeps its unit, and no tiny word is left alone", () => {
    assert.equal(bind("the CPU sits at 84 °C.", true), "the CPU sits at 84" + NBSP + "°C.");
    assert.equal(bind("fan 4900 RPM, 14 minutes", false), "fan 4900" + NBSP + "RPM, 14 minutes");
    assert.equal(bind("this is a test", false), "this is a" + NBSP + "test");
    assert.equal(bind("within a minute or so", true), "within a" + NBSP + "minute or" + NBSP + "so");
    assert.equal(bind("within a minute or so", false), "within a" + NBSP + "minute or so");
    assert.equal(bind("done", true), "done");
});

test("a style cannot smuggle markup", () => {
    const html = render("`x` [l](https://a.example)", { codeBg: '#111"><img>', link: "red;}<b>", mono: "A'B<" });
    assert.doesNotMatch(html, /<img>|<b>|'B</);
});

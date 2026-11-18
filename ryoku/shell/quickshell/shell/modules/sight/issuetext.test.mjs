import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";

const require = createRequire(import.meta.url);
const { messageHtml } = require("./issuetext.js");
const mono = "Martian Mono";
const code = text => '<span style="font-family:' + mono + '">' + text + '</span>';

test("Harper backticks become inline code in the configured mono font", () => {
    assert.equal(messageHtml("Did you mean to spell `neighbours` this way?", mono),
        "Did you mean to spell " + code("neighbours") + " this way?");
    assert.equal(messageHtml("Use `a` rather than `b`.", mono),
        "Use " + code("a") + " rather than " + code("b") + ".");
});

test("plain and code text never become source HTML or images", () => {
    assert.equal(messageHtml('<img src="https://example.com/tracker"> & `<b>word</b>`', mono),
        '&lt;img src=&quot;https://example.com/tracker&quot;&gt; &amp; ' + code("&lt;b&gt;word&lt;/b&gt;"));
    assert.equal(messageHtml('"quotes" and \'apostrophes\' &lt; `a & b`', mono),
        '&quot;quotes&quot; and &#39;apostrophes&#39; &amp;lt; ' + code("a &amp; b"));
    assert.equal(messageHtml("**bold** [link](https://example.com)", mono),
        "**bold** [link](https://example.com)");
});

test("unmatched backticks remain literal and empty balanced spans lose their ticks", () => {
    assert.equal(messageHtml("An unmatched `word", mono), "An unmatched `word");
    assert.equal(messageHtml("`fixed` then `unfinished", mono), code("fixed") + " then `unfinished");
    assert.equal(messageHtml("Empty `` span", mono), "Empty " + code("") + " span");
});

test("line breaks and Unicode survive rich text conversion", () => {
    assert.equal(messageHtml("First\r\n`naïve`\n最後\rline", mono),
        "First<br>" + code("naïve") + "<br>最後<br>line");
    assert.equal(messageHtml("", mono), "");
    assert.equal(messageHtml(null, mono), "");
});

test("font attribute data is escaped and theme colors remain inherited", () => {
    assert.equal(messageHtml("`word`", 'Mono"><img src="x">'),
        '<span style="font-family:Mono&quot;&gt;&lt;img src=&quot;x&quot;&gt;">word</span>');
    assert.equal(messageHtml("Text `word`", mono), "Text " + code("word"));
});

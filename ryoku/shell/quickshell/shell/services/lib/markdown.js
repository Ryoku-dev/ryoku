// Text shown through a Markdown TextEdit in the Ask bar and the Agent chat.
// Qt fetches an image URL on its own as soon as the text is on screen, so a
// reply could make the shell call out to any host. Images are cut down to their
// alt text, and only web and mail links may be opened.

function prose(s) {
    return s
        .replace(/!\[([^\]]*)\]\([^)]*\)/g, function (_, alt) { return alt || "image"; })
        .replace(/!\[([^\]]*)\]\[[^\]]*\]/g, function (_, alt) { return alt || "image"; })
        .replace(/([^\n])\n(?!\n)/g, "$1  \n");
}

// prepare keeps soft line breaks as hard ones and leaves fenced code as written.
function prepare(text) {
    var src = String(text || "");
    var fence = /^(```|~~~)[^\n]*\n[\s\S]*?^\1[^\n]*$/gm;
    var out = "";
    var last = 0;
    var m;
    while ((m = fence.exec(src)) !== null) {
        out += prose(src.slice(last, m.index)) + m[0];
        last = m.index + m[0].length;
    }
    return out + prose(src.slice(last));
}

// safeUrl returns the link when it is http(s) or mailto, and "" otherwise.
function safeUrl(url) {
    var s = String(url || "").trim();
    return /^(https?:\/\/|mailto:)\S+$/i.test(s) ? s : "";
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { prepare: prepare, safeUrl: safeUrl };

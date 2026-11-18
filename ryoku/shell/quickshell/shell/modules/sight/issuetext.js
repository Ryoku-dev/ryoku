function htmlEscape(text) {
    return String(text).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}

// Issue messages use backticks for code, not general Markdown or source HTML.
function messageHtml(text, monoFont) {
    const font = htmlEscape(monoFont);
    return String(text || "").split(/(`[^`]*`)/g).map(function(part) {
        if (part.length > 1 && part[0] === "`" && part[part.length - 1] === "`")
            return '<span style="font-family:' + font + '">' + htmlEscape(part.slice(1, -1)) + '</span>';
        return htmlEscape(part);
    }).join("").replace(/\r\n?|\n/g, "<br>");
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { messageHtml: messageHtml };

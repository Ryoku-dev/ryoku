// A Rashin answer, from Markdown to the rich text a TextEdit lays out. Qt's own
// Markdown import sets every block flush against the next, so this writes the
// blocks itself: a half line between them, a little air between list items, and
// code on a tinted run of its own.
//
// It is also the gate on what an answer may do. Everything is escaped, so no
// markup in an answer reaches the renderer; images are cut down to their alt
// text (Qt would fetch an image URL on its own as soon as the text is on
// screen); and only web and mail links may be opened.

var NBSP = "\u00a0";

// safeUrl returns the link when it is http(s) or mailto, and "" otherwise.
function safeUrl(url) {
    var s = String(url || "").trim();
    return /^(https?:\/\/|mailto:)\S+$/i.test(s) ? s : "";
}

function esc(text) {
    return String(text).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

// bind keeps what belongs together on one line: a figure and its unit, a
// one-letter word and the word after it, and (at the end of a block) a last
// word too short to stand alone.
function bind(text, closing) {
    var s = String(text);
    s = s.replace(/(\d) ([^\s\d]{1,3})(?=[\s.,;:!?)]|$)/g, "$1" + NBSP + "$2");
    s = s.replace(/(^|[\s\u00a0(])([A-Za-z&\u2013\u2014]) (?=\S)/g, "$1$2" + NBSP);
    if (closing)
        s = s.replace(/(\S) (\S{1,3})$/, "$1" + NBSP + "$2");
    return s;
}

function emphasis(html) {
    return html
        .replace(/\*\*([^*\n]+?)\*\*/g, "<b>$1</b>")
        .replace(/(^|[^\w])__([^_\n]+?)__(?!\w)/g, "$1<b>$2</b>")
        .replace(/\*([^*\s][^*\n]*?)\*/g, "<i>$1</i>")
        .replace(/(^|[^\w])_([^_\s][^_\n]*?)_(?!\w)/g, "$1<i>$2</i>")
        .replace(/~~([^~\n]+?)~~/g, "<s>$1</s>");
}

function anchor(url, label, st) {
    return '<a href="' + esc(url) + '" style="color:' + st.link + ';">' + label + "</a>";
}

function prose(text, st, closing) {
    var links = [];
    var s = String(text)
        .replace(/!\[([^\]]*)\]\([^)]*\)/g, function(_, alt) { return alt || "image"; })
        .replace(/!\[([^\]]*)\]\[[^\]]*\]/g, function(_, alt) { return alt || "image"; })
        .replace(/\[([^\]]+)\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g, function(_, label, url) {
            links.push({ label: label, url: url });
            return "\u0001" + (links.length - 1) + "\u0002";
        });
    s = esc(bind(s, closing));
    s = s.replace(/(^|[\s(])(https?:\/\/[^\s<]*[^\s<).,;:!?])/g, function(_, lead, url) {
        var safe = safeUrl(url.replace(/&amp;/g, "&"));
        return safe ? lead + anchor(safe, url, st) : lead + url;
    });
    s = emphasis(s);
    return s.replace(/\u0001(\d+)\u0002/g, function(_, index) {
        var link = links[Number(index)];
        var label = emphasis(esc(link.label));
        var safe = safeUrl(link.url);
        return safe ? anchor(safe, label, st) : label;
    });
}

// inline renders one run of text. Code spans are held aside as written while
// the rest is set as prose, so emphasis around a span still closes.
function inline(text, st, closing) {
    var spans = [];
    var held = String(text).replace(/`([^`\n]+)`/g, function(_, code) {
        spans.push(code);
        return "\u0003" + (spans.length - 1) + "\u0004";
    });
    return prose(held, st, closing).replace(/\u0003(\d+)\u0004/g, function(_, index) {
        return '<span style="font-family:\'' + st.mono + "'; font-size:" + st.monoPx + "px; background-color:" + st.codeBg + ';">'
            + esc(spans[Number(index)]) + "</span>";
    });
}

var LIST = /^(\s*)([-*+]|\d{1,9}[.)])\s+(.*)$/;
var FENCE = /^ {0,3}(`{3,}|~{3,})(.*)$/;
var HEAD = /^ {0,3}(#{1,6})\s+(.*?)\s*#*\s*$/;
var RULE = /^ {0,3}([-*_])( *\1){2,} *$/;
var REFDEF = /^ {0,3}\[[^\]]+\]:\s+\S+/;
var DIVIDER = /^\s*\|?\s*:?-{1,}:?\s*(\|\s*:?-{1,}:?\s*)*\|?\s*$/;

function cells(line) {
    var s = String(line).trim().replace(/^\|/, "").replace(/\|$/, "");
    return s.split("|").map(function(cell) { return cell.trim(); });
}

function starts(lines, i) {
    var line = lines[i];
    return FENCE.test(line) || HEAD.test(line) || RULE.test(line) || /^ {0,3}>/.test(line) || LIST.test(line)
        || (line.indexOf("|") >= 0 && i + 1 < lines.length && DIVIDER.test(lines[i + 1]) && lines[i + 1].indexOf("-") >= 0);
}

// parse splits the text into blocks:
//   {t: "p"|"quote", lines} {t: "h", level, text} {t: "code", text}
//   {t: "list", items: [{depth, ordered, start, lines}]} {t: "table", head, rows} {t: "rule"}
function parse(text) {
    var lines = String(text || "").replace(/\r\n?/g, "\n").split("\n");
    var blocks = [];
    var i = 0;
    while (i < lines.length) {
        var line = lines[i];
        var fence = FENCE.exec(line);
        if (fence) {
            var mark = fence[1].charAt(0);
            var close = new RegExp("^ {0,3}\\" + mark + "{" + fence[1].length + ",}\\s*$");
            var body = [];
            i++;
            // an answer still streaming has not closed its fence yet
            while (i < lines.length && !close.test(lines[i]))
                body.push(lines[i++]);
            i++;
            blocks.push({ t: "code", text: body.join("\n") });
            continue;
        }
        if (/^\s*$/.test(line) || REFDEF.test(line)) {
            i++;
            continue;
        }
        var head = HEAD.exec(line);
        if (head) {
            blocks.push({ t: "h", level: head[1].length, text: head[2] });
            i++;
            continue;
        }
        if (RULE.test(line)) {
            blocks.push({ t: "rule" });
            i++;
            continue;
        }
        if (/^ {0,3}>/.test(line)) {
            var quoted = [];
            while (i < lines.length && /^ {0,3}>/.test(lines[i]))
                quoted.push(lines[i++].replace(/^ {0,3}> ?/, ""));
            blocks.push({ t: "quote", lines: quoted });
            continue;
        }
        if (line.indexOf("|") >= 0 && i + 1 < lines.length && DIVIDER.test(lines[i + 1]) && lines[i + 1].indexOf("-") >= 0) {
            var table = { t: "table", head: cells(line), rows: [] };
            i += 2;
            while (i < lines.length && lines[i].indexOf("|") >= 0 && !/^\s*$/.test(lines[i]))
                table.rows.push(cells(lines[i++]));
            blocks.push(table);
            continue;
        }
        var first = LIST.exec(line);
        if (first) {
            var items = [];
            var base = first[1].length;
            while (i < lines.length) {
                var entry = LIST.exec(lines[i]);
                if (entry) {
                    var ordered = /\d/.test(entry[2]);
                    items.push({
                        depth: Math.max(0, Math.floor((entry[1].length - base) / 2)),
                        ordered: ordered, start: ordered ? parseInt(entry[2], 10) : 1, lines: [entry[3]]
                    });
                    i++;
                } else if (/^\s+\S/.test(lines[i]) && items.length > 0) {
                    items[items.length - 1].lines.push(lines[i].trim());
                    i++;
                } else if (/^\s*$/.test(lines[i]) && i + 1 < lines.length && (LIST.test(lines[i + 1]) || /^\s+\S/.test(lines[i + 1]))) {
                    i++;
                } else {
                    break;
                }
            }
            blocks.push({ t: "list", items: items });
            continue;
        }
        var para = [];
        while (i < lines.length && !/^\s*$/.test(lines[i]) && !(para.length > 0 && starts(lines, i)))
            para.push(lines[i++].trim());
        blocks.push({ t: "p", lines: para });
    }
    return blocks;
}

function runs(lines, st) {
    var out = [];
    for (var i = 0; i < lines.length; i++)
        out.push(inline(lines[i], st, i === lines.length - 1));
    // a line break in the answer is a line break on screen
    return out.join("<br/>");
}

function list(items, st, top) {
    var html = "";
    var open = [];
    var fresh = false;
    for (var i = 0; i < items.length; i++) {
        var item = items[i];
        while (open.length > item.depth + 1)
            html += "</" + open.pop() + ">";
        if (open.length < item.depth + 1) {
            while (open.length < item.depth + 1) {
                var tag = item.ordered ? "ol" : "ul";
                html += "<" + tag + (item.ordered && item.start !== 1 ? ' start="' + item.start + '"' : "")
                    + ' style="margin-top:' + (open.length === 0 ? top : 0) + "px; margin-bottom:0px; margin-left:0px; -qt-list-indent:" + (open.length + 1) + ';">';
                open.push(tag);
            }
            fresh = true;
        }
        html += '<li style="margin-top:' + (i === 0 ? 0 : fresh ? Math.round(st.itemGap / 2) : st.itemGap) + "px; margin-bottom:0px; line-height:" + st.lineHeight + '%;">'
            + runs(item.lines, st) + "</li>";
        fresh = false;
    }
    while (open.length > 0)
        html += "</" + open.pop() + ">";
    return html;
}

function number(value, fallback) {
    var n = Number(value);
    return isFinite(n) && n >= 0 ? Math.round(n) : fallback;
}

// render: the answer as Qt rich text. style (all optional; sizes in px, colours
// as "#rrggbb"): px (body size), gap (between blocks), itemGap (between list
// items), lineHeight (percent), mono, monoPx, codeBg, codePad, link, dim.
function render(text, style) {
    var given = style || {};
    var px = number(given.px, 14);
    var st = {
        px: px,
        gap: number(given.gap, Math.round(px * 0.6)),
        itemGap: number(given.itemGap, Math.round(px * 0.22)),
        lineHeight: number(given.lineHeight, 135),
        mono: String(given.mono || "monospace").replace(/['"<>;]/g, ""),
        monoPx: number(given.monoPx, Math.round(px * 0.86)),
        codeBg: String(given.codeBg || "transparent").replace(/[^#\w]/g, ""),
        codePad: number(given.codePad, Math.round(px * 0.6)),
        link: String(given.link || "inherit").replace(/[^#\w]/g, ""),
        dim: String(given.dim || "inherit").replace(/[^#\w]/g, "")
    };
    var blocks = parse(text);
    var html = "";
    for (var i = 0; i < blocks.length; i++) {
        var block = blocks[i];
        var top = i === 0 ? 0 : st.gap;
        if (block.t === "p") {
            html += '<p style="margin-top:' + top + "px; margin-bottom:0px; line-height:" + st.lineHeight + '%;">' + runs(block.lines, st) + "</p>";
        } else if (block.t === "h") {
            var size = Math.round(st.px * (block.level === 1 ? 1.25 : block.level === 2 ? 1.14 : 1.05));
            html += '<p style="margin-top:' + (i === 0 ? 0 : Math.round(st.gap * 1.5)) + "px; margin-bottom:0px; font-size:" + size + 'px; font-weight:600;">'
                + inline(block.text, st, true) + "</p>";
        } else if (block.t === "quote") {
            html += '<p style="margin-top:' + top + "px; margin-bottom:0px; margin-left:" + Math.round(st.px * 1.1) + "px; color:" + st.dim + "; line-height:" + st.lineHeight + '%;">'
                + runs(block.lines, st) + "</p>";
        } else if (block.t === "code") {
            // a table cell is the one block Qt pads, so the tint has room around the code
            html += '<table width="100%" cellspacing="0" cellpadding="' + st.codePad + '" style="margin-top:' + top + 'px; margin-bottom:0px;"'
                + (st.codeBg.charAt(0) === "#" ? ' bgcolor="' + st.codeBg + '"' : "") + "><tr><td>"
                + '<p style="margin:0px; white-space:pre-wrap; font-family:\'' + st.mono + "'; font-size:" + st.monoPx + 'px; line-height:140%;">'
                + esc(block.text) + "</p></td></tr></table>";
        } else if (block.t === "list") {
            html += list(block.items, st, top);
        } else if (block.t === "table") {
            html += '<table cellspacing="0" cellpadding="' + Math.round(st.px * 0.3) + '" style="margin-top:' + top + 'px; margin-bottom:0px;"><tr>';
            for (var h = 0; h < block.head.length; h++)
                html += '<td style="font-weight:600; color:' + st.dim + ';">' + inline(block.head[h], st, false) + "</td>";
            html += "</tr>";
            for (var r = 0; r < block.rows.length; r++) {
                html += "<tr>";
                for (var c = 0; c < block.rows[r].length; c++)
                    html += "<td>" + inline(block.rows[r][c], st, false) + "</td>";
                html += "</tr>";
            }
            html += "</table>";
        } else if (block.t === "rule") {
            html += '<p style="margin-top:' + top + 'px; margin-bottom:0px; font-size:1px;">' + NBSP + "</p>";
        }
    }
    return html;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { render: render, parse: parse, bind: bind, safeUrl: safeUrl };

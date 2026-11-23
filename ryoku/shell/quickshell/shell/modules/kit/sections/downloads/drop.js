// What a drop onto Downloads holds: links and magnets for the intake or the
// batch card, and local .torrent files. urls are the drag's uri-list entries,
// text its plain text, detect is root.js detectLink.
// Answers {links: [url], files: [path], skipped: n}, where skipped counts
// local files that are not torrents.

function localPath(entry) {
    var raw = String(entry || "").trim();
    if (raw.indexOf("file://") === 0) {
        var rest = raw.slice(7);
        // file://host/path names a host before the path
        var slash = rest.indexOf("/");
        rest = slash > 0 ? rest.slice(slash) : rest;
        try {
            return decodeURIComponent(rest);
        } catch (error) {
            return rest;
        }
    }
    return raw.charAt(0) === "/" ? raw : "";
}

function textEntries(text) {
    var out = [];
    var lines = String(text || "").split(/\r?\n/);
    for (var i = 0; i < lines.length; i++) {
        var line = lines[i].trim();
        // a file manager's bare path keeps its spaces, so only other lines split into words
        if (localPath(line).length > 0)
            out.push(line);
        else
            out = out.concat(line.split(/\s+/));
    }
    return out;
}

function read(urls, text, detect) {
    var out = { links: [], files: [], skipped: 0 };
    // a drag that carries local files may hand its links over only as text
    var entries = (urls || []).map(function(url) { return String(url || ""); })
        .concat(textEntries(text))
        .filter(function(word) { return word.length > 0 && word.charAt(0) !== "#"; });
    var seen = {};
    for (var i = 0; i < entries.length; i++) {
        var path = localPath(entries[i]);
        if (path.length > 0) {
            // the uri list and the text often name the same file twice
            if (seen[path])
                continue;
            seen[path] = true;
            if (/\.torrent$/i.test(path))
                out.files.push(path);
            else
                out.skipped++;
            continue;
        }
        var link = detect(entries[i]);
        if (link && !seen[link.url]) {
            seen[link.url] = true;
            out.links.push(String(link.url));
        }
    }
    return out;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { read: read, localPath: localPath };

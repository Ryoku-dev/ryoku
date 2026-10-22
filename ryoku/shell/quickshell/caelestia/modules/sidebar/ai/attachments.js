.pragma library

// Paths of files attached to a Claude Code message.

function isImagePath(p) {
    return /\.(png|jpe?g|gif|webp|bmp)$/i.test(p || "");
}

// A local path from a path or file:// URL (file pickers and drops give URLs).
function localPath(path) {
    path = (path || "").trim();
    if (path.indexOf("file://") === 0)
        path = decodeURIComponent(path.substring(7));
    return path;
}

function fileName(path) {
    return (path || "").replace(/^.*\//, "");
}

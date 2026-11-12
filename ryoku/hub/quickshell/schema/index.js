.pragma library
.import "window-style.js" as WindowStyle

// each migrated page's schema by its registry `file`, so search reaches its rows
var byFile = {
    "WindowStyle": WindowStyle
};

function forFile(file) {
    return file && byFile[file] ? byFile[file] : null;
}

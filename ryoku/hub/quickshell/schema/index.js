.pragma library
.import "window-style.js" as WindowStyle
.import "bar.js" as Bar

// each migrated page's schema by its registry `file`, so search reaches its rows
var byFile = {
    "WindowStyle": WindowStyle,
    "Bar": Bar
};

function forFile(file) {
    return file && byFile[file] ? byFile[file] : null;
}

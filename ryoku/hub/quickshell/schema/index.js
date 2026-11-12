.pragma library
.import "window-style.js" as WindowStyle
.import "bar.js" as Bar
.import "displays.js" as Displays
.import "shortcuts.js" as Shortcuts

// each migrated page's schema by its registry `file`, so search reaches its rows
var byFile = {
    "WindowStyle": WindowStyle,
    "Bar": Bar,
    "Displays": Displays,
    "Shortcuts": Shortcuts
};

function forFile(file) {
    return file && byFile[file] ? byFile[file] : null;
}

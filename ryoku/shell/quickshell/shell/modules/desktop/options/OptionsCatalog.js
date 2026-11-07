.pragma library

// The widget scopes (Ryoku keys) that ship a right-click options panel. The
// menu resolves each panel by name -- options/<Capitalised scope>Options.qml --
// so registering a new one is a new file plus one line here, and no two owners
// have to touch WidgetMenu.qml. Keep entries one-per-line so appends never
// collide.
var _has = {
    "clock": true,
    "calendar": true,
    "music": true,
    "notes": true,
    "visualizer": true
};

var _panel = {};

function has(widget) {
    return _has[widget] === true;
}

function panelFile(widget) {
    return _panel[widget] || "";
}

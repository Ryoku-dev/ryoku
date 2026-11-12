.pragma library

// SessionPage as data: the two login-time lists (autostart commands, session
// variables) folded onto one page. The page is a hand-built list editor, so
// these rows exist only to keep both settings reachable from search; the
// descriptions are user copy.

var rows = [
    {
        "tab": "",
        "group": "AT LOGIN",
        "key": "desktop.autostart",
        "label": "Autostart command",
        "desc": "Runs once at login, on top of Ryoku's own autostart.",
        "ctl": "list",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "ENVIRONMENT",
        "key": "desktop.env",
        "label": "Session variable",
        "desc": "A NAME=value exported into your session at login.",
        "ctl": "list",
        "src": "desktop.json"
    }
];

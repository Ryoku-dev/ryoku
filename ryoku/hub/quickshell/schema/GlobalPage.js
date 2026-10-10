.pragma library

// Global: the cross-cutting preferences that are not tied to one surface --
// interface language, regional formats (dates/numbers/currency), the machine
// location (weather + clock), and the shell text size. Every key is a plain
// shell.json value (src "shell"), so the daemon persists and the shell retunes
// live. These were collected here from Desktop (weather) and from nowhere at all
// (language, text size were config-only, with no editor), so each key lives in
// exactly one page -- no overlap.
var rows = [
    {
        "tab": "",
        "group": "LANGUAGE & REGION",
        "key": "language",
        "label": "Language",
        "desc": "Interface language; Auto follows your system locale.",
        "ctl": "pick",
        "src": "shell",
        // the list is not written here: `set` points the picker at the one
        // language table (ryoku/i18n/langs.json, via I18n), so adding a
        // language never means editing this file.
        "set": "languages"
    }, {
        "tab": "",
        "group": "LANGUAGE & REGION",
        "key": "formatLocale",
        "label": "Regional formats",
        "desc": "Dates, numbers and month names use this region.",
        "ctl": "pick",
        "src": "shell",
        "set": "locales"
    }, {
        "tab": "",
        "group": "LANGUAGE & REGION",
        "key": "general.clock_format_24_h",
        "label": "24-hour time",
        "desc": "Use 24-hour time everywhere in Ryoku. Turn off for 12-hour time with AM/PM.",
        "ctl": "sw",
        "src": "shell"
    }, {
        "tab": "",
        "group": "LOCATION",
        "key": "weatherLocation",
        "label": "Location",
        "desc": "Search a city; empty reads it from your IP.",
        "ctl": "location",
        "src": "shell"
    }, {
        "tab": "",
        "group": "LOCATION",
        "key": "timezone",
        "label": "Time zone",
        "desc": "The system clock's time zone; applied live.",
        "ctl": "timezone"
    }, {
        "tab": "",
        "group": "LOCATION",
        "key": "weatherUnit",
        "label": "Temperature units",
        "desc": "Auto follows your locale.",
        "ctl": "seg",
        "src": "shell",
        "opts": ["auto", "celsius", "fahrenheit"]
    }, {
        "tab": "",
        "group": "FONT",
        "key": "fontFamily",
        "label": "System font",
        "desc": "One font for the shell, apps and terminal.",
        "ctl": "pick",
        "src": "shell",
        "opts": []
    }, {
        "tab": "",
        "group": "FONT",
        "key": "fontSize",
        "label": "Font size",
        "desc": "Base text size in points for apps and the terminal.",
        "ctl": "step",
        "src": "shell",
        "lo": 8,
        "hi": 24,
        "unit": " pt"
    }, {
        "tab": "",
        "group": "FONT",
        "key": "fontScale",
        "label": "Interface scale",
        "desc": "Scales the shell surfaces and popouts together.",
        "ctl": "slid",
        "src": "shell",
        "lo": 0.8,
        "hi": 1.5,
        "pct": true
    }
];

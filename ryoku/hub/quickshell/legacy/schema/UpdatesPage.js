.pragma library

// UpdatesPage as data. Generated from the page it replaces.
// Descriptions are written by hand; the inventory carries engineering
// notes, which are not user copy.

var rows = [
    {
        "tab": "",
        "group": "AUTOMATIC CHECKS",
        "key": "update_interval",
        "label": "Automatic checks",
        "desc": "How often the hub checks for updates; off is manual only",
        "ctl": "seg",
        "src": "hub.toml), TOML table [ui], field update_interval",
        "opts": [
            "off",
            "hourly",
            "daily",
            "weekly"
        ]
    },
    {
        "tab": "",
        "group": "UPDATE NOW",
        "key": "update_system",
        "label": "Include system packages",
        "desc": "Update now also takes the distribution's packages, kernel included (ryoku update --system)",
        "ctl": "sw",
        "src": "hub.toml), TOML table [ui], field update_system"
    }
];

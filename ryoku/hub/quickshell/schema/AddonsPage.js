.pragma library

// AddonsPage as data. Generated from the page it replaces.
// Descriptions are written by hand; the inventory carries engineering
// notes, which are not user copy.

var rows = [
    {
        "tab": "detail",
        "group": "Placement",
        "key": "<pluginId>.enabled",
        "label": "Enabled",
        "desc": "Runs the add-on; off keeps it installed but idle",
        "ctl": "sw",
        "src": "plugins.json (via `ryoku-plugins-place <id> enabled <true|false>`)"
    },
    {
        "tab": "detail",
        "group": "Placement",
        "key": "<pluginId>.host",
        "label": "Show as",
        "desc": "Where it appears: popout, wallpaper tile, bar glyph or sidebar card",
        "ctl": "seg",
        "src": "plugins.json (via `ryoku-plugins-place <id> host <hostName>`)",
        "opts": [
            "framePopout",
            "desktopWidget",
            "topbarGlyph",
            "sidebarCard",
            "<any"
        ]
    },
    {
        "tab": "detail",
        "group": "(plugin-declared, from manifest.metadata.settings[].group - group headers are rendered by PluginSettingsForm itself, one per distinct `group` string, in schema order; fields with group \"\" get no header)",
        "key": "<pluginId>.settings.<field.key>",
        "label": "(plugin-declared, field.label, falling back to field.key)",
        "desc": "Each add-on defines its own; applied live",
        "ctl": "custom",
        "src": "plugins.json (via `ryoku-plugins-place <id> settings <json>`, one single-key object per change, jq-merged into the existing settings object)",
        "unit": "none"
    },
    {
        "tab": "Plugins",
        "group": "Management",
        "key": "",
        "label": "Update / Remove",
        "desc": "Refreshes or removes it; placement stays as set",
        "ctl": "action",
        "src": "ryostore internal install-guest|remove-guest plugins <id>"
    },
    {
        "tab": "Bundles",
        "group": "Management",
        "key": "",
        "label": "Remove component / bundle",
        "desc": "Shows every component state, with a terminal",
        "ctl": "action",
        "src": "ryostore-install remove item|bundle"
    },
    {
        "tab": "",
        "group": "OTHER",
        "key": "",
        "label": "Browse RyoStore",
        "desc": "Opens the matching plugin or bundle catalogue",
        "ctl": "action",
        "src": "ryostore open plugins|bundles"
    }
];

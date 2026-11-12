.pragma library

// DisplaysPage as data. Generated from the page it replaces.
// Descriptions are written by hand; the inventory carries engineering
// notes, which are not user copy.

var rows = [
    {
        "tab": "",
        "group": "<selected monitor name> (SettingSection title is dynamic: page.sel.name; literal fallback \"DISPLAY\")",
        "key": "position (Set as main re-bases the layout so this display sits at the 0,0 origin; not a per-display disk key)",
        "label": "Main display",
        "desc": "Makes this the primary display, at the global origin",
        "ctl": "action",
        "src": "<name>.json (Save) / monitors-applied.json (Apply)"
    },
    {
        "tab": "",
        "group": "<selected monitor name> (SettingSection title is dynamic: page.sel.name; literal fallback \"DISPLAY\" when no selection)",
        "key": "disabled",
        "label": "Enabled",
        "desc": "Off turns the display dark; takes effect only when you Apply",
        "ctl": "sw",
        "src": "<name>.json (RYOKU_MONITORS_DIR) on Save"
    },
    {
        "tab": "",
        "group": "<selected monitor name> (dynamic; fallback \"DISPLAY\")",
        "key": "mode",
        "label": "Resolution",
        "desc": "Size and refresh rate together; Custom takes any W\u00d7H@Hz.",
        "ctl": "chips",
        "src": "<name>.json (Save)",
        "opts": [
            "DYNAMIC",
            "Option",
            "Option",
            "Parsed",
            "De-duplicated",
            "Sorted:",
            "Fallback",
            "Related"
        ]
    },
    {
        "tab": "",
        "group": "<selected monitor name> (dynamic; fallback \"DISPLAY\")",
        "key": "scale",
        "label": "Scale",
        "desc": "Larger renders everything bigger, with less room",
        "ctl": "step",
        "src": "<name>.json (Save)",
        "lo": 0.5,
        "hi": 3.0
    },
    {
        "tab": "",
        "group": "<selected monitor name> (dynamic; fallback \"DISPLAY\")",
        "key": "transform",
        "label": "Rotation",
        "desc": "Turns the picture; 90 and 270 swap its sides",
        "ctl": "seg",
        "src": "<name>.json (Save)",
        "opts": [
            "0",
            "1",
            "2",
            "3"
        ]
    },
    {
        "tab": "",
        "group": "<selected monitor name> (dynamic; fallback \"DISPLAY\")",
        "key": "vrr",
        "label": "Adaptive sync",
        "desc": "Refresh follows the frame rate when enabled",
        "ctl": "sw",
        "src": "monitors-applied.json (Apply)",
        "opts": [
            "false",
            "true"
        ]
    },
    {
        "tab": "",
        "group": "<selected monitor name> (dynamic; fallback \"DISPLAY\")",
        "key": "cm",
        "label": "Colour",
        "desc": "sRGB is standard; HDR needs a capable display",
        "ctl": "seg",
        "src": "<name>.json (Save)",
        "opts": [
            "sRGB",
            "Wide",
            "HDR"
        ]
    },
    {
        "tab": "",
        "group": "<selected monitor name> (dynamic; fallback \"DISPLAY\")",
        "key": "sdrbrightness",
        "label": "SDR brightness",
        "desc": "Brightness of non-HDR content while in HDR",
        "ctl": "step",
        "src": "<name>.json (Save)",
        "lo": 1.0,
        "hi": 6.0
    },
    {
        "tab": "",
        "group": "<selected monitor name> (dynamic; fallback \"DISPLAY\")",
        "key": "mirror",
        "label": "Mirror of",
        "desc": "Shows a copy of another display here",
        "ctl": "seg",
        "src": "<name>.json (Save)",
        "opts": [
            "\"\"",
            "DYNAMIC:",
            "Excluded"
        ]
    },
    {
        "tab": "",
        "group": "POSITION",
        "key": "position (serialised jointly as \"<x>x<y>\", e.g. \"2560x0\" - X and Y are NOT separate disk keys)",
        "label": "X",
        "desc": "Distance from the layout's left edge",
        "ctl": "step",
        "src": "<name>.json (Save)",
        "lo": 0.0,
        "hi": 20000.0,
        "unit": "px"
    },
    {
        "tab": "",
        "group": "POSITION",
        "key": "position (serialised jointly as \"<x>x<y>\" - X and Y are NOT separate disk keys)",
        "label": "Y",
        "desc": "Distance from the layout's top edge",
        "ctl": "step",
        "src": "<name>.json (Save)",
        "lo": 0.0,
        "hi": 20000.0,
        "unit": "px"
    },
    {
        "tab": "",
        "group": "PROFILES",
        "key": "filename - becomes ~/.config/ryoku/monitors/<name>.json",
        "label": "Profile name",
        "desc": "Name for the new profile; Enter saves it",
        "ctl": "text",
        "src": "<name>.json (RYOKU_MONITORS_DIR)"
    },
    {
        "tab": "",
        "group": "PROFILES",
        "key": "",
        "label": "Save (profile)",
        "desc": "",
        "ctl": "action",
        "src": "desktop.json",
        "caps": "monitorConfig"
    },
    {
        "tab": "",
        "group": "(header quick-actions Row - outside any SettingSection, top-right, aligned to the \"N displays detected\" text)",
        "key": "",
        "label": "Mirror (quick action)",
        "desc": "",
        "ctl": "action",
        "src": "desktop.json",
        "caps": "monitorConfig"
    },
    {
        "tab": "",
        "group": "(header quick-actions Row - outside any SettingSection)",
        "key": "",
        "label": "Extend (quick action)",
        "desc": "",
        "ctl": "action",
        "src": "monitors-applied.json"
    },
    {
        "tab": "",
        "group": "(header quick-actions Row - outside any SettingSection)",
        "key": "",
        "label": "DPI auto-scale (quick action)",
        "desc": "",
        "ctl": "action",
        "src": "monitors-applied.json"
    },
    {
        "tab": "",
        "group": "(bottom action bar - outside any SettingSection)",
        "key": "",
        "label": "Apply (action bar)",
        "desc": "",
        "ctl": "action",
        "src": "monitors-applied.json"
    },
    {
        "tab": "",
        "group": "(bottom action bar - outside any SettingSection)",
        "key": "",
        "label": "Revert (action bar)",
        "desc": "",
        "ctl": "action",
        "src": "shell"
    }
];

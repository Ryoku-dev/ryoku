.pragma library

// FastfetchPage as data. Generated from the page it replaces.
// Descriptions are written by hand; the inventory carries engineering
// notes, which are not user copy.

var rows = [
    {
        "tab": "",
        "group": "EMBLEM",
        "key": "logo.type",
        "label": "Emblem kind",
        "desc": "Art beside the readout: image, ASCII, logo or none",
        "ctl": "seg",
        "src": "config.jsonc), written by `ryoku-hub fastfetch save <json>`",
        "opts": [
            "image",
            "ascii",
            "builtin",
            "none"
        ]
    },
    {
        "tab": "",
        "group": "EMBLEM",
        "key": "logo.source",
        "label": "Emblem file",
        "desc": "Image or ASCII file to draw; SVG is converted",
        "ctl": "text",
        "src": "config.jsonc"
    },
    {
        "tab": "",
        "group": "EMBLEM",
        "key": "logo.width",
        "label": "Width",
        "desc": "Character columns the art spans",
        "ctl": "step",
        "src": "config.jsonc",
        "lo": 0.0,
        "hi": 80.0,
        "unit": "col"
    },
    {
        "tab": "",
        "group": "EMBLEM",
        "key": "logo.height",
        "label": "Height",
        "desc": "Lines of text the art covers",
        "ctl": "step",
        "src": "config.jsonc",
        "lo": 0.0,
        "hi": 60.0,
        "unit": "col"
    },
    {
        "tab": "",
        "group": "EMBLEM",
        "key": "logo.padding.left",
        "label": "Pad",
        "desc": "Blank columns between the terminal edge and the art",
        "ctl": "step",
        "src": "config.jsonc",
        "lo": 0.0,
        "hi": 20.0,
        "unit": "col"
    },
    {
        "tab": "",
        "group": "EMBLEM",
        "key": "logo.dither",
        "label": "Dither",
        "desc": "Draws the emblem as 1-bit stipple",
        "ctl": "sw",
        "src": "config.jsonc"
    },
    {
        "tab": "",
        "group": "ACCENT",
        "key": "display.color.keys",
        "label": "Readout accent",
        "desc": "Tints the label column of every info line",
        "ctl": "color",
        "src": "config.jsonc"
    },
    {
        "tab": "",
        "group": "INFO",
        "key": "modules",
        "label": "Info rows",
        "desc": "The readout's lines: reorder, rename, remove",
        "ctl": "multi",
        "src": "shell",
        "opts": [
            "(per row) move up / move down",
            "(per row) enable / disable",
            "(per row) inline text / key editor",
            "(per row) remove",
            "__header"
        ]
    },
    {
        "tab": "",
        "group": "INFO",
        "key": "(none - derived, not persisted)",
        "label": "Row name",
        "desc": "Name shown in this list; nothing is stored",
        "ctl": "readout",
        "src": "shell"
    }
];

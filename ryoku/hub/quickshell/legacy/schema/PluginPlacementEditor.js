.pragma library

// PluginPlacementEditor as data. Generated from the page it replaces.
// Descriptions are written by hand; the inventory carries engineering
// notes, which are not user copy.

var rows = [
    {
        "tab": "Installed",
        "group": "OTHER",
        "key": "<pluginId>.framePopout.edge",
        "label": "Edge",
        "desc": "Edge the popout grows from; right when unset",
        "ctl": "seg",
        "src": "plugins.json) via `ryoku-plugins-place <id> framePopout <edge> <align> <hoverW> <hoverH>`",
        "opts": [
            "center",
            "top",
            "right",
            "bottom",
            "left"
        ]
    },
    {
        "tab": "Installed",
        "group": "OTHER",
        "key": "<pluginId>.framePopout.align",
        "label": "Align (drag the \"popout\" chip anywhere on the stage)",
        "desc": "Where along that edge the popout sits",
        "ctl": "seg",
        "src": "plugins.json via `ryoku-plugins-place <id> framePopout <edge> <align> <hoverW> <hoverH>`",
        "opts": [
            "start",
            "center",
            "end"
        ]
    },
    {
        "tab": "Installed",
        "group": "OTHER",
        "key": "<pluginId>.framePopout.hoverW",
        "label": "Hover zone width",
        "desc": "Width of the hover strip; 320 when unset",
        "ctl": "step",
        "src": "plugins.json via `ryoku-plugins-place <id> framePopout <edge> <align> <hoverW> <hoverH>`",
        "unit": "px"
    },
    {
        "tab": "Installed",
        "group": "OTHER",
        "key": "<pluginId>.framePopout.hoverH",
        "label": "Hover zone thickness",
        "desc": "How far the hover strip reaches out; 16 unset",
        "ctl": "step",
        "src": "plugins.json via `ryoku-plugins-place <id> framePopout <edge> <align> <hoverW> <hoverH>`",
        "unit": "px"
    }
];

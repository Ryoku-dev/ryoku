.pragma library

var rows = [{
        "tab": "General",
        "group": "SHELL RELOAD",
        "key": "reloadCover",
        "label": "Reload cover",
        "desc": "Shown while the desktop shell restarts",
        "ctl": "reload-cover",
        "src": "brand"
    },{
        "tab": "Clipboard",
        "group": "LAYOUT",
        "key": "clipboard.widthPercent",
        "label": "Width",
        "desc": "How much of the screen the clipboard panel spans",
        "ctl": "step",
        "src": "shell",
        "lo": 40,
        "hi": 90,
        "unit": "%"
    },{
        "tab": "Clipboard",
        "group": "LAYOUT",
        "key": "clipboard.heightPercent",
        "label": "Height",
        "desc": "How tall the clipboard panel can be",
        "ctl": "step",
        "src": "shell",
        "lo": 24,
        "hi": 72,
        "unit": "%"
    },{
        "tab": "Clipboard",
        "group": "LAYOUT",
        "key": "clipboard.bottomPercent",
        "label": "Bottom offset",
        "desc": "How far the panel rests above the screen edge; zero sits it on the edge",
        "ctl": "step",
        "src": "shell",
        "lo": 0,
        "hi": 20,
        "unit": "%"
    },{
        "tab": "Clipboard",
        "group": "CORNERS",
        "key": "clipboard.panelRadius",
        "label": "Panel rounding",
        "desc": "Roundness of the outer clipboard panel",
        "ctl": "step",
        "src": "shell",
        "lo": 0,
        "hi": 32,
        "unit": "px"
    },{
        "tab": "Clipboard",
        "group": "CORNERS",
        "key": "clipboard.paneRadius",
        "label": "Pane rounding",
        "desc": "Roundness of the Clipboard and Starred panes",
        "ctl": "step",
        "src": "shell",
        "lo": 0,
        "hi": 32,
        "unit": "px"
    },{
        "tab": "Clipboard",
        "group": "CORNERS",
        "key": "clipboard.cardRadius",
        "label": "Card rounding",
        "desc": "Roundness of individual clipboard cards",
        "ctl": "step",
        "src": "shell",
        "lo": 0,
        "hi": 32,
        "unit": "px"
    }
];

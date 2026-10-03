.pragma library

var rows = [{
        "tab": "General",
        "group": "BRAND",
        "key": "name",
        "label": "Name",
        "desc": "The name the shell calls this desktop",
        "eg": "Ryoku",
        "ctl": "text",
        "src": "brand"
    },{
        "tab": "General",
        "group": "BRAND",
        "key": "markText",
        "label": "Text mark",
        "desc": "The glyph the shell uses as its mark",
        "eg": "力",
        "ctl": "text",
        "src": "brand"
    },{
        "tab": "General",
        "group": "BRAND",
        "key": "markImage",
        "label": "Logo image",
        "desc": "An image mark, used instead of the glyph",
        "ctl": "image",
        "src": "brand"
    },{
        "tab": "General",
        "group": "BRAND",
        "key": "markTint",
        "label": "Tint image to accent",
        "ctl": "sw",
        "src": "brand"
    },{
        "tab": "General",
        "group": "SHELL RELOAD",
        "key": "reloadCover",
        "label": "Reload cover",
        "desc": "Shown while the desktop shell restarts",
        "ctl": "reload-cover",
        "src": "brand"
    },{
        "tab": "Sidebars",
        "group": "PANEL",
        "key": "sidebars.width",
        "label": "Width",
        "desc": "How wide each sidebar opens",
        "ctl": "step",
        "src": "shell",
        "lo": 280,
        "hi": 560,
        "unit": "px"
    },{
        "tab": "Sidebars",
        "group": "PANEL",
        "key": "sidebars.motion",
        "label": "Motion",
        "desc": "How quickly the sidebars move",
        "ctl": "seg",
        "src": "shell",
        "opts": ["quick","standard","calm"]
    },{
        "tab": "Sidebars",
        "group": "PANEL",
        "key": "sidebars.depth",
        "label": "Depth",
        "desc": "Cast a shadow where the desktop meets the sidebar",
        "ctl": "sw",
        "src": "shell"
    },{
        "tab": "Sidebars",
        "group": "PANEL",
        "key": "sidebars.push",
        "label": "Push windows",
        "desc": "Move tiled windows aside while a sidebar is open",
        "ctl": "sw",
        "src": "shell"
    },{
        "tab": "Sidebars",
        "group": "PANEL",
        "key": "sidebars.wallpaperSlide",
        "label": "Wallpaper parallax",
        "desc": "How far the wallpaper slides past the panel width",
        "ctl": "slid",
        "src": "shell",
        "lo": 1.0,
        "hi": 1.4,
        "unit": "×"
    },{
        "tab": "Sidebars",
        "group": "LEFT SIDEBAR",
        "key": "sidebars.left.enabled",
        "label": "Enabled",
        "desc": "Show the left sidebar",
        "ctl": "sw",
        "src": "shell"
    },{
        "tab": "Sidebars",
        "group": "LEFT SIDEBAR",
        "key": "sidebars.left.cards",
        "label": "Cards",
        "desc": "Cards shown in the left sidebar",
        "ctl": "multi",
        "src": "shell",
        "opts": ["system","notifications","weather","media","capture","stage"]
    },{
        "tab": "Sidebars",
        "group": "RIGHT SIDEBAR",
        "key": "sidebars.right.enabled",
        "label": "Enabled",
        "desc": "Show the right sidebar",
        "ctl": "sw",
        "src": "shell"
    },{
        "tab": "Sidebars",
        "group": "RIGHT SIDEBAR",
        "key": "sidebars.right.cards",
        "label": "Cards",
        "desc": "Cards shown in the right sidebar",
        "ctl": "multi",
        "src": "shell",
        "opts": ["usage","tools","chat"]
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

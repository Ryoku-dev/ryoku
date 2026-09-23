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
        "tab": "Visualizer",
        "group": "STYLE",
        "key": "enabled",
        "label": "Enabled",
        "desc": "Paint the audio spectrum on the desktop",
        "ctl": "sw",
        "src": "viz"
    },{
        "tab": "Visualizer",
        "group": "STYLE",
        "key": "style",
        "label": "Style",
        "desc": "How the spectrum is drawn",
        "ctl": "gallery",
        "src": "viz",
        "set": "viz"
    },{
        "tab": "Visualizer",
        "group": "STYLE",
        "key": "shape",
        "label": "Shape",
        "desc": "Rounded or flat ends on each band",
        "ctl": "seg",
        "src": "viz",
        "opts": ["rounded","flat"],
        "when": {"style":["bars","split","dots","segments","frame","radial","spiral"]}
    },{
        "tab": "Visualizer",
        "group": "STYLE",
        "key": "mirror",
        "label": "Mirror",
        "desc": "Mirror the spectrum around its centre",
        "ctl": "sw",
        "src": "viz",
        "when": {"style":["bars","split","dots","segments","wave","ribbon","curtain","line"]}
    },{
        "tab": "Visualizer",
        "group": "COLOUR",
        "key": "color",
        "label": "Colour",
        "desc": "Pin an exact colour; clear to follow the wallpaper",
        "ctl": "color",
        "src": "viz"
    },{
        "tab": "Visualizer",
        "group": "COLOUR",
        "key": "gradient",
        "label": "Gradient",
        "desc": "Sweep from the colour above to a second one",
        "ctl": "sw",
        "src": "viz"
    },{
        "tab": "Visualizer",
        "group": "COLOUR",
        "key": "color2",
        "label": "Second colour",
        "desc": "The far end of the gradient",
        "ctl": "color",
        "src": "viz",
        "when": {"gradient":[true]}
    },{
        "tab": "Visualizer",
        "group": "PLACEMENT",
        "key": "spin",
        "label": "Rotation",
        "desc": "How fast the shape spins",
        "ctl": "slid",
        "src": "viz",
        "lo": 0,
        "hi": 30,
        "unit": "deg/s",
        "when": {"style":["radial","orb","spiral"]}
    },{
        "tab": "Visualizer",
        "group": "PLACEMENT",
        "key": "vizPlace",
        "label": "Place on the desktop",
        "desc": "Drag and size it directly on screen",
        "ctl": "action",
        "actionLabel": "PLACE"
    },{
        "tab": "Visualizer",
        "group": "PLACEMENT",
        "key": "grow",
        "label": "Grows",
        "desc": "Which edge of its box the bands rise from",
        "ctl": "seg",
        "src": "viz",
        "opts": ["up","down","center","left","right"],
        "when": {"style":["bars","split","dots","segments","wave","ribbon","curtain","line"]}
    },{
        "tab": "Visualizer",
        "group": "PLACEMENT",
        "key": "angle",
        "label": "Angle",
        "desc": "Turn the whole look about its centre",
        "ctl": "slid",
        "src": "viz",
        "lo": 0,
        "hi": 359,
        "unit": "°"
    },{
        "tab": "Visualizer",
        "group": "PLACEMENT",
        "key": "tiltX",
        "label": "Lean back",
        "desc": "Tip the far edge away from you",
        "ctl": "slid",
        "src": "viz",
        "lo": -35,
        "hi": 35,
        "unit": "°"
    },{
        "tab": "Visualizer",
        "group": "PLACEMENT",
        "key": "tiltY",
        "label": "Lean aside",
        "desc": "Tip one side away from you",
        "ctl": "slid",
        "src": "viz",
        "lo": -35,
        "hi": 35,
        "unit": "°"
    },{
        "tab": "Visualizer",
        "group": "PLACEMENT",
        "adv": true,
        "key": "x",
        "label": "Left",
        "desc": "Where its box starts across the screen",
        "ctl": "slid",
        "src": "viz",
        "lo": -0.2,
        "hi": 1,
        "unit": "%",
        "pct": true
    },{
        "tab": "Visualizer",
        "group": "PLACEMENT",
        "adv": true,
        "key": "y",
        "label": "Top",
        "desc": "Where its box starts down the screen",
        "ctl": "slid",
        "src": "viz",
        "lo": -0.2,
        "hi": 1,
        "unit": "%",
        "pct": true
    },{
        "tab": "Visualizer",
        "group": "PLACEMENT",
        "adv": true,
        "key": "w",
        "label": "Width",
        "desc": "How wide its box is",
        "ctl": "slid",
        "src": "viz",
        "lo": 0.05,
        "hi": 1.2,
        "unit": "%",
        "pct": true
    },{
        "tab": "Visualizer",
        "group": "PLACEMENT",
        "adv": true,
        "key": "h",
        "label": "Height",
        "desc": "How tall its box is",
        "ctl": "slid",
        "src": "viz",
        "lo": 0.03,
        "hi": 1.2,
        "unit": "%",
        "pct": true
    },{
        "tab": "Visualizer",
        "group": "SPECTRUM",
        "adv": true,
        "key": "bars",
        "label": "Bars",
        "desc": "How many bars the spectrum is cut into",
        "ctl": "step",
        "src": "viz",
        "lo": 16,
        "hi": 128
    },{
        "tab": "Visualizer",
        "group": "SPECTRUM",
        "adv": true,
        "key": "segments",
        "label": "Segments",
        "desc": "How many segments a bar is cut into",
        "ctl": "step",
        "src": "viz",
        "lo": 3,
        "hi": 24,
        "when": {"style":["segments"]}
    },{
        "tab": "Visualizer",
        "group": "SPECTRUM",
        "adv": true,
        "key": "thickness",
        "label": "Bar width",
        "desc": "How wide each bar is against its gap",
        "ctl": "slid",
        "src": "viz",
        "lo": 0.2,
        "hi": 1,
        "unit": "%",
        "pct": true,
        "when": {"style":["bars","split","dots","segments","frame","radial","spiral"]}
    },{
        "tab": "Visualizer",
        "group": "SPECTRUM",
        "adv": true,
        "key": "gain",
        "label": "Sensitivity",
        "ctl": "slid",
        "src": "viz",
        "lo": 0.5,
        "hi": 2,
        "unit": "%",
        "pct": true
    },{
        "tab": "Visualizer",
        "group": "SPECTRUM",
        "adv": true,
        "key": "peaks",
        "label": "Peak caps",
        "desc": "Hold a mark at each bar's peak",
        "ctl": "sw",
        "src": "viz",
        "when": {"style":["bars","segments","frame"]}
    },{
        "tab": "Visualizer",
        "group": "SPECTRUM",
        "adv": true,
        "key": "reflection",
        "label": "Reflection",
        "desc": "How much of the spectrum mirrors below it",
        "ctl": "slid",
        "src": "viz",
        "lo": 0,
        "hi": 0.3,
        "unit": "%",
        "pct": true,
        "when": {"style":["bars","split","dots","segments","wave","ribbon","curtain","line"],"grow":["up"]}
    },{
        "tab": "Visualizer",
        "group": "SPECTRUM",
        "adv": true,
        "key": "bloom",
        "label": "Bloom",
        "desc": "How much the spectrum glows",
        "ctl": "slid",
        "src": "viz",
        "lo": 0,
        "hi": 1,
        "unit": "%",
        "pct": true
    },{
        "tab": "Visualizer",
        "group": "MOTION",
        "adv": true,
        "key": "smoothing",
        "label": "Smoothing",
        "desc": "How much motion is smoothed between frames",
        "ctl": "slid",
        "src": "viz",
        "lo": 0,
        "hi": 1,
        "unit": "%",
        "pct": true
    },{
        "tab": "Visualizer",
        "group": "MOTION",
        "adv": true,
        "key": "fps",
        "label": "Frame rate",
        "desc": "How often the spectrum redraws",
        "ctl": "seg",
        "src": "viz",
        "opts": ["30","45","60"]
    },{
        "tab": "Visualizer",
        "group": "MOTION",
        "adv": true,
        "key": "adaptive",
        "label": "Adaptive quality",
        "desc": "Drop the frame rate when nothing is playing",
        "ctl": "sw",
        "src": "viz"
    },{
        "tab": "Visualizer",
        "group": "MOTION",
        "adv": true,
        "key": "idleWave",
        "label": "Idle wave",
        "desc": "Keep a slow wave moving when nothing is playing",
        "ctl": "sw",
        "src": "viz"
    },{
        "tab": "General",
        "group": "QUICK SETTINGS",
        "key": "frameBars.menus.quick-settings.anchor",
        "label": "Sidebar edge",
        "desc": "Which edge the Super+Esc sidebar opens from",
        "ctl": "seg",
        "src": "shell",
        "opts": ["left","right","top","bottom"]
    },{
        "tab": "General",
        "group": "QUICK SETTINGS",
        "key": "frameBars.menus.quick-settings.expansion",
        "label": "Fill the edge",
        "desc": "Stretch to the edge, or fit its content",
        "ctl": "seg",
        "src": "shell",
        "opts": ["always","never"]
    },{
        "tab": "General",
        "group": "QUICK SETTINGS",
        "key": "frameBars.menus.quick-settings.minWidth",
        "label": "Minimum width",
        "desc": "How wide the sidebar is at its narrowest",
        "ctl": "step",
        "src": "shell",
        "lo": 200,
        "hi": 1200,
        "unit": "px"
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

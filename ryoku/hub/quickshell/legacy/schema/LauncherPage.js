.pragma library

// LauncherPage as data. Generated from the page it replaces.
// Descriptions are written by hand; the inventory carries engineering
// notes, which are not user copy.

var rows = [
    {
        "tab": "",
        "group": "LAUNCHER",
        "key": "variant",
        "label": "Style",
        "desc": "Picks the launcher Super+Space opens, shipped or from RyoStore",
        "ctl": "seg",
        "src": "launcher.json"
    },
    {
        "tab": "",
        "group": "SHAPE",
        "key": "radius",
        "label": "Corner radius",
        "desc": "Rounds the palette corners; inner cards 4 px tighter",
        "ctl": "step",
        "src": "launcher.json",
        "lo": 0.0,
        "hi": 28.0,
        "unit": "px"
    },
    {
        "tab": "",
        "group": "BACKGROUND",
        "key": "bgBlur",
        "label": "Local frost",
        "desc": "Frosts the frozen desktop behind the result drawer",
        "ctl": "step",
        "src": "launcher.json",
        "lo": 0.0,
        "hi": 30.0,
        "unit": "px"
    },
    {
        "tab": "",
        "group": "OVERLAY",
        "key": "overlayOpacity",
        "label": "Overlay opacity",
        "desc": "How opaque the backdrop behind the launcher is",
        "ctl": "slid",
        "src": "launcher.json",
        "lo": 0.3,
        "hi": 1.0,
        "unit": "%",
        "pct": true
    },
    {
        "tab": "",
        "group": "GRID",
        "key": "columns",
        "label": "Columns",
        "desc": "Apps per row; fewer fit on a narrow screen",
        "ctl": "step",
        "src": "launcher.json",
        "lo": 3.0,
        "hi": 16.0
    },
    {
        "tab": "",
        "group": "GRID",
        "key": "matWidth",
        "label": "Tile width",
        "desc": "Width of each app tile; the grid is columns times this",
        "ctl": "step",
        "src": "launcher.json",
        "lo": 90.0,
        "hi": 240.0,
        "unit": "px"
    },
    {
        "tab": "",
        "group": "RESULT MOTION",
        "key": "resultSettleMs",
        "label": "Type settle",
        "desc": "Pause before the result deck fades in; higher is calmer",
        "ctl": "step",
        "src": "launcher.json",
        "lo": 120.0,
        "hi": 700.0,
        "unit": "ms"
    },
    {
        "tab": "",
        "group": "HERO",
        "key": "weatherUnit",
        "label": "Weather units",
        "desc": "Temperature scale on the hero, Auto follows your locale",
        "ctl": "seg",
        "src": "launcher.json",
        "opts": [
            "auto",
            "C",
            "F"
        ]
    },
    {
        "tab": "",
        "group": "HERO",
        "key": "showWeather",
        "label": "Show weather",
        "desc": "Weather and temperature on the hero; off shows the date",
        "ctl": "sw",
        "src": "launcher.json"
    },
    {
        "tab": "",
        "group": "HERO",
        "key": "showGreeting",
        "label": "Show greeting",
        "desc": "Time-of-day greeting above the hero clock",
        "ctl": "sw",
        "src": "launcher.json"
    },
    {
        "tab": "",
        "group": "HERO IMAGE",
        "key": "heroImage",
        "label": "Hero image",
        "desc": "Wide banner behind the palette; drag to reframe it",
        "ctl": "text",
        "src": "launcher.json"
    },
    {
        "tab": "",
        "group": "HERO IMAGE",
        "key": "heroStrength",
        "label": "Strength",
        "desc": "How visible the hero image is; 0 hides it completely",
        "ctl": "slid",
        "src": "launcher.json",
        "lo": 0.0,
        "hi": 1.0,
        "unit": "%",
        "pct": true
    },
    {
        "tab": "",
        "group": "HERO IMAGE",
        "key": "heroPosX",
        "label": "Hero focal point X",
        "desc": "Horizontal crop point; 0 left, 1 right; drag preview",
        "ctl": "slid",
        "src": "launcher.json",
        "lo": 0.0,
        "hi": 1.0
    },
    {
        "tab": "",
        "group": "HERO IMAGE",
        "key": "heroPosY",
        "label": "Hero focal point Y",
        "desc": "Vertical crop point; 0 top, 1 bottom; drag preview",
        "ctl": "slid",
        "src": "launcher.json",
        "lo": 0.0,
        "hi": 1.0
    },
    {
        "tab": "",
        "group": "HERO",
        "key": "horizonMode",
        "label": "Solar line",
        "desc": "Warm line under the clock: Palette, Fixed or Off",
        "ctl": "seg",
        "src": "launcher.json",
        "opts": [
            "auto",
            "fixed",
            "off"
        ]
    },
    {
        "tab": "",
        "group": "HERO",
        "key": "horizonColor",
        "label": "Line colour",
        "desc": "Colour of the solar line when it is set to Fixed",
        "ctl": "color",
        "src": "launcher.json"
    }
];

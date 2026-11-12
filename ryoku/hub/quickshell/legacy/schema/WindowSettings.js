.pragma library

// AppearancePage as data. Generated from the page it replaces.
// Descriptions are written by hand; the inventory carries engineering
// notes, which are not user copy.

var rows = [{
        "tab": "Layout",
        "group": "TILING",
        "key": "desktop.appearance.layout",
        "label": "Tiling layout",
        "desc": "Binary splits, master and stack, or a scrolling strip",
        "ctl": "seg",
        "src": "desktop.json",
        "caps": "tiledLayout",
        "opts": [
            "dwindle",
            "master",
            "scrolling"
        ]
    },{
        "tab": "Layout",
        "group": "TILING",
        "ctl": "layoutdemo",
        "label": "",
        "desc": "",
        "src": "desktop.json"
    },{
        "tab": "Layout",
        "group": "GAPS",
        "key": "desktop.appearance.gapsIn",
        "label": "Inner (between windows)",
        "desc": "Space between neighbouring tiled windows",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 40.0,
        "unit": "px"
    },{
        "tab": "Layout",
        "group": "GAPS",
        "key": "desktop.appearance.gapsOut",
        "label": "Outer (screen edge)",
        "desc": "Space between windows and the screen edge",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 60.0,
        "unit": "px"
    },{
        "tab": "Layout",
        "group": "GAPS",
        "key": "desktop.appearance.gapsWorkspaces",
        "label": "Workspace gaps",
        "desc": "Extra gap between workspaces while swiping between them",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 80.0,
        "unit": "px"
    },{
        "tab": "Layout",
        "group": "BEHAVIOUR",
        "key": "desktop.windows.tameMaximizeOnOpen",
        "label": "Tame apps that open maximised",
        "desc": "Open maximised apps as a windowed tile inside your gaps",
        "ctl": "sw",
        "src": "desktop.json"
    },{
        "tab": "Layout",
        "group": "BEHAVIOUR",
        "key": "desktop.appearance.resizeOnBorder",
        "label": "Drag to resize at window edges",
        "desc": "Grab a window's edge with the mouse to resize it",
        "ctl": "sw",
        "src": "desktop.json"
    },{
        "tab": "Layout",
        "group": "BEHAVIOUR",
        "key": "desktop.appearance.snapEnabled",
        "label": "Snap floating windows",
        "desc": "Floating windows stick to edges and other windows",
        "ctl": "sw",
        "src": "desktop.json"
    },{
        "tab": "Layout",
        "group": "BEHAVIOUR",
        "key": "desktop.appearance.extendBorderGrab",
        "label": "Border grab area",
        "desc": "How many pixels past the border still grab it for resizing",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 40.0,
        "unit": "px"
    },{
        "tab": "Layout",
        "group": "BEHAVIOUR",
        "key": "desktop.appearance.hoverIconOnBorder",
        "label": "Resize cursor on border",
        "desc": "Show the resize cursor when hovering a window border",
        "ctl": "sw",
        "src": "desktop.json"
    },{
        "tab": "Layout",
        "group": "BEHAVIOUR",
        "key": "desktop.appearance.noFocusFallback",
        "label": "No focus fallback",
        "desc": "On close, don't refocus the window under the cursor",
        "ctl": "sw",
        "src": "desktop.json",
        "adv": true
    },{
        "tab": "Layout",
        "group": "BEHAVIOUR",
        "key": "desktop.appearance.resizeCorner",
        "label": "Resize corner",
        "desc": "Lock resizing to one corner; 0 off, 1-4 pick it",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 4.0,
        "adv": true
    },{
        "tab": "Look",
        "group": "SHAPE",
        "key": "desktop.appearance.rounding",
        "label": "Corner radius",
        "desc": "How rounded window corners are, 0 keeps them square",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 30.0,
        "unit": "px"
    },{
        "tab": "Look",
        "group": "SHAPE",
        "key": "desktop.appearance.roundingPower",
        "label": "Corner softness",
        "desc": "2 is a circle; higher flattens toward square",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 2.0,
        "hi": 8.0
    },{
        "tab": "Look",
        "group": "OPACITY",
        "key": "desktop.appearance.activeOpacity",
        "label": "Active",
        "desc": "Solidity of the focused window; under 100% shows through",
        "ctl": "slid",
        "src": "desktop.json",
        "lo": 0.4,
        "hi": 1.0,
        "unit": "%",
        "pct": true
    },{
        "tab": "Look",
        "group": "OPACITY",
        "key": "desktop.appearance.inactiveOpacity",
        "label": "Inactive",
        "desc": "How solid unfocused windows are drawn",
        "ctl": "slid",
        "src": "desktop.json",
        "lo": 0.4,
        "hi": 1.0,
        "unit": "%",
        "pct": true
    },{
        "tab": "Look",
        "group": "OPACITY",
        "key": "desktop.appearance.dimInactive",
        "label": "Dim inactive windows",
        "desc": "Darkens every window except the focused one",
        "ctl": "sw",
        "src": "desktop.json"
    },{
        "tab": "Look",
        "group": "OPACITY",
        "key": "desktop.appearance.dimStrength",
        "label": "Dim strength",
        "desc": "How dark unfocused windows get while dimming is on",
        "ctl": "slid",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 1.0,
        "unit": "%",
        "pct": true
    },{
        "tab": "Look",
        "group": "OPACITY",
        "key": "desktop.appearance.fullscreenOpacity",
        "label": "Fullscreen opacity",
        "desc": "Opacity of a window while it is fullscreen",
        "ctl": "slid",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 1.0,
        "unit": "%",
        "pct": true,
        "adv": true
    },{
        "tab": "Look",
        "group": "OPACITY",
        "key": "desktop.appearance.dimSpecial",
        "label": "Dim special workspaces",
        "desc": "How much the scratchpad workspace dims the desktop",
        "ctl": "slid",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 1.0,
        "unit": "%",
        "pct": true,
        "adv": true
    },{
        "tab": "Look",
        "group": "OPACITY",
        "key": "desktop.appearance.dimAround",
        "label": "Dim around floating",
        "desc": "How strongly a dim-around window darkens the rest",
        "ctl": "slid",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 1.0,
        "unit": "%",
        "pct": true,
        "adv": true
    },{
        "tab": "Look",
        "group": "OPACITY",
        "key": "desktop.appearance.dimModal",
        "label": "Dim modal dialogs",
        "desc": "Darken the parent while a modal dialog is open",
        "ctl": "sw",
        "src": "desktop.json",
        "adv": true
    },{
        "tab": "Look",
        "group": "BLUR",
        "key": "desktop.appearance.blurEnabled",
        "label": "Enabled",
        "desc": "Frosted-glass blur behind translucent windows and the shell",
        "ctl": "sw",
        "src": "desktop.json"
    },{
        "tab": "Look",
        "group": "BLUR",
        "key": "desktop.appearance.blurSize",
        "label": "Size",
        "desc": "How far each pass spreads; pair with passes",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 20.0,
        "unit": "px"
    },{
        "tab": "Look",
        "group": "BLUR",
        "key": "desktop.appearance.blurPasses",
        "label": "Passes",
        "desc": "Times the blur repeats, more is stronger but costs GPU time",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 1.0,
        "hi": 6.0
    },{
        "tab": "Look",
        "group": "BLUR",
        "key": "desktop.appearance.blurXray",
        "label": "X-ray (blur shows the wallpaper)",
        "desc": "Ignores windows beneath when blurring, lighter on the GPU",
        "ctl": "sw",
        "src": "desktop.json"
    },{
        "tab": "Look",
        "group": "BLUR",
        "key": "desktop.appearance.blurVibrancy",
        "label": "Vibrancy",
        "desc": "Boosts colour saturation seen through the blur",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 0.5
    },{
        "tab": "Look",
        "group": "BLUR",
        "key": "desktop.appearance.blurNoise",
        "label": "Noise",
        "desc": "Grain over blurred areas, hides gradient banding",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 0.1
    },{
        "tab": "Look",
        "group": "BLUR",
        "key": "desktop.appearance.blurContrast",
        "label": "Blur contrast",
        "desc": "Contrast of the blurred backdrop",
        "ctl": "slid",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 2.0,
        "adv": true
    },{
        "tab": "Look",
        "group": "BLUR",
        "key": "desktop.appearance.blurBrightness",
        "label": "Blur brightness",
        "desc": "Brightness of the blurred backdrop",
        "ctl": "slid",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 2.0,
        "adv": true
    },{
        "tab": "Look",
        "group": "BLUR",
        "key": "desktop.appearance.blurSpecial",
        "label": "Blur special workspace",
        "desc": "Also blur behind the special (scratchpad) workspace",
        "ctl": "sw",
        "src": "desktop.json",
        "adv": true
    },{
        "tab": "Look",
        "group": "BLUR",
        "key": "desktop.appearance.blurPopups",
        "label": "Blur popups",
        "desc": "Blur behind popups and menus, not just windows",
        "ctl": "sw",
        "src": "desktop.json",
        "adv": true
    },{
        "tab": "Look",
        "group": "BLUR",
        "key": "desktop.appearance.blurIgnoreOpacity",
        "label": "Blur ignores opacity",
        "desc": "Blur the backdrop even under fully transparent regions",
        "ctl": "sw",
        "src": "desktop.json",
        "adv": true
    },{
        "tab": "Look",
        "group": "BLUR",
        "key": "desktop.appearance.blurNewOptimizations",
        "label": "Blur optimizations",
        "desc": "Cache blur for speed; off only if you see artifacts",
        "ctl": "sw",
        "src": "desktop.json",
        "adv": true
    },{
        "tab": "Look",
        "group": "BLUR",
        "key": "desktop.appearance.blurVibrancyDarkness",
        "label": "Blur vibrancy darkness",
        "desc": "How much vibrancy affects the dark parts of the blur",
        "ctl": "slid",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 1.0,
        "adv": true
    },{
        "tab": "Look",
        "group": "SHADOWS",
        "key": "desktop.appearance.shadowEnabled",
        "label": "Window shadows",
        "desc": "A soft drop shadow under every window",
        "ctl": "sw",
        "src": "desktop.json"
    },{
        "tab": "Look",
        "group": "SHADOWS",
        "key": "desktop.appearance.shadowRange",
        "label": "Shadow range",
        "desc": "How far the shadow spreads from the window",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 60.0,
        "unit": "px"
    },{
        "tab": "Look",
        "group": "SHADOWS",
        "key": "desktop.appearance.shadowSpread",
        "label": "Spread",
        "desc": "Grows the shadow outward before its soft edge begins",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 64.0,
        "unit": "px"
    },{
        "tab": "Look",
        "group": "SHADOWS",
        "key": "desktop.appearance.shadowOffsetX",
        "label": "Horizontal offset",
        "desc": "Shifts the shadow sideways, as if lit from one side",
        "ctl": "step",
        "src": "desktop.json",
        "lo": -64.0,
        "hi": 64.0,
        "unit": "px"
    },{
        "tab": "Look",
        "group": "SHADOWS",
        "key": "desktop.appearance.shadowOffsetY",
        "label": "Vertical offset",
        "desc": "Shifts the shadow up or down; positive is below",
        "ctl": "step",
        "src": "desktop.json",
        "lo": -64.0,
        "hi": 64.0,
        "unit": "px"
    },{
        "tab": "Look",
        "group": "SHADOWS",
        "key": "desktop.appearance.shadowPower",
        "label": "Shadow sharpness",
        "desc": "Higher pulls the shadow in tight, lower leaves a wide haze",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 1.0,
        "hi": 4.0,
        "adv": true
    },{
        "tab": "Look",
        "group": "SHADOWS",
        "key": "desktop.appearance.shadowSharp",
        "label": "Sharp shadow",
        "desc": "Draw the shadow with hard edges instead of a soft falloff",
        "ctl": "sw",
        "src": "desktop.json",
        "adv": true
    },{
        "tab": "Look",
        "group": "SHADOWS",
        "key": "desktop.appearance.shadowScale",
        "label": "Shadow scale",
        "desc": "Scale of the drop shadow, 1 matches the window",
        "ctl": "slid",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 1.0,
        "adv": true
    },{
        "tab": "Look",
        "group": "SHADOWS",
        "key": "desktop.appearance.shadowColor",
        "label": "Shadow color",
        "desc": "",
        "ctl": "color",
        "src": "desktop.json"
    },{
        "tab": "Look",
        "group": "GLOW",
        "key": "desktop.appearance.glowEnabled",
        "label": "Glow behind windows",
        "desc": "A coloured halo cast behind every window",
        "ctl": "sw",
        "src": "desktop.json"
    },{
        "tab": "Look",
        "group": "GLOW",
        "key": "desktop.appearance.glowRange",
        "label": "Range",
        "desc": "How far the halo reaches past the window edge",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 4.0,
        "hi": 60.0,
        "unit": "px"
    },{
        "tab": "Look",
        "group": "GLOW",
        "key": "desktop.appearance.glowColor",
        "label": "Colour",
        "desc": "The halo's tint, picked here, not taken from the theme",
        "ctl": "color",
        "src": "desktop.json"
    },{
        "tab": "Borders",
        "group": "THICKNESS",
        "key": "desktop.appearance.borderSize",
        "label": "Border thickness",
        "desc": "Width of the frame around every window, 0 removes it",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 12.0,
        "unit": "px"
    },{
        "tab": "Borders",
        "group": "THICKNESS",
        "key": "desktop.appearance.borderPartOfWindow",
        "label": "Border inside window",
        "desc": "Count the border as part of the window size",
        "ctl": "sw",
        "src": "desktop.json"
    },{
        "tab": "Borders",
        "group": "COLOUR",
        "key": "desktop.appearance.borderFollowsPalette",
        "label": "Follow the wallpaper",
        "desc": "Colour the border from the wallpaper palette; off keeps the colours below",
        "ctl": "sw",
        "src": "desktop.json"
    },{
        "tab": "Borders",
        "group": "COLOUR",
        "key": "desktop.appearance.activeBorder",
        "label": "Active border",
        "desc": "Colour of the frame around the window you are using",
        "ctl": "color",
        "src": "desktop.json",
        "when": {"desktop.appearance.borderFollowsPalette": [false]}
    },{
        "tab": "Borders",
        "group": "COLOUR",
        "key": "desktop.appearance.inactiveBorder",
        "label": "Inactive border",
        "desc": "Colour of the frame around windows you are not using",
        "ctl": "color",
        "src": "desktop.json",
        "when": {"desktop.appearance.borderFollowsPalette": [false]}
    },{
        "tab": "Borders",
        "group": "ANIMATED",
        "key": "desktop.appearance.animatedBorder",
        "label": "Rotating gradient border",
        "desc": "Sweeps accent colours around the frame; needs a border",
        "ctl": "sw",
        "src": "desktop.json"
    },{
        "tab": "Borders",
        "group": "ANIMATED",
        "key": "desktop.appearance.borderAngleSpeed",
        "label": "Rotation speed",
        "desc": "How fast the gradient circles the window",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 1.0,
        "hi": 10.0
    },{
        "tab": "Motion",
        "group": "MOTION",
        "key": "desktop.appearance.animations",
        "label": "Animations",
        "desc": "Motion for open, close, move, workspaces; off snaps",
        "ctl": "sw",
        "src": "desktop.json"
    },{
        "tab": "Motion",
        "group": "MOTION",
        "key": "desktop.appearance.wobblyWindows",
        "label": "Wobbly windows",
        "desc": "Dragged windows overshoot and spring back",
        "ctl": "sw",
        "src": "desktop.json"
    },{
        "tab": "Motion",
        "group": "MOTION",
        "key": "desktop.appearance.windowStyle",
        "label": "Open / close",
        "desc": "Open and close motion: pop, slide, or GNOME",
        "ctl": "seg",
        "src": "desktop.json",
        "opts": [
            "pop",
            "slide",
            "gnomed"
        ]
    }
];

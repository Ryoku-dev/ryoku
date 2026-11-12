.pragma library

// InputPage as data. Generated from the page it replaces.
// Descriptions are written by hand; the inventory carries engineering
// notes, which are not user copy.

var rows = [
    {
        "tab": "",
        "group": "KEYBOARD",
        "key": "desktop.input.kbLayout",
        "label": "Layout",
        "desc": "What your keys type: QWERTY, AZERTY, Dvorak.",
        "ctl": "seg",
        "src": "desktop.json",
        "opts": [
            "<dynamic:"
        ]
    },
    {
        "tab": "",
        "group": "KEYBOARD",
        "key": "desktop.input.kbVariant",
        "label": "Style",
        "desc": "A tweak on the layout, like intl or Colemak.",
        "ctl": "seg",
        "src": "desktop.json",
        "opts": [
            "\"\"",
            "<dynamic:"
        ]
    },
    {
        "tab": "",
        "group": "KEYBOARD",
        "key": "desktop.input.kbLayout",
        "label": "Second layout",
        "desc": "A spare layout kept loaded; a chord switches to it.",
        "ctl": "seg",
        "src": "desktop.json",
        "caps": "keyboardLayoutSwitch",
        "opts": [
            "\"\"",
            "<dynamic:"
        ]
    },
    {
        "tab": "",
        "group": "KEYBOARD",
        "key": "desktop.input.kbOptions",
        "label": "Switch layouts",
        "desc": "The chord that flips between your two layouts.",
        "ctl": "seg",
        "src": "desktop.json",
        "caps": "keyboardLayoutSwitch",
        "opts": [
            "\"\"",
            "grp:alt_shift_toggle",
            "grp:win_space_toggle"
        ]
    },
    {
        "tab": "",
        "group": "KEYBOARD",
        "key": "desktop.input.numlockByDefault",
        "label": "Numlock on at login",
        "desc": "Start each session with the keypad typing digits.",
        "ctl": "sw",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "KEY REMAPS",
        "key": "desktop.input.kbOptions",
        "label": "Caps Lock",
        "desc": "Remap it to Escape, Ctrl, or switch it off.",
        "ctl": "chips",
        "src": "desktop.json",
        "opts": [
            "\"\"",
            "caps:escape",
            "ctrl:nocaps",
            "caps:swapescape",
            "caps:none"
        ]
    },
    {
        "tab": "",
        "group": "KEY REMAPS",
        "key": "desktop.input.kbOptions",
        "label": "Swap Alt and Super",
        "desc": "Trade the two keys for macOS-style shortcuts.",
        "ctl": "sw",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "KEY REMAPS",
        "key": "desktop.input.kbOptions",
        "label": "Compose key",
        "desc": "Starts a sequence for accents like é and ñ.",
        "ctl": "seg",
        "src": "desktop.json",
        "opts": [
            "\"\"",
            "compose:ralt",
            "compose:menu"
        ]
    },
    {
        "tab": "",
        "group": "KEY REMAPS",
        "key": "desktop.input.kbOptions",
        "label": "Extra options",
        "desc": "Raw xkb options, comma separated, for power users.",
        "ctl": "text",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "KEY REMAPS",
        "key": "",
        "label": "Apply system-wide",
        "desc": "Match the login screen, TTYs, and boot prompt too.",
        "ctl": "action",
        "src": "ryoku-host keymap apply"
    },
    {
        "tab": "",
        "group": "KEY REMAPS",
        "key": "",
        "label": "Login screen and TTY keymap status",
        "desc": "Result of the last apply; each keeps its own keymap.",
        "ctl": "readout",
        "src": "shell"
    },
    {
        "tab": "",
        "group": "POINTER",
        "key": "desktop.input.sensitivity",
        "label": "Sensitivity",
        "desc": "Pointer speed offset; 0 is the device default.",
        "ctl": "slid",
        "src": "desktop.json",
        "lo": -1.0,
        "hi": 1.0
    },
    {
        "tab": "",
        "group": "POINTER",
        "key": "desktop.input.followMouse",
        "label": "Focus behavior",
        "desc": "How windows take focus as the pointer moves.",
        "ctl": "seg",
        "src": "desktop.json",
        "opts": [
            "Ignore pointer movement",
            "Focus under pointer",
            "Click to focus"
        ]
    },
    {
        "tab": "",
        "group": "POINTER",
        "key": "desktop.input.accelProfile",
        "label": "Acceleration",
        "desc": "Flat ties travel to the hand; Adaptive speeds quick moves.",
        "ctl": "seg",
        "src": "desktop.json",
        "opts": [
            "\"\"",
            "flat",
            "adaptive"
        ]
    },
    {
        "tab": "",
        "group": "POINTER",
        "key": "desktop.input.leftHanded",
        "label": "Left-handed buttons",
        "desc": "Swap the left and right mouse buttons.",
        "ctl": "sw",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "POINTER",
        "key": "desktop.input.mouseNaturalScroll",
        "label": "Natural scroll",
        "desc": "Roll the wheel up and the page moves up.",
        "ctl": "sw",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "POINTER",
        "key": "desktop.input.mouseScrollFactor",
        "label": "Scroll speed",
        "desc": "Multiplier on each wheel notch.",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 0.2,
        "hi": 3.0
    },
    {
        "tab": "",
        "group": "POINTER",
        "key": "desktop.input.middleClickPaste",
        "label": "Middle-click pastes",
        "desc": "Press the wheel to insert the last highlighted text.",
        "ctl": "sw",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "TOUCHPAD",
        "key": "desktop.input.naturalScroll",
        "label": "Natural scroll",
        "desc": "Two fingers drag the content like a touchscreen.",
        "ctl": "sw",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "TOUCHPAD",
        "key": "desktop.input.tapToClick",
        "label": "Tap to click",
        "desc": "A tap counts as a click; two fingers right, three middle.",
        "ctl": "sw",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "TOUCHPAD",
        "key": "desktop.input.tapAndDrag",
        "label": "Tap and drag",
        "desc": "Tap, then hold the finger down to drag what you tapped.",
        "ctl": "sw",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "TOUCHPAD",
        "key": "desktop.input.disableWhileTyping",
        "label": "Disable while typing",
        "desc": "Ignore the pad while typing so a palm can't nudge it.",
        "ctl": "sw",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "TOUCHPAD",
        "key": "desktop.input.clickfinger",
        "label": "Click by finger count",
        "desc": "One finger clicks left, two right, three middle.",
        "ctl": "sw",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "TOUCHPAD",
        "key": "desktop.input.middleEmulation",
        "label": "Emulate middle click",
        "desc": "Pressing left and right together counts as a middle click",
        "ctl": "sw",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "TOUCHPAD",
        "key": "desktop.input.touchScrollFactor",
        "label": "Scroll speed",
        "desc": "Multiplier on two-finger scroll distance.",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 0.2,
        "hi": 3.0
    },
    {
        "tab": "",
        "group": "TOUCHPAD",
        "key": "desktop.input.workspaceSwipe",
        "label": "Swipe between workspaces",
        "desc": "A horizontal swipe slides to the next workspace.",
        "ctl": "sw",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "TOUCHPAD",
        "key": "desktop.input.swipeFingers",
        "label": "Swipe fingers",
        "desc": "How many fingers count as a workspace swipe.",
        "ctl": "seg",
        "src": "desktop.json",
        "opts": [
            "3",
            "4"
        ]
    },
    {
        "tab": "",
        "group": "TOUCHPAD",
        "key": "desktop.input.swipeInvert",
        "label": "Natural swipe direction",
        "desc": "The workspace row follows your fingers.",
        "ctl": "sw",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "TOUCHPAD",
        "key": "desktop.input.swipeCreateNew",
        "label": "Swipe past the last workspace to add one",
        "desc": "Swiping past the end opens a fresh workspace.",
        "ctl": "sw",
        "src": "desktop.json"
    },
    {
        "tab": "",
        "group": "TOUCHPAD",
        "key": "desktop.input.swipeDistance",
        "label": "Swipe distance",
        "desc": "Finger travel for a full switch; lower flips sooner.",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 100.0,
        "hi": 600.0
    },
    {
        "tab": "",
        "group": "KEY REPEAT",
        "key": "desktop.input.repeatRate",
        "label": "Repeat rate",
        "desc": "Characters per second while a key is held.",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 1.0,
        "hi": 100.0,
        "unit": "/s"
    },
    {
        "tab": "",
        "group": "KEY REPEAT",
        "key": "desktop.input.repeatDelay",
        "label": "Repeat delay",
        "desc": "Pause before a held key starts repeating.",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 100.0,
        "hi": 2000.0,
        "unit": "ms"
    },
{
        "tab": "",
        "group": "CURSOR",
        "key": "desktop.cursor.theme",
        "label": "Theme",
        "desc": "The installed pointer set; applies now and to new apps.",
        "ctl": "seg",
        "src": "desktop.json",
        "opts": [
            "DYNAMIC"
        ]
    },{
        "tab": "",
        "group": "CURSOR",
        "key": "desktop.cursor.size",
        "label": "Size",
        "desc": "How large the pointer is drawn.",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 12.0,
        "hi": 64.0,
        "unit": "px"
    },{
        "tab": "",
        "group": "CURSOR",
        "key": "desktop.cursor.inactiveTimeout",
        "label": "Hide after idle",
        "desc": "Seconds of stillness before it hides; 0 never hides.",
        "ctl": "step",
        "src": "desktop.json",
        "lo": 0.0,
        "hi": 30.0,
        "unit": "s"
    },{
        "tab": "",
        "group": "CURSOR",
        "key": "desktop.cursor.hideOnKeyPress",
        "label": "Hide while typing",
        "desc": "It vanishes on a keypress and returns when moved.",
        "ctl": "sw",
        "src": "desktop.json"
    }
];

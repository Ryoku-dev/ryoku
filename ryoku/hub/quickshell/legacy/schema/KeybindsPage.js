.pragma library

// KeybindsPage as data, for the Hub's global finder only. The page carries its
// own search now, so this is not a mirror of every bind: one generic row per
// legend section plus the two editors, enough that typing "numpad", "screen" or
// "custom shortcut" in the global search lands on this page. The live legend is
// the source of truth (read from `ryoku-hub keybinds`); these rows never claim a
// specific chord, so they cannot drift from what a compositor actually binds.
// No `key`: nothing here names a store leaf, so a row is never dropped as a
// dead key and never confused with a settable value.

var rows = [
    {
        "tab": "Shortcuts", "group": "WINDOWS",
        "label": "Window shortcuts",
        "desc": "Close, float, fullscreen, resize and tab the focused window",
        "opts": ["close", "float", "fullscreen", "resize", "maximise", "tabbed", "centre"]
    },
    {
        "tab": "Shortcuts", "group": "FOCUS",
        "label": "Focus shortcuts",
        "desc": "Move focus between windows and columns",
        "opts": ["focus", "left", "right", "up", "down", "first", "last", "column"]
    },
    {
        "tab": "Shortcuts", "group": "MOVE",
        "label": "Move shortcuts",
        "desc": "Move and merge windows across columns",
        "opts": ["move", "merge", "left", "right", "up", "down"]
    },
    {
        "tab": "Shortcuts", "group": "RESIZE",
        "label": "Resize shortcuts",
        "desc": "Grow, shrink and reset the focused window",
        "opts": ["resize", "wider", "narrower", "taller", "shorter", "height"]
    },
    {
        "tab": "Shortcuts", "group": "WORKSPACES",
        "label": "Workspace shortcuts",
        "desc": "Focus and move windows across workspaces, on the keys and the number pad",
        "opts": ["workspace", "numpad", "number pad", "scratchpad", "overview", "previous", "next"]
    },
    {
        "tab": "Shortcuts", "group": "DISPLAYS",
        "label": "Display shortcuts",
        "desc": "Move focus, windows and workspaces between screens",
        "opts": ["display", "screen", "monitor", "mirror", "extend"]
    },
    {
        "tab": "Shortcuts", "group": "SHELL",
        "label": "Shell shortcuts",
        "desc": "Launcher, Kit, lock, clipboard, screenshot, wallpaper and more",
        "opts": ["launcher", "cheatsheet", "lock", "clipboard", "screenshot", "wallpaper", "settings"]
    },
    {
        "tab": "Shortcuts", "group": "MEDIA",
        "label": "Media and hardware keys",
        "desc": "Volume, playback, brightness and touchpad keys",
        "opts": ["media", "volume", "mute", "play", "brightness", "touchpad", "hardware"]
    },
    {
        "tab": "Apps", "group": "APPS",
        "label": "App shortcuts",
        "desc": "Which app each launcher key opens, and the key itself",
        "opts": ["terminal", "browser", "files", "editor", "notes", "shell", "launch"]
    },
    {
        "tab": "Custom", "group": "CUSTOM",
        "label": "Custom shortcuts",
        "desc": "Add your own key combos layered over the ones Ryoku ships",
        "opts": ["custom", "add", "record", "command", "run", "bind"]
    }
];

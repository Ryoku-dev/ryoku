.pragma library

var title = "Keyboard shortcuts";
var lead = "The keys that run actions on your desktop. Click a shortcut and press the new keys to change it.";

// one section per group the window manager lists; `group` is its name there
var sections = [
    { id: "windows", title: "Windows", group: "Windows" },
    { id: "focus", title: "Focus", group: "Focus" },
    { id: "move", title: "Move", group: "Move" },
    { id: "resize", title: "Resize", group: "Resize" },
    { id: "workspaces", title: "Workspaces", group: "Workspaces" },
    { id: "displays", title: "Displays", group: "Displays" },
    { id: "apps", title: "Apps", group: "Apps" },
    { id: "desktop", title: "Desktop", group: "Shell" },
    { id: "media", title: "Media", group: "Media" },
    { id: "hardware", title: "Hardware", group: "Hardware" },
    { id: "mouse", title: "Mouse", group: "Mouse" },
    { id: "custom", title: "Your own shortcuts" },
    { id: "modifier", title: "Modifier key" }
];

var related = ["default-apps"];

// the groups' shortcuts come from the window manager, so search reaches each group
var rows = [
    { id: "windows", section: "windows", label: "Window shortcuts", control: "chord",
      detail: "Close, fullscreen, float, maximise and centre windows",
      keywords: ["close window", "fullscreen", "float or tile", "pin window", "maximise", "centre", "tabbed", "last window", "window height", "column width", "keybinds", "hotkeys"] },
    { id: "focus", section: "focus", label: "Focus shortcuts", control: "chord",
      detail: "Move focus between windows",
      keywords: ["focus left", "focus right", "focus up", "focus down", "first column", "last column", "keybinds"] },
    { id: "move", section: "move", label: "Move shortcuts", control: "chord",
      detail: "Move windows around and merge them into columns",
      keywords: ["move window", "merge left", "merge right", "swap", "keybinds"] },
    { id: "resize", section: "resize", label: "Resize shortcuts", control: "chord",
      detail: "Make windows wider, narrower, taller or shorter",
      keywords: ["wider", "narrower", "taller", "shorter", "reset height", "size", "keybinds"] },
    { id: "workspaces", section: "workspaces", label: "Workspace shortcuts", control: "chord",
      detail: "Switch workspaces and send windows to them",
      keywords: ["workspace 1 to 10", "next workspace", "previous workspace", "send window to workspace", "number pad", "overview", "scratchpad", "hide window", "keybinds"] },
    { id: "displays", section: "displays", label: "Display shortcuts", control: "chord",
      detail: "Move focus, windows and workspaces between screens",
      keywords: ["focus screen", "send window to screen", "send workspace to screen", "mirror", "extend", "monitor", "keybinds"] },
    { id: "apps", section: "apps", label: "App shortcuts", control: "chord",
      detail: "Open the terminal, files, browser and other apps",
      keywords: ["terminal", "files", "browser", "editor", "notes", "yazi", "ryotunes", "super enter", "launch", "keybinds"] },
    { id: "desktop", section: "desktop", label: "Desktop shortcuts", control: "chord",
      detail: "The launcher, lock screen, wallpaper, clipboard and capture",
      keywords: ["shell", "app launcher", "lock the screen", "quick settings", "wallpaper picker", "clipboard", "screenshot", "capture", "pick a colour", "voice typing", "visualiser", "kit", "cheatsheet", "rashin", "ryoport", "stage editor", "ryoku settings", "keybinds"] },
    { id: "media", section: "media", label: "Media keys", control: "chord",
      detail: "Volume, mute and playback keys",
      keywords: ["volume up", "volume down", "mute", "microphone", "play", "pause", "next track", "previous track", "xf86"] },
    { id: "hardware", section: "hardware", label: "Hardware keys", control: "chord",
      detail: "Brightness and touchpad keys",
      keywords: ["brightness up", "brightness down", "touchpad", "xf86"] },
    { id: "mouse", section: "mouse", label: "Mouse shortcuts", control: "chord",
      detail: "Move and resize windows with the mouse",
      keywords: ["move window with the mouse", "resize window with the mouse", "drag", "scroll"] },
    { id: "add-shortcut", section: "custom", label: "Add a shortcut", control: "action",
      detail: "Run a command or act on the focused window with keys you choose",
      keywords: ["custom", "own", "new shortcut", "command", "run", "bind", "keybind", "hotkey", "exec"] }
];

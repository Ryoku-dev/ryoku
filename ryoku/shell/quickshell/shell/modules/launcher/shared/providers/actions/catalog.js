// The system-action catalog: each entry is a command the launcher can fire,
// wired to a real Ryoku helper (a shell surface, a daemon verb, ryoku-cmd-*).
// Data only, so the set is validated in a node test and the provider stays a
// thin mapper. Each entry says in a few words what it does, shown under its
// name. Categories group the action-mode tabs (System / Appearance / Tools /
// Media / Settings). An optional `caps` names a window-manager capability the
// action needs; the provider hides the entry on a compositor that lacks it (a
// tool that is not even installed there).

var CATALOG = [
    { id: "lock-screen",      name: "Lock Screen",        description: "Locks the screen right away", category: "System",     icon: "lock",        verb: "lock" },
    { id: "open-clipboard",   name: "Clipboard History",  description: "Paste something you copied earlier", category: "System",     icon: "clipboard",   surface: "clipboard" },
    { id: "open-sysinfo",     name: "System Info",        description: "How the machine is doing, in Quick settings", category: "System",     icon: "info",        surface: "sidebar-left" },
    { id: "open-toolkit",     name: "Control Deck",       description: "Switches, sliders and power in Quick settings", category: "System",     icon: "grid",        surface: "sidebar-left" },
    { id: "toggle-caffeine",  name: "Keep Awake",         description: "Keeps the screen awake until you turn it off", category: "System",     icon: "coffee",      exec: ["ryoku-cmd-caffeine"] },
    { id: "toggle-game-mode", name: "Game Mode",          description: "Tunes the desktop for games until turned off", category: "System",     icon: "gamepad",     exec: ["ryoku-cmd-game-mode"], caps: "liveConfigEval" },
    { id: "mirror-displays",  name: "Mirror Displays",    description: "Shows the same picture on every display", category: "System",     icon: "monitor",     exec: ["ryoku-monitor", "toggle"], caps: "outputMirror" },

    { id: "next-wallpaper",   name: "Next Wallpaper",     description: "Switches to your next wallpaper", category: "Appearance", icon: "image",       exec: ["ryogami", "wallpaper", "next"] },
    { id: "pick-wallpaper",   name: "Wallpaper Picker",   description: "Browse your wallpapers and pick one", category: "Appearance", icon: "image-multi", exec: ["ryogami", "wallpaper", "ui"] },
    { id: "wallpaper-settings", name: "Wallpaper Settings", description: "How wallpapers are shown and how they change", category: "Appearance", icon: "settings",  exec: ["ryogami", "wallpaper", "settings"] },
    { id: "toggle-nightlight",name: "Night Light",        description: "Warms the screen colours for the evening", category: "Appearance", icon: "moon",        exec: ["ryoku-cmd-nightlight"], caps: "nightLight" },

    { id: "screenshot",       name: "Screenshot",         description: "Capture the screen, a window or an area", category: "Tools",      icon: "camera",      exec: ["ryoku-cmd-ryoshot"] },
    { id: "screen-record",    name: "Screen Record",      description: "Records the screen to a video", category: "Tools",      icon: "video",       verb: "record" },
    { id: "color-picker",     name: "Color Picker",       description: "Click any colour on screen and copy its value", category: "Tools",      icon: "eyedropper",  exec: ["ryoku-cmd-color-picker"] },
    { id: "ocr",              name: "OCR Text Grab",      description: "Copies the text inside an area of the screen", category: "Tools",      icon: "text-scan",   exec: ["ryoku-cmd-ocr"] },
    { id: "qr-scan",          name: "Scan QR Code",       description: "Reads a QR code shown on screen", category: "Tools",      icon: "qr",          exec: ["ryoku-cmd-qr-scan"] },
    { id: "google-lens",      name: "Google Lens",        description: "Search an area of the screen with Google Lens", category: "Tools",      icon: "image-search",exec: ["ryoku-cmd-google-lens"] },

    { id: "media-play-pause", name: "Play / Pause",       description: "Plays or pauses what is playing", category: "Media",      icon: "play",        exec: ["playerctl", "play-pause"] },
    { id: "media-next",       name: "Next Track",         description: "Skips to the next track", category: "Media",      icon: "skip-next",   exec: ["playerctl", "next"] },
    { id: "media-previous",   name: "Previous Track",     description: "Goes back to the previous track", category: "Media",      icon: "skip-prev",   exec: ["playerctl", "previous"] },
    { id: "recognize-music",  name: "Recognize Music",    description: "Finds the name of the song that is playing", category: "Media",      icon: "music",       exec: ["ryoku-cmd-songrec"] },
    { id: "media-visualizer", name: "Audio Visualizer",   description: "Shows the music as moving bars on the desktop", category: "Media",      icon: "wave",        surface: "visualizer" },

    { id: "open-settings",    name: "Ryoku Settings",     description: "Opens Ryoku Settings", category: "Settings",   icon: "settings",    verb: "hub open" },
    { id: "keybind-legend",   name: "Keybind Reference",  description: "Every keyboard shortcut, in Kit", category: "Settings",   icon: "keyboard",    surface: "cheatsheet#keys" },
    { id: "reload-shell",     name: "Reload Shell",       description: "Restarts the desktop shell if it looks stuck", category: "Settings",   icon: "refresh",     verb: "reload" }
];

var CATEGORIES = ["All", "System", "Appearance", "Tools", "Media", "Settings"];

// Validate the catalog: unique ids, required fields, a known category, and
// exactly one non-empty action. Returns an array of problem strings (empty = valid).
function validate(catalog) {
    var problems = [];
    var seen = {};
    var cats = {};
    for (var i = 1; i < CATEGORIES.length; i++) cats[CATEGORIES[i]] = true;
    for (var j = 0; j < catalog.length; j++) {
        var a = catalog[j];
        if (!a.id) { problems.push("entry " + j + " missing id"); continue; }
        if (seen[a.id]) problems.push("duplicate id: " + a.id);
        seen[a.id] = true;
        if (!a.name) problems.push(a.id + " missing name");
        if (!a.description) problems.push(a.id + " missing description");
        if (!cats[a.category]) problems.push(a.id + " has unknown category: " + a.category);
        var kinds = (a.surface !== undefined ? 1 : 0) + (a.verb !== undefined ? 1 : 0)
            + (a.exec !== undefined ? 1 : 0);
        if (kinds !== 1) problems.push(a.id + " needs exactly one of surface, verb or exec");
        if (a.surface !== undefined && (typeof a.surface !== "string" || a.surface.length === 0))
            problems.push(a.id + " has empty surface");
        if (a.verb !== undefined && (typeof a.verb !== "string" || a.verb.length === 0))
            problems.push(a.id + " has empty verb");
        if (a.exec !== undefined) {
            if (!Array.isArray(a.exec) || a.exec.length === 0) problems.push(a.id + " has empty exec");
            else if (a.exec[0] === "ryoku-shell") problems.push(a.id + " spawns ryoku-shell instead of a verb");
        }
    }
    return problems;
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { CATALOG, CATEGORIES, validate };
}

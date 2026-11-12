// The system-action catalog: each entry is a command the launcher can fire,
// wired to a real Ryoku helper (a shell surface, a daemon verb, ryoku-cmd-*).
// Data only, so the set is validated in a node test and the provider stays a
// thin mapper. Categories group the action-mode tabs (System / Appearance /
// Tools / Media / Settings). An optional `caps` names a window-manager
// capability the action needs; the provider hides the entry on a compositor
// that lacks it (a tool that is not even installed there).

var CATALOG = [
    { id: "lock-screen",      name: "Lock Screen",        category: "System",     icon: "lock",        verb: "lock" },
    { id: "open-clipboard",   name: "Clipboard History",  category: "System",     icon: "clipboard",   surface: "clipboard" },
    { id: "open-sysinfo",     name: "System Info",        category: "System",     icon: "info",        surface: "sidebar-left" },
    { id: "open-toolkit",     name: "Control Deck",       category: "System",     icon: "grid",        surface: "sidebar-left" },
    { id: "toggle-caffeine",  name: "Keep Awake",         category: "System",     icon: "coffee",      exec: ["ryoku-cmd-caffeine"] },
    { id: "toggle-game-mode", name: "Game Mode",          category: "System",     icon: "gamepad",     exec: ["ryoku-cmd-game-mode"], caps: "liveConfigEval" },
    { id: "mirror-displays",  name: "Mirror Displays",    category: "System",     icon: "monitor",     exec: ["ryoku-monitor", "toggle"], caps: "outputMirror" },

    { id: "next-wallpaper",   name: "Next Wallpaper",     category: "Appearance", icon: "image",       exec: ["ryogami", "wallpaper", "next"] },
    { id: "pick-wallpaper",   name: "Wallpaper Picker",   category: "Appearance", icon: "image-multi", exec: ["ryogami", "wallpaper", "ui"] },
    { id: "wallpaper-settings", name: "Wallpaper Settings", category: "Appearance", icon: "settings",  exec: ["ryogami", "wallpaper", "settings"] },
    { id: "toggle-nightlight",name: "Night Light",        category: "Appearance", icon: "moon",        exec: ["ryoku-cmd-nightlight"], caps: "nightLight" },

    { id: "screenshot",       name: "Screenshot",         category: "Tools",      icon: "camera",      exec: ["ryoku-cmd-ryoshot"] },
    { id: "screen-record",    name: "Screen Record",      category: "Tools",      icon: "video",       verb: "record" },
    { id: "color-picker",     name: "Color Picker",       category: "Tools",      icon: "eyedropper",  exec: ["ryoku-cmd-color-picker"] },
    { id: "ocr",              name: "OCR Text Grab",      category: "Tools",      icon: "text-scan",   exec: ["ryoku-cmd-ocr"] },
    { id: "qr-scan",          name: "Scan QR Code",       category: "Tools",      icon: "qr",          exec: ["ryoku-cmd-qr-scan"] },
    { id: "google-lens",      name: "Google Lens",        category: "Tools",      icon: "image-search",exec: ["ryoku-cmd-google-lens"] },

    { id: "media-play-pause", name: "Play / Pause",       category: "Media",      icon: "play",        exec: ["playerctl", "play-pause"] },
    { id: "media-next",       name: "Next Track",         category: "Media",      icon: "skip-next",   exec: ["playerctl", "next"] },
    { id: "media-previous",   name: "Previous Track",     category: "Media",      icon: "skip-prev",   exec: ["playerctl", "previous"] },
    { id: "recognize-music",  name: "Recognize Music",    category: "Media",      icon: "music",       exec: ["ryoku-cmd-songrec"] },
    { id: "media-visualizer", name: "Audio Visualizer",   category: "Media",      icon: "wave",        surface: "visualizer" },

    { id: "open-settings",    name: "Ryoku Settings",     category: "Settings",   icon: "settings",    verb: "hub open" },
    { id: "keybind-legend",   name: "Keybind Reference",  category: "Settings",   icon: "keyboard",    exec: ["sh", "-c", "flock -n -o /tmp/ryoku-keys.lock qs -c keys"] },
    { id: "reload-shell",     name: "Reload Shell",       category: "Settings",   icon: "refresh",     verb: "reload" }
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

.pragma library

// rail order; pages in their list order
var categories = [
    { id: "appearance", label: "Appearance", glyph: "palette",
      pages: ["wallpaper-colours", "window-style", "fonts-icons", "animations", "lock-screen"] },
    { id: "desktop", label: "Desktop", glyph: "dashboard",
      pages: ["bar", "desktop-dock", "quick-settings", "notifications", "sounds", "launcher", "clipboard"] },
    { id: "windows", label: "Windows", glyph: "select_window",
      pages: ["window-behaviour", "shortcuts", "app-rules", "window-manager"] },
    { id: "devices", label: "Devices", glyph: "devices",
      pages: ["displays", "keyboard", "mouse-touchpad"] },
    { id: "connections", label: "Connections", glyph: "wifi",
      pages: ["wifi", "bluetooth", "phone"] },
    { id: "apps", label: "Apps", glyph: "apps",
      pages: ["default-apps", "startup-apps", "terminal", "screen-capture", "voice-typing", "rashin", "add-ons"] },
    { id: "accounts", label: "Accounts", glyph: "group",
      pages: ["users", "sign-in"] },
    { id: "system", label: "System", glyph: "settings",
      pages: ["power", "graphics", "performance", "time-language", "backup-import", "about"] },
    { id: "updates", label: "Updates", glyph: "system_update", pages: ["updates"] }
];

var ryogamiTheme = ["ryogami", "wallpaper", "settings", "colours"];
var ryogamiWallpaper = ["ryogami", "wallpaper", "settings"];

var pages = {
    "wallpaper-colours": { title: "Wallpaper & colours", category: "appearance", handoff: ryogamiTheme,
        keywords: ["wallpaper", "background", "colours", "colors", "theme", "palette", "scheme", "accent", "dark", "light", "matugen", "ryogami"] },
    "window-style": { title: "Window style", category: "appearance", file: "WindowStyle",
        keywords: ["corners", "rounding", "border", "frame", "opacity", "transparency", "dim", "blur", "glass", "shadow", "glow", "title bar", "titlebar", "csd", "decoration", "frost"] },
    "fonts-icons": { title: "Fonts & icons", category: "appearance", file: "FontsIcons",
        keywords: ["font", "typeface", "text size", "monospace", "icons", "icon theme", "pointer", "cursor", "glosses", "japanese", "kanji"] },
    "animations": { title: "Animations", category: "appearance", file: "Animations",
        keywords: ["animation", "motion", "transition", "bezier", "curve", "speed", "wobbly", "focus flash", "reduce motion"] },
    "lock-screen": { title: "Lock screen", category: "appearance", file: "LockScreen",
        keywords: ["lock", "lockscreen", "screensaver", "greeter", "skin", "qylock"] },

    "bar": { title: "Bar", category: "desktop", file: "Bar",
        keywords: ["bar", "bar style", "subshell", "panel", "taskbar", "frame", "edge bars", "widgets", "obi", "orpheus", "shima"] },
    "desktop-dock": { title: "Desktop & dock", category: "desktop", file: "DesktopDock",
        keywords: ["desktop", "dock", "widgets", "stage", "visualizer", "reload cover", "icons"] },
    "quick-settings": { title: "Quick settings", category: "desktop", file: "QuickSettings",
        keywords: ["quick settings", "controls", "sidebar", "super escape", "panel", "toggles", "blocks"] },
    "notifications": { title: "Notifications & pop-ups", category: "desktop", file: "Notifications",
        keywords: ["notifications", "toasts", "osd", "volume popup", "brightness popup", "do not disturb"] },
    "sounds": { title: "Sounds", category: "desktop", file: "Sounds",
        keywords: ["sound", "sounds", "audio", "key sounds", "keyboard sounds", "click", "chime", "alerts"] },
    "launcher": { title: "App launcher", category: "desktop", file: "Launcher",
        keywords: ["launcher", "app search", "command palette", "greeting", "favourites", "hidden apps", "grid"] },
    "clipboard": { title: "Clipboard", category: "desktop", file: "Clipboard",
        keywords: ["clipboard", "copy", "paste", "history", "clear"] },

    "window-behaviour": { title: "Window behaviour", category: "windows", file: "WindowBehaviour",
        keywords: ["tiling", "layout", "gaps", "spacing", "focus follows mouse", "new windows", "resize", "snap", "tabbed", "workspaces", "overview", "switcher", "urgent", "xwayland", "zoom", "dwindle", "master", "scrolling"] },
    "shortcuts": { title: "Keyboard shortcuts", category: "windows", file: "Shortcuts",
        keywords: ["shortcuts", "keybinds", "hotkeys", "keys", "binds", "super", "modifier", "chord"] },
    "app-rules": { title: "App rules", category: "windows", file: "AppRules", needs: { cap: "windowRules" },
        keywords: ["window rules", "app overrides", "layer rules", "float", "pin", "per app", "class", "namespace"] },
    "window-manager": { title: "Window manager", category: "windows", file: "WindowManager", needs: { provider: true },
        keywords: ["window manager", "compositor", "wm", "switch", "plugins", "hyprpm", "wayland", "session"] },

    "displays": { title: "Displays", category: "devices", file: "Displays", needs: { cap: "monitorConfig" },
        keywords: ["monitor", "screen", "resolution", "refresh rate", "scale", "rotation", "arrange", "mirror", "hidpi", "vrr", "adaptive sync", "hdr", "night light"] },
    "keyboard": { title: "Keyboard", category: "devices", file: "Keyboard",
        keywords: ["keyboard", "layout", "xkb", "repeat", "caps lock", "compose", "num lock", "dvorak", "remap"] },
    "mouse-touchpad": { title: "Mouse & touchpad", category: "devices", file: "MouseTouchpad",
        keywords: ["mouse", "touchpad", "trackpad", "pointer", "sensitivity", "acceleration", "scroll", "natural scrolling", "tap", "gestures", "buttons"] },

    "wifi": { title: "Wi-Fi & internet", category: "connections", file: "Wifi",
        keywords: ["wifi", "wi-fi", "wireless", "network", "internet", "ethernet", "hotspot", "forget", "airplane"] },
    "bluetooth": { title: "Bluetooth", category: "connections", file: "Bluetooth",
        keywords: ["bluetooth", "pair", "headphones", "devices"] },
    "phone": { title: "Phone", category: "connections", file: "Phone",
        keywords: ["phone", "android", "kde connect", "scrcpy", "mirror", "webcam", "adb"] },

    "default-apps": { title: "Default apps", category: "apps", file: "DefaultApps",
        keywords: ["default apps", "browser", "terminal", "editor", "file manager", "notes"] },
    "startup-apps": { title: "Startup apps", category: "apps", file: "StartupApps",
        keywords: ["startup", "autostart", "login", "session", "environment", "variables", "env"] },
    "terminal": { title: "Terminal", category: "apps", file: "Terminal",
        keywords: ["terminal", "shell", "prompt", "starship", "fastfetch", "fish", "zsh", "bash"] },
    "screen-capture": { title: "Screen capture", category: "apps", file: "ScreenCapture",
        keywords: ["screenshot", "recording", "screencast", "capture", "video", "fps", "codec", "key press overlay"] },
    "voice-typing": { title: "Voice typing", category: "apps", file: "VoiceTyping",
        keywords: ["voice typing", "dictation", "speech", "whisper", "microphone", "stt"] },
    "rashin": { title: "Rashin", category: "apps", file: "Rashin",
        keywords: ["rashin", "agent", "ai", "ask", "chat", "models"] },
    "add-ons": { title: "Add-ons", category: "apps", file: "AddOns",
        keywords: ["add-ons", "addons", "plugins", "extensions", "store", "ryostore", "bundles", "omarchy"] },

    "users": { title: "Users", category: "accounts", file: "Users",
        keywords: ["users", "accounts", "people", "password", "picture", "admin", "profiles"] },
    "sign-in": { title: "Sign-in & security", category: "accounts", file: "SignIn",
        keywords: ["sign in", "login", "keyring", "fingerprint", "security keys", "sudo", "pam", "fido"] },

    "power": { title: "Power & battery", category: "system", file: "Power",
        keywords: ["power", "battery", "charge limit", "power saver", "idle", "sleep", "suspend", "lock after", "cpu", "epp", "aspm", "governor"] },
    "graphics": { title: "Graphics", category: "system", file: "Graphics",
        keywords: ["graphics", "gpu", "nvidia", "amd", "hybrid", "mux", "passthrough", "vfio"] },
    "performance": { title: "Performance", category: "system", file: "Performance",
        keywords: ["performance", "low power", "potato", "lag", "fps", "live wallpaper", "unload"] },
    "time-language": { title: "Time & language", category: "system", file: "TimeLanguage",
        keywords: ["language", "locale", "region", "formats", "24-hour clock", "time zone", "timezone", "location", "units", "weather units", "date"] },
    "backup-import": { title: "Backup & import", category: "system", file: "BackupImport",
        keywords: ["import", "backup", "export", "restore", "dotfiles", "migrate", "reset all"] },
    "about": { title: "About", category: "system", file: "About",
        keywords: ["about", "version", "edition", "hardware", "system card", "profile", "credits"] },

    "updates": { title: "Updates", category: "updates", file: "Updates",
        keywords: ["update", "upgrade", "version", "what's new", "rollback", "snapshots"] }
};

// old ids and names that callers use today
var aliases = {
    "profile": "about", "credits": "about", "global": "time-language", "connections": "wifi",
    "input": "keyboard", "cursor": "fonts-icons", "gpu": "graphics", "lockscreen": "lock-screen",
    "windowmanager": "window-behaviour", "windows": "window-behaviour", "plugins": "window-manager",
    "layerrules": "app-rules", "windowrules": "app-rules", "appoverrides": "app-rules",
    "subshells": "bar", "bar-studio": "bar", "frame": "bar", "desktop": "desktop-dock",
    "controls": "quick-settings", "sidebar": "quick-settings", "sidebars-left": "quick-settings",
    "sidebars-right": "quick-settings", "overlays": "notifications", "keybinds": "shortcuts",
    "session": "startup-apps", "autostart": "startup-apps", "environment": "startup-apps",
    "recording": "screen-capture", "dictation": "voice-typing", "fastfetch": "terminal",
    "import": "backup-import", "addons": "add-ons", "appearance": "wallpaper-colours"
};

// ryogami's settings, offered by search as hand-offs
var handoffEntries = [
    { label: "Wallpaper", keywords: ["background", "picture", "image", "video"], command: ryogamiWallpaper },
    { label: "Colours", keywords: ["colors", "palette", "scheme"], command: ryogamiTheme },
    { label: "Theme", keywords: ["scheme", "look", "rice"], command: ryogamiTheme },
    { label: "Palette", keywords: ["colours", "colors", "matugen"], command: ryogamiTheme },
    { label: "Light and dark", keywords: ["dark mode", "light mode", "mode"], command: ryogamiTheme },
    { label: "Accent", keywords: ["accent colour", "highlight colour"], command: ryogamiTheme }
];

// Home's Needs you: each probe prints nothing when nothing waits, else a JSON
// object overriding these fields (args fill %1, %2); see Singletons/Attention.qml.
// action "log-out" logs out through Hub.logOut, which asks first
var logOut = ["ryoku", "wm", "act", "session.exit"];
var attention = [
    { id: "wm-switch", get: ["ryoku-hub", "wm", "pending"], page: "window-manager",
      title: "Log out to finish switching to %1", detail: "Your current session stays as it is until you log out.",
      actionText: "Log out", action: "log-out" },
    { id: "graphics-mode", get: ["ryoku-hub", "gpu", "mode", "pending"], page: "graphics",
      title: "Log out to finish switching to %1 graphics", detail: "The new graphics mode takes effect after you log out.",
      actionText: "Log out", action: "log-out" },
    { id: "display-wiring", get: ["ryoku-hub", "gpu", "mux", "pending"], page: "graphics", row: "display-wired",
      title: "Restart to finish wiring the screen to %1 graphics",
      detail: "The built-in screen moves to the other graphics card when the computer restarts.",
      actionText: "Open Graphics" }
];

// a page id after alias resolution, or "" when nothing answers to it
function resolve(id) {
    var key = String(id || "").trim().toLowerCase();
    if (key === "")
        return "";
    var slash = key.indexOf("/");
    if (slash >= 0)
        key = key.slice(0, slash);
    if (pages[key])
        return key;
    var target = aliases[key];
    return target && pages[target] ? target : "";
}

// the row half of "page/row", or ""
function rowPart(id) {
    var key = String(id || "");
    var slash = key.indexOf("/");
    return slash >= 0 ? key.slice(slash + 1) : "";
}

function page(id) {
    var p = pages[id];
    if (!p)
        return null;
    return Object.assign({ id: id }, p);
}

function category(id) {
    for (var i = 0; i < categories.length; i++)
        if (categories[i].id === id)
            return categories[i];
    return null;
}

function categoryIndex(id) {
    for (var i = 0; i < categories.length; i++)
        if (categories[i].id === id)
            return i;
    return -1;
}

// gate: { supports(cap) -> bool, provider: string, known: bool }
function available(id, gate) {
    var p = pages[id];
    if (!p)
        return false;
    var n = p.needs;
    if (!n || !gate || !gate.known)
        return true;
    if (n.cap !== undefined && !gate.supports(n.cap))
        return false;
    if (n.provider === true && !gate.provider)
        return false;
    return true;
}

function pagesOf(categoryId, gate) {
    var c = category(categoryId);
    if (!c)
        return [];
    var out = [];
    for (var i = 0; i < c.pages.length; i++)
        if (available(c.pages[i], gate))
            out.push(page(c.pages[i]));
    return out;
}

function allPageIds() {
    var out = [];
    for (var i = 0; i < categories.length; i++)
        out = out.concat(categories[i].pages);
    return out;
}

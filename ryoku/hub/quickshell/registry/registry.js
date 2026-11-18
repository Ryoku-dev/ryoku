.pragma library

// rail order; pages in their list order
var categories = [
    { id: "appearance", label: "Appearance", glyph: "palette",
      pages: ["style", "wallpaper-colours", "window-style", "fonts-icons", "animations", "lock-screen"] },
    { id: "desktop", label: "Desktop", glyph: "dashboard",
      pages: ["bar", "desktop-dock", "quick-settings", "notifications", "sounds", "launcher", "clipboard", "game-bar"] },
    { id: "windows", label: "Windows", glyph: "select_window",
      pages: ["window-behaviour", "shortcuts", "app-rules", "window-manager", "hyprdrive"] },
    { id: "devices", label: "Devices", glyph: "devices",
      pages: ["displays", "keyboard", "mouse-touchpad"] },
    { id: "connections", label: "Connections", glyph: "wifi",
      pages: ["wifi", "ethernet", "vpn", "dns", "bluetooth", "phone"] },
    { id: "apps", label: "Apps", glyph: "apps",
      pages: ["installed-apps", "default-apps", "startup-apps", "terminal", "screen-capture", "voice-typing", "rashin", "add-ons"] },
    { id: "accounts", label: "Accounts", glyph: "group",
      pages: ["users", "sign-in"] },
    { id: "system", label: "System", glyph: "settings",
      pages: ["power", "graphics", "performance", "time-language", "backup-import", "about"] },
    { id: "updates", label: "Updates", glyph: "system_update", pages: ["updates"] }
];

var ryogamiTheme = ["ryogami", "wallpaper", "settings", "colours"];
var ryogamiWallpaper = ["ryogami", "wallpaper", "settings"];

var pages = {
    "style": { title: "Style", category: "appearance", file: "Style",
        keywords: ["style", "design", "design language", "look", "material", "material you", "lithic", "adwaita", "gnome", "fluent", "windows 11", "rounded", "shape", "colour scheme"] },
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
    "game-bar": { title: "Game bar", category: "desktop", file: "GameBar",
        keywords: ["game bar", "gamebar", "game overlay", "overlay", "in-game", "gaming", "games", "discord", "voice", "snap", "widgets"] },

    "window-behaviour": { title: "Window behaviour", category: "windows", file: "WindowBehaviour",
        keywords: ["tiling", "layout", "gaps", "spacing", "focus follows mouse", "new windows", "resize", "snap", "tabbed", "workspaces", "overview", "switcher", "urgent", "xwayland", "zoom", "dwindle", "master", "scrolling", "mosaic"] },
    "shortcuts": { title: "Keyboard shortcuts", category: "windows", file: "Shortcuts",
        keywords: ["shortcuts", "keybinds", "hotkeys", "keys", "binds", "super", "modifier", "chord"] },
    "app-rules": { title: "App rules", category: "windows", file: "AppRules", needs: { cap: "windowRules" },
        keywords: ["window rules", "app overrides", "layer rules", "float", "pin", "per app", "class", "namespace"] },
    "window-manager": { title: "Window manager", category: "windows", file: "WindowManager", needs: { provider: true },
        keywords: ["window manager", "compositor", "wm", "switch", "plugins", "hyprpm", "wayland", "session"] },
    "hyprdrive": { title: "Hyprdrive", category: "windows", file: "Hyprdrive", needs: { cap: "hyprdrive" },
        keywords: ["hyprdrive", "overview", "mission control", "expose", "mosaic", "hot corner", "show desktop", "motion blur", "gestures", "swipe"] },

    "displays": { title: "Displays", category: "devices", file: "Displays", needs: { cap: "monitorConfig" },
        keywords: ["monitor", "screen", "resolution", "refresh rate", "scale", "rotation", "arrange", "mirror", "hidpi", "vrr", "adaptive sync", "hdr", "night light"] },
    "keyboard": { title: "Keyboard", category: "devices", file: "Keyboard",
        keywords: ["keyboard", "layout", "xkb", "repeat", "caps lock", "compose", "num lock", "dvorak", "remap"] },
    "mouse-touchpad": { title: "Mouse & touchpad", category: "devices", file: "MouseTouchpad",
        keywords: ["mouse", "touchpad", "trackpad", "pointer", "sensitivity", "acceleration", "scroll", "natural scrolling", "tap", "gestures", "buttons"] },

    "wifi": { title: "Wi-Fi & internet", category: "connections", file: "Wifi",
        keywords: ["wifi", "wi-fi", "wireless", "network", "internet", "ethernet", "hotspot", "forget", "airplane"] },
    "ethernet": { title: "Ethernet", category: "connections", file: "Ethernet",
        keywords: ["ethernet", "cable", "lan", "wired", "network", "ip address", "static ip", "dhcp", "gateway"] },
    "vpn": { title: "VPN", category: "connections", file: "Vpn",
        keywords: ["vpn", "wireguard", "openvpn", "tunnel", "wg", "private network", "import"] },
    "dns": { title: "DNS", category: "connections", file: "Dns",
        keywords: ["dns", "resolver", "cloudflare", "google", "nameserver", "1.1.1.1", "custom dns"] },
    "bluetooth": { title: "Bluetooth", category: "connections", file: "Bluetooth",
        keywords: ["bluetooth", "pair", "headphones", "devices"] },
    "phone": { title: "Phone", category: "connections", file: "Phone",
        keywords: ["phone", "android", "kde connect", "scrcpy", "mirror", "webcam", "adb"] },

    "installed-apps": { title: "Installed apps", category: "apps", file: "InstalledApps",
        keywords: ["installed apps", "apps", "applications", "programs", "software", "uninstall", "remove", "disk space",
                   "storage", "size", "flatpak", "aur", "packages", "gpk", "ryotunes", "ryoport", "ryomanager", "ryostore"] },
    "default-apps": { title: "Defaults", category: "apps", file: "DefaultApps",
        keywords: ["default apps", "defaults", "browser", "terminal", "editor", "file manager", "notes"] },
    "startup-apps": { title: "At sign-in", category: "apps", file: "StartupApps",
        keywords: ["startup apps", "at login", "at sign-in", "startup", "autostart", "login", "session", "environment", "variables", "env"] },
    "terminal": { title: "Terminal", category: "apps", file: "Terminal",
        keywords: ["terminal", "shell", "prompt", "starship", "fastfetch", "fish", "zsh", "bash"] },
    "screen-capture": { title: "Screen capture", category: "apps", file: "ScreenCapture",
        keywords: ["screenshot", "recording", "screencast", "capture", "video", "fps", "codec", "key press overlay"] },
    "voice-typing": { title: "Voice typing", category: "apps", file: "VoiceTyping",
        keywords: ["voice typing", "dictation", "speech", "whisper", "microphone", "stt"] },
    "rashin": { title: "Rashin", category: "apps", file: "Rashin",
        keywords: ["rashin", "agent", "ai", "ask", "chat", "models"] },
    "add-ons": { title: "Desktop add-ons", category: "apps", file: "AddOns",
        keywords: ["add-ons", "addons", "desktop add-ons", "plugins", "extensions", "store", "ryostore", "bundles", "omarchy"] },

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
        keywords: ["about", "version", "edition", "hardware", "credits"] },

    "updates": { title: "Updates", category: "updates", file: "Updates",
        keywords: ["update", "upgrade", "version", "what's new", "rollback", "snapshots"] }
};

// old ids and names that callers use today
var aliases = {
    "credits": "about", "global": "time-language", "connections": "wifi",
    "input": "keyboard", "cursor": "fonts-icons", "gpu": "graphics", "lockscreen": "lock-screen",
    "windowmanager": "window-behaviour", "windows": "window-behaviour", "plugins": "window-manager",
    "layerrules": "app-rules", "windowrules": "app-rules", "appoverrides": "app-rules",
    "subshells": "bar", "bar-studio": "bar", "frame": "bar", "desktop": "desktop-dock",
    "controls": "quick-settings", "sidebar": "quick-settings", "sidebars-left": "quick-settings",
    "sidebars-right": "quick-settings", "overlays": "notifications", "keybinds": "shortcuts",
    "session": "startup-apps", "autostart": "startup-apps", "environment": "startup-apps",
    "recording": "screen-capture", "dictation": "voice-typing", "fastfetch": "terminal",
    "import": "backup-import", "addons": "add-ons", "appearance": "wallpaper-colours",
    "gamebar": "game-bar", "installed": "installed-apps", "uninstall": "installed-apps", "packages": "installed-apps",
    "gpk": "installed-apps", "ryoku-apps": "installed-apps", "defaults": "default-apps", "at-login": "startup-apps",
    "desktop-add-ons": "add-ons", "at-sign-in": "startup-apps"
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
    return Object.assign({ id: id, glyph: glyphs[id] || "article" }, p);
}

// each category's and page's Japanese gloss, set beside its Latin name while the desktop's
// Japanese glosses are on. Latin names the thing, the kanji is its seal; every gloss is the
// real word, never decoration (the stable Hub's set where it had one)
var kanji = {
    "home": "表紙", "appearance": "外観", "desktop": "卓上", "windows": "窓", "devices": "機器",
    "connections": "接続", "apps": "応用", "accounts": "利用者", "system": "系統",
    "style": "様式", "wallpaper-colours": "壁紙", "window-style": "窓飾", "fonts-icons": "書体",
    "animations": "動き", "lock-screen": "施錠",
    "bar": "帯", "desktop-dock": "卓上", "quick-settings": "早設", "notifications": "通知",
    "sounds": "音", "launcher": "起動", "clipboard": "写し", "game-bar": "遊戯",
    "window-behaviour": "配置", "shortcuts": "操作", "app-rules": "規則", "window-manager": "合成",
    "displays": "画面", "keyboard": "鍵盤", "mouse-touchpad": "指針",
    "wifi": "無線", "ethernet": "有線", "vpn": "隧道", "dns": "名前", "bluetooth": "近接", "phone": "電話",
    "installed-apps": "導入", "default-apps": "既定", "startup-apps": "始動", "terminal": "端末", "screen-capture": "録画",
    "voice-typing": "音声", "rashin": "羅針", "add-ons": "拡張",
    "users": "人々", "sign-in": "認証",
    "power": "電力", "graphics": "演算", "performance": "性能", "time-language": "言語",
    "backup-import": "取込", "about": "概要", "updates": "更新"
};

function gloss(id) {
    return kanji[id || "home"] || "";
}

// each page's mark in the navigation tree (Material Symbols names)
var glyphs = {
    "style": "style", "wallpaper-colours": "wallpaper", "window-style": "rounded_corner", "fonts-icons": "match_case",
    "animations": "animation", "lock-screen": "lock",
    "bar": "toolbar", "desktop-dock": "dock_to_bottom", "quick-settings": "toggle_on",
    "notifications": "notifications", "sounds": "volume_up", "launcher": "manage_search",
    "clipboard": "content_paste", "game-bar": "sports_esports",
    "window-behaviour": "view_quilt", "shortcuts": "keyboard_command_key", "app-rules": "rule",
    "window-manager": "layers",
    "displays": "monitor", "keyboard": "keyboard", "mouse-touchpad": "mouse",
    "wifi": "wifi", "ethernet": "settings_ethernet", "vpn": "vpn_key", "dns": "dns", "bluetooth": "bluetooth", "phone": "smartphone",
    "installed-apps": "apps", "default-apps": "star", "startup-apps": "rocket_launch", "terminal": "terminal",
    "screen-capture": "screenshot_monitor", "voice-typing": "mic", "rashin": "explore", "add-ons": "extension",
    "users": "group", "sign-in": "fingerprint",
    "power": "battery_charging_full", "graphics": "developer_board", "performance": "speed",
    "time-language": "language", "backup-import": "backup", "about": "info",
    "updates": "system_update"
};

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

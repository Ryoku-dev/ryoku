.pragma library

// list-pane summary lines. ctx: { tr(s), settings(key), desktop(key),
// desktopReady, provider, barStyle, updateReady, updateKnown, monitors, battery }

function title(s) {
    s = String(s || "");
    return s.length ? s.charAt(0).toUpperCase() + s.slice(1) : "";
}

function onOff(ctx, v) { return v === true ? ctx.tr("on") : ctx.tr("off"); }

var still = {
    "wallpaper-colours": "Opens the wallpaper picker",
    "fonts-icons": "Interface and monospace fonts, icons, pointer",
    "animations": "Motion speed, window open and close",
    "lock-screen": "Lock screen design",
    "desktop-dock": "Widgets, dock and the desktop editor",
    "quick-settings": "Blocks, design and position",
    "notifications": "Design, images and pop-ups",
    "sounds": "Desktop and key sounds",
    "launcher": "Style, results and greeting",
    "clipboard": "Panel and history",
    "window-behaviour": "Tiling, gaps, focus and workspaces",
    "shortcuts": "Every shortcut and your own",
    "app-rules": "Rules for particular apps",
    "keyboard": "Layouts, typing and special keys",
    "mouse-touchpad": "Speed, scrolling and gestures",
    "wifi": "Networks and hotspot",
    "bluetooth": "Pair and connect devices",
    "phone": "Link an Android phone",
    "default-apps": "Browser, terminal, editor and files",
    "startup-apps": "Apps that start when you sign in",
    "terminal": "Login shell, prompt and greeting",
    "screen-capture": "Recording and screenshots",
    "voice-typing": "Type with your voice",
    "rashin": "The system assistant",
    "add-ons": "Installed add-ons and RyoStore",
    "users": "People on this computer",
    "sign-in": "Keyring, fingerprint and security keys",
    "power": "Power mode, battery and idle",
    "graphics": "Graphics mode and tuning",
    "performance": "Low power mode and idle parts",
    "time-language": "Language, formats and time zone",
    "backup-import": "Import, export and reset",
    "about": "This computer and credits",
    "displays": "Arrangement, modes and night light"
};

function pageSummary(id, ctx) {
    switch (id) {
    case "window-style":
        if (!ctx.desktopReady)
            return ctx.tr("Corners, borders, blur and shadows");
        var parts = [];
        var r = ctx.desktop("appearance.rounding");
        if (r !== undefined)
            parts.push(ctx.tr("%1 px corners").replace("%1", r));
        var b = ctx.desktop("appearance.borderSize");
        if (b !== undefined)
            parts.push(ctx.tr("%1 px border").replace("%1", b));
        var blur = ctx.desktop("appearance.blurEnabled");
        if (blur !== undefined)
            parts.push(blur ? ctx.tr("blur on") : ctx.tr("blur off"));
        return parts.join(", ");
    case "bar":
        return ctx.barStyle ? ctx.tr("%1 bar style").replace("%1", title(ctx.barStyle)) : ctx.tr("Bar style and parts");
    case "window-manager":
        return ctx.provider ? ctx.tr("Using %1").replace("%1", title(ctx.provider)) : ctx.tr("Which window manager runs");
    case "updates":
        if (!ctx.updateKnown)
            return ctx.tr("Checking for updates");
        return ctx.updateReady ? ctx.tr("An update is ready") : ctx.tr("Ryoku is up to date");
    }
    return still[id] ? ctx.tr(still[id]) : "";
}

function categorySentence(id, ctx) {
    switch (id) {
    case "appearance":
        if (ctx.desktopReady && ctx.desktop("appearance.blurEnabled") !== undefined)
            return ctx.desktop("appearance.blurEnabled")
                ? ctx.tr("Windows have %1 px corners and blur behind them.").replace("%1", ctx.desktop("appearance.rounding"))
                : ctx.tr("Windows have %1 px corners and no blur.").replace("%1", ctx.desktop("appearance.rounding"));
        return ctx.tr("How the desktop and its windows look.");
    case "desktop":
        return ctx.barStyle ? ctx.tr("The bar uses the %1 style.").replace("%1", title(ctx.barStyle))
                            : ctx.tr("The bar, dock, panels and pop-ups.");
    case "windows":
        return ctx.provider ? ctx.tr("%1 manages your windows.").replace("%1", title(ctx.provider))
                            : ctx.tr("How windows open, move and tile.");
    case "devices":
        var n = parseInt(ctx.monitors);
        if (n === 1)
            return ctx.tr("One display connected.");
        if (n > 1)
            return ctx.tr("%1 displays connected.").replace("%1", n);
        return ctx.tr("Displays, keyboard, mouse and touchpad.");
    case "connections":
        return ctx.tr("Wi-Fi, Bluetooth and your phone.");
    case "apps":
        return ctx.tr("The apps Ryoku opens and what starts with it.");
    case "accounts":
        return ctx.tr("People on this computer and how they sign in.");
    case "system":
        return ctx.battery && ctx.battery !== "-" ? ctx.tr("Battery at %1.").replace("%1", ctx.battery)
                                                  : ctx.tr("Power, graphics, language and backups.");
    case "updates":
        if (!ctx.updateKnown)
            return ctx.tr("Checking for updates.");
        return ctx.updateReady ? ctx.tr("An update is ready.") : ctx.tr("Ryoku is up to date.");
    }
    return "";
}

.pragma library

// list-pane summary lines. ctx: { tr(s), settings(key), desktop(key),
// value(bind) (any Changes bind; a command read runs once), topic(name) (a
// daemon topic's last frame, pushed), config(name) (a JSON file under
// ~/.config/ryoku, watched), text(path) (a plain file under ~/.config, undefined when
// absent or unread), textRead(path) (it was read), attention(id) (a Needs you item or null),
// bluetooth() ({ present, on, connected: [names] }), language() (the chosen
// language's own name), supports(cap), models(key) (the window manager writes
// this desktop store key), hostSupports(cap), desktopReady,
// provider, barStyle, version, distro, updateReady, updateKnown, serviceDown (the desktop
// service has not answered for a few seconds), monitors, battery }

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
    "game-bar": "Overlay, snapping and voice",
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
    "users": "People, passwords and account profiles",
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
        // the window manager's blur only where it draws one; the shell's panel glass everywhere
        if (ctx.models("desktop.appearance.blurEnabled")) {
            var blur = ctx.desktop("appearance.blurEnabled");
            if (blur !== undefined)
                parts.push(blur ? ctx.tr("window blur on") : ctx.tr("window blur off"));
        }
        parts.push(ctx.settings("theme.effects.glass") === false ? ctx.tr("panel glass off") : ctx.tr("panel glass on"));
        return parts.join(", ");
    case "bar":
        return ctx.barStyle ? ctx.tr("%1 bar style").replace("%1", title(ctx.barStyle)) : ctx.tr("Bar style and parts");
    case "window-manager":
        var sw = ctx.attention("wm-switch");
        if (sw && sw.args && sw.args.length > 0)
            return ctx.tr("Switching to %1 at next sign-in").replace("%1", title(sw.args[0]));
        return ctx.provider ? ctx.tr("%1 is running").replace("%1", title(ctx.provider)) : ctx.tr("Which window manager runs");
    case "window-behaviour":
        if (!ctx.desktopReady)
            return ctx.tr(still[id]);
        var gaps = ctx.desktop("appearance.gapsIn");
        var layouts = { dwindle: "Split layout", master: "Main and stack layout", scrolling: "Scrolling layout" };
        var layout = layouts[ctx.desktop("appearance.layout")];
        if (layout && gaps !== undefined)
            return ctx.tr(layout) + ", " + ctx.tr("%1 px gaps").replace("%1", gaps);
        if (gaps !== undefined)
            return ctx.tr("Gaps %1 px").replace("%1", gaps);
        return ctx.tr(still[id]);
    case "notifications":
        // the design's own name from RyoStore's catalogue (read once), its id until then
        var designId = ctx.settings("designs.notification") || "ryoku";
        var catalogue = ctx.value({ store: "command", get: ["ryostore", "catalog", "--category", "notifications"] });
        var item = catalogue && catalogue.items ? catalogue.items.find(i => i.id === designId) : null;
        var design = designId === "ryoku" ? "Ryoku" : item && item.name ? item.name : title(designId);
        return design + ", " + (ctx.settings("notifications.show_images") === true ? ctx.tr("images shown") : ctx.tr("no images"));
    case "sounds":
        var line = ctx.settings("sounds.enabled") === false ? ctx.tr("Desktop sounds off") : ctx.tr("Desktop sounds on");
        var keys = ctx.provider ? ctx.desktop("wm." + ctx.provider + ".keysounds.enabled") : undefined;
        if (keys === undefined)
            keys = ctx.desktop("wm.hyprland.plugins.keysounds.enabled");
        return keys === true ? line + ", " + ctx.tr("key sounds on") : line;
    case "launcher":
        var launcher = ctx.config("launcher.json");
        if (launcher === undefined)
            return ctx.tr(still[id]);
        var variants = { hero: "Hero", main: "Main", okshell: "OkShell" };
        var variant = launcher.variant || "hero";
        return ctx.tr("%1 style").replace("%1", variants[variant] || title(variant));
    case "wifi":
        var net = ctx.topic("network");
        if (!net || !net.wifi)
            return ctx.tr(still[id]);
        if (net.wired && net.wired.connectivity === "Connected")
            return ctx.tr("Wired connection");
        if (net.wifi.enabled === false)
            return ctx.tr("Wi-Fi off");
        return net.wifi.ssid ? ctx.tr("Connected to %1").replace("%1", net.wifi.ssid) : ctx.tr("Not connected");
    case "bluetooth":
        var bt = ctx.bluetooth();
        if (!bt.present)
            return ctx.tr("Not available");
        if (!bt.on)
            return ctx.tr("Off");
        if (bt.connected.length === 1)
            return ctx.tr("%1 connected").replace("%1", bt.connected[0]);
        if (bt.connected.length > 1)
            return ctx.tr("%1 devices connected").replace("%1", bt.connected.length);
        return ctx.tr("On, nothing connected");
    case "phone":
        if (ctx.settings("phone.enabled") !== true)
            return ctx.tr("Off");
        var st = ctx.value({ store: "command", get: ["ryoku-hub", "phone", "status"] });
        if (!st || typeof st !== "object")
            return ctx.tr("On");
        if (!st.installed)
            return ctx.tr("Phone tools are missing");
        var phone = st.primaryDevice;
        if (!phone)
            return ctx.tr("On, no phone paired");
        var phoneName = phone.name || phone.id;
        return phone.reachable ? ctx.tr("Connected to %1").replace("%1", phoneName)
                               : ctx.tr("%1 is not connected").replace("%1", phoneName);
    case "keyboard":
        var kb = ctx.desktop("input.kbLayout");
        if (kb === undefined || kb === null)
            return ctx.tr(still[id]);
        var codes = String(kb).split(",").map(c => c.trim()).filter(c => c.length > 0);
        if (codes.length === 0)
            return ctx.tr(still[id]);
        // xkb's own names ("German (Austria)"), read once; the codes stand in until they arrive
        var known = ctx.value({ store: "command", get: ["ryoku-hub", "desktop", "layouts"] });
        var names = codes.map(c => {
            var hit = Array.isArray(known) ? known.find(l => l && l.code === c) : null;
            return hit && hit.name ? ctx.tr(hit.name) : c.toUpperCase();
        });
        return names.length === 1 ? ctx.tr("%1 layout").replace("%1", names[0])
                                  : ctx.tr("%1 layouts").replace("%1", joined(ctx, names));
    case "mouse-touchpad":
        var bits = [];
        if (ctx.desktop("input.tapToClick") === true)
            bits.push(ctx.tr("tap to click"));
        if (ctx.desktop("input.naturalScroll") === true)
            bits.push(ctx.tr("natural scrolling"));
        var mouse = ctx.topic("mouse");
        // maps is device -> button -> target
        var mapped = mouse && mouse.maps ? Object.keys(mouse.maps).reduce((n, d) => n + Object.keys(mouse.maps[d] || {}).length, 0) : 0;
        if (mapped === 1)
            bits.push(ctx.tr("1 mouse button remapped"));
        else if (mapped > 1)
            bits.push(ctx.tr("%1 mouse buttons remapped").replace("%1", mapped));
        return bits.length > 0 ? title(bits.join(", ")) : ctx.tr(still[id]);
    case "screen-capture":
        if (ctx.value({ store: "recording", key: "status" }) === "ready")
            return ctx.tr("Records at %1 fps").replace("%1", ctx.value({ store: "recording", key: "fps" }));
        return ctx.tr(still[id]);
    case "voice-typing":
        if (ctx.value({ store: "dictation", key: "loaded" }) !== true)
            return ctx.tr(still[id]);
        if (ctx.value({ store: "dictation", key: "installed" }) !== true)
            return ctx.tr("Not installed");
        return ctx.value({ store: "dictation", key: "enabled" }) === true
            ? ctx.tr("On, %1").replace("%1", ctx.tr(String(ctx.value({ store: "dictation", key: "modelLabel" }) || "")))
            : ctx.tr("Off");
    case "rashin":
        var rashinStates = { absent: "Not installed", off: "Off", idle: "On", running: "On and running" };
        var rs = rashinStates[ctx.value({ store: "rashin", key: "state" })];
        return ctx.tr(rs || still[id]);
    case "add-ons":
        var plugins = ctx.config("plugins.json");
        if (!plugins || typeof plugins !== "object")
            return ctx.tr(still[id]);
        var addons = Object.keys(plugins).filter(k => plugins[k] && plugins[k].enabled === true && plugins[k].host !== "desktopWidget").length;
        if (addons === 0)
            return ctx.tr(still[id]);
        return addons === 1 ? ctx.tr("One add-on on") : ctx.tr("%1 add-ons on").replace("%1", addons);
    case "app-rules":
        if (!ctx.desktopReady)
            return ctx.tr(still[id]);
        // one rule per app and title it matches; a rule may name only a title, so rules are counted
        var ruled = {};
        (ctx.desktop("windowRules") || []).concat(ctx.desktop("appOverrides") || []).forEach(r => {
            if (r)
                ruled[(r["class"] || "") + "\n" + (r.title || "")] = true;
        });
        var rules = Object.keys(ruled).length;
        return rules === 0 ? ctx.tr("No rules yet")
            : rules === 1 ? ctx.tr("One rule") : ctx.tr("%1 rules").replace("%1", rules);
    case "power":
        var pp = ctx.topic("powerprofiles");
        var modes = { "power-saver": "Power saver", balanced: "Balanced", performance: "Performance" };
        var mode = pp ? modes[pp.active_profile] : undefined;
        var battery = ctx.battery && ctx.battery !== "-" ? ctx.battery : "";
        if (mode && battery)
            return ctx.tr("%1, battery at %2").replace("%1", ctx.tr(mode)).replace("%2", battery);
        if (mode)
            return ctx.tr("%1 power mode").replace("%1", ctx.tr(mode));
        if (battery)
            return ctx.tr("Battery at %1").replace("%1", battery);
        return ctx.tr(still[id]);
    case "graphics":
        var gm = ctx.attention("graphics-mode");
        if (gm && gm.args && gm.args.length > 0)
            return ctx.tr("Switching to %1 after you log out").replace("%1", ctx.tr(gm.args[0]));
        return ctx.tr(still[id]);
    case "performance":
        var perf = ctx.config("performance.json");
        if (!perf || typeof perf !== "object")
            return ctx.tr(still[id]);
        if (perf.lowPowerMode === true)
            return ctx.tr("Low power mode on");
        return perf.unloadWidgetsWhenCovered === false ? ctx.tr("Low power mode off") : ctx.tr("Low power mode off, idle parts unload");
    case "default-apps":
        var roles = ctx.desktop("apps");
        var picks = roles && typeof roles === "object"
            ? Object.keys(roles).map(k => String(roles[k] || "").trim().split(/\s+/)[0]).filter(c => c.length > 0)
            : [];
        if (!ctx.desktopReady)
            return ctx.tr(still[id]);
        if (picks.length === 0)
            return ctx.tr("Ryoku's picks for browser, terminal and files");
        return joined(ctx, picks.slice(0, 3).map(c => title(c.replace(/^.*\//, ""))));
    case "startup-apps":
        if (!ctx.desktopReady)
            return ctx.tr(still[id]);
        var starts = (ctx.desktop("autostart") || []).filter(a => a && String(a.command || "").trim()).length;
        return starts === 0 ? ctx.tr("Nothing extra starts at sign-in")
            : starts === 1 ? ctx.tr("One app starts at sign-in") : ctx.tr("%1 apps start at sign-in").replace("%1", starts);
    case "terminal":
        var sh = ctx.value({ store: "command", get: ["ryoku-hub", "shell", "get"] });
        var prompt = ctx.value({ store: "command", get: ["ryoku-hub", "starship", "get"] });
        if (!sh || !sh.current || !prompt || !prompt.layout)
            return ctx.tr(still[id]);
        var shells = { fish: "Fish", bash: "Bash", zsh: "Zsh" };
        var prompts = { pill: "Pill", minimal: "Minimal", twoline: "Two line", "two-line": "Two line", powerline: "Powerline", lean: "Lean" };
        return ctx.tr("%1 with the %2 prompt").replace("%1", shells[sh.current] || title(sh.current))
            .replace("%2", ctx.tr(prompts[prompt.layout] || title(prompt.layout)));
    case "fonts-icons":
        var font = ctx.settings("fontFamily"), size = ctx.settings("fontSize");
        if (!font)
            return ctx.tr(still[id]);
        return size ? ctx.tr("%1 at %2 pt").replace("%1", font).replace("%2", size) : String(font);
    case "animations":
        if (ctx.settings("theme.motion.reduce") === true)
            return ctx.tr("Reduce motion on");
        if (ctx.desktopReady && ctx.desktop("appearance.animations") === false)
            return ctx.tr("Window animations off");
        var speed = Number(ctx.settings("theme.motion.scale")) || 1;
        return Math.abs(speed - 1) < 0.001 ? ctx.tr("Normal speed") : ctx.tr("Speed %1%").replace("%1", Math.round(speed * 100));
    case "time-language":
        var zone = ctx.value({ store: "timezone", key: "zone" });
        return zone ? ctx.language() + " · " + String(zone).replace(/_/g, " ") : ctx.language();
    case "about":
        if (!ctx.version || !ctx.distro || ctx.distro === "Linux")
            return ctx.tr(still[id]);
        return ctx.tr("Ryoku %1 on %2").replace("%1", ctx.version).replace("%2", ctx.distro);
    case "updates":
        if (!ctx.updateKnown)
            return ctx.serviceDown ? ctx.tr("Updates can't be checked right now") : ctx.tr("Checking for updates");
        if (ctx.updateReady)
            return ctx.latest ? ctx.tr("An update is ready: %1").replace("%1", ctx.latest) : ctx.tr("An update is ready");
        return ctx.tr("Ryoku is up to date");
    case "desktop-dock":
        var dock = ctx.settings("dock.enabled") === true ? ctx.tr("Dock on") : ctx.tr("Dock off");
        var perDisplay = ctx.settings("displays.widgets") || {};
        var widgetsOff = Object.keys(perDisplay).filter(k => perDisplay[k] === false).length;
        return dock + ", " + (widgetsOff === 1 ? ctx.tr("widgets off on 1 display")
            : widgetsOff > 1 ? ctx.tr("widgets off on %1 displays").replace("%1", widgetsOff) : ctx.tr("widgets on every display"));
    case "quick-settings":
        var corners = { "top-left": "Top left", "top-center": "Top centre", "top-right": "Top right", center: "Centre" };
        var where = corners[ctx.settings("controls.position") || "top-left"] || "Top left";
        var gap = ctx.settings("controls.edgeGap");
        return ctx.tr(where) + ", " + ctx.tr("%1 px from the edges").replace("%1", gap === undefined || gap === null ? 12 : gap);
    case "clipboard":
        var width = ctx.settings("clipboard.widthPercent");
        return ctx.tr("Panel %1% wide").replace("%1", width === undefined || width === null ? 65 : width) + ", "
            + (ctx.settings("clipboard.pruneWeekly") === true ? ctx.tr("cleared every week") : ctx.tr("kept until you clear it"));
    case "game-bar":
        // shell.json is read once the daemon answers; until then the fixed line
        var playing = ctx.settings("gamebar.enabled");
        if (playing === undefined || playing === null)
            return ctx.tr(still[id]);
        if (playing === false)
            return ctx.tr("Off");
        var rawDim = ctx.settings("gamebar.dim");
        var dim = Math.round((rawDim === undefined || rawDim === null ? 0.62 : Number(rawDim)) * 100);
        return dim === 0 ? ctx.tr("On, the game is not dimmed") : ctx.tr("On, dims the game by %1%").replace("%1", dim);
    case "lock-screen":
        // the page's store once it is open (it lists the installed designs), else the theme file;
        // no theme file means the shipped design, as the page says when that design is installed
        var inUse = ctx.value({ store: "lockdesign", key: "active" });
        var theme = inUse !== undefined ? inUse : ctx.text("qylock/theme");
        if (theme === undefined && ctx.textRead("qylock/theme"))
            theme = "clockwork/orbital";
        if (!theme)
            return ctx.tr(still[id]);
        var slug = theme.split("/").filter(s => s.length > 0).pop() || "orbital";
        // "man-bicycle" reads "Man Bicycle", as the design's own name does
        return slug.split(/[-_]+/).filter(w => w.length > 0).map(title).join(" ");
    case "sign-in":
        // the page's store once it is open (it reads `ryoku keyring status`), else the config file
        var storeMode = ctx.value({ store: "signin", key: "keyring.mode" });
        var keyring = ctx.config("keyring.json");
        var mode = storeMode || (keyring ? keyring.mode : "");
        var keyModes = { "unlock-on-login": "Keyring opens at sign-in", "never-ask": "Keyring never asks", ask: "Keyring asks each time" };
        return ctx.tr(keyModes[mode] || still[id]);
    }
    return still[id] ? ctx.tr(still[id]) : "";
}

// "A, B and C", in the current language
function joined(ctx, items) {
    if (items.length <= 1)
        return items.join("");
    return ctx.tr("%1 and %2").replace("%1", items.slice(0, -1).join(", ")).replace("%2", items[items.length - 1]);
}

function categorySentence(id, ctx) {
    switch (id) {
    case "appearance":
        if (ctx.desktopReady && ctx.desktop("appearance.rounding") !== undefined) {
            var corners = ctx.desktop("appearance.rounding");
            var glass = ctx.settings("theme.effects.glass") === false
                ? ctx.tr("Panels and the dock are not frosted.") : ctx.tr("Panels and the dock are frosted.");
            var windowBlur = ctx.models("desktop.appearance.blurEnabled") ? ctx.desktop("appearance.blurEnabled") : undefined;
            var shape = windowBlur === undefined ? ctx.tr("Windows have %1 px corners.")
                : windowBlur ? ctx.tr("Windows have %1 px corners and blur behind them.")
                : ctx.tr("Windows have %1 px corners and no blur.");
            return shape.replace("%1", corners) + " " + glass;
        }
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
        var link = ctx.topic("network");
        if (!link || !link.wifi)
            return ctx.tr("Wi-Fi, Bluetooth and your phone.");
        var said = link.wired && link.wired.connectivity === "Connected" ? ctx.tr("Wired connection.")
            : link.wifi.ssid ? ctx.tr("Connected to %1.").replace("%1", link.wifi.ssid) : ctx.tr("Not connected.");
        var blue = ctx.bluetooth();
        return blue.present ? said + " " + (blue.on ? ctx.tr("Bluetooth is on.") : ctx.tr("Bluetooth is off.")) : said;
    case "apps":
        return ctx.tr("The apps Ryoku opens and what starts with it.");
    case "accounts":
        return ctx.tr("People on this computer and how they sign in.");
    case "system":
        return ctx.battery && ctx.battery !== "-" ? ctx.tr("Battery at %1.").replace("%1", ctx.battery)
                                                  : ctx.tr("Power, graphics, language and backups.");
    case "updates":
        if (!ctx.updateKnown)
            return ctx.serviceDown ? ctx.tr("Updates can't be checked right now.") : ctx.tr("Checking for updates.");
        return ctx.updateReady ? ctx.tr("An update is ready.") : ctx.tr("Ryoku is up to date.");
    }
    return "";
}

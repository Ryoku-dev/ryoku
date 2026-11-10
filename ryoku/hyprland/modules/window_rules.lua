-- A floating window sized past the monitor reads as a broken fullscreen (issue
-- 147: Files on a 1366x768 panel). Every fixed size below is capped to the
-- monitor the window opens on; Hyprland evaluates the expressions per window.
local function fit(w, h)
    return {
        "min(" .. w .. ", monitor_w * 0.92)",
        "min(" .. h .. ", monitor_h * 0.88)",
    }
end

local settings_window = { mode = "full", width = 0.5, height = 0.5 }
if package.searchpath == nil or package.searchpath("settings_window", package.path) then
    local ok, configured = pcall(require, "settings_window")
    if ok and type(configured) == "table" then settings_window = configured end
end

local function settings_fraction(value)
    return math.max(0.25, math.min(tonumber(value) or 0.5, 1))
end

-- The Settings layout needs this much room to remain usable.
local SETTINGS_MIN_WIDTH, SETTINGS_MIN_HEIGHT = 1120, 820

local function page()
    if settings_window.mode == "custom" then
        return {
            "monitor_w * " .. settings_fraction(settings_window.width),
            "monitor_h * " .. settings_fraction(settings_window.height),
        }
    end
    return { "monitor_w * 0.99", "monitor_h * 0.96" }
end

-- Rule expressions only see the whole monitor. Once the Hub opens, fit its full
-- page or custom fraction inside the work area with the same gaps and border a
-- tiled window gets.
local function fit_page_to_work_area(win)
    if not win or win.title ~= "Ryoku Settings" then return end
    local mon = win.monitor
    if not mon then return end
    local scale = mon.scale or 1
    local mw, mh = mon.width / scale, mon.height / scale
    if (mon.transform or 0) % 2 == 1 then mw, mh = mh, mw end
    local r = mon.reserved
    local gap = hl.get_config("general.gaps_out")
    local border = hl.get_config("general.border_size") or 0
    local left = r.left + gap.left + border
    local top = r.top + gap.top + border
    local right = r.right + gap.right + border
    local bottom = r.bottom + gap.bottom + border
    local aw, ah = mw - left - right, mh - top - bottom
    local width, height = aw, ah
    if settings_window.mode == "custom" then
        width = math.max(aw * settings_fraction(settings_window.width), math.min(SETTINGS_MIN_WIDTH, aw))
        height = math.max(ah * settings_fraction(settings_window.height), math.min(SETTINGS_MIN_HEIGHT, ah))
    end
    local x = mon.x + left + (aw - width) / 2
    local y = mon.y + top + (ah - height) / 2
    local target = "address:" .. win.address
    hl.dispatch(hl.dsp.window.resize({ x = math.floor(width + 0.5), y = math.floor(height + 0.5), exact = true, window = target }))
    hl.dispatch(hl.dsp.window.move({ x = math.floor(x + 0.5), y = math.floor(y + 0.5), window = target }))
end
hl.on("window.open", fit_page_to_work_area)

hl.window_rule({
    name           = "suppress-maximize",
    match          = { class = ".*" },
    suppress_event = "maximize",
})

hl.window_rule({
    name  = "fix-xwayland-drags",
    match = {
        class      = "^$",
        title      = "^$",
        xwayland   = true,
        float      = true,
        fullscreen = false,
        pin        = false,
    },
    no_focus = true,
})

hl.window_rule({
    name  = "float-system-dialogs",
    match = { class = "(pavucontrol|nm-connection-editor|blueman-manager|org.kde.polkit-kde-authentication-agent-1|xdg-desktop-portal-gtk)" },
    float = true,
})

hl.window_rule({
    name   = "float-polkit-agent",
    match  = { class = "hyprpolkitagent" },
    float  = true,
    center = true,
})

hl.window_rule({
    name  = "float-file-pickers",
    match = { title = "(Open File|Save File|Save As|Choose Files|Open Folder)" },
    float = true,
})

hl.window_rule({
    name  = "float-ghosttype",
    match = { class = "Ghosttype-app" },
    float = true,
})

hl.window_rule({
    name  = "float-spotify",
    match = { class = "[Ss]potify" },
    float = true,
})

hl.window_rule({
    name   = "float-nautilus",
    match  = { class = "org.gnome.Nautilus" },
    float  = true,
    size   = fit(1500, 850),
    center = true,
})

hl.window_rule({
	name   = "float-ryoku-settings",
	match  = { title = "^(Ryoku Settings)$" },
	float  = true,
	size   = page(),
	center = true,
})


hl.window_rule({
    name   = "float-rashin-app",
    match  = { title = "^(Rashin)$" },
    float  = true,
    size   = fit(1280, 820),
    center = true,
})
hl.window_rule({
    name   = "float-phone-tools",
    match  = { title = "^ryoku-phone-.*$" },
    float  = true,
    size   = fit(520, 860),
    center = true,
})


hl.window_rule({
    -- The update and rollback logs are started from Ryoku Settings itself
    -- (UpdatesPage, and the bar's update widget), and a tiled window always
    -- sits under a float, so the run's output hid behind the 99% settings
    -- page until it finished (issue 288). The launches name this GTK-valid
    -- class (see the ryoport rule below for why a dot is required) and this
    -- rule floats and centres the log, where the user can watch it.
    name   = "float-ryoku-update",
    match  = { class = "^dev\\.ryoku\\.update$" },
    float  = true,
    size   = fit(1180, 760),
    center = true,
})

hl.window_rule({
    name   = "float-ryostore",
    match  = { title = "^(Ryostore)$" },
    float  = true,
    size   = fit(1180, 760),
    center = true,
})

hl.window_rule({
    name   = "float-ryoport",
    match  = { class = "^dev\\.ryoku\\.ryoport$" },
    float  = true,
    size   = fit(1180, 760),
    center = true,
    no_anim = true,
})

hl.window_rule({
    -- The launch asks every terminal for this id (see ryossh.go's ryoportAppID):
    -- Ghostty validates --class as a GTK application id and rejects a name
    -- without a dot, so a bare "ryoport-ssh" never reached the window and this
    -- rule could not match it.
    name   = "float-ryoport-ssh",
    match  = { class = "^dev\\.ryoku\\.ryoport_ssh$" },
    float  = true,
    size   = fit(900, 560),
    center = true,
})

hl.window_rule({
    -- SPICE (the VM yard) and the RDP berth (which asks FreeRDP to carry this
    -- exact class via /wm-class; see ryossh.go) both open as centred floats.
    name   = "float-ryoport-console",
    match  = { class = "^(spicy|dev\\.ryoku\\.ryoport_console)$" },
    float  = true,
    center = true,
})

hl.window_rule({
    -- TigerVNC names its own class: instance "vncviewer", class "Vncviewer".
    name   = "float-ryoport-vnc",
    match  = { class = "^Vncviewer$" },
    float  = true,
    center = true,
})

hl.window_rule({
    name   = "float-ryostore",
    match  = { class = "ryostore" },
    float  = true,
    size   = fit(900, 600),
    center = true,
})

hl.window_rule({
    name   = "float-ryoku-rashin-setup",
    match  = { class = "ryoku-rashin-setup" },
    float  = true,
    size   = fit(900, 600),
    center = true,
})

hl.window_rule({
    name   = "float-looking-glass",
    match  = { class = "looking-glass-client" },
    float  = true,
    size   = fit(1600, 900),
    center = true,
})

hl.window_rule({
    name   = "float-qemu",
    match  = { class = "[Qq]emu" },
    float  = true,
    size   = fit(1280, 800),
    center = true,
})

hl.window_rule({
    name   = "float-ryoku-welcome",
    match  = { title = "^(Welcome to Ryoku)$" },
    float  = true,
    size   = fit(1180, 760),
    center = true,
})

-- Steam is an XWayland app: Big Picture and the client (both class "steam"),
-- launched games (steam_app_<id>) and gamescope otherwise inherit the desktop's
-- blur and shadow (per-frame GPU cost, a floating-card look) and the 0.94
-- inactive opacity, which turns a game translucent the moment focus leaves it.
-- Strip that chrome and force them opaque so they read like a native fullscreen
-- app, and inhibit idle while fullscreen so controller-only play never dims or
-- locks (hypridle has no fullscreen exception). steamwebhelper stays untouched.
hl.window_rule({
    name         = "steam-native",
    match        = { class = "^(steam|steam_app_.*|gamescope)$" },
    no_blur      = true,
    no_shadow    = true,
    opaque       = true,
    idle_inhibit = "fullscreen",
    -- low-latency presentation: let an unthrottled fullscreen game tear instead
    -- of vsyncing through the compositor, which cuts input lag and the frame-time
    -- hit when the Steam overlay composites on top. Pairs with
    -- general.allow_tearing; Game Mode already forces this path at runtime.
    immediate    = true,
})

-- The legacy Quickshell Ryotunes client maps with Quickshell's class and the
-- title "Ryotunes", so it still needs its compositor floating rule. The Go app
-- chooses floating or tiled itself.
hl.window_rule({
    name  = "float-ryotunes-qml",
    match = { class = "^org\\.quickshell$", title = "^Ryotunes$" },
    float = true,
})

-- Chromium's "<site> is sharing a window." bar. On native Wayland it maps with an
-- empty app_id and its geometry computed against the wrong work area (Chromium
-- 517327175), so it lands mid-screen and takes no pointer input at all: verified
-- with the cursor on Stop sharing and on Hide, neither fires. Hyprland's Lua
-- rules have no positioning key, so parking it on a special workspace nobody
-- opens is the only way to stop a dead widget covering the desktop. Chromium's
-- own tab indicator and the site's stop control still show a capture is live.
-- The patterns are FULL matches, not searches, hence the .* on both ends.
hl.window_rule({
    name  = "park-chromium-share-indicator",
    match = {
        class = "^$",
        title = "^(.*is sharing.*)$",
    },
    -- "silent" is load-bearing: without it the rule OPENS special:sharebar on the
    -- monitor and the bar stays on screen, parked but visible.
    workspace = "special:sharebar silent",
    no_focus  = true,
})

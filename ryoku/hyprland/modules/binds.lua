local mod = "SUPER"

local K = require("modules.rebind")

-- The Shima frame can ask before a window closes. Only route through the shell
-- when that is switched on, so every other close stays a native dispatch with
-- no process in the way (reading shell.json here costs no fork).
local function close_active()
    local f = io.open((os.getenv("XDG_CONFIG_HOME") or (os.getenv("HOME") .. "/.config")) .. "/ryoku/shell.json")
    local s = f and f:read("a") or ""
    if f then f:close() end
    local style = s:match('"barStyle"%s*:%s*"([^"]*)"') or "orpheus"
    if style == "iris" and s:find('"closeConfirm"%s*:%s*{[^{}]*"enabled"%s*:%s*true') then
        hl.exec_cmd("qs -c shell ipc call closeConfirm trigger")
    else
        hl.dispatch(hl.dsp.window.close())
    end
end

-- Windows
hl.bind(K(mod .. " + Q"),         close_active)                                    -- close active window (Shima asks first when set to)
hl.bind(K(mod .. " + F"),         hl.dsp.window.fullscreen())                      -- fullscreen
hl.bind(K(mod .. " + SHIFT + P"), hl.dsp.window.pin())                             -- pin a floating window
hl.bind(K(mod .. " + A"),         function() hl.dispatch(hl.dsp.window.float({ action = "toggle" })); hl.dispatch(hl.dsp.window.resize({ x = 1000, y = 660, exact = true })); hl.dispatch(hl.dsp.window.center()) end) -- float at 1000x660, centred (press again to tile back)
hl.bind(K(mod .. " + R"),         function() hl.dispatch(hl.dsp.submap("resize")); hl.dispatch(hl.dsp.exec_cmd("hyprctl notify -1 2200 0 'Resize mode: arrows or hjkl resize, Esc exits'")) end) -- resize mode (arrows/hjkl resize, Esc exits)
hl.bind(K(mod .. " + T"),         hl.dsp.group.toggle())                            -- Group windows (tabbed)
hl.bind(K(mod .. " + D"),         hl.dsp.window.fullscreen({ mode = "maximized" })) -- Maximise
hl.bind(K(mod .. " + C"),         hl.dsp.window.center())                           -- Centre
hl.bind(K("ALT + Tab"),           hl.dsp.focus({ last = true }))                    -- Last window
hl.bind(K(mod .. " + P"),         hl.dsp.exec_cmd("ryoku-monitor toggle"))         -- mirror or extend displays (monitor layout)

-- Directional focus keeps working while a window is maximized or fullscreen.
-- With the default (off) the compositor refuses every direction focus from an
-- FS window: the scrolling layout paints it over the whole workspace, so the
-- neighbour on the tape is unreachable until the state clears. On, a direction
-- move focuses the neighbour and leaves the maximized window maximized.
hl.config({ binds = { movefocus_cycles_fullscreen = true } })

-- A fullscreen cycle cannot cross outputs, so take an adjacent screen first.
local function focus_direction(dir)
    local win = hl.get_active_window()
    local mon = win and win.fullscreen ~= 0 and hl.get_active_monitor() or nil
    if mon then
        local position = type(mon.position) == "table" and mon.position or {}
        local size = type(mon.size) == "table" and mon.size or {}
        local mon_x = mon.x or position.x
        local mon_y = mon.y or position.y
        local mon_width = mon.width or size.width or size.x
        local mon_height = mon.height or size.height or size.y
        if mon_x and mon_y and mon_width and mon_height then
            local x = mon_x + mon_width / 2
            local y = mon_y + mon_height / 2
            if dir == "left" then x = mon_x - 1
            elseif dir == "right" then x = mon_x + mon_width + 1
            elseif dir == "up" then y = mon_y - 1
            else y = mon_y + mon_height + 1 end
            local next_mon = hl.get_monitor_at(x, y)
            if next_mon and next_mon.id ~= mon.id then
                hl.dispatch(hl.dsp.focus({ monitor = dir:sub(1, 1) }))
                return
            end
        end
    end
    hl.dispatch(hl.dsp.focus({ direction = dir }))
end

-- Focus and move windows
hl.bind(K(mod .. " + Left"),          function() focus_direction("left") end)       -- focus left
hl.bind(K(mod .. " + Right"),         function() focus_direction("right") end)      -- focus right
hl.bind(K(mod .. " + Up"),            function() focus_direction("up") end)         -- focus up
hl.bind(K(mod .. " + Down"),          function() focus_direction("down") end)       -- focus down
hl.bind(K(mod .. " + SHIFT + Left"),  hl.dsp.window.move({ direction = "left" }))  -- move window left
hl.bind(K(mod .. " + SHIFT + Right"), hl.dsp.window.move({ direction = "right" })) -- move window right
hl.bind(K(mod .. " + SHIFT + Up"),    hl.dsp.window.move({ direction = "up" }))    -- move window up
hl.bind(K(mod .. " + SHIFT + Down"),  hl.dsp.window.move({ direction = "down" }))  -- move window down
hl.bind(K(mod .. " + CTRL + Left"),   hl.dsp.window.resize({ x = -40, y = 0,   relative = true }), { repeating = true }) -- resize window narrower
hl.bind(K(mod .. " + CTRL + Right"),  hl.dsp.window.resize({ x = 40,  y = 0,   relative = true }), { repeating = true }) -- resize window wider
hl.bind(K(mod .. " + CTRL + Up"),     hl.dsp.window.resize({ x = 0,   y = -40, relative = true }), { repeating = true }) -- resize window shorter
hl.bind(K(mod .. " + CTRL + Down"),   hl.dsp.window.resize({ x = 0,   y = 40,  relative = true }), { repeating = true }) -- resize window taller
hl.bind(K(mod .. " + bracketleft"),  hl.dsp.window.move({ direction = "left",  group_aware = true }))  -- Merge left
hl.bind(K(mod .. " + bracketright"), hl.dsp.window.move({ direction = "right", group_aware = true }))  -- Merge right

-- Displays (multi-monitor). The l/r/u/d selector picks the monitor in that
-- direction from the focused one.
hl.bind(K(mod .. " + ALT + Left"),          hl.dsp.focus({ monitor = "l" }))          -- Focus screen left
hl.bind(K(mod .. " + ALT + Right"),         hl.dsp.focus({ monitor = "r" }))          -- Focus screen right
hl.bind(K(mod .. " + ALT + Up"),            hl.dsp.focus({ monitor = "u" }))          -- Focus screen up
hl.bind(K(mod .. " + ALT + Down"),          hl.dsp.focus({ monitor = "d" }))          -- Focus screen down
hl.bind(K(mod .. " + ALT + SHIFT + Left"),  hl.dsp.window.move({ monitor = "l" }))    -- Send window to screen left
hl.bind(K(mod .. " + ALT + SHIFT + Right"), hl.dsp.window.move({ monitor = "r" }))    -- Send window to screen right
hl.bind(K(mod .. " + ALT + SHIFT + Up"),    hl.dsp.window.move({ monitor = "u" }))    -- Send window to screen up
hl.bind(K(mod .. " + ALT + SHIFT + Down"),  hl.dsp.window.move({ monitor = "d" }))    -- Send window to screen down
hl.bind(K(mod .. " + CTRL + ALT + Left"),   hl.dsp.workspace.move({ monitor = "l" })) -- Send workspace to screen left
hl.bind(K(mod .. " + CTRL + ALT + Right"),  hl.dsp.workspace.move({ monitor = "r" })) -- Send workspace to screen right
hl.bind(K(mod .. " + CTRL + ALT + Up"),     hl.dsp.workspace.move({ monitor = "u" })) -- Send workspace to screen up
hl.bind(K(mod .. " + CTRL + ALT + Down"),   hl.dsp.workspace.move({ monitor = "d" })) -- Send workspace to screen down

-- Apps
hl.bind(K(mod .. " + Return"),    hl.dsp.exec_cmd("ryoku-app terminal"))           -- terminal
hl.bind(K(mod .. " + E"),         hl.dsp.exec_cmd("ryoku-app files"))              -- file manager
hl.bind(K(mod .. " + B"),         hl.dsp.exec_cmd("ryoku-app browser"))            -- browser
hl.bind(K(mod .. " + N"),         hl.dsp.exec_cmd("ryoku-app editor"))             -- editor
hl.bind(K(mod .. " + O"),         hl.dsp.exec_cmd("ryoku-app notes"))              -- notes
hl.bind(K(mod .. " + ALT + E"),   hl.dsp.exec_cmd("kitty -e yazi"))               -- yazi file manager
hl.bind(K(mod .. " + J"),         hl.dsp.exec_cmd("ryotunes"))                     -- open Ryotunes (single-instance: a second press focuses it)

-- Shell surfaces and tools
hl.bind(K(mod .. " + Space"),     hl.dsp.global("ryoku:launcher"))                 -- open the app launcher
hl.bind(K("ALT + Space"),         hl.dsp.global("ryoku:ask"))                      -- ask Rashin
hl.bind(K(mod .. " + ALT + Space"), hl.dsp.exec_cmd("ryoku-summon Rashin flock -n -o /tmp/rashin-app.lock rashin-app")) -- Rashin app: the AI companion window (a second press focuses it)
hl.bind(K(mod .. " + K"),         hl.dsp.exec_cmd("pkill -x -f 'qs -c keys' 2>/dev/null || flock -n -o /tmp/ryoku-keys.lock qs -c keys")) -- Kit: toggle (press to open, press again to close)
hl.bind(K(mod .. " + L"),         hl.dsp.exec_cmd("ryoku-shell lock"))             -- lock the screen
hl.bind(K(mod .. " + Escape"),    hl.dsp.global("ryoku:quicksettings"), { dont_inhibit = true }) -- quick settings: power, logout, restart, shutdown, wifi
hl.bind(K(mod .. " + W"),         hl.dsp.exec_cmd("ryogami wallpaper ui"))     -- ryogami wallpaper picker (full-screen browser: hero cards, colour filters, effects); the frame-blob menu stays on the bar logo
hl.bind(K(mod .. " + SHIFT + W"), hl.dsp.exec_cmd("ryogami wallpaper random")) -- random wallpaper, random transition
hl.bind(K(mod .. " + SHIFT + V"), hl.dsp.exec_cmd("ryoku-summon ryovm flock -n -o /tmp/ryovm.lock qs -c ryovm")) -- ryovm: summon to current workspace
hl.bind(K(mod .. " + V"),         hl.dsp.global("ryoku:clipboard")) -- clipboard (sidebar deep link)
hl.bind(K(mod .. " + Tab"),       hl.dsp.global("ryoku:overview")) -- workspace overview (expo: live previews, drag windows between workspaces, cycle)
hl.bind(K(mod .. " + ALT + Tab"), hl.dsp.global("ryoku:overview")) -- workspace overview, stepping desktops (Alt+Tab again inside cycles desktops)
hl.bind(K(mod .. " + M"),         hl.dsp.global("ryoku:visualizer"))        -- toggle the desktop audio visualiser
hl.bind(K(mod .. " + SHIFT + M"), hl.dsp.global("ryoku:visualizer-overlay")) -- raise the visualiser over windows (flip back to desktop)
hl.bind(K(mod .. " + ALT + M"),   hl.dsp.global("ryoku:visualizer-place"))   -- move the audio visualiser: drag the ring or orb into place
hl.bind(K(mod .. " + grave"),     hl.dsp.exec_cmd("ryoku-shell voice"))             -- voice typing: speech-to-text with a mic wave (tap again to stop)
hl.bind(K(mod .. " + comma"),     hl.dsp.exec_cmd("ryoku-shell hub open"))     -- ryoku settings
hl.bind(K(mod .. " + SHIFT + D"), hl.dsp.exec_cmd("ryoku-shell hub open widgets")) -- Stage Editor: edit the desktop on the focused screen
hl.bind(K(mod .. " + SHIFT + S"), hl.dsp.exec_cmd("ryoku-cmd-ryoshot")) -- screenshot: capture, annotate and beautify
hl.bind(K(mod .. " + SHIFT + C"), hl.dsp.exec_cmd("hyprpicker -a"))                 -- pick a color

-- Move/resize with the mouse
hl.bind(K(mod .. " + mouse:272"), hl.dsp.window.drag(),   { mouse = true })
hl.bind(K(mod .. " + mouse:273"), hl.dsp.window.resize(), { mouse = true })

-- Workspaces. Super+N focuses the Nth workspace OF THE CURRENT DESKTOP (a
-- desktop is a block of 10 workspace ids; see scripts/ryoku-workspace and the
-- overview). On desktop 2, Super+3 -> ws13, never ws3, so each desktop keeps its
-- own 1..10 and windows never jump desktops. The helper also pulls the target to
-- the monitor under the cursor, so the keys drive whichever screen the mouse is
-- on. Super+Alt+N sends the active window to that slot, staying on this desktop.
local ws_helper = (os.getenv("HOME") or "") .. "/.config/hypr/scripts/ryoku-workspace"

hl.bind(K(mod .. " + H"),          hl.dsp.exec_cmd(ws_helper .. " hide"))          -- hide the focused window in the scratchpad (press again on it to bring it back)
-- Through the helper, not toggle_special directly: hiding the scratchpad has to
-- hand keyboard focus back to a visible window, or the next bar panel to close
-- refocuses the hidden one and pops the scratchpad open with it.
hl.bind(K(mod .. " + ALT + H"),    hl.dsp.exec_cmd(ws_helper .. " scratch"))       -- show or hide the scratchpad (special workspace)
hl.bind(K(mod .. " + mouse_up"),   hl.dsp.focus({ workspace = "r-1" }))            -- previous workspace
hl.bind(K(mod .. " + mouse_down"), hl.dsp.focus({ workspace = "r+1" }))            -- next workspace
hl.bind(K(mod .. " + Prior"),         hl.dsp.focus({ workspace = "r-1" }))            -- Previous workspace
hl.bind(K(mod .. " + Next"),          hl.dsp.focus({ workspace = "r+1" }))            -- Next workspace
hl.bind(K(mod .. " + SHIFT + Prior"), hl.dsp.window.move({ workspace = "r-1" }))      -- Send window to previous
hl.bind(K(mod .. " + SHIFT + Next"),  hl.dsp.window.move({ workspace = "r+1" }))      -- Send window to next
-- The number pad drives the same per-desktop workspaces. Each digit is bound
-- twice: on KP_<n> (the keysym with NumLock on) and on the navigation keysym
-- the same key sends with NumLock off, so the shortcut fires either way.
local kp_off = { "KP_End", "KP_Down", "KP_Next", "KP_Left", "KP_Begin", "KP_Right", "KP_Home", "KP_Up", "KP_Prior", "KP_Insert" }
for i = 1, 10 do
    local key = i % 10 -- 10 maps to the 0 key
    hl.bind(K(mod .. " + " .. key),             hl.dsp.exec_cmd(ws_helper .. " focus " .. i))       -- focus workspace 1-10 on this desktop
    hl.bind(K(mod .. " + ALT + " .. key),       hl.dsp.exec_cmd(ws_helper .. " move " .. i))        -- move window to workspace 1-10 on this desktop
    hl.bind(K(mod .. " + SHIFT + " .. key),     hl.dsp.exec_cmd(ws_helper .. " movesilent " .. i))  -- move window silently to workspace 1-10 on this desktop
    hl.bind(K(mod .. " + KP_" .. key),          hl.dsp.exec_cmd(ws_helper .. " focus " .. i))       -- focus workspace 1-10, number pad
    hl.bind(K(mod .. " + ALT + KP_" .. key),    hl.dsp.exec_cmd(ws_helper .. " move " .. i))        -- move window to workspace 1-10, number pad
    hl.bind(K(mod .. " + SHIFT + KP_" .. key),  hl.dsp.exec_cmd(ws_helper .. " movesilent " .. i))  -- move window quietly to workspace 1-10, number pad
    hl.bind(K(mod .. " + " .. kp_off[i]),         hl.dsp.exec_cmd(ws_helper .. " focus " .. i))       -- focus workspace 1-10, number pad NumLock off
    hl.bind(K(mod .. " + ALT + " .. kp_off[i]),   hl.dsp.exec_cmd(ws_helper .. " move " .. i))        -- move window to workspace 1-10, number pad NumLock off
    hl.bind(K(mod .. " + SHIFT + " .. kp_off[i]), hl.dsp.exec_cmd(ws_helper .. " movesilent " .. i))  -- move window quietly to workspace 1-10, number pad NumLock off
end

-- Media and volume keys. ryoku-volume honours the volume panel's BOOST toggle
-- (Config.qsbar.audioBoost in shell.json): off = safe 100% cap, on = 150%.
hl.bind(K("XF86AudioRaiseVolume"), hl.dsp.exec_cmd("ryoku-volume up"),   { locked = true, repeating = true }) -- raise the volume
hl.bind(K("XF86AudioLowerVolume"), hl.dsp.exec_cmd("ryoku-volume down"), { locked = true, repeating = true }) -- lower the volume
hl.bind(K("XF86AudioMute"),        hl.dsp.exec_cmd("wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle"),     { locked = true })
hl.bind(K("XF86AudioMicMute"),     hl.dsp.exec_cmd("wpctl set-mute @DEFAULT_AUDIO_SOURCE@ toggle"),   { locked = true })
hl.bind(K("XF86AudioPlay"),        hl.dsp.exec_cmd("playerctl play-pause"),                           { locked = true })
hl.bind(K("XF86AudioNext"),        hl.dsp.exec_cmd("playerctl next"),                                 { locked = true })
hl.bind(K("XF86AudioPrev"),        hl.dsp.exec_cmd("playerctl previous"),                             { locked = true })
hl.bind(K(mod .. " + SHIFT + A"), hl.dsp.exec_cmd("ryoku-restart-audio")) -- recover audio when sound stops coming back

-- Brightness keys (laptop backlight + external DDC monitors)
hl.bind(K("XF86MonBrightnessUp"),   hl.dsp.exec_cmd("ryoku-cmd-brightness +5"), { locked = true, repeating = true }) -- raise screen brightness
hl.bind(K("XF86MonBrightnessDown"), hl.dsp.exec_cmd("ryoku-cmd-brightness -5"), { locked = true, repeating = true }) -- lower screen brightness

-- Touchpad lock (the FN touchpad key), through the window-manager seam
hl.bind(K("XF86TouchpadToggle"), hl.dsp.exec_cmd("ryoku wm act input.touchpad toggle"), { locked = true }) -- toggle the touchpad
hl.bind(K("XF86TouchpadOn"),     hl.dsp.exec_cmd("ryoku wm act input.touchpad on"),     { locked = true }) -- enable the touchpad
hl.bind(K("XF86TouchpadOff"),    hl.dsp.exec_cmd("ryoku wm act input.touchpad off"),    { locked = true }) -- disable the touchpad

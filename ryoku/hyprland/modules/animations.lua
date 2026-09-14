-- Animation preset loader. The active preset name (plain text in
-- ~/.config/ryoku/anim-preset, written by the Hub; default "ryoku") selects one
-- self-contained set under modules/animations/. Eye-candy presets ported from
-- dusklinux/dusky. Hub per-leaf tweaks still override on top via settings.lua.
hl.config({ animations = { enabled = true } })

local function cfg(sub)
    local base = os.getenv("XDG_CONFIG_HOME")
    if not base or base == "" then
        base = (os.getenv("HOME") or "") .. "/.config"
    end
    return base .. sub
end

local function readName()
    local f = io.open(cfg("/ryoku/anim-preset"), "r")
    if not f then
        return "ryoku"
    end
    local n = (f:read("l") or ""):gsub("%s+", "")
    f:close()
    if n == "" or not n:match("^[%w_]+$") then
        return "ryoku"
    end
    return n
end

-- probe the file so a stale name never trips Hyprland's config-error overlay
local function shipped(name)
    local f = io.open(cfg("/hypr/modules/animations/" .. name .. ".lua"), "r")
    if f then
        f:close()
        return true
    end
    return false
end

local name = readName()
if not shipped(name) then
    name = "ryoku"
end

-- Ryoku's base animation curves. The Hub's generated settings.lua (loaded after
-- this, in hyprland.lua) can reference any of these for its per-leaf and window
-- motion, on ANY preset -- but they used to be defined only by the `ryoku`
-- preset, so a window-style tweak or a curve pick on any other preset hit an
-- undefined bezier and threw Hyprland's config-error overlay ("no bezier
-- ryokuBloom / ryokuSettle"). Define them here, before the preset loads, so
-- those references always resolve whatever preset is active. The preset (ryoku
-- redefines these to the same points; others may set their own) or a user's
-- custom curve in settings.lua both load later, so last still wins.
hl.curve("easeOutQuint", { type = "bezier", points = { { 0.23, 1 }, { 0.32, 1 } } })
hl.curve("quick", { type = "bezier", points = { { 0.15, 0 }, { 0.1, 1 } } })
-- Saved Minimal overrides can outlive the preset that supplied their curve.
hl.curve("snap", { type = "bezier", points = { { 0.05, 0.9 }, { 0.1, 1.05 } } })
hl.curve("almostLinear", { type = "bezier", points = { { 0.5, 0.5 }, { 0.75, 1 } } })
hl.curve("ryokuBloom", { type = "bezier", points = { { 0.16, 1.12 }, { 0.24, 1 } } })
hl.curve("ryokuSettle", { type = "bezier", points = { { 0.18, 0.86 }, { 0.24, 1 } } })
hl.curve("ryokuWobble", { type = "bezier", points = { { 0.34, 1.56 }, { 0.64, 1 } } })

require("modules.animations." .. name)

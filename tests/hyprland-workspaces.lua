-- expects the exact dispatches the retired bash helper sent through hyprctl
local source = "ryoku/hyprland/modules/workspaces.lua"

local state, sent
local function describe(kind)
    return function(spec) return { kind = kind, spec = spec } end
end
local hl = {
    dsp = {
        focus = describe("focus"),
        window = { move = describe("move") },
        workspace = { toggle_special = describe("toggle_special") },
    },
    get_active_workspace = function() return state.active end,
    get_active_special_workspace = function() return state.special end,
    get_active_window = function() return state.window end,
    get_workspace_windows = function(id) return state.windows[id] or {} end,
    dispatch = function(d)
        sent[#sent + 1] = d
        if d.kind == "toggle_special" then
            state.special = (state.special == nil) and { name = "special:" .. d.spec } or nil
        elseif d.kind == "move" and d.spec.silent and state.drags then
            -- Hyprland shows the scratchpad a silent move lands in.
            state.special = { name = d.spec.workspace }
        end
    end,
}
local environment = setmetatable({ hl = hl }, { __index = _G })
local grid = assert(loadfile(source, "t", environment))()

local function render(v)
    if type(v) ~= "table" then
        return type(v) == "string" and string.format("%q", v) or tostring(v)
    end
    local keys = {}
    for k in pairs(v) do keys[#keys + 1] = k end
    table.sort(keys)
    local parts = {}
    for _, k in ipairs(keys) do parts[#parts + 1] = k .. "=" .. render(v[k]) end
    return "{" .. table.concat(parts, ",") .. "}"
end

local function case(name, setup, run, want)
    state = { active = nil, special = nil, window = nil, windows = {} }
    for k, v in pairs(setup) do state[k] = v end
    sent = {}
    run()
    local got = {}
    for _, d in ipairs(sent) do got[#got + 1] = d.kind .. render(d.spec) end
    local g, w = table.concat(got, " "), table.concat(want, " ")
    assert(g == w, string.format("%s:\n got  %s\n want %s", name, g, w))
end

local ws1, ws13 = { id = 1 }, { id = 13 }
local win = { address = "0xa", workspace = { id = 1, special = false } }

case("desktop 1 focus slot 3", { active = ws1 }, function() grid.focus(3) end,
    { "focus{workspace=3}" })
case("desktop 2 focus slot 3", { active = { id = 11 } }, function() grid.focus(3) end,
    { "focus{workspace=13}" })
case("desktop 2 slot 10 stays on desktop 2", { active = { id = 20 } }, function() grid.focus(10) end,
    { "focus{workspace=20}" })
case("special or unresolved workspace is desktop 0", {}, function() grid.focus(3) end,
    { "focus{workspace=3}" })
case("desktop 2 move slot 3", { active = ws13 }, function() grid.move(3) end,
    { "move{workspace=13}" })
case("desktop 2 movesilent slot 3", { active = { id = 11 }, window = win }, function() grid.movesilent(3) end,
    { 'move{follow=false,window="address:0xa",workspace="13"}' })
case("movesilent with no window does nothing", { active = ws1 }, function() grid.movesilent(3) end, {})

case("hide stashes, closes the scratchpad it dragged open, hands focus back",
    { active = ws13, window = win, drags = true },
    function() grid.hide() end,
    { 'move{silent=true,workspace="special:scratch"}', 'toggle_special"scratch"', "focus{workspace=13}" })
case("hide with the scratchpad left closed only refocuses", { active = ws13, window = win },
    function() grid.hide() end,
    { 'move{silent=true,workspace="special:scratch"}', "focus{workspace=13}" })
case("hide on a special workspace unhides with focus", {
    active = ws13, special = { name = "special:scratch" },
    window = { address = "0xb", workspace = { id = -98, special = true } },
}, function() grid.hide() end, { "move{workspace=13}" })
case("hide with no normal workspace does nothing", { window = win }, function() grid.hide() end, {})
case("hide with no window does nothing", { active = ws13 }, function() grid.hide() end, {})

case("scratch opens", { active = ws13 }, function() grid.scratch() end, { 'toggle_special"scratch"' })
case("scratch closes and refocuses the most recent window", {
    active = ws13, special = { name = "special:scratch" },
    windows = { [13] = {
        { address = "0x1", focus_history_id = 3 },
        { address = "0x2", focus_history_id = 1 },
        { address = "0x3", focus_history_id = 2 },
    } },
}, function() grid.scratch() end, { 'toggle_special"scratch"', 'focus{window="address:0x2"}' })
case("scratch closes onto an empty workspace without refocus", {
    active = ws13, special = { name = "special:scratch" },
}, function() grid.scratch() end, { 'toggle_special"scratch"' })

state = { active = { id = 23 } }
assert(grid.desktop() == 2, "ws23 is desktop 2")
state = { active = { id = -98 } }
assert(grid.desktop() == 0, "a special workspace is desktop 0")

print("hyprland-workspaces: OK")

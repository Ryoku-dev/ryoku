local held, clock, commands = {}, nil, 0
local hl = {
    is_key_down = function(key) return held[key] == true end,
    exec_cmd = function() commands = commands + 1 end,
    timer = function(callback)
        clock = { enabled = true }
        function clock:set_enabled(enabled) self.enabled = enabled end
        function clock:tick() if self.enabled then callback() end end
        return clock
    end,
}
local environment = setmetatable({ hl = hl }, { __index = _G })
local voice = assert(loadfile("ryoku/hyprland/modules/voice.lua", "t", environment))()

assert(not clock.enabled, "voice must not poll while idle")
for _, modifier in ipairs({ 37, 50, 62, 64, 105, 108, 133, 134 }) do
    held[modifier] = true
    local before = commands
    voice()
    voice()
    for _ = 1, 10 do clock:tick() end
    assert(commands == before, "voice ran while modifier " .. modifier .. " was held")
    held[modifier] = nil
    clock:tick()
    assert(commands == before + 1, "voice did not run once after modifier release")
    assert(not clock.enabled, "voice kept polling after dispatch")
    clock:tick()
    assert(commands == before + 1, "one shortcut dispatched voice twice")
end

held[133], held[105] = true, true
voice()
held[133] = nil
local before = commands
clock:tick()
assert(commands == before, "voice ran before the last modifier was released")
held[105] = nil
clock:tick()
assert(commands == before + 1, "voice did not run after the last modifier release")

print("hyprland-voice: OK")

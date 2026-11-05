-- The lid module must sync the panel on every event that can leave a panel on
-- under a shut lid. A resume or re-dock with the lid closed sends no switch
-- edge, and an unchanged layout causes no config reload, so the hotplug itself
-- has to run the sync.
local source = "ryoku/hyprland/modules/lid.lua"
local sync = "ryoku-clamshell lid sync"

local handlers = {}
local environment = {
    hl = {
        on = function(event, handler) handlers[event] = handler end,
        bind = function() end,
        exec_cmd = function(command) return command end,
        dsp = { exec_cmd = function(command) return command end },
    },
}
setmetatable(environment, { __index = _G })
assert(loadfile(source, "t", environment))()

for _, event in ipairs({ "hyprland.start", "config.reloaded", "monitor.added" }) do
    local handler = handlers[event]
    assert(handler, event .. " does not run a lid sync")
    local ran = nil
    environment.hl.exec_cmd = function(command) ran = command end
    handler()
    assert(ran and ran:find(sync, 1, true),
        string.format("%s ran %q, want a %q", event, tostring(ran), sync))
end

print("hyprland-lid-hooks: OK")

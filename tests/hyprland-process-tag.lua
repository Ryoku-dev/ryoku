-- Process tagging (modules/process_tag.lua) and the taggers window_rules.lua
-- registers, against a fake /proc and a fake hl: no running compositor.
local files, comm_reads = {}, 0

local function proc(pid, comm, ppid, start, cmdline)
    local dir = "/proc/" .. pid .. "/"
    files[dir .. "comm"] = comm .. "\n"
    files[dir .. "status"] = "Name:\t" .. comm .. "\nPPid:\t" .. ppid .. "\n"
    local fields = {}
    for _ = 3, 21 do fields[#fields + 1] = "0" end
    fields[#fields + 1] = tostring(start)
    files[dir .. "stat"] = pid .. " (" .. comm .. ") " .. table.concat(fields, " ")
    files[dir .. "cmdline"] = ((cmdline or comm):gsub(" ", "\0")) .. "\0"
end

local function gone(pid)
    for _, name in ipairs({ "comm", "status", "stat", "cmdline" }) do
        files["/proc/" .. pid .. "/" .. name] = nil
    end
end

local handlers, dispatched, rules

local function load(open_windows)
    handlers, dispatched, rules = {}, {}, {}
    local environment = {
        hl = {
            on = function(event, handler)
                handlers[event] = handlers[event] or {}
                table.insert(handlers[event], handler)
            end,
            window_rule = function(rule) rules[rule.name] = rule end,
            dispatch = function(action) dispatched[#dispatched + 1] = action end,
            dsp = { window = {
                tag = function(args) return args end,
                resize = function(args) return args end,
                move = function(args) return args end,
            } },
            get_windows = function() return open_windows or {} end,
            get_config = function() return {} end,
        },
        io = {
            open = function(path)
                local body = files[path]
                if not body then return nil end
                if path:match("/comm$") then comm_reads = comm_reads + 1 end
                return { read = function() return body end, close = function() end }
            end,
        },
    }
    setmetatable(environment, { __index = _G })
    local process_tag = assert(loadfile("ryoku/hyprland/modules/process_tag.lua", "t", environment))()
    environment.require = function(name)
        assert(name == "modules.process_tag", "unexpected require " .. name)
        return process_tag
    end
    assert(loadfile("ryoku/hyprland/modules/window_rules.lua", "t", environment))()
end

local function tags_for(address, from)
    local out = {}
    for i = from or 1, #dispatched do
        if dispatched[i].window == "address:" .. address then out[#out + 1] = dispatched[i].tag end
    end
    table.sort(out)
    return table.concat(out, ",")
end

local function open(pid, address)
    local before = #dispatched
    for _, handler in ipairs(handlers["window.open"]) do
        handler({ pid = pid, address = address, tags = {}, title = "" })
    end
    return tags_for(address, before + 1)
end

local function expect(got, want, what)
    assert(got == want, string.format("%s: got %q, want %q", what, got, want))
end

-- systemd -> steam -> reaper -> sh -> pressure-vessel -> bwrap -> python3
-- (proton) -> wine -> the game; the client's own helper and a terminal the
-- game started hang off the same tree.
proc(1, "systemd", 0, 1)
proc(100, "steam", 1, 10)
proc(101, "steamwebhelper", 100, 11)
proc(200, "reaper", 100, 20, "/steam/ubuntu12_32/reaper SteamLaunch AppId=1 -- game")
proc(201, "sh", 200, 21)
proc(202, "pressure-vessel", 201, 22)
proc(203, "bwrap", 202, 23)
proc(204, "python3", 203, 24)
proc(205, "wine64-preloader", 204, 25)
proc(206, "Game.exe", 205, 26)
proc(210, "kitty", 200, 30)
proc(300, "zenity", 1, 40)
proc(400, "reaper", 1, 50, "/usr/bin/reaper --not-steam")
proc(401, "app", 400, 51)

load()
local launched = rules["steam-native-launched"]
assert(launched and launched.match.tag == "steam-game", "no rule carries the steam-game tag")
for _, prop in ipairs({ "no_blur", "no_shadow", "opaque", "idle_inhibit", "immediate" }) do
    assert(launched[prop] == rules["steam-native"][prop], "steam-game rule differs on " .. prop)
end

expect(open(206, "game"), "+steam-game", "a game under the reaper, past every wrapper")
expect(open(101, "helper"), "", "steamwebhelper")
expect(open(210, "kitty"), "", "a terminal started from a game")
expect(open(300, "plain"), "", "an app outside Steam")
expect(open(401, "other-reaper"), "", "a process named reaper without SteamLaunch")

-- A second window from the same process reuses the answer: no walk.
comm_reads = 0
expect(open(206, "game-2"), "+steam-game", "a second window from the game")
assert(comm_reads == 0, "the second window walked /proc again")

-- A recycled pid (new start time) is tested afresh.
proc(206, "zenity", 1, 99)
expect(open(206, "reused"), "", "a recycled pid")

-- A read that fails mid-walk settles nothing, so the next window retries.
proc(220, "Game2.exe", 205, 60)
gone(203)
expect(open(220, "race"), "", "a walk through an exited process")
proc(203, "bwrap", 202, 23)
expect(open(220, "race-2"), "+steam-game", "the retry once the chain reads again")

-- A config reload tags the windows already open.
load({ { pid = 220, address = "open-at-reload", tags = {} } })
expect(tags_for("open-at-reload"), "+steam-game", "the sweep over open windows")

print("hyprland-process-tag: OK")

-- Process tagging (modules/process_tag.lua) and the taggers window_rules.lua
-- registers, against a fake /proc and a fake hl: no running compositor.
local files = {}

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

local handlers, dispatched, rules, windows, timers, rule_order

local function load(open_windows)
    handlers, dispatched, rules, windows, timers, rule_order = {}, {}, {}, open_windows or {}, {}, {}
    local environment = {
        hl = {
            on = function(event, handler)
                handlers[event] = handlers[event] or {}
                table.insert(handlers[event], handler)
            end,
            window_rule = function(rule)
                rules[rule.name] = rule
                rule_order[rule.name] = #rule_order + 1
                rule_order[#rule_order + 1] = rule.name
            end,
            dispatch = function(action) dispatched[#dispatched + 1] = action end,
            dsp = { window = {
                tag = function(args) return args end,
                resize = function(args) return args end,
                move = function(args) return args end,
            } },
            get_windows = function() return windows end,
            timer = function(fn, opts)
                timers[#timers + 1] = { fn = fn, timeout = opts.timeout, type = opts.type }
                return {}
            end,
            get_config = function() return {} end,
        },
        io = {
            open = function(path)
                local body = files[path]
                if not body then return nil end
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
    local win = { pid = pid, address = address, tags = {}, title = "" }
    windows[#windows + 1] = win
    for _, handler in ipairs(handlers["window.open"]) do handler(win) end
    return tags_for(address, before + 1)
end

-- Fire the timers pending now; a timer they schedule waits for the next call.
local function run_timers()
    local due = timers
    timers = {}
    for _, t in ipairs(due) do
        assert(t.type == "oneshot", "retry timers must be oneshot")
        t.fn()
    end
    return #due
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

expect(open(206, "game-2"), "+steam-game", "a second window from the game")

-- No answer outlives its window: once the chain above a process changes (its
-- parents exit and it is reparented to init), its next window is walked
-- afresh rather than handed the first window's answer.
proc(250, "Game8.exe", 205, 66)
expect(open(250, "before-reparent"), "+steam-game", "a game under the reaper")
proc(250, "Game8.exe", 1, 66)
expect(open(250, "after-reparent"), "", "the same process after its parents exited")

-- A recycled pid (new start time) is tested afresh.
proc(206, "zenity", 1, 99)
expect(open(206, "reused"), "", "a recycled pid")

-- A read that fails mid-walk settles nothing, so the next window retries.
proc(220, "Game2.exe", 205, 60)
gone(203)
expect(open(220, "race"), "", "a walk through an exited process")
assert(#timers == 1 and timers[1].timeout == 250, "an unsettled window must be retried soon")
proc(203, "bwrap", 202, 23)
run_timers()
expect(tags_for("race"), "+steam-game", "the timed retry of the unsettled window")
assert(#timers == 0, "a settled retry must not schedule another")
expect(open(220, "race-2"), "+steam-game", "a later window from the same process")

-- The retries are bounded: three, then the window is left alone.
proc(225, "Game5.exe", 205, 63)
gone(205)
expect(open(225, "stuck"), "", "a walk that never settles")
local fired = 0
while #timers > 0 do fired = fired + run_timers() end
assert(fired == 3, "retries must stop after three, fired " .. fired)
proc(205, "wine64-preloader", 204, 25)

-- A window closed before its retry is not touched.
gone(203)
proc(226, "Game6.exe", 205, 64)
expect(open(226, "closed"), "", "an unsettled window")
windows[#windows] = nil
proc(203, "bwrap", 202, 23)
run_timers()
expect(tags_for("closed"), "", "a retry after the window closed")

-- An unreadable window process (its own comm) is retried the same way.
proc(227, "Game7.exe", 205, 65)
local comm227 = files["/proc/227/comm"]
files["/proc/227/comm"] = nil
expect(open(227, "own-comm"), "", "a window whose own process cannot be read")
assert(#timers == 1, "an unreadable window process must be retried")
files["/proc/227/comm"] = comm227
run_timers()
expect(tags_for("own-comm"), "+steam-game", "the retry once its comm reads")

-- An unreadable reaper command line settles nothing either.
proc(230, "Game3.exe", 205, 61)
local reaper_cmdline = files["/proc/200/cmdline"]
files["/proc/200/cmdline"] = nil
expect(open(230, "no-cmdline"), "", "a reaper whose command line cannot be read")
files["/proc/200/cmdline"] = reaper_cmdline
expect(open(230, "no-cmdline-2"), "+steam-game", "the retry once the command line reads")

-- A process that execs keeps its pid and start time; the new program is
-- tested as itself, not handed the old program's answer.
proc(240, "launcher-stub", 1, 62)
expect(open(240, "pre-exec"), "", "the process before its exec")
proc(240, "Game4.exe", 205, 62)
expect(open(240, "post-exec"), "+steam-game", "the same process after its exec")

-- Unreal Engine, always opaque: the editor by name, a pop-up helper by its
-- parent, a build from an engine tree by its path.
proc(500, "UnrealEditor", 1, 70, "/opt/ue/Engine/Binaries/Linux/UnrealEditor")
proc(501, "UnrealEditor", 500, 71)
proc(502, "MyGameEditor", 1, 72, "/home/u/ue/Engine/Binaries/Linux/MyGameEditor /p/My.uproject")
proc(503, "CrashReportClie", 1, 73)
load()
local ue_rule = rules["unreal-engine"]
assert(ue_rule and ue_rule.match.tag == "unreal-engine", "no rule carries the unreal-engine tag")
for _, prop in ipairs({ "no_blur", "no_shadow", "opaque", "idle_inhibit" }) do
    assert(ue_rule[prop] == rules["steam-native"][prop], "unreal-engine rule differs on " .. prop)
end
assert(ue_rule.immediate == false, "the editor UI must not tear, even when Steam launched it")
assert(rules["unreal-game"] and rules["unreal-game"].immediate == true, "UE game windows must tear")
assert(rules["unreal-play-in-editor"] and rules["unreal-play-in-editor"].immediate == true
    and rules["unreal-play-in-editor"].match.tag == "unreal-engine", "the PIE window must tear")
expect(open(500, "ue"), "+unreal-engine", "the editor")
expect(open(501, "ue-popup"), "+unreal-engine", "an editor child process")
expect(open(502, "ue-path"), "+unreal-engine", "a binary under Engine/Binaries")
expect(open(503, "ue-crash"), "+unreal-engine", "the crash reporter (comm cut to 15)")
proc(510, "dotnet", 500, 74)
proc(511, "helper-a", 510, 75)
proc(512, "helper-b", 511, 76)
proc(513, "helper-c", 512, 77)
proc(514, "helper-d", 513, 78)
proc(515, "ue-subwindow", 514, 79)
expect(open(515, "ue-deep"), "+unreal-engine", "a helper six processes below the editor")
proc(520, "UnrealEditor", 1, 90, "/opt/ue/Engine/Binaries/Linux/UnrealEditor /p/Game.uproject -game")
expect(open(520, "ue-game"), "+unreal-engine,+unreal-game", "the editor binary run with -game")
proc(521, "Game.exe", 1, 91, "/games/Game.exe -game")
expect(open(521, "not-ue"), "", "-game on a binary that is not Unreal")
proc(522, "UnrealEditor", 1, 92, "/opt/ue/Engine/Binaries/Linux/UnrealEditor /p/Game.uproject -gamepad")
expect(open(522, "ue-gamepad"), "+unreal-engine", "an option that only starts with -game")

-- Rule order: the editor's immediate = false follows the steam-game rule, and
-- the game window rules follow the editor's.
local at = rule_order
assert(at["steam-native-launched"] < at["unreal-engine"], "the editor rule must follow the steam-game rule")
assert(at["unreal-engine"] < at["unreal-game"] and at["unreal-engine"] < at["unreal-play-in-editor"],
    "the game window rules must follow the editor rule")

-- Browsers are tagged only; the opt-in rule comes from the provider.
proc(600, "firefox", 1, 80, "/usr/lib/firefox/firefox")
proc(601, "chrome", 1, 81, "/opt/google/chrome/chrome --app=https://example.org")
proc(602, "chromium", 1, 82, "/usr/lib/chromium/chromium")
proc(603, "chromedriver", 1, 83, "/usr/bin/chromedriver")
proc(604, "sh", 600, 84)
proc(605, "kitty", 604, 85)
assert(not rules["ryoku-opaque-browsers"], "browsers must not be pinned by the shipped rules")
expect(open(600, "ff"), "+browser", "Firefox")
expect(open(601, "pwa"), "+browser", "a Chrome app window")
expect(open(602, "chromium"), "+browser", "Chromium")
expect(open(603, "driver"), "", "chromedriver is not a browser")
expect(open(605, "kitty-from-ff"), "", "a terminal a browser started through a shell")

-- A config reload tags the windows already open.
load({ { pid = 220, address = "open-at-reload", tags = {} } })
expect(tags_for("open-at-reload"), "+steam-game", "the sweep over open windows")

print("hyprland-process-tag: OK")

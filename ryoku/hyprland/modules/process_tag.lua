-- Tag windows by the process that owns them. A window rule matches class and
-- title only, and plenty of windows map with an empty, generic or per-build
-- class; the owning process and its parents name them reliably. A tagger is a
-- test over the window's pid: a hit adds its tag, and an ordinary
-- `match = { tag = ... }` rule carries the props. Only /proc reads, never a
-- fork: this runs inside the compositor for every new window.
local M = {}

local taggers = {}
-- One test per process, not per window. The key carries the start time, so a
-- recycled pid is tested afresh instead of inheriting a dead process's verdict.
local verdicts = {}

local function read(path)
    local f = io.open(path, "r")
    if not f then return nil end
    local s = f:read("a")
    f:close()
    return s
end

function M.comm(pid)
    return ((read("/proc/" .. pid .. "/comm") or ""):gsub("\n$", ""))
end

function M.cmdline(pid)
    return read("/proc/" .. pid .. "/cmdline") or ""
end

function M.ppid(pid)
    local s = read("/proc/" .. pid .. "/status")
    return s and tonumber(s:match("\nPPid:%s*(%d+)"))
end

-- pid, its parent, its grandparent, ... up to init, bounded so a /proc race
-- can never loop.
function M.ancestors(pid)
    local p, n = pid, 0
    return function()
        if not p or p <= 1 or n >= 64 then return nil end
        local cur = p
        p, n = M.ppid(cur), n + 1
        return cur
    end
end

-- Field 22 of /proc/<pid>/stat. The fields are counted after the last ")",
-- since comm itself may hold spaces and parentheses.
local function start_time(pid)
    local rest = (read("/proc/" .. pid .. "/stat") or ""):match(".*%)%s+(.*)$")
    if not rest then return nil end
    local i = 2
    for field in rest:gmatch("%S+") do
        i = i + 1
        if i == 22 then return field end
    end
end

local function has_tag(win, tag)
    local tags = win.tags
    if type(tags) == "table" then
        for _, t in ipairs(tags) do
            if t == tag then return true end
        end
    elseif type(tags) == "string" then
        return tags:find(tag, 1, true) ~= nil
    end
    return false
end

local function check(win, list)
    if not win or not win.pid or win.pid <= 0 or not win.address then return end
    local started = start_time(win.pid)
    if not started then return end
    for _, t in ipairs(list) do
        local key = win.pid .. ":" .. started .. ":" .. t.tag
        local hit = verdicts[key]
        if hit == nil then
            local ok, res = pcall(t.test, win.pid)
            hit = ok and res == true
            verdicts[key] = hit
        end
        if hit and not has_tag(win, t.tag) then
            hl.dispatch(hl.dsp.window.tag({ tag = "+" .. t.tag, window = "address:" .. win.address }))
        end
    end
end

-- add(tag, test): test(pid) returns true for a process whose windows get tag.
-- Windows already open (a config reload) are swept once for the new tagger.
function M.add(tag, test)
    local t = { tag = tag, test = test }
    taggers[#taggers + 1] = t
    pcall(function()
        for _, win in ipairs(hl.get_windows() or {}) do check(win, { t }) end
    end)
end

hl.on("window.open", function(win) pcall(check, win, taggers) end)

return M

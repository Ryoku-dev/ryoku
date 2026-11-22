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
-- Entries are never matched to exits, so the table is emptied once it holds
-- VERDICT_LIMIT answers: a forgotten answer only costs one more walk.
local verdicts, verdict_count = {}, 0
local VERDICT_LIMIT = 256

local function read(path)
    local f = io.open(path, "r")
    if not f then return nil end
    local s = f:read("a")
    f:close()
    return s
end

-- nil when the process is gone.
function M.comm(pid)
    local s = read("/proc/" .. pid .. "/comm")
    return s and (s:gsub("\n$", ""))
end

function M.cmdline(pid)
    return read("/proc/" .. pid .. "/cmdline") or ""
end

function M.ppid(pid)
    local s = read("/proc/" .. pid .. "/status")
    return s and tonumber(s:match("\nPPid:%s*(%d+)"))
end

-- walk(pid, visit): visit(p, comm) for the process, its parent, and so on up
-- to init. visit returns true or false to settle the answer, or nil to climb
-- on; reaching init settles false. A failed read (a process in the chain
-- exited mid-walk) settles nothing: walk returns nil, the answer is not kept,
-- and the next window from that process is tested again.
function M.walk(pid, visit)
    local p = pid
    for _ = 1, 64 do
        local comm = M.comm(p)
        if not comm then return nil end
        local verdict = visit(p, comm)
        if verdict ~= nil then return verdict end
        p = M.ppid(p)
        if not p then return nil end
        if p <= 1 then return false end
    end
    return false
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
            if ok and res ~= nil then
                if verdict_count >= VERDICT_LIMIT then verdicts, verdict_count = {}, 0 end
                verdicts[key], verdict_count = hit, verdict_count + 1
            end
        end
        if hit and not has_tag(win, t.tag) then
            hl.dispatch(hl.dsp.window.tag({ tag = "+" .. t.tag, window = "address:" .. win.address }))
        end
    end
end

-- add(tag, test): test(pid) returns true for a process whose windows get tag,
-- false for one whose windows never do, nil when it cannot tell yet.
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

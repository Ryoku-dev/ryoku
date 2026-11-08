local M = {}

-- has to match the overview's perDesktop
M.PER = 10

-- the provider's window.hide uses the same name
M.SCRATCH = "scratch"

-- stays the normal workspace while a special one is open over it
local function active_id()
    local ws = hl.get_active_workspace()
    return ws and ws.id or 0
end

local function open_special()
    local ws = hl.get_active_special_workspace()
    return ws and ws.name or nil
end

function M.desktop()
    local a = active_id()
    if a < 1 then
        return 0
    end
    return (a - 1) // M.PER
end

function M.target(slot)
    return M.desktop() * M.PER + slot
end

-- focusing a workspace already moves to the monitor that owns it
function M.focus(slot)
    hl.dispatch(hl.dsp.focus({ workspace = M.target(slot) }))
end

function M.move(slot)
    hl.dispatch(hl.dsp.window.move({ workspace = M.target(slot) }))
end

function M.movesilent(slot)
    local win = hl.get_active_window()
    if not win then
        return
    end
    hl.dispatch(hl.dsp.window.move({
        workspace = tostring(M.target(slot)),
        window = "address:" .. win.address,
        follow = false,
    }))
end

function M.hide()
    local win = hl.get_active_window()
    local ws = win and win.workspace
    if not ws then
        return
    end
    local back = active_id()
    if back < 1 then
        return
    end
    if ws.special then
        -- not silent, following it restores focus
        hl.dispatch(hl.dsp.window.move({ workspace = back }))
        return
    end
    -- no float or resize, that would bring a tiled window back as a centred float
    hl.dispatch(hl.dsp.window.move({ workspace = "special:" .. M.SCRATCH, silent = true }))
    -- a silent move still drags focus into the scratchpad and shows it, so undo both
    if open_special() == "special:" .. M.SCRATCH then
        hl.dispatch(hl.dsp.workspace.toggle_special(M.SCRATCH))
    end
    hl.dispatch(hl.dsp.focus({ workspace = back }))
end

function M.scratch()
    local back = active_id()
    hl.dispatch(hl.dsp.workspace.toggle_special(M.SCRATCH))
    -- Hyprland keeps focus on the window it just hid, so the next panel to close would pop the scratchpad back
    -- it has to be a window, focusing an empty workspace makes Hyprland re-reveal the scratchpad
    if open_special() ~= nil or back < 1 then
        return
    end
    local recent = nil
    for _, w in ipairs(hl.get_workspace_windows(back) or {}) do
        if recent == nil or w.focus_history_id < recent.focus_history_id then
            recent = w
        end
    end
    if recent then
        hl.dispatch(hl.dsp.focus({ window = "address:" .. recent.address }))
    end
end

return M

-- The per-desktop workspace grid and the scratchpad, behind Super+N, Super+H
-- and Super+Alt+H (binds.lua) and the provider's window.hide on the focused
-- window, which runs hide() through `hyprctl eval`.
--
-- A desktop is a block of PER workspace ids: desktop d owns ids
-- d*PER+1 .. d*PER+PER. The current desktop is derived from the focused
-- monitor's active workspace, so Super+N always targets the Nth workspace of the
-- desktop you are on (on desktop 2, Super+3 -> ws13, never ws3) and a window sent
-- with Super+Alt+N stays on the current desktop. PER must match the overview's
-- perDesktop; the live desktop is computed, never stored, so it cannot drift.
--
-- The grid needs a fixed workspace model and hide/scratch need a special
-- workspace. Hyprland always has both, so nothing here asks the seam first.

local M = {}

M.PER = 10

-- The special workspace hide() stashes into and scratch() toggles. The
-- provider's window.hide for a named window uses the same name.
M.SCRATCH = "scratch"

-- The normal workspace on the focused monitor. It stays the normal one while a
-- special workspace is open over it, so it is what sits on screen either way.
local function active_id()
    local ws = hl.get_active_workspace()
    return ws and ws.id or 0
end

-- The special workspace open on the focused monitor, or nil.
local function open_special()
    local ws = hl.get_active_special_workspace()
    return ws and ws.name or nil
end

-- The current desktop index (0-based). A special or unresolved workspace counts
-- as desktop 0.
function M.desktop()
    local a = active_id()
    if a < 1 then
        return 0
    end
    return (a - 1) // M.PER
end

-- slot (1..PER) -> absolute workspace id on the current desktop.
function M.target(slot)
    return M.desktop() * M.PER + slot
end

-- Focusing a workspace already moves focus to the monitor that owns it, so a
-- workspace on a secondary monitor stays where it is.
function M.focus(slot)
    hl.dispatch(hl.dsp.focus({ workspace = M.target(slot) }))
end

-- Move the active window to the slot and follow it.
function M.move(slot)
    hl.dispatch(hl.dsp.window.move({ workspace = M.target(slot) }))
end

-- Move the active window to the slot and stay here.
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

-- Stash the focused window in the scratchpad, or pull it back out when it is
-- already on a special workspace.
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
        -- The normal workspace under the open overlay is where the window came
        -- from. Not silent: following it restores focus.
        hl.dispatch(hl.dsp.window.move({ workspace = back }))
        return
    end
    -- No float/resize/centre: rewriting geometry would return a tiled window as
    -- a centred float instead of to its slot.
    hl.dispatch(hl.dsp.window.move({ workspace = "special:" .. M.SCRATCH, silent = true }))
    -- A silent move still drags focus into the scratchpad, which shows it. Undo
    -- both, or the window stays on screen and typing goes somewhere invisible.
    if open_special() == "special:" .. M.SCRATCH then
        hl.dispatch(hl.dsp.workspace.toggle_special(M.SCRATCH))
    end
    hl.dispatch(hl.dsp.focus({ workspace = back }))
end

-- Show the scratchpad, or hide it and hand keyboard focus back to what is on
-- screen.
function M.scratch()
    local back = active_id()
    hl.dispatch(hl.dsp.workspace.toggle_special(M.SCRATCH))
    -- Hyprland leaves keyboard focus on the window it just hid. The next surface
    -- that takes and releases a keyboard grab (any bar panel or menu) hands focus
    -- back to it, and focusing a window on a special workspace shows that
    -- workspace: closing a panel would pop the scratchpad straight back open.
    --
    -- So hiding hands focus to a window that is actually on screen. It has to be
    -- a window, not the workspace: focusing an empty workspace leaves Hyprland
    -- with nothing to focus, so it keeps the hidden window and re-reveals the
    -- scratchpad on the spot. Nothing to hand focus to means leave it alone.
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

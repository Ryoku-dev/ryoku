-- keycodes still identify held modifiers after a layout or keymap change
local modifiers = { 37, 50, 62, 64, 105, 108, 133, 134 }

-- fast transcription can finish while Super is still held and turn text into workspace shortcuts
local pending
pending = hl.timer(function()
    for _, key in ipairs(modifiers) do
        if hl.is_key_down(key) then return end
    end
    pending:set_enabled(false)
    hl.exec_cmd("ryoku-shell voice")
end, { timeout = 10, type = "repeat" })
pending:set_enabled(false)

return function()
    pending:set_enabled(true)
end

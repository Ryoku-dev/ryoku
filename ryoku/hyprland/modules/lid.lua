-- ryoku-clamshell owns the shared lid-close policy. It sends non-docked closes
-- through the shell's secure suspend transaction; live docked mode hands the
-- covered panel to the external display. Opening clears only the panel's
-- disabled flag, preserving mode, scale and position. locked = fires while
-- qylock is active too, so display handoff remains correct across resume.
local function sync_physical_lid()
    hl.exec_cmd("command -v ryoku-clamshell >/dev/null 2>&1 && ryoku-clamshell lid sync")
end

hl.on("hyprland.start", sync_physical_lid)
hl.on("config.reloaded", sync_physical_lid)
hl.bind("switch:on:Lid Switch",  hl.dsp.exec_cmd("command -v ryoku-clamshell >/dev/null 2>&1 && ryoku-clamshell lid close"), { locked = true })
hl.bind("switch:off:Lid Switch", hl.dsp.exec_cmd("command -v ryoku-clamshell >/dev/null 2>&1 && ryoku-clamshell lid open"),  { locked = true })

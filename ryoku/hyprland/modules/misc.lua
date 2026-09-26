hl.config({
    misc = {
        -- keyboard focus follows an app's activation request, else a window mapping
        -- off the focused monitor comes up un-typeable (Discord, Vivaldi)
        focus_on_activate = true,
        disable_hyprland_logo = true,
        force_default_wallpaper = 0,
        -- Input is the compositor-native way out of DPMS. hypridle still runs
        -- the neutral output.power on action when its idle cycle resumes, but
        -- these keep a dead or late idle client from leaving the panel black.
        key_press_enables_dpms = true,
        mouse_move_enables_dpms = true,
        -- a locker that crashes while locked (GPU glitch on resume) otherwise
        -- wedges the session on a black screen that eats every key. with this,
        -- Hyprland accepts a fresh locker instead of stranding the session.
        allow_session_lock_restore = true,
        -- ryoku-monitor changes scale live during autoscale (login, hotplug,
        -- undock); suppress Hyprland's own "scale changed" popup so a rescale
        -- does not flash a raw toast over the shell's OSD.
        disable_scale_notification = true,
        -- Super+A snaps a window to a centred float and back. Without this a
        -- dispatcher resize jumps in one frame, so the app re-lays-out mid-jump
        -- and its UI flickers; animating it lets Hyprland texture-scale the
        -- window into the new size and hand the app one final-size configure,
        -- so it morphs smoothly instead.
        animate_manual_resizes = true,
    },
    xwayland = {
        force_zero_scaling = true, -- XWayland (Chromium/Electron) crisp on HiDPI
        -- when an XWayland app is scaled by the compositor, nearest-neighbour
        -- keeps text pixel-crisp instead of the default bilinear blur.
        use_nearest_neighbor = true,
    },
})

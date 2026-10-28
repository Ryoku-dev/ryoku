-- Game Mode's strip, shared by the live WM action and every config reload while
-- the request is on. Immediate tearing reaches only the sole fullscreen game and
-- VRR only fullscreen windows; render_unfocused keeps a game on another
-- workspace drawing, so its input and multiplayer never stall.
hl.config({
  general = { allow_tearing = true },
  decoration = {
    rounding = 0,
    active_opacity = 1.0,
    inactive_opacity = 1.0,
    blur = { enabled = false },
    shadow = { enabled = false },
  },
  animations = { enabled = false },
  misc = { vrr = 2 },
})
hl.window_rule({ name = "game-mode-immediate", match = { class = ".*" }, immediate = true })
hl.window_rule({ name = "game-mode-render-unfocused", match = { fullscreen = true }, render_unfocused = true })

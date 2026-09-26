-- follow_mouse = 1 puts keyboard focus under the cursor: the window the pointer
-- is over takes focus as it moves, which is what a pointer-driven desktop feels
-- like, and what niri's focus-follows-mouse mirrors.
hl.config({
    input = {
        follow_mouse = 1,
        sensitivity = 0,
        touchpad = {
            natural_scroll = false,
        },
    },
})

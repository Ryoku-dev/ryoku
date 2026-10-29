-- Ryoku Hyprland Color Palette

return {
    -- Border hex colors
    active = "{{colors.primary.default.hex}}",
    inactive = "{{colors.background.default.hex}}",
    
    border_active = "ff{{colors.primary.default.hex_stripped}}",
    border_active_gradient = "ff{{colors.primary.default.hex_stripped}} ff{{colors.tertiary.default.hex_stripped}} 45deg",
    border_inactive = "aa{{colors.outline_variant.default.hex_stripped}}",

    -- Surface & Shadow
    surface = "ff{{colors.surface.default.hex_stripped}}",
    surface_container = "ff{{colors.surface_container.default.hex_stripped}}",
    shadow = "66{{colors.scrim.default.hex_stripped}}",

    -- Accent Roles
    primary = "ff{{colors.primary.default.hex_stripped}}",
    primary_container = "ff{{colors.primary_container.default.hex_stripped}}",
    tertiary = "ff{{colors.tertiary.default.hex_stripped}}",

    -- Groups
    group_border_active = "ff{{colors.primary.default.hex_stripped}}",
    group_border_inactive = "aa{{colors.outline_variant.default.hex_stripped}}",
    group_bg_active = "ff{{colors.primary_container.default.hex_stripped}}",
    group_bg_inactive = "ff{{colors.surface_container_low.default.hex_stripped}}",
    group_text_active = "ff{{colors.on_primary_container.default.hex_stripped}}",
}

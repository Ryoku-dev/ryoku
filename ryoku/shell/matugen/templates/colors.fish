# Ryoku palette for fish and fzf. Rendered by the theme daemon; do not edit.
#
# Dropped straight into conf.d, which fish sources on its own, so nothing in the
# shipped config has to include it. Your ~/.config/fish/user.fish loads last and
# still wins.

# Syntax highlighting.
set -g fish_color_normal {{colors.on_surface.default.hex_stripped}}
set -g fish_color_command {{colors.primary.default.hex_stripped}}
set -g fish_color_keyword {{colors.tertiary.default.hex_stripped}}
set -g fish_color_quote {{colors.secondary.default.hex_stripped}}
set -g fish_color_redirection {{colors.on_surface_variant.default.hex_stripped}}
set -g fish_color_end {{colors.tertiary.default.hex_stripped}}
set -g fish_color_error {{colors.error.default.hex_stripped}}
set -g fish_color_param {{colors.on_surface.default.hex_stripped}}
set -g fish_color_option {{colors.on_surface_variant.default.hex_stripped}}
set -g fish_color_comment {{colors.on_surface_variant.default.hex_stripped}}
set -g fish_color_selection --background={{colors.primary_container.default.hex_stripped}}
set -g fish_color_operator {{colors.tertiary.default.hex_stripped}}
set -g fish_color_escape {{colors.secondary.default.hex_stripped}}
set -g fish_color_autosuggestion {{colors.on_surface_variant.default.hex_stripped}}
set -g fish_color_cancel {{colors.error.default.hex_stripped}}
set -g fish_color_search_match --background={{colors.primary_container.default.hex_stripped}}
set -g fish_color_valid_path --underline

# Completion pager.
set -g fish_pager_color_progress {{colors.on_surface_variant.default.hex_stripped}}
set -g fish_pager_color_prefix {{colors.primary.default.hex_stripped}}
set -g fish_pager_color_completion {{colors.on_surface.default.hex_stripped}}
set -g fish_pager_color_description {{colors.on_surface_variant.default.hex_stripped}}
set -g fish_pager_color_selected_background --background={{colors.primary_container.default.hex_stripped}}

# fzf takes the same palette, so Ctrl-R and Ctrl-T match the terminal they open
# in. Appended to whatever options are already set rather than replacing them.
set -gx FZF_DEFAULT_OPTS "$FZF_DEFAULT_OPTS \
--color=fg:{{colors.on_surface.default.hex}},bg:-1,hl:{{colors.primary.default.hex}} \
--color=fg+:{{colors.on_surface.default.hex}},bg+:{{colors.primary_container.default.hex}},hl+:{{colors.primary.default.hex}} \
--color=info:{{colors.secondary.default.hex}},prompt:{{colors.primary.default.hex}},pointer:{{colors.tertiary.default.hex}} \
--color=marker:{{colors.tertiary.default.hex}},spinner:{{colors.secondary.default.hex}},header:{{colors.on_surface_variant.default.hex}} \
--color=border:{{colors.outline_variant.default.hex}}"

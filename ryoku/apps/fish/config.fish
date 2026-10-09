# ~/.local/bin on PATH for every shell, not only interactive: tools dropped in
# there (claude, oh-my-posh) work, and the ryoku-fastfetch wrapper below
# resolves. fish_add_path is idempotent.
if test -d $HOME/.local/bin
  fish_add_path $HOME/.local/bin
end

# point `go install` and `cargo install` at ~/.local/bin, and keep an existing
# editor choice.
set -gx GOBIN $HOME/.local/bin
set -gx CARGO_INSTALL_ROOT $HOME/.local
set -q EDITOR; or set -gx EDITOR nvim
set -q VISUAL; or set -gx VISUAL nvim

if not set -q RYOKU_BASE_OS; and command -q ryoku-base-os
  set -gx RYOKU_BASE_OS (ryoku-base-os)
end

if status is-interactive
  set -g fish_greeting

  # legible fish syntax colours. fish applies a palette-tied default before
  # config.fish runs (in our case with a malformed flag that made typed input
  # the same colour as the background). pin a fixed scheme unconditionally so
  # it always wins, regardless of the palette.
  set -g fish_color_normal         F1F3E4
  set -g fish_color_command        e2342a
  set -g fish_color_keyword        e83b30
  set -g fish_color_param          F1F3E4
  set -g fish_color_option         CCD0CF
  set -g fish_color_quote          A3C293
  set -g fish_color_redirection    8AA9CC
  set -g fish_color_end            e83b30
  set -g fish_color_error          FF6B6B
  set -g fish_color_comment        949699
  set -g fish_color_operator       93D4E0
  set -g fish_color_escape         93D4E0
  set -g fish_color_autosuggestion 949699

  # branded system readout on terminal open.
  if command -v ryoku-fastfetch >/dev/null 2>&1
    ryoku-fastfetch
  end

  # prompt.
  if command -v starship >/dev/null 2>&1
    starship init fish | source
  end

  # directory jumper: hook `cd` into zoxide so plain `cd` learns and jumps to
  # frecent dirs (`cdi` for an interactive pick).
  if command -v zoxide >/dev/null 2>&1
    zoxide init fish --cmd cd | source
  end

  # runtime version manager: shims + per-project tool versions.
  if command -v mise >/dev/null 2>&1
    mise activate fish | source
  end

  # fzf walks the tree via fd when present.
  if command -v fd >/dev/null 2>&1
    set -gx FZF_DEFAULT_COMMAND 'fd --hidden --follow --exclude .git'
    set -gx FZF_CTRL_T_COMMAND $FZF_DEFAULT_COMMAND
    set -gx FZF_ALT_C_COMMAND 'fd --type d --hidden --follow --exclude .git'
  end

  # fzf keys: Ctrl-R history, Ctrl-T files, Alt-C cd.
  if command -v fzf >/dev/null 2>&1
    fzf --fish | source
  end

  # eza listings.
  if command -v eza >/dev/null 2>&1
    alias ls 'eza -lh --group-directories-first --icons=auto'
    alias lsa 'ls -a'
    alias lt 'eza --tree --level=2 --long --icons --git'
    alias lta 'lt -a'
  end
end

# user overrides: ~/.config/fish/user.fish is never shipped, never touched on
# update, and loads last so your stuff wins.
test -f $__fish_config_dir/user.fish && source $__fish_config_dir/user.fish

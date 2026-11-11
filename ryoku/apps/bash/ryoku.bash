# shellcheck shell=bash disable=SC1090,SC1091,SC1094
[[ ${__RYOKU_BASH_LOADED:-0} == 1 ]] && return
__RYOKU_BASH_LOADED=1

_ryoku_env=${XDG_CONFIG_HOME:-$HOME/.config}/ryoku-terminal/env.sh
[[ -r $_ryoku_env ]] || _ryoku_env=$(cd "$(dirname "${BASH_SOURCE[0]}")/../terminal-shell" 2>/dev/null && pwd)/env.sh
[[ -r $_ryoku_env ]] && . "$_ryoku_env"
unset _ryoku_env

[[ $- == *i* ]] || return

_ryoku_desktop_env=${XDG_CONFIG_HOME:-$HOME/.config}/ryoku-terminal/desktop-display.sh
[[ -r $_ryoku_desktop_env ]] || _ryoku_desktop_env=$(cd "$(dirname "${BASH_SOURCE[0]}")/../terminal-shell" 2>/dev/null && pwd)/desktop-display.sh
[[ -r $_ryoku_desktop_env ]] && . "$_ryoku_desktop_env"
unset _ryoku_desktop_env

command -v ryoku-fastfetch >/dev/null 2>&1 && ryoku-fastfetch
command -v zoxide >/dev/null 2>&1 && eval "$(zoxide init bash --cmd cd)"
command -v mise >/dev/null 2>&1 && eval "$(mise activate bash)"
command -v fzf >/dev/null 2>&1 && eval "$(fzf --bash)"

[[ -r /usr/share/bash-completion/bash_completion ]] && . /usr/share/bash-completion/bash_completion
if [[ -r /usr/share/blesh/ble.sh ]]; then
  . /usr/share/blesh/ble.sh --noattach
  _ryoku_palette=${XDG_CONFIG_HOME:-$HOME/.config}/bash/ryoku-colors.bash
  if [[ -r $_ryoku_palette ]]; then
    . "$_ryoku_palette"
  else
    ble-face -s syntax_default fg=#F1F3E4 2>/dev/null || true
    ble-face -s syntax_command fg=#e2342a 2>/dev/null || true
    ble-face -s syntax_function_name fg=#e2342a 2>/dev/null || true
    ble-face -s command_builtin fg=#e2342a 2>/dev/null || true
    ble-face -s command_alias fg=#e2342a 2>/dev/null || true
    ble-face -s command_function fg=#e2342a 2>/dev/null || true
    ble-face -s command_file fg=#e2342a 2>/dev/null || true
    ble-face -s command_jobs fg=#e2342a 2>/dev/null || true
    ble-face -s command_keyword fg=#e83b30 2>/dev/null || true
    ble-face -s syntax_varname fg=#93D4E0 2>/dev/null || true
    ble-face -s syntax_param_expansion fg=#93D4E0 2>/dev/null || true
    ble-face -s syntax_history_expansion fg=#93D4E0 2>/dev/null || true
    ble-face -s syntax_glob fg=#93D4E0 2>/dev/null || true
    ble-face -s syntax_brace fg=#93D4E0 2>/dev/null || true
    ble-face -s syntax_delimiter fg=#93D4E0 2>/dev/null || true
    ble-face -s syntax_quoted fg=#A3C293 2>/dev/null || true
    ble-face -s syntax_quotation fg=#A3C293 2>/dev/null || true
    ble-face -s syntax_error fg=#FF6B6B 2>/dev/null || true
    ble-face -s syntax_comment fg=#949699 2>/dev/null || true
    ble-face -s auto_complete fg=#949699 2>/dev/null || true
  fi
  unset _ryoku_palette
fi
command -v starship >/dev/null 2>&1 && eval "$(starship init bash)"

if command -v eza >/dev/null 2>&1; then
  alias ls='eza -lh --group-directories-first --icons=auto'
  alias lsa='ls -a'
  alias lt='eza --tree --level=2 --long --icons --git'
  alias lta='lt -a'
fi

_ryoku_cfg=${XDG_CONFIG_HOME:-$HOME/.config}
[[ -r $_ryoku_cfg/bash/rashin.bash ]] && . "$_ryoku_cfg/bash/rashin.bash"
[[ -r $_ryoku_cfg/bash/user.bash ]] && . "$_ryoku_cfg/bash/user.bash"
unset _ryoku_cfg

if declare -F ble-attach >/dev/null; then
  ble-attach || true
fi

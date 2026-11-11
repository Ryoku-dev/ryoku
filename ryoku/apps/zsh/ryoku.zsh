[[ ${__RYOKU_ZSH_LOADED:-0} == 1 ]] && return
__RYOKU_ZSH_LOADED=1

_ryoku_env=${XDG_CONFIG_HOME:-$HOME/.config}/ryoku-terminal/env.sh
[[ -r $_ryoku_env ]] || _ryoku_env=${${(%):-%N}:A:h}/../terminal-shell/env.sh
[[ -r $_ryoku_env ]] && source "$_ryoku_env"
unset _ryoku_env

[[ -o interactive ]] || return

_ryoku_desktop_env=${XDG_CONFIG_HOME:-$HOME/.config}/ryoku-terminal/desktop-display.sh
[[ -r $_ryoku_desktop_env ]] || _ryoku_desktop_env=${${(%):-%N}:A:h}/../terminal-shell/desktop-display.sh
[[ -r $_ryoku_desktop_env ]] && source "$_ryoku_desktop_env"
unset _ryoku_desktop_env

command -v ryoku-fastfetch >/dev/null 2>&1 && ryoku-fastfetch

_ryoku_prompt=${RYOKU_ZSH_PROMPT:-starship}
_ryoku_omz=0
if [[ -r ${ZSH:-/usr/share/oh-my-zsh}/oh-my-zsh.sh ]]; then
  export ZSH=${ZSH:-/usr/share/oh-my-zsh}
  export ZSH_CUSTOM=${ZSH_CUSTOM:-${XDG_CONFIG_HOME:-$HOME/.config}/zsh/oh-my-zsh}
  DISABLE_AUTO_UPDATE=true
  ZSH_COMPDUMP=${XDG_CACHE_HOME:-$HOME/.cache}/zsh/zcompdump
  (( ${+plugins} )) || plugins=(git)
  if [[ $_ryoku_prompt == oh-my-zsh ]]; then
    : ${ZSH_THEME:=robbyrussell}
  else
    ZSH_THEME=""
  fi
  source "$ZSH/oh-my-zsh.sh"
  _ryoku_omz=1
fi

if [[ $_ryoku_prompt != oh-my-zsh || $_ryoku_omz != 1 ]]; then
  command -v starship >/dev/null 2>&1 && eval "$(starship init zsh)"
fi
command -v zoxide >/dev/null 2>&1 && eval "$(zoxide init zsh --cmd cd)"
command -v mise >/dev/null 2>&1 && eval "$(mise activate zsh)"
command -v fzf >/dev/null 2>&1 && eval "$(fzf --zsh)"

if (( ! _ryoku_omz )); then
  autoload -Uz compinit
  _cache=${XDG_CACHE_HOME:-$HOME/.cache}/zsh
  mkdir -p "$_cache"
  compinit -d "$_cache/zcompdump"
  unset _cache
fi

_ryoku_source_first() {
  local _ryoku_plugin
  for _ryoku_plugin in "$@"; do
    if [[ -r $_ryoku_plugin ]]; then
      source "$_ryoku_plugin"
      return
    fi
  done
  return 1
}

_ryoku_source_first \
  /usr/share/zsh/plugins/zsh-autosuggestions/zsh-autosuggestions.zsh \
  /usr/share/zsh-autosuggestions/zsh-autosuggestions.zsh || true
_ryoku_source_first \
  /usr/share/zsh/plugins/zsh-history-substring-search/zsh-history-substring-search.zsh || true
_ryoku_source_first \
  /usr/share/zsh/plugins/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh \
  /usr/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh || true
unset -f _ryoku_source_first

_ryoku_palette=${XDG_CONFIG_HOME:-$HOME/.config}/zsh/ryoku-colors.zsh
if [[ -r $_ryoku_palette ]]; then
  source "$_ryoku_palette"
else
  ZSH_AUTOSUGGEST_HIGHLIGHT_STYLE='fg=#949699'
  if (( ${+parameters[ZSH_HIGHLIGHT_STYLES]} )); then
    ZSH_HIGHLIGHT_STYLES[default]='fg=#F1F3E4'
    ZSH_HIGHLIGHT_STYLES[command]='fg=#e2342a'
    ZSH_HIGHLIGHT_STYLES[hashed-command]='fg=#e2342a'
    ZSH_HIGHLIGHT_STYLES[builtin]='fg=#e2342a'
    ZSH_HIGHLIGHT_STYLES[function]='fg=#e2342a'
    ZSH_HIGHLIGHT_STYLES[alias]='fg=#e2342a'
    ZSH_HIGHLIGHT_STYLES[suffix-alias]='fg=#e2342a'
    ZSH_HIGHLIGHT_STYLES[global-alias]='fg=#e2342a'
    ZSH_HIGHLIGHT_STYLES[reserved-word]='fg=#e83b30'
    ZSH_HIGHLIGHT_STYLES[precommand]='fg=#e83b30'
    ZSH_HIGHLIGHT_STYLES[commandseparator]='fg=#e83b30'
    ZSH_HIGHLIGHT_STYLES[single-hyphen-option]='fg=#CCD0CF'
    ZSH_HIGHLIGHT_STYLES[double-hyphen-option]='fg=#CCD0CF'
    ZSH_HIGHLIGHT_STYLES[single-quoted-argument]='fg=#A3C293'
    ZSH_HIGHLIGHT_STYLES[double-quoted-argument]='fg=#A3C293'
    ZSH_HIGHLIGHT_STYLES[dollar-quoted-argument]='fg=#A3C293'
    ZSH_HIGHLIGHT_STYLES[redirection]='fg=#8AA9CC'
    ZSH_HIGHLIGHT_STYLES[named-fd]='fg=#8AA9CC'
    ZSH_HIGHLIGHT_STYLES[numeric-fd]='fg=#8AA9CC'
    ZSH_HIGHLIGHT_STYLES[globbing]='fg=#93D4E0'
    ZSH_HIGHLIGHT_STYLES[history-expansion]='fg=#93D4E0'
    ZSH_HIGHLIGHT_STYLES[comment]='fg=#949699'
    ZSH_HIGHLIGHT_STYLES[unknown-token]='fg=#FF6B6B'
  fi
fi
unset _ryoku_palette

if (( $+functions[history-substring-search-up] )); then
  bindkey '^[[A' history-substring-search-up
  bindkey '^[[B' history-substring-search-down
  bindkey '^P' history-substring-search-up
  bindkey '^N' history-substring-search-down
fi

if command -v eza >/dev/null 2>&1; then
  alias ls='eza -lh --group-directories-first --icons=auto'
  alias lsa='ls -a'
  alias lt='eza --tree --level=2 --long --icons --git'
  alias lta='lt -a'
fi

_ryoku_cfg=${XDG_CONFIG_HOME:-$HOME/.config}
[[ -r $_ryoku_cfg/zsh/rashin.zsh ]] && source "$_ryoku_cfg/zsh/rashin.zsh"
[[ -r $_ryoku_cfg/zsh/user.zsh ]] && source "$_ryoku_cfg/zsh/user.zsh"

unset _ryoku_prompt _ryoku_omz
unset _ryoku_cfg

(( $+commands[rashin] )) || return

autoload -Uz add-zsh-hook

__rashin_set_buffer() {
  BUFFER=$1
  CURSOR=${#BUFFER}
}

rashin() {
  if (( $# == 0 )); then
    command rashin
    return
  fi
  if command rashin __is-command "${1:-}" >/dev/null 2>&1; then
    command rashin "$@"
    return
  fi
  local payload
  payload=$(RASHIN_LAST_CMD=${__rashin_last_cmd:-} RASHIN_LAST_STATUS=${__rashin_last_status:-0} \
    command rashin ask --buffer -- "$@") || return
  [[ -n $payload ]] || return
  __rashin_pending=$payload
}

__rashin_line_init() {
  if [[ -n ${__rashin_pending:-} ]]; then
    __rashin_set_buffer "$__rashin_pending"
    unset __rashin_pending
  fi
}
zle -N __rashin_line_init
add-zle-hook-widget line-init __rashin_line_init

__rashin_transmute() {
  [[ -n $BUFFER ]] || return
  local payload
  payload=$(RASHIN_LAST_CMD=${__rashin_last_cmd:-} RASHIN_LAST_STATUS=${__rashin_last_status:-0} \
    command rashin ask --buffer -- "$BUFFER" 2>/dev/null) || return
  [[ -n $payload ]] && __rashin_set_buffer "$payload"
}
zle -N __rashin_transmute
bindkey '\er' __rashin_transmute

__rashin_preexec() {
  __rashin_running=$1
}

__rashin_precmd() {
  __rashin_last_status=$?
  __rashin_last_cmd=${__rashin_running:-}
}
add-zsh-hook preexec __rashin_preexec
add-zsh-hook precmd __rashin_precmd

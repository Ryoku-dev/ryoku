(( $+commands[hyakki] )) || return

autoload -Uz add-zsh-hook

__hyakki_set_buffer() {
  BUFFER=$1
  CURSOR=${#BUFFER}
}

hyakki() {
  if (( $# == 0 )); then
    command hyakki
    return
  fi
  if command hyakki __is-command "${1:-}" >/dev/null 2>&1; then
    command hyakki "$@"
    return
  fi
  local payload
  payload=$(HYAKKI_LAST_CMD=${__hyakki_last_cmd:-} HYAKKI_LAST_STATUS=${__hyakki_last_status:-0} \
    command hyakki ask --buffer -- "$@") || return
  [[ -n $payload ]] || return
  __hyakki_pending=$payload
}

__hyakki_line_init() {
  if [[ -n ${__hyakki_pending:-} ]]; then
    __hyakki_set_buffer "$__hyakki_pending"
    unset __hyakki_pending
  fi
}
zle -N __hyakki_line_init
add-zle-hook-widget line-init __hyakki_line_init

__hyakki_transmute() {
  [[ -n $BUFFER ]] || return
  local payload
  payload=$(HYAKKI_LAST_CMD=${__hyakki_last_cmd:-} HYAKKI_LAST_STATUS=${__hyakki_last_status:-0} \
    command hyakki ask --buffer -- "$BUFFER" 2>/dev/null) || return
  [[ -n $payload ]] && __hyakki_set_buffer "$payload"
}
zle -N __hyakki_transmute
bindkey '\er' __hyakki_transmute

__hyakki_preexec() {
  __hyakki_running=$1
}

__hyakki_precmd() {
  __hyakki_last_status=$?
  __hyakki_last_cmd=${__hyakki_running:-}
}
add-zsh-hook preexec __hyakki_preexec
add-zsh-hook precmd __hyakki_precmd
